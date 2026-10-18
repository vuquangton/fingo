package migrate

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
)

// Runner applies a set of numbered migrations to one database, backing it up
// first and putting it back if any of them fails.
//
// One runner is one database's book. The migrations are loaded once — see Load —
// so that the set the runner will apply is the set the caller validated, not
// whatever is on disk at the moment the first statement runs.
type Runner struct {
	db         *sql.DB
	migrations []Migration

	// root is where backup directories are created.
	root string

	// now is the clock. Injected so that a test can place a backup and a
	// migration record in a known order, and so that a caller already holding a
	// clock keeps one notion of "now" through the whole run.
	now func() time.Time

	// keyTables narrows the post-backup row check. Empty means every table.
	keyTables []string

	// logf is told what the runner is doing. Nil means silent.
	logf func(format string, args ...any)
}

// Option configures a Runner.
type Option func(*Runner)

// WithBackupRoot changes where backups are written. The default is
// DefaultBackupRoot — "backups", relative to the working directory.
func WithBackupRoot(root string) Option {
	return func(r *Runner) {
		if strings.TrimSpace(root) != "" {
			r.root = root
		}
	}
}

// WithClock replaces the runner's clock.
func WithClock(now func() time.Time) Option {
	return func(r *Runner) {
		if now != nil {
			r.now = now
		}
	}
}

// WithKeyTables narrows the row-count check to the tables that must match. Every
// table is checked by default, which is the safer reading: a migration does not
// announce in advance which table it will damage.
func WithKeyTables(tables ...string) Option {
	return func(r *Runner) { r.keyTables = tables }
}

// WithLogger is told each step, so an upgrade that hangs is a visible one.
func WithLogger(logf func(format string, args ...any)) Option {
	return func(r *Runner) { r.logf = logf }
}

// New builds a runner over a database and a loaded migration set.
func New(db *sql.DB, migrations []Migration, opts ...Option) *Runner {
	r := &Runner{
		db:         db,
		migrations: migrations,
		root:       DefaultBackupRoot,
		now:        func() time.Time { return time.Now().UTC() },
	}

	for _, opt := range opts {
		opt(r)
	}

	return r
}

// Result reports what a run did.
type Result struct {
	// Version is the schema's version after the run: the highest version
	// recorded in schema_migrations.
	Version int64

	// BackupDir is the backup taken before this run, or "" when there was
	// nothing to do. A run with no pending migrations takes no backup, because
	// the rule is "before a migration", and nothing ran.
	BackupDir string

	// Applied is what this run applied, in order. Empty after a rollback: the
	// backup precedes the whole batch, so a restore undoes the batch.
	Applied []Record

	// Skipped is what was already applied and was therefore not run again.
	Skipped []Migration
}

// Migrate brings the database up to the loaded set's version.
//
// The order is the point: check the bookkeeping, refuse a set that has drifted,
// work out what is pending, take a backup of the database as it stands, prove
// the backup usable, and only then change anything. A failure at any point
// before the first statement leaves the database untouched.
//
// The backup covers the whole batch rather than each migration. One restore
// point is easier to reason about than five, and a batch that fails halfway is
// rolled back to where it started rather than to the middle of itself — there is
// no such thing as "partway through a schema upgrade" that is worth keeping.
func (r *Runner) Migrate(ctx context.Context) (Result, error) {
	var result Result

	if err := ensureBookkeeping(ctx, r.db); err != nil {
		return result, err
	}

	done, err := applied(ctx, r.db)
	if err != nil {
		return result, err
	}

	// An applied migration whose file has changed is not re-run and not
	// skipped: either answer leaves the database holding a schema that neither
	// the file nor the row describes.
	for _, m := range r.migrations {
		if rec, ok := done[m.Version]; ok && rec.Checksum != m.Checksum {
			return result, fmt.Errorf("%w: %s was applied with checksum %s and now hashes to %s",
				ErrChecksumDrift, m.Path, rec.Checksum, m.Checksum)
		}
	}

	var pending []Migration

	for _, m := range r.migrations {
		if _, ok := done[m.Version]; ok {
			result.Skipped = append(result.Skipped, m)

			continue
		}

		pending = append(pending, m)
	}

	for version := range done {
		if version > result.Version {
			result.Version = version
		}
	}

	if len(pending) == 0 {
		r.log("nothing to do: %d migration(s) already applied, schema is at version %d",
			len(result.Skipped), result.Version)

		return result, nil
	}

	r.log("%d migration(s) pending; backing the database up first", len(pending))

	dir, err := Backup(ctx, r.db, r.root, r.now)
	if err != nil {
		return result, fmt.Errorf("back up before migrating: %w", err)
	}
	result.BackupDir = dir

	if err := VerifyBackup(ctx, r.db, dir, r.keyTables); err != nil {
		return result, fmt.Errorf("the backup taken before migrating is not usable: %w", err)
	}

	r.log("backup verified at %s", dir)

	for _, m := range pending {
		started := r.now().UTC()

		if err := applyMigration(ctx, r.db, m); err != nil {
			return r.rollback(ctx, result, dir, m, fmt.Errorf("apply migration %d (%s): %w", m.Version, m.Name, err))
		}

		rec := Record{
			Version:    m.Version,
			Name:       m.Name,
			Checksum:   m.Checksum,
			AppliedAt:  r.now().UTC(),
			DurationMS: r.now().UTC().Sub(started).Milliseconds(),
		}

		// The record is part of the migration, not a note about it. A schema
		// that changed without a row saying so would be applied a second time
		// on the next run, so a failure to record rolls back as well.
		if err := record(ctx, r.db, rec); err != nil {
			return r.rollback(ctx, result, dir, m, fmt.Errorf("record migration %d (%s): %w", m.Version, m.Name, err))
		}

		result.Applied = append(result.Applied, rec)
		result.Version = m.Version

		r.log("applied %d %s", m.Version, m.Name)
	}

	return result, nil
}

// applyMigration sends a migration's statements in order.
//
// No transaction wraps them. MySQL commits DDL implicitly, so a transaction here
// would be a promise it cannot keep; what actually makes a part-applied
// migration safe is the backup, and that is the point of this package.
func applyMigration(ctx context.Context, db Querier, m Migration) error {
	for i, statement := range m.Statements {
		if _, err := db.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("statement %d of %d: %w", i+1, len(m.Statements), err)
		}
	}

	return nil
}

// rollback restores the database and reports both what went wrong and what was
// done about it.
//
// The error names the backup directory either way. A caller whose restore also
// failed is being told their database is part-way through a migration and needs
// a person; a caller whose restore succeeded is being told nothing was left
// behind. Those are different emergencies and must not read the same.
func (r *Runner) rollback(ctx context.Context, result Result, dir string, m Migration, cause error) (Result, error) {
	if err := Restore(ctx, r.db, dir); err != nil {
		return result, fmt.Errorf(
			"%w; the database could NOT be restored from %s and is part-way through migration %d (%s), which needs attention: %v",
			cause, dir, m.Version, m.Name, err)
	}

	// Everything this run applied is gone: the backup was taken before the
	// batch, so the restore undoes the batch.
	result.Applied = nil

	return result, fmt.Errorf(
		"%w; the database was restored from %s, so none of this run's migrations remain",
		cause, dir)
}

// log writes a progress line, if the runner was given somewhere to write them.
func (r *Runner) log(format string, args ...any) {
	if r.logf != nil {
		r.logf(format, args...)
	}
}

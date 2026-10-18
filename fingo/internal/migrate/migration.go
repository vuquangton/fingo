// Package migrate runs a database's schema migrations, with a verified backup
// taken first and a restore taken automatically when one fails.
//
// The rule the package exists to keep is that a schema change is never the only
// copy of anything. Before a single statement runs, the database as it stands is
// written to backups/YYYY-MM-DD_HH-MM-SS/, the backup is checked against the
// live rows, and only then is the migration applied. A migration that fails
// halfway leaves the schema neither old nor new — MySQL commits DDL as it goes
// and cannot roll it back — so the runner puts the database back rather than
// leaving it in between.
//
// Bookkeeping is a schema_migrations table, one row per applied version. A
// migration already recorded there is not run again, and one whose file has
// changed since it was applied is refused rather than skipped: an edited
// migration is a new migration wearing an old number, and the schema it would
// produce does not match what the row claims.
package migrate

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"path"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// Errors the loader and the runner can produce.
var (
	// ErrNoMigrations is a directory with no .sql files in it.
	ErrNoMigrations = errors.New("migrate: the migrations directory holds no .sql files")

	// ErrBadFilename is a .sql file whose name is not NNNN_name.sql.
	ErrBadFilename = errors.New("migrate: malformed migration filename")

	// ErrDuplicateVersion is two files claiming the same number.
	ErrDuplicateVersion = errors.New("migrate: two migrations share a version")

	// ErrVersionGap is a missing number in the run. Migrations are numbered
	// consecutively so that "which ones have I got?" is answerable from the
	// highest number alone; a gap means a file was renamed or deleted, and the
	// schema it produced is now unreproducible.
	ErrVersionGap = errors.New("migrate: migration versions are not sequential")

	// ErrEmptyMigration is a file with no statements in its Up section.
	ErrEmptyMigration = errors.New("migrate: a migration has no statements")

	// ErrChecksumDrift is an applied migration whose file no longer matches
	// what was applied. Re-running it is not the answer and neither is skipping
	// it quietly: the database already holds a schema this file does not
	// describe.
	ErrChecksumDrift = errors.New("migrate: an applied migration has been edited since it was applied")
)

// migrationName parses "00007_idempotency_keys.sql" into version and name.
var migrationName = regexp.MustCompile(`^([0-9]+)(?:[_-](.+))?\.sql$`)

// Migration is one numbered, sequential step in a schema's history.
type Migration struct {
	// Version is the number the file is prefixed with. It is the identity of
	// the migration and the primary key of its row in schema_migrations.
	Version int64

	// Name is the rest of the filename — what the migration does, for a human
	// reading a list of them.
	Name string

	// Path is where the file was read from, for an error message.
	Path string

	// SQL is the Up section: what runs against the database.
	SQL string

	// Statements is SQL split into the statements that will be sent.
	Statements []string

	// Checksum is the SHA-256 of the whole file, Down section included. An
	// edit to either half is an edit to what the migration means.
	Checksum string
}

// Load reads a directory of numbered .sql files and returns them in version
// order.
//
// Two shapes are accepted. A plain file is its own Up section. A file carrying
// goose's "-- +goose Up" and "-- +goose Down" markers is read for the Up half
// only, which is what lets this runner read a directory this repository already
// has, rather than asking for a second copy of every migration in a new format.
//
// Versions must be unique and must run 1, 2, 3 ... with nothing missing. That is
// stricter than goose, deliberately: goose tolerates a gap because it tracks
// what it applied, and this package would rather refuse a set it cannot describe
// than apply half of one.
func Load(fsys fs.FS, dir string) ([]Migration, error) {
	entries, err := fs.ReadDir(fsys, dir)
	if err != nil {
		return nil, fmt.Errorf("read migrations from %s: %w", dir, err)
	}

	var out []Migration

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(strings.ToLower(entry.Name()), ".sql") {
			continue
		}

		m, err := parseMigration(fsys, path.Join(dir, entry.Name()), entry.Name())
		if err != nil {
			return nil, err
		}

		out = append(out, m)
	}

	if len(out) == 0 {
		return nil, fmt.Errorf("%w: %s", ErrNoMigrations, dir)
	}

	if err := validateVersions(out); err != nil {
		return nil, err
	}

	sort.Slice(out, func(i, j int) bool { return out[i].Version < out[j].Version })

	return out, nil
}

// parseMigration reads one file into a Migration.
func parseMigration(fsys fs.FS, fullPath, filename string) (Migration, error) {
	matches := migrationName.FindStringSubmatch(filename)
	if matches == nil {
		return Migration{}, fmt.Errorf("%w: %q, want NNNN_name.sql", ErrBadFilename, filename)
	}

	version, err := strconv.ParseInt(matches[1], 10, 64)
	if err != nil || version <= 0 {
		return Migration{}, fmt.Errorf("%w: %q carries an unusable version", ErrBadFilename, filename)
	}

	raw, err := fs.ReadFile(fsys, fullPath)
	if err != nil {
		return Migration{}, fmt.Errorf("read %s: %w", fullPath, err)
	}

	sum := sha256.Sum256(raw)

	up, err := upSection(string(raw))
	if err != nil {
		return Migration{}, fmt.Errorf("%s: %w", filename, err)
	}

	statements := splitStatements(up)
	if len(statements) == 0 {
		return Migration{}, fmt.Errorf("%w: %s", ErrEmptyMigration, filename)
	}

	return Migration{
		Version:    version,
		Name:       matches[2],
		Path:       fullPath,
		SQL:        up,
		Statements: statements,
		Checksum:   hex.EncodeToString(sum[:]),
	}, nil
}

// upSection returns the part of a migration file that runs forward.
//
// A file with no goose markers is returned whole: the convention for a plain
// migration is that the file is the change, and a Down section is something a
// numbered migration does not need — the backup taken before it is the way back.
func upSection(sql string) (string, error) {
	lines := strings.Split(sql, "\n")

	start := -1

	for i, line := range lines {
		switch directive(line) {
		case "up":
			start = i + 1
		case "down":
			if start >= 0 {
				return strings.Join(lines[start:i], "\n"), nil
			}
		}
	}

	if start >= 0 {
		return strings.Join(lines[start:], "\n"), nil
	}

	return sql, nil
}

// directive reads a goose marker line, ignoring the spacing a hand-edited file
// may have introduced around it.
func directive(line string) string {
	trimmed := strings.TrimSpace(line)
	if !strings.HasPrefix(trimmed, "--") {
		return ""
	}

	trimmed = strings.TrimSpace(strings.TrimPrefix(trimmed, "--"))
	if !strings.HasPrefix(trimmed, "+goose") {
		return ""
	}

	switch strings.TrimSpace(strings.TrimPrefix(trimmed, "+goose")) {
	case "Up":
		return "up"
	case "Down":
		return "down"
	}

	return ""
}

// validateVersions refuses a set that is not 1, 2, 3 ... with nothing missing.
func validateVersions(migrations []Migration) error {
	seen := make(map[int64]string, len(migrations))

	for _, m := range migrations {
		if other, dup := seen[m.Version]; dup {
			return fmt.Errorf("%w: %d is both %s and %s", ErrDuplicateVersion, m.Version, other, m.Path)
		}

		seen[m.Version] = m.Path
	}

	for want := int64(1); want <= int64(len(migrations)); want++ {
		if _, ok := seen[want]; !ok {
			return fmt.Errorf("%w: version %d is missing", ErrVersionGap, want)
		}
	}

	return nil
}

package migrate

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
)

// Restore puts the database back the way a backup found it.
//
// This is destructive by design: every table is dropped and rebuilt from the
// dump. It is called when a migration has failed partway, where the alternative
// is a schema that is neither the old one nor the new one — MySQL commits DDL as
// it runs and cannot roll it back, so a half-applied migration is a real state
// that only a restore can leave.
//
// Three things guard it, in this order. The backup's manifest must read and its
// data file must still hash to what the manifest says — checked before anything
// is dropped, because restoring from a corrupt backup over a merely broken
// schema would turn a recoverable failure into an unrecoverable one. The backup
// must name the database the connection is pointed at, so a restore cannot be
// aimed at the wrong client's books. And the row counts are checked afterwards,
// so a restore that did not actually work reports that instead of returning
// success.
func Restore(ctx context.Context, db *sql.DB, dir string) error {
	manifest, err := verifyArtifact(dir)
	if err != nil {
		return err
	}

	current, err := currentDatabase(ctx, db)
	if err != nil {
		return err
	}
	if current != manifest.Database {
		return fmt.Errorf("%w: the backup is from %s and this connection is pointed at %s",
			ErrWrongDatabase, manifest.Database, current)
	}

	// One connection for the whole restore. See Querier.
	conn, err := db.Conn(ctx)
	if err != nil {
		return fmt.Errorf("open a dedicated connection for the restore: %w", err)
	}

	// The reset is deferred as well as done on the way out, because every
	// early return above it leaves the flag set. FOREIGN_KEY_CHECKS is session
	// state and this connection goes back into the pool: left off, it would
	// silently disable constraint checking for whatever borrows the connection
	// next, which on this path is the rest of the migration run.
	//
	// Best effort, and deliberately so: the restore has already succeeded or
	// failed by this point, and failing to reset a flag is not a reason to
	// report either of those differently.
	defer func() {
		_, _ = conn.ExecContext(context.WithoutCancel(ctx), "SET FOREIGN_KEY_CHECKS=1")
		_ = conn.Close()
	}()

	if _, err := conn.ExecContext(ctx, "SET FOREIGN_KEY_CHECKS=0"); err != nil {
		return fmt.Errorf("suspend foreign key checks before restoring: %w", err)
	}

	// The tables are dropped in name order with the checks off, and the dump
	// puts them back the same way. A restore that left the constraints on would
	// fail on the first table whose parent had not been recreated yet.
	if tables, err := listTables(ctx, conn); err != nil {
		return err
	} else {
		for _, table := range tables {
			if _, err := conn.ExecContext(ctx, "DROP TABLE IF EXISTS `"+table+"`"); err != nil {
				return fmt.Errorf("drop %s before restoring: %w", table, err)
			}
		}
	}

	if err := execScript(ctx, conn, filepath.Join(dir, backupSchemaFile)); err != nil {
		return fmt.Errorf("restore the schema: %w", err)
	}
	if err := execScript(ctx, conn, filepath.Join(dir, backupDataFile)); err != nil {
		return fmt.Errorf("restore the data: %w", err)
	}

	// The checks go back on before the verification, so that what is verified is
	// the database as it will be used. The deferred reset is the safety net for
	// the paths that return before this line.
	if _, err := conn.ExecContext(ctx, "SET FOREIGN_KEY_CHECKS=1"); err != nil {
		return fmt.Errorf("reinstate foreign key checks after restoring: %w", err)
	}

	// The restore is only a restore if it put back what the backup holds. It is
	// verified on the connection this restore already holds rather than on the
	// pool: waiting for a second connection while holding the first is a
	// deadlock on a pool of one.
	return verifyCounts(ctx, conn, manifest, nil)
}

// execScript runs every statement in a SQL file, in order.
//
// The statements are split rather than sent whole: the connection this runs on
// has multiStatements off, which is the setting FinGo wants everywhere — one
// statement per Exec is one statement per error message.
func execScript(ctx context.Context, db Querier, path string) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}

	statements := splitStatements(string(raw))
	if len(statements) == 0 {
		return fmt.Errorf("%w: %s holds no statements", ErrBackupIncomplete, filepath.Base(path))
	}

	for i, statement := range statements {
		if _, err := db.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("%s, statement %d of %d: %w",
				filepath.Base(path), i+1, len(statements), err)
		}
	}

	return nil
}

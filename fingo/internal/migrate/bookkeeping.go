package migrate

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// Querier is the part of *sql.DB and *sql.Conn these helpers need.
//
// It exists because a restore has to run on one pinned connection: switching
// FOREIGN_KEY_CHECKS off is a session setting, and a pool that handed the next
// statement to a different session would enforce exactly the constraints the
// switch was there to suspend. Reading through this interface also means a
// caller holding a connection can verify a backup without asking the pool for a
// second one — which, on a pool of one, would never be granted.
type Querier interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

// schemaMigrationsTable is where applied versions are recorded.
//
// A table of this package's own, not goose's. goose_db_version records what
// goose applied and is written by goose; this records what this runner applied,
// including the backup that preceded it, and is written by this runner. The two
// are not kept in step — see the note at the top of the package.
const schemaMigrationsTable = "schema_migrations"

// schemaMigrationsDDL creates the bookkeeping table.
//
// duration_ms is stored because "how long did that take?" is the first question
// asked when a migration goes wrong on a client's machine, and a number nobody
// wrote down is a number nobody has. applied_at is DATETIME(6) in UTC like every
// other instant in this schema.
const schemaMigrationsDDL = `
CREATE TABLE IF NOT EXISTS ` + schemaMigrationsTable + ` (
    version BIGINT NOT NULL,
    name VARCHAR(255) NOT NULL,
    checksum CHAR(64) NOT NULL,
    applied_at DATETIME(6) NOT NULL,
    duration_ms BIGINT NOT NULL,
    PRIMARY KEY (version)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci`

// Record is one applied migration.
type Record struct {
	Version    int64
	Name       string
	Checksum   string
	AppliedAt  time.Time
	DurationMS int64
}

// ensureBookkeeping creates schema_migrations if it is not there yet.
func ensureBookkeeping(ctx context.Context, db Querier) error {
	if _, err := db.ExecContext(ctx, schemaMigrationsDDL); err != nil {
		return fmt.Errorf("create %s: %w", schemaMigrationsTable, err)
	}

	return nil
}

// applied returns the migrations already recorded, by version.
func applied(ctx context.Context, db Querier) (map[int64]Record, error) {
	rows, err := db.QueryContext(ctx,
		`SELECT version, name, checksum, applied_at, duration_ms FROM `+schemaMigrationsTable)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", schemaMigrationsTable, err)
	}
	defer rows.Close()

	out := make(map[int64]Record)

	for rows.Next() {
		var r Record
		if err := rows.Scan(&r.Version, &r.Name, &r.Checksum, &r.AppliedAt, &r.DurationMS); err != nil {
			return nil, fmt.Errorf("scan %s row: %w", schemaMigrationsTable, err)
		}

		out[r.Version] = r
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate %s: %w", schemaMigrationsTable, err)
	}

	return out, nil
}

// record writes one applied migration.
func record(ctx context.Context, db Querier, r Record) error {
	if _, err := db.ExecContext(ctx,
		`INSERT INTO `+schemaMigrationsTable+` (version, name, checksum, applied_at, duration_ms)
		 VALUES (?, ?, ?, ?, ?)`,
		r.Version, r.Name, r.Checksum, r.AppliedAt.UTC(), r.DurationMS); err != nil {
		return fmt.Errorf("record migration %d: %w", r.Version, err)
	}

	return nil
}

// Package onboarding answers one question — can this installation reach its
// books? — and, when it cannot, does the least that makes the answer yes.
//
// It is deliberately separate from appconfig, which only reads and writes a
// file, and from practice, which opens books and expects them to be there. This
// is the step in between: the one that runs before there is anything to open,
// and the one whose failures an operator has to read and act on. Every message
// it produces is written to be shown to that operator unchanged.
package onboarding

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"

	"fingo/internal/appconfig"
)

// probeTimeout bounds a probe.
//
// A wrong host name is the commonest thing to get wrong in a connection form,
// and a driver that waits out its own TCP timeout leaves the operator staring at
// a spinner. Ten seconds is long enough for a slow LAN and short enough to be
// obviously a failure rather than a hang.
const probeTimeout = 10 * time.Second

// databaseName is what a database name may contain, when this package is asked
// to create one.
//
// An allowlist rather than escaping alone. A name is used as an identifier, and
// identifiers cannot be bound as parameters — so the choice is between quoting
// and refusing, and refusing also catches the typo that would otherwise create a
// database nobody meant to create. A name outside this set can still be probed;
// it just cannot be created from here.
var databaseName = regexp.MustCompile(`^[A-Za-z0-9_$-]+$`)

// Report is what a probe found.
//
// It is a report and not a verdict: an operator wants to know *which* step
// failed — the server, the database, the schema or the chart — because each one
// is a different thing to go and fix.
type Report struct {
	// ServerReachable is whether the server answered at all.
	ServerReachable bool `json:"serverReachable"`

	// ServerVersion is what the server says it is. Recorded because a dump
	// restored onto a different major version is a question somebody asks later.
	ServerVersion string `json:"serverVersion,omitempty"`

	// DatabaseExists is whether this client's database is there.
	DatabaseExists bool `json:"databaseExists"`

	// SchemaVersion is the highest migration applied, or 0 when the database
	// exists but has never been migrated.
	SchemaVersion int64 `json:"schemaVersion"`

	// Tables is how many tables the database holds.
	Tables int `json:"tables"`

	// Accounts is how many accounts the chart holds. Zero on a migrated but
	// unseeded database, which is the difference between "no schema" and "no
	// chart" — two different fixes.
	Accounts int `json:"accounts"`

	// Ready is whether the application can open these books as they stand.
	Ready bool `json:"ready"`

	// Missing names, in order, what has to happen before Ready can be true.
	Missing []string `json:"missing,omitempty"`
}

// Probe reports what is and is not there, and changes nothing.
//
// It never creates, never migrates and never seeds. An operator who typed the
// wrong database name must be able to find that out without having written
// anything into somebody else's books — which is exactly what a probe that
// "helpfully" prepared the database would have done.
func Probe(ctx context.Context, c appconfig.Connection) (Report, error) {
	ctx, cancel := context.WithTimeout(ctx, probeTimeout)
	defer cancel()

	var report Report

	server, err := open(c, "")
	if err != nil {
		return Report{}, err
	}
	defer server.Close()

	if err := server.PingContext(ctx); err != nil {
		return Report{}, fmt.Errorf("cannot reach the database server at %s:%s: %w",
			c.Server.Host, c.Server.Port, err)
	}

	report.ServerReachable = true

	// Worth recording, not worth failing over.
	_ = server.QueryRowContext(ctx, "SELECT VERSION()").Scan(&report.ServerVersion)

	var found int
	if err := server.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM information_schema.schemata WHERE schema_name = ?`,
		c.Database).Scan(&found); err != nil {
		return report, fmt.Errorf("look for the database %s: %w", c.Database, err)
	}

	report.DatabaseExists = found > 0

	if !report.DatabaseExists {
		report.Missing = []string{"database", "schema", "chart"}

		return report, nil
	}

	client, err := open(c, c.Database)
	if err != nil {
		return report, err
	}
	defer client.Close()

	if err := client.PingContext(ctx); err != nil {
		return report, fmt.Errorf("the server is reachable but %s could not be opened: %w", c.Database, err)
	}

	report.Tables = int(count(ctx, client,
		`SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = DATABASE()`))

	if hasTable(ctx, client, "goose_db_version") {
		report.SchemaVersion = count(ctx, client,
			`SELECT COALESCE(MAX(version_id), 0) FROM goose_db_version WHERE is_applied = 1`)
	}

	if hasTable(ctx, client, "accounts") {
		report.Accounts = int(count(ctx, client, `SELECT COUNT(*) FROM accounts`))
	}

	if report.SchemaVersion == 0 {
		report.Missing = append(report.Missing, "schema")
	}
	if report.Accounts == 0 {
		report.Missing = append(report.Missing, "chart")
	}

	report.Ready = len(report.Missing) == 0

	return report, nil
}

// EnsureDatabase creates the client's database if it is not there.
//
// It does not migrate and does not seed. Those are practice.Open's job and it
// does them by opening the database, so doing them here as well would be two
// places running the same migrations — and the second one is the one nobody
// expects.
//
// The name is checked against an allowlist before it is ever used as an
// identifier. Identifiers cannot be bound as parameters, so the alternative is
// quoting and hoping, and a typo that creates a database nobody meant is a
// mistake that outlives the typo.
func EnsureDatabase(ctx context.Context, c appconfig.Connection) error {
	name := strings.TrimSpace(c.Database)
	if !databaseName.MatchString(name) {
		return fmt.Errorf("%w: %q — use letters, digits, underscore, dash or dollar",
			ErrUnusableDatabase, name)
	}

	ctx, cancel := context.WithTimeout(ctx, probeTimeout)
	defer cancel()

	server, err := open(c, "")
	if err != nil {
		return err
	}
	defer server.Close()

	if err := server.PingContext(ctx); err != nil {
		return fmt.Errorf("cannot reach the database server at %s:%s: %w", c.Server.Host, c.Server.Port, err)
	}

	// Already there? Then there is nothing to create and nothing to need
	// permission for. An operator whose login may use a database but not create
	// one is an ordinary setup, and this must not be the step that refuses them.
	var found int
	if err := server.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM information_schema.schemata WHERE schema_name = ?`, name).Scan(&found); err != nil {
		return fmt.Errorf("look for the database %s: %w", name, err)
	}
	if found > 0 {
		return nil
	}

	if _, err := server.ExecContext(ctx, fmt.Sprintf(
		"CREATE DATABASE IF NOT EXISTS `%s` CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci", name)); err != nil {
		return fmt.Errorf("create the database %s: %w", name, err)
	}

	return nil
}

// open returns a connection to the server, optionally to one database on it.
//
// The database is opened separately from the server so that looking for a
// database does not require opening the database — which cannot be done when it
// is not there yet, and is the whole question at that point.
func open(c appconfig.Connection, database string) (*sql.DB, error) {
	cfg := c.Config()

	// A dial timeout as well as the context: the driver's own TCP timeout is
	// otherwise the kernel's, which is minutes on Windows.
	if cfg.Params == nil {
		cfg.Params = map[string]string{}
	}
	if _, set := cfg.Params["timeout"]; !set {
		cfg.Params["timeout"] = "10s"
	}

	db, err := sql.Open("mysql", cfg.DSN(database))
	if err != nil {
		return nil, fmt.Errorf("build a connection to %s:%s: %w", cfg.Host, cfg.Port, err)
	}

	// One connection, held for the length of the probe. A pool would retry a
	// dead server on every query and turn one failure into four messages.
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)

	return db, nil
}

// hasTable reports whether the current database holds a table.
func hasTable(ctx context.Context, db *sql.DB, table string) bool {
	var found int

	err := db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM information_schema.tables
		 WHERE table_schema = DATABASE() AND table_name = ?`, table).Scan(&found)

	return err == nil && found > 0
}

// count runs a count and treats a failure as nothing there.
//
// Every use is a question about what is present, and "the query failed" and "it
// is not there" lead to the same next step. A probe that returned an error for a
// missing table would fail on precisely the fresh installation it exists for.
func count(ctx context.Context, db *sql.DB, query string) int64 {
	var out int64

	if err := db.QueryRowContext(ctx, query).Scan(&out); err != nil {
		return 0
	}

	return out
}

// ErrUnusableDatabase is a database name this package will not create.
var ErrUnusableDatabase = errors.New("onboarding: the database name is not usable")

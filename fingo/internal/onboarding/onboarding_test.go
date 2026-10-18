package onboarding_test

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	_ "github.com/go-sql-driver/mysql"

	"fingo/internal/adapter/mariadb/conn"
	"fingo/internal/appconfig"
	"fingo/internal/onboarding"
	"fingo/internal/usecase/practice"
)

// scratchPrefix guards the DROP: this test may only drop databases it named
// itself. The prefix is checked in Go and never matched in SQL, because '_' is a
// LIKE wildcard.
const scratchPrefix = "fingo_onboard_test_"

// configuration reads the nearest .env by walking up from the test's directory.
func configuration(t *testing.T) conn.Config {
	t.Helper()

	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}

	for i := 0; i < 4; i++ {
		path := filepath.Join(dir, ".env")
		if _, err := os.Stat(path); err == nil {
			cfg, _, err := conn.FromEnvFile(path)
			if err != nil {
				t.Fatalf("config from %s: %v", path, err)
			}

			return cfg
		}

		dir = filepath.Dir(dir)
	}

	t.Skip("no .env found; skipping database integration test")

	return conn.Config{}
}

// target builds a connection to a database that is expected not to exist.
func target(t *testing.T, database string) appconfig.Connection {
	t.Helper()

	if testing.Short() {
		t.Skip("integration test skipped in short mode")
	}

	cfg := configuration(t)

	admin, err := sql.Open("mysql", cfg.DSN(""))
	if err != nil {
		t.Fatalf("open admin connection: %v", err)
	}
	t.Cleanup(func() { admin.Close() })

	if err := admin.Ping(); err != nil {
		t.Skipf("MariaDB not reachable at %s:%s: %v", cfg.Host, cfg.Port, err)
	}

	t.Cleanup(func() {
		if !strings.HasPrefix(database, scratchPrefix) {
			t.Errorf("refusing to drop %q: outside the scratch prefix", database)

			return
		}
		if _, err := admin.Exec("DROP DATABASE IF EXISTS `" + database + "`"); err != nil {
			t.Errorf("drop scratch database %s: %v", database, err)
		}
	})

	return appconfig.Connection{Server: cfg, Database: database}
}

func scratchName() string { return fmt.Sprintf("%s%d", scratchPrefix, time.Now().UnixNano()) }

func exists(t *testing.T, c appconfig.Connection) bool {
	t.Helper()

	admin, err := sql.Open("mysql", c.Server.DSN(""))
	if err != nil {
		t.Fatalf("open admin connection: %v", err)
	}
	defer admin.Close()

	var found int

	if err := admin.QueryRow(
		`SELECT COUNT(*) FROM information_schema.schemata WHERE schema_name = ?`, c.Database).Scan(&found); err != nil {
		t.Fatalf("look for %s: %v", c.Database, err)
	}

	return found > 0
}

// A probe reports and changes nothing. An operator who mistyped a database name
// has to be able to find that out without having written anything into somebody
// else's books.
func TestProbe_ReportsAFreshDatabaseWithoutCreatingIt(t *testing.T) {
	ctx := context.Background()
	c := target(t, scratchName())

	report, err := onboarding.Probe(ctx, c)
	if err != nil {
		t.Fatalf("Probe: %v", err)
	}

	if !report.ServerReachable {
		t.Error("the server is running and was reported unreachable")
	}
	if report.ServerVersion == "" {
		t.Error("the server version was not recorded")
	}
	if report.DatabaseExists {
		t.Error("a database that does not exist was reported present")
	}
	if report.Ready {
		t.Error("a database that does not exist was reported ready")
	}

	want := []string{"database", "schema", "chart"}
	if strings.Join(report.Missing, ",") != strings.Join(want, ",") {
		t.Errorf("Missing = %v, want %v", report.Missing, want)
	}

	// The probe is a read. If it were not, this test would have created the
	// database it was asked about.
	if exists(t, c) {
		t.Fatal("probing created the database")
	}
}

// The whole onboarding path: create the database, migrate it, seed the chart,
// and the same probe that reported it missing reports it ready.
func TestEnsureDatabase_ThenMigrateAndSeedMakesItReady(t *testing.T) {
	ctx := context.Background()
	c := target(t, scratchName())

	if err := onboarding.EnsureDatabase(ctx, c); err != nil {
		t.Fatalf("EnsureDatabase: %v", err)
	}
	if !exists(t, c) {
		t.Fatal("EnsureDatabase reported success and created nothing")
	}

	// Created, but nothing in it: the difference between "no schema" and "no
	// chart" is two different fixes, so the report has to tell them apart.
	report, err := onboarding.Probe(ctx, c)
	if err != nil {
		t.Fatalf("Probe after creating: %v", err)
	}
	if !report.DatabaseExists {
		t.Error("the database was created and is reported missing")
	}
	if report.SchemaVersion != 0 {
		t.Errorf("SchemaVersion = %d on an empty database, want 0", report.SchemaVersion)
	}
	if report.Ready {
		t.Error("an empty database was reported ready")
	}
	if !contains(report.Missing, "schema") || !contains(report.Missing, "chart") {
		t.Errorf("Missing = %v, want schema and chart", report.Missing)
	}

	// practice.Open is the only path into a client's books: it migrates and it
	// is what the application calls next.
	client, err := practice.Open(ctx, c.Config(), c.Database)
	if err != nil {
		t.Fatalf("practice.Open: %v", err)
	}
	t.Cleanup(func() { client.Close() })

	if err := client.EnsureChart(ctx); err != nil {
		t.Fatalf("EnsureChart: %v", err)
	}

	report, err = onboarding.Probe(ctx, c)
	if err != nil {
		t.Fatalf("Probe after migrating: %v", err)
	}
	if !report.Ready {
		t.Errorf("a migrated, seeded database is not ready: %+v", report)
	}
	if report.SchemaVersion == 0 {
		t.Error("the schema version is still 0 after migrating")
	}
	if report.Accounts == 0 {
		t.Error("the chart is still empty after seeding")
	}
	if len(report.Missing) != 0 {
		t.Errorf("Missing = %v, want nothing", report.Missing)
	}
}

// Running it twice is not an error: the screen that calls it may be in front of
// an operator who is not sure whether it worked.
func TestEnsureDatabase_IsIdempotent(t *testing.T) {
	ctx := context.Background()
	c := target(t, scratchName())

	for i := 1; i <= 2; i++ {
		if err := onboarding.EnsureDatabase(ctx, c); err != nil {
			t.Fatalf("EnsureDatabase call %d: %v", i, err)
		}
	}
}

// A name this package will not use as an identifier is refused with something an
// operator can act on, and nothing is created.
func TestEnsureDatabase_RefusesANameItCannotUseSafely(t *testing.T) {
	ctx := context.Background()
	c := target(t, scratchName())
	c.Database = "fingo bad`name"

	err := onboarding.EnsureDatabase(ctx, c)
	if !errors.Is(err, onboarding.ErrUnusableDatabase) {
		t.Fatalf("EnsureDatabase = %v, want ErrUnusableDatabase", err)
	}
	if !strings.Contains(err.Error(), "letters, digits") {
		t.Errorf("the error does not say what would be usable: %v", err)
	}
}

// An unreachable server is an error, and the message names the address that
// could not be reached — that is the thing the operator typed.
func TestProbe_ReportsAServerItCannotReach(t *testing.T) {
	cfg := configuration(t)

	connection, err := appconfig.NewConnection(cfg.Host, "1", cfg.User, cfg.Password, "fingo", "timeout=2s")
	if err != nil {
		t.Fatalf("NewConnection: %v", err)
	}

	_, err = onboarding.Probe(context.Background(), connection)
	if err == nil {
		t.Fatal("a port nothing is listening on was reported reachable")
	}
	if !strings.Contains(err.Error(), cfg.Host+":1") {
		t.Errorf("the error does not name the address: %v", err)
	}
}

func contains(haystack []string, needle string) bool {
	for _, item := range haystack {
		if item == needle {
			return true
		}
	}

	return false
}

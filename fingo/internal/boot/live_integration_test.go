package boot_test

import (
	"context"
	"database/sql"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	_ "github.com/go-sql-driver/mysql"

	"fingo/internal/boot"
)

// mariadbBin finds a MariaDB bin directory, or skips — the same gate the dbhost
// tests use, so a machine without a server runs the unit tests and says so.
func mariadbBin(t *testing.T) string {
	t.Helper()

	if dir := os.Getenv("FINGO_MARIADB_BIN"); dir != "" {
		if _, err := os.Stat(filepath.Join(dir, "mariadbd.exe")); err != nil {
			t.Fatalf("FINGO_MARIADB_BIN=%s holds no mariadbd.exe: %v", dir, err)
		}

		return dir
	}

	for _, dir := range []string{
		`C:\Program Files\MariaDB 12.3\bin`,
		`C:\Program Files\MariaDB 11.4\bin`,
	} {
		if _, err := os.Stat(filepath.Join(dir, "mariadbd.exe")); err == nil {
			return dir
		}
	}

	t.Skip("no MariaDB binaries found: set FINGO_MARIADB_BIN to run the provisioning test")

	return ""
}

func freePort(t *testing.T) int {
	t.Helper()

	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("find a free port: %v", err)
	}
	defer l.Close()

	return l.Addr().(*net.TCPAddr).Port
}

// TestLive_ProvisionsAnInstallationFromNothing — the objective's verification,
// at the level a test can reach it.
//
// A directory that does not exist, a server that is not running, a database that
// has never been created, a schema that has never been applied and a chart of
// accounts that has never been seeded — and one call that produces all five.
// Nothing is configured by hand: the port is chosen, the paths are derived, and
// the only input is an empty directory.
func TestLive_ProvisionsAnInstallationFromNothing(t *testing.T) {
	bin := mariadbBin(t)
	root := t.TempDir()

	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Minute)
	defer cancel()

	live, err := boot.NewLive(boot.LiveConfig{
		Root:   root,
		BinDir: bin,
		Port:   freePort(t),
	})
	if err != nil {
		t.Fatalf("NewLive: %v", err)
	}
	t.Cleanup(func() {
		if err := live.Close(ctx); err != nil {
			t.Errorf("Close: %v", err)
		}
	})

	start := time.Now()

	if err := boot.Ensure(ctx, live); err != nil {
		t.Fatalf("provisioning from nothing: %v", err)
	}

	t.Logf("provisioned from nothing in %s", time.Since(start).Round(time.Millisecond))

	// Everything the sequence promised is actually there.
	if !live.DataDirectoryExists() {
		t.Error("the data directory does not exist after provisioning")
	}

	version, err := live.SchemaVersion(ctx)
	if err != nil {
		t.Fatalf("SchemaVersion: %v", err)
	}
	if version == 0 {
		t.Error("the schema was not applied")
	}
	t.Logf("schema version %d", version)

	hasChart, err := live.ChartExists(ctx)
	if err != nil {
		t.Fatalf("ChartExists: %v", err)
	}
	if !hasChart {
		t.Fatal("the chart of accounts was not seeded")
	}

	// The DSN is what the application opens the books with, so it has to work.
	db, err := sql.Open("mysql", live.DSN())
	if err != nil {
		t.Fatalf("open the provisioned database: %v", err)
	}
	defer db.Close()

	var accounts int
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM accounts").Scan(&accounts); err != nil {
		t.Fatalf("count the chart of accounts: %v", err)
	}
	if accounts == 0 {
		t.Fatal("the provisioned database has no accounts")
	}
	t.Logf("chart of accounts: %d accounts", accounts)

	// Provisioning again must create nothing.
	//
	// This is the assertion that matters most: a launch that re-seeded the chart
	// or re-applied migrations is a launch that can destroy books it was
	// supposed to open, and the damage would not be visible until somebody read
	// a report.
	if err := boot.Ensure(ctx, live); err != nil {
		t.Fatalf("second Ensure: %v", err)
	}

	var after int
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM accounts").Scan(&after); err != nil {
		t.Fatalf("count the chart again: %v", err)
	}
	if after != accounts {
		t.Errorf("the chart went from %d accounts to %d: the second Ensure seeded it again", accounts, after)
	}

	second, err := live.SchemaVersion(ctx)
	if err != nil {
		t.Fatalf("SchemaVersion after the second Ensure: %v", err)
	}
	if second != version {
		t.Errorf("the schema moved from version %d to %d on a second Ensure", version, second)
	}
}

// TestLive_RefusesAnInstallationWithNowhereToKeepBooks — a root that cannot be
// created is a failure at the first step, not at the last.
func TestLive_RefusesAnInstallationWithNowhereToKeepBooks(t *testing.T) {
	bin := mariadbBin(t)

	// A root under a path that cannot exist as a directory.
	base := t.TempDir()
	bad := filepath.Join(base, "afile", "FinGo")

	if err := os.WriteFile(filepath.Join(base, "afile"), []byte("not a directory"), 0o644); err != nil {
		t.Fatalf("set up the test: %v", err)
	}

	live, err := boot.NewLive(boot.LiveConfig{Root: bad, BinDir: bin, Port: freePort(t)})
	if err != nil {
		return // Refused at construction, which is also fine.
	}
	t.Cleanup(func() { _ = live.Close(t.Context()) })

	if err := boot.Ensure(t.Context(), live); err == nil {
		t.Error("provisioning under a file succeeded, want a refusal")
	}
}

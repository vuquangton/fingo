package dbhost_test

import (
	"context"
	"database/sql"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	_ "github.com/go-sql-driver/mysql"

	"fingo/internal/dbhost"
)

// mariadbBin finds a MariaDB bin directory, or skips.
//
// This test spawns a real server, so it cannot run everywhere. It skips rather
// than fails, so that `go test ./...` stays honest on a machine without one.
// FINGO_MARIADB_BIN makes it required, the way REQUIRE_DB does for the
// repository's other database tests.
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
		`C:\Program Files\MariaDB 10.11\bin`,
	} {
		if _, err := os.Stat(filepath.Join(dir, "mariadbd.exe")); err == nil {
			return dir
		}
	}

	t.Skip("no MariaDB binaries found: set FINGO_MARIADB_BIN to run the bundled-server test")

	return ""
}

// freePort finds a port nobody is listening on.
//
// There is a race between closing this listener and the server binding it. That
// is acceptable in a test and not in production, where the installation picks a
// port once at first run and records it.
func freePort(t *testing.T) int {
	t.Helper()

	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("find a free port: %v", err)
	}
	defer l.Close()

	return l.Addr().(*net.TCPAddr).Port
}

// TestBundledServer_StartsFromNothingAndServes — slice 1d, and the verification
// that plan-native-shell.md §12.1 rests on.
//
// The whole storage decision is a guess until this passes: a data directory
// created from nothing, a private server started as a child process on its own
// port, reachable, queryable, and shut down again. If this does not work, the
// bundled-server plan reopens and everything downstream moves with it.
func TestBundledServer_StartsFromNothingAndServes(t *testing.T) {
	bin := mariadbBin(t)

	root := t.TempDir()
	layout := dbhost.NewLayout(root)
	port := freePort(t)

	ctx, cancel := context.WithTimeout(t.Context(), 4*time.Minute)
	defer cancel()

	// 1. Create a data directory from nothing.
	initStart := time.Now()

	if err := dbhost.Init(ctx, dbhost.InitConfig{
		InstallExe: filepath.Join(bin, "mariadb-install-db.exe"),
		Datadir:    layout.Data,
	}); err != nil {
		t.Fatalf("initialise a data directory: %v", err)
	}

	t.Logf("data directory initialised in %s", time.Since(initStart).Round(time.Millisecond))

	if _, err := os.Stat(layout.Data); err != nil {
		t.Fatalf("the data directory does not exist after Init: %v", err)
	}

	// 2. Start it as a private child process.
	sup := dbhost.NewSupervisor(&dbhost.ExecLauncher{})

	cfg := dbhost.Config{
		MysqldExe: filepath.Join(bin, "mariadbd.exe"),
		AdminExe:  filepath.Join(bin, "mariadb-admin.exe"),
		Datadir:   layout.Data,
		Port:      port,
		LogFile:   layout.Log,
		PIDFile:   layout.PID,
	}

	start := time.Now()

	if err := sup.EnsureStarted(ctx, cfg); err != nil {
		t.Fatalf("start the bundled server: %v", err)
	}
	t.Cleanup(func() {
		if sup.Running() {
			if err := sup.Stop(ctx); err != nil {
				t.Errorf("stop the bundled server: %v", err)
			}
		}
	})

	// 3. Wait until it can serve a query — not merely accept a connection.
	dsn := fmt.Sprintf("root:@tcp(127.0.0.1:%d)/?parseTime=true&loc=UTC", port)

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatalf("open a connection to the bundled server: %v", err)
	}
	defer db.Close()

	readyStart := time.Now()

	if err := dbhost.WaitReady(ctx, func(ctx context.Context) error {
		return db.PingContext(ctx)
	}, 500*time.Millisecond); err != nil {
		log, _ := os.ReadFile(layout.Log)
		t.Fatalf("the server never became ready: %v\nlog tail:\n%s", err, tail(string(log), 2000))
	}

	t.Logf("server ready in %s (start %s, readiness polling %s)",
		time.Since(start).Round(time.Millisecond),
		time.Since(start).Round(time.Millisecond),
		time.Since(readyStart).Round(time.Millisecond))

	// 4. It is really a server, and really ours.
	var version string
	if err := db.QueryRowContext(ctx, "SELECT VERSION()").Scan(&version); err != nil {
		t.Fatalf("query the bundled server: %v", err)
	}
	if version == "" {
		t.Fatal("the server reported no version")
	}
	t.Logf("bundled server reports: %s", version)

	// It must be listening on the loopback address only. A bundled server on
	// every interface is a database exposed to the office network.
	var bind string
	if err := db.QueryRowContext(ctx, "SELECT @@bind_address").Scan(&bind); err != nil {
		t.Errorf("read @@bind_address: %v", err)
	} else if bind != "127.0.0.1" {
		t.Errorf("the server is bound to %q, want 127.0.0.1 only", bind)
	}

	// And it must be on the port we chose, not the machine's default.
	var gotPort int
	if err := db.QueryRowContext(ctx, "SELECT @@port").Scan(&gotPort); err != nil {
		t.Errorf("read @@port: %v", err)
	} else if gotPort != port {
		t.Errorf("the server is on port %d, want %d", gotPort, port)
	}

	// 5. Clean shutdown through the supervisor.
	if err := sup.Stop(ctx); err != nil {
		t.Errorf("stop: %v", err)
	}
	if sup.Running() {
		t.Error("the supervisor still reports the server running after Stop")
	}

	// A stopped server must actually be gone, or the next launch finds the data
	// directory locked.
	deadline := time.Now().Add(10 * time.Second)
	for {
		if err := db.PingContext(ctx); err != nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("the server is still answering after Stop")
		}

		time.Sleep(200 * time.Millisecond)
	}
}

func tail(s string, n int) string {
	if len(s) <= n {
		return s
	}

	return "…" + s[len(s)-n:]
}

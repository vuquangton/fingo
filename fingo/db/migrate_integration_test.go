package db_test

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	_ "github.com/go-sql-driver/mysql"

	"fingo/db"
	"fingo/internal/adapter/mariadb/conn"
)

// scratchPrefix guards the DROP: this test may only drop databases it named
// itself.
const scratchPrefix = "fingo_migrate_test_"

// loadConfig reads the nearest .env by walking up from the test's directory.
func loadConfig(t *testing.T) conn.Config {
	t.Helper()

	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}

	for i := 0; i < 4; i++ {
		path := filepath.Join(dir, ".env")
		if f, err := os.Open(path); err == nil {
			defer f.Close()
			env, err := conn.ParseDotEnv(f)
			if err != nil {
				t.Fatalf("parse %s: %v", path, err)
			}
			cfg, err := conn.FromEnv(func(k string) string { return env[k] })
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

// TestMigrationsApplyToRealMariaDB proves the DDL actually runs.
//
// It creates a scratch database, applies every migration to it, checks the
// resulting shape of voucher_lines, and drops the database again. Migration
// 00002 had never been executed before this test existed — sqlc parsing a file
// is not the same as MariaDB accepting it.
func TestMigrationsApplyToRealMariaDB(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test skipped in short mode")
	}

	cfg := loadConfig(t)

	admin, err := sql.Open("mysql", cfg.DSN(""))
	if err != nil {
		t.Fatalf("open admin connection: %v", err)
	}
	// Registered before the drop so that, by t.Cleanup's LIFO order, it runs
	// after it. A deferred Close would fire first and the drop would fail on a
	// closed connection.
	t.Cleanup(func() { admin.Close() })

	if err := admin.Ping(); err != nil {
		t.Skipf("MariaDB not reachable at %s:%s: %v", cfg.Host, cfg.Port, err)
	}

	sweepScratch(t, admin)

	name := fmt.Sprintf("%s%d", scratchPrefix, time.Now().UnixNano())
	if !strings.HasPrefix(name, scratchPrefix) {
		t.Fatalf("refusing to create database %q: outside the scratch prefix", name)
	}

	if _, err := admin.Exec(fmt.Sprintf(
		"CREATE DATABASE `%s` CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci", name)); err != nil {
		t.Fatalf("create scratch database %s: %v", name, err)
	}

	t.Cleanup(func() {
		if !strings.HasPrefix(name, scratchPrefix) {
			t.Errorf("refusing to drop %q: outside the scratch prefix", name)
			return
		}
		if _, err := admin.Exec(fmt.Sprintf("DROP DATABASE IF EXISTS `%s`", name)); err != nil {
			t.Errorf("drop scratch database %s: %v", name, err)
		}
	})

	client, err := sql.Open("mysql", cfg.DSN(name))
	if err != nil {
		t.Fatalf("open client connection: %v", err)
	}
	t.Cleanup(func() { client.Close() })

	if err := db.Up(client); err != nil {
		t.Fatalf("migrations failed against %s: %v", name, err)
	}

	// Every migration must be recorded as applied.
	var applied int
	if err := client.QueryRow("SELECT COUNT(*) FROM goose_db_version WHERE is_applied = 1").Scan(&applied); err != nil {
		t.Fatalf("read goose_db_version: %v", err)
	}
	if applied < 2 {
		t.Fatalf("applied migrations = %d, want at least 2", applied)
	}

	cols := columnsOf(t, client, name, "voucher_lines")

	// The reshaped line: one account, one side, one whole-đồng amount.
	for _, want := range []string{"account_id", "side", "amount"} {
		if _, ok := cols[want]; !ok {
			t.Errorf("voucher_lines is missing %q; columns present: %v", want, keysOf(cols))
		}
	}

	// The pair shape must be gone.
	for _, gone := range []string{"debit_account_id", "credit_account_id"} {
		if _, ok := cols[gone]; ok {
			t.Errorf("voucher_lines still has the old pair column %q", gone)
		}
	}

	if got := cols["side"]; !strings.Contains(strings.ToLower(got), "enum") {
		t.Errorf("side is %q, want an ENUM", got)
	}
	// information_schema reports the full type, e.g. "decimal(19,0)".
	if got := cols["amount"]; !strings.HasPrefix(strings.ToLower(got), "decimal") {
		t.Errorf("amount is %q, want a decimal type", got)
	}

	// VND has no subunit: the column must not carry a fractional scale.
	var scale int
	if err := client.QueryRow(`
		SELECT numeric_scale FROM information_schema.columns
		WHERE table_schema = ? AND table_name = 'voucher_lines' AND column_name = 'amount'`,
		name).Scan(&scale); err != nil {
		t.Fatalf("read amount scale: %v", err)
	}
	if scale != 0 {
		t.Errorf("amount scale = %d, want 0 — VND has no subunit", scale)
	}
}

// sweepScratch drops scratch databases left behind by an earlier failed run.
//
// The prefix test is done in Go, not by a SQL LIKE pattern: '_' is a LIKE
// wildcard, so "fingo_migrate_test_%" would also match names this test must
// never touch.
func sweepScratch(t *testing.T, sqlDB *sql.DB) {
	t.Helper()

	rows, err := sqlDB.Query("SELECT schema_name FROM information_schema.schemata")
	if err != nil {
		t.Fatalf("list databases: %v", err)
	}

	var all []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatalf("scan schema name: %v", err)
		}
		all = append(all, name)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate schemata: %v", err)
	}
	rows.Close()

	for _, name := range all {
		if !strings.HasPrefix(name, scratchPrefix) {
			continue
		}
		if _, err := sqlDB.Exec(fmt.Sprintf("DROP DATABASE IF EXISTS `%s`", name)); err != nil {
			t.Fatalf("drop stale scratch database %s: %v", name, err)
		}
		t.Logf("dropped stale scratch database %s", name)
	}
}

func columnsOf(t *testing.T, sqlDB *sql.DB, schema, table string) map[string]string {

	rows, err := sqlDB.Query(`
		SELECT column_name, column_type FROM information_schema.columns
		WHERE table_schema = ? AND table_name = ?`, schema, table)
	if err != nil {
		t.Fatalf("read columns of %s: %v", table, err)
	}
	defer rows.Close()

	out := make(map[string]string)
	for rows.Next() {
		var name, typ string
		if err := rows.Scan(&name, &typ); err != nil {
			t.Fatalf("scan column row: %v", err)
		}
		out[name] = typ
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate columns: %v", err)
	}
	return out
}

func keysOf(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

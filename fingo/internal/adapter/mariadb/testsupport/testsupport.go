// Package testsupport provides helpers for tests that need a real MariaDB.
//
// This is test-only code. Nothing outside a _test.go file should import it.
package testsupport

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/google/uuid"

	"fingo/db"
	"fingo/internal/adapter/mariadb/conn"
	mariadb "fingo/internal/adapter/mariadb/sqlc"
	"fingo/internal/domain/gl"
)

// TestPartnerID is the partner SeedPartner creates, so a test that records a
// credit sale can name the customer that owes it.
const TestPartnerID = "partner-test-1"

// SeedPartner inserts a customer and returns its id.
//
// A credit sale is owed by somebody, and voucher_lines.partner_id is a foreign
// key, so a test that records a sale needs a real partner row.
func SeedPartner(t *testing.T, db *sql.DB) string {
	t.Helper()

	if _, err := db.Exec(`
		INSERT INTO partners (id, code, name, kind, is_active)
		VALUES (?, ?, ?, 'CUSTOMER', TRUE)`,
		TestPartnerID, "KH001", "Khách hàng thử nghiệm"); err != nil {
		t.Fatalf("seed partner %s: %v", TestPartnerID, err)
	}

	return TestPartnerID
}

// SeedChart inserts the whole TT133 chart of accounts, parents before children.
//
// It uses the real chart rather than a handful of hand-written rows so that a
// join from a voucher line back to an account code is exercised for real.
func SeedChart(t *testing.T, db *sql.DB) {
	t.Helper()

	ctx := context.Background()
	q := mariadb.New(db)
	ids := make(map[string]string)

	for _, a := range gl.TT133Chart() {
		id := uuid.NewString()

		parent := sql.NullString{}
		if a.ParentCode != "" {
			parentID, ok := ids[a.ParentCode]
			if !ok {
				t.Fatalf("chart lists %s before its parent %s", a.Code, a.ParentCode)
			}
			parent = sql.NullString{String: parentID, Valid: true}
		}

		if err := q.CreateAccount(ctx, mariadb.CreateAccountParams{
			ID:          id,
			Code:        a.Code,
			Name:        a.Name,
			ParentID:    parent,
			AccountType: mariadb.AccountsAccountType(a.Type),
			Nature:      mariadb.AccountsNature(a.Nature),
			IsActive:    true,
		}); err != nil {
			t.Fatalf("seed account %s: %v", a.Code, err)
		}

		ids[a.Code] = id
	}
}

// scratchPrefix guards every DROP. A test may only drop databases it named.
const scratchPrefix = "fingo_test_"

// RequireEnv makes a missing database a failure rather than a skip.
//
// With it set, a test that cannot reach its database stops the run. Without it,
// the test is skipped — which is the right default on a laptop and the wrong one
// anywhere the result is being trusted.
//
// It exists because of what .env being gitignored does to this suite. Sixty-plus
// database tests sit behind two skips, so a machine without a .env reports
// `ok` for every adapter package having run none of them, and nothing in the
// output says so. CI sets this, and a green run there means the tests ran.
const RequireEnv = "REQUIRE_DB"

// Skipped counts how many database tests this process has skipped.
//
// Exported so a package can report it — a silent skip is the failure mode this
// whole file exists to prevent, and a count that nobody prints is another silent
// skip. Read it from a TestMain after m.Run.
var Skipped int64

// unavailable is the one place that decides between skipping and failing.
//
// Every gate below goes through it, so there is no second opinion about what a
// missing database means. A gate that called t.Skip directly would be a gate
// that Phase 0 of docs/testing-strategy.md never reached.
func unavailable(t *testing.T, format string, args ...any) {
	t.Helper()

	if os.Getenv(RequireEnv) == "1" {
		t.Fatalf("%s=%s, so a missing database is a failure: %s",
			RequireEnv, "1", fmt.Sprintf(format, args...))
	}

	atomic.AddInt64(&Skipped, 1)
	t.Skipf(format, args...)
}

// Config loads the nearest .env, walking up from the working directory. The
// caller is skipped when none is found — or fails, when REQUIRE_DB is set.
func Config(t *testing.T) conn.Config {
	t.Helper()

	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}

	for i := 0; i < 5; i++ {
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

	unavailable(t, "no .env found walking up from the working directory: database-backed tests cannot run (write one, or set %s=1 to make this a failure)", RequireEnv)

	return conn.Config{}
}

// Admin opens a connection to the server without selecting a database, and
// skips the test when the server cannot be reached — or fails, when REQUIRE_DB
// is set.
func Admin(t *testing.T, cfg conn.Config) *sql.DB {
	t.Helper()

	sqlDB, err := sql.Open("mysql", cfg.DSN(""))
	if err != nil {
		t.Fatalf("open admin connection: %v", err)
	}
	t.Cleanup(func() { sqlDB.Close() })

	if err := sqlDB.Ping(); err != nil {
		unavailable(t, "MariaDB not reachable at %s:%s: %v", cfg.Host, cfg.Port, err)
	}

	return sqlDB
}

// EmptyDatabase creates a uniquely named database with no schema, and returns a
// connection to it together with its name.
//
// Empty on purpose: a test that opens the database itself must be able to prove
// that opening it applies the migrations.
func EmptyDatabase(t *testing.T, cfg conn.Config, admin *sql.DB) (*sql.DB, string) {
	t.Helper()

	name := fmt.Sprintf("%s%d", scratchPrefix, time.Now().UnixNano())
	if !strings.HasPrefix(name, scratchPrefix) {
		t.Fatalf("refusing to create %q: outside the scratch prefix", name)
	}

	if _, err := admin.Exec(fmt.Sprintf(
		"CREATE DATABASE `%s` CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci", name)); err != nil {
		t.Fatalf("create scratch database %s: %v", name, err)
	}

	// Registered before the connection cleanup, so by t.Cleanup's LIFO order
	// the drop runs first — while the connection is still open.
	t.Cleanup(func() {
		if !strings.HasPrefix(name, scratchPrefix) {
			t.Errorf("refusing to drop %q: outside the scratch prefix", name)
			return
		}
		if _, err := admin.Exec(fmt.Sprintf("DROP DATABASE IF EXISTS `%s`", name)); err != nil {
			t.Errorf("drop scratch database %s: %v", name, err)
		}
	})

	sqlDB, err := sql.Open("mysql", cfg.DSN(name))
	if err != nil {
		t.Fatalf("open %s: %v", name, err)
	}
	t.Cleanup(func() { sqlDB.Close() })

	return sqlDB, name
}

// MigratedDatabase creates a uniquely named database, applies every migration,
// and returns a connection together with the database name.
func MigratedDatabase(t *testing.T, cfg conn.Config, admin *sql.DB) (*sql.DB, string) {
	t.Helper()

	sqlDB, name := EmptyDatabase(t, cfg, admin)

	if err := db.Up(sqlDB); err != nil {
		t.Fatalf("migrate %s: %v", name, err)
	}

	return sqlDB, name
}

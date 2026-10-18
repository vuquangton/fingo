// Package testdb gives the suite a real PostgreSQL database or refuses to run.
//
// The failure this package exists to prevent is a green suite that proves
// nothing: a database test that skips reports "ok". When REQUIRE_DB=1 a missing
// database is a failure, and the message says what to run.
package testdb

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// RequireEnv turns a missing database into a failure.
const RequireEnv = "REQUIRE_DB"

// Config is the connection the suite uses.
type Config struct {
	Host     string
	Port     string
	User     string
	Password string
	Database string
}

// DSN builds a keyword/value connection string.
func (c Config) DSN() string {
	return fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		c.Host, c.Port, c.User, c.Password, c.Database)
}

// Load reads the connection from the environment, falling back to the
// repository's gitignored .env. It fails the test when REQUIRE_DB=1 and nothing
// is configured, and skips otherwise.
func Load(t *testing.T) Config {
	t.Helper()

	cfg := Config{
		Host:     firstNonEmpty(os.Getenv("PGHOST"), "127.0.0.1"),
		Port:     firstNonEmpty(os.Getenv("PGPORT"), "5432"),
		User:     firstNonEmpty(os.Getenv("PGUSER"), "fingo_app"),
		Password: os.Getenv("PGPASSWORD"),
		Database: firstNonEmpty(os.Getenv("FINGO_TEST_DATABASE"), "fingo_test"),
	}

	if cfg.Password == "" {
		if env, ok := readDotEnv(); ok {
			cfg.Host = firstNonEmpty(cfg.Host, env["PGHOST"])
			if v := env["PGPORT"]; v != "" && os.Getenv("PGPORT") == "" {
				cfg.Port = v
			}
			if v := env["PGUSER"]; v != "" && os.Getenv("PGUSER") == "" {
				cfg.User = v
			}
			cfg.Password = env["PGPASSWORD"]
			if v := env["FINGO_TEST_DATABASE"]; v != "" {
				cfg.Database = v
			}
		}
	}

	if cfg.Password == "" {
		unavailable(t, "no PostgreSQL credentials: set PGPASSWORD or create .env with scripts/pgsetup.cmd")
	}

	return cfg
}

// Open connects as the application role. It is the only role the suite should
// use: a superuser bypasses row-level security, so a test that runs as one
// cannot observe the boundary it claims to test.
func Open(t *testing.T, cfg Config) *pgxpool.Pool {
	t.Helper()

	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, cfg.DSN())
	if err != nil {
		unavailable(t, "open %s: %v", cfg.Database, err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		unavailable(t, "connect to %s at %s:%s as %s: %v", cfg.Database, cfg.Host, cfg.Port, cfg.User, err)
	}

	t.Cleanup(pool.Close)

	if err := checkSchema(ctx, pool); err != nil {
		pool.Close()
		t.Fatalf("%v\n\nApply the schema first:  $env:PGADMIN_PASSWORD='...'; go run ./cmd/migrate -database fingo_test", err)
	}

	return pool
}

// checkSchema proves the migration ran, so a test never fails later with a
// confusing "relation does not exist".
func checkSchema(ctx context.Context, pool *pgxpool.Pool) error {
	var version int64
	err := pool.QueryRow(ctx,
		`SELECT max(version_id) FROM goose_db_version WHERE is_applied`).Scan(&version)
	if err != nil {
		return fmt.Errorf("testdb: the test database has no schema (%v)", err)
	}
	if version < 1 {
		return fmt.Errorf("testdb: the test database has no migrations applied")
	}
	return nil
}

// unavailable fails when REQUIRE_DB=1, skips otherwise.
func unavailable(t *testing.T, format string, args ...any) {
	t.Helper()

	if os.Getenv(RequireEnv) == "1" {
		t.Fatalf("%s=1, so a missing database is a failure: %s", RequireEnv, fmt.Sprintf(format, args...))
	}
	t.Skipf("%s", fmt.Sprintf(format, args...))
}

// readDotEnv looks for the repository's .env, walking up from the test's
// working directory (a package directory under internal/).
func readDotEnv() (map[string]string, bool) {
	dir, err := os.Getwd()
	if err != nil {
		return nil, false
	}

	for range 6 {
		path := filepath.Join(dir, ".env")
		if f, err := os.Open(path); err == nil {
			defer func() { _ = f.Close() }()

			values := map[string]string{}
			scanner := bufio.NewScanner(f)
			for scanner.Scan() {
				line := strings.TrimSpace(scanner.Text())
				if line == "" || strings.HasPrefix(line, "#") {
					continue
				}
				key, value, ok := strings.Cut(line, "=")
				if !ok {
					continue
				}
				values[strings.TrimSpace(key)] = strings.TrimSpace(value)
			}
			return values, true
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return nil, false
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

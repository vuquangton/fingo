package conn_test

import (
	"strings"
	"testing"

	"github.com/go-sql-driver/mysql"

	"fingo/internal/adapter/mariadb/conn"
)

func testConfig() conn.Config {
	return conn.Config{
		Host:     "127.0.0.1",
		Port:     "3306",
		User:     "dev",
		Password: "p@ss:word/1?#",
		Params:   map[string]string{"charset": "utf8mb4", "parseTime": "True"},
	}
}

// The DSN is asserted by parsing it back with the driver itself, rather than by
// comparing against a hand-built string. That makes the test independent of the
// driver's formatting, and proves escaping works for a hostile password.
func TestConfig_DSNRoundTripsThroughDriver(t *testing.T) {
	got := testConfig().DSN("client_acme")

	parsed, err := mysql.ParseDSN(got)
	if err != nil {
		t.Fatalf("driver cannot parse the DSN it was given (%q): %v", got, err)
	}

	if parsed.DBName != "client_acme" {
		t.Fatalf("DBName = %q, want %q", parsed.DBName, "client_acme")
	}
	if parsed.User != "dev" {
		t.Fatalf("User = %q, want %q", parsed.User, "dev")
	}
	if parsed.Passwd != "p@ss:word/1?#" {
		t.Fatalf("Passwd = %q, want the original — the password was not escaped correctly", parsed.Passwd)
	}
	if parsed.Addr != "127.0.0.1:3306" {
		t.Fatalf("Addr = %q, want %q", parsed.Addr, "127.0.0.1:3306")
	}
	if !parsed.ParseTime {
		t.Fatal("parseTime must survive: DATE and DATETIME columns must come back as time.Time")
	}

	// charset is applied by the driver at connect time and is NOT surfaced back
	// on parse (Params comes back nil, Collation empty). Assert it reached the
	// DSN string instead, or Vietnamese text would be mangled at the server.
	if !strings.Contains(got, "charset=utf8mb4") {
		t.Fatalf("DSN %q is missing charset=utf8mb4", got)
	}
}

// The per-client model depends on this: swapping the database name must change
// nothing else about how the server is reached.
func TestConfig_DSNIsPerClientAndNothingElse(t *testing.T) {
	cfg := testConfig()

	a, err := mysql.ParseDSN(cfg.DSN("client_a"))
	if err != nil {
		t.Fatalf("parse client_a: %v", err)
	}
	b, err := mysql.ParseDSN(cfg.DSN("client_b"))
	if err != nil {
		t.Fatalf("parse client_b: %v", err)
	}

	if a.DBName == b.DBName {
		t.Fatal("two client databases produced the same DBName")
	}
	if a.User != b.User || a.Passwd != b.Passwd || a.Addr != b.Addr {
		t.Fatal("swapping the client database changed the connection identity, not just the database")
	}
}

func envLookup(env map[string]string) func(string) string {
	return func(k string) string { return env[k] }
}

func TestFromEnv_ReadsConnectionSettings(t *testing.T) {
	cfg, err := conn.FromEnv(envLookup(map[string]string{
		"DB_HOST":     "127.0.0.1",
		"DB_PORT":     "3306",
		"DB_USER":     "dev",
		"DB_PASSWORD": "secret",
		"DB_PARAMS":   "charset=utf8mb4&parseTime=True",
	}))
	if err != nil {
		t.Fatalf("FromEnv failed on a complete environment: %v", err)
	}

	if cfg.Host != "127.0.0.1" || cfg.Port != "3306" || cfg.User != "dev" || cfg.Password != "secret" {
		t.Fatalf("unexpected config: %+v", cfg)
	}
	if cfg.Params["charset"] != "utf8mb4" {
		t.Fatalf("charset = %q, want utf8mb4", cfg.Params["charset"])
	}
	if cfg.Params["parseTime"] != "True" {
		t.Fatalf("parseTime = %q, want True", cfg.Params["parseTime"])
	}
}

// A host with no port would build an unusable address ("host:"), so the
// conventional MariaDB port is assumed.
func TestFromEnv_DefaultsThePort(t *testing.T) {
	cfg, err := conn.FromEnv(envLookup(map[string]string{
		"DB_HOST": "127.0.0.1",
		"DB_USER": "dev",
	}))
	if err != nil {
		t.Fatalf("FromEnv failed: %v", err)
	}
	if cfg.Port != "3306" {
		t.Fatalf("Port = %q, want 3306", cfg.Port)
	}
}

func TestFromEnv_RejectsMissingRequiredSettings(t *testing.T) {
	cases := map[string]map[string]string{
		"no host": {"DB_USER": "dev"},
		"no user": {"DB_HOST": "127.0.0.1"},
	}

	for name, env := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := conn.FromEnv(envLookup(env)); err == nil {
				t.Fatalf("%s must be rejected, got nil", name)
			}
		})
	}
}

// A malformed parameter would be silently dropped otherwise, and a missing
// parseTime changes how every date is read.
func TestFromEnv_RejectsMalformedParams(t *testing.T) {
	_, err := conn.FromEnv(envLookup(map[string]string{
		"DB_HOST":   "127.0.0.1",
		"DB_USER":   "dev",
		"DB_PARAMS": "charset=utf8mb4&parseTime",
	}))
	if err == nil {
		t.Fatal("a parameter with no '=' must be rejected, got nil")
	}
}

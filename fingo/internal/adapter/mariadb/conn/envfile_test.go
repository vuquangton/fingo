package conn_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"fingo/internal/adapter/mariadb/conn"
)

func writeEnv(t *testing.T, body string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), ".env")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	return path
}

func TestFromEnvFile_ReadsConnectionAndClientDatabase(t *testing.T) {
	path := writeEnv(t, "# FinGo local environment\n"+
		"DB_HOST=127.0.0.1\n"+
		"DB_USER=dev\n"+
		"DB_PASSWORD=secret\n"+
		"DB_PARAMS=charset=utf8mb4&parseTime=True\n"+
		"DB_NAME=fingo\n")

	cfg, database, err := conn.FromEnvFile(path)
	if err != nil {
		t.Fatalf("FromEnvFile: %v", err)
	}

	if cfg.Host != "127.0.0.1" || cfg.User != "dev" || cfg.Password != "secret" {
		t.Fatalf("unexpected config: %+v", cfg)
	}
	if cfg.Port != conn.DefaultPort {
		t.Fatalf("Port = %q, want the default %q", cfg.Port, conn.DefaultPort)
	}
	if cfg.Params["parseTime"] != "True" {
		t.Fatalf("parseTime = %q, want True", cfg.Params["parseTime"])
	}
	if database != "fingo" {
		t.Fatalf("database = %q, want %q", database, "fingo")
	}
}

// Without a database name there is no client to open, so the failure must be
// here rather than as a confusing connection error later.
func TestFromEnvFile_RequiresAClientDatabase(t *testing.T) {
	path := writeEnv(t, "DB_HOST=127.0.0.1\nDB_USER=dev\n")

	_, _, err := conn.FromEnvFile(path)
	if err == nil {
		t.Fatal("a missing DB_NAME must be an error, got nil")
	}
	if !strings.Contains(err.Error(), "DB_NAME") {
		t.Fatalf("error should name the missing setting, got: %v", err)
	}
}

func TestFromEnvFile_ReportsAMissingFile(t *testing.T) {
	if _, _, err := conn.FromEnvFile(filepath.Join(t.TempDir(), "absent.env")); err == nil {
		t.Fatal("a missing file must be an error, got nil")
	}
}

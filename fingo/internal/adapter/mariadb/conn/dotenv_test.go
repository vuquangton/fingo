package conn_test

import (
	"strings"
	"testing"

	"fingo/internal/adapter/mariadb/conn"
)

func TestParseDotEnv_ReadsKeysAndSkipsNoise(t *testing.T) {
	content := `# FinGo Local Environment Configuration
DB_HOST=127.0.0.1
DB_PORT=3306
DB_USER=dev
DB_PASSWORD=123456
DB_PARAMS=charset=utf8mb4&parseTime=True&loc=Local

# a trailing comment

DB_NAME=fingo
`

	env, err := conn.ParseDotEnv(strings.NewReader(content))
	if err != nil {
		t.Fatalf("ParseDotEnv failed: %v", err)
	}

	want := map[string]string{
		"DB_HOST":     "127.0.0.1",
		"DB_PORT":     "3306",
		"DB_USER":     "dev",
		"DB_PASSWORD": "123456",
		"DB_PARAMS":   "charset=utf8mb4&parseTime=True&loc=Local",
		"DB_NAME":     "fingo",
	}
	for k, v := range want {
		if env[k] != v {
			t.Fatalf("env[%s] = %q, want %q", k, env[k], v)
		}
	}
}

// An '&' in a value is normal for DB_PARAMS and must not be treated as a
// comment or split.
func TestParseDotEnv_KeepsSeparatorsInsideValues(t *testing.T) {
	env, err := conn.ParseDotEnv(strings.NewReader("DB_PARAMS=charset=utf8mb4&parseTime=True\n"))
	if err != nil {
		t.Fatalf("ParseDotEnv failed: %v", err)
	}
	if got, want := env["DB_PARAMS"], "charset=utf8mb4&parseTime=True"; got != want {
		t.Fatalf("DB_PARAMS = %q, want %q", got, want)
	}
}

// Values are commonly quoted; the quotes are not part of the value.
func TestParseDotEnv_StripsSurroundingQuotes(t *testing.T) {
	env, err := conn.ParseDotEnv(strings.NewReader(`DB_PASSWORD="p@ss word"` + "\n" + `DB_USER='dev'` + "\n"))
	if err != nil {
		t.Fatalf("ParseDotEnv failed: %v", err)
	}
	if got, want := env["DB_PASSWORD"], "p@ss word"; got != want {
		t.Fatalf("DB_PASSWORD = %q, want %q", got, want)
	}
	if got, want := env["DB_USER"], "dev"; got != want {
		t.Fatalf("DB_USER = %q, want %q", got, want)
	}
}

// A typo'd line must surface rather than be silently dropped — a lost
// parseTime quietly changes how every date is read.
func TestParseDotEnv_RejectsMalformedLines(t *testing.T) {
	_, err := conn.ParseDotEnv(strings.NewReader("DB_HOST=127.0.0.1\nTHIS_IS_NOT_A_PAIR\n"))
	if err == nil {
		t.Fatal("a line with no '=' must be rejected, got nil")
	}
}

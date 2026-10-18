package appconfig_test

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/go-sql-driver/mysql"

	"fingo/internal/appconfig"
)

func TestNewConnection_FillsInWhatTheOperatorLeftOut(t *testing.T) {
	c, err := appconfig.NewConnection("  db.internal  ", "", "  dev  ", " pw ", " fingo ", "")
	if err != nil {
		t.Fatalf("NewConnection: %v", err)
	}

	if c.Server.Host != "db.internal" {
		t.Errorf("host = %q, want it trimmed", c.Server.Host)
	}
	if c.Server.User != "dev" {
		t.Errorf("user = %q, want it trimmed", c.Server.User)
	}
	if c.Server.Port != "3306" {
		t.Errorf("port = %q, want the default", c.Server.Port)
	}
	if c.Database != "fingo" {
		t.Errorf("database = %q, want it trimmed", c.Database)
	}
	// The password is the one field never trimmed: a leading or trailing space
	// may be part of it, and a login that fails on a space nobody can see is a
	// support call nobody can answer.
	if c.Server.Password != " pw " {
		t.Errorf("password = %q, want it left exactly as typed", c.Server.Password)
	}
	if c.Params() != "charset=utf8mb4&loc=UTC&parseTime=True" {
		t.Errorf("params = %q, want the defaults in a stable order", c.Params())
	}
}

func TestNewConnection_RefusesWhatMakesAnAttemptMeaningless(t *testing.T) {
	cases := map[string][6]string{
		"no host":     {"", "3306", "dev", "pw", "fingo", ""},
		"no user":     {"h", "3306", "", "pw", "fingo", ""},
		"no database": {"h", "3306", "dev", "pw", "", ""},
	}

	for name, args := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := appconfig.NewConnection(args[0], args[1], args[2], args[3], args[4], args[5])
			if err == nil {
				t.Fatal("a connection missing a required field was accepted")
			}
		})
	}
}

func TestNewConnection_RefusesMalformedParameters(t *testing.T) {
	// A parameter with no value would silently drop parseTime, and losing
	// parseTime changes how every date column is read.
	_, err := appconfig.NewConnection("h", "3306", "dev", "pw", "fingo", "parseTime")
	if err == nil {
		t.Fatal("a parameter with no value was accepted")
	}
}

// The DSN is built by the driver's formatter, and it has to survive the driver's
// own parser. Anything else is a password that works for the person who chose it
// and fails for the person who has an '@' in theirs.
func TestConnection_DSNSurvivesTheDriversOwnParser(t *testing.T) {
	passwords := []string{
		"simple",
		"with@sign",
		"with:colon",
		"with/slash",
		"with@and:both",
		`with\backslash`,
		"",
	}

	for _, password := range passwords {
		t.Run("password "+password, func(t *testing.T) {
			c, err := appconfig.NewConnection("127.0.0.1", "3307", "dev", password, "fingo_test", "")
			if err != nil {
				t.Fatalf("NewConnection: %v", err)
			}

			parsed, err := mysql.ParseDSN(c.DSN())
			if err != nil {
				t.Fatalf("the driver cannot parse the DSN it was given: %v (dsn=%q)", err, c.DSN())
			}

			if parsed.Passwd != password {
				t.Errorf("password round-tripped as %q, want %q", parsed.Passwd, password)
			}
			if parsed.DBName != "fingo_test" {
				t.Errorf("database round-tripped as %q", parsed.DBName)
			}
			if parsed.Addr != "127.0.0.1:3307" {
				t.Errorf("address round-tripped as %q", parsed.Addr)
			}
			if parsed.User != "dev" {
				t.Errorf("user round-tripped as %q", parsed.User)
			}
		})
	}
}

func TestStore_LoadsNothingFromAnEmptyInstallation(t *testing.T) {
	store := appconfig.NewStoreAt(filepath.Join(t.TempDir(), "FinGo", "config.json"))

	if store.Exists() {
		t.Fatal("a store with no file reports that it exists")
	}

	_, err := store.Load()
	if !errors.Is(err, appconfig.ErrNotConfigured) {
		t.Fatalf("Load on an empty installation = %v, want ErrNotConfigured", err)
	}
}

func TestStore_SavesAndLoadsTheConnection(t *testing.T) {
	path := filepath.Join(t.TempDir(), "FinGo", "config.json")
	store := appconfig.NewStoreAt(path)

	want, err := appconfig.NewConnection("db.internal", "3307", "dev", "pw", "fingo", "")
	if err != nil {
		t.Fatalf("NewConnection: %v", err)
	}

	if err := store.Save(appconfig.Settings{Connection: want}); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if !store.Exists() {
		t.Fatal("the store does not exist after saving")
	}

	got, err := store.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got.Connection.DSN() != want.DSN() {
		t.Errorf("round trip changed the connection:\n got %s\nwant %s", got.Connection.DSN(), want.DSN())
	}
	if got.SavedAt.IsZero() {
		t.Error("SavedAt was not recorded; nobody can tell when the settings changed")
	}
}

// The file holds a password in the clear. On Unix the mode is what keeps it
// from other accounts; on Windows os.WriteFile does not set an ACL, the file
// inherits the profile directory's, and there is nothing here to assert.
func TestStore_WritesTheFileForTheOwnerOnly(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("mode bits are not the Windows story; the inherited profile ACL is")
	}

	path := filepath.Join(t.TempDir(), "FinGo", "config.json")
	store := appconfig.NewStoreAt(path)

	c, err := appconfig.NewConnection("h", "3306", "dev", "pw", "fingo", "")
	if err != nil {
		t.Fatalf("NewConnection: %v", err)
	}
	if err := store.Save(appconfig.Settings{Connection: c}); err != nil {
		t.Fatalf("Save: %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf("the settings file is %o, want 600", perm)
	}
}

// A settings file somebody meant something by is never silently ignored. Falling
// back to a .env because the real settings were unreadable would open a
// different database than the one they configured.
func TestStore_RefusesAFileItCannotUnderstand(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")

	if err := os.WriteFile(path, []byte("{ not json"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}

	store := appconfig.NewStoreAt(path)

	_, err := store.Load()
	if err == nil {
		t.Fatal("unreadable settings were accepted")
	}
	if errors.Is(err, appconfig.ErrNotConfigured) {
		t.Fatalf("unreadable settings were reported as merely absent: %v", err)
	}

	// And the message says which file, because that is the thing to go and fix.
	if !strings.Contains(err.Error(), path) {
		t.Errorf("the error does not name the file: %v", err)
	}
}

// The company the books belong to survives a save and a load.
//
// It lives here rather than in the client's own database for now, and that is a
// deliberate P0 compromise: the profile belongs with the books, and moving it
// there is a schema change with a migration behind it. What P0 needs is that a
// first run collects it once and does not ask again.
func TestStore_SavesAndLoadsTheCompany(t *testing.T) {
	path := filepath.Join(t.TempDir(), "FinGo", "config.json")
	store := appconfig.NewStoreAt(path)

	c, err := appconfig.NewConnection("127.0.0.1", "3306", "root", "", "fingo", "")
	if err != nil {
		t.Fatalf("NewConnection: %v", err)
	}

	want := appconfig.Company{
		Name:    "Công ty TNHH Bình Minh",
		TaxCode: "0123456789-001",
	}

	if err := store.Save(appconfig.Settings{Connection: c, Company: want}); err != nil {
		t.Fatalf("Save: %v", err)
	}

	loaded, err := store.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if loaded.Company != want {
		t.Errorf("Company = %+v, want %+v", loaded.Company, want)
	}
}

// A settings file written before there was a company still loads.
//
// This is the reason the field is a value and not a pointer: an installation
// upgraded from an earlier build has no company in its file, and refusing to
// read it would make the upgrade look like corruption. An empty company is the
// honest reading of a file that does not name one — and the first run will ask.
func TestStore_LoadsAFileFromBeforeThereWasACompany(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")

	c, err := appconfig.NewConnection("127.0.0.1", "3306", "root", "", "fingo", "")
	if err != nil {
		t.Fatalf("NewConnection: %v", err)
	}

	// Written as the earlier build would have written it: the real connection,
	// and no company key at all. Marshalled rather than hand-written, so the
	// test is about a missing field and not about whether I typed the file
	// format correctly.
	older, err := json.Marshal(map[string]any{
		"connection": c,
		"saved_at":   time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("marshal the older settings: %v", err)
	}

	if err := os.WriteFile(path, older, 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}

	loaded, err := appconfig.NewStoreAt(path).Load()
	if err != nil {
		t.Fatalf("Load of an older settings file: %v", err)
	}

	if loaded.Company.Name != "" || loaded.Company.TaxCode != "" {
		t.Errorf("Company = %+v, want it empty for a file that names none", loaded.Company)
	}
	if loaded.Connection.Database != "fingo" {
		t.Errorf("the connection was not read: %+v", loaded.Connection)
	}
}

// Save refuses a settings file with no connection, and writes nothing.
//
// Pinned because it cost a first run its answer. The company form was filled in
// and accepted, the profile came back correct, and then Save rejected the whole
// settings object because the connection was zero — and a fresh install has no
// connection to load, so this was not an edge case. It was every first run.
//
// The fix is the caller's: a first run records the connection it derived. This
// test is here so that the next person meets the constraint in a failing test
// rather than in a company name that never appears.
func TestStore_SaveRefusesASettingsFileWithNoConnection(t *testing.T) {
	path := filepath.Join(t.TempDir(), "FinGo", "config.json")
	store := appconfig.NewStoreAt(path)

	err := store.Save(appconfig.Settings{
		Company: appconfig.Company{Name: "Công ty Bình Minh", TaxCode: "0123456789-001"},
	})
	if err == nil {
		t.Fatal("Save wrote a settings file with no connection")
	}

	if _, statErr := os.Stat(path); statErr == nil {
		t.Error("a refused Save left a file behind")
	}
}

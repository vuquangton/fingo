// Package appconfig is the settings this installation keeps between runs.
//
// # Where it lives
//
// One file per Windows user, under %APPDATA%\FinGo\config.json. Not beside the
// executable and not beside the working directory: an installed application sits
// in a directory a standard user cannot write to, and the whole point of this
// file is that the operator can create it from inside the application.
//
// # The password is in the clear
//
// Deliberate, and documented rather than implied. The file holds the database
// password as text. It is created with mode 0600, which is the whole of the
// protection on Unix.
//
// **On Windows — which is where this runs — mode bits are not the mechanism.**
// os.WriteFile does not set a Windows ACL, and the 0600 above buys nothing
// there. What protects the file is that it inherits the ACL of the directory it
// is created in, and %APPDATA%\<user> is restricted to that user, SYSTEM and
// Administrators. So the security story is: the password is readable by the
// Windows account that runs FinGo and by anyone who is administrator on the
// machine. That is a reasonable trade for a single-operator application on a
// machine the operator controls. It is not reasonable for a shared machine or a
// hosted database, and it should be said out loud to anybody who deploys this.
//
// The alternative — DPAPI, which encrypts per Windows user and needs no key
// management — is the right answer if this is ever deployed where the profile
// directory is not trusted.
//
// # One connection, for now
//
// This is the connection the application opens at startup. internal/client is
// the registry of *which clients exist and where each one's database lives*, and
// this file is that registry's first row living on its own. When a second client
// arrives this becomes a list, and the screen that writes it becomes "add a
// client" — which is why the shape here is a Connection and not a pile of
// settings.
package appconfig

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"fingo/internal/adapter/mariadb/conn"
)

// DefaultParams is what a connection is given when the operator does not say.
//
// parseTime and loc are not cosmetic. Without parseTime the driver hands DATE
// and DATETIME columns back as []byte and every scan into a time.Time fails;
// with a loc other than UTC, a voucher posted at 23:00 on a closing date lands in
// the next period on a machine in another timezone.
const DefaultParams = "charset=utf8mb4&parseTime=True&loc=UTC"

// ErrNotConfigured is a store that holds no connection yet. It is the state a
// fresh installation is in, and the screen that fixes it is the point of this
// package.
var ErrNotConfigured = errors.New("appconfig: no connection has been saved yet")

// Connection is one client's database, as this installation reaches it.
//
// Server is conn.Config rather than a DSN string. A DSN typed by hand is a
// password with an '@' in it waiting to be misparsed, and conn.Config builds the
// string through the driver's own formatter so that escaping is not this
// package's problem.
type Connection struct {
	Server   conn.Config `json:"server"`
	Database string      `json:"database"`
}

// NewConnection builds a connection from what the form collected.
//
// Everything a person can get wrong is settled here rather than at the first
// query: an empty port becomes the default, whitespace goes, and the parameters
// are parsed by the same function the environment uses.
func NewConnection(host, port, user, password, database, params string) (Connection, error) {
	out := Connection{
		Server: conn.Config{
			Host:     strings.TrimSpace(host),
			Port:     strings.TrimSpace(port),
			User:     strings.TrimSpace(user),
			Password: password, // never trimmed: a space may be part of it
		},
		Database: strings.TrimSpace(database),
	}

	if out.Server.Port == "" {
		out.Server.Port = conn.DefaultPort
	}

	if strings.TrimSpace(params) == "" {
		params = DefaultParams
	}

	parsed, err := conn.ParseParams(params)
	if err != nil {
		return Connection{}, err
	}
	out.Server.Params = parsed

	if err := out.Validate(); err != nil {
		return Connection{}, err
	}

	return out, nil
}

// Validate reports whether the connection could be attempted.
//
// It checks only what makes the attempt meaningless. Whether the server is
// reachable, whether the database exists and whether the login is accepted are
// questions for the server, and answering them here by guessing would be a
// second, wrong answer.
func (c Connection) Validate() error {
	if strings.TrimSpace(c.Server.Host) == "" {
		return errors.New("a database connection needs a host")
	}
	if strings.TrimSpace(c.Server.User) == "" {
		return errors.New("a database connection needs a user")
	}
	if strings.TrimSpace(c.Database) == "" {
		return errors.New("a database connection needs a database name; one database holds one client's books")
	}
	if strings.TrimSpace(c.Server.Port) == "" {
		return fmt.Errorf("a database connection needs a port, usually %s", conn.DefaultPort)
	}

	return nil
}

// Config is the server half, as a copy.
//
// The parameters map is copied rather than shared. A caller that adds a dial
// timeout to what it was handed — which internal/onboarding does on every probe
// — would otherwise be writing into this connection, and a probe that changes
// the thing it probes turns up later as a settings file holding a parameter
// nobody typed.
func (c Connection) Config() conn.Config {
	out := c.Server

	out.Params = make(map[string]string, len(c.Server.Params))
	for key, value := range c.Server.Params {
		out.Params[key] = value
	}

	return out
}

// DSN is the connection string, built by the driver's own formatter.
func (c Connection) DSN() string { return c.Server.DSN(c.Database) }

// Params renders the parameters the way the form shows them: key=value pairs
// joined by '&', in a fixed order so that the same settings always render the
// same way.
func (c Connection) Params() string {
	keys := make([]string, 0, len(c.Server.Params))
	for key := range c.Server.Params {
		keys = append(keys, key)
	}

	sort.Strings(keys)

	pairs := make([]string, 0, len(keys))
	for _, key := range keys {
		pairs = append(pairs, key+"="+c.Server.Params[key])
	}

	return strings.Join(pairs, "&")
}

// Settings is what the file holds.
type Settings struct {
	// Connection is the database the application opens at startup.
	Connection Connection `json:"connection"`

	// Company is whose books these are: the name that heads every document and
	// the mã số thuế that appears on every declaration.
	//
	// It lives here rather than in the client's own database, which is where it
	// belongs, because moving it there is a schema change with a migration
	// behind it. P0 needs one thing from it: a first run collects it once and
	// does not ask again.
	//
	// A value rather than a pointer, so a settings file written by an earlier
	// build — one with no company in it — still loads. An empty company is the
	// honest reading of a file that names none.
	Company Company `json:"company,omitempty"`

	// SavedAt is when it was last written. Recorded because "when did somebody
	// change this?" is the first question asked when an installation starts
	// opening the wrong books.
	SavedAt time.Time `json:"saved_at"`
}

// Company is the client whose books these are.
type Company struct {
	// Name heads every printed document.
	Name string `json:"name,omitempty"`

	// TaxCode is the Vietnamese mã số thuế: ten digits, or ten and a three-digit
	// branch suffix. Empty for a company that has not given one yet.
	TaxCode string `json:"tax_code,omitempty"`
}

// Store is the settings file.
type Store struct {
	path string
}

// NewStore returns the store for this Windows user.
func NewStore() (Store, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return Store{}, fmt.Errorf("find the user's configuration directory: %w", err)
	}

	return NewStoreAt(filepath.Join(dir, "FinGo", "config.json")), nil
}

// NewStoreAt returns a store at a path, for a test or for a deployment that
// keeps its settings somewhere else.
func NewStoreAt(path string) Store { return Store{path: path} }

// Path is where the settings are read from and written to.
func (s Store) Path() string { return s.path }

// Exists reports whether anything has been saved.
func (s Store) Exists() bool {
	_, err := os.Stat(s.path)

	return err == nil
}

// Load reads the settings.
//
// A missing file is ErrNotConfigured, which is a state and not a failure: a
// fresh installation has nothing saved and is expected to. A file that exists
// and cannot be read or understood is a real error, because somebody meant
// something by it and silently ignoring it would open a different database than
// the one they configured.
func (s Store) Load() (Settings, error) {
	raw, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return Settings{}, ErrNotConfigured
		}

		return Settings{}, fmt.Errorf("read %s: %w", s.path, err)
	}

	var out Settings
	if err := json.Unmarshal(raw, &out); err != nil {
		return Settings{}, fmt.Errorf("%s is not readable as settings: %w", s.path, err)
	}
	if err := out.Connection.Validate(); err != nil {
		return Settings{}, fmt.Errorf("%s holds an incomplete connection: %w", s.path, err)
	}

	return out, nil
}

// Save writes the settings, creating the directory if it is not there.
//
// The file is written by replacing it whole, so a half-written settings file
// cannot be produced by an interrupted write — an installation that cannot start
// and cannot say why is the worst outcome of a failed save.
func (s Store) Save(settings Settings) error {
	if err := settings.Connection.Validate(); err != nil {
		return err
	}

	settings.SavedAt = time.Now().UTC()

	raw, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return fmt.Errorf("encode the settings: %w", err)
	}

	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create %s: %w", dir, err)
	}

	temporary := s.path + ".tmp"
	if err := os.WriteFile(temporary, append(raw, '\n'), 0o600); err != nil {
		return fmt.Errorf("write %s: %w", temporary, err)
	}

	if err := os.Rename(temporary, s.path); err != nil {
		return fmt.Errorf("replace %s: %w", s.path, err)
	}

	return nil
}

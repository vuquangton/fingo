package boot

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"

	"fingo/db"
	"fingo/internal/adapter/mariadb/conn"
	"fingo/internal/appconfig"
	"fingo/internal/books"
	"fingo/internal/dbhost"
	"fingo/internal/onboarding"
)

// The bundled server's account.
//
// No password, deliberately rather than lazily: the server binds to the loopback
// address, it exists only while the application is running, and it holds one
// user's own books. A password would have to be stored on the same machine,
// which is not a boundary — it is the appearance of one.
const (
	bundledUser     = "root"
	bundledDatabase = "fingo"
)

// LiveConfig is what a real installation needs to provision itself.
type LiveConfig struct {
	// Root is where everything this installation owns is kept. One directory, so
	// uninstalling is deleting it.
	Root string

	// BinDir holds mariadbd.exe, mariadb-install-db.exe and mariadb-admin.exe.
	BinDir string

	// Port is the loopback port for the private server. Chosen by the caller,
	// because choosing one is a fact about the machine rather than about books.
	Port int
}

// Live provisions a real installation: a data directory, a private server, a
// database, a schema and a chart of accounts.
type Live struct {
	layout     dbhost.Layout
	server     dbhost.Config
	installExe string

	connection appconfig.Connection
	connCfg    conn.Config
	serverDSN  string
	clientDSN  string

	sup *dbhost.Supervisor

	serverConn *sql.DB
	clientConn *sql.DB
}

// NewLive builds the provisioner for one installation.
func NewLive(c LiveConfig) (*Live, error) {
	switch {
	case strings.TrimSpace(c.Root) == "":
		return nil, errors.New("a provisioned installation needs a root directory")
	case strings.TrimSpace(c.BinDir) == "":
		return nil, errors.New("a bundled server needs a directory to find its binaries in")
	case c.Port <= 0 || c.Port > 65535:
		return nil, fmt.Errorf("port %d is not a port a server can listen on", c.Port)
	}

	layout := dbhost.NewLayout(c.Root)

	connection, err := appconfig.NewConnection(
		"127.0.0.1", strconv.Itoa(c.Port), bundledUser, "", bundledDatabase,
		"charset=utf8mb4&parseTime=True&loc=UTC")
	if err != nil {
		return nil, fmt.Errorf("build the local connection: %w", err)
	}

	connCfg := connection.Config()

	return &Live{
		layout: layout,
		server: dbhost.Config{
			MysqldExe: filepath.Join(c.BinDir, "mariadbd.exe"),
			AdminExe:  filepath.Join(c.BinDir, "mariadb-admin.exe"),
			Datadir:   layout.Data,
			Port:      c.Port,
			LogFile:   layout.Log,
			PIDFile:   layout.PID,
		},
		installExe: filepath.Join(c.BinDir, "mariadb-install-db.exe"),

		connection: connection,
		connCfg:    connCfg,
		serverDSN:  connCfg.DSN(""),
		clientDSN:  connection.DSN(),

		sup: dbhost.NewSupervisor(&dbhost.ExecLauncher{}),
	}, nil
}

// DSN is the connection string the application opens the books with.
func (l *Live) DSN() string { return l.clientDSN }

// Connection is where this installation keeps its books.
//
// Exposed because the settings file records the connection, and its own
// validation refuses a settings file without one — so a first run that saved
// only the company wrote nothing at all, and failed doing it. The connection is
// derived here rather than configured, but it is still the connection, and
// recording it is what makes the file complete.
func (l *Live) Connection() appconfig.Connection { return l.connection }

// DataDirectoryExists reports whether the server's data directory is there.
func (l *Live) DataDirectoryExists() bool {
	info, err := os.Stat(l.layout.Data)

	return err == nil && info.IsDir()
}

// CreateDataDirectory initialises one. Slow, and it happens once.
func (l *Live) CreateDataDirectory(ctx context.Context) error {
	return dbhost.Init(ctx, dbhost.InitConfig{
		InstallExe: l.installExe,
		Datadir:    l.layout.Data,
	})
}

// StartServer brings the private server up and waits until it can answer.
//
// Readiness is asked for rather than slept on: mariadbd accepts a connection
// before it can serve a query, so a caller that dials and stops there migrates
// against a server still opening its tables.
func (l *Live) StartServer(ctx context.Context) error {
	if err := l.sup.EnsureStarted(ctx, l.server); err != nil {
		return l.withLogTail(err)
	}

	server, err := sql.Open("mysql", l.serverDSN)
	if err != nil {
		return fmt.Errorf("open the bundled server: %w", err)
	}
	l.serverConn = server

	if err := dbhost.WaitReadyOrExit(ctx, server.PingContext, l.sup.Done(), 300*time.Millisecond); err != nil {
		return l.withLogTail(err)
	}

	return nil
}

// DatabaseExists reports whether the client's database is there.
func (l *Live) DatabaseExists(ctx context.Context) (bool, error) {
	if l.serverConn == nil {
		return false, errors.New("the database server is not open")
	}

	var found int
	if err := l.serverConn.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM information_schema.schemata WHERE schema_name = ?`,
		bundledDatabase).Scan(&found); err != nil {
		return false, fmt.Errorf("look for the database %s: %w", bundledDatabase, err)
	}

	return found > 0, nil
}

// CreateDatabase creates it.
//
// onboarding owns this — the name check, and the operator who may use a database
// but not create one — and a second implementation here would be a second
// answer to a question already settled.
func (l *Live) CreateDatabase(ctx context.Context) error {
	return onboarding.EnsureDatabase(ctx, l.connection)
}

// SchemaVersion reports the applied migration version, 0 for none.
func (l *Live) SchemaVersion(ctx context.Context) (int64, error) {
	client, err := l.openClient()
	if err != nil {
		return 0, err
	}

	var present int
	if err := client.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM information_schema.tables
		 WHERE table_schema = ? AND table_name = 'goose_db_version'`,
		bundledDatabase).Scan(&present); err != nil {
		return 0, fmt.Errorf("look for the migration table: %w", err)
	}
	if present == 0 {
		return 0, nil
	}

	var version int64
	if err := client.QueryRowContext(ctx,
		`SELECT COALESCE(MAX(version_id), 0) FROM goose_db_version WHERE is_applied = 1`).Scan(&version); err != nil {
		return 0, fmt.Errorf("read the schema version: %w", err)
	}

	return version, nil
}

// Migrate applies every pending migration, and the triggers the schema relies on.
func (l *Live) Migrate(ctx context.Context) error {
	client, err := l.openClient()
	if err != nil {
		return err
	}

	if err := db.Up(client); err != nil {
		return fmt.Errorf("apply the migrations: %w", err)
	}

	return nil
}

// ChartExists reports whether the chart of accounts has been seeded.
func (l *Live) ChartExists(ctx context.Context) (bool, error) {
	client, err := l.openClient()
	if err != nil {
		return false, err
	}

	var accounts int
	if err := client.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM accounts WHERE is_active = TRUE`).Scan(&accounts); err != nil {
		return false, fmt.Errorf("count the chart of accounts: %w", err)
	}

	return accounts > 0, nil
}

// Books opens the client's books for the screens to read.
//
// It is a pool of its own, beside the one provisioning used, and that is
// deliberate: the screens outlive provisioning, and closing the books when boot
// finished would leave every screen holding a closed handle.
//
// It returns the books *composed* — the ledger and the reports over it — because
// assembling the two is not something either of them should learn, and doing it
// in one place is what makes it true that every screen reads the same books.
//
// The caller owns the result and closes it.
func (l *Live) Books(ctx context.Context) (*books.Books, error) {
	held, err := books.Open(ctx, l.connCfg, bundledDatabase)
	if err != nil {
		return nil, fmt.Errorf("open the client's books: %w", err)
	}

	return held, nil
}

// SeedChart seeds the TT133 chart.
//
// Through practice, not by inserting rows: the chart is the domain's, and a
// second copy of it here is a second answer that would drift.
func (l *Live) SeedChart(ctx context.Context) error {
	held, err := l.Books(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = held.Close() }()

	if err := held.EnsureChart(ctx); err != nil {
		return fmt.Errorf("seed the chart of accounts: %w", err)
	}

	return nil
}

// Close stops the server and gives up every connection.
func (l *Live) Close(ctx context.Context) error {
	var errs []error

	if l.clientConn != nil {
		errs = append(errs, l.clientConn.Close())
		l.clientConn = nil
	}
	if l.serverConn != nil {
		errs = append(errs, l.serverConn.Close())
		l.serverConn = nil
	}
	if err := l.sup.Stop(ctx); err != nil {
		errs = append(errs, err)
	}

	return errors.Join(errs...)
}

// openClient opens the client's database, once.
func (l *Live) openClient() (*sql.DB, error) {
	if l.clientConn != nil {
		return l.clientConn, nil
	}

	client, err := sql.Open("mysql", l.clientDSN)
	if err != nil {
		return nil, fmt.Errorf("open the client's database: %w", err)
	}

	l.clientConn = client

	return client, nil
}

// withLogTail adds the server's own last words to a failure.
//
// When a bundled server will not start, its log is the only thing that says why,
// and the operator has no reason to know the file exists.
func (l *Live) withLogTail(err error) error {
	raw, readErr := os.ReadFile(l.layout.Log)
	if readErr != nil {
		return err
	}

	const tail = 1500
	if len(raw) > tail {
		raw = raw[len(raw)-tail:]
	}

	return fmt.Errorf("%w\nserver log tail:\n%s", err, raw)
}

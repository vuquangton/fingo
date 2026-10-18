// Package boot brings an installation to the point where books can be opened.
//
// On the first run it creates whatever is missing — a data directory, a server,
// a database, a schema, a chart of accounts. On every run after that it leaves
// all of it exactly as it is.
//
// The order below is not a preference. A data directory must exist before a
// server can open it; a server must be up before a database can be created in
// it; a database must exist before migrations can run; and the migrations must
// have run before a chart of accounts can be seeded into their tables. Any other
// order fails, and each failure looks like a different problem.
package boot

import (
	"context"
	"fmt"
)

// Provisioner is everything a first run has to check and create.
//
// Declared here, where it is used, so the sequence can be driven with a fake.
// What each step actually does — spawn a server, run goose, seed the TT133
// chart — is somebody else's business, and is tested there.
type Provisioner interface {
	// DataDirectoryExists reports whether the server's data directory is there.
	DataDirectoryExists() bool

	// CreateDataDirectory initialises one. Slow, and it happens once.
	CreateDataDirectory(ctx context.Context) error

	// StartServer brings the server up. Safe when it is already up.
	StartServer(ctx context.Context) error

	// DatabaseExists reports whether the client's database is there.
	DatabaseExists(ctx context.Context) (bool, error)

	// CreateDatabase creates it.
	CreateDatabase(ctx context.Context) error

	// SchemaVersion reports the applied migration version, 0 for none.
	SchemaVersion(ctx context.Context) (int64, error)

	// Migrate applies every pending migration.
	Migrate(ctx context.Context) error

	// ChartExists reports whether the chart of accounts has been seeded.
	ChartExists(ctx context.Context) (bool, error)

	// SeedChart seeds it. Idempotent by contract, and never overwrites.
	SeedChart(ctx context.Context) error
}

// Ensure brings an installation to the point where books can be opened.
//
// Every step is guarded by a probe rather than attempted and forgiven. A launch
// that re-seeds a chart or re-runs migrations is a launch that can destroy books
// it was supposed to open, so anything already there is left alone.
//
// It stops at the first failure and says which step it was. Continuing past one
// would migrate a database that was never created, and report the resulting
// error instead of the real one.
func Ensure(ctx context.Context, p Provisioner) error {
	if !p.DataDirectoryExists() {
		if err := p.CreateDataDirectory(ctx); err != nil {
			return fmt.Errorf("create the data directory: %w", err)
		}
	}

	if err := p.StartServer(ctx); err != nil {
		return fmt.Errorf("start the database server: %w", err)
	}

	exists, err := p.DatabaseExists(ctx)
	if err != nil {
		return fmt.Errorf("check whether the database exists: %w", err)
	}
	if !exists {
		if err := p.CreateDatabase(ctx); err != nil {
			return fmt.Errorf("create the database: %w", err)
		}
	}

	version, err := p.SchemaVersion(ctx)
	if err != nil {
		return fmt.Errorf("read the schema version: %w", err)
	}
	if version == 0 {
		if err := p.Migrate(ctx); err != nil {
			return fmt.Errorf("migrate the database: %w", err)
		}
	}

	hasChart, err := p.ChartExists(ctx)
	if err != nil {
		return fmt.Errorf("check whether the chart of accounts exists: %w", err)
	}
	if !hasChart {
		if err := p.SeedChart(ctx); err != nil {
			return fmt.Errorf("seed the chart of accounts: %w", err)
		}
	}

	return nil
}

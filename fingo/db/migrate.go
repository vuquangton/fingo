package db

import (
	"database/sql"
	"fmt"
	"io/fs"

	"github.com/pressly/goose/v3"
)

const (
	// migrationsDir is the versioned migration directory inside Migrations.
	migrationsDir = "migrations"

	// triggersDir holds DDL that is re-applied on every open rather than
	// versioned. See applyTriggers.
	triggersDir = "triggers"
)

// Up applies every pending migration to one client's database, then installs the
// triggers.
//
// FinGo keeps one database per client (docs/requirements-v1.md §1.1), so this
// is called once per client against that client's own connection. Every client
// gets an identical schema; there is no per-client schema drift by design.
//
// Note: goose keeps its base filesystem and dialect in package-level state, so
// Up is not safe to call concurrently. FinGo has a single operator, so that is
// acceptable — but it is a real constraint, not an oversight.
func Up(sqlDB *sql.DB) error {
	goose.SetBaseFS(Migrations)

	if err := goose.SetDialect("mysql"); err != nil {
		return fmt.Errorf("set goose dialect: %w", err)
	}

	if err := goose.Up(sqlDB, migrationsDir); err != nil {
		return fmt.Errorf("apply migrations: %w", err)
	}

	return applyTriggers(sqlDB)
}

// applyTriggers installs the schema's triggers.
//
// They are deliberately not versioned migrations. sqlc reads db/migrations as
// its schema source and its MySQL parser cannot read CREATE TRIGGER, so keeping
// them there would break code generation; and listing schema files by hand in
// sqlc.yaml would let the generated models drift the first time someone added a
// migration and forgot to list it.
//
// Each trigger is written CREATE OR REPLACE, so applying them on every open is
// idempotent — and an accidentally dropped trigger restores itself.
//
// One statement per file: a trigger body contains semicolons of its own, so a
// file cannot be split naively and is executed whole.
func applyTriggers(sqlDB *sql.DB) error {
	// fs.Glob returns its matches in lexical order, which is why the files are
	// numbered.
	names, err := fs.Glob(Migrations, triggersDir+"/*.sql")
	if err != nil {
		return fmt.Errorf("list triggers: %w", err)
	}

	for _, name := range names {
		statement, err := Migrations.ReadFile(name)
		if err != nil {
			return fmt.Errorf("read trigger %s: %w", name, err)
		}

		if _, err := sqlDB.Exec(string(statement)); err != nil {
			return fmt.Errorf("apply trigger %s: %w", name, err)
		}
	}

	return nil
}

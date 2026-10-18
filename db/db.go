// Package db embeds the schema and applies it with goose.
//
// PostgreSQL is the only store, and the schema carries the rules that Go must
// not be able to bypass: row-level security, immutability, and a deferred
// balance check. Applying it is therefore part of starting the application, not
// a separate operational step a deployment can forget (ADR 0006).
package db

import (
	"context"
	"database/sql"
	"embed"
	"fmt"

	"github.com/pressly/goose/v3"
)

//go:embed migrations/*.sql
var migrations embed.FS

// Up applies every pending migration.
//
// goose keeps its dialect in package state, so Up must not be called
// concurrently. The application calls it once at startup; tests call it from a
// single TestMain.
func Up(ctx context.Context, sqlDB *sql.DB) error {
	goose.SetBaseFS(migrations)
	if err := goose.SetDialect("postgres"); err != nil {
		return fmt.Errorf("set goose dialect: %w", err)
	}
	if err := goose.UpContext(ctx, sqlDB, "migrations"); err != nil {
		return fmt.Errorf("apply migrations: %w", err)
	}
	return nil
}

// Version reports the applied migration version, 0 for none.
func Version(ctx context.Context, sqlDB *sql.DB) (int64, error) {
	goose.SetBaseFS(migrations)
	if err := goose.SetDialect("postgres"); err != nil {
		return 0, fmt.Errorf("set goose dialect: %w", err)
	}
	v, err := goose.GetDBVersionContext(ctx, sqlDB)
	if err != nil {
		return 0, fmt.Errorf("read schema version: %w", err)
	}
	return v, nil
}

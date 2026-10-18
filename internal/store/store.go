// Package store is the only way to reach the database.
//
// Every business query runs inside a transaction that first sets app.client_id,
// because row-level security filters on it: a query without a scope returns zero
// rows, never every client's rows (ADR 0006). Making the scope a required
// argument, and the transaction the only entry point, is what turns that from a
// convention into a shape.
package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ErrNoScope is returned when a caller tries to work on books without naming a
// client. It is an error, not a wildcard.
var ErrNoScope = errors.New("store: no client scope")

// Scope names the client a transaction is for.
type Scope struct {
	id string
}

// NewScope validates a client id and returns the scope that names it.
func NewScope(id string) (Scope, error) {
	if id == "" {
		return Scope{}, ErrNoScope
	}
	if _, err := parseUUID(id); err != nil {
		return Scope{}, fmt.Errorf("store: client id %q is not a uuid: %w", id, err)
	}
	return Scope{id: id}, nil
}

// ID is the client id this scope names.
func (s Scope) ID() string { return s.id }

// IsZero reports whether the scope names nobody.
func (s Scope) IsZero() bool { return s.id == "" }

// Store owns the connection pool.
type Store struct {
	pool *pgxpool.Pool
}

// Config is what Open needs.
type Config struct {
	DSN             string
	MaxOpenConns    int32
	MaxIdleConns    int32
	ConnMaxLifetime time.Duration
}

// Open builds a pool. A descriptor that cannot connect is a startup failure, not
// something to discover on the first request.
func Open(ctx context.Context, cfg Config) (*Store, error) {
	poolCfg, err := pgxpool.ParseConfig(cfg.DSN)
	if err != nil {
		return nil, fmt.Errorf("store: parse dsn: %w", err)
	}

	if cfg.MaxOpenConns > 0 {
		poolCfg.MaxConns = cfg.MaxOpenConns
	}
	if cfg.MaxIdleConns > 0 {
		poolCfg.MinConns = cfg.MaxIdleConns
	}
	if cfg.ConnMaxLifetime > 0 {
		poolCfg.MaxConnLifetime = cfg.ConnMaxLifetime
	}

	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		return nil, fmt.Errorf("store: open pool: %w", err)
	}

	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if err := pool.Ping(pingCtx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("store: ping: %w", err)
	}

	return &Store{pool: pool}, nil
}

// Close releases every connection.
func (s *Store) Close() {
	if s.pool != nil {
		s.pool.Close()
	}
}

// Pool exposes the pool for migrations and tests. Business code should not use
// it: an unscoped query bypasses the tenant boundary by construction.
func (s *Store) Pool() *pgxpool.Pool { return s.pool }

// InClient runs fn in one transaction scoped to a client.
//
// set_config(..., true) is SET LOCAL: the setting dies with the transaction, so
// a pooled connection cannot carry one client's scope into the next request.
// The scope is re-applied on every call rather than cached for the same reason.
func (s *Store) InClient(ctx context.Context, scope Scope, fn func(pgx.Tx) error) error {
	if scope.IsZero() {
		return ErrNoScope
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("store: begin: %w", err)
	}
	// Rollback after a successful commit is a no-op, so this is the correct
	// defer: it closes the transaction on every error path without a commit flag.
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx, "SELECT set_config('app.client_id', $1, true)", scope.ID()); err != nil {
		return fmt.Errorf("store: set client scope: %w", err)
	}

	if err := fn(tx); err != nil {
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("store: commit: %w", err)
	}
	return nil
}

// Read is InClient for a read-only transaction.
func (s *Store) Read(ctx context.Context, scope Scope, fn func(pgx.Tx) error) error {
	return s.InClient(ctx, scope, fn)
}

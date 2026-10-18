// Command migrate applies the schema to one database.
//
// It connects as the PostgreSQL owner (the role that can run DDL), because the
// application role owns nothing: migrations are an administrative act, and the
// runtime role must never be able to change the schema it is filtered by.
//
//	$env:PGADMIN_PASSWORD='...'
//	go run ./cmd/migrate -database fingo_dev
package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"

	"fingo/db"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "migrate:", err)
		os.Exit(1)
	}
}

func run() error {
	database := flag.String("database", "fingo_dev", "database to migrate")
	host := flag.String("host", envOr("PGHOST", "127.0.0.1"), "host")
	port := flag.String("port", envOr("PGPORT", "5432"), "port")
	user := flag.String("user", envOr("PGADMIN_USER", "postgres"), "owner role")
	flag.Parse()

	password := os.Getenv("PGADMIN_PASSWORD")
	if password == "" {
		return fmt.Errorf("PGADMIN_PASSWORD is not set")
	}

	dsn := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		*host, *port, *user, password, *database)

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	sqlDB, err := sql.Open("pgx", dsn)
	if err != nil {
		return fmt.Errorf("open %s: %w", *database, err)
	}
	defer func() { _ = sqlDB.Close() }()

	pingCtx, pingCancel := context.WithTimeout(ctx, 10*time.Second)
	defer pingCancel()

	if err := sqlDB.PingContext(pingCtx); err != nil {
		return fmt.Errorf("connect to %s: %w", *database, err)
	}

	if err := db.Up(ctx, sqlDB); err != nil {
		return err
	}

	version, err := db.Version(ctx, sqlDB)
	if err != nil {
		return err
	}

	fmt.Printf("%s: schema at version %d\n", *database, version)
	return nil
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

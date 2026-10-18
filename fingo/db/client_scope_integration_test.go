package db_test

import (
	"database/sql"
	"fmt"
	"strings"
	"testing"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/pressly/goose/v3"

	"fingo/db"
)

// tablesThatAreNotClientScoped are the tables migration 00015 leaves alone.
//
// The registry is the client; it is not a client's data. goose_db_version is
// goose's own bookkeeping. Everything else in the schema is a client's, and the
// tests below derive their list from the database rather than repeating it, so a
// table added later without a client column fails here.
var tablesThatAreNotClientScoped = map[string]bool{
	"clients":          true,
	"goose_db_version": true,
}

// scratch creates an empty database, migrates it, and drops it when the test
// ends. It is the setup both tests below need and neither of them is about.
func scratch(t *testing.T) *sql.DB {
	t.Helper()

	if testing.Short() {
		t.Skip("integration test skipped in short mode")
	}

	cfg := loadConfig(t)

	admin, err := sql.Open("mysql", cfg.DSN(""))
	if err != nil {
		t.Fatalf("open admin connection: %v", err)
	}
	t.Cleanup(func() { admin.Close() })

	if err := admin.Ping(); err != nil {
		t.Skipf("MariaDB not reachable at %s:%s: %v", cfg.Host, cfg.Port, err)
	}

	sweepScratch(t, admin)

	name := fmt.Sprintf("%s%d", scratchPrefix, time.Now().UnixNano())
	if !strings.HasPrefix(name, scratchPrefix) {
		t.Fatalf("refusing to create database %q: outside the scratch prefix", name)
	}

	if _, err := admin.Exec(fmt.Sprintf(
		"CREATE DATABASE `%s` CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci", name)); err != nil {
		t.Fatalf("create scratch database %s: %v", name, err)
	}
	t.Cleanup(func() {
		if !strings.HasPrefix(name, scratchPrefix) {
			t.Errorf("refusing to drop %q: outside the scratch prefix", name)
			return
		}
		if _, err := admin.Exec(fmt.Sprintf("DROP DATABASE IF EXISTS `%s`", name)); err != nil {
			t.Errorf("drop scratch database %s: %v", name, err)
		}
	})

	client, err := sql.Open("mysql", cfg.DSN(name))
	if err != nil {
		t.Fatalf("open client connection: %v", err)
	}
	t.Cleanup(func() { client.Close() })

	if err := db.Up(client); err != nil {
		t.Fatalf("migrations failed against %s: %v", name, err)
	}

	return client
}

// TestEveryFinancialTableIsClientScoped proves migration 00015 against a real
// MariaDB: the registry exists, every other table carries a client column, that
// column is a real foreign key, and the registry's uniqueness rule behaves the
// way internal/client says it does.
func TestEveryFinancialTableIsClientScoped(t *testing.T) {
	client := scratch(t)
	name := schemaOf(t, client)

	registry := columnsOf(t, client, name, "clients")
	for _, want := range []string{"id", "name", "tax_id", "created_at", "is_active"} {
		if _, ok := registry[want]; !ok {
			t.Errorf("clients is missing %q; columns present: %v", want, keysOf(registry))
		}
	}

	// Every other table carries the client column, and it is a VARCHAR(36)
	// like every other identifier in this schema.
	scoped := 0
	for _, table := range otherTables(t, client, name) {
		columns := columnsOf(t, client, name, table)

		typ, ok := columns["client_id"]
		if !ok {
			t.Errorf("%s has no client_id column; columns present: %v", table, keysOf(columns))
			continue
		}
		if !strings.HasPrefix(strings.ToLower(typ), "varchar(36)") {
			t.Errorf("%s.client_id is %q, want varchar(36)", table, typ)
		}
		scoped++
	}

	if scoped < 10 {
		t.Fatalf("only %d tables were client-scoped; the schema was not scanned", scoped)
	}
	t.Logf("%d tables carry a client_id column", scoped)

	// A foreign key, not a comment: a client_id naming no registered client
	// cannot be written.
	if err := insertTradePartner(client, "p-orphan", "Công ty Ma", "'no-such-client'"); err == nil {
		t.Error("a row naming an unregistered client was accepted")
	}

	// The registry's own rules, which internal/client makes in Go: a tax code
	// is one company, and no tax code is exempt.
	mustExec(t, client, `INSERT INTO clients (id, name, tax_id, created_at) VALUES
		('c-a', 'Công ty Bình Minh', '0101234567', UTC_TIMESTAMP(6)),
		('c-b', 'Công ty Đông Á', NULL, UTC_TIMESTAMP(6)),
		('c-c', 'Công ty Chưa Có MST', NULL, UTC_TIMESTAMP(6))`)

	if err := insertClient(client, "c-d", "Same Company Twice", "'0101234567'"); err == nil {
		t.Error("two clients were registered under one tax code")
	}
	if err := insertClient(client, "c-e", "Another Without A Code", "NULL"); err != nil {
		t.Errorf("a client with no tax code was refused: %v", err)
	}

	// And now the foreign key has something to point at.
	if err := insertTradePartner(client, "p-1", "Công ty Bình Minh", "'c-a'"); err != nil {
		t.Errorf("a row naming a registered client was refused: %v", err)
	}
}

// TestClientScopeMigrationRollsBack proves the Down section runs.
//
// A migration whose Down fails is one nobody can undo, and the failure surfaces
// on the day somebody needs to undo it. Eighteen foreign keys have to go before
// the columns that carry them and before the registry they point at.
func TestClientScopeMigrationRollsBack(t *testing.T) {
	client := scratch(t)
	name := schemaOf(t, client)

	goose.SetBaseFS(db.Migrations)
	if err := goose.SetDialect("mysql"); err != nil {
		t.Fatalf("set goose dialect: %v", err)
	}
	if err := goose.DownTo(client, "migrations", 14); err != nil {
		t.Fatalf("roll back migration 00015: %v", err)
	}

	for _, table := range baseTables(t, client, name) {
		if table == "goose_db_version" {
			continue
		}
		if table == "clients" {
			t.Error("clients survived the rollback but the migration that created it was undone")

			continue
		}
		if _, ok := columnsOf(t, client, name, table)["client_id"]; ok {
			t.Errorf("%s still has a client_id column after the rollback", table)
		}
	}

	// Re-applying must work on the rolled-back schema, or "undo and try again"
	// is not a thing anyone can do.
	if err := db.Up(client); err != nil {
		t.Fatalf("re-applying migration 00015 after a rollback: %v", err)
	}
	if _, ok := columnsOf(t, client, name, "vouchers")["client_id"]; !ok {
		t.Error("vouchers.client_id did not come back after re-applying")
	}
}

// schemaOf returns the database the connection is pointed at.
func schemaOf(t *testing.T, sqlDB *sql.DB) string {
	t.Helper()

	var name string
	if err := sqlDB.QueryRow("SELECT DATABASE()").Scan(&name); err != nil {
		t.Fatalf("read current schema: %v", err)
	}

	return name
}

// otherTables lists the database's own tables minus the ones that are not a
// client's data.
func otherTables(t *testing.T, sqlDB *sql.DB, schema string) []string {
	t.Helper()

	var out []string
	for _, table := range baseTables(t, sqlDB, schema) {
		if !tablesThatAreNotClientScoped[table] {
			out = append(out, table)
		}
	}

	return out
}

// baseTables lists the database's own tables.
func baseTables(t *testing.T, sqlDB *sql.DB, schema string) []string {
	t.Helper()

	rows, err := sqlDB.Query(`
		SELECT table_name FROM information_schema.tables
		WHERE table_schema = ? AND table_type = 'BASE TABLE'
		ORDER BY table_name`, schema)
	if err != nil {
		t.Fatalf("list tables: %v", err)
	}
	defer rows.Close()

	var out []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatalf("scan table name: %v", err)
		}
		out = append(out, name)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate tables: %v", err)
	}

	return out
}

// insertClient writes one registry row, returning the driver's error if the
// database refuses it.
func insertClient(sqlDB *sql.DB, id, name, taxID string) error {
	_, err := sqlDB.Exec(fmt.Sprintf(
		`INSERT INTO clients (id, name, tax_id, created_at) VALUES ('%s', '%s', %s, UTC_TIMESTAMP(6))`,
		id, name, taxID))

	return err
}

func insertTradePartner(sqlDB *sql.DB, id, name, clientID string) error {
	_, err := sqlDB.Exec(fmt.Sprintf(
		`INSERT INTO trade_partners (id, name, partner_type, client_id) VALUES ('%s', '%s', 'CUSTOMER', %s)`,
		id, name, clientID))

	return err
}

func mustExec(t *testing.T, sqlDB *sql.DB, query string) {
	t.Helper()

	if _, err := sqlDB.Exec(query); err != nil {
		t.Fatalf("exec %q: %v", query, err)
	}
}

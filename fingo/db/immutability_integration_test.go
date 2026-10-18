package db_test

import (
	"database/sql"
	"fmt"
	"strings"
	"testing"
	"time"

	"fingo/db"
)

// scratchSchema creates, migrates and returns a throwaway database.
//
// It reuses the migrate test's scratch prefix, so a run that fails part-way is
// swept by the same guard rather than leaving a database behind.
func scratchSchema(t *testing.T) *sql.DB {
	t.Helper()

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

// seedAccounts inserts the two accounts a test voucher posts between.
func seedAccounts(t *testing.T, sqlDB *sql.DB) {
	t.Helper()

	accounts := []struct{ id, code, name, accountType, nature string }{
		{"acc-1111", "1111", "Tiền mặt", "ASSET", "DEBIT"},
		{"acc-5111", "5111", "Doanh thu bán hàng", "REVENUE", "CREDIT"},
	}

	for _, a := range accounts {
		if _, err := sqlDB.Exec(`
			INSERT INTO accounts (id, code, name, account_type, nature, is_active)
			VALUES (?, ?, ?, ?, ?, TRUE)`,
			a.id, a.code, a.name, a.accountType, a.nature); err != nil {
			t.Fatalf("seed account %s: %v", a.code, err)
		}
	}
}

// seedVoucher inserts a voucher and two lines, posted or not.
//
// It follows the real lifecycle — created as a draft, lines added, then posted —
// because the immutability trigger makes any other order impossible. The first
// draft of this helper tried to add lines after marking the voucher posted and
// the trigger refused it, which is the trigger doing its job.
func seedVoucher(t *testing.T, sqlDB *sql.DB, id string, posted bool) {
	t.Helper()

	if _, err := sqlDB.Exec(`
		INSERT INTO vouchers (id, voucher_no, voucher_date, voucher_type, description, is_posted)
		VALUES (?, '', ?, 'GENERAL', ?, FALSE)`,
		id,
		time.Date(2026, 3, 14, 0, 0, 0, 0, time.UTC),
		descriptionOf(posted)); err != nil {
		t.Fatalf("seed voucher %s: %v", id, err)
	}

	lines := []struct {
		id, account, side string
		amount            int64
	}{
		{id + "-l1", "acc-1111", "DEBIT", 100},
		{id + "-l2", "acc-5111", "CREDIT", 100},
	}

	for i, l := range lines {
		if _, err := sqlDB.Exec(`
			INSERT INTO voucher_lines (id, voucher_id, line_order, account_id, side, amount)
			VALUES (?, ?, ?, ?, ?, ?)`, l.id, id, i+1, l.account, l.side, l.amount); err != nil {
			t.Fatalf("seed line %s of %s: %v", l.id, id, err)
		}
	}

	if !posted {
		return
	}

	// Posting is the one transition the trigger permits: it guards OLD.is_posted,
	// so a draft becoming posted is allowed and a posted voucher changing is not.
	at := time.Date(2026, 3, 14, 10, 30, 0, 0, time.UTC)
	if _, err := sqlDB.Exec(`
		UPDATE vouchers
		SET is_posted = TRUE, book = 'PK', fiscal_year = 2026, seq = 1,
		    voucher_no = 'PK2026-0001', posted_date = ?, posted_at = ?, posted_by = ?
		WHERE id = ?`,
		time.Date(2026, 3, 14, 0, 0, 0, 0, time.UTC), at, "operator-1", id); err != nil {
		t.Fatalf("post voucher %s: %v", id, err)
	}
}

func descriptionOf(posted bool) string {
	if posted {
		return "posted entry"
	}
	return "draft entry"
}

// A posted voucher must be unreachable by anything that writes SQL directly.
//
// The domain already refuses to change one. This test exists because the domain
// is not the only thing that can reach the table: a repair script, a future
// migration, or a bug in an adapter all bypass it. A ledger whose entries can be
// silently rewritten is not a ledger.
func TestSchema_PostedVouchersCannotBeChanged(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test skipped in short mode")
	}

	sqlDB := scratchSchema(t)
	seedAccounts(t, sqlDB)
	seedVoucher(t, sqlDB, "v-posted", true)
	seedVoucher(t, sqlDB, "v-draft", false)

	mutations := map[string]string{
		"update a posted voucher": "UPDATE vouchers SET description = 'tampered' WHERE id = 'v-posted'",
		"delete a posted voucher": "DELETE FROM vouchers WHERE id = 'v-posted'",
		"add a line to a posted voucher": `
			INSERT INTO voucher_lines (id, voucher_id, line_order, account_id, side, amount)
			VALUES ('l-extra', 'v-posted', 9, 'acc-1111', 'DEBIT', 1)`,
		"update a line of a posted voucher": "UPDATE voucher_lines SET amount = 1 WHERE voucher_id = 'v-posted'",
		"delete a line of a posted voucher": "DELETE FROM voucher_lines WHERE voucher_id = 'v-posted'",
	}

	for name, statement := range mutations {
		t.Run(name, func(t *testing.T) {
			if _, err := sqlDB.Exec(statement); err == nil {
				t.Fatalf("%s must be refused by the database, got nil", name)
			}
		})
	}

	// The posted voucher survived every attempt, unaltered.
	var description string
	if err := sqlDB.QueryRow("SELECT description FROM vouchers WHERE id = 'v-posted'").Scan(&description); err != nil {
		t.Fatalf("the posted voucher must still exist: %v", err)
	}
	if description != "posted entry" {
		t.Errorf("description = %q, want it unchanged", description)
	}

	var lines int
	if err := sqlDB.QueryRow("SELECT COUNT(*) FROM voucher_lines WHERE voucher_id = 'v-posted'").Scan(&lines); err != nil {
		t.Fatalf("count lines: %v", err)
	}
	if lines != 2 {
		t.Errorf("lines = %d, want the original 2", lines)
	}
}

// A draft is not a record yet, so it stays a working copy: editable and
// discardable. Freezing drafts would make the ledger unusable.
func TestSchema_DraftsStayEditable(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test skipped in short mode")
	}

	sqlDB := scratchSchema(t)
	seedAccounts(t, sqlDB)
	seedVoucher(t, sqlDB, "v-draft", false)

	if _, err := sqlDB.Exec("UPDATE vouchers SET description = 'edited' WHERE id = 'v-draft'"); err != nil {
		t.Errorf("a draft must stay editable: %v", err)
	}
	if _, err := sqlDB.Exec(`
		UPDATE voucher_lines SET amount = 250 WHERE voucher_id = 'v-draft' AND line_order = 1`); err != nil {
		t.Errorf("a draft's line must stay editable: %v", err)
	}
	if _, err := sqlDB.Exec("DELETE FROM voucher_lines WHERE voucher_id = 'v-draft' AND line_order = 2"); err != nil {
		t.Errorf("a draft's line must stay deletable: %v", err)
	}
	if _, err := sqlDB.Exec("DELETE FROM vouchers WHERE id = 'v-draft'"); err != nil {
		t.Errorf("a draft must stay deletable: %v", err)
	}
}

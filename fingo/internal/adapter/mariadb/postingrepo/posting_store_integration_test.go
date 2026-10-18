package postingrepo_test

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/shopspring/decimal"

	"fingo/internal/adapter/mariadb/postingrepo"
	"fingo/internal/adapter/mariadb/testsupport"
	"fingo/internal/domain/gl"
	"fingo/internal/usecase/ledger"
)

var (
	at   = time.Date(2026, 3, 10, 9, 0, 0, 0, time.UTC)
	date = time.Date(2026, 3, 10, 0, 0, 0, 0, time.UTC)
)

// newStore opens a migrated scratch database with the TT133 chart in it, so the
// account codes a voucher names are the codes the database holds.
func newStore(t *testing.T) (*postingrepo.Store, *sql.DB) {
	t.Helper()

	cfg := testsupport.Config(t)
	admin := testsupport.Admin(t, cfg)
	db, _ := testsupport.MigratedDatabase(t, cfg, admin)
	testsupport.SeedChart(t, db)

	return postingrepo.NewStore(db), db
}

func voucher(t *testing.T, id string, lines ...gl.VoucherLine) *gl.Voucher {
	t.Helper()

	v := gl.NewVoucher(id, "Test voucher "+id, gl.VoucherTypeGeneral, date)
	for _, line := range lines {
		if err := v.AddLine(line); err != nil {
			t.Fatalf("AddLine on %s: %v", id, err)
		}
	}

	return v
}

func line(account string, side gl.EntrySide, amount int64) gl.VoucherLine {
	return gl.VoucherLine{AccountCode: account, Side: side, Amount: decimal.NewFromInt(amount)}
}

// stored reports whether a voucher row is there.
func stored(t *testing.T, db *sql.DB, id string) bool {
	t.Helper()

	var count int
	if err := db.QueryRow("SELECT COUNT(*) FROM vouchers WHERE id = ?", id).Scan(&count); err != nil {
		t.Fatalf("count vouchers: %v", err)
	}

	return count > 0
}

// linesOf counts a voucher's lines.
func linesOf(t *testing.T, db *sql.DB, id string) int {
	t.Helper()

	var count int
	if err := db.QueryRow("SELECT COUNT(*) FROM voucher_lines WHERE voucher_id = ?", id).Scan(&count); err != nil {
		t.Fatalf("count voucher lines: %v", err)
	}

	return count
}

// A sale and the cost of what it sold are one write. Both rows are there
// afterwards, and the request key still names the request.
func TestStore_SaveWritesTheCostVoucherInTheSameTransaction(t *testing.T) {
	store, db := newStore(t)
	ctx := context.Background()

	revenue := voucher(t, "sale-1",
		line("131", gl.SideDebit, 2000),
		line("5111", gl.SideCredit, 2000),
	)
	cost := voucher(t, "cogs-1",
		line("632", gl.SideDebit, 1500),
		line("156", gl.SideCredit, 1500),
	)

	posting := ledger.Posting{
		Voucher:    revenue,
		Cost:       cost,
		At:         at,
		By:         "operator-1",
		RequestKey: "request-1",
	}

	if err := store.Save(ctx, posting); err != nil {
		t.Fatalf("Save: %v", err)
	}

	if !stored(t, db, "sale-1") {
		t.Error("the sale was not written")
	}
	if !stored(t, db, "cogs-1") {
		t.Error("the cost of the sale was not written")
	}
	if got := linesOf(t, db, "cogs-1"); got != 2 {
		t.Errorf("the cost voucher has %d lines, want 2", got)
	}

	var voucherID string
	if err := db.QueryRow(
		"SELECT voucher_id FROM idempotency_keys WHERE key_value = ?", "request-1",
	).Scan(&voucherID); err != nil {
		t.Fatalf("read the request key: %v", err)
	}
	if voucherID != "sale-1" {
		t.Errorf("the request key names %s, want the sale it answered", voucherID)
	}

	// And the cost voucher did not claim a key of its own: one request is one
	// event, and the cost is half of it.
	var keys int
	if err := db.QueryRow("SELECT COUNT(*) FROM idempotency_keys").Scan(&keys); err != nil {
		t.Fatalf("count request keys: %v", err)
	}
	if keys != 1 {
		t.Errorf("request keys = %d, want the one the request carried", keys)
	}
}

// When the cost cannot be written, the sale it belongs to is not written either.
//
// This is the case the transaction exists for. The sale is written first, inside
// the transaction, and the cost fails afterwards — so a commit that ignored the
// failure would leave a sale on the books with its inventory account still
// carrying the goods, and profit overstated by exactly the cost.
func TestStore_SaveCommitsNeitherWhenTheCostCannotBeWritten(t *testing.T) {
	store, db := newStore(t)
	ctx := context.Background()

	revenue := voucher(t, "sale-2",
		line("131", gl.SideDebit, 2000),
		line("5111", gl.SideCredit, 2000),
	)

	// 9999 is not in the TT133 chart, so the cost voucher cannot be stored.
	cost := voucher(t, "cogs-2",
		line("632", gl.SideDebit, 1500),
		line("9999", gl.SideCredit, 1500),
	)

	err := store.Save(ctx, ledger.Posting{Voucher: revenue, Cost: cost, At: at, By: "operator-1"})
	if err == nil {
		t.Fatal("a cost naming an account that is not in the chart must be refused")
	}

	if stored(t, db, "sale-2") {
		t.Error("the sale was committed even though its cost was not: the books now overstate profit")
	}
	if stored(t, db, "cogs-2") {
		t.Error("the cost was committed even though it could not be written")
	}
	if got := linesOf(t, db, "sale-2"); got != 0 {
		t.Errorf("the refused sale left %d lines behind", got)
	}
}

// A posting with no cost is a posting: a service sale moves no stock, and there
// is nothing to write beside the revenue.
func TestStore_SaveWithoutACostWritesOnlyTheVoucher(t *testing.T) {
	store, db := newStore(t)
	ctx := context.Background()

	revenue := voucher(t, "sale-3",
		line("131", gl.SideDebit, 500),
		line("5111", gl.SideCredit, 500),
	)

	if err := store.Save(ctx, ledger.Posting{Voucher: revenue, At: at, By: "operator-1"}); err != nil {
		t.Fatalf("Save: %v", err)
	}

	if !stored(t, db, "sale-3") {
		t.Error("the sale was not written")
	}
}

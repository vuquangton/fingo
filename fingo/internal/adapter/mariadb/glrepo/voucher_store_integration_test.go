package glrepo_test

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/shopspring/decimal"

	"fingo/internal/adapter/mariadb/glrepo"
	"fingo/internal/adapter/mariadb/testsupport"
	"fingo/internal/domain/gl"
	"fingo/internal/domain/gl/gltest"
)

var voucherDate = time.Date(2026, 3, 14, 0, 0, 0, 0, time.UTC)

// postedAt is when these vouchers enter the ledger, distinct from the business
// date they carry.
var postedAt = time.Date(2026, 3, 14, 10, 30, 0, 0, time.UTC)

// testRules supplies the chart a voucher is posted against — the same chart
// testsupport.SeedChart writes into the scratch database.
func testRules(t *testing.T) gl.PostingRules {
	t.Helper()
	return gltest.TT133Rules(t)
}

func newStore(t *testing.T) (*glrepo.VoucherStore, *sql.DB) {
	t.Helper()

	cfg := testsupport.Config(t)
	admin := testsupport.Admin(t, cfg)
	db, _ := testsupport.MigratedDatabase(t, cfg, admin)
	testsupport.SeedChart(t, db)

	return glrepo.NewVoucherStore(db), db
}

// The whole point of the round: a posted VAT invoice survives a trip through
// MariaDB and comes back as the same voucher. Before this, no voucher could be
// stored at all.
func TestVoucherStore_RoundTripsAPostedVATInvoice(t *testing.T) {
	store, _ := newStore(t)
	ctx := context.Background()

	postedAt := time.Date(2026, 3, 14, 10, 30, 0, 0, time.UTC)

	v := gl.NewVoucher("v-1", "Bán hàng cho khách A", gl.VoucherTypeSales, voucherDate)
	mustAdd(t, v, "131", gl.SideDebit, 110)   // Nợ 131  — receivable
	mustAdd(t, v, "5111", gl.SideCredit, 100) // Có 5111 — revenue, net of VAT
	mustAdd(t, v, "33311", gl.SideCredit, 10) // Có 33311 — output VAT

	if err := v.Post(testRules(t), gltest.NumberFor(t, v, 1), postedAt, "operator-1"); err != nil {
		t.Fatalf("post: %v", err)
	}

	if err := store.SaveVoucher(ctx, v, ""); err != nil {
		t.Fatalf("save: %v", err)
	}

	got, err := store.GetVoucher(ctx, "v-1")
	if err != nil {
		t.Fatalf("get: %v", err)
	}

	if !got.IsPosted() {
		t.Fatal("voucher came back unposted")
	}
	if !got.PostedAt().Equal(postedAt) {
		t.Fatalf("PostedAt = %v, want %v", got.PostedAt(), postedAt)
	}
	if got.PostedBy() != "operator-1" {
		t.Fatalf("PostedBy = %q, want %q", got.PostedBy(), "operator-1")
	}
	if got.VoucherNo() != "HD2026-0001" || got.VoucherType() != gl.VoucherTypeSales {
		t.Fatalf("identity lost: no=%q type=%q", got.VoucherNo(), got.VoucherType())
	}

	// The lines must come back as account CODES, in order, on the right side.
	want := []struct {
		code   string
		side   gl.EntrySide
		amount int64
	}{
		{"131", gl.SideDebit, 110},
		{"5111", gl.SideCredit, 100},
		{"33311", gl.SideCredit, 10},
	}

	lines := got.Lines()
	if len(lines) != len(want) {
		t.Fatalf("lines = %d, want %d", len(lines), len(want))
	}
	for i, w := range want {
		if lines[i].AccountCode != w.code || lines[i].Side != w.side {
			t.Errorf("line %d = %s/%s, want %s/%s",
				i, lines[i].AccountCode, lines[i].Side, w.code, w.side)
		}
		if !lines[i].Amount.Equal(decimal.NewFromInt(w.amount)) {
			t.Errorf("line %d amount = %s, want %d", i, lines[i].Amount, w.amount)
		}
		if lines[i].LineOrder != i+1 {
			t.Errorf("line %d order = %d, want %d", i, lines[i].LineOrder, i+1)
		}
	}

	// Rehydration re-validates, so a successful read implies a balanced voucher.
	if err := got.ValidateBalance(); err != nil {
		t.Fatalf("rehydrated voucher does not balance: %v", err)
	}
}

// A draft must survive too, and must not come back posted.
func TestVoucherStore_RoundTripsADraft(t *testing.T) {
	store, _ := newStore(t)
	ctx := context.Background()

	v := gl.NewVoucher("v-2", "Thu tiền mặt", gl.VoucherTypeGeneral, voucherDate)
	mustAdd(t, v, "111", gl.SideDebit, 100)
	mustAdd(t, v, "511", gl.SideCredit, 100)

	if err := store.SaveVoucher(ctx, v, ""); err != nil {
		t.Fatalf("save: %v", err)
	}

	got, err := store.GetVoucher(ctx, "v-2")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.IsPosted() {
		t.Fatal("a stored draft came back posted")
	}
	if !got.PostedAt().IsZero() {
		t.Fatalf("a draft must carry no posting instant, got %v", got.PostedAt())
	}
}

// Atomicity. A voucher naming an account that is not in the chart must leave
// nothing behind — not a header, not a single line. A partial write would be an
// unbalanced voucher nothing could repair.
func TestVoucherStore_LeavesNothingBehindWhenALineFails(t *testing.T) {
	store, db := newStore(t)
	ctx := context.Background()

	v := gl.NewVoucher("v-3", "Bút toán sai", gl.VoucherTypeGeneral, voucherDate)
	mustAdd(t, v, "111", gl.SideDebit, 100)
	mustAdd(t, v, "9999", gl.SideCredit, 100) // not in the TT133 chart

	if err := store.SaveVoucher(ctx, v, ""); err == nil {
		t.Fatal("saving a voucher with an unknown account must fail, got nil")
	}

	var headers int
	if err := db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM vouchers WHERE id = ?", "v-3").Scan(&headers); err != nil {
		t.Fatalf("count vouchers: %v", err)
	}
	if headers != 0 {
		t.Errorf("vouchers left behind = %d, want 0 — the transaction did not roll back", headers)
	}

	var lines int
	if err := db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM voucher_lines WHERE voucher_id = ?", "v-3").Scan(&lines); err != nil {
		t.Fatalf("count lines: %v", err)
	}
	if lines != 0 {
		t.Errorf("voucher_lines left behind = %d, want 0", lines)
	}
}

// Two vouchers cannot take the same place in a book's run. A number is how a
// document is referred to, and a duplicate makes it ambiguous.
//
// Drafts are deliberately not covered: a draft holds no place in the run, which
// is why book/fiscal_year/seq are nullable — otherwise every draft would
// collide on (NULL, NULL, NULL) rather than coexisting.
func TestVoucherStore_RejectsADuplicateNumber(t *testing.T) {
	store, _ := newStore(t)
	ctx := context.Background()

	samePlace := gl.Number{Book: gl.BookGeneral, Year: 2026, Seq: 1}

	first := gl.NewVoucher("v-4", "Bút toán đầu", gl.VoucherTypeGeneral, voucherDate)
	mustAdd(t, first, "1111", gl.SideDebit, 100)
	mustAdd(t, first, "5111", gl.SideCredit, 100)
	if err := first.Post(testRules(t), samePlace, postedAt, "operator-1"); err != nil {
		t.Fatalf("first post: %v", err)
	}
	if err := store.SaveVoucher(ctx, first, ""); err != nil {
		t.Fatalf("first save: %v", err)
	}

	second := gl.NewVoucher("v-5", "Bút toán trùng số", gl.VoucherTypeGeneral, voucherDate)
	mustAdd(t, second, "1111", gl.SideDebit, 50)
	mustAdd(t, second, "5111", gl.SideCredit, 50)
	if err := second.Post(testRules(t), samePlace, postedAt, "operator-1"); err != nil {
		t.Fatalf("second post: %v", err)
	}

	if err := store.SaveVoucher(ctx, second, ""); err == nil {
		t.Fatal("two vouchers sharing a place in the run must be refused, got nil")
	}
}

// Two drafts coexist: neither holds a place in a run, so neither can collide.
func TestVoucherStore_AllowsManyDrafts(t *testing.T) {
	store, _ := newStore(t)
	ctx := context.Background()

	for _, id := range []string{"v-6", "v-7", "v-8"} {
		draft := gl.NewVoucher(id, "Bút toán nháp", gl.VoucherTypeGeneral, voucherDate)
		mustAdd(t, draft, "1111", gl.SideDebit, 100)
		mustAdd(t, draft, "5111", gl.SideCredit, 100)

		if err := store.SaveVoucher(ctx, draft, ""); err != nil {
			t.Fatalf("saving draft %s: %v", id, err)
		}
	}
}

func mustAdd(t *testing.T, v *gl.Voucher, code string, side gl.EntrySide, amount int64) {
	t.Helper()
	line := gl.VoucherLine{AccountCode: code, Side: side, Amount: decimal.NewFromInt(amount)}
	if err := v.AddLine(line); err != nil {
		t.Fatalf("AddLine(%s, %s, %d): %v", code, side, amount, err)
	}
}

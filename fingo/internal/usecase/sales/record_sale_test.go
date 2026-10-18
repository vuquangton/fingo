package sales_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/shopspring/decimal"

	"fingo/internal/domain/gl"
	"fingo/internal/domain/gl/gltest"
	"fingo/internal/domain/money"
	"fingo/internal/domain/tax"
	"fingo/internal/usecase/ledger"
	"fingo/internal/usecase/sales"
)

var (
	saleDate = time.Date(2026, 3, 14, 0, 0, 0, 0, time.UTC)
	postedAt = time.Date(2026, 3, 14, 10, 30, 0, 0, time.UTC)
)

// testRules supplies the chart a sale is posted against — the TT133 chart, so
// the accounts the use case names must genuinely exist and be postable.
func testRules(t *testing.T) gl.PostingRules {
	t.Helper()
	return gltest.TT133Rules(t)
}

// fakeLedger is the in-memory adapter at the ledger seam. Its existence is what
// makes the seam real rather than hypothetical: ledger.Service is the other
// adapter.
//
// It applies the same rules the real ledger does, so the vouchers the tests
// inspect are genuinely posted rather than merely recorded.
type fakeLedger struct {
	rules  gl.PostingRules
	posted []*gl.Voucher
	issues []ledger.StockIssue
	err    error
}

func newFakeLedger(t *testing.T) *fakeLedger {
	t.Helper()
	return &fakeLedger{rules: testRules(t)}
}

func (f *fakeLedger) Post(_ context.Context, p ledger.Posting) error {
	if f.err != nil {
		return f.err
	}

	book, err := gl.BookFor(p.Voucher.VoucherType())
	if err != nil {
		return err
	}
	number := gl.Number{Book: book, Year: p.Voucher.VoucherDate().UTC().Year(), Seq: 1}

	if err := p.Voucher.Post(f.rules, number, p.At, p.By); err != nil {
		return err
	}

	f.posted = append(f.posted, p.Voucher)
	f.issues = append(f.issues, p.StockIssues...)

	return nil
}

// fakeStock answers what goods cost without a warehouse. It stands at the
// StockSource seam; the real one reads the moving average.
type fakeStock struct {
	unitCost decimal.Decimal
	err      error
}

func (f fakeStock) CostOfIssue(
	_ context.Context,
	_, _ string,
	quantity decimal.Decimal,
) (decimal.Decimal, error) {
	if f.err != nil {
		return decimal.Zero, f.err
	}

	return money.LineNet(quantity, f.unitCost), nil
}

func lineFor(t *testing.T, v *gl.Voucher, code string, side gl.EntrySide) gl.VoucherLine {
	t.Helper()
	for _, l := range v.Lines() {
		if l.AccountCode == code && l.Side == side {
			return l
		}
	}
	t.Fatalf("voucher %s has no %s/%s line; lines: %+v", v.VoucherNo(), code, side, v.Lines())
	return gl.VoucherLine{}
}

func noLineFor(t *testing.T, v *gl.Voucher, code string, side gl.EntrySide) {
	t.Helper()
	for _, l := range v.Lines() {
		if l.AccountCode == code && l.Side == side {
			t.Fatalf("voucher %s must not have a %s/%s line", v.VoucherNo(), code, side)
		}
	}
}

func baseCommand(lines ...sales.SaleLine) sales.Command {
	return sales.Command{
		VoucherID:     "v-1",
		PartnerID:     "customer-1",
		WarehouseCode: "KHO1",
		Description:   "Bán hàng cho khách A",
		Date:          saleDate,
		At:            postedAt,
		Operator:      "operator-1",
		Lines:         lines,
	}
}

// A credit sale posts receivable / revenue / output VAT on one balanced
// voucher. This is the transaction the old double-entry shape could not hold at
// all — see docs/adr/0001-journal-line-model.md.
func TestRecordSale_PostsReceivableRevenueAndOutputVAT(t *testing.T) {
	vouchers := newFakeLedger(t)
	uc := sales.NewRecordSale(vouchers, fakeStock{unitCost: decimal.NewFromInt(60)}, tax.DefaultSchedule())

	res, err := uc.Execute(context.Background(), baseCommand(sales.SaleLine{
		ItemCode:  "VT01",
		Quantity:  decimal.NewFromInt(1),
		UnitPrice: decimal.NewFromInt(100),
		VAT:       tax.KindStandard,
	}))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	if !res.Net.Equal(decimal.NewFromInt(100)) ||
		!res.VAT.Equal(decimal.NewFromInt(10)) ||
		!res.Gross.Equal(decimal.NewFromInt(110)) {
		t.Fatalf("totals = net %s vat %s gross %s, want 100 / 10 / 110", res.Net, res.VAT, res.Gross)
	}

	if len(vouchers.posted) != 1 {
		t.Fatalf("saved vouchers = %d, want 1 — one business event, one voucher", len(vouchers.posted))
	}
	v := vouchers.posted[0]

	if !v.IsPosted() {
		t.Fatal("the sale voucher must be posted")
	}
	if v.PostedBy() != "operator-1" || !v.PostedAt().Equal(postedAt) {
		t.Fatalf("posting attribution lost: by=%q at=%v", v.PostedBy(), v.PostedAt())
	}
	if v.VoucherType() != gl.VoucherTypeSales {
		t.Fatalf("voucher type = %q, want SALES", v.VoucherType())
	}

	if got := lineFor(t, v, "131", gl.SideDebit).Amount; !got.Equal(decimal.NewFromInt(110)) {
		t.Errorf("Nợ 131 = %s, want 110", got)
	}
	if got := lineFor(t, v, "5111", gl.SideCredit).Amount; !got.Equal(decimal.NewFromInt(100)) {
		t.Errorf("Có 5111 = %s, want 100 (net of VAT)", got)
	}
	if got := lineFor(t, v, "33311", gl.SideCredit).Amount; !got.Equal(decimal.NewFromInt(10)) {
		t.Errorf("Có 33311 = %s, want 10", got)
	}

	// Cost of goods: the number a trading SME is managed on.
	if got := lineFor(t, v, "632", gl.SideDebit).Amount; !got.Equal(decimal.NewFromInt(60)) {
		t.Errorf("Nợ 632 = %s, want 60", got)
	}
	if got := lineFor(t, v, "156", gl.SideCredit).Amount; !got.Equal(decimal.NewFromInt(60)) {
		t.Errorf("Có 156 = %s, want 60", got)
	}

	// One voucher carrying both the revenue and the cost entry must still balance.
	if err := v.ValidateBalance(); err != nil {
		t.Fatalf("sale voucher does not balance: %v", err)
	}
}

// VND has no subunit. VAT is computed and rounded per line, so the invoice total
// is the sum of its rounded parts. Rounding the invoice total instead would give
// 33 here, not 34 — and would leave the invoice disagreeing with its own lines.
func TestRecordSale_RoundsVATPerLineNotPerInvoice(t *testing.T) {
	vouchers := newFakeLedger(t)
	uc := sales.NewRecordSale(vouchers, fakeStock{}, tax.DefaultSchedule())

	// 333 × 5% = 16.65 per line → 17 each.
	res, err := uc.Execute(context.Background(), baseCommand(
		sales.SaleLine{ItemCode: "A", Quantity: decimal.NewFromInt(1), UnitPrice: decimal.NewFromInt(333), VAT: tax.KindReduced},
		sales.SaleLine{ItemCode: "B", Quantity: decimal.NewFromInt(1), UnitPrice: decimal.NewFromInt(333), VAT: tax.KindReduced},
	))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	if !res.Net.Equal(decimal.NewFromInt(666)) {
		t.Fatalf("net = %s, want 666", res.Net)
	}
	if !res.VAT.Equal(decimal.NewFromInt(34)) {
		t.Fatalf("VAT = %s, want 34 (17 + 17, rounded per line; rounding the total would give 33)", res.VAT)
	}
	if !res.Gross.Equal(decimal.NewFromInt(700)) {
		t.Fatalf("gross = %s, want 700", res.Gross)
	}
}

// A fractional VAT that rounds to a whole đồng must not leave a fractional
// amount anywhere in the voucher — the ledger refuses those outright.
func TestRecordSale_ProducesOnlyWholeDong(t *testing.T) {
	vouchers := newFakeLedger(t)
	uc := sales.NewRecordSale(vouchers, fakeStock{}, tax.DefaultSchedule())

	_, err := uc.Execute(context.Background(), baseCommand(sales.SaleLine{
		ItemCode:  "VT01",
		Quantity:  decimal.NewFromInt(3),
		UnitPrice: decimal.NewFromInt(1234567),
		VAT:       tax.KindTemporarilyReduced,
	}))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	v := vouchers.posted[0]
	// 3 × 1,234,567 = 3,703,701 net; 8% = 296,296.08 → 296,296.
	if got := lineFor(t, v, "33311", gl.SideCredit).Amount; !got.Equal(decimal.NewFromInt(296296)) {
		t.Errorf("Có 33311 = %s, want 296296", got)
	}
	if got := lineFor(t, v, "131", gl.SideDebit).Amount; !got.Equal(decimal.NewFromInt(3999997)) {
		t.Errorf("Nợ 131 = %s, want 3999997", got)
	}
}

// A service sale has no cost of goods, so it must not post an empty cost entry.
func TestRecordSale_OmitsCostEntryWhenThereIsNoCost(t *testing.T) {
	vouchers := newFakeLedger(t)
	uc := sales.NewRecordSale(vouchers, fakeStock{}, tax.DefaultSchedule())

	res, err := uc.Execute(context.Background(), baseCommand(sales.SaleLine{
		ItemCode:  "DV01",
		Quantity:  decimal.NewFromInt(1),
		UnitPrice: decimal.NewFromInt(500),
		VAT:       tax.KindStandard,
	}))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !res.COGS.IsZero() {
		t.Fatalf("COGS = %s, want 0", res.COGS)
	}

	v := vouchers.posted[0]
	noLineFor(t, v, "632", gl.SideDebit)
	noLineFor(t, v, "156", gl.SideCredit)
	if err := v.ValidateBalance(); err != nil {
		t.Fatalf("service sale voucher does not balance: %v", err)
	}
}

// A zero-rated or non-taxable sale posts no output VAT line at all.
func TestRecordSale_OmitsVATLineWhenRateIsZero(t *testing.T) {
	vouchers := newFakeLedger(t)
	uc := sales.NewRecordSale(vouchers, fakeStock{}, tax.DefaultSchedule())

	_, err := uc.Execute(context.Background(), baseCommand(sales.SaleLine{
		ItemCode:  "XK01",
		Quantity:  decimal.NewFromInt(1),
		UnitPrice: decimal.NewFromInt(1000),
		VAT:       tax.KindZero,
	}))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	noLineFor(t, vouchers.posted[0], "33311", gl.SideCredit)
}

func TestRecordSale_RejectsMalformedCommands(t *testing.T) {
	cases := map[string]sales.Command{
		"no lines": baseCommand(),
		"zero quantity": baseCommand(sales.SaleLine{
			ItemCode: "A", Quantity: decimal.Zero, UnitPrice: decimal.NewFromInt(10),
		}),
		"negative quantity": baseCommand(sales.SaleLine{
			ItemCode: "A", Quantity: decimal.NewFromInt(-1), UnitPrice: decimal.NewFromInt(10),
		}),
		"negative price": baseCommand(sales.SaleLine{
			ItemCode: "A", Quantity: decimal.NewFromInt(1), UnitPrice: decimal.NewFromInt(-10),
		}),
		"unknown VAT treatment": baseCommand(sales.SaleLine{
			ItemCode: "A", Quantity: decimal.NewFromInt(1), UnitPrice: decimal.NewFromInt(10), VAT: tax.Kind("MYSTERY"),
		}),
		"no operator": func() sales.Command {
			c := baseCommand(sales.SaleLine{ItemCode: "A", Quantity: decimal.NewFromInt(1), UnitPrice: decimal.NewFromInt(10)})
			c.Operator = ""
			return c
		}(),
	}

	for name, cmd := range cases {
		t.Run(name, func(t *testing.T) {
			vouchers := newFakeLedger(t)
			uc := sales.NewRecordSale(vouchers, fakeStock{}, tax.DefaultSchedule())

			if _, err := uc.Execute(context.Background(), cmd); err == nil {
				t.Fatalf("%s must be rejected, got nil", name)
			}
			if len(vouchers.posted) != 0 {
				t.Fatalf("%s must not save a voucher, saved %d", name, len(vouchers.posted))
			}
		})
	}
}

// A storage failure must surface rather than be swallowed: the caller has to
// know the sale was not recorded.
func TestRecordSale_PropagatesRepositoryFailure(t *testing.T) {
	sentinel := errors.New("storage unavailable")
	vouchers := &fakeLedger{err: sentinel}
	uc := sales.NewRecordSale(vouchers, fakeStock{}, tax.DefaultSchedule())

	_, err := uc.Execute(context.Background(), baseCommand(sales.SaleLine{
		ItemCode: "A", Quantity: decimal.NewFromInt(1), UnitPrice: decimal.NewFromInt(10),
		VAT: tax.KindStandard,
	}))
	if !errors.Is(err, sentinel) {
		t.Fatalf("error = %v, want it to wrap %v", err, sentinel)
	}
}

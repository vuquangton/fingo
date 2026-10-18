package sales_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/shopspring/decimal"

	"fingo/internal/domain/tax"
	uisales "fingo/internal/ui/sales"
	"fingo/internal/usecase/sales"
)

var (
	day  = time.Date(2026, 3, 15, 9, 0, 0, 0, time.UTC)
	item = "HANG-A"
)

// schedule is the VAT treatments in force, as the application would be given
// them: 10% standard, 5% reduced, and an 8% window that closed in 2026.
func schedule() *tax.Schedule {
	return tax.NewSchedule([]tax.Rate{
		{Kind: tax.KindStandard, Percent: decimal.NewFromInt(10),
			From: time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)},
		{Kind: tax.KindReduced, Percent: decimal.NewFromInt(5),
			From: time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)},
		{Kind: tax.KindTemporarilyReduced, Percent: decimal.NewFromInt(8),
			From: time.Date(2025, 7, 1, 0, 0, 0, 0, time.UTC),
			To:   time.Date(2025, 12, 31, 0, 0, 0, 0, time.UTC)},
	})
}

// recorder stands in for the ledger.
type recorder struct {
	posted  []sales.Command
	result  sales.Result
	postErr error
}

func (r *recorder) RecordSale(_ context.Context, cmd sales.Command) (sales.Result, error) {
	r.posted = append(r.posted, cmd)

	return r.result, r.postErr
}

// filled is a form the operator has completed, ready to post.
func filled(t *testing.T) (*uisales.Model, *recorder) {
	t.Helper()

	rec := &recorder{result: sales.Result{VoucherNo: "BH00001"}}

	m := uisales.New(schedule(), rec, day)
	m.Party = "KH-001"
	m.Warehouse = "KHO1"
	m.Description = "Bán hàng cho khách A"
	m.Operator = "ketoan"

	m.AddLine(uisales.Line{
		ItemCode:  item,
		Quantity:  decimal.NewFromInt(10),
		UnitPrice: decimal.NewFromInt(100000),
		VAT:       tax.KindStandard,
	})

	return m, rec
}

// TestNew_OpensABlankFormDatedToday — the date is the one thing the operator
// should not have to remember, and every rate on the form depends on it.
func TestNew_OpensABlankFormDatedToday(t *testing.T) {
	m := uisales.New(schedule(), &recorder{}, day)

	if len(m.Lines()) != 0 {
		t.Errorf("a new form has %d lines, want none", len(m.Lines()))
	}
	if !m.Date.Equal(day) {
		t.Errorf("Date = %s, want %s", m.Date, day)
	}
}

// TestTotals_AddUpTheLinesTheWayTheLedgerWill — the operator has to see the
// numbers before they post, and the numbers they see have to be the numbers
// that get posted.
//
// So this does not do its own arithmetic: it calls the same money helpers the
// ledger does, including the rounding. VAT is rounded per line, which is what
// makes an invoice total the sum of its rounded parts rather than a second,
// slightly different number.
func TestTotals_AddUpTheLinesTheWayTheLedgerWill(t *testing.T) {
	m := uisales.New(schedule(), &recorder{}, day)

	m.AddLine(uisales.Line{
		ItemCode: item, Quantity: decimal.NewFromInt(10),
		UnitPrice: decimal.NewFromInt(100000), VAT: tax.KindStandard,
	})
	m.AddLine(uisales.Line{
		ItemCode: "HANG-B", Quantity: decimal.NewFromInt(4),
		UnitPrice: decimal.NewFromInt(250000), VAT: tax.KindReduced,
	})

	totals, err := m.Totals()
	if err != nil {
		t.Fatalf("Totals: %v", err)
	}

	// 10 x 100.000 = 1.000.000 net, 10% VAT  = 100.000
	//  4 x 250.000 = 1.000.000 net,  5% VAT  =  50.000
	if got := totals.Net.String(); got != "2000000" {
		t.Errorf("Net = %s, want 2000000", got)
	}
	if got := totals.VAT.String(); got != "150000" {
		t.Errorf("VAT = %s, want 150000", got)
	}
	if got := totals.Gross.String(); got != "2150000" {
		t.Errorf("Gross = %s, want 2150000", got)
	}
}

// TestTotals_RefuseARateThatWasNotInForce — a line names a treatment, not a
// percentage, and the percentage depends on when the document is dated. The 8%
// window closed on 31 December 2025.
//
// The dangerous alternative is to fall back to zero: the invoice would post with
// no output VAT at all, look perfectly reasonable, and understate the tax.
func TestTotals_RefuseARateThatWasNotInForce(t *testing.T) {
	m := uisales.New(schedule(), &recorder{}, day)

	m.AddLine(uisales.Line{
		ItemCode: item, Quantity: decimal.NewFromInt(1),
		UnitPrice: decimal.NewFromInt(100000), VAT: tax.KindTemporarilyReduced,
	})

	_, err := m.Totals()
	if err == nil {
		t.Fatal("Totals accepted a treatment that was not in force on the document's date")
	}
	if !strings.Contains(err.Error(), item) {
		t.Errorf("the refusal %q does not name the line it is about", err)
	}
}

// TestCommand_CarriesTheSameIdempotencyKeyEveryTime — the key names the request,
// not the attempt.
//
// An operator who does not see a response clicks again. If each call minted a
// new key, that second click would post a second sale: two receivables, twice
// the revenue, and a customer who owes double. The key is fixed when the form
// opens and reused until something succeeds.
func TestCommand_CarriesTheSameIdempotencyKeyEveryTime(t *testing.T) {
	m, _ := filled(t)

	first, err := m.Command()
	if err != nil {
		t.Fatalf("Command: %v", err)
	}
	if first.IdempotencyKey == "" {
		t.Fatal("the command carries no idempotency key, so a double click posts twice")
	}

	for i := 0; i < 3; i++ {
		again, err := m.Command()
		if err != nil {
			t.Fatalf("Command: %v", err)
		}
		if again.IdempotencyKey != first.IdempotencyKey {
			t.Fatalf("call %d minted a new key %q, want %q", i+2, again.IdempotencyKey, first.IdempotencyKey)
		}
	}
}

// TestCommand_CarriesWhatTheFormSays — the form's fields reach the ledger
// unaltered, including the date the VAT rate was resolved against.
func TestCommand_CarriesWhatTheFormSays(t *testing.T) {
	m, _ := filled(t)

	cmd, err := m.Command()
	if err != nil {
		t.Fatalf("Command: %v", err)
	}

	if cmd.PartnerID != "KH-001" {
		t.Errorf("PartnerID = %q", cmd.PartnerID)
	}
	if cmd.WarehouseCode != "KHO1" {
		t.Errorf("WarehouseCode = %q", cmd.WarehouseCode)
	}
	if cmd.Operator != "ketoan" {
		t.Errorf("Operator = %q", cmd.Operator)
	}
	if !cmd.Date.Equal(day) {
		t.Errorf("Date = %s, want %s", cmd.Date, day)
	}
	if len(cmd.Lines) != 1 || cmd.Lines[0].ItemCode != item {
		t.Errorf("Lines = %+v", cmd.Lines)
	}
	if len(cmd.Lines) != 1 || !cmd.Lines[0].Quantity.Equal(decimal.NewFromInt(10)) {
		t.Errorf("the line's quantity did not survive: %+v", cmd.Lines)
	}
}

// TestCommand_RefusesAnIncompleteFormWithReasonsTheOperatorCanActOn.
func TestCommand_RefusesAnIncompleteFormWithReasonsTheOperatorCanActOn(t *testing.T) {
	build := func(mutate func(*uisales.Model)) *uisales.Model {
		m, _ := filled(t)
		mutate(m)

		return m
	}

	tests := []struct {
		name    string
		mutate  func(*uisales.Model)
		mention string
	}{
		{"no customer", func(m *uisales.Model) { m.Party = "" }, "khách"},
		{"no warehouse", func(m *uisales.Model) { m.Warehouse = "" }, "kho"},
		{"no lines", func(m *uisales.Model) { m.RemoveLine(0) }, "dòng"},
		{"no description", func(m *uisales.Model) { m.Description = "" }, "diễn giải"},
		{"no operator", func(m *uisales.Model) { m.Operator = "" }, "người"},
		{"a zero price", func(m *uisales.Model) {
			m.RemoveLine(0)
			m.AddLine(uisales.Line{ItemCode: item, Quantity: decimal.NewFromInt(1),
				UnitPrice: decimal.Zero, VAT: tax.KindStandard})
		}, "đơn giá"},
		{"a zero quantity", func(m *uisales.Model) {
			m.RemoveLine(0)
			m.AddLine(uisales.Line{ItemCode: item, Quantity: decimal.Zero,
				UnitPrice: decimal.NewFromInt(1000), VAT: tax.KindStandard})
		}, "số lượng"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := build(tt.mutate)

			_, err := m.Command()
			if err == nil {
				t.Fatal("Command accepted an incomplete form")
			}

			if !strings.Contains(strings.ToLower(err.Error()), tt.mention) {
				t.Errorf("the refusal %q does not mention %q", err, tt.mention)
			}
		})
	}
}

// TestPost_RecordsAndOpensAFreshForm — after a sale is recorded the form is
// done with, and the next document is a new request with a new key. Reusing the
// success key would make the second sale vanish.
func TestPost_RecordsAndOpensAFreshForm(t *testing.T) {
	m, rec := filled(t)

	before, err := m.Command()
	if err != nil {
		t.Fatalf("Command: %v", err)
	}

	result, err := m.Post(t.Context())
	if err != nil {
		t.Fatalf("Post: %v", err)
	}
	if result.VoucherNo != "BH00001" {
		t.Errorf("Post returned %+v", result)
	}
	if len(rec.posted) != 1 {
		t.Fatalf("the ledger was asked to record %d sales, want 1", len(rec.posted))
	}

	// The form is blank again, with a new key.
	if len(m.Lines()) != 0 {
		t.Errorf("the form still holds %d lines after posting", len(m.Lines()))
	}

	m.Party = "KH-002"
	m.Warehouse = "KHO1"
	m.Description = "Bán cho khách B"
	m.Operator = "ketoan"
	m.AddLine(uisales.Line{ItemCode: item, Quantity: decimal.NewFromInt(1),
		UnitPrice: decimal.NewFromInt(100000), VAT: tax.KindStandard})

	after, err := m.Command()
	if err != nil {
		t.Fatalf("Command after posting: %v", err)
	}
	if after.IdempotencyKey == before.IdempotencyKey {
		t.Error("the second document reused the first one's key, so the ledger would ignore it")
	}
}

// TestPost_KeepsTheFormWhenTheLedgerRefuses — a refusal is not a reason to make
// the operator type the invoice again. And the key stays, so a retry of the same
// document is still the same request.
func TestPost_KeepsTheFormWhenTheLedgerRefuses(t *testing.T) {
	m, rec := filled(t)
	rec.postErr = errors.New("the ledger is not reachable")

	before, _ := m.Command()

	if _, err := m.Post(t.Context()); err == nil {
		t.Fatal("Post reported success while the ledger refused")
	}

	if len(m.Lines()) != 1 {
		t.Errorf("the form lost its lines: %d left", len(m.Lines()))
	}
	if m.Party != "KH-001" {
		t.Errorf("the form lost its customer: %q", m.Party)
	}

	after, _ := m.Command()
	if after.IdempotencyKey != before.IdempotencyKey {
		t.Error("a failed post minted a new key, so retrying would post a second sale")
	}
}

// TestPost_DoesNotCallTheLedgerWithAnInvalidForm — the refusal belongs on the
// screen, not in a database error the operator cannot act on.
func TestPost_DoesNotCallTheLedgerWithAnInvalidForm(t *testing.T) {
	m, rec := filled(t)
	m.Party = ""

	if _, err := m.Post(t.Context()); err == nil {
		t.Fatal("Post accepted a form with no customer")
	}
	if len(rec.posted) != 0 {
		t.Errorf("the ledger was asked to record %d sales from an invalid form", len(rec.posted))
	}
}

// TestRemoveLine_TakesTheRightOneOut — removing "the last one" and removing
// "index 0" are different, and a grid that deletes the wrong row is a grid that
// loses an invoice line.
func TestRemoveLine_TakesTheRightOneOut(t *testing.T) {
	m := uisales.New(schedule(), &recorder{}, day)

	for _, code := range []string{"A", "B", "C"} {
		m.AddLine(uisales.Line{ItemCode: code, Quantity: decimal.NewFromInt(1),
			UnitPrice: decimal.NewFromInt(1000), VAT: tax.KindStandard})
	}

	m.RemoveLine(1)

	var left []string
	for _, l := range m.Lines() {
		left = append(left, l.ItemCode)
	}

	if strings.Join(left, ",") != "A,C" {
		t.Errorf("after removing index 1 the form holds %v, want A,C", left)
	}

	// Out of range is ignored rather than a panic: a grid can fire a remove for
	// a row that is already gone.
	m.RemoveLine(99)
	m.RemoveLine(-1)

	if len(m.Lines()) != 2 {
		t.Errorf("an out-of-range remove changed the form: %d lines", len(m.Lines()))
	}
}

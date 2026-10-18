package trading_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	arap "fingo/internal/document"
	"fingo/internal/domain/gl"
	"fingo/internal/money"
	"fingo/internal/posting"
	"fingo/internal/trading"
	"fingo/internal/vat"
)

var vnd = money.Currency("VND")

func cents(amount int64) money.Money { return money.New(amount, vnd) }

func day(year int, month time.Month, dayOfMonth int) time.Time {
	return time.Date(year, month, dayOfMonth, 0, 0, 0, 0, time.UTC)
}

var (
	saleDate    = day(2026, 3, 10)
	paymentDate = day(2026, 3, 20)
	postedAt    = time.Date(2026, 3, 10, 9, 0, 0, 0, time.UTC)
)

// vatRates is a rate table with the three rates the tests price at.
//
// The effective dates ascend because the registry only appends: a rate is added
// after the ones already in force, which is what stops two rates claiming the
// same date.
func vatRates(t *testing.T) *vat.Registry {
	t.Helper()

	registry := vat.New()

	for _, rate := range []vat.VATRate{
		{ID: "vat-10", Rate: 1000, EffectiveFrom: day(2020, 1, 1), Active: true},
		{ID: "vat-8", Rate: 800, EffectiveFrom: day(2021, 1, 1), Active: true},
		{ID: "vat-0", Rate: 0, EffectiveFrom: day(2022, 1, 1), Active: true},
	} {
		if err := registry.Create(rate); err != nil {
			t.Fatalf("Create %s: %v", rate.ID, err)
		}
	}

	return registry
}

func rules(t *testing.T) *posting.RuleSet {
	t.Helper()

	loaded, err := posting.Load(posting.TT133Defaults())
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	return loaded
}

// service wires the flow over an in-memory receivables book and a poster that
// keeps what it was handed.
func service(t *testing.T) (*trading.Service, *recordingPoster, arap.Repository) {
	t.Helper()

	receivables := arap.NewMemoryRepository()
	poster := &recordingPoster{}

	flow, err := trading.NewService(receivables, poster, vatRates(t), rules(t))
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}

	return flow, poster, receivables
}

// line is one sale line.
func line(item string, qty int, unitPrice int64, rate string) trading.InvoiceLine {
	return trading.InvoiceLine{
		ItemID:    item,
		Quantity:  qty,
		UnitPrice: cents(unitPrice),
		VATRateID: rate,
	}
}

// invoiceCmd is a sale of the given lines.
func invoiceCmd(id string, lines ...trading.InvoiceLine) trading.InvoiceCommand {
	return trading.InvoiceCommand{
		InvoiceID:   id,
		PartnerID:   "p-1",
		Date:        saleDate,
		PeriodID:    "fy2026",
		Description: "Bán hàng cho khách A",
		Lines:       lines,
		At:          postedAt,
		By:          "operator-1",
		RequestKey:  "request-" + id,
	}
}

// recordingPoster stands in for the ledger.
type recordingPoster struct {
	mu       sync.Mutex
	vouchers []*gl.Voucher
	fail     error
}

func (p *recordingPoster) Post(_ context.Context, posting trading.Posting) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.fail != nil {
		return p.fail
	}

	p.vouchers = append(p.vouchers, posting.Voucher)

	return nil
}

func (p *recordingPoster) posted() []*gl.Voucher {
	p.mu.Lock()
	defer p.mu.Unlock()

	return append([]*gl.Voucher(nil), p.vouchers...)
}

// voucher finds a posted voucher by id.
func (p *recordingPoster) voucher(t *testing.T, id string) *gl.Voucher {
	t.Helper()

	for _, voucher := range p.posted() {
		if voucher.ID() == id {
			return voucher
		}
	}

	t.Fatalf("no voucher %s was posted; posted: %v", id, p.ids())

	return nil
}

func (p *recordingPoster) ids() []string {
	var out []string
	for _, voucher := range p.posted() {
		out = append(out, voucher.ID())
	}

	return out
}

// lineFor returns the line on an account.
func lineFor(t *testing.T, voucher *gl.Voucher, account string) gl.VoucherLine {
	t.Helper()

	for _, line := range voucher.Lines() {
		if line.AccountCode == account {
			return line
		}
	}

	t.Fatalf("voucher %s has no line on %s: %+v", voucher.ID(), account, voucher.Lines())

	return gl.VoucherLine{}
}

// The whole flow: a sale is invoiced, the customer pays it, and the invoice says
// so.
func TestFlow_InvoiceThenPaymentLeavesTheInvoicePaid(t *testing.T) {
	flow, poster, receivables := service(t)
	ctx := context.Background()

	// Ten units at 100, taxed at 10%: 1000 + 100.
	invoice, err := flow.Invoice(ctx, invoiceCmd("inv-1", line("item-1", 10, 100, "vat-10")))
	if err != nil {
		t.Fatalf("Invoice: %v", err)
	}

	if invoice.Base.Amount() != 1000 {
		t.Errorf("base = %s, want 1000", invoice.Base)
	}
	if invoice.VAT.Amount() != 100 {
		t.Errorf("VAT = %s, want 100", invoice.VAT)
	}
	if invoice.Total.Amount() != 1100 {
		t.Errorf("total = %s, want 1100", invoice.Total)
	}

	// The invoice is recorded against the partner, open, for what it is worth.
	recorded, err := receivables.Get(ctx, "inv-1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if recorded.Type != arap.TypeInvoice || recorded.PartnerID != "p-1" {
		t.Errorf("recorded = %+v, want an invoice owed by p-1", recorded)
	}
	if recorded.Total.Amount() != 1100 || recorded.Status != arap.StatusOpen {
		t.Errorf("recorded = %s/%s, want 1100 open", recorded.Total, recorded.Status)
	}
	if recorded.VATRateID != "vat-10" {
		t.Errorf("VAT rate = %q, want the one every line was taxed at", recorded.VATRateID)
	}

	// And the customer pays it.
	payment, err := flow.Pay(ctx, trading.PaymentCommand{
		PaymentID:   "pay-1",
		InvoiceID:   "inv-1",
		Amount:      cents(1100),
		Account:     "112",
		Date:        paymentDate,
		Description: "Khách A trả tiền",
		At:          postedAt,
		By:          "operator-1",
		RequestKey:  "request-pay-1",
	})
	if err != nil {
		t.Fatalf("Pay: %v", err)
	}

	if payment.Status != arap.StatusPaid {
		t.Errorf("status = %s, want paid", payment.Status)
	}
	if payment.Applied.Amount() != 1100 {
		t.Errorf("applied = %s, want the whole 1100", payment.Applied)
	}
	if !payment.Unapplied.IsZero() {
		t.Errorf("unapplied = %s, want nothing", payment.Unapplied)
	}

	settled, err := receivables.Get(ctx, "inv-1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if settled.Status != arap.StatusPaid {
		t.Errorf("the stored invoice is %s, want paid", settled.Status)
	}
	if settled.Allocated.Amount() != 1100 {
		t.Errorf("allocated = %s, want 1100", settled.Allocated)
	}
	if !settled.Outstanding().IsZero() {
		t.Errorf("outstanding = %s, want nothing", settled.Outstanding())
	}

	// Two entries and no more: the sale, and the money that settled it.
	posted := poster.ids()
	if len(posted) != 2 || posted[0] != "invoice-inv-1" || posted[1] != "payment-pay-1" {
		t.Errorf("posted %v, want the revenue voucher and the cash voucher", posted)
	}
}

// Every voucher the flow posts balances, and the revenue entry says what the
// sale was worth on each of the three accounts it touches.
func TestFlow_TheRevenueVoucherBalancesAndPostsWhatWasSold(t *testing.T) {
	flow, poster, _ := service(t)
	ctx := context.Background()

	if _, err := flow.Invoice(ctx, invoiceCmd("inv-1", line("item-1", 10, 100, "vat-10"))); err != nil {
		t.Fatalf("Invoice: %v", err)
	}

	voucher := poster.voucher(t, "invoice-inv-1")

	if err := voucher.ValidateBalance(); err != nil {
		t.Fatalf("the revenue voucher does not balance: %v", err)
	}

	receivable := lineFor(t, voucher, "131")
	if receivable.Side != gl.SideDebit || receivable.Amount.String() != "1100" {
		t.Errorf("131 = %s %s, want a debit of 1100", receivable.Side, receivable.Amount)
	}
	if receivable.PartnerID != "p-1" {
		t.Errorf("the receivable names partner %q, want the customer who owes it", receivable.PartnerID)
	}

	revenue := lineFor(t, voucher, "5111")
	if revenue.Side != gl.SideCredit || revenue.Amount.String() != "1000" {
		t.Errorf("5111 = %s %s, want a credit of 1000", revenue.Side, revenue.Amount)
	}

	vatOut := lineFor(t, voucher, "33311")
	if vatOut.Side != gl.SideCredit || vatOut.Amount.String() != "100" {
		t.Errorf("33311 = %s %s, want a credit of 100", vatOut.Side, vatOut.Amount)
	}
}

// The cash voucher is the money arriving: the bank debited, the receivable
// cleared.
func TestFlow_ThePaymentVoucherBalancesAndClearsTheReceivable(t *testing.T) {
	flow, poster, _ := service(t)
	ctx := context.Background()

	if _, err := flow.Invoice(ctx, invoiceCmd("inv-1", line("item-1", 10, 100, "vat-10"))); err != nil {
		t.Fatalf("Invoice: %v", err)
	}
	if _, err := flow.Pay(ctx, trading.PaymentCommand{
		PaymentID: "pay-1", InvoiceID: "inv-1", Amount: cents(1100), Account: "112",
		Date: paymentDate, At: postedAt, By: "operator-1",
	}); err != nil {
		t.Fatalf("Pay: %v", err)
	}

	voucher := poster.voucher(t, "payment-pay-1")

	if err := voucher.ValidateBalance(); err != nil {
		t.Fatalf("the cash voucher does not balance: %v", err)
	}

	bank := lineFor(t, voucher, "112")
	if bank.Side != gl.SideDebit || bank.Amount.String() != "1100" {
		t.Errorf("112 = %s %s, want a debit of 1100", bank.Side, bank.Amount)
	}

	receivable := lineFor(t, voucher, "131")
	if receivable.Side != gl.SideCredit || receivable.Amount.String() != "1100" {
		t.Errorf("131 = %s %s, want a credit of 1100", receivable.Side, receivable.Amount)
	}

	// The receipt is numbered in the cash book, not the invoice book.
	if voucher.VoucherType() != gl.VoucherTypeCashReceipt {
		t.Errorf("voucher type = %s, want a cash receipt", voucher.VoucherType())
	}
}

// A credit note mirrors what the invoice recorded, and cancels it to the minor
// unit.
func TestFlow_ACreditNoteReversesTheInvoice(t *testing.T) {
	flow, poster, receivables := service(t)
	ctx := context.Background()

	if _, err := flow.Invoice(ctx, invoiceCmd("inv-1", line("item-1", 10, 100, "vat-10"))); err != nil {
		t.Fatalf("Invoice: %v", err)
	}

	credit, err := flow.Credit(ctx, trading.CreditCommand{
		CreditID:    "cn-1",
		InvoiceID:   "inv-1",
		Date:        day(2026, 3, 25),
		Description: "Khách trả lại hàng",
		At:          postedAt,
		By:          "operator-1",
		RequestKey:  "request-cn-1",
	})
	if err != nil {
		t.Fatalf("Credit: %v", err)
	}

	if credit.Amount.Amount() != 1100 {
		t.Errorf("the credit note is worth %s, want the invoice's 1100", credit.Amount)
	}

	voucher := poster.voucher(t, "credit-cn-1")

	if err := voucher.ValidateBalance(); err != nil {
		t.Fatalf("the credit note does not balance: %v", err)
	}

	// Every side is the other way round from the invoice.
	revenue := lineFor(t, voucher, "5111")
	if revenue.Side != gl.SideDebit || revenue.Amount.String() != "1000" {
		t.Errorf("5111 = %s %s, want a debit of 1000 reversing the credit", revenue.Side, revenue.Amount)
	}

	vatOut := lineFor(t, voucher, "33311")
	if vatOut.Side != gl.SideDebit || vatOut.Amount.String() != "100" {
		t.Errorf("33311 = %s %s, want a debit of 100 reversing the credit", vatOut.Side, vatOut.Amount)
	}

	receivable := lineFor(t, voucher, "131")
	if receivable.Side != gl.SideCredit || receivable.Amount.String() != "1100" {
		t.Errorf("131 = %s %s, want a credit of 1100 reversing the debit", receivable.Side, receivable.Amount)
	}

	// And the invoice is no longer owed: what it recorded has been reversed.
	settled, err := receivables.Get(ctx, "inv-1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if settled.Status != arap.StatusPaid {
		t.Errorf("status = %s, want the credit to have settled the invoice", settled.Status)
	}
	if !settled.Outstanding().IsZero() {
		t.Errorf("outstanding = %s after a full credit note, want nothing", settled.Outstanding())
	}

	// The reversal is built from what was recorded, so the two net to nothing
	// on every account the invoice touched.
	invoice := poster.voucher(t, "invoice-inv-1")
	for _, account := range []string{"131", "5111", "33311"} {
		original := lineFor(t, invoice, account)
		reversal := lineFor(t, voucher, account)

		if !original.Amount.Equal(reversal.Amount) {
			t.Errorf("%s: the invoice moved %s and the credit note %s", account, original.Amount, reversal.Amount)
		}
		if original.Side == reversal.Side {
			t.Errorf("%s: the credit note did not reverse the side", account)
		}
	}
}

// Part of the money leaves the invoice partly paid, and the rest is still owed.
func TestFlow_APartialPaymentLeavesTheInvoicePartlyPaid(t *testing.T) {
	flow, _, receivables := service(t)
	ctx := context.Background()

	if _, err := flow.Invoice(ctx, invoiceCmd("inv-1", line("item-1", 10, 100, "vat-10"))); err != nil {
		t.Fatalf("Invoice: %v", err)
	}

	payment, err := flow.Pay(ctx, trading.PaymentCommand{
		PaymentID: "pay-1", InvoiceID: "inv-1", Amount: cents(500), Account: "111",
		Date: paymentDate, At: postedAt, By: "operator-1",
	})
	if err != nil {
		t.Fatalf("Pay: %v", err)
	}

	if payment.Status != arap.StatusPartiallyPaid {
		t.Errorf("status = %s, want partially_paid", payment.Status)
	}
	if payment.Applied.Amount() != 500 {
		t.Errorf("applied = %s, want 500", payment.Applied)
	}

	recorded, err := receivables.Get(ctx, "inv-1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got := recorded.Outstanding().Amount(); got != 600 {
		t.Errorf("outstanding = %d, want 600", got)
	}
}

// Paying more than is owed is not an error: the difference becomes a credit the
// partner holds.
func TestFlow_AnOverpaymentBecomesACreditBalance(t *testing.T) {
	flow, _, receivables := service(t)
	ctx := context.Background()

	if _, err := flow.Invoice(ctx, invoiceCmd("inv-1", line("item-1", 10, 100, "vat-10"))); err != nil {
		t.Fatalf("Invoice: %v", err)
	}

	payment, err := flow.Pay(ctx, trading.PaymentCommand{
		PaymentID: "pay-1", InvoiceID: "inv-1", Amount: cents(1500), Account: "112",
		Date: paymentDate, At: postedAt, By: "operator-1",
	})
	if err != nil {
		t.Fatalf("an overpayment must not be refused: %v", err)
	}

	if payment.Applied.Amount() != 1100 {
		t.Errorf("applied = %s, want the 1100 the invoice was worth", payment.Applied)
	}
	if payment.Unapplied.Amount() != 400 {
		t.Errorf("unapplied = %s, want the 400 no invoice could absorb", payment.Unapplied)
	}
	if payment.Status != arap.StatusPaid {
		t.Errorf("status = %s, want paid", payment.Status)
	}

	credits, err := receivables.Credits(ctx, "p-1")
	if err != nil {
		t.Fatalf("Credits: %v", err)
	}
	if len(credits) != 1 || credits[0].Amount.Amount() != 400 {
		t.Fatalf("credits = %+v, want one of 400", credits)
	}
}

// Each line is taxed at its own rate, and the totals are the sum of what the
// lines came to.
func TestFlow_EachLineIsTaxedAtItsOwnRate(t *testing.T) {
	flow, poster, receivables := service(t)
	ctx := context.Background()

	invoice, err := flow.Invoice(ctx, invoiceCmd("inv-1",
		line("item-1", 2, 100, "vat-10"), // 200 + 20
		line("item-2", 1, 500, "vat-8"),  // 500 + 40
		line("item-3", 3, 50, "vat-0"),   // 150 + 0
	))
	if err != nil {
		t.Fatalf("Invoice: %v", err)
	}

	if invoice.Base.Amount() != 850 {
		t.Errorf("base = %s, want 200 + 500 + 150", invoice.Base)
	}
	if invoice.VAT.Amount() != 60 {
		t.Errorf("VAT = %s, want 20 + 40 + 0", invoice.VAT)
	}
	if invoice.Total.Amount() != 910 {
		t.Errorf("total = %s, want 850 + 60", invoice.Total)
	}

	// The total is the sum of the lines, not a figure computed beside them.
	var summed int64
	for _, priced := range invoice.Lines {
		summed += priced.Total.Amount()
	}
	if summed != invoice.Total.Amount() {
		t.Errorf("the lines come to %d and the invoice to %d", summed, invoice.Total.Amount())
	}

	if err := poster.voucher(t, "invoice-inv-1").ValidateBalance(); err != nil {
		t.Errorf("the voucher does not balance: %v", err)
	}

	// An invoice with three rates on it has no single rate, and claims none.
	recorded, err := receivables.Get(ctx, "inv-1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if recorded.VATRateID != "" {
		t.Errorf("VAT rate = %q, want none: the lines were taxed at different rates", recorded.VATRateID)
	}
}

// A zero-rated sale carries no output VAT, and writes no VAT line.
func TestFlow_AZeroRatedSaleWritesNoVATLine(t *testing.T) {
	flow, poster, _ := service(t)
	ctx := context.Background()

	invoice, err := flow.Invoice(ctx, invoiceCmd("inv-1", line("item-1", 4, 250, "vat-0")))
	if err != nil {
		t.Fatalf("Invoice: %v", err)
	}

	if !invoice.VAT.IsZero() {
		t.Errorf("VAT = %s, want nothing", invoice.VAT)
	}
	if invoice.Total.Amount() != 1000 {
		t.Errorf("total = %s, want 1000", invoice.Total)
	}

	voucher := poster.voucher(t, "invoice-inv-1")
	if err := voucher.ValidateBalance(); err != nil {
		t.Fatalf("the voucher does not balance: %v", err)
	}
	if lines := voucher.Lines(); len(lines) != 2 {
		t.Errorf("the voucher has %d lines, want the receivable and the revenue only", len(lines))
	}
}

// The voucher ids are derived from what they record, so a retried request
// collides with the row it already wrote instead of posting twice.
func TestFlow_VoucherIdsAreDerivedSoARetryCollides(t *testing.T) {
	flow, poster, _ := service(t)
	ctx := context.Background()

	if _, err := flow.Invoice(ctx, invoiceCmd("inv-1", line("item-1", 1, 100, "vat-10"))); err != nil {
		t.Fatalf("Invoice: %v", err)
	}
	if _, err := flow.Pay(ctx, trading.PaymentCommand{
		PaymentID: "pay-1", InvoiceID: "inv-1", Amount: cents(110), Account: "112",
		Date: paymentDate, At: postedAt, By: "operator-1",
	}); err != nil {
		t.Fatalf("Pay: %v", err)
	}
	if _, err := flow.Credit(ctx, trading.CreditCommand{
		CreditID: "cn-1", InvoiceID: "inv-1", Date: day(2026, 3, 25), At: postedAt, By: "operator-1",
	}); err != nil {
		t.Fatalf("Credit: %v", err)
	}

	want := []string{"invoice-inv-1", "payment-pay-1", "credit-cn-1"}

	posted := poster.ids()
	if len(posted) != len(want) {
		t.Fatalf("posted %v, want %v", posted, want)
	}
	for i, id := range want {
		if posted[i] != id {
			t.Errorf("voucher %d = %q, want %q", i, posted[i], id)
		}
	}
}

// A flow that cannot post posts nothing, and records nothing.
func TestFlow_PostsNothingWhenTheLedgerRefuses(t *testing.T) {
	receivables := arap.NewMemoryRepository()
	poster := &recordingPoster{fail: errors.New("the period is closed")}

	flow, err := trading.NewService(receivables, poster, vatRates(t), rules(t))
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}

	ctx := context.Background()

	if _, err := flow.Invoice(ctx, invoiceCmd("inv-1", line("item-1", 1, 100, "vat-10"))); err == nil {
		t.Fatal("a refused posting must be reported")
	}

	if _, err := receivables.Get(ctx, "inv-1"); !errors.Is(err, arap.ErrUnknownDocument) {
		t.Errorf("the invoice was recorded anyway: %v", err)
	}
}

// A line priced at nothing, or beyond what an amount can hold, is refused before
// anything is posted.
func TestFlow_RefusesAnInvoiceItCannotPrice(t *testing.T) {
	flow, poster, receivables := service(t)
	ctx := context.Background()

	cases := map[string]trading.InvoiceCommand{
		"no id":                           {PartnerID: "p-1", Date: saleDate, PeriodID: "fy2026", Lines: []trading.InvoiceLine{line("i", 1, 100, "vat-10")}},
		"no partner":                      {InvoiceID: "inv-1", Date: saleDate, PeriodID: "fy2026", Lines: []trading.InvoiceLine{line("i", 1, 100, "vat-10")}},
		"no date":                         {InvoiceID: "inv-1", PartnerID: "p-1", PeriodID: "fy2026", Lines: []trading.InvoiceLine{line("i", 1, 100, "vat-10")}},
		"no period":                       {InvoiceID: "inv-1", PartnerID: "p-1", Date: saleDate, Lines: []trading.InvoiceLine{line("i", 1, 100, "vat-10")}},
		"no lines":                        {InvoiceID: "inv-1", PartnerID: "p-1", Date: saleDate, PeriodID: "fy2026"},
		"no item":                         invoiceCmd("inv-1", line("", 1, 100, "vat-10")),
		"no quantity":                     invoiceCmd("inv-1", line("i", 0, 100, "vat-10")),
		"no VAT rate":                     invoiceCmd("inv-1", line("i", 1, 100, "")),
		"an unknown VAT rate":             invoiceCmd("inv-1", line("i", 1, 100, "vat-99")),
		"a price of nothing":              invoiceCmd("inv-1", line("i", 1, 0, "vat-10")),
		"a price beyond what can be held": invoiceCmd("inv-1", line("i", 2, 1<<62, "vat-10")),
	}

	for name, cmd := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := flow.Invoice(ctx, cmd); err == nil {
				t.Fatal("the invoice must be refused")
			}
			if len(poster.posted()) != 0 {
				t.Errorf("a refused invoice posted %v", poster.ids())
			}
		})
	}

	listed, err := receivables.List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(listed) != 0 {
		t.Errorf("recorded %d invoices after every one was refused, want none", len(listed))
	}
}

// A payment that cannot be taken is refused, and an invoice that does not exist
// cannot be paid.
func TestFlow_RefusesAPaymentItCannotTake(t *testing.T) {
	flow, poster, _ := service(t)
	ctx := context.Background()

	if _, err := flow.Invoice(ctx, invoiceCmd("inv-1", line("item-1", 1, 100, "vat-10"))); err != nil {
		t.Fatalf("Invoice: %v", err)
	}
	posted := len(poster.posted())

	cases := map[string]trading.PaymentCommand{
		"no id":             {InvoiceID: "inv-1", Amount: cents(100), Account: "112"},
		"no invoice":        {PaymentID: "pay-1", Amount: cents(100), Account: "112"},
		"nothing at all":    {PaymentID: "pay-1", InvoiceID: "inv-1", Amount: cents(0), Account: "112"},
		"no currency":       {PaymentID: "pay-1", InvoiceID: "inv-1", Amount: money.Money{}, Account: "112"},
		"nowhere to arrive": {PaymentID: "pay-1", InvoiceID: "inv-1", Amount: cents(100)},
		"another currency":  {PaymentID: "pay-1", InvoiceID: "inv-1", Amount: money.New(100, money.Currency("USD")), Account: "112"},
	}

	for name, cmd := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := flow.Pay(ctx, cmd); err == nil {
				t.Fatal("the payment must be refused")
			}
		})
	}

	if got := len(poster.posted()); got != posted {
		t.Errorf("a refused payment posted a voucher: %d vouchers, want %d", got, posted)
	}

	// And an invoice nobody issued cannot be paid.
	if _, err := flow.Pay(ctx, trading.PaymentCommand{
		PaymentID: "pay-9", InvoiceID: "inv-9", Amount: cents(100), Account: "112",
		Date: paymentDate, At: postedAt, By: "operator-1",
	}); !errors.Is(err, arap.ErrUnknownDocument) {
		t.Errorf("paying an invoice that does not exist = %v, want ErrUnknownDocument", err)
	}
}

// A credit note for an invoice nothing was recorded against is refused.
func TestFlow_RefusesACreditNoteForAnInvoiceItCannotRead(t *testing.T) {
	flow, _, _ := service(t)
	ctx := context.Background()

	if _, err := flow.Credit(ctx, trading.CreditCommand{
		CreditID: "cn-1", InvoiceID: "inv-9", Date: day(2026, 3, 25), At: postedAt, By: "operator-1",
	}); err == nil {
		t.Fatal("crediting an invoice that does not exist must be refused")
	}

	if _, err := flow.Credit(ctx, trading.CreditCommand{
		CreditID: "", InvoiceID: "inv-1", Date: day(2026, 3, 25), At: postedAt, By: "operator-1",
	}); err == nil {
		t.Fatal("a credit note with no id must be refused")
	}
	if _, err := flow.Credit(ctx, trading.CreditCommand{
		CreditID: "cn-1", InvoiceID: "inv-1", At: postedAt, By: "operator-1",
	}); err == nil {
		t.Fatal("a credit note with no date must be refused")
	}
}

func TestNewService_NeedsEveryPart(t *testing.T) {
	receivables := arap.NewMemoryRepository()
	poster := &recordingPoster{}
	rates := vatRates(t)
	loaded := rules(t)

	cases := map[string]func() (*trading.Service, error){
		"no receivables": func() (*trading.Service, error) { return trading.NewService(nil, poster, rates, loaded) },
		"no poster":      func() (*trading.Service, error) { return trading.NewService(receivables, nil, rates, loaded) },
		"no VAT rates":   func() (*trading.Service, error) { return trading.NewService(receivables, poster, nil, loaded) },
		"no rules":       func() (*trading.Service, error) { return trading.NewService(receivables, poster, rates, nil) },
	}

	for name, build := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := build(); err == nil {
				t.Fatal("the flow must refuse to be built without it")
			}
		})
	}
}

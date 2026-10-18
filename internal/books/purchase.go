package books

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"fingo/internal/document"
	"fingo/internal/domain/gl"
	"fingo/internal/ledger"
	"fingo/internal/money"
)

// PurchaseBillCommand records a supplier invoice that is a cost, not stock.
//
// The three-way match (order → receipt → invoice) is the purchase flow's job when
// goods move through a warehouse. A service or an expense has no receipt, so this
// path posts the bill directly: Dr expense / Dr input VAT / Cr payable. It is a
// narrower flow, not a second implementation of the same one.
type PurchaseBillCommand struct {
	InvoiceID      string
	PartnerID      string
	ExpenseAccount string
	VATRateID      string
	Net            money.Dong
	Date           time.Time
	Actor          string
	Description    string
}

// PurchaseBill posts a supplier bill and records the payable.
func (b *Books) PurchaseBill(ctx context.Context, cmd PurchaseBillCommand) (ledger.Posted, error) {
	switch {
	case cmd.PartnerID == "":
		return ledger.Posted{}, errors.New("books: a purchase bill needs a supplier")
	case strings.TrimSpace(cmd.ExpenseAccount) == "":
		return ledger.Posted{}, errors.New("books: a purchase bill needs the account the cost belongs to")
	case cmd.Net <= money.Zero:
		return ledger.Posted{}, fmt.Errorf("books: a bill of %s is not a cost", cmd.Net)
	case cmd.Actor == "":
		return ledger.Posted{}, errors.New("books: a purchase bill needs an operator")
	}

	rates, err := salesVATRates(ctx)
	if err != nil {
		return ledger.Posted{}, err
	}

	rate, err := rates.Get(cmd.VATRateID)
	if err != nil {
		return ledger.Posted{}, fmt.Errorf("books: VAT rate %q: %w", cmd.VATRateID, err)
	}

	vatAmount := money.FromInt(rate.Tax(cmd.Net.Int64()))
	total := cmd.Net.Add(vatAmount)

	day := time.Date(cmd.Date.Year(), cmd.Date.Month(), cmd.Date.Day(), 0, 0, 0, 0, time.UTC)
	invoiceID := cmd.InvoiceID
	if invoiceID == "" {
		invoiceID = newUUID()
	}

	lines := []ledger.Line{
		{Account: cmd.ExpenseAccount, Side: gl.SideDebit, Amount: cmd.Net},
	}
	// A zero-rated or exempt bill has no input VAT to claim: posting a zero-amount
	// 1331 line would be refused by the amount CHECK, and claiming nothing is the
	// correct accounting, not a missing line.
	if vatAmount > money.Zero {
		lines = append(lines, ledger.Line{Account: "1331", Side: gl.SideDebit, Amount: vatAmount})
	}
	lines = append(lines, ledger.Line{Account: "331", Side: gl.SideCredit, Amount: total})

	posted, err := ledger.Post(ctx, b.store, b.scope, ledger.Request{
		ID:          invoiceID,
		Description: defaultText(cmd.Description, "Hóa đơn mua vào"),
		Actor:       cmd.Actor,
		Date:        day,
		Type:        gl.VoucherTypePurchase,
		Lines:       lines,
	})
	if err != nil {
		return ledger.Posted{}, err
	}

	period, err := b.EnsureCurrentYear(ctx, day)
	if err != nil {
		return ledger.Posted{}, err
	}

	doc := document.Document{
		ID:        invoiceID,
		Type:      document.TypeInvoice,
		PartnerID: cmd.PartnerID,
		Date:      day,
		PeriodID:  period.ID,
		VATRateID: cmd.VATRateID,
		Total:     money.New(total.Int64(), "VND"),
		Allocated: money.New(0, "VND"),
		Status:    document.StatusOpen,
	}

	docLines := []document.Line{
		{DocumentID: invoiceID, AccountID: cmd.ExpenseAccount, Debit: money.New(cmd.Net.Int64(), "VND"), BaseAmount: money.New(cmd.Net.Int64(), "VND")},
	}
	if vatAmount > money.Zero {
		docLines = append(docLines, document.Line{DocumentID: invoiceID, AccountID: "1331", Debit: money.New(vatAmount.Int64(), "VND"), VATAmount: money.New(vatAmount.Int64(), "VND")})
	}
	docLines = append(docLines, document.Line{DocumentID: invoiceID, AccountID: "331", Credit: money.New(total.Int64(), "VND")})

	if err := b.Documents().CreateAs(ctx, "PURCHASE_BILL", doc, docLines); err != nil {
		return posted, fmt.Errorf("bill %s posted, but recording the payable failed: %w", posted.VoucherNo, err)
	}

	return posted, nil
}

func defaultText(s, fallback string) string {
	if strings.TrimSpace(s) == "" {
		return fallback
	}
	return s
}

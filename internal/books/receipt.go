package books

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"time"

	"fingo/internal/document"
	"fingo/internal/domain/gl"
	"fingo/internal/ledger"
	"fingo/internal/money"
)

// ReceiptCommand records money received from a customer against one invoice.
type ReceiptCommand struct {
	DocumentID string
	Amount     money.Dong
	Date       time.Time
	Actor      string
}

// ErrNotReceivable is returned for a document that cannot be paid.
var ErrNotReceivable = errors.New("books: the document is not open for collection")

// Receipt posts Dr 1111 / Cr 131 and settles the invoice.
//
// Two steps, one intention: the receipt voucher is posted, then the payment is
// applied to the document. They are not one transaction because Apply owns its
// own row lock and its own transaction; a crash between them leaves an
// unallocated receipt, which is visible in the ledger and can be applied by hand
// — never a document marked paid by money that was not recorded.
//
// The voucher id is derived from (client, document, amount, date), so a retried
// receipt collides with itself at the primary key instead of paying twice.
func (b *Books) Receipt(ctx context.Context, cmd ReceiptCommand) (ledger.Posted, error) {
	if cmd.Amount <= money.Zero {
		return ledger.Posted{}, fmt.Errorf("books: a receipt of %s is not money received", cmd.Amount)
	}
	if cmd.DocumentID == "" {
		return ledger.Posted{}, errors.New("books: a receipt needs the invoice it settles")
	}
	if cmd.Actor == "" {
		return ledger.Posted{}, errors.New("books: a receipt needs an operator")
	}

	doc, err := b.Documents().Get(ctx, cmd.DocumentID)
	if err != nil {
		return ledger.Posted{}, err
	}
	if doc.Status != document.StatusOpen && doc.Status != document.StatusPartiallyPaid {
		return ledger.Posted{}, fmt.Errorf("%w: %s is %s", ErrNotReceivable, cmd.DocumentID, doc.Status)
	}
	if doc.PartnerID == "" {
		return ledger.Posted{}, fmt.Errorf("%w: %s names no customer", ErrNotReceivable, cmd.DocumentID)
	}

	day := time.Date(cmd.Date.Year(), cmd.Date.Month(), cmd.Date.Day(), 0, 0, 0, 0, time.UTC)
	voucherID := derivedID(b.scope.ID(), cmd.DocumentID, cmd.Amount.Int64(), day)

	posted, err := ledger.Post(ctx, b.store, b.scope, ledger.Request{
		ID:          voucherID,
		Description: "Thu tiền khách hàng",
		Actor:       cmd.Actor,
		Date:        day,
		Type:        gl.VoucherTypeCashReceipt,
		Lines: []ledger.Line{
			{Account: "1111", Side: gl.SideDebit, Amount: cmd.Amount},
			{Account: "131", Side: gl.SideCredit, Amount: cmd.Amount},
		},
	})
	if err != nil {
		return ledger.Posted{}, err
	}

	// A retried request carries the same derived allocation id, and Apply refuses
	// the same payment against the same document twice.
	allocationID := derivedID("allocation", cmd.DocumentID, cmd.Amount.Int64(), day)

	if _, err := b.Documents().Apply(ctx, document.Application{
		ID:         allocationID,
		DocumentID: cmd.DocumentID,
		PaymentID:  posted.ID,
		Amount:     money.New(cmd.Amount.Int64(), "VND"),
		At:         time.Now(),
	}); err != nil {
		return posted, fmt.Errorf("receipt %s posted, but settling %s failed: %w",
			posted.VoucherNo, cmd.DocumentID, err)
	}

	return posted, nil
}

// OpenReceivables returns the invoices a customer still owes.
func (b *Books) OpenReceivables(ctx context.Context) ([]document.Document, error) {
	all, err := b.Documents().List(ctx)
	if err != nil {
		return nil, err
	}

	out := make([]document.Document, 0, len(all))
	for _, d := range all {
		if d.Type != document.TypeInvoice {
			continue
		}
		if d.Status == document.StatusOpen || d.Status == document.StatusPartiallyPaid {
			out = append(out, d)
		}
	}
	return out, nil
}

// derivedID builds a stable uuid from the facts of a request, so a retry is the
// same request rather than a second one. It is not a security boundary.
func derivedID(parts ...any) string {
	h := sha256.New()
	for _, p := range parts {
		_, _ = fmt.Fprintf(h, "%v|", p)
	}
	sum := h.Sum(nil)

	var b [16]byte
	copy(b[:], sum[:16])
	b[6] = (b[6] & 0x0f) | 0x50
	b[8] = (b[8] & 0x3f) | 0x80

	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

package books

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"fingo/internal/document"
	"fingo/internal/domain/gl"
	"fingo/internal/ledger"
	"fingo/internal/money"
)

// PaymentCommand records money paid to a supplier against one bill.
type PaymentCommand struct {
	DocumentID string
	Amount     money.Dong
	Date       time.Time
	Actor      string
}

// Payment posts Dr 331 / Cr 1111 and settles the bill.
//
// It is the receipt's mirror image, and it shares the same ordering choice: the
// voucher is posted first, then the payment is applied. A crash between the two
// leaves an unallocated payment rather than a bill marked paid by money that was
// never recorded.
func (b *Books) Payment(ctx context.Context, cmd PaymentCommand) (ledger.Posted, error) {
	if cmd.Amount <= money.Zero {
		return ledger.Posted{}, fmt.Errorf("books: a payment of %s is not money paid", cmd.Amount)
	}
	if cmd.DocumentID == "" {
		return ledger.Posted{}, errors.New("books: a payment needs the bill it settles")
	}
	if cmd.Actor == "" {
		return ledger.Posted{}, errors.New("books: a payment needs an operator")
	}

	doc, err := b.Documents().Get(ctx, cmd.DocumentID)
	if err != nil {
		return ledger.Posted{}, err
	}
	if doc.Status != document.StatusOpen && doc.Status != document.StatusPartiallyPaid {
		return ledger.Posted{}, fmt.Errorf("%w: %s is %s", ErrNotReceivable, cmd.DocumentID, doc.Status)
	}

	day := time.Date(cmd.Date.Year(), cmd.Date.Month(), cmd.Date.Day(), 0, 0, 0, 0, time.UTC)

	// The payment is numbered in the cash-payment book, and its id is derived so a
	// retried request collides with itself.
	posted, err := ledger.Post(ctx, b.store, b.scope, ledger.Request{
		ID:          derivedID(b.scope.ID(), "payment", cmd.DocumentID, cmd.Amount.Int64(), day),
		Description: "Trả tiền nhà cung cấp",
		Actor:       cmd.Actor,
		Date:        day,
		Type:        gl.VoucherTypeCashPayment,
		Lines: []ledger.Line{
			{Account: "331", Side: gl.SideDebit, Amount: cmd.Amount},
			{Account: "1111", Side: gl.SideCredit, Amount: cmd.Amount},
		},
	})
	if err != nil {
		return ledger.Posted{}, err
	}

	if _, err := b.Documents().Apply(ctx, document.Application{
		ID:         derivedID("payment-allocation", cmd.DocumentID, cmd.Amount.Int64(), day),
		DocumentID: cmd.DocumentID,
		PaymentID:  posted.ID,
		Amount:     money.New(cmd.Amount.Int64(), "VND"),
		At:         time.Now(),
	}); err != nil {
		return posted, fmt.Errorf("payment %s posted, but settling %s failed: %w",
			posted.VoucherNo, cmd.DocumentID, err)
	}

	return posted, nil
}

// OpenPayables returns the supplier bills still owed.
//
// A bill and a sales invoice are both document.TypeInvoice, so the distinction is
// read from the stored kind rather than guessed from the domain type.
func (b *Books) OpenPayables(ctx context.Context) ([]document.Document, error) {
	var out []document.Document

	err := b.store.Read(ctx, b.scope, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			SELECT id, COALESCE(partner_id::text, ''), COALESCE(period_id::text, ''),
			       document_date, total, allocated, status
			  FROM documents
			 WHERE kind = 'PURCHASE_BILL'
			   AND status IN ('OPEN', 'PARTIAL')
			 ORDER BY document_date, id`)
		if err != nil {
			return err
		}
		defer rows.Close()

		for rows.Next() {
			var (
				doc       document.Document
				total     int64
				allocated int64
				status    string
			)
			if err := rows.Scan(&doc.ID, &doc.PartnerID, &doc.PeriodID,
				&doc.Date, &total, &allocated, &status); err != nil {
				return err
			}
			doc.Type = document.TypeInvoice
			doc.Total = money.New(total, "VND")
			doc.Allocated = money.New(allocated, "VND")
			doc.Status = statusFromDB(status)
			out = append(out, doc)
		}
		return rows.Err()
	})

	return out, err
}

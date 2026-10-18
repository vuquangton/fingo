package books

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"fingo/internal/document"
	"fingo/internal/money"
	"fingo/internal/store"
)

// Documents is the AR/AP document book over PostgreSQL.
//
// It implements document.Repository. Apply is the method that matters: it re-reads
// the document under a row lock, decides what it can absorb from what it finds,
// and writes the allocation, the header and any credit together. A version that
// reads, thinks, and then writes without the lock over-allocates the first time
// two payments arrive at once.
type Documents struct {
	store *store.Store
	scope store.Scope
}

// DocumentsFor returns the document book for one client.
func (b *Books) Documents() *Documents {
	return &Documents{store: b.store, scope: b.scope}
}

// Create stores a document and its lines.
func (d *Documents) Create(ctx context.Context, doc document.Document, lines []document.Line) error {
	if err := doc.Validate(); err != nil {
		return err
	}

	return d.store.InClient(ctx, d.scope, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `
			INSERT INTO documents
				(id, client_id, kind, partner_id, document_date, period_id, total, allocated, status)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
			doc.ID, d.scope.ID(), kindOf(doc.Type), nullUUID(doc.PartnerID), doc.Date,
			nullUUID(doc.PeriodID), doc.Total.Amount(), doc.Allocated.Amount(), statusToDB(doc.Status)); err != nil {
			return fmt.Errorf("write the document: %w", err)
		}

		for i, l := range lines {
			if err := l.Validate(doc.Total.Currency()); err != nil {
				return err
			}
			if _, err := tx.Exec(ctx, `
				INSERT INTO document_lines
					(client_id, document_id, line_order, account_code, debit, credit, vat_amount, base_amount)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
				d.scope.ID(), doc.ID, i+1, l.AccountID,
				l.Debit.Amount(), l.Credit.Amount(), l.VATAmount.Amount(), l.BaseAmount.Amount()); err != nil {
				return fmt.Errorf("write line %d: %w", i+1, err)
			}
		}
		return nil
	})
}

// Get returns one document.
func (d *Documents) Get(ctx context.Context, id string) (document.Document, error) {
	var doc document.Document

	err := d.store.Read(ctx, d.scope, func(tx pgx.Tx) error {
		var kind string
		var total, allocated int64
		var status string

		if err := tx.QueryRow(ctx, `
			SELECT id, kind, COALESCE(partner_id::text, ''), COALESCE(period_id::text, ''), document_date, total, allocated, status
			  FROM documents WHERE id = $1`, id).
			Scan(&doc.ID, &kind, &doc.PartnerID, &doc.PeriodID, &doc.Date, &total, &allocated, &status); err != nil {
			return err
		}

		doc.Type = typeOf(kind)
		doc.Total = money.New(total, "VND")
		doc.Allocated = money.New(allocated, "VND")
		doc.Status = statusFromDB(status)
		return nil
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return document.Document{}, document.ErrUnknownDocument
	}
	return doc, err
}

// List returns every document, oldest first.
func (d *Documents) List(ctx context.Context) ([]document.Document, error) {
	var out []document.Document

	err := d.store.Read(ctx, d.scope, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			SELECT id, kind, COALESCE(partner_id::text, ''), COALESCE(period_id::text, ''), document_date, total, allocated, status
			  FROM documents ORDER BY document_date, id`)
		if err != nil {
			return err
		}
		defer rows.Close()

		for rows.Next() {
			var doc document.Document
			var kind string
			var total, allocated int64
			var status string
			if err := rows.Scan(&doc.ID, &kind, &doc.PartnerID, &doc.PeriodID, &doc.Date, &total, &allocated, &status); err != nil {
				return err
			}
			doc.Type = typeOf(kind)
			doc.Total = money.New(total, "VND")
			doc.Allocated = money.New(allocated, "VND")
			doc.Status = statusFromDB(status)
			out = append(out, doc)
		}
		return rows.Err()
	})

	return out, err
}

// Lines returns a document's lines in order.
func (d *Documents) Lines(ctx context.Context, documentID string) ([]document.Line, error) {
	var out []document.Line

	err := d.store.Read(ctx, d.scope, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			SELECT account_code, debit, credit, vat_amount, base_amount
			  FROM document_lines WHERE document_id = $1 ORDER BY line_order`, documentID)
		if err != nil {
			return err
		}
		defer rows.Close()

		for rows.Next() {
			var l document.Line
			var debit, credit, vat, base int64
			if err := rows.Scan(&l.AccountID, &debit, &credit, &vat, &base); err != nil {
				return err
			}
			l.DocumentID = documentID
			l.Debit = money.New(debit, "VND")
			l.Credit = money.New(credit, "VND")
			l.VATAmount = money.New(vat, "VND")
			l.BaseAmount = money.New(base, "VND")
			out = append(out, l)
		}
		return rows.Err()
	})

	return out, err
}

// Apply settles a payment against one document, under a row lock.
func (d *Documents) Apply(ctx context.Context, app document.Application) (document.Result, error) {
	var result document.Result

	err := d.store.InClient(ctx, d.scope, func(tx pgx.Tx) error {
		var doc document.Document
		var total, allocated int64
		var status string
		var kind string

		err := tx.QueryRow(ctx, `
			SELECT id, kind, COALESCE(partner_id::text, ''), COALESCE(period_id::text, ''), document_date, total, allocated, status
			  FROM documents WHERE id = $1 FOR UPDATE`, app.DocumentID).
			Scan(&doc.ID, &kind, &doc.PartnerID, &doc.PeriodID, &doc.Date, &total, &allocated, &status)
		if errors.Is(err, pgx.ErrNoRows) {
			return document.ErrUnknownDocument
		}
		if err != nil {
			return fmt.Errorf("lock the document: %w", err)
		}

		doc.Type = typeOf(kind)
		doc.Total = money.New(total, "VND")
		doc.Allocated = money.New(allocated, "VND")
		doc.Status = statusFromDB(status)

		result, err = document.Settle(doc, app)
		if err != nil {
			return err
		}

		if result.Allocation != nil {
			// The domain derives readable ids ("allocation-<uuid>"); the uuid column
			// needs one of its own, as the ledger does for vouchers.
			allocationID := result.Allocation.ID
			if !isUUID(allocationID) {
				allocationID = newUUID()
			}

			if _, err := tx.Exec(ctx, `
				INSERT INTO allocations (id, client_id, document_id, voucher_id, amount)
				VALUES ($1, $2, $3, $4, $5)`,
				allocationID, d.scope.ID(), doc.ID, result.Allocation.PaymentID,
				result.Allocation.Amount.Amount()); err != nil {
				return fmt.Errorf("write the allocation: %w", err)
			}
		}

		if result.Credit != nil {
			creditID := result.Credit.ID
			if !isUUID(creditID) {
				creditID = newUUID()
			}

			if _, err := tx.Exec(ctx, `
				INSERT INTO document_credits (id, client_id, partner_id, document_id, payment_id, amount)
				VALUES ($1, $2, $3, $4, $5, $6)`,
				creditID, d.scope.ID(), nullUUID(doc.PartnerID), doc.ID,
				result.Credit.PaymentID, result.Credit.Amount.Amount()); err != nil {
				return fmt.Errorf("write the credit: %w", err)
			}
		}

		settled := result.Document
		if _, err := tx.Exec(ctx, `
			UPDATE documents SET allocated = $2, status = $3 WHERE id = $1`,
			doc.ID, settled.Allocated.Amount(), statusToDB(settled.Status)); err != nil {
			return fmt.Errorf("update the document: %w", err)
		}

		return nil
	})

	return result, err
}

// Allocations returns what has been applied to a document, oldest first.
func (d *Documents) Allocations(ctx context.Context, documentID string) ([]document.Allocation, error) {
	var out []document.Allocation

	err := d.store.Read(ctx, d.scope, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			SELECT id, document_id, voucher_id, amount, created_at
			  FROM allocations WHERE document_id = $1 ORDER BY created_at`, documentID)
		if err != nil {
			return err
		}
		defer rows.Close()

		for rows.Next() {
			var a document.Allocation
			var amount int64
			if err := rows.Scan(&a.ID, &a.DocumentID, &a.PaymentID, &amount, &a.At); err != nil {
				return err
			}
			a.Amount = money.New(amount, "VND")
			out = append(out, a)
		}
		return rows.Err()
	})

	return out, err
}

// Credits returns a partner's credit balances, oldest first.
func (d *Documents) Credits(ctx context.Context, partnerID string) ([]document.Credit, error) {
	var out []document.Credit

	err := d.store.Read(ctx, d.scope, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			SELECT id, COALESCE(partner_id::text, ''), document_id, payment_id, amount, created_at
			  FROM document_credits WHERE partner_id = $1 ORDER BY created_at`, partnerID)
		if err != nil {
			return err
		}
		defer rows.Close()

		for rows.Next() {
			var c document.Credit
			var amount int64
			if err := rows.Scan(&c.ID, &c.PartnerID, &c.DocumentID, &c.PaymentID, &amount, &c.At); err != nil {
				return err
			}
			c.Amount = money.New(amount, "VND")
			out = append(out, c)
		}
		return rows.Err()
	})

	return out, err
}

// Void marks a document cancelled. One that has moved money is not voidable.
func (d *Documents) Void(ctx context.Context, id string) error {
	return d.store.InClient(ctx, d.scope, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `
			UPDATE documents SET status = 'VOID'
			 WHERE id = $1 AND allocated = 0 AND status <> 'VOID'`, id)
		if err != nil {
			return fmt.Errorf("void the document: %w", err)
		}
		if tag.RowsAffected() == 0 {
			return fmt.Errorf("%w: document %s is settled or unknown and cannot be voided",
				document.ErrInvalidDocument, id)
		}
		return nil
	})
}

func kindOf(t document.Type) string {
	switch t {
	case document.TypeCreditNote:
		return "CREDIT_NOTE"
	case document.TypeReceipt:
		return "RECEIPT"
	case document.TypePayment:
		return "PAYMENT"
	default:
		return "SALES_INVOICE"
	}
}

func typeOf(kind string) document.Type {
	switch kind {
	case "CREDIT_NOTE":
		return document.TypeCreditNote
	case "RECEIPT":
		return document.TypeReceipt
	case "PAYMENT":
		return document.TypePayment
	default:
		return document.TypeInvoice
	}
}

func statusToDB(s document.Status) string {
	switch s {
	case document.StatusPartiallyPaid:
		return "PARTIAL"
	case document.StatusPaid:
		return "SETTLED"
	case document.StatusVoid:
		return "VOID"
	default:
		return "OPEN"
	}
}

func statusFromDB(s string) document.Status {
	switch s {
	case "PARTIAL":
		return document.StatusPartiallyPaid
	case "SETTLED":
		return document.StatusPaid
	case "VOID":
		return document.StatusVoid
	default:
		return document.StatusOpen
	}
}

// nullUUID stores an empty partner as NULL rather than as an empty string, which
// a uuid column would refuse.
func nullUUID(id string) any {
	if id == "" {
		return nil
	}
	return id
}

// newUUID is the RFC 4122 v4 layout, from crypto/rand.
func newUUID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic("books: crypto/rand failed: " + err.Error())
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

// isUUID reports whether an id is already in the form the database key needs.
func isUUID(s string) bool {
	if len(s) != 36 {
		return false
	}
	for i, r := range s {
		switch i {
		case 8, 13, 18, 23:
			if r != '-' {
				return false
			}
		default:
			if !((r >= '0' && r <= '9') || (r >= 'a' && r <= 'f') || (r >= 'A' && r <= 'F')) {
				return false
			}
		}
	}
	return true
}

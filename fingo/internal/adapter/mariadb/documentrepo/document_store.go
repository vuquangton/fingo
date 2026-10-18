// Package documentrepo stores the business documents a voucher records, and the
// allocations that settle them.
package documentrepo

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	mariadb "fingo/internal/adapter/mariadb/sqlc"
	"fingo/internal/domain/document"
)

// Store is the MariaDB adapter for documents and their allocations.
type Store struct {
	db      *sql.DB
	queries *mariadb.Queries
}

func NewStore(db *sql.DB) *Store {
	return &Store{db: db, queries: mariadb.New(db)}
}

// Record saves a document.
func (s *Store) Record(ctx context.Context, d document.Document) error {
	if err := d.Validate(); err != nil {
		return err
	}

	if err := s.queries.CreateDocument(ctx, mariadb.CreateDocumentParams{
		ID:           d.ID,
		PartnerID:    d.PartnerID,
		VoucherID:    d.VoucherID,
		Kind:         mariadb.DocumentsKind(d.Kind),
		Reference:    d.Reference,
		DocumentDate: d.DocumentDate,
		DueDate:      sql.NullTime{Time: d.DueDate, Valid: !d.DueDate.IsZero()},
		Amount:       d.Amount.String(),
	}); err != nil {
		return fmt.Errorf("record document %s: %w", d.Reference, err)
	}

	return nil
}

// OpenDocuments returns a partner's documents with what is still owed on each.
//
// Every document comes back, including settled ones — a caller that wants only
// the open ones filters on Outstanding, and a query that hid a paid invoice
// would make it impossible to show one.
//
// The subtraction happens here, in exact decimal arithmetic, rather than in SQL:
// sqlc cannot type the aggregate, and money is added with decimals in Go
// everywhere else.
func (s *Store) OpenDocuments(ctx context.Context, partnerID string) ([]document.Open, error) {
	rows, err := s.queries.ListDocumentsByPartner(ctx, partnerID)
	if err != nil {
		return nil, fmt.Errorf("list documents for partner %s: %w", partnerID, err)
	}

	allocationRows, err := s.queries.ListAllocationsByPartner(ctx, partnerID)
	if err != nil {
		return nil, fmt.Errorf("list allocations for partner %s: %w", partnerID, err)
	}

	settled := make(map[string]decimal.Decimal, len(allocationRows))
	for _, a := range allocationRows {
		amount, err := decimal.NewFromString(a.Amount)
		if err != nil {
			return nil, fmt.Errorf("allocation against document %s: %w", a.DocumentID, err)
		}

		settled[a.DocumentID] = settled[a.DocumentID].Add(amount)
	}

	out := make([]document.Open, 0, len(rows))
	for _, r := range rows {
		amount, err := decimal.NewFromString(r.Amount)
		if err != nil {
			return nil, fmt.Errorf("document %s: %w", r.ID, err)
		}

		out = append(out, document.Open{
			Document: document.Document{
				ID:           r.ID,
				PartnerID:    r.PartnerID,
				VoucherID:    r.VoucherID,
				Kind:         document.Kind(r.Kind),
				Reference:    r.Reference,
				DocumentDate: r.DocumentDate,
				DueDate:      r.DueDate.Time,
				Amount:       amount,
			},
			Outstanding: amount.Sub(settled[r.ID]),
		})
	}

	return out, nil
}

// Allocate records a payment's allocations against the documents it settles.
//
// All of one payment's allocations commit together. A payment applied to three
// invoices that recorded only two would leave books that disagree with
// themselves, and no later reader could tell which invoice the money went to.
func (s *Store) Allocate(
	ctx context.Context,
	paymentVoucherID string,
	plan document.Plan,
	at time.Time,
) error {
	if len(plan.Allocations) == 0 {
		return nil
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("record allocations: begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	q := s.queries.WithTx(tx)

	for _, a := range plan.Allocations {
		if err := q.CreateAllocation(ctx, mariadb.CreateAllocationParams{
			ID:          uuid.NewString(),
			DocumentID:  a.DocumentID,
			VoucherID:   paymentVoucherID,
			Amount:      a.Amount.String(),
			AllocatedAt: at.UTC(),
		}); err != nil {
			return fmt.Errorf("allocate %s against document %s: %w", a.Amount, a.DocumentID, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("record allocations: commit: %w", err)
	}

	return nil
}

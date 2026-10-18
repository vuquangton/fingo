// Package documentstore stores the AR/AP document book in MariaDB.
//
// It is the adapter for document.Repository, and it writes the `arap_*` tables —
// the model that tracks how much of a document has been settled, not the older
// `documents` and `allocations` tables that the posting path currently writes.
// The two coexist; see the note at the top of migration 00014.
package documentstore

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	mariadb "fingo/internal/adapter/mariadb/sqlc"
	"fingo/internal/document"
	"fingo/internal/money"
)

var _ document.Repository = (*Store)(nil)

// Store is the MariaDB adapter for document.Repository.
type Store struct {
	db      *sql.DB
	queries *mariadb.Queries
}

// NewStore builds the adapter over an open connection.
func NewStore(db *sql.DB) *Store {
	return &Store{db: db, queries: mariadb.New(db)}
}

// Create stores a document together with its lines, in one transaction.
//
// A document without its lines is a header nobody can check, and lines without
// their document are orphans, so the two are written together or not at all.
//
// The existence check inside the transaction asks for the row FOR UPDATE. On a
// primary key that does not exist, InnoDB takes a gap lock, so a second create
// of the same id waits rather than racing past the check to the insert.
func (s *Store) Create(ctx context.Context, d document.Document, lines []document.Line) error {
	if err := document.CheckNew(d, lines); err != nil {
		return err
	}

	kind, err := toDBType(d.Type)
	if err != nil {
		return err
	}
	status, err := toDBStatus(d.Status)
	if err != nil {
		return err
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("create document %s: begin: %w", d.ID, err)
	}
	defer func() { _ = tx.Rollback() }()

	q := s.queries.WithTx(tx)

	if _, err := q.GetArapDocumentForUpdate(ctx, d.ID); err == nil {
		return fmt.Errorf("%w: %s", document.ErrDocumentExists, d.ID)
	} else if !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("create document %s: read: %w", d.ID, err)
	}

	if err := q.CreateArapDocument(ctx, mariadb.CreateArapDocumentParams{
		ID:              d.ID,
		DocType:         kind,
		PartnerID:       d.PartnerID,
		DocumentDate:    d.Date,
		PeriodID:        d.PeriodID,
		VatRateID:       nullString(d.VATRateID),
		TotalAmount:     d.Total.Amount(),
		AllocatedAmount: d.Allocated.Amount(),
		Currency:        string(d.Total.Currency()),
		Status:          status,
	}); err != nil {
		return fmt.Errorf("create document %s: %w", d.ID, err)
	}

	for i, line := range lines {
		if err := q.CreateArapLine(ctx, mariadb.CreateArapLineParams{
			DocumentID: d.ID,
			LineNo:     int32(i + 1),
			AccountID:  line.AccountID,
			Debit:      line.Debit.Amount(),
			Credit:     line.Credit.Amount(),
			VatAmount:  line.VATAmount.Amount(),
			BaseAmount: line.BaseAmount.Amount(),
		}); err != nil {
			return fmt.Errorf("create document %s line %d: %w", d.ID, i+1, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("create document %s: commit: %w", d.ID, err)
	}

	return nil
}

// Get returns one document.
func (s *Store) Get(ctx context.Context, id string) (document.Document, error) {
	row, err := s.queries.GetArapDocument(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return document.Document{}, fmt.Errorf("%w: %s", document.ErrUnknownDocument, id)
	}
	if err != nil {
		return document.Document{}, fmt.Errorf("get document %s: %w", id, err)
	}

	return toDocument(row.ID, row.DocType, row.PartnerID, row.DocumentDate, row.PeriodID,
		row.VatRateID, row.TotalAmount, row.AllocatedAmount, row.Currency, row.Status)
}

// List returns every document, oldest first.
func (s *Store) List(ctx context.Context) ([]document.Document, error) {
	rows, err := s.queries.ListArapDocuments(ctx)
	if err != nil {
		return nil, fmt.Errorf("list documents: %w", err)
	}

	out := make([]document.Document, 0, len(rows))
	for _, r := range rows {
		d, err := toDocument(r.ID, r.DocType, r.PartnerID, r.DocumentDate, r.PeriodID,
			r.VatRateID, r.TotalAmount, r.AllocatedAmount, r.Currency, r.Status)
		if err != nil {
			return nil, err
		}
		out = append(out, d)
	}

	return out, nil
}

// Lines returns a document's lines.
//
// The document is read first, because a stored line carries amounts and not a
// currency: a line's amounts are in its document's currency, which is what keeps
// one currency per document rather than one per line.
func (s *Store) Lines(ctx context.Context, documentID string) ([]document.Line, error) {
	d, err := s.Get(ctx, documentID)
	if err != nil {
		return nil, err
	}

	rows, err := s.queries.ListArapLines(ctx, documentID)
	if err != nil {
		return nil, fmt.Errorf("list lines of document %s: %w", documentID, err)
	}

	out := make([]document.Line, 0, len(rows))
	for _, r := range rows {
		out = append(out, toLine(documentID, d.Total.Currency(),
			r.AccountID, r.Debit, r.Credit, r.VatAmount, r.BaseAmount))
	}

	return out, nil
}

// Apply settles a payment against one document, atomically.
//
// Everything happens inside one transaction, and the document row is read FOR
// UPDATE before anything is decided. That lock is the whole point: a second
// application of a payment to the same document waits for this transaction to
// finish, then reads the allocated amount this one wrote, and decides again from
// there. Without it both would read the same outstanding balance and both would
// spend it.
func (s *Store) Apply(ctx context.Context, app document.Application) (document.Result, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return document.Result{}, fmt.Errorf("apply to document %s: begin: %w", app.DocumentID, err)
	}
	defer func() { _ = tx.Rollback() }()

	q := s.queries.WithTx(tx)

	row, err := q.GetArapDocumentForUpdate(ctx, app.DocumentID)
	if errors.Is(err, sql.ErrNoRows) {
		return document.Result{}, fmt.Errorf("%w: %s", document.ErrUnknownDocument, app.DocumentID)
	}
	if err != nil {
		return document.Result{}, fmt.Errorf("apply to document %s: lock: %w", app.DocumentID, err)
	}

	current, err := toDocument(row.ID, row.DocType, row.PartnerID, row.DocumentDate, row.PeriodID,
		row.VatRateID, row.TotalAmount, row.AllocatedAmount, row.Currency, row.Status)
	if err != nil {
		return document.Result{}, err
	}

	result, err := document.Settle(current, app)
	if err != nil {
		return document.Result{}, err
	}

	applied, err := s.alreadyApplied(ctx, q, app)
	if err != nil {
		return document.Result{}, err
	}
	if applied {
		return document.Result{}, fmt.Errorf("%w: payment %s, document %s",
			document.ErrAlreadyApplied, app.PaymentID, app.DocumentID)
	}

	status, err := toDBStatus(result.Document.Status)
	if err != nil {
		return document.Result{}, err
	}

	// The row is not counted. MySQL reports the rows an UPDATE CHANGED, not the
	// rows it matched, and a settlement that writes the values already there —
	// which every overpayment against an already-full document does — changes
	// nothing and reports zero. Treating that zero as "no such document" would
	// turn the ordinary overpayment into a missing row. The row cannot have
	// vanished: it is locked, and it was read two statements ago.
	if _, err := q.UpdateArapDocumentSettlement(ctx, mariadb.UpdateArapDocumentSettlementParams{
		AllocatedAmount: result.Document.Allocated.Amount(),
		Status:          status,
		ID:              current.ID,
	}); err != nil {
		return document.Result{}, fmt.Errorf("apply to document %s: settle: %w", current.ID, err)
	}

	if result.Allocation != nil {
		if err := q.CreateArapAllocation(ctx, mariadb.CreateArapAllocationParams{
			ID:          result.Allocation.ID,
			DocumentID:  result.Allocation.DocumentID,
			PaymentID:   result.Allocation.PaymentID,
			Amount:      result.Allocation.Amount.Amount(),
			Currency:    string(result.Allocation.Amount.Currency()),
			AllocatedAt: result.Allocation.At.UTC(),
		}); err != nil {
			return document.Result{}, fmt.Errorf("apply to document %s: record allocation: %w", current.ID, err)
		}
	}

	if result.Credit != nil {
		if err := q.CreateArapCredit(ctx, mariadb.CreateArapCreditParams{
			ID:         result.Credit.ID,
			PartnerID:  result.Credit.PartnerID,
			DocumentID: result.Credit.DocumentID,
			PaymentID:  result.Credit.PaymentID,
			Amount:     result.Credit.Amount.Amount(),
			Currency:   string(result.Credit.Amount.Currency()),
			CreatedAt:  result.Credit.At.UTC(),
		}); err != nil {
			return document.Result{}, fmt.Errorf("apply to document %s: record credit: %w", current.ID, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return document.Result{}, fmt.Errorf("apply to document %s: commit: %w", current.ID, err)
	}

	return result, nil
}

// Allocations returns what has been applied to a document, oldest first.
func (s *Store) Allocations(ctx context.Context, documentID string) ([]document.Allocation, error) {
	rows, err := s.queries.ListArapAllocations(ctx, documentID)
	if err != nil {
		return nil, fmt.Errorf("list allocations of document %s: %w", documentID, err)
	}

	out := make([]document.Allocation, 0, len(rows))
	for _, r := range rows {
		amount, err := money.FromDB(r.Amount, r.Currency)
		if err != nil {
			return nil, fmt.Errorf("allocation %s: %w", r.ID, err)
		}
		out = append(out, document.Allocation{
			ID:         r.ID,
			DocumentID: r.DocumentID,
			PaymentID:  r.PaymentID,
			Amount:     amount,
			At:         r.AllocatedAt.UTC(),
		})
	}

	return out, nil
}

// Credits returns a partner's credit balances, oldest first.
func (s *Store) Credits(ctx context.Context, partnerID string) ([]document.Credit, error) {
	rows, err := s.queries.ListArapCreditsByPartner(ctx, partnerID)
	if err != nil {
		return nil, fmt.Errorf("list credits of partner %s: %w", partnerID, err)
	}

	out := make([]document.Credit, 0, len(rows))
	for _, r := range rows {
		amount, err := money.FromDB(r.Amount, r.Currency)
		if err != nil {
			return nil, fmt.Errorf("credit %s: %w", r.ID, err)
		}
		out = append(out, document.Credit{
			ID:         r.ID,
			PartnerID:  r.PartnerID,
			DocumentID: r.DocumentID,
			PaymentID:  r.PaymentID,
			Amount:     amount,
			At:         r.CreatedAt.UTC(),
		})
	}

	return out, nil
}

// Void marks a document cancelled.
//
// The row is locked and re-read first, so "already allocated" and "no such
// document" are told apart from a successful void rather than collapsed into an
// update that matched nothing.
func (s *Store) Void(ctx context.Context, id string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("void document %s: begin: %w", id, err)
	}
	defer func() { _ = tx.Rollback() }()

	q := s.queries.WithTx(tx)

	row, err := q.GetArapDocumentForUpdate(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("%w: %s", document.ErrUnknownDocument, id)
	}
	if err != nil {
		return fmt.Errorf("void document %s: lock: %w", id, err)
	}

	current, err := toDocument(row.ID, row.DocType, row.PartnerID, row.DocumentDate, row.PeriodID,
		row.VatRateID, row.TotalAmount, row.AllocatedAmount, row.Currency, row.Status)
	if err != nil {
		return err
	}
	if !current.Allocated.IsZero() {
		return fmt.Errorf("%w: %s", document.ErrVoidNotAllowed, id)
	}
	if current.Status == document.StatusVoid {
		return nil
	}

	status, err := toDBStatus(document.StatusVoid)
	if err != nil {
		return err
	}

	// A zero here means the row was already void — the guard in the statement
	// and the check above agree — and not that the document is missing, which
	// the locked read has already ruled out. See the note on the settlement
	// update: MySQL counts rows changed, not rows matched.
	if _, err := q.VoidArapDocument(ctx, mariadb.VoidArapDocumentParams{Status: status, ID: id}); err != nil {
		return fmt.Errorf("void document %s: %w", id, err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("void document %s: commit: %w", id, err)
	}

	return nil
}

// alreadyApplied reports whether this payment has already been applied to this
// document, as an allocation that moved money or as a credit that could not.
//
// It is checked inside the transaction, after the document row is locked, and
// the tables carry unique keys on (document_id, payment_id) behind it: a retry
// that slipped past this check would still be refused by the database, rather
// than settling the document twice.
func (s *Store) alreadyApplied(ctx context.Context, q *mariadb.Queries, app document.Application) (bool, error) {
	allocations, err := q.CountArapAllocationsForPayment(ctx, mariadb.CountArapAllocationsForPaymentParams{
		DocumentID: app.DocumentID,
		PaymentID:  app.PaymentID,
	})
	if err != nil {
		return false, fmt.Errorf("check whether payment %s was applied to document %s: %w",
			app.PaymentID, app.DocumentID, err)
	}

	credits, err := q.CountArapCreditsForPayment(ctx, mariadb.CountArapCreditsForPaymentParams{
		DocumentID: app.DocumentID,
		PaymentID:  app.PaymentID,
	})
	if err != nil {
		return false, fmt.Errorf("check whether payment %s left a credit on document %s: %w",
			app.PaymentID, app.DocumentID, err)
	}

	return allocations > 0 || credits > 0, nil
}

// toDocument assembles a domain document from a stored row.
func toDocument(
	id string,
	docType mariadb.ArapDocumentsDocType,
	partnerID string,
	date time.Time,
	periodID string,
	vatRateID sql.NullString,
	totalAmount, allocatedAmount int64,
	currency string,
	status mariadb.ArapDocumentsStatus,
) (document.Document, error) {
	kind, err := fromDBType(docType)
	if err != nil {
		return document.Document{}, fmt.Errorf("document %s: %w", id, err)
	}
	settlement, err := fromDBStatus(status)
	if err != nil {
		return document.Document{}, fmt.Errorf("document %s: %w", id, err)
	}

	total, err := money.FromDB(totalAmount, currency)
	if err != nil {
		return document.Document{}, fmt.Errorf("document %s total: %w", id, err)
	}
	allocated, err := money.FromDB(allocatedAmount, currency)
	if err != nil {
		return document.Document{}, fmt.Errorf("document %s allocated: %w", id, err)
	}

	return document.Document{
		ID:        id,
		Type:      kind,
		PartnerID: partnerID,
		Date:      date.UTC(),
		PeriodID:  periodID,
		VATRateID: vatRateID.String,
		Total:     total,
		Allocated: allocated,
		Status:    settlement,
	}, nil
}

// toLine assembles a domain line from a stored row. A line's amounts are in the
// document's currency, which the caller has already read.
func toLine(documentID string, currency money.Currency, accountID string, debit, credit, vatAmount, baseAmount int64) document.Line {
	return document.Line{
		DocumentID: documentID,
		AccountID:  accountID,
		Debit:      money.New(debit, currency),
		Credit:     money.New(credit, currency),
		VATAmount:  money.New(vatAmount, currency),
		BaseAmount: money.New(baseAmount, currency),
	}
}

// toDBType maps the domain's type onto the stored enum.
func toDBType(t document.Type) (mariadb.ArapDocumentsDocType, error) {
	switch t {
	case document.TypeInvoice:
		return mariadb.ArapDocumentsDocTypeINVOICE, nil
	case document.TypeCreditNote:
		return mariadb.ArapDocumentsDocTypeCREDITNOTE, nil
	case document.TypeReceipt:
		return mariadb.ArapDocumentsDocTypeRECEIPT, nil
	case document.TypePayment:
		return mariadb.ArapDocumentsDocTypePAYMENT, nil
	case document.TypeJournal:
		return mariadb.ArapDocumentsDocTypeJOURNAL, nil
	}

	return "", fmt.Errorf("%w: document type %q has no stored form", document.ErrInvalidDocument, t)
}

// fromDBType maps a stored enum back onto the domain's type.
func fromDBType(t mariadb.ArapDocumentsDocType) (document.Type, error) {
	switch t {
	case mariadb.ArapDocumentsDocTypeINVOICE:
		return document.TypeInvoice, nil
	case mariadb.ArapDocumentsDocTypeCREDITNOTE:
		return document.TypeCreditNote, nil
	case mariadb.ArapDocumentsDocTypeRECEIPT:
		return document.TypeReceipt, nil
	case mariadb.ArapDocumentsDocTypePAYMENT:
		return document.TypePayment, nil
	case mariadb.ArapDocumentsDocTypeJOURNAL:
		return document.TypeJournal, nil
	}

	return "", fmt.Errorf("%w: stored document type %q is not one the domain knows", document.ErrInvalidDocument, t)
}

// toDBStatus maps the domain's status onto the stored enum.
func toDBStatus(s document.Status) (mariadb.ArapDocumentsStatus, error) {
	switch s {
	case document.StatusOpen:
		return mariadb.ArapDocumentsStatusOPEN, nil
	case document.StatusPartiallyPaid:
		return mariadb.ArapDocumentsStatusPARTIALLYPAID, nil
	case document.StatusPaid:
		return mariadb.ArapDocumentsStatusPAID, nil
	case document.StatusVoid:
		return mariadb.ArapDocumentsStatusVOID, nil
	}

	return "", fmt.Errorf("%w: document status %q has no stored form", document.ErrInvalidDocument, s)
}

// fromDBStatus maps a stored enum back onto the domain's status.
func fromDBStatus(s mariadb.ArapDocumentsStatus) (document.Status, error) {
	switch s {
	case mariadb.ArapDocumentsStatusOPEN:
		return document.StatusOpen, nil
	case mariadb.ArapDocumentsStatusPARTIALLYPAID:
		return document.StatusPartiallyPaid, nil
	case mariadb.ArapDocumentsStatusPAID:
		return document.StatusPaid, nil
	case mariadb.ArapDocumentsStatusVOID:
		return document.StatusVoid, nil
	}

	return "", fmt.Errorf("%w: stored document status %q is not one the domain knows", document.ErrInvalidDocument, s)
}

// nullString stores an absent reference as NULL rather than as an empty string,
// so that "none" is one value in the column and not two.
func nullString(value string) sql.NullString {
	return sql.NullString{String: value, Valid: value != ""}
}

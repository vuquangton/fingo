package client

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

// Errors a scoped financial repository can produce.
var (
	// ErrInvalidDocument is a document that could not be stored: no id, no
	// number, or no date.
	ErrInvalidDocument = errors.New("client: invalid document")

	// ErrDocumentExists is a write of an id already held.
	ErrDocumentExists = errors.New("client: a document with this id already exists")

	// ErrUnknownDocument is a lookup or delete of a document the scope's
	// client does not hold.
	//
	// A document that exists but belongs to another client is reported as
	// unknown, not as foreign. Distinguishing the two would let a caller
	// probe for the existence of another client's records, which is itself
	// the leak this type exists to prevent.
	ErrUnknownDocument = errors.New("client: no such document")
)

// Document is one financial record — an invoice, a receipt, a voucher — as a
// client-scoped table holds it.
//
// ClientID is the column every table that holds financial data carries. It is
// not a convenience: it is what makes a row addressable by exactly one client,
// and it is written from the Scope rather than from the caller, so a record
// cannot be filed under a client the request was not authenticated for.
//
// This is the reference shape for the rule, not the production model. The fuller
// document model — lines, allocations, settlement — lives in internal/document,
// and adopts the same Scope as its first argument.
type Document struct {
	// ID identifies the document within its client.
	ID string

	// ClientID is the client that owns this row — the `client_id` column
	// migration 00015 puts on every table that holds a client's data. Set
	// from the Scope on every write; a caller-supplied value that disagrees
	// is refused. A row read back with no owner is a legacy row and is
	// adopted by the scope that asked for it.
	ClientID string

	// Number is the document's own number (số chứng từ).
	Number string

	// Date is the business date of the document, in UTC.
	Date time.Time

	// Total is the document's value in whole đồng. VND has no subunit, so
	// there is no fractional part to carry — the same rule the ledger's
	// amount column follows.
	Total int64
}

// Validate reports whether the document could be stored.
func (d Document) Validate() error {
	if strings.TrimSpace(d.ID) == "" {
		return fmt.Errorf("%w: a document needs an id", ErrInvalidDocument)
	}
	if strings.TrimSpace(d.Number) == "" {
		return fmt.Errorf("%w: document %s needs a number", ErrInvalidDocument, d.ID)
	}
	if d.Date.IsZero() {
		return fmt.Errorf("%w: document %s needs a date", ErrInvalidDocument, d.ID)
	}

	return nil
}

// normalized returns the document with its identifiers trimmed, so that a
// document stored as " INV-1 " and looked up as "INV-1" is one document.
func (d Document) normalized() Document {
	d.ID = strings.TrimSpace(d.ID)
	d.ClientID = strings.TrimSpace(d.ClientID)
	d.Number = strings.TrimSpace(d.Number)
	d.Date = d.Date.UTC()

	return d
}

// DocumentRepository stores the financial documents of one client at a time.
//
// There is no method that does not take a Scope, and that is the whole design.
// "List every document" is not an operation this interface can express, so the
// commonest multi-tenant bug — a query that forgot its WHERE clause — cannot be
// written here. A repository for the ledger proper (vouchers, journal lines,
// periods) follows the same shape.
type DocumentRepository interface {
	// Put stores a document for the scope's client. A document that names a
	// different client is refused.
	Put(ctx context.Context, scope Scope, d Document) error

	// Get returns one of the scope's client's documents.
	Get(ctx context.Context, scope Scope, id string) (Document, error)

	// List returns every document of the scope's client, oldest first.
	List(ctx context.Context, scope Scope) ([]Document, error)

	// Delete removes one of the scope's client's documents.
	Delete(ctx context.Context, scope Scope, id string) error
}

// MemoryDocumentRepository is an in-memory, client-scoped document book.
//
// Rows are filed under the client that owns them and are only ever reached
// through a Scope, so a lookup can return another client's document only by
// naming that client — which the middleware will not let a request do.
//
// The zero value is an empty book.
type MemoryDocumentRepository struct {
	mu sync.RWMutex

	// rows is client id -> document id -> document. The outer key is the
	// ClientID column; there is no path through this repository that reads
	// the inner maps without it.
	rows map[string]map[string]Document
}

// A compile-time statement of intent: the memory book is a DocumentRepository.
var _ DocumentRepository = (*MemoryDocumentRepository)(nil)

// NewMemoryDocumentRepository returns an empty document book.
func NewMemoryDocumentRepository() *MemoryDocumentRepository {
	return &MemoryDocumentRepository{}
}

// Put stores a document for the scope's client.
func (r *MemoryDocumentRepository) Put(_ context.Context, scope Scope, d Document) error {
	if err := scope.guard(); err != nil {
		return err
	}

	d = d.normalized()
	if err := d.Validate(); err != nil {
		return err
	}
	if err := scope.owns(d.ClientID); err != nil {
		return err
	}
	d.ClientID = scope.ClientID()

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.rows[scope.ClientID()][d.ID]; exists {
		return fmt.Errorf("%w: %s", ErrDocumentExists, d.ID)
	}
	if r.rows == nil {
		r.rows = make(map[string]map[string]Document)
	}
	if r.rows[scope.ClientID()] == nil {
		r.rows[scope.ClientID()] = make(map[string]Document)
	}
	r.rows[scope.ClientID()][d.ID] = d

	return nil
}

// Get returns one of the scope's client's documents.
func (r *MemoryDocumentRepository) Get(_ context.Context, scope Scope, id string) (Document, error) {
	if err := scope.guard(); err != nil {
		return Document{}, err
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	d, ok := r.rows[scope.ClientID()][strings.TrimSpace(id)]
	if !ok {
		return Document{}, fmt.Errorf("%w: %s", ErrUnknownDocument, id)
	}

	return d, nil
}

// List returns the scope's client's documents, oldest first.
func (r *MemoryDocumentRepository) List(_ context.Context, scope Scope) ([]Document, error) {
	if err := scope.guard(); err != nil {
		return nil, err
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	out := make([]Document, 0, len(r.rows[scope.ClientID()]))
	for _, d := range r.rows[scope.ClientID()] {
		out = append(out, d)
	}

	sort.Slice(out, func(i, j int) bool {
		if !out[i].Date.Equal(out[j].Date) {
			return out[i].Date.Before(out[j].Date)
		}

		return out[i].ID < out[j].ID
	})

	return out, nil
}

// Delete removes one of the scope's client's documents.
func (r *MemoryDocumentRepository) Delete(_ context.Context, scope Scope, id string) error {
	if err := scope.guard(); err != nil {
		return err
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	key := strings.TrimSpace(id)
	if _, ok := r.rows[scope.ClientID()][key]; !ok {
		return fmt.Errorf("%w: %s", ErrUnknownDocument, id)
	}

	delete(r.rows[scope.ClientID()], key)

	return nil
}

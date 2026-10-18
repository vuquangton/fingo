package document

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
)

// MemoryRepository is an in-memory document book.
//
// It applies the same rules as the MariaDB adapter — the validation, the
// balancing of lines, and above all the read-decide-write in Apply — and forgets
// everything when the process exits. It is a fixture rather than a store.
//
// Where the adapter locks a row with SELECT ... FOR UPDATE, this holds a mutex
// across the whole of Apply. The lock is coarser than the database's — it
// serialises applications to different documents as well as to the same one —
// and that is the right trade here: a fixture's job is to be obviously correct,
// and a per-document lock is one more thing to get wrong.
//
// The zero value is an empty book.
type MemoryRepository struct {
	mu sync.Mutex

	documents   map[string]Document
	lines       map[string][]Line
	allocations map[string][]Allocation
	credits     map[string][]Credit

	// applications remembers every (document, payment) pair that has been
	// applied, whether it settled anything or became a credit. It is what makes
	// a repeated application refuse instead of settling twice.
	applications map[string]bool
}

// NewMemoryRepository returns an empty in-memory document book.
func NewMemoryRepository() *MemoryRepository { return &MemoryRepository{} }

// Create stores a document together with its lines.
func (r *MemoryRepository) Create(_ context.Context, d Document, lines []Line) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// The taken id is reported before the lines are judged: a document that is
	// already in the book is not going in whatever its lines say.
	if _, exists := r.documents[d.ID]; exists {
		return fmt.Errorf("%w: %s", ErrDocumentExists, d.ID)
	}
	if err := CheckNew(d, lines); err != nil {
		return err
	}

	r.init()
	r.documents[d.ID] = d
	r.lines[d.ID] = append([]Line(nil), lines...)

	return nil
}

// Get returns one document.
func (r *MemoryRepository) Get(_ context.Context, id string) (Document, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	d, ok := r.documents[id]
	if !ok {
		return Document{}, fmt.Errorf("%w: %s", ErrUnknownDocument, id)
	}

	return d, nil
}

// List returns every document, oldest first.
func (r *MemoryRepository) List(_ context.Context) ([]Document, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	return r.sortedLocked(), nil
}

// Lines returns a document's lines.
func (r *MemoryRepository) Lines(_ context.Context, documentID string) ([]Line, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.documents[documentID]; !ok {
		return nil, fmt.Errorf("%w: %s", ErrUnknownDocument, documentID)
	}

	return append([]Line(nil), r.lines[documentID]...), nil
}

// Apply settles a payment against one document, under the lock.
//
// The whole of the decision happens while the lock is held. Reading the document
// outside it and writing it back afterwards is the bug this method exists to
// avoid: two callers would each see the same outstanding amount and each settle
// the document with it.
func (r *MemoryRepository) Apply(_ context.Context, app Application) (Result, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	d, ok := r.documents[app.DocumentID]
	if !ok {
		return Result{}, fmt.Errorf("%w: %s", ErrUnknownDocument, app.DocumentID)
	}
	if r.applications[applicationKey(app.DocumentID, app.PaymentID)] {
		return Result{}, fmt.Errorf("%w: payment %s, document %s", ErrAlreadyApplied, app.PaymentID, app.DocumentID)
	}

	result, err := Settle(d, app)
	if err != nil {
		return Result{}, err
	}

	r.init()
	r.documents[d.ID] = result.Document
	r.applications[applicationKey(app.DocumentID, app.PaymentID)] = true

	if result.Allocation != nil {
		r.allocations[d.ID] = append(r.allocations[d.ID], *result.Allocation)
	}
	if result.Credit != nil {
		r.credits[d.PartnerID] = append(r.credits[d.PartnerID], *result.Credit)
	}

	return result, nil
}

// Allocations returns what has been applied to a document, oldest first.
func (r *MemoryRepository) Allocations(_ context.Context, documentID string) ([]Allocation, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.documents[documentID]; !ok {
		return nil, fmt.Errorf("%w: %s", ErrUnknownDocument, documentID)
	}

	out := append([]Allocation(nil), r.allocations[documentID]...)
	sort.SliceStable(out, func(i, j int) bool {
		if !out[i].At.Equal(out[j].At) {
			return out[i].At.Before(out[j].At)
		}

		return out[i].ID < out[j].ID
	})

	return out, nil
}

// Credits returns a partner's credit balances, oldest first.
func (r *MemoryRepository) Credits(_ context.Context, partnerID string) ([]Credit, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	out := append([]Credit(nil), r.credits[partnerID]...)
	sort.SliceStable(out, func(i, j int) bool {
		if !out[i].At.Equal(out[j].At) {
			return out[i].At.Before(out[j].At)
		}

		return out[i].ID < out[j].ID
	})

	return out, nil
}

// Void marks a document cancelled.
func (r *MemoryRepository) Void(_ context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	d, ok := r.documents[id]
	if !ok {
		return fmt.Errorf("%w: %s", ErrUnknownDocument, id)
	}
	if len(r.allocations[id]) > 0 || !d.Allocated.IsZero() {
		return fmt.Errorf("%w: %s", ErrVoidNotAllowed, id)
	}

	d.Status = StatusVoid
	r.documents[id] = d

	return nil
}

// init prepares the maps on first write, so the zero value is usable.
func (r *MemoryRepository) init() {
	if r.documents == nil {
		r.documents = make(map[string]Document)
		r.lines = make(map[string][]Line)
		r.allocations = make(map[string][]Allocation)
		r.credits = make(map[string][]Credit)
		r.applications = make(map[string]bool)
	}
}

// sortedLocked returns every document, oldest first. The order is the book's and
// not the map's: a caller rendering a list, or settling oldest first, must not
// depend on Go's map iteration order.
func (r *MemoryRepository) sortedLocked() []Document {
	out := make([]Document, 0, len(r.documents))
	for _, d := range r.documents {
		out = append(out, d)
	}

	sort.Slice(out, func(i, j int) bool {
		if !out[i].Date.Equal(out[j].Date) {
			return out[i].Date.Before(out[j].Date)
		}

		return out[i].ID < out[j].ID
	})

	return out
}

// applicationKey names one payment against one document.
func applicationKey(documentID, paymentID string) string {
	return strings.TrimSpace(documentID) + "\x00" + strings.TrimSpace(paymentID)
}

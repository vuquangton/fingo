package document

import (
	"context"
)

// Repository stores a client's documents, the lines that make them up, and what
// has settled them.
//
// The one method that has to be transactional is Apply, and the contract is the
// same for every implementation: re-read the document inside a lock, decide from
// what is found, and write Allocated and Status together. An implementation that
// reads the document, thinks about it, and then writes without holding a lock
// along the way will over-allocate the first time two payments arrive at once —
// each one having seen the same outstanding balance and each believing it may
// spend it.
type Repository interface {
	// Create stores a document together with its lines.
	//
	// A document is created open and with nothing allocated: what has been
	// settled is applied to it, never asserted about it when it is written.
	Create(ctx context.Context, d Document, lines []Line) error

	// Get returns one document.
	Get(ctx context.Context, id string) (Document, error)

	// List returns every document, oldest first.
	List(ctx context.Context) ([]Document, error)

	// Lines returns a document's lines, in the order they were stored.
	Lines(ctx context.Context, documentID string) ([]Line, error)

	// Apply settles a payment against one document, atomically.
	//
	// It re-reads the document under a lock, computes what the document can
	// absorb from what it finds, and writes the allocation, the document's
	// Allocated and Status, and any credit balance the payment left behind — in
	// one transaction. Either all of that happened or none of it did; there is
	// no state in which a document has been paid and does not say so.
	//
	// An overpayment is not an error. What the document cannot absorb becomes a
	// Credit, which the caller can read back through Credits.
	//
	// The same payment applied to the same document twice is refused, so that a
	// retried request cannot settle a document twice.
	Apply(ctx context.Context, app Application) (Result, error)

	// Allocations returns what has been applied to a document, oldest first.
	Allocations(ctx context.Context, documentID string) ([]Allocation, error)

	// Credits returns a partner's credit balances, oldest first.
	Credits(ctx context.Context, partnerID string) ([]Credit, error)

	// Void marks a document cancelled.
	//
	// A document that has been allocated may not be voided: voiding it would
	// leave the payment that settled it pointing at nothing. Reversing a
	// document that has moved money is a credit note, which is a document of its
	// own with a date and a period, and not a status change.
	Void(ctx context.Context, id string) error
}

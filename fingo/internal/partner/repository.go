package partner

import (
	"context"
	"strings"
)

// Repository stores the partner directory — the businesses a client trades with.
//
// Create and Update report a Warning alongside the error, and a warning is not a
// refusal: a duplicate tax id is written and reported. Only a partner that could
// not be stored at all — no id, no name, unknown type — produces an error. A
// caller that ignores the warning gets a correct directory and a missed chance
// to look at it, which is the intended trade: the operator, not the database,
// decides whether two rows are one business.
type Repository interface {
	// Create stores a partner. It fails if the id is already in the directory.
	Create(ctx context.Context, p Partner) (*Warning, error)

	// Get returns one partner. It fails if the directory holds no such id.
	Get(ctx context.Context, id string) (Partner, error)

	// List returns every partner, by name.
	List(ctx context.Context) ([]Partner, error)

	// Update replaces a stored partner. It fails if the id is unknown: an
	// update that matched nothing must not look like a success.
	Update(ctx context.Context, p Partner) (*Warning, error)

	// Delete removes a partner.
	//
	// A partner that has been traded with is deactivated rather than deleted —
	// Active false — so the documents that name it keep their meaning. Delete is
	// for a row created by mistake; once a partner is referred to by a journal
	// line, the reference is what says whether it may go.
	Delete(ctx context.Context, id string) error
}

// Normalized returns the partner as it should be stored: trimmed, and with its
// tax id in the one form the directory compares on.
//
// It runs on the way into every repository as well as in NewPartner, because a
// Partner assembled as a struct literal would otherwise skip it — and a tax id
// stored with its punctuation still in it is a duplicate nothing can find.
func (p Partner) Normalized() Partner {
	p.ID = strings.TrimSpace(p.ID)
	p.Name = strings.TrimSpace(p.Name)
	p.TaxID = NormalizeTaxID(p.TaxID)

	return p
}

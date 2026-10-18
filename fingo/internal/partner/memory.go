package partner

import (
	"context"
	"fmt"
	"sort"
)

// MemoryRepository is an in-memory partner directory.
//
// It applies the same rules as the MariaDB adapter — validation, normalisation
// and the duplicate warning — and forgets everything when the process exits. It
// exists so that a rule about partners can be tested, and a screen prototyped,
// without a database. It is a fixture rather than a store: it is not safe for
// concurrent use, and nothing durable may depend on it.
//
// The zero value is an empty directory.
type MemoryRepository struct {
	partners map[string]Partner
}

// NewMemoryRepository returns an empty in-memory directory.
func NewMemoryRepository() *MemoryRepository { return &MemoryRepository{} }

// Create stores a partner, warning when another row already holds its tax id.
func (r *MemoryRepository) Create(_ context.Context, p Partner) (*Warning, error) {
	p = p.Normalized()
	if err := p.Validate(); err != nil {
		return nil, err
	}

	if _, exists := r.partners[p.ID]; exists {
		return nil, fmt.Errorf("%w: %s", ErrPartnerExists, p.ID)
	}

	warning := CheckDuplicateTaxID(p, r.all())

	if r.partners == nil {
		r.partners = make(map[string]Partner)
	}
	r.partners[p.ID] = p

	return warning, nil
}

// Get returns one partner.
func (r *MemoryRepository) Get(_ context.Context, id string) (Partner, error) {
	p, ok := r.partners[id]
	if !ok {
		return Partner{}, fmt.Errorf("%w: %s", ErrUnknownPartner, id)
	}

	return p, nil
}

// List returns every partner, by name and then id.
func (r *MemoryRepository) List(_ context.Context) ([]Partner, error) {
	return r.all(), nil
}

// Update replaces a stored partner, warning when another row already holds its
// tax id.
func (r *MemoryRepository) Update(_ context.Context, p Partner) (*Warning, error) {
	p = p.Normalized()
	if err := p.Validate(); err != nil {
		return nil, err
	}

	if _, exists := r.partners[p.ID]; !exists {
		return nil, fmt.Errorf("%w: %s", ErrUnknownPartner, p.ID)
	}

	warning := CheckDuplicateTaxID(p, r.all())
	r.partners[p.ID] = p

	return warning, nil
}

// Delete removes a partner.
func (r *MemoryRepository) Delete(_ context.Context, id string) error {
	if _, exists := r.partners[id]; !exists {
		return fmt.Errorf("%w: %s", ErrUnknownPartner, id)
	}

	delete(r.partners, id)

	return nil
}

// all returns every partner by name and then id.
//
// The order is the directory's, not the map's: a caller rendering a list, and a
// warning that names one of several holders of a tax id, must not depend on Go's
// map iteration order.
func (r *MemoryRepository) all() []Partner {
	out := make([]Partner, 0, len(r.partners))
	for _, p := range r.partners {
		out = append(out, p)
	}

	sort.Slice(out, func(i, j int) bool {
		if out[i].Name != out[j].Name {
			return out[i].Name < out[j].Name
		}

		return out[i].ID < out[j].ID
	})

	return out
}

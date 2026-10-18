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

// ErrTaxIDTaken is a registration whose tax code is already a registered
// client's.
//
// It is an error rather than a warning, unlike a duplicate tax code in the
// partner directory. A partner may be a one-man supplier with no tax code and a
// payment to chase; a client is a company whose books the practice is legally
// keeping, and the same tax code means the same company. Registering it twice
// would split one company's books in two, which is a mistake nobody can undo
// afterwards by looking at a warning.
var ErrTaxIDTaken = errors.New("client: this tax code is already registered")

// Registry is the practice's list of clients.
//
// It holds no financial data of its own: which clients exist, what they are
// called, and whether they are still being served. Books are reached through a
// Scope, never through this interface, and there is deliberately no method here
// that returns one.
type Registry interface {
	// Create registers a client and sets its books up.
	//
	// Registration is one act, not two. A client that has been registered but
	// whose chart of accounts was never installed cannot keep books, and the
	// failure surfaces later as an empty ledger somebody has to diagnose. So
	// provisioning is part of Create, and a client whose books could not be
	// set up is not registered at all.
	Create(ctx context.Context, c Client) error

	// Get returns one client. It answers for a client that has been closed
	// as well as an open one; deciding what to do about a closed client
	// belongs to the caller.
	Get(ctx context.Context, id string) (Client, error)

	// List returns every client, by name.
	List(ctx context.Context) ([]Client, error)

	// Update replaces a registered client's details. It fails if the id is
	// unknown: an update that matched nothing must not look like a success.
	//
	// It does not change whether the client is being served — that is
	// SetActive — and it does not restamp CreatedAt.
	Update(ctx context.Context, c Client) error

	// SetActive opens or closes a client. A closed client's books are kept;
	// it is simply no longer served, and requests for it are refused.
	SetActive(ctx context.Context, id string, active bool) error
}

// Provisioner installs the books a new client needs before any of them can be
// used.
//
// It is a separate interface from Registry because it is a separate concern:
// the registry is the practice's list, and provisioning is work done inside one
// client's books. Keeping them apart is also what makes the rule testable — a
// test can prove that a client is not registered when its books cannot be set
// up, without a database or a chart.
type Provisioner interface {
	Provision(ctx context.Context, c Client) error
}

// MemoryRegistry is an in-memory client registry.
//
// It applies the same rules as a stored registry — normalisation, validation,
// one id and one tax code per client, and provisioning as part of Create — and
// forgets everything when the process exits. It exists so that a rule about
// clients can be tested without a database, and is a fixture rather than a
// store.
//
// A nil Provisioner registers clients without setting any books up, which is
// what a test that only cares about the registry itself wants. Production
// wiring passes a ChartProvisioner.
type MemoryRegistry struct {
	mu      sync.RWMutex
	clients map[string]Client

	provisioner Provisioner
}

// A compile-time statement of intent: the memory registry is a Registry.
var _ Registry = (*MemoryRegistry)(nil)

// NewMemoryRegistry returns an empty registry that provisions a new client's
// books with p. Pass nil for a registry that only keeps the list.
func NewMemoryRegistry(p Provisioner) *MemoryRegistry {
	return &MemoryRegistry{provisioner: p}
}

// Create registers a client and sets its books up.
func (r *MemoryRegistry) Create(ctx context.Context, c Client) error {
	c = c.Normalized()
	if c.CreatedAt.IsZero() {
		c.CreatedAt = time.Now().UTC()
	}
	if err := c.Validate(); err != nil {
		return err
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.clients[c.ID]; exists {
		return fmt.Errorf("%w: %s", ErrClientExists, c.ID)
	}
	if holder, taken := r.holder(c.TaxID); taken {
		return fmt.Errorf("%w: %s is client %s", ErrTaxIDTaken, c.TaxID, holder)
	}

	// Books before paperwork. If the chart cannot be installed the client is
	// not registered, so there is never a client the practice cannot keep
	// books for.
	if r.provisioner != nil {
		if err := r.provisioner.Provision(ctx, c); err != nil {
			return fmt.Errorf("register client %s: %w", c.ID, err)
		}
	}

	if r.clients == nil {
		r.clients = make(map[string]Client)
	}
	r.clients[c.ID] = c

	return nil
}

// Get returns one client.
func (r *MemoryRegistry) Get(_ context.Context, id string) (Client, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	c, ok := r.clients[strings.TrimSpace(id)]
	if !ok {
		return Client{}, fmt.Errorf("%w: %s", ErrUnknownClient, id)
	}

	return c, nil
}

// List returns every client, by name and then id.
func (r *MemoryRegistry) List(_ context.Context) ([]Client, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.sortedLocked(), nil
}

// Update replaces a registered client's details.
func (r *MemoryRegistry) Update(_ context.Context, c Client) error {
	c = c.Normalized()
	if err := c.Validate(); err != nil {
		return err
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	existing, ok := r.clients[c.ID]
	if !ok {
		return fmt.Errorf("%w: %s", ErrUnknownClient, c.ID)
	}
	if holder, taken := r.holder(c.TaxID); taken && holder != c.ID {
		return fmt.Errorf("%w: %s is client %s", ErrTaxIDTaken, c.TaxID, holder)
	}

	// An update carries what is being changed. A caller that does not repeat
	// the creation instant must not erase it, and must not be able to move it.
	c.CreatedAt = existing.CreatedAt
	c.Active = existing.Active

	r.clients[c.ID] = c

	return nil
}

// SetActive opens or closes a client.
func (r *MemoryRegistry) SetActive(_ context.Context, id string, active bool) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	c, ok := r.clients[strings.TrimSpace(id)]
	if !ok {
		return fmt.Errorf("%w: %s", ErrUnknownClient, id)
	}

	c.Active = active
	r.clients[c.ID] = c

	return nil
}

// holder returns the client already registered against a tax code, if any.
//
// An empty tax code is not held by anybody: a client may be engaged before its
// paperwork is to hand, and several such clients may exist at once. The empty
// string is exempt for the same reason NULL is exempt from the unique key that
// enforces this rule in the database.
func (r *MemoryRegistry) holder(taxID string) (string, bool) {
	if taxID == "" {
		return "", false
	}

	for id, c := range r.clients {
		if c.TaxID == taxID {
			return id, true
		}
	}

	return "", false
}

// sortedLocked returns every client by name and then id. The order is the
// registry's, not the map's: a caller rendering the list must not depend on Go's
// map iteration order.
func (r *MemoryRegistry) sortedLocked() []Client {
	out := make([]Client, 0, len(r.clients))
	for _, c := range r.clients {
		out = append(out, c)
	}

	sort.Slice(out, func(i, j int) bool {
		if out[i].Name != out[j].Name {
			return out[i].Name < out[j].Name
		}

		return out[i].ID < out[j].ID
	})

	return out
}

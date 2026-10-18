package client

import (
	"context"
	"fmt"
	"sync"

	"fingo/internal/domain/gl"
)

// ChartStore holds one chart of accounts per client.
//
// Every method takes a Scope, so there is no way to read or write a chart
// without saying whose it is — the same rule the financial repositories follow.
// The accounts are gl.Account values rather than this package's own type,
// because a chart's rules (numeric codes, a parent that exists, no cycles) are
// the ledger's rules and are already stated once in internal/domain/gl.
type ChartStore interface {
	// SaveChart replaces a client's chart of accounts, refusing one that is
	// not internally consistent.
	SaveChart(ctx context.Context, scope Scope, accounts []gl.Account) error

	// Chart returns a client's chart of accounts, loaded and validated.
	Chart(ctx context.Context, scope Scope) (*gl.ChartOfAccounts, error)
}

// DefaultChart is the chart a newly registered client is given.
//
// TT133, the regime FinGo v1 implements: Phụ lục 1 of Circular 133/2016/TT-BTC
// as amended by Circular 46/2025/TT-BTC. It is a function rather than a slice
// variable so that a caller cannot mutate the chart every client is seeded from.
func DefaultChart() []gl.Account { return gl.TT133Chart() }

// ChartProvisioner gives a new client the standard chart of accounts for its
// regime.
//
// This is what makes onboarding one act. A registered client with no chart has
// books nothing can be posted to, and the operator discovers it at the first
// entry rather than at registration, when the fix was obvious.
type ChartProvisioner struct {
	charts ChartStore
	chart  func() []gl.Account
}

// NewChartProvisioner provisions the TT133 chart into a chart store.
func NewChartProvisioner(charts ChartStore) *ChartProvisioner {
	return &ChartProvisioner{charts: charts, chart: DefaultChart}
}

// WithChart replaces the chart a new client is given, for a client on another
// regime. It returns the provisioner so it can be built in one expression.
func (p *ChartProvisioner) WithChart(chart func() []gl.Account) *ChartProvisioner {
	if chart != nil {
		p.chart = chart
	}

	return p
}

// Provision installs the default chart for a newly registered client.
func (p *ChartProvisioner) Provision(ctx context.Context, c Client) error {
	scope, err := ScopeFor(c.ID)
	if err != nil {
		return err
	}

	accounts := p.chart()
	if len(accounts) == 0 {
		return fmt.Errorf("the default chart of accounts for %s is empty; a client with no accounts cannot keep books", c.Name)
	}

	if err := p.charts.SaveChart(ctx, scope, accounts); err != nil {
		return fmt.Errorf("install the default chart of accounts: %w", err)
	}

	return nil
}

// MemoryChartStore is an in-memory chart store, one chart per client.
//
// It applies the same rules as a stored chart store — the chart must be
// internally consistent, and it is filed under the client the scope names — and
// forgets everything when the process exits. It is a fixture rather than a
// store: it is safe for concurrent use, but nothing durable may depend on it.
//
// The zero value is an empty store.
type MemoryChartStore struct {
	mu     sync.RWMutex
	charts map[string]*gl.ChartOfAccounts
}

// A compile-time statement of intent: the memory chart store is a ChartStore.
var _ ChartStore = (*MemoryChartStore)(nil)

// NewMemoryChartStore returns an empty chart store.
func NewMemoryChartStore() *MemoryChartStore { return &MemoryChartStore{} }

// SaveChart replaces a client's chart, refusing one that does not hold together.
func (s *MemoryChartStore) SaveChart(_ context.Context, scope Scope, accounts []gl.Account) error {
	if err := scope.guard(); err != nil {
		return err
	}

	chart := gl.NewChartOfAccounts()
	for _, a := range accounts {
		if err := chart.Add(a); err != nil {
			return fmt.Errorf("the chart of accounts for client %s is not usable: %w", scope.ClientID(), err)
		}
	}

	// Parents are checked after every account is in: a chart's order must not
	// decide whether it is valid.
	if err := chart.Validate(); err != nil {
		return fmt.Errorf("the chart of accounts for client %s is not usable: %w", scope.ClientID(), err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.charts == nil {
		s.charts = make(map[string]*gl.ChartOfAccounts)
	}
	s.charts[scope.ClientID()] = chart

	return nil
}

// Chart returns a client's chart.
//
// The chart is shared, not copied: gl.ChartOfAccounts is a mutable index and
// copying it on every read would cost more than a fixture is worth. A caller
// must therefore treat what it gets back as read-only, and a store that matters
// should hand out a chart loaded for the request.
func (s *MemoryChartStore) Chart(_ context.Context, scope Scope) (*gl.ChartOfAccounts, error) {
	if err := scope.guard(); err != nil {
		return nil, err
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	chart, ok := s.charts[scope.ClientID()]
	if !ok {
		return nil, fmt.Errorf("%w: client %s has no chart of accounts", ErrUnknownClient, scope.ClientID())
	}

	return chart, nil
}

// ProvisionerFunc adapts a function to the Provisioner interface, for a test or
// a one-off job that has no reason to be a type.
type ProvisionerFunc func(ctx context.Context, c Client) error

// Provision calls f.
func (f ProvisionerFunc) Provision(ctx context.Context, c Client) error { return f(ctx, c) }

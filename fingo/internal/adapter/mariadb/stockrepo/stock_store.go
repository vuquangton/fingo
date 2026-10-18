// Package stockrepo stores a client's stock movements and the valuation method
// each fiscal year is kept by.
package stockrepo

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	mariadb "fingo/internal/adapter/mariadb/sqlc"
	"fingo/internal/domain/inventory"
)

// Store is the MariaDB adapter for stock movements and valuation policy.
type Store struct {
	db      *sql.DB
	queries *mariadb.Queries
}

func NewStore(db *sql.DB) *Store {
	return &Store{db: db, queries: mariadb.New(db)}
}

// Position replays one item's movements into its current balance.
func (s *Store) Position(ctx context.Context, itemCode, warehouseCode string) (inventory.Stock, error) {
	return PositionWithin(ctx, s.queries, itemCode, warehouseCode)
}

// PositionWithin replays a position using queries that may already be inside a
// transaction.
//
// A posting re-checks the cost it was built from against this, inside its own
// transaction, so two postings cannot both take the same stock at the same
// average.
func PositionWithin(
	ctx context.Context,
	q *mariadb.Queries,
	itemCode, warehouseCode string,
) (inventory.Stock, error) {
	rows, err := q.ListStockMovements(ctx, mariadb.ListStockMovementsParams{
		ItemCode:      itemCode,
		WarehouseCode: warehouseCode,
	})
	if err != nil {
		return inventory.Stock{}, fmt.Errorf("list movements for %s in %s: %w", itemCode, warehouseCode, err)
	}

	if len(rows) == 0 {
		// Nothing has ever moved, which is an empty position rather than an
		// error: the client simply does not stock this item yet.
		return inventory.NewStock(itemCode, warehouseCode), nil
	}

	movements, err := movementsOf(rows)
	if err != nil {
		return inventory.Stock{}, err
	}

	return inventory.Replay(movements)
}

// Positions returns every item and warehouse that has moved, with its balance.
func (s *Store) Positions(ctx context.Context) ([]inventory.Stock, error) {
	positions, err := s.queries.ListStockPositions(ctx)
	if err != nil {
		return nil, fmt.Errorf("list stock positions: %w", err)
	}

	out := make([]inventory.Stock, 0, len(positions))
	for _, p := range positions {
		position, err := s.Position(ctx, p.ItemCode, p.WarehouseCode)
		if err != nil {
			return nil, err
		}
		out = append(out, position)
	}

	return out, nil
}

// Record appends a movement to the log.
func (s *Store) Record(ctx context.Context, m inventory.Movement) error {
	if !m.Kind.Valid() {
		return fmt.Errorf("stock movement kind %q is neither a receipt nor an issue", m.Kind)
	}
	if m.ItemCode == "" {
		return fmt.Errorf("a stock movement needs an item")
	}

	if err := s.queries.CreateStockMovement(ctx, mariadb.CreateStockMovementParams{
		ID:            uuid.NewString(),
		ItemCode:      m.ItemCode,
		WarehouseCode: m.WarehouseCode,
		Kind:          mariadb.StockMovementsKind(m.Kind),
		Quantity:      m.Quantity.String(),
		Cost:          m.Cost.String(),
		VoucherID:     sql.NullString{String: m.VoucherID, Valid: m.VoucherID != ""},
		MovedAt:       m.MovedAt.UTC(),
		Note:          sql.NullString{},
	}); err != nil {
		return fmt.Errorf("record movement of %s in %s: %w", m.ItemCode, m.WarehouseCode, err)
	}

	return nil
}

// LoadPolicy reads the valuation method chosen for each fiscal year.
func (s *Store) LoadPolicy(ctx context.Context) (*inventory.Policy, error) {
	rows, err := s.queries.ListInventoryPolicies(ctx)
	if err != nil {
		return nil, fmt.Errorf("list inventory policies: %w", err)
	}

	policy := inventory.NewPolicy()
	for _, r := range rows {
		// Loaded through Set, which re-applies the within-year consistency rule.
		// Two methods for one year is impossible under the primary key, so this
		// cannot fail — and if it ever did, the caller should hear about it.
		if err := policy.Set(int(r.FiscalYear), inventory.Method(r.Method)); err != nil {
			return nil, fmt.Errorf("read valuation policy for %d: %w", r.FiscalYear, err)
		}
	}

	return policy, nil
}

// SavePolicy records the method chosen for a fiscal year.
func (s *Store) SavePolicy(ctx context.Context, year int, method inventory.Method, at time.Time) error {
	if err := s.queries.SetInventoryPolicy(ctx, mariadb.SetInventoryPolicyParams{
		FiscalYear: int16(year),
		Method:     mariadb.InventoryPoliciesMethod(method),
		ChosenAt:   at.UTC(),
	}); err != nil {
		return fmt.Errorf("record valuation method %s for %d: %w", method, year, err)
	}

	return nil
}

func movementsOf(rows []mariadb.ListStockMovementsRow) ([]inventory.Movement, error) {
	out := make([]inventory.Movement, 0, len(rows))

	for _, r := range rows {
		quantity, err := decimal.NewFromString(r.Quantity)
		if err != nil {
			return nil, fmt.Errorf("movement %s quantity: %w", r.ID, err)
		}
		cost, err := decimal.NewFromString(r.Cost)
		if err != nil {
			return nil, fmt.Errorf("movement %s cost: %w", r.ID, err)
		}

		out = append(out, inventory.Movement{
			ID:            r.ID,
			ItemCode:      r.ItemCode,
			WarehouseCode: r.WarehouseCode,
			Kind:          inventory.Kind(r.Kind),
			Quantity:      quantity,
			Cost:          cost,
			VoucherID:     r.VoucherID.String,
			MovedAt:       r.MovedAt,
		})
	}

	return out, nil
}

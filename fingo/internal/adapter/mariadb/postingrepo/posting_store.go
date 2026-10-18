// Package postingrepo writes a posting — a voucher together with everything the
// document it records entails — in one transaction.
//
// The need is structural. Recording a sale moves money, takes stock out of the
// warehouse, and records the invoice it was billed on; those are one event and
// must all happen or none. The ledger's own save path owns a transaction and so
// does the stock log; neither can span the other, so the transaction lives here
// and each store contributes its writes to it.
//
// Every write in this package that could disagree with itself belongs here for
// the same reason. When in doubt: if two rows describe one event, they commit
// together.
package postingrepo

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"fingo/internal/adapter/mariadb/glrepo"
	mariadb "fingo/internal/adapter/mariadb/sqlc"
	"fingo/internal/adapter/mariadb/stockrepo"
	"fingo/internal/domain/document"
	"fingo/internal/domain/inventory"
	"fingo/internal/usecase/ledger"
)

var _ ledger.Repository = (*Store)(nil)

// ErrStockMoved reports that a posting's stock cost no longer matches the
// position it was resolved from.
var ErrStockMoved = fmt.Errorf("stock moved while this posting was being prepared")

// Store writes postings.
type Store struct {
	db      *sql.DB
	queries *mariadb.Queries
}

func NewStore(db *sql.DB) *Store {
	return &Store{db: db, queries: mariadb.New(db)}
}

// Save writes the whole posting in one transaction.
//
// The order within it is the lifecycle the immutability trigger enforces: the
// voucher is stored as a draft, gets its lines, and is only then marked posted.
// The transaction keeps that intermediate state invisible to everybody else.
func (s *Store) Save(ctx context.Context, p ledger.Posting) error {
	if p.Voucher == nil {
		return fmt.Errorf("a posting needs a voucher")
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("post %s: begin: %w", p.Voucher.ID(), err)
	}
	defer func() { _ = tx.Rollback() }()

	q := s.queries.WithTx(tx)

	// Check the stock before writing anything, so a posting that cannot take the
	// goods it names is refused without a row being touched.
	//
	// It is only a check. The movements are written after the voucher, because
	// voucher_id is a foreign key and the voucher does not exist until then.
	if err := s.checkStock(ctx, q, p); err != nil {
		return err
	}

	if err := glrepo.SaveWithin(ctx, q, p.Voucher, p.RequestKey); err != nil {
		return err
	}

	// The cost of what the posting moved, written inside the same transaction
	// and therefore committed with it. No idempotency key: the key names the
	// request, and a request is one event — the cost voucher is half of the
	// answer to it, not a second one.
	if p.Cost != nil {
		if err := glrepo.SaveWithin(ctx, q, p.Cost, ""); err != nil {
			return err
		}
	}

	for _, issue := range p.StockIssues {
		if err := s.writeMovement(ctx, q, movement{
			kind:          inventory.KindIssue,
			itemCode:      issue.ItemCode,
			warehouseCode: issue.WarehouseCode,
			quantity:      issue.Quantity,
			cost:          issue.Cost,
			voucherID:     p.Voucher.ID(),
			at:            p.At,
		}); err != nil {
			return err
		}
	}

	for _, receipt := range p.StockReceipts {
		if err := s.writeMovement(ctx, q, movement{
			kind:          inventory.KindReceipt,
			itemCode:      receipt.ItemCode,
			warehouseCode: receipt.WarehouseCode,
			quantity:      receipt.Quantity,
			cost:          receipt.Cost,
			voucherID:     p.Voucher.ID(),
			at:            p.At,
		}); err != nil {
			return err
		}
	}

	if p.Document != nil {
		if err := s.recordDocument(ctx, q, *p.Document); err != nil {
			return err
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("post %s: commit: %w", p.Voucher.ID(), err)
	}

	return nil
}

// checkStock verifies that the posting can take and add the goods it names,
// without writing anything.
//
// The running positions are held per item and warehouse so several lines of the
// same item are checked against each other, not merely against what was there
// before the posting started. Two lines that together overdraw a warehouse must
// be refused even though neither would on its own.
func (s *Store) checkStock(ctx context.Context, q *mariadb.Queries, p ledger.Posting) error {
	held := make(map[string]inventory.Stock)

	running := func(itemCode, warehouseCode string) (inventory.Stock, error) {
		key := itemCode + "\x00" + warehouseCode
		if position, ok := held[key]; ok {
			return position, nil
		}

		position, err := stockrepo.PositionWithin(ctx, q, itemCode, warehouseCode)
		if err != nil {
			return inventory.Stock{}, err
		}
		held[key] = position

		return position, nil
	}

	for _, issue := range p.StockIssues {
		position, err := running(issue.ItemCode, issue.WarehouseCode)
		if err != nil {
			return err
		}

		next, cost, err := position.Issue(issue.Quantity)
		if err != nil {
			return fmt.Errorf("post %s: %w", p.Voucher.ID(), err)
		}

		if !cost.Equal(issue.Cost) {
			return fmt.Errorf("%w: %s in %s was costed at %s đồng but now issues at %s",
				ErrStockMoved, issue.ItemCode, issue.WarehouseCode, issue.Cost, cost)
		}

		held[issue.ItemCode+"\x00"+issue.WarehouseCode] = next
	}

	for _, receipt := range p.StockReceipts {
		position, err := running(receipt.ItemCode, receipt.WarehouseCode)
		if err != nil {
			return err
		}

		next, err := position.Receive(receipt.Quantity, receipt.Cost)
		if err != nil {
			return fmt.Errorf("post %s: %w", p.Voucher.ID(), err)
		}

		held[receipt.ItemCode+"\x00"+receipt.WarehouseCode] = next
	}

	return nil
}

// movement is one row of the stock log, assembled for writing.
type movement struct {
	kind          inventory.Kind
	itemCode      string
	warehouseCode string
	quantity      decimal.Decimal
	cost          decimal.Decimal
	voucherID     string
	at            time.Time
}

func (s *Store) writeMovement(ctx context.Context, q *mariadb.Queries, m movement) error {
	if err := q.CreateStockMovement(ctx, mariadb.CreateStockMovementParams{
		ID:            uuid.NewString(),
		ItemCode:      m.itemCode,
		WarehouseCode: m.warehouseCode,
		Kind:          mariadb.StockMovementsKind(m.kind),
		Quantity:      m.quantity.String(),
		Cost:          m.cost.String(),
		VoucherID:     sql.NullString{String: m.voucherID, Valid: m.voucherID != ""},
		MovedAt:       m.at.UTC(),
		Note:          sql.NullString{},
	}); err != nil {
		return fmt.Errorf("record the stock movement for %s in %s: %w",
			m.itemCode, m.warehouseCode, err)
	}

	return nil
}

func (s *Store) recordDocument(ctx context.Context, q *mariadb.Queries, d document.Document) error {
	if err := d.Validate(); err != nil {
		return err
	}

	if err := q.CreateDocument(ctx, mariadb.CreateDocumentParams{
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

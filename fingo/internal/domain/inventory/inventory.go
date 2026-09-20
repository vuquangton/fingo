package inventory

import (
	"context"
	"time"

	"github.com/shopspring/decimal"
)

type StockMovementLine struct {
	ID          string
	ItemID      string
	Quantity    decimal.Decimal
	UnitPrice   decimal.Decimal
	WarehouseID string
}

type StockInward struct {
	ID          string
	VoucherNo   string
	WarehouseID string
	Date        time.Time
	Lines       []StockMovementLine
	IsPosted    bool
}

type StockOutward struct {
	ID          string
	VoucherNo   string
	WarehouseID string
	Date        time.Time
	Lines       []StockMovementLine
	IsPosted    bool
}

func NewStockInwardStub(id, voucherNo, warehouseID string, date time.Time) *StockInward {
	return &StockInward{
		ID:          id,
		VoucherNo:   voucherNo,
		WarehouseID: warehouseID,
		Date:        date,
		Lines:       make([]StockMovementLine, 0),
		IsPosted:    false,
	}
}

func (s *StockInward) AddLine(itemID string, qty, unitPrice decimal.Decimal) {
	s.Lines = append(s.Lines, StockMovementLine{
		ID:          itemID + "-line",
		ItemID:      itemID,
		Quantity:    qty,
		UnitPrice:   unitPrice,
		WarehouseID: s.WarehouseID,
	})
}

type InventoryRepositoryStub interface {
	SaveStockInward(ctx context.Context, inward *StockInward) error
	SaveStockOutward(ctx context.Context, outward *StockOutward) error
}

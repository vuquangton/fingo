package opening_test

import (
	"testing"
	"time"

	"github.com/shopspring/decimal"

	"fingo/internal/domain/opening"
)

func TestOpeningBalance_TrialBalanceInvariance(t *testing.T) {
	batch := opening.NewOpeningBatchStub("batch-01", time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))

	batch.AddAccountBalance("1111", decimal.NewFromInt(10000000), decimal.Zero)
	batch.AddAccountBalance("4111", decimal.Zero, decimal.NewFromInt(10000000))

	if err := batch.ValidateBalance(); err != nil {
		t.Fatalf("expected opening batch to balance: %v", err)
	}
}

func TestOpeningBalance_PartnerAndInventoryInvariance(t *testing.T) {
	cob := opening.CustomerOpeningBalance{
		CustomerID:  "cust-1",
		DebitAmount: decimal.NewFromInt(5000000),
	}
	if cob.DebitAmount.IsZero() {
		t.Fatal("expected non-zero customer debt")
	}

	iob := opening.InventoryOpeningBalance{
		WarehouseID: "wh-01",
		ItemID:      "item-01",
		Quantity:    decimal.NewFromInt(10),
		UnitCost:    decimal.NewFromInt(100000),
	}
	if iob.TotalAmount().Cmp(decimal.NewFromInt(1000000)) != 0 {
		t.Fatalf("expected 1,000,000 inventory value, got %s", iob.TotalAmount())
	}
}

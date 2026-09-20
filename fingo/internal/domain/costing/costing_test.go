package costing_test

import (
	"testing"

	"github.com/shopspring/decimal"

	"fingo/internal/domain/costing"
)

func TestProductionCost_Allocation(t *testing.T) {
	card := costing.NewCostCardStub("p-01", "PROD_TABLE_01", decimal.NewFromInt(10))

	// Direct materials (621/154): 10,000,000
	card.DirectMaterialCost = decimal.NewFromInt(10000000)
	// Direct labor (622/154): 5,000,000
	card.DirectLaborCost = decimal.NewFromInt(5000000)
	// Factory overhead (627/154): 2,000,000
	card.OverheadCost = decimal.NewFromInt(2000000)

	totalCost := card.CalculateTotalCost()
	expectedTotal := decimal.NewFromInt(17000000)
	if totalCost.Cmp(expectedTotal) != 0 {
		t.Fatalf("expected total cost %s, got %s", expectedTotal, totalCost)
	}

	unitCost := card.CalculateUnitCost()
	expectedUnit := decimal.NewFromInt(1700000) // 17,000,000 / 10 = 1,700,000
	if unitCost.Cmp(expectedUnit) != 0 {
		t.Fatalf("expected unit cost %s, got %s", expectedUnit, unitCost)
	}
}

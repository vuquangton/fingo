package costing

import (
	"context"

	"github.com/shopspring/decimal"
)

type ProductionCostCard struct {
	ID                 string
	ProductCode        string
	CompletedQuantity  decimal.Decimal
	DirectMaterialCost decimal.Decimal // TK 621 / 154
	DirectLaborCost    decimal.Decimal // TK 622 / 154
	OverheadCost       decimal.Decimal // TK 627 / 154
	WIPBeginning       decimal.Decimal // Dở dang đầu kỳ
	WIPEnding          decimal.Decimal // Dở dang cuối kỳ
}

func NewCostCardStub(id, productCode string, completedQty decimal.Decimal) *ProductionCostCard {
	return &ProductionCostCard{
		ID:                 id,
		ProductCode:        productCode,
		CompletedQuantity:  completedQty,
		DirectMaterialCost: decimal.Zero,
		DirectLaborCost:    decimal.Zero,
		OverheadCost:       decimal.Zero,
		WIPBeginning:       decimal.Zero,
		WIPEnding:          decimal.Zero,
	}
}

// CalculateTotalCost = WIPBeginning + (DirectMaterial + DirectLabor + Overhead) - WIPEnding
func (c *ProductionCostCard) CalculateTotalCost() decimal.Decimal {
	periodExpenses := c.DirectMaterialCost.Add(c.DirectLaborCost).Add(c.OverheadCost)
	return c.WIPBeginning.Add(periodExpenses).Sub(c.WIPEnding)
}

func (c *ProductionCostCard) CalculateUnitCost() decimal.Decimal {
	if c.CompletedQuantity.LessThanOrEqual(decimal.Zero) {
		return decimal.Zero
	}
	return c.CalculateTotalCost().Div(c.CompletedQuantity)
}

type CostingRepositoryStub interface {
	SaveCostCard(ctx context.Context, card *ProductionCostCard) error
	GetCostCard(ctx context.Context, id string) (*ProductionCostCard, error)
}

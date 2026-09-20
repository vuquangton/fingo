package asset

import (
	"context"
	"time"

	"github.com/shopspring/decimal"
)

type FixedAsset struct {
	ID                     string
	Code                   string
	Name                   string
	OriginalCost           decimal.Decimal // Nguyên giá (211)
	AccumulatedDep         decimal.Decimal // Hao mòn lũy kế (214)
	UsefulLifeMonths       int             // Thời gian sử dụng (tháng)
	PurchaseDate           time.Time
	DepreciationStartDate  time.Time
	CostAccount            string // 211
	DepreciationAccount    string // 214
	ExpenseAccount         string // 642, 627, 154
	IsActive               bool
}

func NewFixedAssetStub(id, code, name string, cost decimal.Decimal, months int, purchaseDate time.Time) *FixedAsset {
	return &FixedAsset{
		ID:                    id,
		Code:                  code,
		Name:                  name,
		OriginalCost:          cost,
		AccumulatedDep:        decimal.Zero,
		UsefulLifeMonths:      months,
		PurchaseDate:          purchaseDate,
		DepreciationStartDate: purchaseDate,
		CostAccount:           "2111",
		DepreciationAccount:   "2141",
		ExpenseAccount:        "6422",
		IsActive:              true,
	}
}

func (fa *FixedAsset) CalculateMonthlyDepreciation() decimal.Decimal {
	if fa.UsefulLifeMonths <= 0 {
		return decimal.Zero
	}
	return fa.OriginalCost.Div(decimal.NewFromInt(int64(fa.UsefulLifeMonths)))
}

type AssetRepositoryStub interface {
	SaveFixedAsset(ctx context.Context, fa *FixedAsset) error
	GetFixedAsset(ctx context.Context, id string) (*FixedAsset, error)
}

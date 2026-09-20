package asset_test

import (
	"testing"
	"time"

	"github.com/shopspring/decimal"

	"fingo/internal/domain/asset"
)

func TestFixedAsset_DepreciationRate(t *testing.T) {
	fa := asset.NewFixedAssetStub("fa-1", "TS001", "MacBook Pro M3", decimal.NewFromInt(60000000), 36, time.Now())
	monthlyDep := fa.CalculateMonthlyDepreciation()
	expected := decimal.NewFromInt(60000000).Div(decimal.NewFromInt(36))

	if monthlyDep.Cmp(expected) != 0 {
		t.Fatalf("expected monthly dep %s, got %s", expected, monthlyDep)
	}
}

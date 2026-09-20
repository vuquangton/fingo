package inventory_test

import (
	"testing"
	"time"

	"github.com/shopspring/decimal"

	"fingo/internal/domain/inventory"
)

func TestStockInward_StubInvariants(t *testing.T) {
	in := inventory.NewStockInwardStub("si-01", "PNK001", "KHO_CHINH", time.Now())
	in.AddLine("VT01", decimal.NewFromInt(50), decimal.NewFromInt(20000))

	if len(in.Lines) != 1 || in.Lines[0].Quantity.Cmp(decimal.NewFromInt(50)) != 0 {
		t.Fatalf("unexpected stock inward line: %+v", in.Lines)
	}
}

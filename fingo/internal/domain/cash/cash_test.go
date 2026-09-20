package cash_test

import (
	"testing"
	"time"

	"github.com/shopspring/decimal"

	"fingo/internal/domain/cash"
)

func TestCashReceipt_StubInvariants(t *testing.T) {
	cr := cash.NewCashReceiptStub("cr-01", "PT001", time.Now(), decimal.NewFromInt(500000))
	if cr.VoucherNo != "PT001" || cr.Amount.Cmp(decimal.NewFromInt(500000)) != 0 {
		t.Fatalf("unexpected cash receipt stub: %+v", cr)
	}
}

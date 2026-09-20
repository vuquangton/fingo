package gl_test

import (
	"testing"
	"time"

	"github.com/shopspring/decimal"

	"fingo/internal/domain/gl"
)

func TestVoucher_DoubleEntryBalanceCheck(t *testing.T) {
	voucher := gl.NewVoucherStub("v-1", "PK001", gl.VoucherTypeGeneral, time.Now())

	voucher.AddLine(gl.VoucherLine{
		ID:              "l-1",
		DebitAccountID:  "1111",
		CreditAccountID: "5111",
		Amount:          decimal.NewFromInt(100000),
	})

	if err := voucher.ValidateBalance(); err != nil {
		t.Fatalf("expected voucher to be balanced: %v", err)
	}
}

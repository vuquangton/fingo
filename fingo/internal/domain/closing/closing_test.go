package closing_test

import (
	"testing"
	"time"

	"github.com/shopspring/decimal"

	"fingo/internal/domain/closing"
)

func TestClosing_NetProfitCalculation(t *testing.T) {
	cl := closing.NewPeriodClosingStub("cl-2026-01", 2026, 1, time.Date(2026, 1, 31, 23, 59, 59, 0, time.UTC))

	cl.AddRevenue("5111", decimal.NewFromInt(100000000))
	cl.AddExpense("632", decimal.NewFromInt(60000000))
	cl.AddExpense("642", decimal.NewFromInt(15000000))

	netProfit := cl.CalculateNetProfit()
	expected := decimal.NewFromInt(25000000) // 100M - 60M - 15M = 25M

	if netProfit.Cmp(expected) != 0 {
		t.Fatalf("expected profit %s, got %s", expected, netProfit)
	}
}

func TestClosing_LockDateValidation(t *testing.T) {
	lockDate := time.Date(2026, 1, 31, 0, 0, 0, 0, time.UTC)
	config := closing.SystemLockConfig{LockDate: lockDate}

	voucherDateBefore := time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC)
	if err := config.CheckVoucherDateAllowed(voucherDateBefore); err == nil {
		t.Fatal("expected error for voucher before lock date")
	}

	voucherDateAfter := time.Date(2026, 2, 5, 0, 0, 0, 0, time.UTC)
	if err := config.CheckVoucherDateAllowed(voucherDateAfter); err != nil {
		t.Fatalf("unexpected error for voucher after lock date: %v", err)
	}
}

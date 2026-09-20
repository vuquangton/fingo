package report_test

import (
	"testing"

	"github.com/shopspring/decimal"

	"fingo/internal/domain/report"
)

func TestTrialBalance_Invariance(t *testing.T) {
	tb := report.NewTrialBalanceStub(2026, 1)

	tb.AddRow("1111", "Tiền mặt",
		decimal.NewFromInt(1000), decimal.Zero,
		decimal.NewFromInt(500), decimal.NewFromInt(200),
		decimal.NewFromInt(1300), decimal.Zero,
	)

	tb.AddRow("4111", "Vốn đầu tư của chủ sở hữu",
		decimal.Zero, decimal.NewFromInt(1000),
		decimal.Zero, decimal.NewFromInt(300),
		decimal.Zero, decimal.NewFromInt(1300),
	)

	if err := tb.ValidateBalance(); err != nil {
		t.Fatalf("expected trial balance to balance: %v", err)
	}
}

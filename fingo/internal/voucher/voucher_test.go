package voucher_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	"fingo/internal/money"
	"fingo/internal/voucher"
)

func day(year int, month time.Month, dayOfMonth int) time.Time {
	return time.Date(year, month, dayOfMonth, 0, 0, 0, 0, time.UTC)
}

// balancedSale is the entry this engine records most: a credit sale with output
// VAT. Three accounts, two of them credited — which is exactly the shape a
// debit/credit pair could not express.
func balancedSale() voucher.Voucher {
	return voucher.Voucher{
		ID:          "v-1",
		Number:      "INV-2026-00001",
		Date:        day(2026, 3, 14),
		Description: "Bán hàng cho khách A",
		PeriodID:    "fy2026",
		Lines: []voucher.Line{
			{AccountID: "131", Debit: money.New(110, "VND")},
			{AccountID: "5111", Credit: money.New(100, "VND")},
			{AccountID: "33311", Credit: money.New(10, "VND")},
		},
	}
}

func TestValidateBalance_AcceptsABalancedVoucher(t *testing.T) {
	if err := balancedSale().ValidateBalance(); err != nil {
		t.Fatalf("a balanced voucher must be accepted: %v", err)
	}
}

// Unbalanced voucher → ErrUnbalanced.
func TestValidateBalance_RefusesAnUnbalancedVoucher(t *testing.T) {
	cases := map[string]voucher.Voucher{
		"debits short": func() voucher.Voucher {
			v := balancedSale()
			v.Lines[0].Debit = money.New(100, "VND")
			return v
		}(),
		"credits short": func() voucher.Voucher {
			v := balancedSale()
			v.Lines[1].Credit = money.New(90, "VND")
			return v
		}(),
		"debits long": func() voucher.Voucher {
			v := balancedSale()
			v.Lines[0].Debit = money.New(120, "VND")
			return v
		}(),
		"one line only": {
			ID: "v-2", Number: "INV-2026-00002", Date: day(2026, 3, 14),
			Description: "x",
			Lines:       []voucher.Line{{AccountID: "131", Debit: money.New(100, "VND")}},
		},
	}

	for name, v := range cases {
		t.Run(name, func(t *testing.T) {
			err := v.ValidateBalance()
			if err == nil {
				t.Fatalf("%s must be refused, got nil", name)
			}
			if !errors.Is(err, voucher.ErrUnbalanced) {
				t.Fatalf("error = %v, want ErrUnbalanced", err)
			}
		})
	}
}

// The error says by how much, because "unbalanced" alone does not tell the
// person reading it which side to look at.
func TestValidateBalance_SaysHowFarOutTheVoucherIs(t *testing.T) {
	v := balancedSale()
	v.Lines[0].Debit = money.New(150, "VND")

	err := v.ValidateBalance()
	if err == nil {
		t.Fatal("expected an error")
	}

	for _, part := range []string{"150", "110", "40"} {
		if !strings.Contains(err.Error(), part) {
			t.Errorf("error %q should mention %s: debits 150, credits 110, out by 40", err, part)
		}
	}
}

// A voucher with no lines is not balanced, it is empty — and saying "unbalanced"
// about it would be technically true and practically useless.
func TestValidateBalance_RefusesAVoucherWithNoLines(t *testing.T) {
	v := balancedSale()
	v.Lines = nil

	if err := v.ValidateBalance(); !errors.Is(err, voucher.ErrNoLines) {
		t.Fatalf("error = %v, want ErrNoLines", err)
	}
}

// A line is one account movement. It has a side or it is not a movement.
func TestValidateBalance_RefusesALineThatIsNeitherDebitNorCredit(t *testing.T) {
	v := balancedSale()
	v.Lines = append(v.Lines, voucher.Line{AccountID: "1111"})

	if err := v.ValidateBalance(); !errors.Is(err, voucher.ErrLineWithoutSide) {
		t.Fatalf("error = %v, want ErrLineWithoutSide", err)
	}
}

// And it has one side, not two: debiting and crediting the same account on the
// same line nets to nothing and hides the entry.
func TestValidateBalance_RefusesALineWithBothSides(t *testing.T) {
	v := balancedSale()
	v.Lines = append(v.Lines, voucher.Line{
		AccountID: "1111",
		Debit:     money.New(50, "VND"),
		Credit:    money.New(50, "VND"),
	})

	if err := v.ValidateBalance(); !errors.Is(err, voucher.ErrLineWithBothSides) {
		t.Fatalf("error = %v, want ErrLineWithBothSides", err)
	}
}

// A negative amount is a credit written on the wrong side. Refusing it keeps one
// meaning for a debit.
func TestValidateBalance_RefusesANegativeAmount(t *testing.T) {
	v := balancedSale()
	v.Lines[0].Debit = money.New(-110, "VND")

	if err := v.ValidateBalance(); !errors.Is(err, voucher.ErrNegativeAmount) {
		t.Fatalf("error = %v, want ErrNegativeAmount", err)
	}
}

// Adding VND to USD panics in the money package, so a voucher that mixes them
// must be refused before anything tries to total it.
func TestValidateBalance_RefusesMixedCurrencies(t *testing.T) {
	v := balancedSale()
	v.Lines[1].Credit = money.New(100, "USD")

	if err := v.ValidateBalance(); !errors.Is(err, voucher.ErrMixedCurrency) {
		t.Fatalf("error = %v, want ErrMixedCurrency", err)
	}
}

func TestTotalDebitsAndCredits_ReportTheTwoSides(t *testing.T) {
	v := balancedSale()

	debits, err := v.TotalDebits()
	if err != nil {
		t.Fatalf("TotalDebits: %v", err)
	}
	credits, err := v.TotalCredits()
	if err != nil {
		t.Fatalf("TotalCredits: %v", err)
	}

	if debits.Amount() != 110 || debits.Currency() != "VND" {
		t.Errorf("debits = %s, want 110 VND", debits)
	}
	if credits.Amount() != 110 || credits.Currency() != "VND" {
		t.Errorf("credits = %s, want 110 VND", credits)
	}
}

// A voucher needs to be identifiable and datable before it can be anything else.
func TestValidate_RequiresIdentityAndAReasonableDescription(t *testing.T) {
	cases := map[string]func() voucher.Voucher{
		"no id":          func() voucher.Voucher { v := balancedSale(); v.ID = ""; return v },
		"no number":      func() voucher.Voucher { v := balancedSale(); v.Number = ""; return v },
		"no date":        func() voucher.Voucher { v := balancedSale(); v.Date = time.Time{}; return v },
		"no description": func() voucher.Voucher { v := balancedSale(); v.Description = "  "; return v },
	}

	for name, build := range cases {
		t.Run(name, func(t *testing.T) {
			if err := build().Validate(); err == nil {
				t.Fatalf("%s must be refused, got nil", name)
			}
		})
	}

	if err := balancedSale().Validate(); err != nil {
		t.Fatalf("a well-formed voucher must be accepted: %v", err)
	}
}

// Validate is the whole check: identity, then balance.
func TestValidate_IncludesTheBalance(t *testing.T) {
	v := balancedSale()
	v.Lines[0].Debit = money.New(999, "VND")

	if err := v.Validate(); !errors.Is(err, voucher.ErrUnbalanced) {
		t.Fatalf("error = %v, want ErrUnbalanced", err)
	}
}

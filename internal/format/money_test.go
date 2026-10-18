package format_test

import (
	"testing"

	"github.com/shopspring/decimal"

	"fingo/internal/format"
)

// TestMoney_WritesNumbersTheVietnameseWay — slice P1.4.
//
// "1.234.567" is one million two hundred and thirty-four thousand five hundred
// and sixty-seven. A Vietnamese accountant reading "1,234,567" sees a number
// written in a foreign convention, and a report that prints one is a report they
// will not sign.
//
// The comma and the dot are the other way round from English, and getting it
// wrong is not cosmetic: it changes the value by three orders of magnitude when
// the reader applies their own convention.
func TestMoney_WritesNumbersTheVietnameseWay(t *testing.T) {
	tests := []struct {
		name string
		in   string // a decimal literal, so the test states exact values
		want string
	}{
		{"zero", "0", "0"},
		{"a single digit", "7", "7"},
		{"exactly one thousand", "1000", "1.000"},
		{"four digits", "1234", "1.234"},
		{"seven digits", "1234567", "1.234.567"},
		{"ten digits", "1234567890", "1.234.567.890"},
		{"a negative amount", "-1234", "-1.234"},
		{"a negative under a thousand", "-42", "-42"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := format.Money(decimal.RequireFromString(tt.in))
			if got != tt.want {
				t.Errorf("Money(%s) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

// TestMoney_UsesACommaForTheDecimalPart — đồng are not usually written with a
// fraction, but costs and exchange rates are, and when they are the separator is
// a comma. Trailing zeroes are dropped: "1.234,50" is the price of something
// that costs one thousand two hundred and thirty-four and a half, and the
// trailing zero implies a precision the books do not carry.
func TestMoney_UsesACommaForTheDecimalPart(t *testing.T) {
	tests := []struct{ in, want string }{
		{"1234.5", "1.234,5"},
		{"1234.50", "1.234,5"},
		{"1234.567", "1.234,567"},
		{"0.5", "0,5"},
		{"-1234.25", "-1.234,25"},
	}

	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			if got := format.Money(decimal.RequireFromString(tt.in)); got != tt.want {
				t.Errorf("Money(%s) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

// TestMoney_BlankForNothing — a trial balance column full of "0" is unreadable,
// and a ledger where every account has a figure is a ledger where none stands
// out. Zero and empty are different requests, and the caller chooses.
func TestMoney_BlankForNothing(t *testing.T) {
	if got := format.MoneyIfAny(decimal.Zero); got != "" {
		t.Errorf("MoneyIfAny(0) = %q, want an empty cell", got)
	}
	if got := format.MoneyIfAny(decimal.RequireFromString("1234")); got != "1.234" {
		t.Errorf("MoneyIfAny(1234) = %q, want 1.234", got)
	}
}

// TestAmount_ParsesWhatAnAccountantTypes — the inverse, and the one this
// repository has already got wrong once.
//
// `internal/web/render.go`'s formAmount strips commas AND dots, so "1.234,56"
// becomes 123456 — a hundred times the number that was entered, silently, in a
// filing instrument. The rule has to be the mirror of Money: a dot groups, a
// comma separates the fraction, and both are optional because nobody types the
// separators into a small number.
func TestAmount_ParsesWhatAnAccountantTypes(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"a plain integer", "1234", "1234"},
		{"grouped with dots", "1.234", "1234"},
		{"grouped and fractional", "1.234,56", "1234.56"},
		{"only fractional", "0,5", "0.5"},
		{"a negative", "-1.234", "-1234"},
		{"a negative fractional", "-1.234,56", "-1234.56"},
		{"spaces around it", "  1.234  ", "1234"},
		{"a single group of three", "123", "123"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := format.Amount(tt.in)
			if err != nil {
				t.Fatalf("Amount(%q): %v", tt.in, err)
			}
			if !got.Equal(decimal.RequireFromString(tt.want)) {
				t.Errorf("Amount(%q) = %s, want %s", tt.in, got, tt.want)
			}
		})
	}
}

// TestAmount_RoundTripsThroughMoney — whatever the application prints, it must
// be able to read back. A form that shows a number and then refuses it is how
// an operator loses an afternoon.
func TestAmount_RoundTripsThroughMoney(t *testing.T) {
	for _, in := range []string{"1234", "1234567", "0", "-1234", "1234.56"} {
		original := decimal.RequireFromString(in)

		printed := format.Money(original)

		back, err := format.Amount(printed)
		if err != nil {
			t.Fatalf("Amount(Money(%s)) = error: %v (printed %q)", in, err, printed)
		}
		if !back.Equal(original) {
			t.Errorf("%s printed as %q and read back as %s", in, printed, back)
		}
	}
}

// TestAmount_RefusesWhatCannotBeANumber — an entry grid that silently reads a
// word as zero is a grid that posts a zero.
func TestAmount_RefusesWhatCannotBeANumber(t *testing.T) {
	for _, bad := range []string{"abc", "12a3", "1,2,3", "", "   ", "1.2.3", "1,", ",5", "-"} {
		if got, err := format.Amount(bad); err == nil {
			t.Errorf("Amount(%q) = %s, want a refusal", bad, got)
		}
	}
}

// TestAmount_RefusesAnAmbiguousDot — and this is the case that decides whether
// the parser is safe.
//
// In the Vietnamese convention a dot groups digits in threes. "1234.56" groups
// as 1 · 234 · 56, and the last group is two digits, so it is not a Vietnamese
// number at all. The tempting thing is to treat the dot as a decimal point and
// answer 1234.56 — but the operator may equally have meant 1234,56 and typed it
// in the English style, in which case the two readings differ by a factor of a
// hundred.
//
// `internal/web/render.go` already resolves this the wrong way, by stripping
// every separator, so "1.234,56" becomes 123456. Refusing is the only answer
// that cannot be silently wrong.
func TestAmount_RefusesAnAmbiguousDot(t *testing.T) {
	for _, bad := range []string{"1234.56", "1.23", "12.3456", "1.2345"} {
		got, err := format.Amount(bad)
		if err == nil {
			t.Errorf("Amount(%q) = %s, want a refusal — the dot grouping is not groups of three", bad, got)
		}
	}
}

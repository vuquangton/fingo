package money_test

import (
	"testing"

	"fingo/internal/money"
)

func TestNew_StoresAmountAndCurrency(t *testing.T) {
	m := money.New(1234, "VND")

	if m.Amount() != 1234 {
		t.Errorf("Amount() = %d, want 1234", m.Amount())
	}
	if m.Currency() != "VND" {
		t.Errorf("Currency() = %q, want VND", m.Currency())
	}
}

// The zero value is usable: it is 0 with no currency yet, which is what makes
// var total Money; total = total.Add(x) work.
func TestZeroValue_IsValid(t *testing.T) {
	var m money.Money

	if !m.IsZero() {
		t.Error("the zero value must be zero")
	}
	if m.Amount() != 0 {
		t.Errorf("Amount() = %d, want 0", m.Amount())
	}
	if m.Currency() != "" {
		t.Errorf("Currency() = %q, want empty", m.Currency())
	}
	if m.IsNegative() {
		t.Error("zero is not negative")
	}
}

func TestCurrency_Valid(t *testing.T) {
	cases := map[string]bool{
		"VND":  true,
		"USD":  true,
		"EUR":  true,
		"":     false,
		"VN":   false,
		"VNDD": false,
		"vnd":  false,
		"VN1":  false,
		"V D":  false,
		"VNĐ":  false,
	}

	for code, want := range cases {
		if got := money.Currency(code).Valid(); got != want {
			t.Errorf("Currency(%q).Valid() = %v, want %v", code, got, want)
		}
	}
}

// An amount with no currency is a programming error, and it is caught where it
// is made rather than surfacing as a wrong balance later.
func TestNew_PanicsOnAnInvalidCurrency(t *testing.T) {
	for _, code := range []string{"", "VN", "vnd", "VNDD"} {
		t.Run(code, func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Fatalf("New with currency %q must panic", code)
				}
			}()

			money.New(1, money.Currency(code))
		})
	}
}

func TestAdd_SumsTheSameCurrency(t *testing.T) {
	got := money.New(1250, "VND").Add(money.New(750, "VND"))

	if got.Amount() != 2000 || got.Currency() != "VND" {
		t.Errorf("Add = %d %s, want 2000 VND", got.Amount(), got.Currency())
	}
}

// The zero value carries no currency, so it takes the other side's — that is
// what makes it an identity an accumulator can start from.
func TestAdd_ZeroValueIsTheIdentity(t *testing.T) {
	var total money.Money

	total = total.Add(money.New(100, "USD"))
	total = total.Add(money.New(50, "USD"))

	if total.Amount() != 150 || total.Currency() != "USD" {
		t.Errorf("accumulated = %d %s, want 150 USD", total.Amount(), total.Currency())
	}

	// And adding zero to a real amount leaves it alone.
	if got := money.New(100, "USD").Add(money.Money{}); got.Amount() != 100 || got.Currency() != "USD" {
		t.Errorf("100 USD + zero = %d %s, want 100 USD", got.Amount(), got.Currency())
	}
}

func TestAdd_PanicsOnCurrencyMismatch(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("adding VND to USD must panic")
		}
	}()

	money.New(100, "VND").Add(money.New(100, "USD"))
}

// A mismatch panics even when both amounts are zero: the currencies are what
// disagree, and silently picking one would hide the mistake.
func TestAdd_PanicsOnMismatchOfZeroAmounts(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("adding 0 VND to 0 USD must panic")
		}
	}()

	money.New(0, "VND").Add(money.New(0, "USD"))
}

func TestSub_SubtractsTheSameCurrency(t *testing.T) {
	got := money.New(2000, "VND").Sub(money.New(500, "VND"))

	if got.Amount() != 1500 {
		t.Errorf("Sub = %d, want 1500", got.Amount())
	}

	// And it can go negative, which is a real position and not an error.
	if got := money.New(500, "VND").Sub(money.New(2000, "VND")); !got.IsNegative() || got.Amount() != -1500 {
		t.Errorf("Sub = %d, want -1500", got.Amount())
	}
}

func TestSub_PanicsOnCurrencyMismatch(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("subtracting USD from VND must panic")
		}
	}()

	money.New(100, "VND").Sub(money.New(100, "USD"))
}

func TestMul_ScalesExactly(t *testing.T) {
	cases := []struct {
		amount int64
		factor int64
		want   int64
	}{
		{100, 3, 300},
		{100, 0, 0},
		{100, 1, 100},
		{-100, 3, -300},
		{100, -3, -300},
		{7, 1_000_000, 7_000_000},
	}

	for _, c := range cases {
		got := money.New(c.amount, "VND").Mul(c.factor)
		if got.Amount() != c.want {
			t.Errorf("%d × %d = %d, want %d", c.amount, c.factor, got.Amount(), c.want)
		}
		if got.Currency() != "VND" {
			t.Errorf("%d × %d lost the currency", c.amount, c.factor)
		}
	}
}

// Division exposes the remainder rather than hiding it: rounding here is a
// decision for the caller, not for the arithmetic.
func TestDiv_ExposesTheRemainder(t *testing.T) {
	cases := []struct {
		amount    int64
		divisor   int64
		quotient  int64
		remainder int64
	}{
		{1000, 3, 333, 1},
		{1000, 2, 500, 0},
		{7, 7, 1, 0},
		{1, 2, 0, 1},
		{-1000, 3, -333, -1},
	}

	for _, c := range cases {
		q, r := money.New(c.amount, "VND").Div(c.divisor)

		if q.Amount() != c.quotient || r.Amount() != c.remainder {
			t.Errorf("%d ÷ %d = (%d, %d), want (%d, %d)",
				c.amount, c.divisor, q.Amount(), r.Amount(), c.quotient, c.remainder)
		}
		if q.Currency() != "VND" || r.Currency() != "VND" {
			t.Errorf("%d ÷ %d lost the currency", c.amount, c.divisor)
		}
	}
}

// Quotient and remainder reconstruct the original, which is what makes the
// remainder safe to drop only deliberately.
func TestDiv_QuotientAndRemainderReconstruct(t *testing.T) {
	original := money.New(1_234_567, "VND")

	q, r := original.Div(97)

	if got := q.Mul(97).Add(r); got.Amount() != original.Amount() {
		t.Errorf("q×97 + r = %d, want %d", got.Amount(), original.Amount())
	}
}

func TestDiv_PanicsOnZeroDivisor(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("dividing by zero must panic")
		}
	}()

	money.New(100, "VND").Div(0)
}

func TestIsZeroAndIsNegative(t *testing.T) {
	if !money.New(0, "VND").IsZero() {
		t.Error("0 VND is zero")
	}
	if money.New(1, "VND").IsZero() {
		t.Error("1 VND is not zero")
	}
	if !money.New(-1, "VND").IsNegative() {
		t.Error("-1 VND is negative")
	}
	if money.New(0, "VND").IsNegative() {
		t.Error("0 VND is not negative")
	}
}

// String reports the minor-unit amount, not a display format. It names the
// currency so a logged amount cannot be mistaken for one of another.
func TestString_NamesTheMinorUnitAmount(t *testing.T) {
	cases := map[string]string{
		money.New(1234, "VND").String(): "1234 VND",
		money.New(-1, "USD").String():   "-1 USD",
		money.New(0, "VND").String():    "0 VND",
	}

	for got, want := range cases {
		if got != want {
			t.Errorf("String() = %q, want %q", got, want)
		}
	}

	// The zero value has no currency to name.
	var zero money.Money
	if got := zero.String(); got != "0" {
		t.Errorf("zero String() = %q, want %q", got, "0")
	}
}

// The zero value is not merely readable: it can be added to and subtracted from,
// and it goes on naming no currency until a real amount gives it one.
func TestZeroValue_IsAnAccumulatorStart(t *testing.T) {
	var total money.Money

	if got := total.Sub(money.New(10, "VND")); got.Amount() != -10 || got.Currency() != "VND" {
		t.Errorf("zero − 10 VND = %d %s, want -10 VND", got.Amount(), got.Currency())
	}
	if got := total.Mul(1_000); !got.IsZero() || got.Currency() != "" {
		t.Errorf("zero × 1000 = %s, want the zero value", got)
	}

	q, r := total.Div(7)
	if !q.IsZero() || !r.IsZero() {
		t.Errorf("zero ÷ 7 = (%s, %s), want zero", q, r)
	}
}

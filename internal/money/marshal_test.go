package money_test

import (
	"encoding/json"
	"testing"

	"fingo/internal/money"
)

// JSON carries the amount as an integer and the currency as a code. No float
// anywhere: 0.1 + 0.2 is not 0.3, and a ledger that says it is cannot balance.
func TestMarshalJSON_KeepsAmountAndCurrency(t *testing.T) {
	encoded, err := json.Marshal(money.New(1234, "VND"))
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	if got, want := string(encoded), `{"amount":1234,"currency":"VND"}`; got != want {
		t.Errorf("JSON = %s, want %s", got, want)
	}
}

func TestUnmarshalJSON_RoundTrips(t *testing.T) {
	original := money.New(-987_654_321, "USD")

	encoded, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var decoded money.Money
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if decoded.Amount() != original.Amount() || decoded.Currency() != original.Currency() {
		t.Errorf("round trip = %d %s, want %d %s",
			decoded.Amount(), decoded.Currency(), original.Amount(), original.Currency())
	}
}

// The exact integer survives the round trip, including values past the range a
// float could hold without loss.
func TestUnmarshalJSON_KeepsLargeAmountsExact(t *testing.T) {
	const amount = int64(9_007_199_254_740_993) // 2^53 + 1, not representable as float64

	var decoded money.Money
	if err := json.Unmarshal([]byte(`{"amount":9007199254740993,"currency":"VND"}`), &decoded); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if decoded.Amount() != amount {
		t.Errorf("amount = %d, want %d", decoded.Amount(), amount)
	}
}

func TestUnmarshalJSON_RefusesAnInvalidCurrency(t *testing.T) {
	for _, body := range []string{
		`{"amount":1,"currency":""}`,
		`{"amount":1,"currency":"VN"}`,
		`{"amount":1,"currency":"vnd"}`,
		`{"amount":1,"currency":"VNDD"}`,
	} {
		var decoded money.Money
		if err := json.Unmarshal([]byte(body), &decoded); err == nil {
			t.Errorf("Unmarshal(%s) must fail on the currency, got nil", body)
		}
	}
}

// A malformed body is refused rather than half-read.
func TestUnmarshalJSON_RefusesMalformedInput(t *testing.T) {
	for _, body := range []string{
		`not json`,
		`{"amount":"100","currency":"VND"}`,
		`{"amount":1.5,"currency":"VND"}`,
		`[1,2]`,
	} {
		var decoded money.Money
		if err := json.Unmarshal([]byte(body), &decoded); err == nil {
			t.Errorf("Unmarshal(%s) must fail, got nil", body)
		}
	}
}

// A missing field is zero, which is ordinary JSON. Stated here because it is a
// choice: a producer that forgets the amount sends 0 VND rather than failing, and
// that must be a known behaviour rather than a surprise.
func TestUnmarshalJSON_MissingFieldsAreZero(t *testing.T) {
	cases := map[string]struct {
		amount   int64
		currency money.Currency
	}{
		`{"currency":"VND"}`:            {0, "VND"},
		`{"amount":0,"currency":"VND"}`: {0, "VND"},
		`{}`:                            {0, ""},
	}

	for body, want := range cases {
		var decoded money.Money
		if err := json.Unmarshal([]byte(body), &decoded); err != nil {
			t.Errorf("Unmarshal(%s): %v", body, err)
			continue
		}
		if decoded.Amount() != want.amount || decoded.Currency() != want.currency {
			t.Errorf("Unmarshal(%s) = %d %s, want %d %s",
				body, decoded.Amount(), decoded.Currency(), want.amount, want.currency)
		}
	}
}

// The zero value survives a round trip. It has no currency, and it must not be
// refused on the way back in — an accumulator's starting point is a real value.
func TestMarshalJSON_ZeroValueRoundTrips(t *testing.T) {
	var original money.Money

	encoded, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var decoded money.Money
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatalf("Unmarshal(%s): %v", encoded, err)
	}

	if !decoded.IsZero() || decoded.Currency() != "" {
		t.Errorf("round trip = %s, want the zero value", decoded)
	}
}

// The database holds a Money in two columns: an int64 amount and a varchar(3)
// code. These are the two halves of that, and they round trip.
func TestFromDB_RoundTripsThroughTheTwoColumns(t *testing.T) {
	original := money.New(500_000, "VND")

	amount, currency := original.Amount(), string(original.Currency())

	decoded, err := money.FromDB(amount, currency)
	if err != nil {
		t.Fatalf("FromDB: %v", err)
	}

	if decoded.Amount() != original.Amount() || decoded.Currency() != original.Currency() {
		t.Errorf("FromDB = %d %s, want %d %s",
			decoded.Amount(), decoded.Currency(), original.Amount(), original.Currency())
	}
}

func TestFromDB_RefusesAnInvalidCurrency(t *testing.T) {
	if _, err := money.FromDB(100, "VN"); err == nil {
		t.Error("FromDB must refuse a two-letter currency, got nil")
	}
	if _, err := money.FromDB(100, ""); err == nil {
		t.Error("FromDB must refuse an empty currency, got nil")
	}
}

// A row written before the currency was known reads back as the zero value
// rather than as a wrong amount.
func TestFromDB_RefusesRatherThanGuessing(t *testing.T) {
	decoded, err := money.FromDB(0, "VND")
	if err != nil {
		t.Fatalf("FromDB: %v", err)
	}
	if !decoded.IsZero() {
		t.Errorf("0 VND = %s, want zero", decoded)
	}
}

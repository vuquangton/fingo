package money

import (
	"encoding/json"
	"fmt"
)

// wireMoney is the JSON shape: an integer amount and a currency code, both
// explicit.
//
// Never a bare number, which would carry no currency. Never a float, which
// cannot hold a minor unit exactly. Never a formatted string, which would need
// this package to know how many minor units make a major one.
type wireMoney struct {
	Amount   int64  `json:"amount"`
	Currency string `json:"currency"`
}

func (m Money) MarshalJSON() ([]byte, error) {
	return json.Marshal(wireMoney{Amount: m.amount, Currency: string(m.currency)})
}

// UnmarshalJSON reads a Money back, refusing a currency it cannot name.
//
// An amount with no currency is refused rather than guessed at: a balance in an
// unknown currency cannot be added to anything.
func (m *Money) UnmarshalJSON(data []byte) error {
	var wire wireMoney
	if err := json.Unmarshal(data, &wire); err != nil {
		return fmt.Errorf("money: %w", err)
	}

	decoded, err := FromDB(wire.Amount, wire.Currency)
	if err != nil {
		return err
	}

	*m = decoded

	return nil
}

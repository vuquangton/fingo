package money

import (
	"encoding/json"
	"fmt"
	"strconv"
)

// Dong is an amount of Vietnamese đồng: a whole number of them.
//
// VND has no subunit. The type says so: there is no fractional value to
// represent, no scale to agree on, and no way to pass a decimal where an amount
// is expected. It replaces shopspring/decimal in the ledger (ADR 0008).
//
// The zero value is zero đồng and is usable, so a struct holding an amount needs
// no constructor to be safe.
type Dong int64

// Zero is no đồng.
const Zero Dong = 0

// FromInt converts a whole number of đồng.
func FromInt(v int64) Dong { return Dong(v) }

// Int64 returns the amount as an integer, for storage and transport.
func (d Dong) Int64() int64 { return int64(d) }

// Add returns d + other.
func (d Dong) Add(other Dong) Dong { return d + other }

// Sub returns d − other.
func (d Dong) Sub(other Dong) Dong { return d - other }

// Neg returns −d.
func (d Dong) Neg() Dong { return -d }

// Abs returns |d|.
func (d Dong) Abs() Dong {
	if d < 0 {
		return -d
	}
	return d
}

// Mul returns d × factor.
//
// Both sides are whole đồng, so the product is whole đồng. The callers that use
// this multiply by a count (months) or a percentage and divide afterwards; a
// fractional factor is not expressible, which is the point.
func (d Dong) Mul(factor Dong) Dong { return d * factor }

// Div returns d ÷ divisor, truncated toward zero.
//
// Truncation matches what the engine did with decimal values followed by
// Truncate(0), so converting to integers does not move a figure. Panics on a
// zero divisor, as the decimal version did.
func (d Dong) Div(divisor Dong) Dong {
	if divisor == 0 {
		panic("money: division by zero")
	}
	return d / divisor
}

// Cmp compares d and other: -1, 0 or +1.
func (d Dong) Cmp(other Dong) int {
	switch {
	case d < other:
		return -1
	case d > other:
		return 1
	default:
		return 0
	}
}

// Equal reports whether the amounts are the same.
func (d Dong) Equal(other Dong) bool { return d == other }

// IsZero reports whether the amount is zero.
func (d Dong) IsZero() bool { return d == 0 }

// IsPositive reports whether the amount is greater than zero.
func (d Dong) IsPositive() bool { return d > 0 }

// IsNegative reports whether the amount is less than zero.
func (d Dong) IsNegative() bool { return d < 0 }

// Sign returns -1, 0 or +1.
func (d Dong) Sign() int {
	switch {
	case d < 0:
		return -1
	case d > 0:
		return 1
	default:
		return 0
	}
}

// GreaterThan reports d > other.
func (d Dong) GreaterThan(other Dong) bool { return d > other }

// LessThan reports d < other.
func (d Dong) LessThan(other Dong) bool { return d < other }

// GreaterThanOrEqual reports d ≥ other.
func (d Dong) GreaterThanOrEqual(other Dong) bool { return d >= other }

// LessThanOrEqual reports d ≤ other.
func (d Dong) LessThanOrEqual(other Dong) bool { return d <= other }

// IntPart returns the whole part, which for đồng is the amount itself.
func (d Dong) IntPart() int64 { return int64(d) }

// Round returns the amount unchanged: an integer is already at every precision
// it can have. The parameter is kept so callers that meant "round to đồng" still
// say so, and so the intent survives the removal of decimals.
func (d Dong) Round(int32) Dong { return d }

// Truncate returns the amount unchanged, for the same reason as Round.
func (d Dong) Truncate(int32) Dong { return d }

// String renders the amount with no separators, e.g. "-1234".
func (d Dong) String() string { return strconv.FormatInt(int64(d), 10) }

// MarshalJSON writes the amount as a string.
//
// JSON numbers are doubles in every browser, and 2^53 is the exact-integer
// limit: an amount above it would be silently rounded by the client even though
// Go and PostgreSQL held it exactly. A string closes that hole; the cost is that
// the client formats and never computes.
func (d Dong) MarshalJSON() ([]byte, error) {
	return []byte(`"` + strconv.FormatInt(int64(d), 10) + `"`), nil
}

// UnmarshalJSON accepts a string or a number, so a hand-written request and a
// recorded fixture both work.
func (d *Dong) UnmarshalJSON(data []byte) error {
	var asString string
	if err := json.Unmarshal(data, &asString); err == nil {
		v, err := strconv.ParseInt(asString, 10, 64)
		if err != nil {
			return fmt.Errorf("money: %q is not a whole number of đồng: %w", asString, err)
		}
		*d = Dong(v)
		return nil
	}

	var asNumber int64
	if err := json.Unmarshal(data, &asNumber); err != nil {
		return fmt.Errorf("money: amount must be an integer or a string: %w", err)
	}
	*d = Dong(asNumber)
	return nil
}

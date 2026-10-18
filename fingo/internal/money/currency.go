package money

// Currency is a three-letter ISO 4217 code, such as "VND" or "USD".
type Currency string

// Valid reports whether the code has the shape ISO 4217 gives: exactly three
// letters, upper case.
//
// Only the shape is checked. Whether a code is actually assigned is not
// something this package can know without carrying a currency table, and a table
// that goes stale is worse than a shape check that cannot.
func (c Currency) Valid() bool {
	if len(c) != 3 {
		return false
	}

	for i := 0; i < len(c); i++ {
		if c[i] < 'A' || c[i] > 'Z' {
			return false
		}
	}

	return true
}

// Package format writes and reads amounts the way a Vietnamese accountant does.
//
// The comma and the dot are the other way round from English: "1.234.567,89" is
// one million two hundred and thirty-four thousand five hundred and sixty-seven
// and eighty-nine hundredths. This matters more than it looks. A number read
// under the wrong convention is wrong by a factor of a thousand, and it is wrong
// *plausibly* — it looks like a number, it is in the right column, and nobody
// notices until a return is filed.
//
// So the two functions here are mirrors of each other, and the parser refuses
// anything it cannot read under exactly one interpretation rather than guessing.
package format

import (
	"errors"
	"fmt"
	"strings"

	"github.com/shopspring/decimal"
)

// groupSeparator and decimalSeparator are the Vietnamese convention.
const (
	groupSeparator   = '.'
	decimalSeparator = ','
)

// Money writes an amount the way a Vietnamese accountant reads it: groups of
// three separated by dots, and a comma before any fractional part.
//
// Trailing zeroes in the fraction are dropped. "1.234,50" claims a precision the
// books do not carry, and in a column of figures the extra digit is noise.
func Money(d decimal.Decimal) string {
	text := d.String()

	sign := ""

	switch {
	case strings.HasPrefix(text, "-"):
		sign, text = "-", text[1:]
	case strings.HasPrefix(text, "+"):
		text = text[1:]
	}

	whole, fraction, _ := strings.Cut(text, ".")

	grouped := group(whole)

	fraction = strings.TrimRight(fraction, "0")
	if fraction == "" {
		return sign + grouped
	}

	return sign + grouped + string(decimalSeparator) + fraction
}

// MoneyIfAny is Money, and an empty string for zero.
//
// A trial balance with "0" in every unused column is a table nobody can read
// across; an empty cell is a fact — this account was not touched — and a zero
// is a measurement. The caller decides which it wants to say.
func MoneyIfAny(d decimal.Decimal) string {
	if d.IsZero() {
		return ""
	}

	return Money(d)
}

// Amount reads an amount an accountant typed.
//
// A dot groups digits in threes and a comma introduces the fraction, both
// optional — nobody types the separators into a small number.
//
// Anything that cannot be read under exactly that rule is refused rather than
// guessed at. "1234.56" is the case worth knowing about: it groups as 1 · 234 ·
// 56, so it is not a Vietnamese number, and it could equally have been meant as
// 1234,56 typed in the English style. The two readings differ by a hundred, so
// there is no safe answer except to say so.
func Amount(s string) (decimal.Decimal, error) {
	text := strings.TrimSpace(s)
	if text == "" {
		return decimal.Zero, errors.New("there is no amount here")
	}

	sign := ""

	switch text[0] {
	case '-':
		sign, text = "-", text[1:]
	case '+':
		text = text[1:]
	}

	whole, fraction, fractional := strings.Cut(text, string(decimalSeparator))
	if fractional {
		if strings.Contains(fraction, string(decimalSeparator)) {
			return decimal.Zero, fmt.Errorf("%q has more than one %q", s, decimalSeparator)
		}
		if fraction == "" {
			return decimal.Zero, fmt.Errorf("%q ends with a %q and nothing after it", s, decimalSeparator)
		}
	}

	if err := checkGrouping(s, whole); err != nil {
		return decimal.Zero, err
	}

	digits := strings.ReplaceAll(whole, string(groupSeparator), "")

	if fraction != "" {
		if !allDigits(fraction) {
			return decimal.Zero, fmt.Errorf("%q has something in its fraction that is not a digit", s)
		}

		digits += "." + fraction
	}

	amount, err := decimal.NewFromString(sign + digits)
	if err != nil {
		return decimal.Zero, fmt.Errorf("%q is not an amount: %w", s, err)
	}

	return amount, nil
}

// checkGrouping enforces the rule that, WHEN dots are present, they separate
// groups of exactly three.
//
// A dot is only a separator if it is being used as one. "1234" is one thousand
// two hundred and thirty-four written without its separator, which is how most
// people write it, and refusing it would make the grid refuse the commonest
// input there is. But once a dot appears it has to mean something: the first
// group may be shorter — "1.234" — and every group after it is exactly three
// digits. A shorter one is the ambiguous case, and it is refused.
func checkGrouping(original, whole string) error {
	if whole == "" {
		return fmt.Errorf("%q has no digits before its separator", original)
	}

	groups := strings.Split(whole, string(groupSeparator))

	for _, g := range groups {
		if !allDigits(g) {
			return fmt.Errorf("%q has something in it that is not a digit", original)
		}
	}

	if len(groups) == 1 {
		return nil
	}

	if len(groups[0]) > 3 {
		return fmt.Errorf("%q does not group in threes", original)
	}

	for i, g := range groups[1:] {
		if len(g) != 3 {
			return fmt.Errorf(
				"%q groups in threes, and group %d has %d digit(s): write %c%03d instead",
				original, i+1, len(g), groupSeparator, mustInt(g))
		}
	}

	return nil
}

func allDigits(s string) bool {
	if s == "" {
		return false
	}

	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}

	return true
}

// mustInt reads a run of digits, for a message. It is a string of digits by the
// time it is called, and the error path is not worth a second failure.
func mustInt(s string) int {
	n := 0
	for _, r := range s {
		n = n*10 + int(r-'0')
	}

	return n
}

// group inserts a dot before every third digit from the right.
//
// The input is digits by the time it arrives, but it is checked anyway: this is
// called from Money, which is called from a report, and a report that panics
// because an account code reached it is a worse outcome than one that prints
// something odd.
func group(digits string) string {
	if len(digits) <= 3 || !allDigits(digits) {
		return digits
	}

	var parts []string

	for len(digits) > 3 {
		parts = append([]string{digits[len(digits)-3:]}, parts...)
		digits = digits[:len(digits)-3]
	}

	parts = append([]string{digits}, parts...)

	return strings.Join(parts, string(groupSeparator))
}

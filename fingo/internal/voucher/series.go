// Package voucher holds the documents a book numbers: a prefix, a year, and a
// set of balanced account movements.
package voucher

import (
	"errors"
	"fmt"
)

// maxPrefixLen bounds a prefix so the rendered number stays readable.
const maxPrefixLen = 16

// Prefix is the short code a series numbers its vouchers with: "JV", "INV", "PO".
type Prefix string

// Valid reports whether a prefix can appear in a voucher number.
//
// Upper-case letters and digits only. A prefix containing a hyphen would make
// "JV-1-2026-00001" ambiguous against the separator, and one in lower case would
// let two series that read the same be different.
func (p Prefix) Valid() bool {
	if len(p) == 0 || len(p) > maxPrefixLen {
		return false
	}

	for i := 0; i < len(p); i++ {
		c := p[i]
		if (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') {
			continue
		}

		return false
	}

	return true
}

// Format renders a number as it is written on the document: PREFIX-YYYY-NNNNN.
//
// The widths are minimums. A run that passes 99,999 keeps counting rather than
// wrapping back to five digits, because a wrapped number would collide with one
// already issued — and two documents sharing a number is worse than an untidy one.
func Format(prefix Prefix, year, number int) string {
	return fmt.Sprintf("%s-%04d-%05d", prefix, year, number)
}

// Series is one run of numbers: a prefix, a fiscal year, and how far it has got.
//
// A series is per prefix AND per year, so the counter restarts every January —
// JV-2026-00042 and JV-2027-00042 are two different documents.
type Series struct {
	ID         string
	Prefix     Prefix
	Year       int
	LastNumber int
}

// Next is the number that follows the ones already issued.
func (s Series) Next() int { return s.LastNumber + 1 }

// Validate reports whether the series describes a run that could be numbered.
func (s Series) Validate() error {
	if s.ID == "" {
		return errors.New("a voucher series needs an id")
	}
	if !s.Prefix.Valid() {
		return fmt.Errorf("voucher series %s has prefix %q, which is not a usable code", s.ID, s.Prefix)
	}
	if s.Year < 1 {
		return fmt.Errorf("voucher series %s has year %d", s.ID, s.Year)
	}
	if s.LastNumber < 0 {
		return fmt.Errorf("voucher series %s has issued %d numbers, which cannot be", s.ID, s.LastNumber)
	}

	return nil
}

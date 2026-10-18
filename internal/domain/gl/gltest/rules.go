// Package gltest provides helpers shared by tests that post vouchers.
//
// It exists because posting requires rules at every call site, and three copies
// of the same chart-building helper drifted apart the moment a field was added
// to PostingRules. One source, one place to change.
package gltest

import (
	"testing"

	"fingo/internal/domain/gl"
)

// TT133Rules returns posting rules over the full TT133 chart, with no period
// closed — the state a client is in before anything has been shut.
func TT133Rules(t *testing.T) gl.PostingRules {
	t.Helper()

	chart := gl.NewChartOfAccounts()
	for _, account := range gl.TT133Chart() {
		if err := chart.Add(account); err != nil {
			t.Fatalf("seed chart with %s: %v", account.Code, err)
		}
	}

	return gl.PostingRules{Chart: chart, Periods: gl.NewPeriods()}
}

// NumberFor returns a number in the right book and fiscal year for a voucher,
// so tests need not restate the book mapping.
func NumberFor(t *testing.T, v *gl.Voucher, seq int) gl.Number {
	t.Helper()

	book, err := gl.BookFor(v.VoucherType())
	if err != nil {
		t.Fatalf("voucher type %s has no number book: %v", v.VoucherType(), err)
	}

	return gl.Number{Book: book, Year: v.VoucherDate().UTC().Year(), Seq: seq}
}

package voucher

import "context"

// SeriesRepository allocates numbers from a client's voucher series.
type SeriesRepository interface {
	// Next allocates the next number in a series, creating the series on first
	// use.
	//
	// It returns the bare number, not the formatted one: Format renders it, and
	// keeping the two apart means a number can be compared and sorted as a
	// number.
	//
	// Concurrent callers never receive the same number and never skip one. The
	// allocation holds the series row for the rest of its transaction, so the
	// read of the counter and the write that advances it cannot be interleaved.
	Next(ctx context.Context, prefix Prefix, year int) (int, error)
}

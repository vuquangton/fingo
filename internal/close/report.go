package close

import (
	"context"
	"fmt"
	"sync"
	"time"

	"fingo/internal/document"
	"fingo/internal/money"
)

// Report is the snapshot taken when a period was closed: what the period earned,
// what it spent, what it left, and what closing it did.
//
// It is a snapshot and not a report that can be regenerated, which is the point
// of writing it down. The P&L accounts are at zero afterwards, so the figures
// this holds cannot be recomputed from the books — and it is exactly the figures
// a later question about the period is answered from.
type Report struct {
	PeriodID  string
	StartDate time.Time
	EndDate   time.Time

	// Revenues and Expenses are the accounts as they stood when the period was
	// closed, in each account's own sense.
	Revenues []AccountBalance
	Expenses []AccountBalance

	// NetProfit is what was carried to retained earnings: revenue less expense,
	// positive for a profit and negative for a loss.
	NetProfit money.Dong

	// Provisions is what the close provided for at the period's end — the
	// depreciation it charged and the bad debt it provided against. The entries
	// on the closing voucher say the amounts; this says what they were for.
	// Empty for a close that provides for nothing.
	Provisions ProvisionSummary

	ClosedAt time.Time
	ClosedBy string

	// VoucherID and VoucherNo identify the closing entries. VoucherNo is empty
	// when there was nothing to close — a period with no revenue and no expense
	// still closes, and there is no voucher for it.
	VoucherID string
	VoucherNo string

	// Forced and ForceReason record a close that was made past documents still
	// unsettled in the period, and why.
	Forced      bool
	ForceReason string

	// OpenDocuments is what was still owed when the close was forced.
	OpenDocuments []document.Document
}

// MemoryReports is a close report book held in memory.
//
// It applies the same rules as any other implementation — one report per period,
// replaced on a retry — and forgets everything when the process exits. It is a
// fixture rather than a store.
//
// The zero value is an empty book.
type MemoryReports struct {
	mu sync.Mutex

	byPeriod map[string]Report
}

// NewMemoryReports returns an empty in-memory report book.
func NewMemoryReports() *MemoryReports { return &MemoryReports{} }

// Save records a period's report, replacing any earlier one.
func (m *MemoryReports) Save(_ context.Context, r Report) error {
	if r.PeriodID == "" {
		return fmt.Errorf("%w: a close report names no period", ErrInvalidCommand)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if m.byPeriod == nil {
		m.byPeriod = make(map[string]Report)
	}
	m.byPeriod[r.PeriodID] = copyReport(r)

	return nil
}

// Get returns the report recorded for a period, or ErrNoReport.
func (m *MemoryReports) Get(_ context.Context, periodID string) (Report, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	r, ok := m.byPeriod[periodID]
	if !ok {
		return Report{}, fmt.Errorf("%w: %s", ErrNoReport, periodID)
	}

	return copyReport(r), nil
}

// copyReport returns a report whose slices the caller cannot reach back into.
//
// A stored snapshot that a caller could edit by appending to a slice it was
// handed is not a snapshot.
func copyReport(r Report) Report {
	r.Revenues = append([]AccountBalance(nil), r.Revenues...)
	r.Expenses = append([]AccountBalance(nil), r.Expenses...)
	r.OpenDocuments = append([]document.Document(nil), r.OpenDocuments...)
	r.Provisions.Depreciation = append([]DepreciationCharge(nil), r.Provisions.Depreciation...)
	r.Provisions.BadDebt = append([]BadDebtCharge(nil), r.Provisions.BadDebt...)

	return r
}

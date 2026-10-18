package vat

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

// Registry is the VAT rate table: the rows, and the rules that keep them
// consistent with one another.
//
// The zero value is an empty table, ready to use. A Registry is not safe for
// concurrent use; it is configured once, at start-up, from the rows the
// database holds.
type Registry struct {
	// rows is kept ordered by effective date, oldest first, so the row in force
	// on a day is the last active row that started on or before it.
	rows []VATRate

	// index maps a row's ID to its position in rows.
	index map[string]int
}

// New returns an empty rate table.
func New() *Registry { return &Registry{} }

// Create appends a row to the table.
//
// This is the only write the table has. There is no Update and no Delete,
// because there is no correct way to change what a document was already taxed
// at: a rate change is a new row with a new EffectiveFrom, and the new row takes
// over from the old one on its own start date without the old row being
// rewritten. A tax figure recomputed from a historical row next year is
// therefore the figure it was computed from last year.
//
// An active row may only be appended AFTER every active row already in the
// table, and only once their declared windows have closed. A row that would
// cover a day an active row already covers is refused — the table must answer
// "which rate applies on this date" with one row, never a tie-break — and that
// refuses both a second rate starting on a taken date and a rate inserted into
// the middle of a history that has already been reported.
func (g *Registry) Create(rate VATRate) error {
	if err := rate.Validate(); err != nil {
		return err
	}
	if _, exists := g.index[rate.ID]; exists {
		return fmt.Errorf("%w: %s", ErrDuplicateRateID, rate.ID)
	}
	if rate.Active {
		if err := g.admits(rate); err != nil {
			return err
		}
	}

	g.insert(rate.clone())

	return nil
}

// admits reports whether an active row may be appended, given the active rows
// already held.
func (g *Registry) admits(candidate VATRate) error {
	from := truncateToDay(candidate.EffectiveFrom)

	for _, held := range g.rows {
		if !held.Active {
			continue
		}

		heldFrom := truncateToDay(held.EffectiveFrom)

		// A row that starts on or before an existing row's start would sit
		// alongside it rather than after it: the two would claim the same days,
		// and the table could no longer say which one a report of those days
		// was made under.
		if !from.After(heldFrom) {
			return fmt.Errorf("%w: %s starts %s, at or before %s which starts %s",
				ErrOverlappingRate, candidate.ID, day(from), held.ID, day(heldFrom))
		}

		// A row that starts inside a window another row declared the end of
		// overlaps it: the earlier row says it is still in force, the new one
		// says it has taken over.
		if held.EffectiveTo != nil && !from.After(truncateToDay(*held.EffectiveTo)) {
			return fmt.Errorf("%w: %s starts %s, inside the window %s closes on %s",
				ErrOverlappingRate, candidate.ID, day(from), held.ID, day(*held.EffectiveTo))
		}
	}

	return nil
}

// insert stores a row and restores the ordering the lookups rely on.
func (g *Registry) insert(rate VATRate) {
	if g.index == nil {
		g.index = make(map[string]int, 1)
	}

	g.rows = append(g.rows, rate)

	sort.SliceStable(g.rows, func(i, j int) bool {
		return truncateToDay(g.rows[i].EffectiveFrom).Before(truncateToDay(g.rows[j].EffectiveFrom))
	})

	for i, r := range g.rows {
		g.index[r.ID] = i
	}
}

// Get returns the row with this ID. The row is a copy, own pointers and all: a
// caller cannot edit the table through what it is handed.
func (g *Registry) Get(id string) (VATRate, error) {
	i, ok := g.index[id]
	if !ok {
		return VATRate{}, fmt.Errorf("%w: %s", ErrUnknownRateID, id)
	}

	return g.rows[i].clone(), nil
}

// List returns every row, oldest first, inactive rows included. The rows are
// copies, so an operator can be shown the table without being able to change it.
func (g *Registry) List() []VATRate {
	out := make([]VATRate, len(g.rows))
	for i, r := range g.rows {
		out[i] = r.clone()
	}

	return out
}

// RateOn returns the active row that governs a date.
//
// The row in force on a day is the latest active row that started on or before
// it — that is what lets a new row take over from an old one without either
// being edited. A row that declared an end stops governing after it, so a date
// falling in a gap between windows is an error rather than a silent fallback to
// an older rate, which would tax a document at a rate nobody chose.
func (g *Registry) RateOn(date time.Time) (VATRate, error) {
	d := truncateToDay(date)

	for i := len(g.rows) - 1; i >= 0; i-- {
		r := g.rows[i]
		if !r.Active || truncateToDay(r.EffectiveFrom).After(d) {
			continue
		}

		if r.EffectiveTo != nil && truncateToDay(*r.EffectiveTo).Before(d) {
			return VATRate{}, fmt.Errorf("%w: %s falls after %s ended on %s",
				ErrNoRateInForce, day(d), r.ID, day(*r.EffectiveTo))
		}

		return r.clone(), nil
	}

	return VATRate{}, fmt.Errorf("%w: no active rate starts on or before %s", ErrNoRateInForce, day(d))
}

// Charge resolves a transaction line to the rate row it names and the tax that
// row charges on it.
//
// The row is read by ID rather than re-derived from the document's date: the
// tax on a line is a fact about the rate applied when the line was written, and
// it must not move because a later rate was added or an older one lapsed.
func (g *Registry) Charge(line Line) (Charge, error) {
	if strings.TrimSpace(line.VATRateID) == "" {
		return Charge{}, fmt.Errorf("%w: net amount %d", ErrUnnamedRate, line.NetAmount)
	}

	rate, err := g.Get(line.VATRateID)
	if err != nil {
		return Charge{}, err
	}

	return Charge{Line: line, Rate: rate, Tax: rate.Tax(line.NetAmount)}, nil
}

// clone returns a row whose EffectiveTo is its own. Two rows sharing one
// *time.Time would let a caller who was handed a copy edit the stored row by
// writing through the pointer.
func (r VATRate) clone() VATRate {
	if r.EffectiveTo != nil {
		to := *r.EffectiveTo
		r.EffectiveTo = &to
	}

	return r
}

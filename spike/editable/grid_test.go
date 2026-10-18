package main

import (
	"strings"
	"testing"
)

// fakeChart is a chart that knows a fixed set of codes, so the grid's rules can
// be tested without dragging the real chart of accounts in.
type fakeChart map[string]bool

func (c fakeChart) Has(code string) bool { return c[code] }

func chart() fakeChart {
	return fakeChart{"1111": true, "1121": true, "131": true, "5111": true, "632": true, "156": true}
}

// TestEntryGrid_SetCell_AmountIsWholeDong — slice 1: the amount cell, and the
// one rule VND imposes on it.
//
// VND has no subunit, so a fractional entry cannot be rounded into agreement —
// it has to be refused. Note the last two cases: the shipped admin UI's
// formAmount strips '.' AND ',' before parsing, so "1.234,56" is silently read
// as 123456. That is a real defect in a real path, and the grid is where it gets
// caught.
func TestEntryGrid_SetCell_AmountIsWholeDong(t *testing.T) {
	tests := []struct {
		name    string
		typed   string
		want    int64
		wantErr bool
	}{
		{name: "plain digits", typed: "1000000", want: 1_000_000},
		{name: "dot grouped, as a reader writes it", typed: "1.000.000", want: 1_000_000},
		{name: "spaces around it", typed: "  250000  ", want: 250_000},
		{name: "empty leaves the row untouched", typed: "", want: 0},
		{name: "letters are not money", typed: "abc", wantErr: true},
		{name: "negative is not a movement you can post", typed: "-500", wantErr: true},
		{name: "a fractional dong is refused, not rounded", typed: "1.234,56", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewEntryGrid(chart(), 1)

			err := g.SetCell(0, colAmount, tt.typed)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("SetCell(amount=%q) = nil, want a refusal", tt.typed)
				}
				return
			}
			if err != nil {
				t.Fatalf("SetCell(amount=%q): %v", tt.typed, err)
			}
			if got := g.Cells(0).Amount; got != tt.want {
				t.Errorf("amount = %d, want %d", got, tt.want)
			}
		})
	}
}

// TestEntryGrid_SetCell_AccountMustExist — slice 2a: a row cannot name an
// account the client's chart does not hold.
//
// An empty account is not a mistake: the grid renders more rows than most
// vouchers need, and a row nobody has touched is simply not part of the entry.
func TestEntryGrid_SetCell_AccountMustExist(t *testing.T) {
	tests := []struct {
		name    string
		typed   string
		want    string
		wantErr bool
	}{
		{name: "a code in the chart", typed: "1111", want: "1111"},
		{name: "surrounding spaces are the operator's, not the code's", typed: "  5111 ", want: "5111"},
		{name: "an untouched row is not an error", typed: "", want: ""},
		{name: "a code the chart does not hold", typed: "9999", wantErr: true},
		{name: "a code that is almost right", typed: "111", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewEntryGrid(chart(), 1)

			err := g.SetCell(0, colAccount, tt.typed)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("SetCell(account=%q) = nil, want a refusal", tt.typed)
				}
				return
			}
			if err != nil {
				t.Fatalf("SetCell(account=%q): %v", tt.typed, err)
			}
			if got := g.Cells(0).Account; got != tt.want {
				t.Errorf("account = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestEntryGrid_SetCell_SideIsDebitOrCredit — slice 2b: a movement has a side,
// and the only two sides are the two the ledger has.
func TestEntryGrid_SetCell_SideIsDebitOrCredit(t *testing.T) {
	tests := []struct {
		name    string
		typed   string
		want    string
		wantErr bool
	}{
		{name: "debit", typed: "debit", want: "debit"},
		{name: "credit", typed: "credit", want: "credit"},
		{name: "as the ledger spells it", typed: "DEBIT", want: "debit"},
		{name: "with spaces", typed: " Credit ", want: "credit"},
		{name: "an untouched row has no side yet", typed: "", want: ""},
		{name: "anything else is not a movement", typed: "both", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewEntryGrid(chart(), 1)

			err := g.SetCell(0, colSide, tt.typed)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("SetCell(side=%q) = nil, want a refusal", tt.typed)
				}
				return
			}
			if err != nil {
				t.Fatalf("SetCell(side=%q): %v", tt.typed, err)
			}
			if got := g.Cells(0).Side; got != tt.want {
				t.Errorf("side = %q, want %q", got, tt.want)
			}
		})
	}
}

// fill types a whole row, cell by cell, the way an operator tabbing across it
// would.
func fill(t *testing.T, g *EntryGrid, row int, account, side, amount string) {
	t.Helper()

	for col, text := range []string{account, side, amount, "Bút toán kiểm tra"} {
		if err := g.SetCell(row, col, text); err != nil {
			t.Fatalf("fill row %d, column %d with %q: %v", row, col, text, err)
		}
	}
}

// TestEntryGrid_Validate_RefusesAnUnbalancedEntry — slice 3, and the point of
// the whole spike.
//
// A voucher whose two sides do not agree is the one thing the ledger must never
// accept, and the grid is the last place it can be caught while somebody is
// still looking at the numbers. The refusal has to say by how much.
func TestEntryGrid_Validate_RefusesAnUnbalancedEntry(t *testing.T) {
	tests := []struct {
		name         string
		debitAmount  string
		creditAmount string
		wantErr      bool
		wantBoth     []string
	}{
		{name: "they agree", debitAmount: "1.000.000", creditAmount: "1.000.000"},
		{name: "the credit is short", debitAmount: "1.000.000", creditAmount: "900.000",
			wantErr: true, wantBoth: []string{"1.000.000", "900.000"}},
		{name: "the debit is short", debitAmount: "40.000", creditAmount: "50.000",
			wantErr: true, wantBoth: []string{"40.000", "50.000"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewEntryGrid(chart(), 2)

			fill(t, g, 0, "1111", "debit", tt.debitAmount)
			fill(t, g, 1, "5111", "credit", tt.creditAmount)

			err := g.Validate()

			if !tt.wantErr {
				if err != nil {
					t.Fatalf("Validate() = %v, want the entry accepted", err)
				}
				return
			}

			if err == nil {
				t.Fatal("Validate() = nil; an entry whose sides disagree must be refused")
			}

			// The refusal has to be checkable by the operator, so it names both
			// sides rather than saying the entry is wrong.
			for _, want := range tt.wantBoth {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("refusal %q does not name %s", err, want)
				}
			}
		})
	}
}

// TestEntryGrid_Validate_SkipsUntouchedRows — slice 3b: the grid renders more
// rows than most vouchers need, so a row nobody filled in is not part of the
// entry and must not be reported as one.
func TestEntryGrid_Validate_SkipsUntouchedRows(t *testing.T) {
	g := NewEntryGrid(chart(), 8)

	fill(t, g, 0, "1111", "debit", "500.000")
	fill(t, g, 1, "5111", "credit", "500.000")

	if err := g.Validate(); err != nil {
		t.Fatalf("Validate() = %v; six empty rows must not count against the entry", err)
	}
}

// TestEntryGrid_Validate_RefusesAHalfFilledRow — slice 3c: an amount with no
// account, or with no side, is not an entry that can be posted and is not an
// empty row either. Refusing it is the only honest answer.
func TestEntryGrid_Validate_RefusesAHalfFilledRow(t *testing.T) {
	tests := []struct {
		name  string
		build func(t *testing.T, g *EntryGrid)
	}{
		{
			name: "an amount with no account",
			build: func(t *testing.T, g *EntryGrid) {
				t.Helper()
				fill(t, g, 0, "1111", "debit", "500.000")
				fill(t, g, 1, "", "credit", "500.000")
			},
		},
		{
			name: "an amount with no side",
			build: func(t *testing.T, g *EntryGrid) {
				t.Helper()
				fill(t, g, 0, "1111", "debit", "500.000")
				fill(t, g, 1, "5111", "", "500.000")
			},
		},
		{
			name: "an account and a side with no amount",
			build: func(t *testing.T, g *EntryGrid) {
				t.Helper()
				fill(t, g, 0, "1111", "debit", "500.000")
				fill(t, g, 1, "5111", "credit", "")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewEntryGrid(chart(), 2)
			tt.build(t, g)

			if err := g.Validate(); err == nil {
				t.Fatal("Validate() = nil; a half-filled row must be refused")
			}
		})
	}
}

// TestEntryGrid_Validate_RefusesASingleLine — slice 3d: a whole entry needs two
// sides. One movement alone is not an entry.
func TestEntryGrid_Validate_RefusesASingleLine(t *testing.T) {
	g := NewEntryGrid(chart(), 4)

	fill(t, g, 0, "1111", "debit", "500.000")

	if err := g.Validate(); err == nil {
		t.Fatal("Validate() = nil; an entry with one movement must be refused")
	}
}

// TestEntryGrid_Totals — slice 3e: the running totals the grid shows under the
// last row. They are what tells the operator which way the entry is out.
func TestEntryGrid_Totals(t *testing.T) {
	g := NewEntryGrid(chart(), 3)

	// Two debits and one credit: an entry that does not agree, by design.
	fill(t, g, 0, "1111", "debit", "300.000")
	fill(t, g, 1, "632", "debit", "200.000")
	fill(t, g, 2, "5111", "credit", "450.000")

	debit, credit := g.Totals()

	if debit != 500_000 {
		t.Errorf("debit total = %d, want 500000", debit)
	}
	if credit != 450_000 {
		t.Errorf("credit total = %d, want 450000", credit)
	}
}

// TestEntryGrid_AddRowAndRemoveRow — slice 4: the structure the keyboard drives.
// An entry of unknown length needs a row added at the end and taken away again.
func TestEntryGrid_AddRowAndRemoveRow(t *testing.T) {
	g := NewEntryGrid(chart(), 1)
	fill(t, g, 0, "1111", "debit", "100.000")

	g.AddRow()

	if got := g.RowCount(); got != 2 {
		t.Fatalf("after AddRow, RowCount = %d, want 2", got)
	}
	if got := g.Cells(0).Account; got != "1111" {
		t.Errorf("AddRow disturbed the row above it: account = %q, want 1111", got)
	}

	fill(t, g, 1, "5111", "credit", "100.000")

	// Removing the first row moves the second one up, and the entry still
	// agrees with itself.
	g.RemoveRow(0)

	if got := g.RowCount(); got != 1 {
		t.Fatalf("after RemoveRow, RowCount = %d, want 1", got)
	}
	if got := g.Cells(0).Account; got != "5111" {
		t.Errorf("the row below did not move up: account = %q, want 5111", got)
	}
}

// TestEntryGrid_RemoveRowRefusesTheLastOne — slice 4b: a grid with no rows has
// nowhere to type. The last row cannot be removed.
func TestEntryGrid_RemoveRowRefusesTheLastOne(t *testing.T) {
	g := NewEntryGrid(chart(), 1)

	g.RemoveRow(0)

	if got := g.RowCount(); got != 1 {
		t.Errorf("RowCount = %d after removing the only row, want 1", got)
	}
}

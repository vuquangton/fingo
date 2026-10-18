package voucher_test

import (
	"testing"
	"time"

	"github.com/google/uuid"

	"fingo/internal/domain/gl"
	"fingo/internal/ui/voucher"
)

var postingDay = time.Date(2026, 3, 15, 0, 0, 0, 0, time.UTC)

// balanced is a grid an operator has filled in and that agrees with itself.
func balanced(t *testing.T) *voucher.EntryGrid {
	t.Helper()

	g := voucher.NewEntryGrid(chart(), 4)
	fill(t, g, 0, "1111", "debit", "500.000")
	fill(t, g, 1, "632", "debit", "1.500.000")
	fill(t, g, 2, "5111", "credit", "2.000.000")

	return g
}

// TestEntryGrid_Voucher_LineIDsFitTheirColumn — found by driving the real
// application, and by nothing else.
//
// journal_lines.id is VARCHAR(36), and a UUID is exactly 36 characters. Deriving
// a line's id from the voucher's — the obvious thing, and what this did — makes
// it two characters too long, and MariaDB refuses the insert with
// "Data too long for column 'id'" on the *first voucher a real user enters*.
//
// The integration test passed because it used an id of its own choosing, and
// "vch-integration-1" is short enough to hide the bug. The application uses a
// UUID, so the test has to as well.
func TestEntryGrid_Voucher_LineIDsFitTheirColumn(t *testing.T) {
	const columnWidth = 36

	// What the application actually passes.
	voucherID := uuid.NewString()

	if len(voucherID) != columnWidth {
		t.Fatalf("the test's premise is wrong: a UUID is %d characters", len(voucherID))
	}

	g := balanced(t)

	v, err := g.Voucher(voucherID, postingDay, "Bút toán kiểm tra")
	if err != nil {
		t.Fatalf("Voucher: %v", err)
	}

	seen := map[string]bool{}

	for i, line := range v.Lines() {
		if len(line.ID) > columnWidth {
			t.Errorf("line %d id is %d characters, and the column holds %d: %q",
				i+1, len(line.ID), columnWidth, line.ID)
		}
		if line.ID == "" {
			t.Errorf("line %d has no id", i+1)
		}
		if seen[line.ID] {
			t.Errorf("line %d reuses the id %q, and the column is a primary key", i+1, line.ID)
		}
		seen[line.ID] = true
	}
}

// TestEntryGrid_Voucher_CarriesTheEntryTheOperatorTyped — the conversion from
// the grid to the ledger's own voucher.
//
// It is a rule rather than plumbing, which is why it is tested: the order of the
// lines, which side each one lands on, and whether the row's note travels with
// it are all decisions, and a screen that got them wrong would post a voucher
// that balances and says the wrong thing.
func TestEntryGrid_Voucher_CarriesTheEntryTheOperatorTyped(t *testing.T) {
	g := balanced(t)

	v, err := g.Voucher("vch-1", postingDay, "Kết chuyển giá vốn")
	if err != nil {
		t.Fatalf("Voucher: %v", err)
	}

	if v.ID() != "vch-1" {
		t.Errorf("ID = %q", v.ID())
	}
	if v.Description() != "Kết chuyển giá vốn" {
		t.Errorf("Description = %q", v.Description())
	}
	if v.VoucherType() != gl.VoucherTypeGeneral {
		t.Errorf("VoucherType = %q, want GENERAL — this is a phiếu kế toán", v.VoucherType())
	}
	if !v.VoucherDate().Equal(postingDay) {
		t.Errorf("VoucherDate = %s, want %s", v.VoucherDate(), postingDay)
	}

	lines := v.Lines()
	if len(lines) != 3 {
		t.Fatalf("the voucher has %d lines, want 3", len(lines))
	}

	want := []struct {
		account string
		side    gl.EntrySide
		amount  string
	}{
		{"1111", gl.SideDebit, "500000"},
		{"632", gl.SideDebit, "1500000"},
		{"5111", gl.SideCredit, "2000000"},
	}

	for i, w := range want {
		got := lines[i]

		if got.AccountCode != w.account {
			t.Errorf("line %d account = %q, want %q", i+1, got.AccountCode, w.account)
		}
		if got.Side != w.side {
			t.Errorf("line %d side = %q, want %q", i+1, got.Side, w.side)
		}
		if got.Amount.String() != w.amount {
			t.Errorf("line %d amount = %s, want %s", i+1, got.Amount, w.amount)
		}
		if got.LineOrder != i+1 {
			t.Errorf("line %d LineOrder = %d, want %d", i+1, got.LineOrder, i+1)
		}
		if got.VoucherID != "vch-1" {
			t.Errorf("line %d VoucherID = %q", i+1, got.VoucherID)
		}
	}
}

// TestEntryGrid_Voucher_SkipsTheRowsNobodyFilledIn — the grid renders more rows
// than most vouchers need, and an untouched row is not a line.
//
// The lines that remain must keep their original order, because the operator
// reads the voucher back in the order they typed it.
func TestEntryGrid_Voucher_SkipsTheRowsNobodyFilledIn(t *testing.T) {
	g := voucher.NewEntryGrid(chart(), 6)
	fill(t, g, 0, "1111", "debit", "500.000")
	fill(t, g, 3, "5111", "credit", "500.000")

	v, err := g.Voucher("vch-2", postingDay, "Bút toán có dòng trống ở giữa")
	if err != nil {
		t.Fatalf("Voucher: %v", err)
	}

	lines := v.Lines()
	if len(lines) != 2 {
		t.Fatalf("the voucher has %d lines, want 2", len(lines))
	}
	if lines[0].AccountCode != "1111" || lines[1].AccountCode != "5111" {
		t.Errorf("lines = %s then %s, want 1111 then 5111", lines[0].AccountCode, lines[1].AccountCode)
	}
	if lines[1].LineOrder != 2 {
		t.Errorf("the second line is number %d, want 2 — numbering must not follow the grid's gaps", lines[1].LineOrder)
	}
}

// TestEntryGrid_Voucher_CarriesEachRowsOwnNote — the description column is per
// row, and a ledger line with no explanation cannot be defended to an auditor.
func TestEntryGrid_Voucher_CarriesEachRowsOwnNote(t *testing.T) {
	g := voucher.NewEntryGrid(chart(), 2)

	if err := g.SetCell(0, voucher.ColAccount, "1111"); err != nil {
		t.Fatal(err)
	}
	if err := g.SetCell(0, voucher.ColSide, "debit"); err != nil {
		t.Fatal(err)
	}
	if err := g.SetCell(0, voucher.ColAmount, "500.000"); err != nil {
		t.Fatal(err)
	}
	if err := g.SetCell(0, voucher.ColDescription, "Rút tiền gửi về quỹ"); err != nil {
		t.Fatal(err)
	}

	fill(t, g, 1, "5111", "credit", "500.000")

	v, err := g.Voucher("vch-3", postingDay, "Rút tiền")
	if err != nil {
		t.Fatalf("Voucher: %v", err)
	}

	if got := v.Lines()[0].Note; got != "Rút tiền gửi về quỹ" {
		t.Errorf("line 1 note = %q, want the row's own description", got)
	}
}

// TestEntryGrid_Voucher_RefusesAnEntryThatDoesNotBalance — the ledger would
// refuse it too, but by then the operator has lost the screen and is reading a
// database error about a voucher they can no longer see.
func TestEntryGrid_Voucher_RefusesAnEntryThatDoesNotBalance(t *testing.T) {
	g := voucher.NewEntryGrid(chart(), 2)
	fill(t, g, 0, "1111", "debit", "500.000")
	fill(t, g, 1, "5111", "credit", "400.000")

	if _, err := g.Voucher("vch-4", postingDay, "Bút toán lệch"); err == nil {
		t.Fatal("Voucher built a voucher from an entry whose sides disagree")
	}
}

// TestEntryGrid_Voucher_RefusesAnEmptyGrid — an empty grid is not a voucher, and
// the ledger's own complaint about it would arrive a layer too late.
func TestEntryGrid_Voucher_RefusesAnEmptyGrid(t *testing.T) {
	g := voucher.NewEntryGrid(chart(), 4)

	if _, err := g.Voucher("vch-5", postingDay, "Chưa nhập gì"); err == nil {
		t.Fatal("Voucher built a voucher from an empty grid")
	}
}

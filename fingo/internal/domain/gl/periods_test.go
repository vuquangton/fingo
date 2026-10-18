package gl_test

import (
	"testing"
	"time"

	"fingo/internal/domain/gl"
)

func ym(year int, month time.Month) gl.YearMonth {
	return gl.YearMonth{Year: year, Month: month}
}

func on(t *testing.T, day string) time.Time {
	t.Helper()

	parsed, err := time.Parse("2006-01-02", day)
	if err != nil {
		t.Fatalf("parse %s: %v", day, err)
	}

	return parsed
}

// A client with no closed periods can post anywhere. That is the honest default
// for books nobody has closed yet.
func TestPeriods_EverythingIsOpenByDefault(t *testing.T) {
	p := gl.NewPeriods()

	if p.IsClosed(ym(2026, time.March)) {
		t.Fatal("no period is closed on a fresh calendar")
	}
	if !p.IsOpen(on(t, "2026-03-14")) {
		t.Fatal("a fresh calendar must allow any date")
	}
	if got := p.Closed(); len(got) != 0 {
		t.Fatalf("Closed() = %v, want nothing", got)
	}
}

// Closing covers the whole month, first and last day included.
func TestPeriods_ClosingCoversTheWholeMonth(t *testing.T) {
	p := gl.NewPeriods()

	if err := p.Close(ym(2026, time.February)); err != nil {
		t.Fatalf("Close: %v", err)
	}

	for _, day := range []string{"2026-02-01", "2026-02-14", "2026-02-28"} {
		if p.IsOpen(on(t, day)) {
			t.Errorf("%s falls in a closed period but is reported open", day)
		}
		if !p.IsClosed(gl.MonthOf(on(t, day))) {
			t.Errorf("%s falls in a closed period but IsClosed says otherwise", day)
		}
	}

	for _, day := range []string{"2026-01-31", "2026-03-01"} {
		if !p.IsOpen(on(t, day)) {
			t.Errorf("%s falls outside the closed period but is reported closed", day)
		}
	}
}

// Closing twice is a mistake rather than a no-op: the second attempt means the
// caller believed the period was still open.
func TestPeriods_CannotCloseTwice(t *testing.T) {
	p := gl.NewPeriods()

	if err := p.Close(ym(2026, time.February)); err != nil {
		t.Fatalf("first Close: %v", err)
	}
	if err := p.Close(ym(2026, time.February)); err == nil {
		t.Fatal("closing an already-closed period must be refused, got nil")
	}
}

// Reopening is an explicit act, and reopening something already open is equally
// a mistake — it means the caller has lost track of the period's state.
func TestPeriods_ReopenIsExplicit(t *testing.T) {
	p := gl.NewPeriods()

	if err := p.Reopen(ym(2026, time.February)); err == nil {
		t.Fatal("reopening a period that is not closed must be refused, got nil")
	}

	if err := p.Close(ym(2026, time.February)); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if err := p.Reopen(ym(2026, time.February)); err != nil {
		t.Fatalf("Reopen: %v", err)
	}
	if p.IsClosed(ym(2026, time.February)) {
		t.Fatal("a reopened period must be open again")
	}
}

// The closed set is reported oldest first, so callers and error messages are
// deterministic.
func TestPeriods_ClosedIsSorted(t *testing.T) {
	p := gl.NewPeriods()

	for _, m := range []gl.YearMonth{
		ym(2026, time.March), ym(2026, time.January), ym(2025, time.December),
	} {
		if err := p.Close(m); err != nil {
			t.Fatalf("Close(%v): %v", m, err)
		}
	}

	got := p.Closed()
	want := []gl.YearMonth{ym(2025, time.December), ym(2026, time.January), ym(2026, time.March)}

	if len(got) != len(want) {
		t.Fatalf("Closed() = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("Closed() = %v, want %v", got, want)
		}
	}
}

// A posted figure must not change underneath. This is the control that stops it.
func TestVoucher_PostRefusesAClosedPeriod(t *testing.T) {
	closed := gl.NewPeriods()
	if err := closed.Close(gl.MonthOf(voucherDate)); err != nil {
		t.Fatalf("Close: %v", err)
	}

	v := newVoucher("v-30", "PK030", gl.VoucherTypeGeneral)
	mustAdd(t, v, "1111", gl.SideDebit, 100)
	mustAdd(t, v, "5111", gl.SideCredit, 100)

	rules := testRules(t)
	rules.Periods = closed

	if err := v.Post(rules, testNumber(t, v), voucherDate, "operator-1"); err == nil {
		t.Fatal("posting into a closed period must be refused, got nil")
	}
	if v.IsPosted() {
		t.Fatal("a voucher refused for a closed period must stay unposted")
	}
}

// Posting without a calendar is not possible either: the calendar is rules, and
// rules cannot be omitted.
func TestVoucher_PostRequiresAPeriodCalendar(t *testing.T) {
	v := newVoucher("v-31", "PK031", gl.VoucherTypeGeneral)
	mustAdd(t, v, "1111", gl.SideDebit, 100)
	mustAdd(t, v, "5111", gl.SideCredit, 100)

	rules := testRules(t)
	rules.Periods = nil

	if err := v.Post(rules, testNumber(t, v), voucherDate, "operator-1"); err == nil {
		t.Fatal("posting with no period calendar must be refused, got nil")
	}
}

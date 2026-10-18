package nav_test

import (
	"strings"
	"testing"

	"fingo/internal/ui/nav"
)

// nothing is a shell that can open nothing at all — which is P0's honest state.
type nothing struct{}

func (nothing) CanOpen(string) bool { return false }

// only is a shell that can open exactly the ids it was given.
type only map[string]bool

func (o only) CanOpen(id string) bool { return o[id] }

// TestRail_EveryEntrySaysWhatItIs — a rail entry with no id, no label or no
// section renders as a blank row, and a blank row in a navigation rail is worse
// than a missing one: it looks like something to click.
func TestRail_EveryEntrySaysWhatItIs(t *testing.T) {
	for _, e := range nav.Rail() {
		t.Run(e.ID, func(t *testing.T) {
			switch {
			case strings.TrimSpace(e.ID) == "":
				t.Errorf("%q has no id", e.Label)
			case strings.TrimSpace(e.Label) == "":
				t.Errorf("%q has no label", e.ID)
			case strings.TrimSpace(e.Section) == "":
				t.Errorf("%s has no section, so it lands nowhere in the rail", e.ID)
			}
		})
	}
}

// TestRail_NoDuplicateIDs — two entries with one id are one destination, and the
// second silently shadows the first.
func TestRail_NoDuplicateIDs(t *testing.T) {
	seen := map[string]bool{}

	for _, e := range nav.Rail() {
		if seen[e.ID] {
			t.Errorf("%s appears twice in the rail", e.ID)
		}
		seen[e.ID] = true
	}
}

// TestRail_EveryEntryCanExplainItself — this is the plan's rule, made
// enforceable instead of remembered:
//
//	A rail entry with no engine behind it renders "chưa hỗ trợ" and why — never a
//	dead link and never an empty table.
//
// An entry with no reason cannot honour that, so the reason is required for
// every entry, including the ones that work today.
func TestRail_EveryEntryCanExplainItself(t *testing.T) {
	for _, e := range nav.Rail() {
		if strings.TrimSpace(e.Reason) == "" {
			t.Errorf("%s has no reason, so it cannot say why it is unavailable", e.ID)
		}
	}
}

// TestEntry_OpenAllowsWhatTheShellHasWired — an entry whose engine exists opens.
func TestEntry_OpenAllowsWhatTheShellHasWired(t *testing.T) {
	e := nav.Entry{ID: "today", Label: "Hôm nay", Section: nav.SectionWork, Reason: "the work cycle"}

	if err := e.Open(only{"today": true}); err != nil {
		t.Errorf("Open on a wired entry = %v, want nil", err)
	}
}

// TestEntry_OpenRefusesWithTheReason — and an entry whose engine does not exist
// refuses, in words, rather than opening an empty screen.
//
// The refusal says "chưa hỗ trợ" because that is the phrase the operator will
// see; the reason follows it. An empty screen teaches the operator that the
// application is broken; this teaches them that the feature is not built yet,
// which is a different and true thing.
func TestEntry_OpenRefusesWithTheReason(t *testing.T) {
	e := nav.Entry{
		ID:      "payroll",
		Label:   "Tiền lương",
		Section: nav.SectionBusiness,
		Reason:  "chưa có bảng lương, bảo hiểm và thuế TNCN",
	}

	err := e.Open(nothing{})
	if err == nil {
		t.Fatal("Open on an unwired entry = nil, want a refusal")
	}

	if !strings.Contains(err.Error(), "chưa hỗ trợ") {
		t.Errorf("refusal %q does not say chưa hỗ trợ", err)
	}
	if !strings.Contains(err.Error(), "Tiền lương") {
		t.Errorf("refusal %q does not name the entry", err)
	}
	if !strings.Contains(err.Error(), "bảng lương") {
		t.Errorf("refusal %q does not give the reason", err)
	}
}

// TestRail_NamesTheModulesWithNoEngineBehindThem — the thirteen modules the plan
// deliberately does not build have to appear in the rail, as unsupported.
//
// Leaving them out would be tidier and would tell the operator that FinGo has no
// payroll. Showing them greyed out with a reason tells them FinGo is an
// accounting application that has not got there yet — which is the truth, and
// the difference decides whether they keep looking or stop.
func TestRail_NamesTheModulesWithNoEngineBehindThem(t *testing.T) {
	rail := map[string]bool{}
	for _, e := range nav.Rail() {
		rail[e.ID] = true
	}

	for _, want := range []string{
		"today", "sales", "purchase", "cash", "bank", "inventory",
		"tools", "assets", "payroll", "einvoice", "books", "reports",
		"tax", "masterdata", "practice", "system",
	} {
		if !rail[want] {
			t.Errorf("the rail does not name %q", want)
		}
	}
}

// TestRail_SectionsAreTheOnesTheRailShows — the ordering is the information
// architecture, so it is asserted rather than left to whatever order a map
// happens to iterate in.
func TestRail_SectionsAreTheOnesTheRailShows(t *testing.T) {
	want := []string{
		nav.SectionWork,
		nav.SectionBusiness,
		nav.SectionBooks,
		nav.SectionMasterData,
		nav.SectionSystem,
	}

	var got []string

	for _, e := range nav.Rail() {
		if len(got) == 0 || got[len(got)-1] != e.Section {
			got = append(got, e.Section)
		}
	}

	if len(got) != len(want) {
		t.Fatalf("the rail runs through sections %v, want %v", got, want)
	}

	for i := range want {
		if got[i] != want[i] {
			t.Errorf("section %d is %q, want %q", i, got[i], want[i])
		}
	}
}

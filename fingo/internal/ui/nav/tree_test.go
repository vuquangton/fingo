package nav_test

import (
	"testing"

	"fingo/internal/ui/nav"
)

// TestBuildTree_SectionsBecomeTheRoots — the rail shows five sections, in order,
// and each holds its own entries.
//
// The tree is built from the same Rail() the rest of the application reads, so
// an entry cannot exist in the navigation and be missing from the rail the
// operator sees.
func TestBuildTree_SectionsBecomeTheRoots(t *testing.T) {
	roots := nav.BuildTree(nav.Rail())

	if len(roots) != 5 {
		t.Fatalf("the rail has %d sections, want 5", len(roots))
	}

	want := []string{
		nav.SectionWork, nav.SectionBusiness, nav.SectionBooks,
		nav.SectionMasterData, nav.SectionSystem,
	}

	for i, w := range want {
		if got := roots[i].Text(); got != w {
			t.Errorf("section %d is %q, want %q", i, got, w)
		}
	}
}

// TestBuildTree_EveryEntryHangsOffItsSection — nothing may be dropped and
// nothing may be invented: the tree holds the same entries as the rail.
func TestBuildTree_EveryEntryHangsOffItsSection(t *testing.T) {
	rail := nav.Rail()
	roots := nav.BuildTree(rail)

	total := 0
	for _, r := range roots {
		total += r.ChildCount()
	}

	if total != len(rail) {
		t.Fatalf("the tree holds %d entries, want %d", total, len(rail))
	}

	for _, r := range roots {
		for i := 0; i < r.ChildCount(); i++ {
			child := r.ChildAt(i)

			if child.Parent() != r {
				t.Errorf("%q does not know its section is %q", child.Text(), r.Text())
			}
			if child.ChildCount() != 0 {
				t.Errorf("%q is a leaf but has %d children", child.Text(), child.ChildCount())
			}
		}
	}

	// And the order within a section is the rail's order, not a map's.
	for _, r := range roots {
		var fromTree []string

		for i := 0; i < r.ChildCount(); i++ {
			fromTree = append(fromTree, r.ChildAt(i).Text())
		}

		var fromRail []string

		for _, e := range rail {
			if e.Section == r.Text() {
				fromRail = append(fromRail, e.Label)
			}
		}

		if len(fromTree) != len(fromRail) {
			t.Fatalf("%s has %d entries in the tree and %d in the rail", r.Text(), len(fromTree), len(fromRail))
		}

		for i := range fromRail {
			if fromTree[i] != fromRail[i] {
				t.Errorf("%s entry %d is %q in the tree and %q in the rail",
					r.Text(), i, fromTree[i], fromRail[i])
			}
		}
	}
}

// TestBuildTree_HoldsTheEntryNotJustItsLabel — the operator clicks a label, but
// the application needs the entry: its id decides what opens, and its reason
// decides what to say when nothing can.
func TestBuildTree_HoldsTheEntryNotJustItsLabel(t *testing.T) {
	roots := nav.BuildTree(nav.Rail())

	var found bool

	for _, r := range roots {
		for i := 0; i < r.ChildCount(); i++ {
			child := r.ChildAt(i)

			if !child.HasEntry() {
				t.Errorf("%q hangs off %q but carries no destination", child.Text(), r.Text())
				continue
			}
			if child.Entry().ID == "payroll" {
				found = true

				if child.Entry().Reason == "" {
					t.Error("the payroll entry reached the tree with no reason")
				}
			}
		}
	}

	if !found {
		t.Error("no node in the tree carries its nav.Entry")
	}
}

// TestBuildTree_ASectionWithNoEntriesIsNotShown — a heading with nothing under it
// is a promise the rail does not keep.
func TestBuildTree_ASectionWithNoEntriesIsNotShown(t *testing.T) {
	roots := nav.BuildTree([]nav.Entry{
		{ID: "a", Section: "ONE", Label: "A", Reason: "r"},
		{ID: "b", Section: "TWO", Label: "B", Reason: "r"},
	})

	if len(roots) != 2 {
		t.Fatalf("got %d sections, want 2", len(roots))
	}
}

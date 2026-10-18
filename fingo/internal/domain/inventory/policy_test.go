package inventory_test

import (
	"testing"

	"fingo/internal/domain/inventory"
)

// The method has to be chosen, not defaulted. A default would value a client's
// stock by a method they never picked, and the choice has to appear in their
// accounting-policy note.
func TestPolicy_AYearWithNoMethodIsAnError(t *testing.T) {
	p := inventory.NewPolicy()

	if _, err := p.MethodFor(2026); err == nil {
		t.Fatal("a year with no chosen method must be an error, got nil")
	}
}

func TestPolicy_RemembersTheChosenMethod(t *testing.T) {
	p := inventory.NewPolicy()

	if err := p.Set(2026, inventory.MethodMovingAverage); err != nil {
		t.Fatalf("Set: %v", err)
	}

	got, err := p.MethodFor(2026)
	if err != nil {
		t.Fatalf("MethodFor: %v", err)
	}
	if got != inventory.MethodMovingAverage {
		t.Errorf("method = %s, want %s", got, inventory.MethodMovingAverage)
	}
}

// TT133 requires one method for a whole fiscal year. Changing it within the year
// would silently restate a cost of sales that has already been reported.
func TestPolicy_RefusesAChangeWithinTheSameFiscalYear(t *testing.T) {
	p := inventory.NewPolicy()

	if err := p.Set(2026, inventory.MethodMovingAverage); err != nil {
		t.Fatalf("Set: %v", err)
	}

	err := p.Set(2026, inventory.MethodFIFO)
	if err == nil {
		t.Fatal("changing a fiscal year's method must be refused, got nil")
	}

	// And the original choice stands.
	got, err := p.MethodFor(2026)
	if err != nil {
		t.Fatalf("MethodFor: %v", err)
	}
	if got != inventory.MethodMovingAverage {
		t.Errorf("method = %s, want the original %s", got, inventory.MethodMovingAverage)
	}
}

// Choosing the same method again is the same decision, not a change.
func TestPolicy_SettingTheSameMethodAgainIsAllowed(t *testing.T) {
	p := inventory.NewPolicy()

	for i := 0; i < 2; i++ {
		if err := p.Set(2026, inventory.MethodMovingAverage); err != nil {
			t.Fatalf("Set (attempt %d): %v", i+1, err)
		}
	}
}

// Consistency is required within a year, not across years: a business may value
// 2026 by moving average and 2027 by FIFO.
func TestPolicy_AllowsADifferentMethodInADifferentYear(t *testing.T) {
	p := inventory.NewPolicy()

	if err := p.Set(2026, inventory.MethodMovingAverage); err != nil {
		t.Fatalf("Set 2026: %v", err)
	}
	if err := p.Set(2027, inventory.MethodFIFO); err != nil {
		t.Fatalf("Set 2027: %v", err)
	}

	if got, _ := p.MethodFor(2026); got != inventory.MethodMovingAverage {
		t.Errorf("2026 = %s, want %s", got, inventory.MethodMovingAverage)
	}
	if got, _ := p.MethodFor(2027); got != inventory.MethodFIFO {
		t.Errorf("2027 = %s, want %s", got, inventory.MethodFIFO)
	}
}

func TestPolicy_RefusesAnUnknownMethodOrYear(t *testing.T) {
	p := inventory.NewPolicy()

	if err := p.Set(2026, inventory.Method("MYSTERY")); err == nil {
		t.Error("an unknown method must be refused, got nil")
	}
	if err := p.Set(2026, inventory.Method("")); err == nil {
		t.Error("an empty method must be refused, got nil")
	}
	if err := p.Set(0, inventory.MethodFIFO); err == nil {
		t.Error("fiscal year 0 must be refused, got nil")
	}
}

// Every method TT133 names is a method this code recognises — otherwise a client
// could be refused their lawful choice.
func TestMethod_TheThreeTT133MethodsAreValid(t *testing.T) {
	for _, method := range []inventory.Method{
		inventory.MethodSpecificCost,
		inventory.MethodMovingAverage,
		inventory.MethodPeriodicAverage,
		inventory.MethodFIFO,
	} {
		if !method.Valid() {
			t.Errorf("%s must be valid: TT133 permits it", method)
		}
	}
}

func TestPolicy_YearsAreReportedInOrder(t *testing.T) {
	p := inventory.NewPolicy()

	for _, year := range []int{2027, 2025, 2026} {
		if err := p.Set(year, inventory.MethodFIFO); err != nil {
			t.Fatalf("Set(%d): %v", year, err)
		}
	}

	years := p.Years()
	if len(years) != 3 || years[0] != 2025 || years[1] != 2026 || years[2] != 2027 {
		t.Errorf("Years() = %v, want [2025 2026 2027]", years)
	}
}

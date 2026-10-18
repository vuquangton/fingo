package system_test

import (
	"testing"
	"time"

	"fingo/internal/domain/system"
)

func TestUser_StubInvariants(t *testing.T) {
	u := system.NewUserStub("u-01", "admin", "admin@fingo.local")
	if u.ID != "u-01" || u.Username != "admin" || !u.IsActive {
		t.Fatalf("unexpected user stub state: %+v", u)
	}
}

func TestCompanyProfile_StubInvariants(t *testing.T) {
	cp := system.NewCompanyProfileStub("c-01", "FinGo SME JSC", "0101234567")
	if cp.TaxCode != "0101234567" || cp.FiscalYearStartMonth != time.January {
		t.Fatalf("unexpected company profile stub state: %+v", cp)
	}
}

// Each client's books are kept under exactly one accounting regime, and the
// regime is a closed set of known instruments — not a free-form string. The
// regime drives the chart of accounts, the statutory forms and the books.
// See docs/legal-basis.md.
func TestCompanyProfile_DefaultsToTT133Regime(t *testing.T) {
	cp := system.NewCompanyProfileStub("c-01", "FinGo SME JSC", "0101234567")

	if cp.Regime != system.RegimeTT133 {
		t.Fatalf("default regime = %q, want %q", cp.Regime, system.RegimeTT133)
	}
	if err := cp.Validate(); err != nil {
		t.Fatalf("a default profile must be valid: %v", err)
	}
}

// An unmodelled regime must be refused rather than silently accepted. TT200 is
// the specific trap here: it is superseded for FY2026 and the project README
// still claims compliance with it.
func TestCompanyProfile_RejectsUnsupportedRegime(t *testing.T) {
	cp := system.NewCompanyProfileStub("c-01", "FinGo SME JSC", "0101234567")
	cp.Regime = system.Regime("TT200")

	if err := cp.Validate(); err == nil {
		t.Fatal("an unmodelled regime must be rejected, got nil")
	}
}

// A profile with no tax code cannot produce a statutory return.
func TestCompanyProfile_RejectsMissingTaxCode(t *testing.T) {
	cp := system.NewCompanyProfileStub("c-01", "FinGo SME JSC", "")
	cp.TaxCode = ""

	if err := cp.Validate(); err == nil {
		t.Fatal("a profile with no tax code must be rejected, got nil")
	}
}

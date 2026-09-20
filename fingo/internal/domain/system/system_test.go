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

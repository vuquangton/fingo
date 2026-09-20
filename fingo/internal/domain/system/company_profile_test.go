package system_test

import (
	"testing"
	"time"

	"fingo/internal/domain/system"
)

func TestProductionCompanyProfile_ValidCreation(t *testing.T) {
	req := system.CreateCompanyProfileParams{
		ID:                  "cp-001",
		TaxCode:             "0101243150",
		LegalName:           "CÔNG TY CỔ PHẦN FIN GO",
		TradeName:           "FinGo JSC",
		Address:             "Tầng 5, Tòa nhà Tech, Cầu Giấy, Hà Nội",
		ProvinceCity:        "Hà Nội",
		LegalRepresentative: "Nguyễn Văn Giám Đốc",
		ChiefAccountant:     "Trần Thị Kế Toán",
		TaxAuthorityCode:    "10500",
		TaxAuthorityName:    "Chi cục Thuế Cầu Giấy",
		StateBudgetChapter:  "754",
		Regime:              system.RegimeCircular133,
		VATMethod:           system.VATMethodDeduction,
		CostingMethod:       system.CostingMovingWeighted,
		BusinessType:        system.BusinessTrading,
	}

	profile, err := system.NewProductionCompanyProfile(req)
	if err != nil {
		t.Fatalf("expected valid profile creation, got error: %v", err)
	}

	if profile.BaseCurrency != "VND" {
		t.Errorf("expected BaseCurrency 'VND', got %s", profile.BaseCurrency)
	}
	if profile.FiscalYearStartMonth != time.January {
		t.Errorf("expected FiscalYearStartMonth Jan, got %v", profile.FiscalYearStartMonth)
	}
	if !profile.IsActive {
		t.Errorf("expected profile to be active")
	}
}

func TestProductionCompanyProfile_ValidationFailures(t *testing.T) {
	validReq := system.CreateCompanyProfileParams{
		ID:                  "cp-001",
		TaxCode:             "0101243150",
		LegalName:           "CÔNG TY CỔ PHẦN FIN GO",
		Address:             "Hà Nội",
		LegalRepresentative: "Nguyễn Văn Giám Đốc",
		ChiefAccountant:     "Trần Thị Kế Toán",
		TaxAuthorityCode:    "10500",
		TaxAuthorityName:    "Chi cục Thuế Cầu Giấy",
		StateBudgetChapter:  "754",
		Regime:              system.RegimeCircular133,
		VATMethod:           system.VATMethodDeduction,
		CostingMethod:       system.CostingMovingWeighted,
		BusinessType:        system.BusinessTrading,
	}

	// Case 1: Invalid Tax Code
	invMST := validReq
	invMST.TaxCode = "0101243159" // invalid checksum
	if _, err := system.NewProductionCompanyProfile(invMST); err == nil {
		t.Errorf("expected error for invalid tax code checksum")
	}

	// Case 2: Empty Legal Name
	emptyName := validReq
	emptyName.LegalName = ""
	if _, err := system.NewProductionCompanyProfile(emptyName); err == nil {
		t.Errorf("expected error for empty legal name")
	}

	// Case 3: Empty Tax Authority
	noTaxAuth := validReq
	noTaxAuth.TaxAuthorityCode = ""
	if _, err := system.NewProductionCompanyProfile(noTaxAuth); err == nil {
		t.Errorf("expected error for missing tax authority code")
	}
}

func TestProductionCompanyProfile_LockDateEnforcement(t *testing.T) {
	profile, err := system.NewProductionCompanyProfile(system.CreateCompanyProfileParams{
		ID:                  "cp-001",
		TaxCode:             "0101243150",
		LegalName:           "CÔNG TY FIN GO",
		Address:             "Hà Nội",
		LegalRepresentative: "Giám Đốc",
		ChiefAccountant:     "Kế Toán",
		TaxAuthorityCode:    "10500",
		TaxAuthorityName:    "Thuế Cầu Giấy",
		Regime:              system.RegimeCircular133,
		VATMethod:           system.VATMethodDeduction,
		CostingMethod:       system.CostingFIFO,
		BusinessType:        system.BusinessTrading,
	})
	if err != nil {
		t.Fatalf("failed creating profile: %v", err)
	}

	lockDate := time.Date(2026, 1, 31, 23, 59, 59, 0, time.UTC)
	profile.SetLockDate(lockDate)

	// Voucher on or before lock date must be rejected
	beforeDate := time.Date(2026, 1, 15, 10, 0, 0, 0, time.UTC)
	if err := profile.CheckVoucherDate(beforeDate); err == nil {
		t.Errorf("expected voucher before lock date to be rejected")
	}

	// Voucher after lock date must be allowed
	afterDate := time.Date(2026, 2, 1, 10, 0, 0, 0, time.UTC)
	if err := profile.CheckVoucherDate(afterDate); err != nil {
		t.Errorf("expected voucher after lock date to be accepted, got %v", err)
	}
}

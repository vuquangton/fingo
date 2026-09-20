package repository_test

import (
	"context"
	"testing"

	"fingo/internal/adapter/mariadb/repository"
	"fingo/internal/domain/system"
)

func TestBranchOrgUnitRepo_CreateAndList(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	companyRepo := repository.NewCompanyProfileRepo(db)
	orgRepo := repository.NewBranchOrgUnitRepo(db)
	ctx := context.Background()

	// 1. Ensure company exists
	activeComp, err := companyRepo.GetProfile(ctx)
	var companyID string
	var parentMST string
	if err == nil && activeComp != nil {
		companyID = activeComp.ID
		parentMST = activeComp.TaxCode
	} else {
		companyID = "test-cp-org-01"
		parentMST = "0101243150"
		comp, err := system.NewProductionCompanyProfile(system.CreateCompanyProfileParams{
			ID:                  companyID,
			TaxCode:             parentMST,
			LegalName:           "CÔNG TY TNHH TEST ORG",
			Address:             "Hà Nội",
			LegalRepresentative: "Giám Đốc",
			ChiefAccountant:     "Kế Toán Trưởng",
			TaxAuthorityCode:    "10500",
			TaxAuthorityName:    "Chi cục Thuế Cầu Giấy",
			Regime:              system.RegimeCircular133,
			VATMethod:           system.VATMethodDeduction,
			CostingMethod:       system.CostingMovingWeighted,
			BusinessType:        system.BusinessTrading,
		})
		if err != nil {
			t.Fatalf("failed creating test company: %v", err)
		}
		if err := companyRepo.SaveProfile(ctx, comp); err != nil {
			t.Fatalf("failed saving company profile: %v", err)
		}
	}

	// 2. Create Branch
	branch, err := system.NewBranchOrgUnit(system.CreateBranchOrgUnitParams{
		ID:                   "b-unit-01",
		CompanyProfileID:     companyID,
		ParentCompanyTaxCode: parentMST,
		Code:                 "CN_HCM",
		Name:                 "Chi nhánh TP Hồ Chí Minh",
		UnitType:             system.OrgUnitBranch,
		AccountingGovernance: system.GovDependent,
		TaxFilingMechanism:   system.TaxFilingAllocated,
		TaxCode:              "0101243150-001",
		TaxAuthorityCode:     "79001",
		TaxAuthorityName:     "Chi cục Thuế Quận 1",
		ProvinceCityCode:     "79",
		Address:              "Nguyễn Huệ, Quận 1, TP HCM",
	})
	if err != nil {
		t.Fatalf("failed creating domain branch: %v", err)
	}

	// Clean up previous test runs if any
	_, _ = db.Exec("DELETE FROM branch_org_units WHERE id = ?", "b-unit-01")

	if err := orgRepo.CreateOrgUnit(ctx, branch); err != nil {
		t.Fatalf("CreateOrgUnit failed: %v", err)
	}

	// Fetch by ID
	fetched, err := orgRepo.GetOrgUnitByID(ctx, "b-unit-01")
	if err != nil {
		t.Fatalf("GetOrgUnitByID failed: %v", err)
	}
	if fetched.Code != "CN_HCM" || fetched.TaxCode != "0101243150-001" {
		t.Errorf("unexpected fetched branch: %+v", fetched)
	}

	// List by Company
	list, err := orgRepo.ListOrgUnitsByCompany(ctx, companyID)
	if err != nil {
		t.Fatalf("ListOrgUnitsByCompany failed: %v", err)
	}
	if len(list) == 0 {
		t.Errorf("expected at least 1 org unit, got 0")
	}
}

package system_test

import (
	"testing"

	"fingo/internal/domain/system"
)

func TestBranchOrgUnit_ValidCreation(t *testing.T) {
	parentMST := "0101243150"

	// 1. Valid Branch in different province (Đà Nẵng vs Hà Nội)
	branchParams := system.CreateBranchOrgUnitParams{
		ID:                   "b-01",
		CompanyProfileID:     "cp-01",
		ParentCompanyTaxCode: parentMST,
		Code:                 "CN_DANANG",
		Name:                 "Chi nhánh Đà Nẵng",
		UnitType:             system.OrgUnitBranch,
		AccountingGovernance: system.GovDependent,
		TaxFilingMechanism:   system.TaxFilingAllocated,
		TaxCode:              "0101243150-001",
		TaxAuthorityCode:     "48001",
		TaxAuthorityName:     "Chi cục Thuế Quận Hải Châu",
		ProvinceCityCode:     "48", // Đà Nẵng
		Address:              "123 Nguyễn Văn Linh, Đà Nẵng",
	}

	branch, err := system.NewBranchOrgUnit(branchParams)
	if err != nil {
		t.Fatalf("expected valid branch creation, got %v", err)
	}
	if branch.Code != "CN_DANANG" || branch.TaxCode != "0101243150-001" {
		t.Errorf("unexpected branch attributes: %+v", branch)
	}

	// 2. Valid Business Location (Mã 5 số)
	locParams := system.CreateBranchOrgUnitParams{
		ID:                   "loc-01",
		CompanyProfileID:     "cp-01",
		ParentCompanyTaxCode: parentMST,
		Code:                 "KHO_01",
		Name:                 "Kho Tổng Hòa Khánh",
		UnitType:             system.OrgUnitBusinessLocation,
		AccountingGovernance: system.GovCostCenter,
		TaxFilingMechanism:   system.TaxFilingCentralized,
		TaxCode:              "00001",
		ProvinceCityCode:     "48",
		Address:              "KCN Hòa Khánh, Đà Nẵng",
	}

	loc, err := system.NewBranchOrgUnit(locParams)
	if err != nil {
		t.Fatalf("expected valid business location, got %v", err)
	}
	if loc.TaxCode != "00001" {
		t.Errorf("expected 5-digit location code 00001, got %s", loc.TaxCode)
	}

	// 3. Valid Department (Cost center without tax code)
	deptParams := system.CreateBranchOrgUnitParams{
		ID:                   "dept-01",
		CompanyProfileID:     "cp-01",
		ParentCompanyTaxCode: parentMST,
		Code:                 "PB_KETOAN",
		Name:                 "Phòng Kế toán Tài chính",
		UnitType:             system.OrgUnitDepartment,
		AccountingGovernance: system.GovCostCenter,
		TaxFilingMechanism:   system.TaxFilingCentralized,
		ProvinceCityCode:     "01",
		Address:              "Trụ sở chính Hà Nội",
	}

	dept, err := system.NewBranchOrgUnit(deptParams)
	if err != nil {
		t.Fatalf("expected valid department, got %v", err)
	}
	if dept.UnitType != system.OrgUnitDepartment {
		t.Errorf("expected department unit type")
	}
}

func TestBranchOrgUnit_ValidationErrors(t *testing.T) {
	parentMST := "0101243150"

	// Case 1: Branch base tax code does not match parent company MST
	mismatchedMST := system.CreateBranchOrgUnitParams{
		ID:                   "b-err-1",
		CompanyProfileID:     "cp-01",
		ParentCompanyTaxCode: parentMST,
		Code:                 "CN_ERR",
		Name:                 "Chi nhánh Lỗi MST",
		UnitType:             system.OrgUnitBranch,
		AccountingGovernance: system.GovDependent,
		TaxFilingMechanism:   system.TaxFilingCentralized,
		TaxCode:              "0101248141-001", // Different base MST
		ProvinceCityCode:     "01",
		Address:              "Hà Nội",
	}
	if _, err := system.NewBranchOrgUnit(mismatchedMST); err != system.ErrBranchTaxCodeMismatch {
		t.Errorf("expected ErrBranchTaxCodeMismatch, got %v", err)
	}

	// Case 2: Business location code is not 5 digits
	invalidLoc := system.CreateBranchOrgUnitParams{
		ID:                   "b-err-2",
		CompanyProfileID:     "cp-01",
		ParentCompanyTaxCode: parentMST,
		Code:                 "LOC_ERR",
		Name:                 "Địa điểm lỗi",
		UnitType:             system.OrgUnitBusinessLocation,
		AccountingGovernance: system.GovCostCenter,
		TaxFilingMechanism:   system.TaxFilingCentralized,
		TaxCode:              "123", // only 3 digits
		ProvinceCityCode:     "01",
		Address:              "Hà Nội",
	}
	if _, err := system.NewBranchOrgUnit(invalidLoc); err != system.ErrInvalidBusinessLocationCode {
		t.Errorf("expected ErrInvalidBusinessLocationCode, got %v", err)
	}

	// Case 3: Department or Business Location cannot have INDEPENDENT accounting governance
	invalidGov := system.CreateBranchOrgUnitParams{
		ID:                   "b-err-3",
		CompanyProfileID:     "cp-01",
		ParentCompanyTaxCode: parentMST,
		Code:                 "DEPT_ERR",
		Name:                 "Phòng ban độc lập lỗi",
		UnitType:             system.OrgUnitDepartment,
		AccountingGovernance: system.GovIndependent, // Invalid
		TaxFilingMechanism:   system.TaxFilingCentralized,
		ProvinceCityCode:     "01",
		Address:              "Hà Nội",
	}
	if _, err := system.NewBranchOrgUnit(invalidGov); err != system.ErrInvalidGovernanceForUnitType {
		t.Errorf("expected ErrInvalidGovernanceForUnitType, got %v", err)
	}

	// Case 4: Independent branch must be DECENTRALIZED tax filing
	indepBranchCentral := system.CreateBranchOrgUnitParams{
		ID:                   "b-err-4",
		CompanyProfileID:     "cp-01",
		ParentCompanyTaxCode: parentMST,
		Code:                 "CN_INDEP_ERR",
		Name:                 "Chi nhánh độc lập nhưng khai tập trung",
		UnitType:             system.OrgUnitBranch,
		AccountingGovernance: system.GovIndependent,
		TaxFilingMechanism:   system.TaxFilingCentralized, // Invalid
		TaxCode:              "0101243150-001",
		ProvinceCityCode:     "48",
		Address:              "Đà Nẵng",
	}
	if _, err := system.NewBranchOrgUnit(indepBranchCentral); err != system.ErrIndependentBranchMustBeDecentralized {
		t.Errorf("expected ErrIndependentBranchMustBeDecentralized, got %v", err)
	}
}

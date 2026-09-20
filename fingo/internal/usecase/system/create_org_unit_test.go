package system_test

import (
	"context"
	"errors"
	"testing"

	"fingo/internal/domain/system"
	usecase "fingo/internal/usecase/system"
)

type mockBranchOrgUnitRepo struct {
	units map[string]*system.BranchOrgUnit
}

func newMockBranchOrgUnitRepo() *mockBranchOrgUnitRepo {
	return &mockBranchOrgUnitRepo{units: make(map[string]*system.BranchOrgUnit)}
}

func (m *mockBranchOrgUnitRepo) CreateOrgUnit(ctx context.Context, u *system.BranchOrgUnit) error {
	m.units[u.ID] = u
	return nil
}

func (m *mockBranchOrgUnitRepo) GetOrgUnitByID(ctx context.Context, id string) (*system.BranchOrgUnit, error) {
	if u, ok := m.units[id]; ok {
		return u, nil
	}
	return nil, errors.New("not found")
}

func (m *mockBranchOrgUnitRepo) GetOrgUnitByCode(ctx context.Context, companyID, code string) (*system.BranchOrgUnit, error) {
	for _, u := range m.units {
		if u.CompanyProfileID == companyID && u.Code == code {
			return u, nil
		}
	}
	return nil, errors.New("not found")
}

func (m *mockBranchOrgUnitRepo) ListOrgUnitsByCompany(ctx context.Context, companyID string) ([]system.BranchOrgUnit, error) {
	var res []system.BranchOrgUnit
	for _, u := range m.units {
		if u.CompanyProfileID == companyID {
			res = append(res, *u)
		}
	}
	return res, nil
}

func (m *mockBranchOrgUnitRepo) UpdateOrgUnit(ctx context.Context, u *system.BranchOrgUnit) error {
	m.units[u.ID] = u
	return nil
}

func TestCreateBranchOrgUnitUseCase_Success(t *testing.T) {
	compRepo := &mockCompanyRepo{}
	parentMST := "0101243150"
	comp, _ := system.NewProductionCompanyProfile(system.CreateCompanyProfileParams{
		ID:                  "cp-01",
		TaxCode:             parentMST,
		LegalName:           "CÔNG TY TEST",
		Address:             "Hà Nội",
		LegalRepresentative: "Giám Đốc",
		ChiefAccountant:     "Kế Toán",
		TaxAuthorityCode:    "10500",
		TaxAuthorityName:    "Cầu Giấy",
		Regime:              system.RegimeCircular133,
		VATMethod:           system.VATMethodDeduction,
		CostingMethod:       system.CostingMovingWeighted,
		BusinessType:        system.BusinessTrading,
	})
	_ = compRepo.SaveProfile(context.Background(), comp)

	orgRepo := newMockBranchOrgUnitRepo()
	uc := usecase.NewCreateBranchOrgUnitUseCase(compRepo, orgRepo)

	cmd := usecase.CreateBranchOrgUnitCommand{
		CompanyProfileID:     "cp-01",
		Code:                 "CN_HCM",
		Name:                 "Chi nhánh TP HCM",
		UnitType:             "BRANCH",
		AccountingGovernance: "DEPENDENT",
		TaxFilingMechanism:   "ALLOCATED",
		TaxCode:              "0101243150-001",
		TaxAuthorityCode:     "79001",
		TaxAuthorityName:     "Chi cục Thuế Quận 1",
		ProvinceCityCode:     "79",
		Address:              "Quận 1, TP HCM",
	}

	res, err := uc.Execute(context.Background(), cmd)
	if err != nil {
		t.Fatalf("expected successful creation, got %v", err)
	}
	if res.Code != "CN_HCM" || res.TaxCode != "0101243150-001" {
		t.Errorf("unexpected created org unit: %+v", res)
	}
	if len(orgRepo.units) != 1 {
		t.Errorf("expected 1 unit in repo, got %d", len(orgRepo.units))
	}
}

func TestCreateBranchOrgUnitUseCase_TaxCodeMismatchFailure(t *testing.T) {
	compRepo := &mockCompanyRepo{}
	parentMST := "0101243150"
	comp, _ := system.NewProductionCompanyProfile(system.CreateCompanyProfileParams{
		ID:                  "cp-01",
		TaxCode:             parentMST,
		LegalName:           "CÔNG TY TEST",
		Address:             "Hà Nội",
		LegalRepresentative: "Giám Đốc",
		ChiefAccountant:     "Kế Toán",
		TaxAuthorityCode:    "10500",
		TaxAuthorityName:    "Cầu Giấy",
		Regime:              system.RegimeCircular133,
		VATMethod:           system.VATMethodDeduction,
		CostingMethod:       system.CostingMovingWeighted,
		BusinessType:        system.BusinessTrading,
	})
	_ = compRepo.SaveProfile(context.Background(), comp)

	orgRepo := newMockBranchOrgUnitRepo()
	uc := usecase.NewCreateBranchOrgUnitUseCase(compRepo, orgRepo)

	// Command with mismatched base MST
	cmd := usecase.CreateBranchOrgUnitCommand{
		CompanyProfileID:     "cp-01",
		Code:                 "CN_ERR",
		Name:                 "Chi nhánh Sai MST",
		UnitType:             "BRANCH",
		AccountingGovernance: "DEPENDENT",
		TaxFilingMechanism:   "CENTRALIZED",
		TaxCode:              "0101248141-001", // Mismatched base
		ProvinceCityCode:     "79",
		Address:              "TP HCM",
	}

	_, err := uc.Execute(context.Background(), cmd)
	if err == nil {
		t.Fatal("expected error for mismatched branch tax code")
	}
}

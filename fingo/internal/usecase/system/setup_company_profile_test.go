package system_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"fingo/internal/domain/system"
	usecase "fingo/internal/usecase/system"
)

type mockCompanyRepo struct {
	savedProfile *system.ProductionCompanyProfile
}

func (m *mockCompanyRepo) GetProfile(ctx context.Context) (*system.ProductionCompanyProfile, error) {
	if m.savedProfile == nil {
		return nil, errors.New("not found")
	}
	return m.savedProfile, nil
}

func (m *mockCompanyRepo) SaveProfile(ctx context.Context, p *system.ProductionCompanyProfile) error {
	m.savedProfile = p
	return nil
}

func (m *mockCompanyRepo) SetLockDate(ctx context.Context, id string, lockDate time.Time) error {
	if m.savedProfile != nil {
		m.savedProfile.SetLockDate(lockDate)
	}
	return nil
}

func TestSetupCompanyProfileUseCase_ExecuteSuccess(t *testing.T) {
	repo := &mockCompanyRepo{}
	uc := usecase.NewSetupCompanyProfileUseCase(repo)

	cmd := usecase.SetupCompanyProfileCommand{
		TaxCode:             "0101243150",
		LegalName:           "CÔNG TY TNHH PHẦN MỀM KẾ TOÁN",
		TradeName:           "FinGo Software",
		Address:             "Quận 1, TP Hồ Chí Minh",
		ProvinceCity:        "TP Hồ Chí Minh",
		LegalRepresentative: "Lê Văn Giám Đốc",
		ChiefAccountant:     "Phạm Thị Kế Toán Trưởng",
		TaxAuthorityCode:    "79001",
		TaxAuthorityName:    "Chi cục Thuế Quận 1",
		StateBudgetChapter:  "754",
		Regime:              "TT133_2016",
		VATMethod:           "DEDUCTION",
		CostingMethod:       "MOVING_WEIGHTED_AVG",
		BusinessType:        "SERVICE",
	}

	res, err := uc.Execute(context.Background(), cmd)
	if err != nil {
		t.Fatalf("expected successful execution, got %v", err)
	}

	if res.TaxCode != "0101243150" {
		t.Errorf("expected TaxCode 0101243150, got %s", res.TaxCode)
	}
	if repo.savedProfile == nil {
		t.Fatal("expected profile to be persisted in repo")
	}
}

func TestSetupCompanyProfileUseCase_ExecuteValidationFailure(t *testing.T) {
	repo := &mockCompanyRepo{}
	uc := usecase.NewSetupCompanyProfileUseCase(repo)

	// Command with invalid tax code checksum
	cmd := usecase.SetupCompanyProfileCommand{
		TaxCode:   "0101243159", // invalid
		LegalName: "Test Company",
	}

	_, err := uc.Execute(context.Background(), cmd)
	if err == nil {
		t.Fatal("expected validation error for invalid tax code")
	}
}

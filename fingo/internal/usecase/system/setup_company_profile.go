package system

import (
	"context"

	"github.com/google/uuid"

	"fingo/internal/domain/system"
)

type SetupCompanyProfileCommand struct {
	ID                     string `json:"id"`
	TaxCode                string `json:"tax_code"`
	LegalName              string `json:"legal_name"`
	TradeName              string `json:"trade_name"`
	EnglishName            string `json:"english_name"`
	Address                string `json:"address"`
	ProvinceCity           string `json:"province_city"`
	DistrictWard           string `json:"district_ward"`
	Phone                  string `json:"phone"`
	Email                  string `json:"email"`
	Website                string `json:"website"`
	LegalRepresentative    string `json:"legal_representative"`
	RepresentativePosition string `json:"representative_position"`
	ChiefAccountant        string `json:"chief_accountant"`
	TaxAuthorityCode       string `json:"tax_authority_code"`
	TaxAuthorityName       string `json:"tax_authority_name"`
	StateBudgetChapter     string `json:"state_budget_chapter"`
	Regime                 string `json:"regime"`
	VATMethod              string `json:"vat_method"`
	CostingMethod          string `json:"costing_method"`
	BusinessType           string `json:"business_type"`
}

type SetupCompanyProfileUseCase struct {
	repo system.CompanyProfileRepository
}

func NewSetupCompanyProfileUseCase(repo system.CompanyProfileRepository) *SetupCompanyProfileUseCase {
	return &SetupCompanyProfileUseCase{repo: repo}
}

func (u *SetupCompanyProfileUseCase) Execute(ctx context.Context, cmd SetupCompanyProfileCommand) (*system.ProductionCompanyProfile, error) {
	id := cmd.ID
	if id == "" {
		id = uuid.New().String()
	}

	profileParams := system.CreateCompanyProfileParams{
		ID:                     id,
		TaxCode:                cmd.TaxCode,
		LegalName:              cmd.LegalName,
		TradeName:              cmd.TradeName,
		EnglishName:            cmd.EnglishName,
		Address:                cmd.Address,
		ProvinceCity:           cmd.ProvinceCity,
		DistrictWard:           cmd.DistrictWard,
		Phone:                  cmd.Phone,
		Email:                  cmd.Email,
		Website:                cmd.Website,
		LegalRepresentative:    cmd.LegalRepresentative,
		RepresentativePosition: cmd.RepresentativePosition,
		ChiefAccountant:        cmd.ChiefAccountant,
		TaxAuthorityCode:       cmd.TaxAuthorityCode,
		TaxAuthorityName:       cmd.TaxAuthorityName,
		StateBudgetChapter:     cmd.StateBudgetChapter,
		Regime:                 system.AccountingRegime(cmd.Regime),
		VATMethod:              system.VATMethod(cmd.VATMethod),
		CostingMethod:          system.CostingMethod(cmd.CostingMethod),
		BusinessType:           system.BusinessType(cmd.BusinessType),
	}

	entity, err := system.NewProductionCompanyProfile(profileParams)
	if err != nil {
		return nil, err
	}

	if err := u.repo.SaveProfile(ctx, entity); err != nil {
		return nil, err
	}

	return entity, nil
}

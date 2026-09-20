package system

import (
	"context"
	"errors"

	"github.com/google/uuid"

	"fingo/internal/domain/system"
)

var (
	ErrDuplicateOrgUnitCode = errors.New("organizational unit code already exists for this company")
	ErrCompanyNotFound      = errors.New("company profile not found")
)

type CreateBranchOrgUnitCommand struct {
	ID                        string  `json:"id"`
	ParentID                  *string `json:"parent_id,omitempty"`
	CompanyProfileID          string  `json:"company_profile_id"`
	Code                      string  `json:"code"`
	Name                      string  `json:"name"`
	UnitType                  string  `json:"unit_type"`
	AccountingGovernance      string  `json:"accounting_governance"`
	TaxFilingMechanism        string  `json:"tax_filing_mechanism"`
	TaxCode                   string  `json:"tax_code"`
	TaxAuthorityCode          string  `json:"tax_authority_code"`
	TaxAuthorityName          string  `json:"tax_authority_name"`
	ProvinceCityCode          string  `json:"province_city_code"`
	Address                   string  `json:"address"`
	ManagerName               string  `json:"manager_name"`
	ChiefAccountant           string  `json:"chief_accountant"`
	InternalReceivableAccount string  `json:"internal_receivable_account"`
	InternalPayableAccount    string  `json:"internal_payable_account"`
	HasOwnEInvoice            bool    `json:"has_own_einvoice"`
}

type CreateBranchOrgUnitUseCase struct {
	companyRepo system.CompanyProfileRepository
	orgRepo     system.BranchOrgUnitRepository
}

func NewCreateBranchOrgUnitUseCase(
	companyRepo system.CompanyProfileRepository,
	orgRepo system.BranchOrgUnitRepository,
) *CreateBranchOrgUnitUseCase {
	return &CreateBranchOrgUnitUseCase{
		companyRepo: companyRepo,
		orgRepo:     orgRepo,
	}
}

func (u *CreateBranchOrgUnitUseCase) Execute(ctx context.Context, cmd CreateBranchOrgUnitCommand) (*system.BranchOrgUnit, error) {
	// 1. Fetch parent company to retrieve base MST
	comp, err := u.companyRepo.GetProfile(ctx)
	if err != nil {
		return nil, ErrCompanyNotFound
	}

	// 2. Check duplicate code
	existing, _ := u.orgRepo.GetOrgUnitByCode(ctx, cmd.CompanyProfileID, cmd.Code)
	if existing != nil {
		return nil, ErrDuplicateOrgUnitCode
	}

	id := cmd.ID
	if id == "" {
		id = uuid.New().String()
	}

	// 3. Construct and validate domain entity
	entity, err := system.NewBranchOrgUnit(system.CreateBranchOrgUnitParams{
		ID:                        id,
		ParentID:                  cmd.ParentID,
		CompanyProfileID:          cmd.CompanyProfileID,
		ParentCompanyTaxCode:      comp.TaxCode,
		Code:                      cmd.Code,
		Name:                      cmd.Name,
		UnitType:                  system.OrgUnitType(cmd.UnitType),
		AccountingGovernance:      system.AccountingGovernance(cmd.AccountingGovernance),
		TaxFilingMechanism:        system.TaxFilingMechanism(cmd.TaxFilingMechanism),
		TaxCode:                   cmd.TaxCode,
		TaxAuthorityCode:          cmd.TaxAuthorityCode,
		TaxAuthorityName:          cmd.TaxAuthorityName,
		ProvinceCityCode:          cmd.ProvinceCityCode,
		Address:                   cmd.Address,
		ManagerName:               cmd.ManagerName,
		ChiefAccountant:           cmd.ChiefAccountant,
		InternalReceivableAccount: cmd.InternalReceivableAccount,
		InternalPayableAccount:    cmd.InternalPayableAccount,
		HasOwnEInvoice:            cmd.HasOwnEInvoice,
	})
	if err != nil {
		return nil, err
	}

	// 4. Save to repository
	if err := u.orgRepo.CreateOrgUnit(ctx, entity); err != nil {
		return nil, err
	}

	return entity, nil
}

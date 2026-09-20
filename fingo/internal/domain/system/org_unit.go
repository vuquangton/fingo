package system

import (
	"errors"
	"strings"
	"time"
	"unicode"
)

type OrgUnitType string

const (
	OrgUnitHeadOffice       OrgUnitType = "HEAD_OFFICE"
	OrgUnitBranch           OrgUnitType = "BRANCH"
	OrgUnitRepOffice        OrgUnitType = "REP_OFFICE"
	OrgUnitBusinessLocation OrgUnitType = "BUSINESS_LOCATION"
	OrgUnitDepartment       OrgUnitType = "DEPARTMENT"
)

type AccountingGovernance string

const (
	GovIndependent AccountingGovernance = "INDEPENDENT"
	GovDependent   AccountingGovernance = "DEPENDENT"
	GovCostCenter  AccountingGovernance = "COST_CENTER"
)

type TaxFilingMechanism string

const (
	TaxFilingCentralized   TaxFilingMechanism = "CENTRALIZED"
	TaxFilingDecentralized TaxFilingMechanism = "DECENTRALIZED"
	TaxFilingAllocated     TaxFilingMechanism = "ALLOCATED"
)

var (
	ErrEmptyOrgUnitCode                     = errors.New("organizational unit code cannot be empty")
	ErrEmptyOrgUnitName                     = errors.New("organizational unit name cannot be empty")
	ErrBranchTaxCodeMismatch                = errors.New("branch tax code base does not match parent company tax code")
	ErrInvalidBranchTaxCode                 = errors.New("branch tax code must be 14 characters formatted as XXXXXXXXXX-YYY")
	ErrInvalidBusinessLocationCode          = errors.New("business location code must be exactly 5 numeric digits (00001-99999)")
	ErrInvalidGovernanceForUnitType         = errors.New("independent accounting governance is only allowed for branches")
	ErrIndependentBranchMustBeDecentralized = errors.New("independent branches must use decentralized tax filing per Circular 80/2021/TT-BTC")
)

type CreateBranchOrgUnitParams struct {
	ID                        string
	ParentID                  *string
	CompanyProfileID          string
	ParentCompanyTaxCode      string
	Code                      string
	Name                      string
	UnitType                  OrgUnitType
	AccountingGovernance      AccountingGovernance
	TaxFilingMechanism        TaxFilingMechanism
	TaxCode                   string
	TaxAuthorityCode          string
	TaxAuthorityName          string
	ProvinceCityCode          string
	Address                   string
	ManagerName               string
	ChiefAccountant           string
	InternalReceivableAccount string
	InternalPayableAccount    string
	HasOwnEInvoice            bool
}

type BranchOrgUnit struct {
	ID                        string
	ParentID                  *string
	CompanyProfileID          string
	Code                      string
	Name                      string
	UnitType                  OrgUnitType
	AccountingGovernance      AccountingGovernance
	TaxFilingMechanism        TaxFilingMechanism
	TaxCode                   string
	TaxAuthorityCode          string
	TaxAuthorityName          string
	ProvinceCityCode          string
	Address                   string
	ManagerName               string
	ChiefAccountant           string
	InternalReceivableAccount string
	InternalPayableAccount    string
	HasOwnEInvoice            bool
	IsActive                  bool
	CreatedAt                 time.Time
	UpdatedAt                 time.Time
}

// NewBranchOrgUnit constructs and validates an organizational unit according to Vietnamese regulations
func NewBranchOrgUnit(p CreateBranchOrgUnitParams) (*BranchOrgUnit, error) {
	code := strings.TrimSpace(p.Code)
	if code == "" {
		return nil, ErrEmptyOrgUnitCode
	}

	name := strings.TrimSpace(p.Name)
	if name == "" {
		return nil, ErrEmptyOrgUnitName
	}

	taxCode := strings.TrimSpace(p.TaxCode)

	switch p.UnitType {
	case OrgUnitBranch, OrgUnitRepOffice:
		if err := ValidateTaxCode(taxCode); err != nil {
			return nil, err
		}
		if len(taxCode) != 14 {
			return nil, ErrInvalidBranchTaxCode
		}
		cleanParentMST := strings.TrimSpace(p.ParentCompanyTaxCode)
		if cleanParentMST != "" && !strings.HasPrefix(taxCode, cleanParentMST+"-") {
			return nil, ErrBranchTaxCodeMismatch
		}

	case OrgUnitBusinessLocation:
		if len(taxCode) != 5 {
			return nil, ErrInvalidBusinessLocationCode
		}
		for _, r := range taxCode {
			if !unicode.IsDigit(r) {
				return nil, ErrInvalidBusinessLocationCode
			}
		}

	case OrgUnitDepartment, OrgUnitHeadOffice:
		// Tax code is optional for internal departments
	}

	// Invariant BR-ORG-02: Governance constraints
	if p.AccountingGovernance == GovIndependent {
		if p.UnitType != OrgUnitBranch {
			return nil, ErrInvalidGovernanceForUnitType
		}
		if p.TaxFilingMechanism != TaxFilingDecentralized {
			return nil, ErrIndependentBranchMustBeDecentralized
		}
	}

	recAcc := strings.TrimSpace(p.InternalReceivableAccount)
	if recAcc == "" && p.AccountingGovernance == GovDependent {
		recAcc = "1361"
	}

	payAcc := strings.TrimSpace(p.InternalPayableAccount)
	if payAcc == "" && p.AccountingGovernance == GovDependent {
		payAcc = "3361"
	}

	now := time.Now()

	return &BranchOrgUnit{
		ID:                        p.ID,
		ParentID:                  p.ParentID,
		CompanyProfileID:          p.CompanyProfileID,
		Code:                      code,
		Name:                      name,
		UnitType:                  p.UnitType,
		AccountingGovernance:      p.AccountingGovernance,
		TaxFilingMechanism:        p.TaxFilingMechanism,
		TaxCode:                   taxCode,
		TaxAuthorityCode:          strings.TrimSpace(p.TaxAuthorityCode),
		TaxAuthorityName:          strings.TrimSpace(p.TaxAuthorityName),
		ProvinceCityCode:          strings.TrimSpace(p.ProvinceCityCode),
		Address:                   strings.TrimSpace(p.Address),
		ManagerName:               strings.TrimSpace(p.ManagerName),
		ChiefAccountant:           strings.TrimSpace(p.ChiefAccountant),
		InternalReceivableAccount: recAcc,
		InternalPayableAccount:    payAcc,
		HasOwnEInvoice:            p.HasOwnEInvoice,
		IsActive:                  true,
		CreatedAt:                 now,
		UpdatedAt:                 now,
	}, nil
}

package catalog

import (
	"fmt"
	"strings"
	"time"

	"github.com/shopspring/decimal"
)

// Employee represents a worker or staff member (Nhân viên)
type Employee struct {
	ID                 string          `json:"id"`
	CompanyProfileID   string          `json:"company_profile_id"`
	BranchID           *string         `json:"branch_id,omitempty"`
	Code               string          `json:"code"`
	FullName           string          `json:"full_name"`
	Department         string          `json:"department"`
	Position           string          `json:"position"`
	CitizenID          string          `json:"citizen_id"`          // CCCD (12 digits)
	TaxCode            string          `json:"tax_code,omitempty"`  // MST cá nhân (10 digits)
	SocialInsuranceNo  string          `json:"social_insurance_no"` // Mã số BHXH (10 digits)
	BaseSalary         decimal.Decimal `json:"base_salary"`
	SalaryCoefficient  decimal.Decimal `json:"salary_coefficient"`
	BankAccountNumber  string          `json:"bank_account_number,omitempty"`
	BankName           string          `json:"bank_name,omitempty"`
	DefaultAdvanceAcc  string          `json:"default_advance_acc"` // FK to accounts (141)
	DefaultPayrollAcc  string          `json:"default_payroll_acc"` // FK to accounts (334)
	IsActive           bool            `json:"is_active"`
	CreatedAt          time.Time       `json:"created_at"`
	UpdatedAt          time.Time       `json:"updated_at"`
}

// ValidateCitizenID verifies CCCD has exactly 12 numeric digits
func ValidateCitizenID(cccd string) bool {
	cccd = strings.TrimSpace(cccd)
	if len(cccd) != 12 {
		return false
	}
	for i := 0; i < 12; i++ {
		if cccd[i] < '0' || cccd[i] > '9' {
			return false
		}
	}
	return true
}

// ValidateSocialInsuranceNo verifies BHXH has exactly 10 numeric digits
func ValidateSocialInsuranceNo(bhxh string) bool {
	bhxh = strings.TrimSpace(bhxh)
	if len(bhxh) != 10 {
		return false
	}
	for i := 0; i < 10; i++ {
		if bhxh[i] < '0' || bhxh[i] > '9' {
			return false
		}
	}
	return true
}

func NewEmployee(
	id, companyID string,
	branchID *string,
	code, fullName, department, position string,
	citizenID, taxCode, socialInsuranceNo string,
	baseSalary, salaryCoefficient decimal.Decimal,
	bankAccountNumber, bankName string,
	defaultAdvanceAcc, defaultPayrollAcc string,
) (*Employee, error) {
	code = strings.ToUpper(strings.TrimSpace(code))
	if len(code) < 2 || len(code) > 50 {
		return nil, fmt.Errorf("%w: '%s'", ErrInvalidCode, code)
	}
	fullName = strings.TrimSpace(fullName)
	if fullName == "" {
		return nil, fmt.Errorf("employee full name cannot be empty")
	}

	citizenID = strings.TrimSpace(citizenID)
	if !ValidateCitizenID(citizenID) {
		return nil, fmt.Errorf("%w: '%s'", ErrInvalidCitizenID, citizenID)
	}

	socialInsuranceNo = strings.TrimSpace(socialInsuranceNo)
	if !ValidateSocialInsuranceNo(socialInsuranceNo) {
		return nil, fmt.Errorf("%w: '%s'", ErrInvalidSocialInsuranceNo, socialInsuranceNo)
	}

	taxCode = strings.TrimSpace(taxCode)
	if taxCode != "" && len(taxCode) != 10 {
		return nil, fmt.Errorf("employee personal tax code must be 10 digits: '%s'", taxCode)
	}

	if defaultAdvanceAcc == "" {
		return nil, fmt.Errorf("%w: default advance account (TK 141) required", ErrMissingAccount)
	}
	if defaultPayrollAcc == "" {
		return nil, fmt.Errorf("%w: default payroll account (TK 334) required", ErrMissingAccount)
	}

	if baseSalary.IsNegative() {
		baseSalary = decimal.Zero
	}
	if salaryCoefficient.LessThanOrEqual(decimal.Zero) {
		salaryCoefficient = decimal.NewFromInt(1)
	}

	return &Employee{
		ID:                id,
		CompanyProfileID:  companyID,
		BranchID:          branchID,
		Code:              code,
		FullName:          fullName,
		Department:        strings.TrimSpace(department),
		Position:          strings.TrimSpace(position),
		CitizenID:         citizenID,
		TaxCode:           taxCode,
		SocialInsuranceNo: socialInsuranceNo,
		BaseSalary:        baseSalary,
		SalaryCoefficient: salaryCoefficient,
		BankAccountNumber: strings.TrimSpace(bankAccountNumber),
		BankName:          strings.TrimSpace(bankName),
		DefaultAdvanceAcc: defaultAdvanceAcc,
		DefaultPayrollAcc: defaultPayrollAcc,
		IsActive:          true,
		CreatedAt:         time.Now(),
		UpdatedAt:         time.Now(),
	}, nil
}

// MaskCitizenID returns masked CCCD for PDPL Law 91/2025/QH15 compliance in logs and non-sensitive views
func (e *Employee) MaskCitizenID() string {
	if len(e.CitizenID) != 12 {
		return "************"
	}
	return e.CitizenID[:3] + "******" + e.CitizenID[9:]
}

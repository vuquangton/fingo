package catalog

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/shopspring/decimal"
)

// ValidateTaxCode validates Vietnamese Tax Code (MST) per Circular 105/2020/TT-BTC
// Formats:
// - 10 digits: Primary enterprise / headquarter
// - 13 digits (or 10 digits + '-' + 3 digits): Dependent branch / unit (suffix 001-999)
func ValidateTaxCode(mst string) bool {
	clean := strings.ReplaceAll(strings.TrimSpace(mst), "-", "")
	clean = strings.ReplaceAll(clean, " ", "")

	if len(clean) != 10 && len(clean) != 13 {
		return false
	}

	// First 10 characters must be digits
	for i := 0; i < 10; i++ {
		if clean[i] < '0' || clean[i] > '9' {
			return false
		}
	}

	// Circular 105 Modulo-11 weights
	weights := [9]int{31, 29, 23, 19, 17, 13, 7, 5, 3}
	sum := 0
	for i := 0; i < 9; i++ {
		digit := int(clean[i] - '0')
		sum += digit * weights[i]
	}

	rem := sum % 11
	expectedCheckDigit := 10 - rem
	if expectedCheckDigit == 10 {
		expectedCheckDigit = 0
	}

	actualCheckDigit := int(clean[9] - '0')
	if actualCheckDigit != expectedCheckDigit {
		return false
	}

	// If 13 digits, check branch suffix (001-999)
	if len(clean) == 13 {
		suffix := clean[10:13]
		branchNum, err := strconv.Atoi(suffix)
		if err != nil || branchNum < 1 || branchNum > 999 {
			return false
		}
	}

	return true
}

// Customer represents a buyer or client (Khách hàng)
type Customer struct {
	ID                 string          `json:"id"`
	CompanyProfileID   string          `json:"company_profile_id"`
	Code               string          `json:"code"`
	Name               string          `json:"name"`
	TaxCode            string          `json:"tax_code,omitempty"`
	Address            string          `json:"address,omitempty"`
	Phone              string          `json:"phone,omitempty"`
	Email              string          `json:"email,omitempty"`
	ContactPerson      string          `json:"contact_person,omitempty"`
	PaymentTermDays    int             `json:"payment_term_days"`
	CreditLimit        decimal.Decimal `json:"credit_limit"`
	EnforceCreditLimit bool            `json:"enforce_credit_limit"`
	DefaultARAccountID string          `json:"default_ar_account_id"` // FK to accounts (e.g. 1311)
	DefaultARAccountCode string        `json:"default_ar_account_code"`
	IsActive           bool            `json:"is_active"`
	CreatedAt          time.Time       `json:"created_at"`
	UpdatedAt          time.Time       `json:"updated_at"`
}

func NewCustomer(
	id, companyID, code, name, taxCode string,
	address, phone, email, contactPerson string,
	paymentTermDays int,
	creditLimit decimal.Decimal,
	enforceCreditLimit bool,
	defaultARAccountID, defaultARAccountCode string,
) (*Customer, error) {
	code = strings.ToUpper(strings.TrimSpace(code))
	if len(code) < 2 || len(code) > 50 {
		return nil, fmt.Errorf("%w: '%s'", ErrInvalidCode, code)
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, fmt.Errorf("customer name cannot be empty")
	}

	taxCode = strings.TrimSpace(taxCode)
	if taxCode != "" && !ValidateTaxCode(taxCode) {
		return nil, fmt.Errorf("%w: '%s'", ErrInvalidTaxCode, taxCode)
	}

	if defaultARAccountID == "" {
		return nil, fmt.Errorf("%w: default AR account ID (TK 131) is required", ErrMissingAccount)
	}

	if creditLimit.IsNegative() {
		creditLimit = decimal.Zero
	}
	if paymentTermDays < 0 {
		paymentTermDays = 0
	}

	return &Customer{
		ID:                   id,
		CompanyProfileID:     companyID,
		Code:                 code,
		Name:                 name,
		TaxCode:              taxCode,
		Address:              address,
		Phone:                phone,
		Email:                email,
		ContactPerson:        contactPerson,
		PaymentTermDays:      paymentTermDays,
		CreditLimit:          creditLimit,
		EnforceCreditLimit:   enforceCreditLimit,
		DefaultARAccountID:   defaultARAccountID,
		DefaultARAccountCode: defaultARAccountCode,
		IsActive:             true,
		CreatedAt:            time.Now(),
		UpdatedAt:            time.Now(),
	}, nil
}

// CheckCreditLimit evaluates INV-CAT-05: verifies if new transaction pushes balance above limit
func (c *Customer) CheckCreditLimit(currentOutstandingAR, newVoucherAmount decimal.Decimal) error {
	if !c.EnforceCreditLimit || c.CreditLimit.IsZero() {
		return nil
	}

	projectedAR := currentOutstandingAR.Add(newVoucherAmount)
	if projectedAR.GreaterThan(c.CreditLimit) {
		return fmt.Errorf("%w: projected AR %s exceeds credit limit %s (customer %s)",
			ErrCreditLimitExceeded, projectedAR.String(), c.CreditLimit.String(), c.Code)
	}
	return nil
}

// Vendor represents a supplier or contractor (Nhà cung cấp)
type Vendor struct {
	ID                 string    `json:"id"`
	CompanyProfileID   string    `json:"company_profile_id"`
	Code               string    `json:"code"`
	Name               string    `json:"name"`
	TaxCode            string    `json:"tax_code,omitempty"`
	Address            string    `json:"address,omitempty"`
	Phone              string    `json:"phone,omitempty"`
	Email              string    `json:"email,omitempty"`
	ContactPerson      string    `json:"contact_person,omitempty"`
	BankAccountNumber  string    `json:"bank_account_number,omitempty"`
	BankName           string    `json:"bank_name,omitempty"`
	BankBranch         string    `json:"bank_branch,omitempty"`
	PaymentTermDays    int       `json:"payment_term_days"`
	DefaultAPAccountID string    `json:"default_ap_account_id"` // FK to accounts (e.g. 3311)
	DefaultAPAccountCode string  `json:"default_ap_account_code"`
	IsActive           bool      `json:"is_active"`
	CreatedAt          time.Time `json:"created_at"`
	UpdatedAt          time.Time `json:"updated_at"`
}

func NewVendor(
	id, companyID, code, name, taxCode string,
	address, phone, email, contactPerson string,
	bankAccountNumber, bankName, bankBranch string,
	paymentTermDays int,
	defaultAPAccountID, defaultAPAccountCode string,
) (*Vendor, error) {
	code = strings.ToUpper(strings.TrimSpace(code))
	if len(code) < 2 || len(code) > 50 {
		return nil, fmt.Errorf("%w: '%s'", ErrInvalidCode, code)
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, fmt.Errorf("vendor name cannot be empty")
	}

	taxCode = strings.TrimSpace(taxCode)
	if taxCode != "" && !ValidateTaxCode(taxCode) {
		return nil, fmt.Errorf("%w: '%s'", ErrInvalidTaxCode, taxCode)
	}

	if defaultAPAccountID == "" {
		return nil, fmt.Errorf("%w: default AP account ID (TK 331) is required", ErrMissingAccount)
	}

	if paymentTermDays < 0 {
		paymentTermDays = 0
	}

	return &Vendor{
		ID:                   id,
		CompanyProfileID:     companyID,
		Code:                 code,
		Name:                 name,
		TaxCode:              taxCode,
		Address:              address,
		Phone:                phone,
		Email:                email,
		ContactPerson:        contactPerson,
		BankAccountNumber:    strings.TrimSpace(bankAccountNumber),
		BankName:             strings.TrimSpace(bankName),
		BankBranch:           strings.TrimSpace(bankBranch),
		PaymentTermDays:      paymentTermDays,
		DefaultAPAccountID:   defaultAPAccountID,
		DefaultAPAccountCode: defaultAPAccountCode,
		IsActive:             true,
		CreatedAt:            time.Now(),
		UpdatedAt:            time.Now(),
	}, nil
}

// HasValidBankAccountForNonCash checks if vendor maintains bank info for Decree 181/2025/NĐ-CP (>= 5M VND)
func (v *Vendor) HasValidBankAccountForNonCash() bool {
	return v.BankAccountNumber != "" && v.BankName != ""
}

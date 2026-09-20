package catalog

import (
	"fmt"
	"strings"
	"time"
)

// BankAccount represents a company bank account (Tài khoản tiền gửi ngân hàng)
type BankAccount struct {
	ID               string    `json:"id"`
	CompanyProfileID string    `json:"company_profile_id"`
	BranchID         *string   `json:"branch_id,omitempty"`
	AccountNumber    string    `json:"account_number"`
	BankName         string    `json:"bank_name"`
	BankCode         string    `json:"bank_code"`
	BranchName       string    `json:"branch_name,omitempty"`
	CurrencyCode     string    `json:"currency_code"` // "VND", "USD", etc.
	GLAccountID      string    `json:"gl_account_id"` // FK to accounts
	GLAccountCode    string    `json:"gl_account_code"` // e.g. "11211", "11221"
	IsActive         bool      `json:"is_active"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

// ValidateCurrencyGLAlignment enforces INV-CAT-06: VND must use 1121, foreign currencies must use 1122
func ValidateCurrencyGLAlignment(currencyCode, glAccountCode string) error {
	curr := strings.ToUpper(strings.TrimSpace(currencyCode))
	acc := strings.TrimSpace(glAccountCode)

	if curr == "VND" {
		if !strings.HasPrefix(acc, "1121") {
			return fmt.Errorf("%w: VND account must use 1121 subledger, got '%s'", ErrInvalidCurrencyGLAlignment, acc)
		}
	} else {
		// Foreign currency
		if !strings.HasPrefix(acc, "1122") {
			return fmt.Errorf("%w: foreign currency '%s' must use 1122 subledger, got '%s'", ErrInvalidCurrencyGLAlignment, curr, acc)
		}
	}
	return nil
}

func NewBankAccount(
	id, companyID string,
	branchID *string,
	accountNumber, bankName, bankCode, branchName, currencyCode string,
	glAccountID, glAccountCode string,
) (*BankAccount, error) {
	accountNumber = strings.TrimSpace(accountNumber)
	if len(accountNumber) < 3 || len(accountNumber) > 50 {
		return nil, fmt.Errorf("account number must be between 3 and 50 characters: '%s'", accountNumber)
	}

	bankName = strings.TrimSpace(bankName)
	if bankName == "" {
		return nil, fmt.Errorf("bank name cannot be empty")
	}

	currencyCode = strings.ToUpper(strings.TrimSpace(currencyCode))
	if len(currencyCode) != 3 {
		return nil, fmt.Errorf("currency code must be 3 characters: '%s'", currencyCode)
	}

	if glAccountID == "" {
		return nil, fmt.Errorf("%w: GL account ID is required", ErrMissingAccount)
	}

	if glAccountCode != "" {
		if err := ValidateCurrencyGLAlignment(currencyCode, glAccountCode); err != nil {
			return nil, err
		}
	}

	return &BankAccount{
		ID:               id,
		CompanyProfileID: companyID,
		BranchID:         branchID,
		AccountNumber:    accountNumber,
		BankName:         bankName,
		BankCode:         strings.ToUpper(strings.TrimSpace(bankCode)),
		BranchName:       strings.TrimSpace(branchName),
		CurrencyCode:     currencyCode,
		GLAccountID:      glAccountID,
		GLAccountCode:    glAccountCode,
		IsActive:         true,
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
	}, nil
}

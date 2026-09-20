package spine

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/shopspring/decimal"
)

var (
	ErrPostingToParentAccount   = errors.New("cannot post journal entry to parent account; postings strictly require leaf accounts")
	ErrInvalidAccountCode       = errors.New("account code must contain between 3 and 10 alphanumeric characters")
	ErrCircularAccountHierarchy = errors.New("circular hierarchy detected: account cannot be its own parent")
	ErrInactiveAccount          = errors.New("cannot post to inactive account")
)

type AccountNature string

const (
	NatureDebit         AccountNature = "DEBIT"
	NatureCredit        AccountNature = "CREDIT"
	NatureHermaphrodite AccountNature = "HERMAPHRODITE" // Lưỡng tính: 131, 331, 1388, 3388, 333, 421
	NatureNoBalance     AccountNature = "NO_BALANCE"     // Loại 5, 6, 7, 8, 9 (Doanh thu, chi phí, xác định KQKD)
)

type AccountCategory string

const (
	CategoryAsset        AccountCategory = "ASSET"
	CategoryLiability    AccountCategory = "LIABILITY"
	CategoryEquity       AccountCategory = "EQUITY"
	CategoryRevenue      AccountCategory = "REVENUE"
	CategoryExpense      AccountCategory = "EXPENSE"
	CategoryOtherIncome  AccountCategory = "OTHER_INCOME"
	CategoryOtherExpense AccountCategory = "OTHER_EXPENSE"
	CategorySummary      AccountCategory = "SUMMARY"
)

// Account represents a standardized General Ledger account conforming to Circular 99/2025/TT-BTC
type Account struct {
	ID                  string          `json:"id"`
	CompanyProfileID    string          `json:"company_profile_id"`
	Code                string          `json:"code"`
	Name                string          `json:"name"`
	EnglishName         string          `json:"english_name,omitempty"`
	ParentID            *string         `json:"parent_id,omitempty"`
	AccountLevel        int             `json:"account_level"`
	Nature              AccountNature   `json:"nature"`
	Category            AccountCategory `json:"category"`
	IsLeaf              bool            `json:"is_leaf"`
	IsForeignCurrency   bool            `json:"is_foreign_currency"`
	RequiresPartner     bool            `json:"requires_partner"`
	RequiresBankAccount bool            `json:"requires_bank_account"`
	RequiresCostCenter  bool            `json:"requires_cost_center"`
	RequiresExpenseItem bool            `json:"requires_expense_item"`
	IsActive            bool            `json:"is_active"`
	CreatedAt           time.Time       `json:"created_at"`
	UpdatedAt           time.Time       `json:"updated_at"`
}

func NewAccount(
	id, companyID, code, name string,
	parentID *string,
	level int,
	nature AccountNature,
	category AccountCategory,
	isLeaf bool,
) (*Account, error) {
	code = strings.TrimSpace(code)
	if len(code) < 3 || len(code) > 10 {
		return nil, fmt.Errorf("%w: '%s'", ErrInvalidAccountCode, code)
	}
	if parentID != nil && *parentID == id {
		return nil, ErrCircularAccountHierarchy
	}
	if level < 1 {
		level = 1
	}

	return &Account{
		ID:               id,
		CompanyProfileID: companyID,
		Code:             code,
		Name:             name,
		ParentID:         parentID,
		AccountLevel:     level,
		Nature:           nature,
		Category:         category,
		IsLeaf:           isLeaf,
		IsActive:         true,
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
	}, nil
}

// ValidatePostingAccount enforces INV-SPINE-01: strictly leaf accounts allowed for journal posting
func ValidatePostingAccount(acc *Account) error {
	if acc == nil {
		return errors.New("account cannot be nil")
	}
	if !acc.IsActive {
		return fmt.Errorf("%w: account %s (%s)", ErrInactiveAccount, acc.Code, acc.Name)
	}
	if !acc.IsLeaf {
		return fmt.Errorf("%w: account %s is a parent account with sub-accounts", ErrPostingToParentAccount, acc.Code)
	}
	return nil
}

// SeparateDualBalances implements INV-SPINE-02: decomposes counterparty sub-ledger balances
// into independent Debit (Asset) and Credit (Liability) aggregates without illegal netting.
func SeparateDualBalances(partnerBalances map[string]decimal.Decimal) (debitSum, creditSum decimal.Decimal) {
	debitSum = decimal.Zero
	creditSum = decimal.Zero

	for _, bal := range partnerBalances {
		if bal.IsPositive() {
			debitSum = debitSum.Add(bal)
		} else if bal.IsNegative() {
			creditSum = creditSum.Add(bal.Abs())
		}
	}
	return debitSum, creditSum
}

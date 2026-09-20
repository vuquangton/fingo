package spine

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

var (
	ErrMissingExpenseItem  = errors.New("expense item is mandatory for this account")
	ErrMissingCostCenter   = errors.New("cost center is mandatory for this account")
	ErrInvalidCode         = errors.New("code must be between 2 and 50 characters")
	ErrCircularParentChain = errors.New("cannot set parent to self")
)

type ExpenseCategory string

const (
	ExpenseCategoryLabor        ExpenseCategory = "LABOR"
	ExpenseCategoryMaterial     ExpenseCategory = "MATERIAL"
	ExpenseCategoryDepreciation ExpenseCategory = "DEPRECIATION"
	ExpenseCategoryOutsourced   ExpenseCategory = "OUTSOURCED"
	ExpenseCategoryOther        ExpenseCategory = "OTHER"
)

// CostCenter represents an organizational or managerial cost-collecting entity
type CostCenter struct {
	ID               string    `json:"id"`
	CompanyProfileID string    `json:"company_profile_id"`
	BranchID         *string   `json:"branch_id,omitempty"`
	Code             string    `json:"code"`
	Name             string    `json:"name"`
	ParentID         *string   `json:"parent_id,omitempty"`
	IsLeaf           bool      `json:"is_leaf"`
	IsActive         bool      `json:"is_active"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

func validateDimensionCodeAndParent(id, code string, parentID *string) (string, error) {
	code = strings.TrimSpace(code)
	if len(code) < 2 || len(code) > 50 {
		return "", fmt.Errorf("%w: '%s'", ErrInvalidCode, code)
	}
	if parentID != nil && *parentID == id {
		return "", ErrCircularParentChain
	}
	return code, nil
}

func NewCostCenter(id, companyID string, branchID *string, code, name string, parentID *string, isLeaf bool) (*CostCenter, error) {
	validCode, err := validateDimensionCodeAndParent(id, code, parentID)
	if err != nil {
		return nil, err
	}

	return &CostCenter{
		ID:               id,
		CompanyProfileID: companyID,
		BranchID:         branchID,
		Code:             validCode,
		Name:             name,
		ParentID:         parentID,
		IsLeaf:           isLeaf,
		IsActive:         true,
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
	}, nil
}

// ExpenseItem represents a statutory or managerial expense breakdown category
type ExpenseItem struct {
	ID               string          `json:"id"`
	CompanyProfileID string          `json:"company_profile_id"`
	Code             string          `json:"code"`
	Name             string          `json:"name"`
	Category         ExpenseCategory `json:"category"`
	ParentID         *string         `json:"parent_id,omitempty"`
	IsLeaf           bool            `json:"is_leaf"`
	IsActive         bool            `json:"is_active"`
	CreatedAt        time.Time       `json:"created_at"`
	UpdatedAt        time.Time       `json:"updated_at"`
}

func NewExpenseItem(id, companyID, code, name string, category ExpenseCategory, parentID *string, isLeaf bool) (*ExpenseItem, error) {
	validCode, err := validateDimensionCodeAndParent(id, code, parentID)
	if err != nil {
		return nil, err
	}

	return &ExpenseItem{
		ID:               id,
		CompanyProfileID: companyID,
		Code:             validCode,
		Name:             name,
		Category:         category,
		ParentID:         parentID,
		IsLeaf:           isLeaf,
		IsActive:         true,
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
	}, nil
}

// IsStatutoryNominalExpenseAccount checks if an account code requires mandatory ExpenseItem under INV-SPINE-07
func IsStatutoryNominalExpenseAccount(accCode string) bool {
	prefixes := []string{"154", "621", "622", "627", "641", "642", "635", "811"}
	for _, p := range prefixes {
		if strings.HasPrefix(accCode, p) {
			return true
		}
	}
	return false
}

// ValidateLineAllocation enforces INV-SPINE-07: checks mandatory CostCenter and ExpenseItem dimensions
func ValidateLineAllocation(acc *Account, costCenterID, expenseItemID *string) error {
	if acc == nil {
		return errors.New("account cannot be nil")
	}

	// INV-SPINE-07: Mandatory for nominal expense accounts (154, 621, 622, 627, 641, 642, 635, 811) or flagged
	if acc.RequiresExpenseItem || IsStatutoryNominalExpenseAccount(acc.Code) {
		if expenseItemID == nil || strings.TrimSpace(*expenseItemID) == "" {
			return fmt.Errorf("%w: account %s (%s)", ErrMissingExpenseItem, acc.Code, acc.Name)
		}
	}

	if acc.RequiresCostCenter {
		if costCenterID == nil || strings.TrimSpace(*costCenterID) == "" {
			return fmt.Errorf("%w: account %s (%s)", ErrMissingCostCenter, acc.Code, acc.Name)
		}
	}

	return nil
}


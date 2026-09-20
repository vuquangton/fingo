package catalog

import (
	"fmt"
	"strings"
	"time"
)

// Warehouse represents a physical storage location (Kho hàng)
type Warehouse struct {
	ID               string    `json:"id"`
	CompanyProfileID string    `json:"company_profile_id"`
	BranchID         *string   `json:"branch_id,omitempty"`
	Code             string    `json:"code"`
	Name             string    `json:"name"`
	Address          string    `json:"address,omitempty"`
	DefaultAccountID string    `json:"default_account_id"` // FK to accounts (e.g. 1561, 152)
	DefaultAccountCode string  `json:"default_account_code"` // e.g. "1561"
	IsActive         bool      `json:"is_active"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

// IsValidWarehouseAssetAccount enforces INV-CAT-07: checks if account belongs to inventory asset group 15
func IsValidWarehouseAssetAccount(accountCode string) bool {
	validPrefixes := []string{"151", "152", "153", "155", "156", "157", "158"}
	for _, p := range validPrefixes {
		if strings.HasPrefix(accountCode, p) {
			return true
		}
	}
	return false
}

func NewWarehouse(
	id, companyID string,
	branchID *string,
	code, name, address string,
	defaultAccountID, defaultAccountCode string,
) (*Warehouse, error) {
	code = strings.ToUpper(strings.TrimSpace(code))
	if len(code) < 2 || len(code) > 50 {
		return nil, fmt.Errorf("%w: '%s'", ErrInvalidCode, code)
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, fmt.Errorf("warehouse name cannot be empty")
	}

	if defaultAccountID == "" {
		return nil, fmt.Errorf("%w: default inventory account ID required", ErrMissingAccount)
	}

	if defaultAccountCode != "" && !IsValidWarehouseAssetAccount(defaultAccountCode) {
		return nil, fmt.Errorf("%w: '%s'", ErrIncompatibleWarehouseAccount, defaultAccountCode)
	}

	return &Warehouse{
		ID:                 id,
		CompanyProfileID:   companyID,
		BranchID:           branchID,
		Code:               code,
		Name:               name,
		Address:            address,
		DefaultAccountID:   defaultAccountID,
		DefaultAccountCode: defaultAccountCode,
		IsActive:           true,
		CreatedAt:          time.Now(),
		UpdatedAt:          time.Now(),
	}, nil
}

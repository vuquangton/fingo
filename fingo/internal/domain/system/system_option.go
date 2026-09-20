package system

import (
	"errors"
	"fmt"
	"time"

	"github.com/shopspring/decimal"
)

var (
	ErrNegativeStockBlocked = errors.New("negative stock blocked: available inventory is insufficient")
	ErrNegativeCashBlocked  = errors.New("negative cash blocked: vault cash balance is insufficient")
	ErrVatRateExpired       = errors.New("VAT rate 8% has expired under Resolution 204/2025/QH15 and Decree 174/2025/ND-CP")
	ErrOptionReadonly       = errors.New("cannot update read-only system option")
	ErrLifoProhibited       = errors.New("LIFO inventory costing method is strictly prohibited under VAS 02 and Circular 99/2025/TT-BTC")
	ErrInvalidOptionValue   = errors.New("invalid option value for configured data type")
)

const (
	PolicyDisallow = "DISALLOW"
	PolicyWarn     = "WARN"
	PolicyAllow    = "ALLOW"
)

// OptionCategory classifies configuration options by business sub-system
type OptionCategory string

const (
	OptionCategoryGeneral   OptionCategory = "GENERAL"
	OptionCategoryInventory OptionCategory = "INVENTORY"
	OptionCategoryCashBank  OptionCategory = "CASH_BANK"
	OptionCategoryVoucher   OptionCategory = "VOUCHER"
	OptionCategorySales     OptionCategory = "SALES"
	OptionCategoryPurchase  OptionCategory = "PURCHASE"
	OptionCategoryTax       OptionCategory = "TAX"
	OptionCategoryClosing   OptionCategory = "CLOSING"
	OptionCategorySecurity  OptionCategory = "SECURITY"
)

// OptionDataType defines the strictly-typed value representation
type OptionDataType string

const (
	DataTypeString  OptionDataType = "STRING"
	DataTypeInt     OptionDataType = "INT"
	DataTypeDecimal OptionDataType = "DECIMAL"
	DataTypeBoolean OptionDataType = "BOOLEAN"
	DataTypeJSON    OptionDataType = "JSON"
)

// ScopeLevel defines where an option is configured and applied
type ScopeLevel string

const (
	ScopeGlobal  ScopeLevel = "GLOBAL"
	ScopeCompany ScopeLevel = "COMPANY"
	ScopeBranch  ScopeLevel = "BRANCH"
	ScopeUser    ScopeLevel = "USER"
)

// SystemOption represents a strongly-typed, auditable system configuration parameter
type SystemOption struct {
	ID               string         `json:"id"`
	CompanyProfileID string         `json:"company_profile_id"`
	BranchID         *string        `json:"branch_id,omitempty"`
	Category         OptionCategory `json:"category"`
	OptionKey        string         `json:"option_key"`
	OptionValue      string         `json:"option_value"`
	DataType         OptionDataType `json:"data_type"`
	DefaultValue     string         `json:"default_value"`
	ScopeLevel       ScopeLevel     `json:"scope_level"`
	Description      string         `json:"description"`
	IsReadonly       bool           `json:"is_readonly"`
	IsEncrypted      bool           `json:"is_encrypted"`
	UpdatedAt        time.Time      `json:"updated_at"`
	UpdatedBy        string         `json:"updated_by"`
}

// ValidateNonCashPaymentThreshold checks if a cash payment violates the 5M statutory non-cash requirement
// under Luật Thuế GTGT số 48/2024/QH15 (effective 2025-07-01) & Decree 181/2025/NĐ-CP
func ValidateNonCashPaymentThreshold(amount decimal.Decimal, isCash bool, threshold decimal.Decimal) (isWarning bool, err error) {
	if !isCash {
		return false, nil
	}
	if threshold.IsZero() {
		threshold = decimal.NewFromInt(5_000_000)
	}

	if amount.GreaterThanOrEqual(threshold) {
		return true, nil
	}
	return false, nil
}

// CheckNegativeInventory validates physical stock dispatch against enterprise policy (VAS 02 & Circular 99/2025)
func CheckNegativeInventory(currentStock, dispatchQty decimal.Decimal, policy string) error {
	if policy == "" {
		policy = PolicyDisallow
	}

	remaining := currentStock.Sub(dispatchQty)
	if remaining.IsNegative() {
		if policy == PolicyDisallow {
			return fmt.Errorf("%w: current stock is %s, attempted dispatch is %s",
				ErrNegativeStockBlocked, currentStock.String(), dispatchQty.String())
		}
	}
	return nil
}

// CheckNegativeCash asserts physical cash on hand (TK 111) is never negative
func CheckNegativeCash(currentBalance, paymentAmount decimal.Decimal, policy string) error {
	if policy == "" {
		policy = PolicyDisallow
	}

	remaining := currentBalance.Sub(paymentAmount)
	if remaining.IsNegative() {
		if policy == PolicyDisallow {
			return fmt.Errorf("%w: vault cash balance is %s, attempted payment is %s",
				ErrNegativeCashBlocked, currentBalance.String(), paymentAmount.String())
		}
	}
	return nil
}

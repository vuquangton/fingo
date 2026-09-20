package catalog

import (
	"fmt"
	"strings"
	"time"

	"github.com/shopspring/decimal"
)

type ItemType string

const (
	ItemTypeMaterial     ItemType = "MATERIAL"      // Nguyên vật liệu (152)
	ItemTypeTool         ItemType = "TOOL"          // Công cụ dụng cụ (153)
	ItemTypeFinishedGood ItemType = "FINISHED_GOOD" // Thành phẩm (155)
	ItemTypeMerchandise  ItemType = "MERCHANDISE"   // Hàng hóa (156)
	ItemTypeService      ItemType = "SERVICE"       // Dịch vụ (Không qua kho)
)

// Item represents a material, tool, merchandise, finished good, or service (Vật tư, Hàng hóa, Dịch vụ)
type Item struct {
	ID                 string          `json:"id"`
	CompanyProfileID   string          `json:"company_profile_id"`
	Code               string          `json:"code"`
	Name               string          `json:"name"`
	Barcode            string          `json:"barcode,omitempty"`
	ItemType           ItemType        `json:"item_type"`
	BaseUOMID          string          `json:"base_uom_id"` // FK to unit_of_measures
	DefaultWarehouseID *string         `json:"default_warehouse_id,omitempty"`
	InventoryAccountID *string         `json:"inventory_account_id,omitempty"` // 152, 153, 155, 1561
	COGSAccountID      string          `json:"cogs_account_id"`                // 632
	RevenueAccountID   string          `json:"revenue_account_id"`             // 5111, 5112, 5113
	DefaultVATRate     decimal.Decimal `json:"default_vat_rate"`               // 0, 5, 8, 10%
	StandardCostPrice  decimal.Decimal `json:"standard_cost_price"`
	StandardSalePrice  decimal.Decimal `json:"standard_sale_price"`
	IsActive           bool            `json:"is_active"`
	CreatedAt          time.Time       `json:"created_at"`
	UpdatedAt          time.Time       `json:"updated_at"`
}

// ValidateItemAccounts verifies account compatibility with ItemType per Circular 99/2025/TT-BTC
func ValidateItemAccounts(itemType ItemType, invAccCode, cogsAccCode, revAccCode string) error {
	// Physical goods must have inventory account
	if itemType != ItemTypeService {
		if invAccCode == "" {
			return fmt.Errorf("%w: physical item type '%s' requires inventory account", ErrMissingAccount, itemType)
		}
		switch itemType {
		case ItemTypeMaterial:
			if !strings.HasPrefix(invAccCode, "152") {
				return fmt.Errorf("%w: material item requires 152 account, got '%s'", ErrIncompatibleItemTypeAccount, invAccCode)
			}
		case ItemTypeTool:
			if !strings.HasPrefix(invAccCode, "153") {
				return fmt.Errorf("%w: tool item requires 153 account, got '%s'", ErrIncompatibleItemTypeAccount, invAccCode)
			}
		case ItemTypeFinishedGood:
			if !strings.HasPrefix(invAccCode, "155") {
				return fmt.Errorf("%w: finished good requires 155 account, got '%s'", ErrIncompatibleItemTypeAccount, invAccCode)
			}
		case ItemTypeMerchandise:
			if !strings.HasPrefix(invAccCode, "156") {
				return fmt.Errorf("%w: merchandise requires 156 account, got '%s'", ErrIncompatibleItemTypeAccount, invAccCode)
			}
		}
	}

	if cogsAccCode != "" && !strings.HasPrefix(cogsAccCode, "632") {
		return fmt.Errorf("%w: COGS account must belong to group 632, got '%s'", ErrIncompatibleItemTypeAccount, cogsAccCode)
	}

	if revAccCode != "" && !strings.HasPrefix(revAccCode, "511") {
		return fmt.Errorf("%w: revenue account must belong to group 511, got '%s'", ErrIncompatibleItemTypeAccount, revAccCode)
	}

	return nil
}

func NewItem(
	id, companyID, code, name, barcode string,
	itemType ItemType,
	baseUOMID string,
	defaultWarehouseID *string,
	inventoryAccountID *string,
	cogsAccountID, revenueAccountID string,
	defaultVATRate, standardCostPrice, standardSalePrice decimal.Decimal,
) (*Item, error) {
	code = strings.ToUpper(strings.TrimSpace(code))
	if len(code) < 2 || len(code) > 50 {
		return nil, fmt.Errorf("%w: '%s'", ErrInvalidCode, code)
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, fmt.Errorf("item name cannot be empty")
	}

	if baseUOMID == "" {
		return nil, ErrMissingBaseUOM
	}

	if itemType != ItemTypeService && (inventoryAccountID == nil || *inventoryAccountID == "") {
		return nil, fmt.Errorf("%w: inventory account ID is required for non-service items", ErrMissingAccount)
	}

	if cogsAccountID == "" {
		return nil, fmt.Errorf("%w: COGS account ID (632) is required", ErrMissingAccount)
	}
	if revenueAccountID == "" {
		return nil, fmt.Errorf("%w: revenue account ID (511) is required", ErrMissingAccount)
	}

	if standardCostPrice.IsNegative() {
		standardCostPrice = decimal.Zero
	}
	if standardSalePrice.IsNegative() {
		standardSalePrice = decimal.Zero
	}

	return &Item{
		ID:                 id,
		CompanyProfileID:   companyID,
		Code:               code,
		Name:               name,
		Barcode:            strings.TrimSpace(barcode),
		ItemType:           itemType,
		BaseUOMID:          baseUOMID,
		DefaultWarehouseID: defaultWarehouseID,
		InventoryAccountID: inventoryAccountID,
		COGSAccountID:      cogsAccountID,
		RevenueAccountID:   revenueAccountID,
		DefaultVATRate:     defaultVATRate,
		StandardCostPrice:  standardCostPrice,
		StandardSalePrice:  standardSalePrice,
		IsActive:           true,
		CreatedAt:          time.Now(),
		UpdatedAt:          time.Now(),
	}, nil
}

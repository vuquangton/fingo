package catalog

import (
	"context"

	"github.com/shopspring/decimal"
)

type ItemType string

const (
	ItemTypeMaterial ItemType = "MATERIAL" // Nguyên vật liệu (152)
	ItemTypeTool     ItemType = "TOOL"     // Công cụ dụng cụ (153)
	ItemTypeGood     ItemType = "GOOD"     // Hàng hóa (156)
	ItemTypeProduct  ItemType = "PRODUCT"  // Thành phẩm (155)
	ItemTypeService  ItemType = "SERVICE"  // Dịch vụ
)

type Customer struct {
	ID          string
	Code        string
	Name        string
	TaxCode     string
	Address     string
	Phone       string
	Email       string
	CreditLimit decimal.Decimal
	IsActive    bool
}

type Vendor struct {
	ID          string
	Code        string
	Name        string
	TaxCode     string
	Address     string
	BankAccount string
	BankName    string
	IsActive    bool
}

type BankAccount struct {
	ID            string
	AccountNumber string
	BankName      string
	BranchName    string
	GLAccountCode string // e.g. 1121
	IsActive      bool
}

type Item struct {
	ID               string
	Code             string
	Name             string
	Barcode          string
	ItemType         ItemType
	BaseUOM          string
	InventoryAccount string          // 152, 156, etc.
	COGSAccount      string          // 632
	RevenueAccount   string          // 511
	DefaultVATRate   decimal.Decimal // 0, 5, 8, 10
	IsActive         bool
}

type Warehouse struct {
	ID       string
	Code     string
	Name     string
	Address  string
	IsActive bool
}

func NewCustomerStub(id, code, name, taxCode string) *Customer {
	return &Customer{
		ID:          id,
		Code:        code,
		Name:        name,
		TaxCode:     taxCode,
		CreditLimit: decimal.Zero,
		IsActive:    true,
	}
}

func NewItemStub(id, code, name, uom string) *Item {
	return &Item{
		ID:             id,
		Code:           code,
		Name:           name,
		ItemType:       ItemTypeGood,
		BaseUOM:        uom,
		DefaultVATRate: decimal.NewFromInt(10),
		IsActive:       true,
	}
}

type CatalogRepositoryStub interface {
	GetCustomer(ctx context.Context, id string) (*Customer, error)
	GetVendor(ctx context.Context, id string) (*Vendor, error)
	GetItem(ctx context.Context, id string) (*Item, error)
	GetWarehouse(ctx context.Context, id string) (*Warehouse, error)
}

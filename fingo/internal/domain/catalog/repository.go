package catalog

import (
	"context"
)

// UOMRepository defines the persistence seam for Units of Measure and conversions
type UOMRepository interface {
	SaveUOM(ctx context.Context, uom *UnitOfMeasure) error
	GetUOMByID(ctx context.Context, id string) (*UnitOfMeasure, error)
	GetUOMByCode(ctx context.Context, companyID, code string) (*UnitOfMeasure, error)
	ListUOMs(ctx context.Context, companyID string) ([]UnitOfMeasure, error)

	SaveConversion(ctx context.Context, conv *UOMConversion) error
	GetConversion(ctx context.Context, companyID string, itemID *string, fromUOMID, toUOMID string) (*UOMConversion, error)
	ListConversionsByItem(ctx context.Context, companyID string, itemID string) ([]UOMConversion, error)
}

// WarehouseRepository defines persistence for Warehouses
type WarehouseRepository interface {
	SaveWarehouse(ctx context.Context, w *Warehouse) error
	GetWarehouseByID(ctx context.Context, id string) (*Warehouse, error)
	GetWarehouseByCode(ctx context.Context, companyID, code string) (*Warehouse, error)
	ListWarehouses(ctx context.Context, companyID string) ([]Warehouse, error)
}

// BankAccountRepository defines persistence for Bank Accounts
type BankAccountRepository interface {
	SaveBankAccount(ctx context.Context, b *BankAccount) error
	GetBankAccountByID(ctx context.Context, id string) (*BankAccount, error)
	GetBankAccountByNumber(ctx context.Context, companyID, accNum string) (*BankAccount, error)
	ListBankAccounts(ctx context.Context, companyID string) ([]BankAccount, error)
}

// CustomerRepository defines persistence for Customers
type CustomerRepository interface {
	SaveCustomer(ctx context.Context, c *Customer) error
	GetCustomerByID(ctx context.Context, id string) (*Customer, error)
	GetCustomerByCode(ctx context.Context, companyID, code string) (*Customer, error)
	GetCustomerByTaxCode(ctx context.Context, companyID, taxCode string) (*Customer, error)
	ListCustomers(ctx context.Context, companyID string) ([]Customer, error)
}

// VendorRepository defines persistence for Vendors
type VendorRepository interface {
	SaveVendor(ctx context.Context, v *Vendor) error
	GetVendorByID(ctx context.Context, id string) (*Vendor, error)
	GetVendorByCode(ctx context.Context, companyID, code string) (*Vendor, error)
	GetVendorByTaxCode(ctx context.Context, companyID, taxCode string) (*Vendor, error)
	ListVendors(ctx context.Context, companyID string) ([]Vendor, error)
}

// ItemRepository defines persistence for Items
type ItemRepository interface {
	SaveItem(ctx context.Context, item *Item) error
	GetItemByID(ctx context.Context, id string) (*Item, error)
	GetItemByCode(ctx context.Context, companyID, code string) (*Item, error)
	ListItem(ctx context.Context, companyID string) ([]Item, error)
}

// EmployeeRepository defines persistence for Employees
type EmployeeRepository interface {
	SaveEmployee(ctx context.Context, emp *Employee) error
	GetEmployeeByID(ctx context.Context, id string) (*Employee, error)
	GetEmployeeByCode(ctx context.Context, companyID, code string) (*Employee, error)
	GetEmployeeByCitizenID(ctx context.Context, companyID, citizenID string) (*Employee, error)
	ListEmployees(ctx context.Context, companyID string) ([]Employee, error)
}

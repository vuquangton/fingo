package repository

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/shopspring/decimal"

	sqlc "fingo/internal/adapter/mariadb/sqlc"
	domain "fingo/internal/domain/catalog"
)

type CatalogRepo struct {
	db      *sql.DB
	queries *sqlc.Queries
}

func NewCatalogRepo(db *sql.DB) *CatalogRepo {
	return &CatalogRepo{
		db:      db,
		queries: sqlc.New(db),
	}
}

// Ensure CatalogRepo satisfies all domain repository interfaces
var (
	_ domain.UOMRepository         = (*CatalogRepo)(nil)
	_ domain.WarehouseRepository   = (*CatalogRepo)(nil)
	_ domain.BankAccountRepository = (*CatalogRepo)(nil)
	_ domain.CustomerRepository    = (*CatalogRepo)(nil)
	_ domain.VendorRepository      = (*CatalogRepo)(nil)
	_ domain.ItemRepository        = (*CatalogRepo)(nil)
	_ domain.EmployeeRepository    = (*CatalogRepo)(nil)
)

// ============================================================================
// UOMRepository Implementation
// ============================================================================

func (r *CatalogRepo) SaveUOM(ctx context.Context, uom *domain.UnitOfMeasure) error {
	var desc sql.NullString
	if uom.Description != "" {
		desc = sql.NullString{String: uom.Description, Valid: true}
	}

	err := r.queries.CreateUOM(ctx, sqlc.CreateUOMParams{
		ID:               uom.ID,
		CompanyProfileID: uom.CompanyProfileID,
		Code:             uom.Code,
		Name:             uom.Name,
		Description:      desc,
		IsActive:         uom.IsActive,
		CreatedAt:        uom.CreatedAt,
		UpdatedAt:        uom.UpdatedAt,
	})
	if err != nil {
		return fmt.Errorf("failed to save UOM (%s): %w", uom.Code, err)
	}
	return nil
}

func (r *CatalogRepo) GetUOMByID(ctx context.Context, id string) (*domain.UnitOfMeasure, error) {
	row, err := r.queries.GetUOMByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get UOM by ID (%s): %w", id, err)
	}
	return &domain.UnitOfMeasure{
		ID:               row.ID,
		CompanyProfileID: row.CompanyProfileID,
		Code:             row.Code,
		Name:             row.Name,
		Description:      row.Description.String,
		IsActive:         row.IsActive,
		CreatedAt:        row.CreatedAt,
		UpdatedAt:        row.UpdatedAt,
	}, nil
}

func (r *CatalogRepo) GetUOMByCode(ctx context.Context, companyID, code string) (*domain.UnitOfMeasure, error) {
	row, err := r.queries.GetUOMByCode(ctx, sqlc.GetUOMByCodeParams{
		CompanyProfileID: companyID,
		Code:             code,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get UOM by code (%s): %w", code, err)
	}
	return &domain.UnitOfMeasure{
		ID:               row.ID,
		CompanyProfileID: row.CompanyProfileID,
		Code:             row.Code,
		Name:             row.Name,
		Description:      row.Description.String,
		IsActive:         row.IsActive,
		CreatedAt:        row.CreatedAt,
		UpdatedAt:        row.UpdatedAt,
	}, nil
}

func (r *CatalogRepo) ListUOMs(ctx context.Context, companyID string) ([]domain.UnitOfMeasure, error) {
	rows, err := r.queries.ListUOMs(ctx, companyID)
	if err != nil {
		return nil, fmt.Errorf("failed to list UOMs: %w", err)
	}
	res := make([]domain.UnitOfMeasure, 0, len(rows))
	for _, row := range rows {
		res = append(res, domain.UnitOfMeasure{
			ID:               row.ID,
			CompanyProfileID: row.CompanyProfileID,
			Code:             row.Code,
			Name:             row.Name,
			Description:      row.Description.String,
			IsActive:         row.IsActive,
			CreatedAt:        row.CreatedAt,
			UpdatedAt:        row.UpdatedAt,
		})
	}
	return res, nil
}

func (r *CatalogRepo) SaveConversion(ctx context.Context, conv *domain.UOMConversion) error {
	var itemID sql.NullString
	if conv.ItemID != nil && *conv.ItemID != "" {
		itemID = sql.NullString{String: *conv.ItemID, Valid: true}
	}

	err := r.queries.CreateUOMConversion(ctx, sqlc.CreateUOMConversionParams{
		ID:               conv.ID,
		CompanyProfileID: conv.CompanyProfileID,
		ItemID:           itemID,
		FromUomID:        conv.FromUOMID,
		ToUomID:          conv.ToUOMID,
		Multiplier:       conv.Multiplier.String(),
		ConversionType:   sqlc.UomConversionsConversionType(conv.ConversionType),
		IsActive:         conv.IsActive,
		CreatedAt:        conv.CreatedAt,
		UpdatedAt:        conv.UpdatedAt,
	})
	if err != nil {
		return fmt.Errorf("failed to save UOM conversion: %w", err)
	}
	return nil
}

func (r *CatalogRepo) GetConversion(ctx context.Context, companyID string, itemID *string, fromUOMID, toUOMID string) (*domain.UOMConversion, error) {
	var iID sql.NullString
	if itemID != nil && *itemID != "" {
		iID = sql.NullString{String: *itemID, Valid: true}
	}

	row, err := r.queries.GetUOMConversion(ctx, sqlc.GetUOMConversionParams{
		CompanyProfileID: companyID,
		ItemID:           iID,
		FromUomID:        fromUOMID,
		ToUomID:          toUOMID,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get UOM conversion: %w", err)
	}

	multiplierDec, err := decimal.NewFromString(row.Multiplier)
	if err != nil {
		return nil, fmt.Errorf("corrupt decimal multiplier '%s': %w", row.Multiplier, err)
	}

	var resItemID *string
	if row.ItemID.Valid {
		s := row.ItemID.String
		resItemID = &s
	}

	return &domain.UOMConversion{
		ID:               row.ID,
		CompanyProfileID: row.CompanyProfileID,
		ItemID:           resItemID,
		FromUOMID:        row.FromUomID,
		ToUOMID:          row.ToUomID,
		Multiplier:       multiplierDec,
		ConversionType:   domain.ConversionType(row.ConversionType),
		IsActive:         row.IsActive,
		CreatedAt:        row.CreatedAt,
		UpdatedAt:        row.UpdatedAt,
	}, nil
}

func (r *CatalogRepo) ListConversionsByItem(ctx context.Context, companyID string, itemID string) ([]domain.UOMConversion, error) {
	var iID sql.NullString
	if itemID != "" {
		iID = sql.NullString{String: itemID, Valid: true}
	}

	rows, err := r.queries.ListUOMConversionsByItem(ctx, sqlc.ListUOMConversionsByItemParams{
		CompanyProfileID: companyID,
		ItemID:           iID,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list conversions: %w", err)
	}

	res := make([]domain.UOMConversion, 0, len(rows))
	for _, row := range rows {
		mult, err := decimal.NewFromString(row.Multiplier)
		if err != nil {
			return nil, fmt.Errorf("corrupt multiplier '%s': %w", row.Multiplier, err)
		}
		var rItemID *string
		if row.ItemID.Valid {
			s := row.ItemID.String
			rItemID = &s
		}
		res = append(res, domain.UOMConversion{
			ID:               row.ID,
			CompanyProfileID: row.CompanyProfileID,
			ItemID:           rItemID,
			FromUOMID:        row.FromUomID,
			ToUOMID:          row.ToUomID,
			Multiplier:       mult,
			ConversionType:   domain.ConversionType(row.ConversionType),
			IsActive:         row.IsActive,
			CreatedAt:        row.CreatedAt,
			UpdatedAt:        row.UpdatedAt,
		})
	}
	return res, nil
}

// ============================================================================
// WarehouseRepository Implementation
// ============================================================================

func (r *CatalogRepo) SaveWarehouse(ctx context.Context, w *domain.Warehouse) error {
	var branchID sql.NullString
	if w.BranchID != nil && *w.BranchID != "" {
		branchID = sql.NullString{String: *w.BranchID, Valid: true}
	}
	var addr sql.NullString
	if w.Address != "" {
		addr = sql.NullString{String: w.Address, Valid: true}
	}

	err := r.queries.CreateWarehouse(ctx, sqlc.CreateWarehouseParams{
		ID:               w.ID,
		CompanyProfileID: w.CompanyProfileID,
		BranchID:         branchID,
		Code:             w.Code,
		Name:             w.Name,
		Address:          addr,
		DefaultAccountID: w.DefaultAccountID,
		IsActive:         w.IsActive,
		CreatedAt:        w.CreatedAt,
		UpdatedAt:        w.UpdatedAt,
	})
	if err != nil {
		return fmt.Errorf("failed to save warehouse (%s): %w", w.Code, err)
	}
	return nil
}

func (r *CatalogRepo) GetWarehouseByID(ctx context.Context, id string) (*domain.Warehouse, error) {
	row, err := r.queries.GetWarehouseByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get warehouse by ID (%s): %w", id, err)
	}
	return r.mapWarehouse(row), nil
}

func (r *CatalogRepo) GetWarehouseByCode(ctx context.Context, companyID, code string) (*domain.Warehouse, error) {
	row, err := r.queries.GetWarehouseByCode(ctx, sqlc.GetWarehouseByCodeParams{
		CompanyProfileID: companyID,
		Code:             code,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get warehouse by code (%s): %w", code, err)
	}
	return r.mapWarehouse(row), nil
}

func (r *CatalogRepo) ListWarehouses(ctx context.Context, companyID string) ([]domain.Warehouse, error) {
	rows, err := r.queries.ListWarehouses(ctx, companyID)
	if err != nil {
		return nil, fmt.Errorf("failed to list warehouses: %w", err)
	}
	res := make([]domain.Warehouse, 0, len(rows))
	for _, row := range rows {
		res = append(res, *r.mapWarehouse(row))
	}
	return res, nil
}

func (r *CatalogRepo) mapWarehouse(row sqlc.Warehouse) *domain.Warehouse {
	var bID *string
	if row.BranchID.Valid {
		s := row.BranchID.String
		bID = &s
	}
	return &domain.Warehouse{
		ID:               row.ID,
		CompanyProfileID: row.CompanyProfileID,
		BranchID:         bID,
		Code:             row.Code,
		Name:             row.Name,
		Address:          row.Address.String,
		DefaultAccountID: row.DefaultAccountID,
		IsActive:         row.IsActive,
		CreatedAt:        row.CreatedAt,
		UpdatedAt:        row.UpdatedAt,
	}
}

// ============================================================================
// BankAccountRepository Implementation
// ============================================================================

func (r *CatalogRepo) SaveBankAccount(ctx context.Context, b *domain.BankAccount) error {
	var branchID sql.NullString
	if b.BranchID != nil && *b.BranchID != "" {
		branchID = sql.NullString{String: *b.BranchID, Valid: true}
	}
	var bBranch sql.NullString
	if b.BranchName != "" {
		bBranch = sql.NullString{String: b.BranchName, Valid: true}
	}

	err := r.queries.CreateBankAccount(ctx, sqlc.CreateBankAccountParams{
		ID:               b.ID,
		CompanyProfileID: b.CompanyProfileID,
		BranchID:         branchID,
		AccountNumber:    b.AccountNumber,
		BankName:         b.BankName,
		BankCode:         b.BankCode,
		BranchName:       bBranch,
		CurrencyCode:     b.CurrencyCode,
		GlAccountID:      b.GLAccountID,
		IsActive:         b.IsActive,
		CreatedAt:        b.CreatedAt,
		UpdatedAt:        b.UpdatedAt,
	})
	if err != nil {
		return fmt.Errorf("failed to save bank account (%s): %w", b.AccountNumber, err)
	}
	return nil
}

func (r *CatalogRepo) GetBankAccountByID(ctx context.Context, id string) (*domain.BankAccount, error) {
	row, err := r.queries.GetBankAccountByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get bank account by ID (%s): %w", id, err)
	}
	return r.mapBankAccount(row), nil
}

func (r *CatalogRepo) GetBankAccountByNumber(ctx context.Context, companyID, accNum string) (*domain.BankAccount, error) {
	row, err := r.queries.GetBankAccountByNumber(ctx, sqlc.GetBankAccountByNumberParams{
		CompanyProfileID: companyID,
		AccountNumber:    accNum,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get bank account by number (%s): %w", accNum, err)
	}
	return r.mapBankAccount(row), nil
}

func (r *CatalogRepo) ListBankAccounts(ctx context.Context, companyID string) ([]domain.BankAccount, error) {
	rows, err := r.queries.ListBankAccounts(ctx, companyID)
	if err != nil {
		return nil, fmt.Errorf("failed to list bank accounts: %w", err)
	}
	res := make([]domain.BankAccount, 0, len(rows))
	for _, row := range rows {
		res = append(res, *r.mapBankAccount(row))
	}
	return res, nil
}

func (r *CatalogRepo) mapBankAccount(row sqlc.BankAccount) *domain.BankAccount {
	var bID *string
	if row.BranchID.Valid {
		s := row.BranchID.String
		bID = &s
	}
	return &domain.BankAccount{
		ID:               row.ID,
		CompanyProfileID: row.CompanyProfileID,
		BranchID:         bID,
		AccountNumber:    row.AccountNumber,
		BankName:         row.BankName,
		BankCode:         row.BankCode,
		BranchName:       row.BranchName.String,
		CurrencyCode:     row.CurrencyCode,
		GLAccountID:      row.GlAccountID,
		IsActive:         row.IsActive,
		CreatedAt:        row.CreatedAt,
		UpdatedAt:        row.UpdatedAt,
	}
}

// ============================================================================
// CustomerRepository Implementation
// ============================================================================

func (r *CatalogRepo) SaveCustomer(ctx context.Context, c *domain.Customer) error {
	var taxCode, addr, phone, email, contact sql.NullString
	if c.TaxCode != "" {
		taxCode = sql.NullString{String: c.TaxCode, Valid: true}
	}
	if c.Address != "" {
		addr = sql.NullString{String: c.Address, Valid: true}
	}
	if c.Phone != "" {
		phone = sql.NullString{String: c.Phone, Valid: true}
	}
	if c.Email != "" {
		email = sql.NullString{String: c.Email, Valid: true}
	}
	if c.ContactPerson != "" {
		contact = sql.NullString{String: c.ContactPerson, Valid: true}
	}

	err := r.queries.CreateCustomer(ctx, sqlc.CreateCustomerParams{
		ID:                 c.ID,
		CompanyProfileID:   c.CompanyProfileID,
		Code:               c.Code,
		Name:               c.Name,
		TaxCode:            taxCode,
		Address:            addr,
		Phone:              phone,
		Email:              email,
		ContactPerson:      contact,
		PaymentTermDays:    int32(c.PaymentTermDays),
		CreditLimit:        c.CreditLimit.String(),
		EnforceCreditLimit: c.EnforceCreditLimit,
		DefaultArAccountID: c.DefaultARAccountID,
		IsActive:           c.IsActive,
		CreatedAt:          c.CreatedAt,
		UpdatedAt:          c.UpdatedAt,
	})
	if err != nil {
		return fmt.Errorf("failed to save customer (%s): %w", c.Code, err)
	}
	return nil
}

func (r *CatalogRepo) GetCustomerByID(ctx context.Context, id string) (*domain.Customer, error) {
	row, err := r.queries.GetCustomerByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get customer by ID (%s): %w", id, err)
	}
	return r.mapCustomerRow(row)
}

func (r *CatalogRepo) GetCustomerByCode(ctx context.Context, companyID, code string) (*domain.Customer, error) {
	row, err := r.queries.GetCustomerByCode(ctx, sqlc.GetCustomerByCodeParams{
		CompanyProfileID: companyID,
		Code:             code,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get customer by code (%s): %w", code, err)
	}
	return r.mapCustomerRow(row)
}

func (r *CatalogRepo) GetCustomerByTaxCode(ctx context.Context, companyID, taxCode string) (*domain.Customer, error) {
	row, err := r.queries.GetCustomerByTaxCode(ctx, sqlc.GetCustomerByTaxCodeParams{
		CompanyProfileID: companyID,
		TaxCode:          sql.NullString{String: strings.TrimSpace(taxCode), Valid: true},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get customer by tax code (%s): %w", taxCode, err)
	}
	return r.mapCustomerRow(row)
}

func (r *CatalogRepo) ListCustomers(ctx context.Context, companyID string) ([]domain.Customer, error) {
	rows, err := r.queries.ListCustomers(ctx, companyID)
	if err != nil {
		return nil, fmt.Errorf("failed to list customers: %w", err)
	}
	res := make([]domain.Customer, 0, len(rows))
	for _, row := range rows {
		c, err := r.mapCustomerRow(row)
		if err != nil {
			return nil, err
		}
		res = append(res, *c)
	}
	return res, nil
}

func (r *CatalogRepo) mapCustomerRow(row sqlc.Customer) (*domain.Customer, error) {
	limitDec, err := decimal.NewFromString(row.CreditLimit)
	if err != nil {
		return nil, fmt.Errorf("corrupt customer credit limit '%s': %w", row.CreditLimit, err)
	}
	return &domain.Customer{
		ID:                 row.ID,
		CompanyProfileID:   row.CompanyProfileID,
		Code:               row.Code,
		Name:               row.Name,
		TaxCode:            row.TaxCode.String,
		Address:            row.Address.String,
		Phone:              row.Phone.String,
		Email:              row.Email.String,
		ContactPerson:      row.ContactPerson.String,
		PaymentTermDays:    int(row.PaymentTermDays),
		CreditLimit:        limitDec,
		EnforceCreditLimit: row.EnforceCreditLimit,
		DefaultARAccountID: row.DefaultArAccountID,
		IsActive:           row.IsActive,
		CreatedAt:          row.CreatedAt,
		UpdatedAt:          row.UpdatedAt,
	}, nil
}

// ============================================================================
// VendorRepository Implementation
// ============================================================================

func (r *CatalogRepo) SaveVendor(ctx context.Context, v *domain.Vendor) error {
	var taxCode, addr, phone, email, contact, bAcc, bName, bBranch sql.NullString
	if v.TaxCode != "" {
		taxCode = sql.NullString{String: v.TaxCode, Valid: true}
	}
	if v.Address != "" {
		addr = sql.NullString{String: v.Address, Valid: true}
	}
	if v.Phone != "" {
		phone = sql.NullString{String: v.Phone, Valid: true}
	}
	if v.Email != "" {
		email = sql.NullString{String: v.Email, Valid: true}
	}
	if v.ContactPerson != "" {
		contact = sql.NullString{String: v.ContactPerson, Valid: true}
	}
	if v.BankAccountNumber != "" {
		bAcc = sql.NullString{String: v.BankAccountNumber, Valid: true}
	}
	if v.BankName != "" {
		bName = sql.NullString{String: v.BankName, Valid: true}
	}
	if v.BankBranch != "" {
		bBranch = sql.NullString{String: v.BankBranch, Valid: true}
	}

	err := r.queries.CreateVendor(ctx, sqlc.CreateVendorParams{
		ID:                 v.ID,
		CompanyProfileID:   v.CompanyProfileID,
		Code:               v.Code,
		Name:               v.Name,
		TaxCode:            taxCode,
		Address:            addr,
		Phone:              phone,
		Email:              email,
		ContactPerson:      contact,
		BankAccountNumber:  bAcc,
		BankName:           bName,
		BankBranch:         bBranch,
		PaymentTermDays:    int32(v.PaymentTermDays),
		DefaultApAccountID: v.DefaultAPAccountID,
		IsActive:           v.IsActive,
		CreatedAt:          v.CreatedAt,
		UpdatedAt:          v.UpdatedAt,
	})
	if err != nil {
		return fmt.Errorf("failed to save vendor (%s): %w", v.Code, err)
	}
	return nil
}

func (r *CatalogRepo) GetVendorByID(ctx context.Context, id string) (*domain.Vendor, error) {
	row, err := r.queries.GetVendorByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get vendor by ID (%s): %w", id, err)
	}
	return r.mapVendorRow(row)
}

func (r *CatalogRepo) GetVendorByCode(ctx context.Context, companyID, code string) (*domain.Vendor, error) {
	row, err := r.queries.GetVendorByCode(ctx, sqlc.GetVendorByCodeParams{
		CompanyProfileID: companyID,
		Code:             code,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get vendor by code (%s): %w", code, err)
	}
	return r.mapVendorRow(row)
}

func (r *CatalogRepo) GetVendorByTaxCode(ctx context.Context, companyID, taxCode string) (*domain.Vendor, error) {
	row, err := r.queries.GetVendorByTaxCode(ctx, sqlc.GetVendorByTaxCodeParams{
		CompanyProfileID: companyID,
		TaxCode:          sql.NullString{String: strings.TrimSpace(taxCode), Valid: true},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get vendor by tax code (%s): %w", taxCode, err)
	}
	return r.mapVendorRow(row)
}

func (r *CatalogRepo) ListVendors(ctx context.Context, companyID string) ([]domain.Vendor, error) {
	rows, err := r.queries.ListVendors(ctx, companyID)
	if err != nil {
		return nil, fmt.Errorf("failed to list vendors: %w", err)
	}
	res := make([]domain.Vendor, 0, len(rows))
	for _, row := range rows {
		v, err := r.mapVendorRow(row)
		if err != nil {
			return nil, err
		}
		res = append(res, *v)
	}
	return res, nil
}

func (r *CatalogRepo) mapVendorRow(row sqlc.Vendor) (*domain.Vendor, error) {
	return &domain.Vendor{
		ID:                 row.ID,
		CompanyProfileID:   row.CompanyProfileID,
		Code:               row.Code,
		Name:               row.Name,
		TaxCode:            row.TaxCode.String,
		Address:            row.Address.String,
		Phone:              row.Phone.String,
		Email:              row.Email.String,
		ContactPerson:      row.ContactPerson.String,
		BankAccountNumber:  row.BankAccountNumber.String,
		BankName:           row.BankName.String,
		BankBranch:         row.BankBranch.String,
		PaymentTermDays:    int(row.PaymentTermDays),
		DefaultAPAccountID: row.DefaultApAccountID,
		IsActive:           row.IsActive,
		CreatedAt:          row.CreatedAt,
		UpdatedAt:          row.UpdatedAt,
	}, nil
}

// ============================================================================
// ItemRepository Implementation
// ============================================================================

func (r *CatalogRepo) SaveItem(ctx context.Context, item *domain.Item) error {
	var barcode, whID, invAccID sql.NullString
	if item.Barcode != "" {
		barcode = sql.NullString{String: item.Barcode, Valid: true}
	}
	if item.DefaultWarehouseID != nil && *item.DefaultWarehouseID != "" {
		whID = sql.NullString{String: *item.DefaultWarehouseID, Valid: true}
	}
	if item.InventoryAccountID != nil && *item.InventoryAccountID != "" {
		invAccID = sql.NullString{String: *item.InventoryAccountID, Valid: true}
	}

	err := r.queries.CreateItem(ctx, sqlc.CreateItemParams{
		ID:                 item.ID,
		CompanyProfileID:   item.CompanyProfileID,
		Code:               item.Code,
		Name:               item.Name,
		Barcode:            barcode,
		ItemType:           sqlc.ItemsItemType(item.ItemType),
		BaseUomID:          item.BaseUOMID,
		DefaultWarehouseID: whID,
		InventoryAccountID: invAccID,
		CogsAccountID:      item.COGSAccountID,
		RevenueAccountID:   item.RevenueAccountID,
		DefaultVatRate:     item.DefaultVATRate.String(),
		StandardCostPrice:  item.StandardCostPrice.String(),
		StandardSalePrice:  item.StandardSalePrice.String(),
		IsActive:           item.IsActive,
		CreatedAt:          item.CreatedAt,
		UpdatedAt:          item.UpdatedAt,
	})
	if err != nil {
		return fmt.Errorf("failed to save item (%s): %w", item.Code, err)
	}
	return nil
}

func (r *CatalogRepo) GetItemByID(ctx context.Context, id string) (*domain.Item, error) {
	row, err := r.queries.GetItemByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get item by ID (%s): %w", id, err)
	}
	return r.mapItemRow(row)
}

func (r *CatalogRepo) GetItemByCode(ctx context.Context, companyID, code string) (*domain.Item, error) {
	row, err := r.queries.GetItemByCode(ctx, sqlc.GetItemByCodeParams{
		CompanyProfileID: companyID,
		Code:             code,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get item by code (%s): %w", code, err)
	}
	return r.mapItemRow(row)
}

func (r *CatalogRepo) ListItem(ctx context.Context, companyID string) ([]domain.Item, error) {
	rows, err := r.queries.ListItems(ctx, companyID)
	if err != nil {
		return nil, fmt.Errorf("failed to list items: %w", err)
	}
	res := make([]domain.Item, 0, len(rows))
	for _, row := range rows {
		it, err := r.mapItemRow(row)
		if err != nil {
			return nil, err
		}
		res = append(res, *it)
	}
	return res, nil
}

func (r *CatalogRepo) mapItemRow(row sqlc.Item) (*domain.Item, error) {
	vatDec, err := decimal.NewFromString(row.DefaultVatRate)
	if err != nil {
		return nil, fmt.Errorf("corrupt item VAT rate '%s': %w", row.DefaultVatRate, err)
	}
	costDec, err := decimal.NewFromString(row.StandardCostPrice)
	if err != nil {
		return nil, fmt.Errorf("corrupt item cost price '%s': %w", row.StandardCostPrice, err)
	}
	saleDec, err := decimal.NewFromString(row.StandardSalePrice)
	if err != nil {
		return nil, fmt.Errorf("corrupt item sale price '%s': %w", row.StandardSalePrice, err)
	}

	var whID, invAcc *string
	if row.DefaultWarehouseID.Valid {
		s := row.DefaultWarehouseID.String
		whID = &s
	}
	if row.InventoryAccountID.Valid {
		s := row.InventoryAccountID.String
		invAcc = &s
	}

	return &domain.Item{
		ID:                 row.ID,
		CompanyProfileID:   row.CompanyProfileID,
		Code:               row.Code,
		Name:               row.Name,
		Barcode:            row.Barcode.String,
		ItemType:           domain.ItemType(row.ItemType),
		BaseUOMID:          row.BaseUomID,
		DefaultWarehouseID: whID,
		InventoryAccountID: invAcc,
		COGSAccountID:      row.CogsAccountID,
		RevenueAccountID:   row.RevenueAccountID,
		DefaultVATRate:     vatDec,
		StandardCostPrice:  costDec,
		StandardSalePrice:  saleDec,
		IsActive:           row.IsActive,
		CreatedAt:          row.CreatedAt,
		UpdatedAt:          row.UpdatedAt,
	}, nil
}

// ============================================================================
// EmployeeRepository Implementation
// ============================================================================

func (r *CatalogRepo) SaveEmployee(ctx context.Context, emp *domain.Employee) error {
	var branchID, taxCode, bAcc, bName sql.NullString
	if emp.BranchID != nil && *emp.BranchID != "" {
		branchID = sql.NullString{String: *emp.BranchID, Valid: true}
	}
	if emp.TaxCode != "" {
		taxCode = sql.NullString{String: emp.TaxCode, Valid: true}
	}
	if emp.BankAccountNumber != "" {
		bAcc = sql.NullString{String: emp.BankAccountNumber, Valid: true}
	}
	if emp.BankName != "" {
		bName = sql.NullString{String: emp.BankName, Valid: true}
	}

	err := r.queries.CreateEmployee(ctx, sqlc.CreateEmployeeParams{
		ID:                emp.ID,
		CompanyProfileID:  emp.CompanyProfileID,
		BranchID:          branchID,
		Code:              emp.Code,
		FullName:          emp.FullName,
		Department:        emp.Department,
		Position:          emp.Position,
		CitizenID:         emp.CitizenID,
		TaxCode:           taxCode,
		SocialInsuranceNo: emp.SocialInsuranceNo,
		BaseSalary:        emp.BaseSalary.String(),
		SalaryCoefficient: emp.SalaryCoefficient.String(),
		BankAccountNumber: bAcc,
		BankName:          bName,
		DefaultAdvanceAcc: emp.DefaultAdvanceAcc,
		DefaultPayrollAcc: emp.DefaultPayrollAcc,
		IsActive:          emp.IsActive,
		CreatedAt:         emp.CreatedAt,
		UpdatedAt:         emp.UpdatedAt,
	})
	if err != nil {
		return fmt.Errorf("failed to save employee (%s): %w", emp.Code, err)
	}
	return nil
}

func (r *CatalogRepo) GetEmployeeByID(ctx context.Context, id string) (*domain.Employee, error) {
	row, err := r.queries.GetEmployeeByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get employee by ID (%s): %w", id, err)
	}
	return r.mapEmployeeRow(row)
}

func (r *CatalogRepo) GetEmployeeByCode(ctx context.Context, companyID, code string) (*domain.Employee, error) {
	row, err := r.queries.GetEmployeeByCode(ctx, sqlc.GetEmployeeByCodeParams{
		CompanyProfileID: companyID,
		Code:             code,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get employee by code (%s): %w", code, err)
	}
	return r.mapEmployeeRow(row)
}

func (r *CatalogRepo) GetEmployeeByCitizenID(ctx context.Context, companyID, citizenID string) (*domain.Employee, error) {
	row, err := r.queries.GetEmployeeByCitizenID(ctx, sqlc.GetEmployeeByCitizenIDParams{
		CompanyProfileID: companyID,
		CitizenID:        citizenID,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get employee by citizen ID (%s): %w", citizenID, err)
	}
	return r.mapEmployeeRow(row)
}

func (r *CatalogRepo) ListEmployees(ctx context.Context, companyID string) ([]domain.Employee, error) {
	rows, err := r.queries.ListEmployees(ctx, companyID)
	if err != nil {
		return nil, fmt.Errorf("failed to list employees: %w", err)
	}
	res := make([]domain.Employee, 0, len(rows))
	for _, row := range rows {
		emp, err := r.mapEmployeeRow(row)
		if err != nil {
			return nil, err
		}
		res = append(res, *emp)
	}
	return res, nil
}

func (r *CatalogRepo) mapEmployeeRow(row sqlc.Employee) (*domain.Employee, error) {
	salDec, err := decimal.NewFromString(row.BaseSalary)
	if err != nil {
		return nil, fmt.Errorf("corrupt employee salary '%s': %w", row.BaseSalary, err)
	}
	coeffDec, err := decimal.NewFromString(row.SalaryCoefficient)
	if err != nil {
		return nil, fmt.Errorf("corrupt employee coefficient '%s': %w", row.SalaryCoefficient, err)
	}

	var bID *string
	if row.BranchID.Valid {
		s := row.BranchID.String
		bID = &s
	}

	return &domain.Employee{
		ID:                row.ID,
		CompanyProfileID:  row.CompanyProfileID,
		BranchID:          bID,
		Code:              row.Code,
		FullName:          row.FullName,
		Department:        row.Department,
		Position:          row.Position,
		CitizenID:         row.CitizenID,
		TaxCode:           row.TaxCode.String,
		SocialInsuranceNo: row.SocialInsuranceNo,
		BaseSalary:        salDec,
		SalaryCoefficient: coeffDec,
		BankAccountNumber: row.BankAccountNumber.String,
		BankName:          row.BankName.String,
		DefaultAdvanceAcc: row.DefaultAdvanceAcc,
		DefaultPayrollAcc: row.DefaultPayrollAcc,
		IsActive:          row.IsActive,
		CreatedAt:         row.CreatedAt,
		UpdatedAt:         row.UpdatedAt,
	}, nil
}

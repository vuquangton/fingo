package catalog_test

import (
	"context"
	"database/sql"
	"testing"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	domain "fingo/internal/domain/catalog"
	spineDomain "fingo/internal/domain/spine"
	usecase "fingo/internal/usecase/catalog"
)

// ============================================================================
// In-Memory Stubs
// ============================================================================

type memUOMRepo struct {
	uoms        map[string]*domain.UnitOfMeasure
	conversions map[string]*domain.UOMConversion
}

func newMemUOMRepo() *memUOMRepo {
	return &memUOMRepo{
		uoms:        make(map[string]*domain.UnitOfMeasure),
		conversions: make(map[string]*domain.UOMConversion),
	}
}

func (m *memUOMRepo) SaveUOM(ctx context.Context, u *domain.UnitOfMeasure) error {
	m.uoms[u.ID] = u
	return nil
}
func (m *memUOMRepo) GetUOMByID(ctx context.Context, id string) (*domain.UnitOfMeasure, error) {
	u, ok := m.uoms[id]
	if !ok {
		return nil, sql.ErrNoRows
	}
	return u, nil
}
func (m *memUOMRepo) GetUOMByCode(ctx context.Context, companyID, code string) (*domain.UnitOfMeasure, error) {
	for _, u := range m.uoms {
		if u.CompanyProfileID == companyID && u.Code == code {
			return u, nil
		}
	}
	return nil, sql.ErrNoRows
}
func (m *memUOMRepo) ListUOMs(ctx context.Context, companyID string) ([]domain.UnitOfMeasure, error) {
	var res []domain.UnitOfMeasure
	for _, u := range m.uoms {
		if u.CompanyProfileID == companyID {
			res = append(res, *u)
		}
	}
	return res, nil
}
func (m *memUOMRepo) SaveConversion(ctx context.Context, c *domain.UOMConversion) error {
	key := c.CompanyProfileID + ":" + c.FromUOMID + ":" + c.ToUOMID
	m.conversions[key] = c
	return nil
}
func (m *memUOMRepo) GetConversion(ctx context.Context, companyID string, itemID *string, fromUOMID, toUOMID string) (*domain.UOMConversion, error) {
	key := companyID + ":" + fromUOMID + ":" + toUOMID
	c, ok := m.conversions[key]
	if !ok {
		return nil, sql.ErrNoRows
	}
	return c, nil
}
func (m *memUOMRepo) ListConversionsByItem(ctx context.Context, companyID string, itemID string) ([]domain.UOMConversion, error) {
	return nil, nil
}

type memCustomerRepo struct {
	customers map[string]*domain.Customer
}

func newMemCustomerRepo() *memCustomerRepo {
	return &memCustomerRepo{customers: make(map[string]*domain.Customer)}
}
func (m *memCustomerRepo) SaveCustomer(ctx context.Context, c *domain.Customer) error {
	m.customers[c.ID] = c
	return nil
}
func (m *memCustomerRepo) GetCustomerByID(ctx context.Context, id string) (*domain.Customer, error) {
	c, ok := m.customers[id]
	if !ok {
		return nil, sql.ErrNoRows
	}
	return c, nil
}
func (m *memCustomerRepo) GetCustomerByCode(ctx context.Context, companyID, code string) (*domain.Customer, error) {
	for _, c := range m.customers {
		if c.CompanyProfileID == companyID && c.Code == code {
			return c, nil
		}
	}
	return nil, sql.ErrNoRows
}
func (m *memCustomerRepo) GetCustomerByTaxCode(ctx context.Context, companyID, taxCode string) (*domain.Customer, error) {
	for _, c := range m.customers {
		if c.CompanyProfileID == companyID && c.TaxCode == taxCode {
			return c, nil
		}
	}
	return nil, sql.ErrNoRows
}
func (m *memCustomerRepo) ListCustomers(ctx context.Context, companyID string) ([]domain.Customer, error) {
	return nil, nil
}

type memVendorRepo struct {
	vendors map[string]*domain.Vendor
}

func newMemVendorRepo() *memVendorRepo {
	return &memVendorRepo{vendors: make(map[string]*domain.Vendor)}
}
func (m *memVendorRepo) SaveVendor(ctx context.Context, v *domain.Vendor) error {
	m.vendors[v.ID] = v
	return nil
}
func (m *memVendorRepo) GetVendorByID(ctx context.Context, id string) (*domain.Vendor, error) {
	v, ok := m.vendors[id]
	if !ok {
		return nil, sql.ErrNoRows
	}
	return v, nil
}
func (m *memVendorRepo) GetVendorByCode(ctx context.Context, companyID, code string) (*domain.Vendor, error) {
	for _, v := range m.vendors {
		if v.CompanyProfileID == companyID && v.Code == code {
			return v, nil
		}
	}
	return nil, sql.ErrNoRows
}
func (m *memVendorRepo) GetVendorByTaxCode(ctx context.Context, companyID, taxCode string) (*domain.Vendor, error) {
	return nil, nil
}
func (m *memVendorRepo) ListVendors(ctx context.Context, companyID string) ([]domain.Vendor, error) {
	return nil, nil
}

type memItemRepo struct {
	items map[string]*domain.Item
}

func newMemItemRepo() *memItemRepo {
	return &memItemRepo{items: make(map[string]*domain.Item)}
}
func (m *memItemRepo) SaveItem(ctx context.Context, item *domain.Item) error {
	m.items[item.ID] = item
	return nil
}
func (m *memItemRepo) GetItemByID(ctx context.Context, id string) (*domain.Item, error) {
	it, ok := m.items[id]
	if !ok {
		return nil, sql.ErrNoRows
	}
	return it, nil
}
func (m *memItemRepo) GetItemByCode(ctx context.Context, companyID, code string) (*domain.Item, error) {
	for _, it := range m.items {
		if it.CompanyProfileID == companyID && it.Code == code {
			return it, nil
		}
	}
	return nil, sql.ErrNoRows
}
func (m *memItemRepo) ListItem(ctx context.Context, companyID string) ([]domain.Item, error) {
	return nil, nil
}

type memWarehouseRepo struct {
	warehouses map[string]*domain.Warehouse
}

func newMemWarehouseRepo() *memWarehouseRepo {
	return &memWarehouseRepo{warehouses: make(map[string]*domain.Warehouse)}
}
func (m *memWarehouseRepo) SaveWarehouse(ctx context.Context, w *domain.Warehouse) error {
	m.warehouses[w.ID] = w
	return nil
}
func (m *memWarehouseRepo) GetWarehouseByID(ctx context.Context, id string) (*domain.Warehouse, error) {
	w, ok := m.warehouses[id]
	if !ok {
		return nil, sql.ErrNoRows
	}
	return w, nil
}
func (m *memWarehouseRepo) GetWarehouseByCode(ctx context.Context, companyID, code string) (*domain.Warehouse, error) {
	for _, w := range m.warehouses {
		if w.CompanyProfileID == companyID && w.Code == code {
			return w, nil
		}
	}
	return nil, sql.ErrNoRows
}
func (m *memWarehouseRepo) ListWarehouses(ctx context.Context, companyID string) ([]domain.Warehouse, error) {
	return nil, nil
}

type memBankAccountRepo struct {
	bankAccounts map[string]*domain.BankAccount
}

func newMemBankAccountRepo() *memBankAccountRepo {
	return &memBankAccountRepo{bankAccounts: make(map[string]*domain.BankAccount)}
}
func (m *memBankAccountRepo) SaveBankAccount(ctx context.Context, b *domain.BankAccount) error {
	m.bankAccounts[b.ID] = b
	return nil
}
func (m *memBankAccountRepo) GetBankAccountByID(ctx context.Context, id string) (*domain.BankAccount, error) {
	b, ok := m.bankAccounts[id]
	if !ok {
		return nil, sql.ErrNoRows
	}
	return b, nil
}
func (m *memBankAccountRepo) GetBankAccountByNumber(ctx context.Context, companyID, accNum string) (*domain.BankAccount, error) {
	for _, b := range m.bankAccounts {
		if b.CompanyProfileID == companyID && b.AccountNumber == accNum {
			return b, nil
		}
	}
	return nil, sql.ErrNoRows
}
func (m *memBankAccountRepo) ListBankAccounts(ctx context.Context, companyID string) ([]domain.BankAccount, error) {
	return nil, nil
}

type memEmployeeRepo struct {
	employees map[string]*domain.Employee
}

func newMemEmployeeRepo() *memEmployeeRepo {
	return &memEmployeeRepo{employees: make(map[string]*domain.Employee)}
}
func (m *memEmployeeRepo) SaveEmployee(ctx context.Context, e *domain.Employee) error {
	m.employees[e.ID] = e
	return nil
}
func (m *memEmployeeRepo) GetEmployeeByID(ctx context.Context, id string) (*domain.Employee, error) {
	e, ok := m.employees[id]
	if !ok {
		return nil, sql.ErrNoRows
	}
	return e, nil
}
func (m *memEmployeeRepo) GetEmployeeByCode(ctx context.Context, companyID, code string) (*domain.Employee, error) {
	for _, e := range m.employees {
		if e.CompanyProfileID == companyID && e.Code == code {
			return e, nil
		}
	}
	return nil, sql.ErrNoRows
}
func (m *memEmployeeRepo) GetEmployeeByCitizenID(ctx context.Context, companyID, citizenID string) (*domain.Employee, error) {
	for _, e := range m.employees {
		if e.CompanyProfileID == companyID && e.CitizenID == citizenID {
			return e, nil
		}
	}
	return nil, sql.ErrNoRows
}
func (m *memEmployeeRepo) ListEmployees(ctx context.Context, companyID string) ([]domain.Employee, error) {
	return nil, nil
}

type memAccountRepo struct {
	accounts map[string]*spineDomain.Account
}

func newMemAccountRepo() *memAccountRepo {
	return &memAccountRepo{accounts: make(map[string]*spineDomain.Account)}
}
func (m *memAccountRepo) SaveAccount(ctx context.Context, a *spineDomain.Account) error {
	m.accounts[a.ID] = a
	return nil
}
func (m *memAccountRepo) GetAccountByID(ctx context.Context, id string) (*spineDomain.Account, error) {
	a, ok := m.accounts[id]
	if !ok {
		return nil, sql.ErrNoRows
	}
	return a, nil
}
func (m *memAccountRepo) GetAccountByCode(ctx context.Context, companyID, code string) (*spineDomain.Account, error) {
	for _, a := range m.accounts {
		if a.CompanyProfileID == companyID && a.Code == code {
			return a, nil
		}
	}
	return nil, sql.ErrNoRows
}
func (m *memAccountRepo) ListAccounts(ctx context.Context, companyID string) ([]spineDomain.Account, error) {
	return nil, nil
}
func (m *memAccountRepo) ListChildAccounts(ctx context.Context, parentID string) ([]spineDomain.Account, error) {
	return nil, nil
}
func (m *memAccountRepo) UpdateLeafStatus(ctx context.Context, id string, isLeaf bool) error {
	return nil
}

// ============================================================================
// Usecase Tests
// ============================================================================

func TestCatalogUseCase_Customer_And_CreditLimit(t *testing.T) {
	t.Parallel()

	uomRepo := newMemUOMRepo()
	custRepo := newMemCustomerRepo()
	vendorRepo := newMemVendorRepo()
	itemRepo := newMemItemRepo()
	accRepo := newMemAccountRepo()

	uc := usecase.NewCatalogUseCase(uomRepo, nil, nil, custRepo, vendorRepo, itemRepo, nil, accRepo, nil)
	ctx := context.Background()
	companyID := "comp-01"

	// Seed leaf account TK 1311
	acc131 := &spineDomain.Account{
		ID: "acc-1311", CompanyProfileID: companyID, Code: "1311", Name: "Phải thu",
		IsLeaf: true, IsActive: true,
	}
	_ = accRepo.SaveAccount(ctx, acc131)

	// Seed non-leaf account TK 131
	accParent131 := &spineDomain.Account{
		ID: "acc-131", CompanyProfileID: companyID, Code: "131", Name: "Phải thu tổng hợp",
		IsLeaf: false, IsActive: true,
	}
	_ = accRepo.SaveAccount(ctx, accParent131)

	t.Run("Register customer success with valid MST and leaf account", func(t *testing.T) {
		cust, err := uc.RegisterCustomer(ctx, usecase.RegisterCustomerCommand{
			CompanyProfileID:   companyID,
			Code:               "KH001",
			Name:               "Công ty Alpha",
			TaxCode:            "0100109106",
			PaymentTermDays:    30,
			CreditLimit:        decimal.RequireFromString("100000000"),
			EnforceCreditLimit: true,
			DefaultARAccountID: "acc-1311",
		})
		require.NoError(t, err)
		assert.Equal(t, "KH001", cust.Code)

		// Idempotent resubmission
		reCust, err := uc.RegisterCustomer(ctx, usecase.RegisterCustomerCommand{
			CompanyProfileID:   companyID,
			Code:               "KH001",
			Name:               "Công ty Alpha",
			DefaultARAccountID: "acc-1311",
		})
		require.NoError(t, err)
		assert.Equal(t, cust.ID, reCust.ID)
	})

	t.Run("Register customer fails when AR account is non-leaf (INV-CAT-01)", func(t *testing.T) {
		_, err := uc.RegisterCustomer(ctx, usecase.RegisterCustomerCommand{
			CompanyProfileID:   companyID,
			Code:               "KH002",
			Name:               "Công ty Beta",
			DefaultARAccountID: "acc-131",
		})
		require.ErrorIs(t, err, usecase.ErrInvalidPostingAccount)
	})

	t.Run("Validate customer credit limit check (INV-CAT-05)", func(t *testing.T) {
		cust, err := custRepo.GetCustomerByCode(ctx, companyID, "KH001")
		require.NoError(t, err)

		// Outstanding 80M + new 10M <= 100M: PASS
		err = uc.ValidateCustomerCreditLimit(ctx, companyID, cust.ID, decimal.RequireFromString("80000000"), decimal.RequireFromString("10000000"))
		assert.NoError(t, err)

		// Outstanding 80M + new 30M > 100M: REJECT
		err = uc.ValidateCustomerCreditLimit(ctx, companyID, cust.ID, decimal.RequireFromString("80000000"), decimal.RequireFromString("30000000"))
		require.ErrorIs(t, err, domain.ErrCreditLimitExceeded)

		// Customer not found
		err = uc.ValidateCustomerCreditLimit(ctx, companyID, "cust-nonexistent", decimal.Zero, decimal.Zero)
		require.ErrorIs(t, err, usecase.ErrCustomerNotFound)
	})
}

func TestCatalogUseCase_Vendor_And_NonCashRule(t *testing.T) {
	t.Parallel()

	vendorRepo := newMemVendorRepo()
	accRepo := newMemAccountRepo()
	uc := usecase.NewCatalogUseCase(nil, nil, nil, nil, vendorRepo, nil, nil, accRepo, nil)
	ctx := context.Background()
	companyID := "comp-01"

	acc331 := &spineDomain.Account{
		ID: "acc-3311", CompanyProfileID: companyID, Code: "3311", Name: "Phải trả NCC",
		IsLeaf: true, IsActive: true,
	}
	_ = accRepo.SaveAccount(ctx, acc331)

	t.Run("Register vendor with bank account and validate non-cash rule (Decree 181/2025)", func(t *testing.T) {
		v, err := uc.RegisterVendor(ctx, usecase.RegisterVendorCommand{
			CompanyProfileID:   companyID,
			Code:               "NCC01",
			Name:               "Nhà cung cấp Petro",
			TaxCode:            "0100681592",
			BankAccountNumber:  "19020011223344",
			BankName:           "Techcombank",
			DefaultAPAccountID: "acc-3311",
		})
		require.NoError(t, err)

		// Amount 10,000,000 >= 5M with bank account: PASS
		err = uc.ValidateVendorNonCashPayment(ctx, companyID, v.ID, decimal.RequireFromString("10000000"))
		assert.NoError(t, err)
	})

	t.Run("Register vendor without bank details fails non-cash payment >= 5M", func(t *testing.T) {
		v, err := uc.RegisterVendor(ctx, usecase.RegisterVendorCommand{
			CompanyProfileID:   companyID,
			Code:               "NCC02",
			Name:               "Nhà cung cấp Tiền mặt",
			TaxCode:            "0100681592",
			DefaultAPAccountID: "acc-3311",
		})
		require.NoError(t, err)

		// Amount 4,999,999 < 5M: PASS (cash allowed)
		err = uc.ValidateVendorNonCashPayment(ctx, companyID, v.ID, decimal.RequireFromString("4999999"))
		assert.NoError(t, err)

		// Amount 5,000,000 >= 5M without bank account: REJECT
		err = uc.ValidateVendorNonCashPayment(ctx, companyID, v.ID, decimal.RequireFromString("5000000"))
		require.ErrorIs(t, err, usecase.ErrNonCashBankDetailsRequired)

		// Vendor not found
		err = uc.ValidateVendorNonCashPayment(ctx, companyID, "vendor-nonexistent", decimal.RequireFromString("10000000"))
		require.ErrorIs(t, err, usecase.ErrVendorNotFound)
	})

	t.Run("Register vendor fails when AP account is non-leaf", func(t *testing.T) {
		accNonLeaf := &spineDomain.Account{
			ID: "acc-331-parent", CompanyProfileID: companyID, Code: "331", Name: "Phải trả tổng",
			IsLeaf: false, IsActive: true,
		}
		_ = accRepo.SaveAccount(ctx, accNonLeaf)

		_, err := uc.RegisterVendor(ctx, usecase.RegisterVendorCommand{
			CompanyProfileID:   companyID,
			Code:               "NCC_NON_LEAF",
			Name:               "NCC Lỗi",
			DefaultAPAccountID: "acc-331-parent",
		})
		require.ErrorIs(t, err, usecase.ErrInvalidPostingAccount)
	})
}

func TestCatalogUseCase_Item_And_UOMConversion(t *testing.T) {
	t.Parallel()

	uomRepo := newMemUOMRepo()
	itemRepo := newMemItemRepo()
	accRepo := newMemAccountRepo()

	uc := usecase.NewCatalogUseCase(uomRepo, nil, nil, nil, nil, itemRepo, nil, accRepo, nil)
	ctx := context.Background()
	companyID := "comp-01"

	// Seed UOMs
	uomLon := &domain.UnitOfMeasure{ID: "uom-lon", CompanyProfileID: companyID, Code: "LON", Name: "Lon", IsActive: true}
	uomThung := &domain.UnitOfMeasure{ID: "uom-thung", CompanyProfileID: companyID, Code: "THUNG", Name: "Thùng", IsActive: true}
	_ = uomRepo.SaveUOM(ctx, uomLon)
	_ = uomRepo.SaveUOM(ctx, uomThung)

	// Seed Accounts
	acc156 := &spineDomain.Account{ID: "acc-1561", CompanyProfileID: companyID, Code: "1561", Name: "Hàng hóa", IsLeaf: true, IsActive: true}
	acc632 := &spineDomain.Account{ID: "acc-632", CompanyProfileID: companyID, Code: "632", Name: "Giá vốn", IsLeaf: true, IsActive: true}
	acc511 := &spineDomain.Account{ID: "acc-5111", CompanyProfileID: companyID, Code: "5111", Name: "Doanh thu", IsLeaf: true, IsActive: true}
	_ = accRepo.SaveAccount(ctx, acc156)
	_ = accRepo.SaveAccount(ctx, acc632)
	_ = accRepo.SaveAccount(ctx, acc511)

	invAcc := "acc-1561"

	t.Run("Register item with conversions and convert quantities", func(t *testing.T) {
		item, err := uc.RegisterItem(ctx, usecase.RegisterItemCommand{
			CompanyProfileID:   companyID,
			Code:               "BIA_SAIGON",
			Name:               "Bia Sài Gòn Special",
			ItemType:           domain.ItemTypeMerchandise,
			BaseUOMID:          "uom-lon",
			InventoryAccountID: &invAcc,
			COGSAccountID:      "acc-632",
			RevenueAccountID:   "acc-5111",
			DefaultVATRate:     decimal.RequireFromString("10"),
			Conversions: []usecase.ConversionParam{
				{
					FromUOMID:      "uom-thung",
					ToUOMID:        "uom-lon",
					Multiplier:     decimal.RequireFromString("24"),
					ConversionType: domain.ConversionTypeMultiply,
				},
			},
		})
		require.NoError(t, err)
		assert.Equal(t, "BIA_SAIGON", item.Code)

		// 1. Forward conversion: 10 Thùng -> 240 Lon
		qtyLon, err := uc.ConvertItemQuantity(ctx, companyID, item.ID, "uom-thung", "uom-lon", decimal.RequireFromString("10"), 0)
		require.NoError(t, err)
		assert.Equal(t, "240", qtyLon.String())

		// 2. Reverse conversion: 240 Lon -> 10 Thùng (using domain Invert)
		qtyThung, err := uc.ConvertItemQuantity(ctx, companyID, item.ID, "uom-lon", "uom-thung", decimal.RequireFromString("240"), 2)
		require.NoError(t, err)
		assert.Equal(t, "10", qtyThung.String())

		// 3. Same UOM conversion: 10 Lon -> 10 Lon
		qtySame, err := uc.ConvertItemQuantity(ctx, companyID, item.ID, "uom-lon", "uom-lon", decimal.RequireFromString("10"), 0)
		require.NoError(t, err)
		assert.Equal(t, "10", qtySame.String())

		// 4. Missing conversion
		_, err = uc.ConvertItemQuantity(ctx, companyID, item.ID, "uom-lon", "uom-nonexistent", decimal.RequireFromString("10"), 0)
		require.ErrorIs(t, err, usecase.ErrConversionNotFound)
	})

	t.Run("Register item fails if base UOM does not exist", func(t *testing.T) {
		_, err := uc.RegisterItem(ctx, usecase.RegisterItemCommand{
			CompanyProfileID:   companyID,
			Code:               "ITEM_NO_UOM",
			Name:               "Test Item",
			ItemType:           domain.ItemTypeMerchandise,
			BaseUOMID:          "non-existent-uom",
			InventoryAccountID: &invAcc,
			COGSAccountID:      "acc-632",
			RevenueAccountID:   "acc-5111",
		})
		require.ErrorIs(t, err, usecase.ErrUOMNotFound)
	})

	t.Run("Register item fails if COGS account is non-leaf", func(t *testing.T) {
		accNonLeaf632 := &spineDomain.Account{
			ID: "acc-632-parent", CompanyProfileID: companyID, Code: "632", Name: "Giá vốn tổng",
			IsLeaf: false, IsActive: true,
		}
		_ = accRepo.SaveAccount(ctx, accNonLeaf632)

		_, err := uc.RegisterItem(ctx, usecase.RegisterItemCommand{
			CompanyProfileID:   companyID,
			Code:               "ITEM_ERR_COGS",
			Name:               "Item COGS Error",
			ItemType:           domain.ItemTypeMerchandise,
			BaseUOMID:          "uom-lon",
			InventoryAccountID: &invAcc,
			COGSAccountID:      "acc-632-parent",
			RevenueAccountID:   "acc-5111",
		})
		require.ErrorIs(t, err, usecase.ErrInvalidPostingAccount)
	})
}

func TestCatalogUseCase_Warehouse_BankAccount_Employee(t *testing.T) {
	t.Parallel()

	whRepo := newMemWarehouseRepo()
	bankRepo := newMemBankAccountRepo()
	empRepo := newMemEmployeeRepo()
	accRepo := newMemAccountRepo()

	uc := usecase.NewCatalogUseCase(nil, whRepo, bankRepo, nil, nil, nil, empRepo, accRepo, nil)
	ctx := context.Background()
	companyID := "comp-01"

	// Seed COA Accounts
	acc156 := &spineDomain.Account{ID: "acc-156", CompanyProfileID: companyID, Code: "1561", Name: "Hàng hóa", IsLeaf: true, IsActive: true}
	acc1121 := &spineDomain.Account{ID: "acc-1121", CompanyProfileID: companyID, Code: "1121", Name: "Tiền gửi VND", IsLeaf: true, IsActive: true}
	acc141 := &spineDomain.Account{ID: "acc-141", CompanyProfileID: companyID, Code: "141", Name: "Tạm ứng", IsLeaf: true, IsActive: true}
	acc334 := &spineDomain.Account{ID: "acc-334", CompanyProfileID: companyID, Code: "3341", Name: "Phải trả lương", IsLeaf: true, IsActive: true}
	accInvalid642 := &spineDomain.Account{ID: "acc-642", CompanyProfileID: companyID, Code: "642", Name: "Chi phí QLDN", IsLeaf: true, IsActive: true}

	_ = accRepo.SaveAccount(ctx, acc156)
	_ = accRepo.SaveAccount(ctx, acc1121)
	_ = accRepo.SaveAccount(ctx, acc141)
	_ = accRepo.SaveAccount(ctx, acc334)
	_ = accRepo.SaveAccount(ctx, accInvalid642)

	t.Run("Register warehouse success", func(t *testing.T) {
		wh, err := uc.RegisterWarehouse(ctx, usecase.RegisterWarehouseCommand{
			CompanyProfileID: companyID,
			Code:             "KHO_TONG",
			Name:             "Kho Tổng Hà Nội",
			DefaultAccountID: "acc-156",
		})
		require.NoError(t, err)
		assert.Equal(t, "KHO_TONG", wh.Code)

		// Idempotent
		reWh, err := uc.RegisterWarehouse(ctx, usecase.RegisterWarehouseCommand{
			CompanyProfileID: companyID,
			Code:             "KHO_TONG",
			DefaultAccountID: "acc-156",
		})
		require.NoError(t, err)
		assert.Equal(t, wh.ID, reWh.ID)
	})

	t.Run("Register warehouse fails with non-inventory account (INV-CAT-07)", func(t *testing.T) {
		_, err := uc.RegisterWarehouse(ctx, usecase.RegisterWarehouseCommand{
			CompanyProfileID: companyID,
			Code:             "KHO_ERR",
			Name:             "Kho Lỗi",
			DefaultAccountID: "acc-642",
		})
		require.Error(t, err)
	})

	t.Run("Register bank account success", func(t *testing.T) {
		bankAcc, err := uc.RegisterBankAccount(ctx, usecase.RegisterBankAccountCommand{
			CompanyProfileID: companyID,
			AccountNumber:    "001100223344",
			BankName:         "Vietcombank",
			BankCode:         "VCB",
			CurrencyCode:     "VND",
			GLAccountID:      "acc-1121",
		})
		require.NoError(t, err)
		assert.Equal(t, "001100223344", bankAcc.AccountNumber)
	})

	t.Run("Register bank account fails with mismatched currency alignment (INV-CAT-06)", func(t *testing.T) {
		_, err := uc.RegisterBankAccount(ctx, usecase.RegisterBankAccountCommand{
			CompanyProfileID: companyID,
			AccountNumber:    "9988776655",
			BankName:         "Vietcombank USD",
			BankCode:         "VCB",
			CurrencyCode:     "USD", // USD mapped to 1121 must fail!
			GLAccountID:      "acc-1121",
		})
		require.Error(t, err)
	})

	t.Run("Register employee success with valid CCCD & BHXH (INV-CAT-08)", func(t *testing.T) {
		emp, err := uc.RegisterEmployee(ctx, usecase.RegisterEmployeeCommand{
			CompanyProfileID:    companyID,
			Code:                "NV001",
			FullName:            "Nguyễn Văn An",
			Department:          "Kế toán",
			Position:            "Kế toán viên",
			CitizenID:           "001200001234",
			SocialInsuranceNo:   "0123456789",
			BaseSalary:          decimal.RequireFromString("15000000"),
			SalaryCoefficient:   decimal.RequireFromString("1.0"),
			DefaultAdvanceAccID: "acc-141",
			DefaultPayrollAccID: "acc-334",
		})
		require.NoError(t, err)
		assert.Equal(t, "NV001", emp.Code)
		assert.Equal(t, "001200001234", emp.CitizenID)
	})

	t.Run("Register employee fails with invalid citizen ID", func(t *testing.T) {
		_, err := uc.RegisterEmployee(ctx, usecase.RegisterEmployeeCommand{
			CompanyProfileID:    companyID,
			Code:                "NV002",
			FullName:            "Trần Thị B",
			Department:          "Kế toán",
			Position:            "Nhân viên",
			CitizenID:           "123", // invalid
			SocialInsuranceNo:   "0123456789",
			DefaultAdvanceAccID: "acc-141",
			DefaultPayrollAccID: "acc-334",
		})
		require.ErrorIs(t, err, domain.ErrInvalidCitizenID)
	})

	t.Run("Register warehouse fails when account not found", func(t *testing.T) {
		_, err := uc.RegisterWarehouse(ctx, usecase.RegisterWarehouseCommand{
			CompanyProfileID: companyID,
			Code:             "KHO_NOT_FOUND",
			Name:             "Kho Không Tồn Tại",
			DefaultAccountID: "acc-nonexistent",
		})
		require.ErrorIs(t, err, usecase.ErrInvalidPostingAccount)
	})

	t.Run("Register bank account fails when account not found", func(t *testing.T) {
		_, err := uc.RegisterBankAccount(ctx, usecase.RegisterBankAccountCommand{
			CompanyProfileID: companyID,
			AccountNumber:    "1122334455",
			BankName:         "ACB",
			BankCode:         "ACB",
			CurrencyCode:     "VND",
			GLAccountID:      "acc-nonexistent",
		})
		require.ErrorIs(t, err, usecase.ErrInvalidPostingAccount)
	})

	t.Run("Register employee fails when advance account invalid", func(t *testing.T) {
		_, err := uc.RegisterEmployee(ctx, usecase.RegisterEmployeeCommand{
			CompanyProfileID:    companyID,
			Code:                "NV003",
			FullName:            "Phạm Văn C",
			Department:          "Kế toán",
			Position:            "Nhân viên",
			CitizenID:           "001200005678",
			SocialInsuranceNo:   "0123456780",
			DefaultAdvanceAccID: "acc-nonexistent",
			DefaultPayrollAccID: "acc-334",
		})
		require.ErrorIs(t, err, usecase.ErrInvalidPostingAccount)
	})

	t.Run("Register employee fails when payroll account invalid", func(t *testing.T) {
		_, err := uc.RegisterEmployee(ctx, usecase.RegisterEmployeeCommand{
			CompanyProfileID:    companyID,
			Code:                "NV004",
			FullName:            "Đỗ Thị D",
			Department:          "Kế toán",
			Position:            "Nhân viên",
			CitizenID:           "001200009999",
			SocialInsuranceNo:   "0123456781",
			DefaultAdvanceAccID: "acc-141",
			DefaultPayrollAccID: "acc-nonexistent",
		})
		require.ErrorIs(t, err, usecase.ErrInvalidPostingAccount)
	})
}

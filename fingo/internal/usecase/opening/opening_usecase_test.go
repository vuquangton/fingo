package opening_test

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	catalogDomain "fingo/internal/domain/catalog"
	domain "fingo/internal/domain/opening"
	spineDomain "fingo/internal/domain/spine"
	usecase "fingo/internal/usecase/opening"
)

// ============================================================================
// In-Memory Stubs
// ============================================================================

type memOpeningRepo struct {
	batches map[string]*domain.OpeningBatch
}

func newMemOpeningRepo() *memOpeningRepo {
	return &memOpeningRepo{batches: make(map[string]*domain.OpeningBatch)}
}

func (m *memOpeningRepo) SaveOpeningBatch(ctx context.Context, b *domain.OpeningBatch) error {
	m.batches[b.ID] = b
	return nil
}

func (m *memOpeningRepo) GetOpeningBatchByID(ctx context.Context, id string) (*domain.OpeningBatch, error) {
	b, ok := m.batches[id]
	if !ok {
		return nil, sql.ErrNoRows
	}
	return b, nil
}

func (m *memOpeningRepo) GetOpeningBatchByDate(ctx context.Context, companyID string, asOfDate time.Time) (*domain.OpeningBatch, error) {
	for _, b := range m.batches {
		if b.CompanyProfileID == companyID && b.AsOfDate.Equal(asOfDate) {
			return b, nil
		}
	}
	return nil, sql.ErrNoRows
}

func (m *memOpeningRepo) SaveAccountBalances(ctx context.Context, batchID string, balances []domain.AccountOpeningBalance) error {
	b, ok := m.batches[batchID]
	if !ok {
		return sql.ErrNoRows
	}
	b.Accounts = balances
	return nil
}

func (m *memOpeningRepo) SaveCustomerBalances(ctx context.Context, batchID string, balances []domain.CustomerOpeningBalance) error {
	b, ok := m.batches[batchID]
	if !ok {
		return sql.ErrNoRows
	}
	b.Customers = balances
	return nil
}

func (m *memOpeningRepo) SaveVendorBalances(ctx context.Context, batchID string, balances []domain.VendorOpeningBalance) error {
	b, ok := m.batches[batchID]
	if !ok {
		return sql.ErrNoRows
	}
	b.Vendors = balances
	return nil
}

func (m *memOpeningRepo) SaveInventoryBalances(ctx context.Context, batchID string, balances []domain.InventoryOpeningBalance) error {
	b, ok := m.batches[batchID]
	if !ok {
		return sql.ErrNoRows
	}
	b.Inventory = balances
	return nil
}

func (m *memOpeningRepo) SaveAssetBalances(ctx context.Context, batchID string, balances []domain.AssetOpeningBalance) error {
	b, ok := m.batches[batchID]
	if !ok {
		return sql.ErrNoRows
	}
	b.Assets = balances
	return nil
}

func (m *memOpeningRepo) UpdateBatchStatus(ctx context.Context, batchID string, status domain.BatchStatus, committedBy *string, committedAt *time.Time) error {
	b, ok := m.batches[batchID]
	if !ok {
		return sql.ErrNoRows
	}
	b.Status = status
	b.CommittedBy = committedBy
	b.CommittedAt = committedAt
	return nil
}

// Stubs for Account, Customer, Vendor, Warehouse, Item
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

type memCustomerRepo struct {
	customers map[string]*catalogDomain.Customer
}

func newMemCustomerRepo() *memCustomerRepo {
	return &memCustomerRepo{customers: make(map[string]*catalogDomain.Customer)}
}
func (m *memCustomerRepo) SaveCustomer(ctx context.Context, c *catalogDomain.Customer) error {
	m.customers[c.ID] = c
	return nil
}
func (m *memCustomerRepo) GetCustomerByID(ctx context.Context, id string) (*catalogDomain.Customer, error) {
	c, ok := m.customers[id]
	if !ok {
		return nil, sql.ErrNoRows
	}
	return c, nil
}
func (m *memCustomerRepo) GetCustomerByCode(ctx context.Context, companyID, code string) (*catalogDomain.Customer, error) {
	return nil, nil
}
func (m *memCustomerRepo) GetCustomerByTaxCode(ctx context.Context, companyID, taxCode string) (*catalogDomain.Customer, error) {
	return nil, nil
}
func (m *memCustomerRepo) ListCustomers(ctx context.Context, companyID string) ([]catalogDomain.Customer, error) {
	return nil, nil
}

type memVendorRepo struct {
	vendors map[string]*catalogDomain.Vendor
}

func newMemVendorRepo() *memVendorRepo {
	return &memVendorRepo{vendors: make(map[string]*catalogDomain.Vendor)}
}
func (m *memVendorRepo) SaveVendor(ctx context.Context, v *catalogDomain.Vendor) error {
	m.vendors[v.ID] = v
	return nil
}
func (m *memVendorRepo) GetVendorByID(ctx context.Context, id string) (*catalogDomain.Vendor, error) {
	v, ok := m.vendors[id]
	if !ok {
		return nil, sql.ErrNoRows
	}
	return v, nil
}
func (m *memVendorRepo) GetVendorByCode(ctx context.Context, companyID, code string) (*catalogDomain.Vendor, error) {
	return nil, nil
}
func (m *memVendorRepo) GetVendorByTaxCode(ctx context.Context, companyID, taxCode string) (*catalogDomain.Vendor, error) {
	return nil, nil
}
func (m *memVendorRepo) ListVendors(ctx context.Context, companyID string) ([]catalogDomain.Vendor, error) {
	return nil, nil
}

type memWarehouseRepo struct {
	warehouses map[string]*catalogDomain.Warehouse
}

func newMemWarehouseRepo() *memWarehouseRepo {
	return &memWarehouseRepo{warehouses: make(map[string]*catalogDomain.Warehouse)}
}
func (m *memWarehouseRepo) SaveWarehouse(ctx context.Context, w *catalogDomain.Warehouse) error {
	m.warehouses[w.ID] = w
	return nil
}
func (m *memWarehouseRepo) GetWarehouseByID(ctx context.Context, id string) (*catalogDomain.Warehouse, error) {
	w, ok := m.warehouses[id]
	if !ok {
		return nil, sql.ErrNoRows
	}
	return w, nil
}
func (m *memWarehouseRepo) GetWarehouseByCode(ctx context.Context, companyID, code string) (*catalogDomain.Warehouse, error) {
	return nil, nil
}
func (m *memWarehouseRepo) ListWarehouses(ctx context.Context, companyID string) ([]catalogDomain.Warehouse, error) {
	return nil, nil
}

type memItemRepo struct {
	items map[string]*catalogDomain.Item
}

func newMemItemRepo() *memItemRepo {
	return &memItemRepo{items: make(map[string]*catalogDomain.Item)}
}
func (m *memItemRepo) SaveItem(ctx context.Context, item *catalogDomain.Item) error {
	m.items[item.ID] = item
	return nil
}
func (m *memItemRepo) GetItemByID(ctx context.Context, id string) (*catalogDomain.Item, error) {
	i, ok := m.items[id]
	if !ok {
		return nil, sql.ErrNoRows
	}
	return i, nil
}
func (m *memItemRepo) GetItemByCode(ctx context.Context, companyID, code string) (*catalogDomain.Item, error) {
	return nil, nil
}
func (m *memItemRepo) ListItem(ctx context.Context, companyID string) ([]catalogDomain.Item, error) {
	return nil, nil
}

// ============================================================================
// Usecase Tests
// ============================================================================

func TestOpeningUseCase_FullCutoverWorkflow(t *testing.T) {
	t.Parallel()

	openingRepo := newMemOpeningRepo()
	accRepo := newMemAccountRepo()
	custRepo := newMemCustomerRepo()
	vendorRepo := newMemVendorRepo()
	whRepo := newMemWarehouseRepo()
	itemRepo := newMemItemRepo()

	uc := usecase.NewOpeningUseCase(openingRepo, accRepo, custRepo, vendorRepo, whRepo, itemRepo, nil)
	ctx := context.Background()
	companyID := "comp-cutover-01"
	asOfDate := time.Date(2025, 12, 31, 0, 0, 0, 0, time.UTC)

	// Seed COA
	acc111 := &spineDomain.Account{ID: "acc-1111", CompanyProfileID: companyID, Code: "1111", Name: "Tiền mặt", IsLeaf: true, IsActive: true}
	acc131 := &spineDomain.Account{ID: "acc-1311", CompanyProfileID: companyID, Code: "1311", Name: "Phải thu", IsLeaf: true, IsActive: true}
	acc331 := &spineDomain.Account{ID: "acc-3311", CompanyProfileID: companyID, Code: "3311", Name: "Phải trả", IsLeaf: true, IsActive: true}
	acc156 := &spineDomain.Account{ID: "acc-1561", CompanyProfileID: companyID, Code: "1561", Name: "Hàng hóa", IsLeaf: true, IsActive: true}
	acc211 := &spineDomain.Account{ID: "acc-2111", CompanyProfileID: companyID, Code: "2111", Name: "TSCĐ", IsLeaf: true, IsActive: true}
	acc214 := &spineDomain.Account{ID: "acc-2141", CompanyProfileID: companyID, Code: "2141", Name: "Hao mòn", IsLeaf: true, IsActive: true}
	acc642 := &spineDomain.Account{ID: "acc-642", CompanyProfileID: companyID, Code: "6421", Name: "CPQL", IsLeaf: true, IsActive: true}
	acc411 := &spineDomain.Account{ID: "acc-4111", CompanyProfileID: companyID, Code: "4111", Name: "Vốn CSH", IsLeaf: true, IsActive: true}
	accParent111 := &spineDomain.Account{ID: "acc-111", CompanyProfileID: companyID, Code: "111", Name: "Tiền mặt tổng", IsLeaf: false, IsActive: true}

	_ = accRepo.SaveAccount(ctx, acc111)
	_ = accRepo.SaveAccount(ctx, acc131)
	_ = accRepo.SaveAccount(ctx, acc331)
	_ = accRepo.SaveAccount(ctx, acc156)
	_ = accRepo.SaveAccount(ctx, acc211)
	_ = accRepo.SaveAccount(ctx, acc214)
	_ = accRepo.SaveAccount(ctx, acc642)
	_ = accRepo.SaveAccount(ctx, acc411)
	_ = accRepo.SaveAccount(ctx, accParent111)

	// Seed Catalogs
	_ = custRepo.SaveCustomer(ctx, &catalogDomain.Customer{ID: "cust-01", CompanyProfileID: companyID, Code: "KH001", Name: "Alpha"})
	_ = vendorRepo.SaveVendor(ctx, &catalogDomain.Vendor{ID: "vend-01", CompanyProfileID: companyID, Code: "NCC001", Name: "Petro"})
	_ = whRepo.SaveWarehouse(ctx, &catalogDomain.Warehouse{ID: "wh-01", CompanyProfileID: companyID, Code: "KHO_TONG", Name: "Kho Tổng"})
	_ = itemRepo.SaveItem(ctx, &catalogDomain.Item{ID: "item-01", CompanyProfileID: companyID, Code: "SP01", Name: "Sản phẩm A"})

	var batchID string

	t.Run("Create opening batch successfully", func(t *testing.T) {
		b, err := uc.CreateOpeningBatch(ctx, usecase.CreateOpeningBatchCommand{
			CompanyProfileID: companyID,
			AsOfDate:         asOfDate,
			Notes:            "Opening cutover test",
		})
		require.NoError(t, err)
		assert.Equal(t, domain.BatchStatusDraft, b.Status)
		batchID = b.ID

		// Idempotent call returns existing
		sameBatch, err := uc.CreateOpeningBatch(ctx, usecase.CreateOpeningBatchCommand{
			CompanyProfileID: companyID,
			AsOfDate:         asOfDate,
		})
		require.NoError(t, err)
		assert.Equal(t, batchID, sameBatch.ID)
	})

	t.Run("Save account balances with leaf validation", func(t *testing.T) {
		// Non-leaf account fails
		err := uc.SaveAccountBalances(ctx, usecase.SaveAccountBalancesCommand{
			BatchID: batchID,
			Balances: []usecase.AccountBalanceItem{
				{AccountID: "acc-111", DebitAmountVND: decimal.RequireFromString("100000000")},
			},
		})
		require.ErrorIs(t, err, usecase.ErrNonLeafPostingAccount)

		// Balanced accounts pass
		err = uc.SaveAccountBalances(ctx, usecase.SaveAccountBalancesCommand{
			BatchID: batchID,
			Balances: []usecase.AccountBalanceItem{
				{AccountID: "acc-1111", DebitAmountVND: decimal.RequireFromString("100000000")},
				{AccountID: "acc-1311", DebitAmountVND: decimal.RequireFromString("200000000")},
				{AccountID: "acc-1561", DebitAmountVND: decimal.RequireFromString("300000000")},
				{AccountID: "acc-2111", DebitAmountVND: decimal.RequireFromString("500000000")},
				{AccountID: "acc-2141", CreditAmountVND: decimal.RequireFromString("100000000")},
				{AccountID: "acc-3311", CreditAmountVND: decimal.RequireFromString("200000000")},
				{AccountID: "acc-4111", CreditAmountVND: decimal.RequireFromString("800000000")},
			},
		})
		require.NoError(t, err)
	})

	t.Run("Save customer balances with catalog validation", func(t *testing.T) {
		// Unknown customer fails
		err := uc.SaveCustomerBalances(ctx, usecase.SaveCustomerBalancesCommand{
			BatchID: batchID,
			Balances: []usecase.CustomerBalanceItem{
				{CustomerID: "cust-nonexistent", DebitAmountVND: decimal.RequireFromString("200000000")},
			},
		})
		require.ErrorIs(t, err, usecase.ErrCustomerNotFound)

		// Valid customer passes
		err = uc.SaveCustomerBalances(ctx, usecase.SaveCustomerBalancesCommand{
			BatchID: batchID,
			Balances: []usecase.CustomerBalanceItem{
				{
					CustomerID:     "cust-01",
					InvoiceNo:      "HD-01",
					DebitAmountVND: decimal.RequireFromString("200000000"),
				},
			},
		})
		require.NoError(t, err)
	})

	t.Run("Save vendor balances with catalog validation", func(t *testing.T) {
		// Unknown vendor fails
		err := uc.SaveVendorBalances(ctx, usecase.SaveVendorBalancesCommand{
			BatchID: batchID,
			Balances: []usecase.VendorBalanceItem{
				{VendorID: "vend-nonexistent", CreditAmountVND: decimal.RequireFromString("200000000")},
			},
		})
		require.ErrorIs(t, err, usecase.ErrVendorNotFound)

		// Valid vendor passes
		err = uc.SaveVendorBalances(ctx, usecase.SaveVendorBalancesCommand{
			BatchID: batchID,
			Balances: []usecase.VendorBalanceItem{
				{
					VendorID:        "vend-01",
					BillNo:          "BILL-01",
					CreditAmountVND: decimal.RequireFromString("200000000"),
				},
			},
		})
		require.NoError(t, err)
	})

	t.Run("Save inventory balances with positive quantity/cost check", func(t *testing.T) {
		// Non-positive quantity fails
		err := uc.SaveInventoryBalances(ctx, usecase.SaveInventoryBalancesCommand{
			BatchID: batchID,
			Balances: []usecase.InventoryBalanceItem{
				{
					WarehouseID: "wh-01",
					ItemID:      "item-01",
					UOMID:       "uom-cai",
					Quantity:    decimal.Zero,
					UnitCost:    decimal.RequireFromString("100000"),
				},
			},
		})
		require.ErrorIs(t, err, domain.ErrInvalidInventoryBalance)

		// Valid inventory passes (1,000 x 300,000 = 300,000,000 matching TK 1561)
		err = uc.SaveInventoryBalances(ctx, usecase.SaveInventoryBalancesCommand{
			BatchID: batchID,
			Balances: []usecase.InventoryBalanceItem{
				{
					WarehouseID: "wh-01",
					ItemID:      "item-01",
					UOMID:       "uom-cai",
					Quantity:    decimal.RequireFromString("1000"),
					UnitCost:    decimal.RequireFromString("300000"),
				},
			},
		})
		require.NoError(t, err)
	})

	t.Run("Save asset balances with account and net book value validation", func(t *testing.T) {
		// Accumulated depreciation > original cost fails
		err := uc.SaveAssetBalances(ctx, usecase.SaveAssetBalancesCommand{
			BatchID: batchID,
			Balances: []usecase.AssetBalanceItem{
				{
					AssetCode:               "TS01",
					AssetName:               "O to",
					AssetAccountID:          "acc-2111",
					DepreciationAccountID:   "acc-2141",
					CostAccountID:           "acc-642",
					OriginalCost:            decimal.RequireFromString("500000000"),
					AccumulatedDepreciation: decimal.RequireFromString("600000000"), // excess depr
					UsefulLifeMonths:        60,
				},
			},
		})
		require.Error(t, err)

		// Valid asset passes (Cost: 500M, Depr: 100M matching TK 211 & 214)
		err = uc.SaveAssetBalances(ctx, usecase.SaveAssetBalancesCommand{
			BatchID: batchID,
			Balances: []usecase.AssetBalanceItem{
				{
					AssetCode:               "TS01",
					AssetName:               "O to",
					AssetAccountID:          "acc-2111",
					DepreciationAccountID:   "acc-2141",
					CostAccountID:           "acc-642",
					AcquisitionDate:         asOfDate,
					StartDepreciationDate:   asOfDate,
					OriginalCost:            decimal.RequireFromString("500000000"),
					AccumulatedDepreciation: decimal.RequireFromString("100000000"),
					UsefulLifeMonths:        60,
					MonthlyDepreciation:     decimal.RequireFromString("8333333"),
				},
			},
		})
		require.NoError(t, err)
	})

	t.Run("Reconcile batch passes all cross-layer checks", func(t *testing.T) {
		report, err := uc.ReconcileBatch(ctx, batchID)
		require.NoError(t, err)
		assert.True(t, report.AllReconciled)
		assert.True(t, report.TrialBalanceBalanced)
		assert.True(t, report.CustomerARReconciled)
		assert.True(t, report.VendorAPReconciled)
		assert.True(t, report.InventoryReconciled)
		assert.True(t, report.FixedAssetsReconciled)
	})

	t.Run("Commit opening batch locks cutover permanently", func(t *testing.T) {
		err := uc.CommitOpeningBatch(ctx, usecase.CommitOpeningBatchCommand{
			BatchID:     batchID,
			CommittedBy: "ktt_lead",
		})
		require.NoError(t, err)

		// Re-commit fails
		err = uc.CommitOpeningBatch(ctx, usecase.CommitOpeningBatchCommand{
			BatchID:     batchID,
			CommittedBy: "ktt_lead",
		})
		require.ErrorIs(t, err, domain.ErrBatchAlreadyCommitted)

		// Mutating balances after commit is rejected
		err = uc.SaveAccountBalances(ctx, usecase.SaveAccountBalancesCommand{
			BatchID: batchID,
		})
		require.ErrorIs(t, err, domain.ErrBatchAlreadyCommitted)
	})

	t.Run("GetOpeningBatch query tests", func(t *testing.T) {
		// Existing
		b, err := uc.GetOpeningBatch(ctx, batchID)
		require.NoError(t, err)
		assert.Equal(t, batchID, b.ID)

		// Not found
		_, err = uc.GetOpeningBatch(ctx, "nonexistent-batch")
		require.ErrorIs(t, err, usecase.ErrBatchNotFound)
	})

	t.Run("Error paths for non-existent batches", func(t *testing.T) {
		_, err := uc.ReconcileBatch(ctx, "nonexistent-batch")
		require.ErrorIs(t, err, usecase.ErrBatchNotFound)

		err = uc.CommitOpeningBatch(ctx, usecase.CommitOpeningBatchCommand{BatchID: "nonexistent-batch"})
		require.ErrorIs(t, err, usecase.ErrBatchNotFound)

		err = uc.SaveCustomerBalances(ctx, usecase.SaveCustomerBalancesCommand{BatchID: "nonexistent-batch"})
		require.ErrorIs(t, err, usecase.ErrBatchNotFound)

		err = uc.SaveVendorBalances(ctx, usecase.SaveVendorBalancesCommand{BatchID: "nonexistent-batch"})
		require.ErrorIs(t, err, usecase.ErrBatchNotFound)

		err = uc.SaveInventoryBalances(ctx, usecase.SaveInventoryBalancesCommand{BatchID: "nonexistent-batch"})
		require.ErrorIs(t, err, usecase.ErrBatchNotFound)

		err = uc.SaveAssetBalances(ctx, usecase.SaveAssetBalancesCommand{BatchID: "nonexistent-batch"})
		require.ErrorIs(t, err, usecase.ErrBatchNotFound)
	})
}

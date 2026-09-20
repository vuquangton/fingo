package repository_test

import (
	"context"
	"database/sql"
	"testing"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"fingo/internal/adapter/mariadb/repository"
	"fingo/internal/domain/opening"
)

func TestOpeningRepo_LiveMariaDB(t *testing.T) {
	dsn := "dev:123456@tcp(127.0.0.1:3306)/fingo?parseTime=true"
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Skipf("skipping live MariaDB test: %v", err)
		return
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		t.Skipf("skipping live MariaDB test (cannot ping): %v", err)
		return
	}

	ctx := context.Background()
	now := time.Now()

	// Clean up any stale dummy test company
	_, _ = db.ExecContext(ctx, "DELETE FROM company_profile WHERE legal_name = 'Opening Test Co'")

	var companyID string
	err = db.QueryRowContext(ctx, "SELECT id FROM company_profile WHERE is_active = TRUE LIMIT 1").Scan(&companyID)
	if err != nil || companyID == "" {
		companyID = "comp-open-test"
		_, err = db.ExecContext(ctx, "INSERT INTO company_profile (id, tax_code, legal_name, address, legal_representative, chief_accountant, tax_authority_code, tax_authority_name, created_at, updated_at) VALUES (?, '0101243150', 'FinGo Production Test', 'Hanoi', 'GD', 'KTT', '101', 'CCT', ?, ?)", companyID, now, now)
		require.NoError(t, err)
	}

	// 2. Seed catalog dependencies if not existing
	_, _ = db.ExecContext(ctx, "INSERT IGNORE INTO accounts (id, company_profile_id, code, name, nature, category, is_leaf, is_active, created_at, updated_at) VALUES ('acc-open-1111', ?, '1111', 'Tien VND', 'DEBIT', 'ASSET', 1, 1, ?, ?)", companyID, now, now)
	_, _ = db.ExecContext(ctx, "INSERT IGNORE INTO accounts (id, company_profile_id, code, name, nature, category, is_leaf, is_active, created_at, updated_at) VALUES ('acc-open-1311', ?, '1311', 'Phai thu KH', 'HERMAPHRODITE', 'ASSET', 1, 1, ?, ?)", companyID, now, now)
	_, _ = db.ExecContext(ctx, "INSERT IGNORE INTO accounts (id, company_profile_id, code, name, nature, category, is_leaf, is_active, created_at, updated_at) VALUES ('acc-open-3311', ?, '3311', 'Phai tra NCC', 'HERMAPHRODITE', 'LIABILITY', 1, 1, ?, ?)", companyID, now, now)
	_, _ = db.ExecContext(ctx, "INSERT IGNORE INTO accounts (id, company_profile_id, code, name, nature, category, is_leaf, is_active, created_at, updated_at) VALUES ('acc-open-1561', ?, '1561', 'Hang hoa', 'DEBIT', 'ASSET', 1, 1, ?, ?)", companyID, now, now)
	_, _ = db.ExecContext(ctx, "INSERT IGNORE INTO accounts (id, company_profile_id, code, name, nature, category, is_leaf, is_active, created_at, updated_at) VALUES ('acc-open-2111', ?, '2111', 'TSCD huu hinh', 'DEBIT', 'ASSET', 1, 1, ?, ?)", companyID, now, now)
	_, _ = db.ExecContext(ctx, "INSERT IGNORE INTO accounts (id, company_profile_id, code, name, nature, category, is_leaf, is_active, created_at, updated_at) VALUES ('acc-open-2141', ?, '2141', 'Hao mon TSCD', 'CREDIT', 'ASSET', 1, 1, ?, ?)", companyID, now, now)
	_, _ = db.ExecContext(ctx, "INSERT IGNORE INTO accounts (id, company_profile_id, code, name, nature, category, is_leaf, is_active, created_at, updated_at) VALUES ('acc-open-642', ?, '642', 'Chi phi QLDN', 'DEBIT', 'EXPENSE', 1, 1, ?, ?)", companyID, now, now)

	_, _ = db.ExecContext(ctx, "INSERT IGNORE INTO unit_of_measures (id, company_profile_id, code, name, is_active, created_at, updated_at) VALUES ('uom-open-cai', ?, 'CAI', 'Cai', 1, ?, ?)", companyID, now, now)
	_, _ = db.ExecContext(ctx, "INSERT IGNORE INTO warehouses (id, company_profile_id, code, name, default_account_id, is_active, created_at, updated_at) VALUES ('wh-open-1', ?, 'KHO_OPEN', 'Kho Open', 'acc-open-1561', 1, ?, ?)", companyID, now, now)
	_, _ = db.ExecContext(ctx, "INSERT IGNORE INTO customers (id, company_profile_id, code, name, default_ar_account_id, is_active, created_at, updated_at) VALUES ('cust-open-1', ?, 'KH_OPEN_1', 'Khach hang Open', 'acc-open-1311', 1, ?, ?)", companyID, now, now)
	_, _ = db.ExecContext(ctx, "INSERT IGNORE INTO vendors (id, company_profile_id, code, name, default_ap_account_id, is_active, created_at, updated_at) VALUES ('vend-open-1', ?, 'NCC_OPEN_1', 'Nha cung cap Open', 'acc-open-3311', 1, ?, ?)", companyID, now, now)
	_, _ = db.ExecContext(ctx, "INSERT IGNORE INTO items (id, company_profile_id, code, name, item_type, base_uom_id, cogs_account_id, revenue_account_id, is_active, created_at, updated_at) VALUES ('item-open-1', ?, 'ITEM_OPEN_1', 'Item Open', 'MERCHANDISE', 'uom-open-cai', 'acc-open-642', 'acc-open-642', 1, ?, ?)", companyID, now, now)

	repo := repository.NewOpeningRepo(db)
	asOfDate := time.Date(2025, 12, 31, 0, 0, 0, 0, time.UTC)
	batchID := "batch-open-" + time.Now().Format("150405")

	// Cleanup any previous batch for idempotency
	_, _ = db.ExecContext(ctx, "DELETE FROM opening_batches WHERE id = ? OR (company_profile_id = ? AND as_of_date = ?)", batchID, companyID, asOfDate)

	t.Run("Full OpeningBatch Lifecycle CRUD in MariaDB", func(t *testing.T) {
		batch := opening.NewOpeningBatch(batchID, companyID, asOfDate, "Cutover 2026 Live Test")
		batch.TotalDebit = decimal.RequireFromString("1500000000")
		batch.TotalCredit = decimal.RequireFromString("1500000000")

		err := repo.SaveOpeningBatch(ctx, batch)
		require.NoError(t, err)

		// 1. Save Account balances
		accBalances := []opening.AccountOpeningBalance{
			{
				AccountID:       "acc-open-1111",
				CurrencyCode:    "VND",
				DebitAmountVND:  decimal.RequireFromString("1500000000"),
				CreditAmountVND: decimal.Zero,
				ExchangeRate:    decimal.RequireFromString("1"),
			},
		}
		err = repo.SaveAccountBalances(ctx, batchID, accBalances)
		require.NoError(t, err)

		// 2. Save Customer balances
		custBalances := []opening.CustomerOpeningBalance{
			{
				CustomerID:      "cust-open-1",
				InvoiceNo:       "HD-001",
				CurrencyCode:    "VND",
				DebitAmountVND:  decimal.RequireFromString("50000000"),
				CreditAmountVND: decimal.Zero,
				ExchangeRate:    decimal.RequireFromString("1"),
			},
		}
		err = repo.SaveCustomerBalances(ctx, batchID, custBalances)
		require.NoError(t, err)

		// 3. Save Vendor balances
		vendBalances := []opening.VendorOpeningBalance{
			{
				VendorID:        "vend-open-1",
				BillNo:          "BILL-001",
				CurrencyCode:    "VND",
				DebitAmountVND:  decimal.Zero,
				CreditAmountVND: decimal.RequireFromString("80000000"),
				ExchangeRate:    decimal.RequireFromString("1"),
			},
		}
		err = repo.SaveVendorBalances(ctx, batchID, vendBalances)
		require.NoError(t, err)

		// 4. Save Inventory balances
		invBalances := []opening.InventoryOpeningBalance{
			{
				WarehouseID:    "wh-open-1",
				ItemID:         "item-open-1",
				UOMID:          "uom-open-cai",
				Quantity:       decimal.RequireFromString("100"),
				UnitCost:       decimal.RequireFromString("50000"),
				TotalAmountVND: decimal.RequireFromString("5000000"),
			},
		}
		err = repo.SaveInventoryBalances(ctx, batchID, invBalances)
		require.NoError(t, err)

		// 5. Save Asset balances
		assetBalances := []opening.AssetOpeningBalance{
			{
				AssetCode:               "TS-OPEN-1",
				AssetName:               "May in Canon",
				AssetAccountID:          "acc-open-2111",
				DepreciationAccountID:   "acc-open-2141",
				CostAccountID:           "acc-open-642",
				AcquisitionDate:         asOfDate,
				StartDepreciationDate:   asOfDate,
				OriginalCost:            decimal.RequireFromString("30000000"),
				AccumulatedDepreciation: decimal.RequireFromString("5000000"),
				NetBookValue:            decimal.RequireFromString("25000000"),
				UsefulLifeMonths:        36,
				RemainingLifeMonths:     30,
				MonthlyDepreciation:     decimal.RequireFromString("833333"),
			},
		}
		err = repo.SaveAssetBalances(ctx, batchID, assetBalances)
		require.NoError(t, err)

		// 6. Fetch batch by ID and verify all child balances populated
		fetched, err := repo.GetOpeningBatchByID(ctx, batchID)
		require.NoError(t, err)
		assert.Equal(t, batchID, fetched.ID)
		assert.Equal(t, opening.BatchStatusDraft, fetched.Status)
		assert.Len(t, fetched.Accounts, 1)
		assert.Equal(t, "1111", fetched.Accounts[0].AccountCode)
		assert.Len(t, fetched.Customers, 1)
		assert.Equal(t, "HD-001", fetched.Customers[0].InvoiceNo)
		assert.Len(t, fetched.Vendors, 1)
		assert.Equal(t, "BILL-001", fetched.Vendors[0].BillNo)
		assert.Len(t, fetched.Inventory, 1)
		assert.Equal(t, "100.0000", fetched.Inventory[0].Quantity.StringFixed(4))
		assert.Len(t, fetched.Assets, 1)
		assert.Equal(t, "TS-OPEN-1", fetched.Assets[0].AssetCode)

		// 7. Update status to COMMITTED
		commBy := "ktt_admin"
		commAt := time.Now()
		err = repo.UpdateBatchStatus(ctx, batchID, opening.BatchStatusCommitted, &commBy, &commAt)
		require.NoError(t, err)

		// Re-fetch to verify updated status
		updated, err := repo.GetOpeningBatchByDate(ctx, companyID, asOfDate)
		require.NoError(t, err)
		assert.Equal(t, opening.BatchStatusCommitted, updated.Status)
		require.NotNil(t, updated.CommittedBy)
		assert.Equal(t, "ktt_admin", *updated.CommittedBy)
	})
}

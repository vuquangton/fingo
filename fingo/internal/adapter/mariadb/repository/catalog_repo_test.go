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
	"fingo/internal/domain/catalog"
)

func TestCatalogRepo_LiveMariaDB(t *testing.T) {
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

	repo := repository.NewCatalogRepo(db)
	ctx := context.Background()

	companyID := "comp-cat-test"
	now := time.Now()

	// 1. Seed or find existing CompanyProfile
	var existsID string
	err = db.QueryRowContext(ctx, "SELECT id FROM company_profile LIMIT 1").Scan(&existsID)
	if err == nil && existsID != "" {
		companyID = existsID
	} else {
		_, err = db.ExecContext(ctx, "INSERT INTO company_profile (id, tax_code, legal_name, address, legal_representative, chief_accountant, tax_authority_code, tax_authority_name, created_at, updated_at) VALUES (?, '0109999888', 'Catalog Test Co', 'Hanoi', 'GD', 'KTT', '101', 'CCT', ?, ?)", companyID, now, now)
		require.NoError(t, err)
	}

	_, _ = db.ExecContext(ctx, "INSERT IGNORE INTO currencies (code, company_profile_id, name, symbol, decimal_places, is_base, is_active, created_at, updated_at) VALUES ('VND', ?, 'Dong', 'd', 0, 1, 1, ?, ?)", companyID, now, now)

	// Seed leaf accounts needed by catalog entities
	accountsToSeed := []struct {
		id, code, name string
	}{
		{"acc-cat-1121", "11211", "Tien gui VND"},
		{"acc-cat-131", "1311", "Phai thu KH"},
		{"acc-cat-331", "3311", "Phai tra NCC"},
		{"acc-cat-1561", "1561", "Hang hoa"},
		{"acc-cat-632", "6321", "Gia von"},
		{"acc-cat-5111", "5111", "Doanh thu"},
		{"acc-cat-141", "1411", "Tam ung"},
		{"acc-cat-334", "3341", "Luong"},
	}
	for _, a := range accountsToSeed {
		_, _ = db.ExecContext(ctx, "INSERT IGNORE INTO accounts (id, company_profile_id, code, name, account_level, nature, category, is_leaf, is_active, created_at, updated_at) VALUES (?, ?, ?, ?, 2, 'DEBIT', 'ASSET', 1, 1, ?, ?)", a.id, companyID, a.code, a.name, now, now)
	}

	t.Run("UOM and Conversion CRUD", func(t *testing.T) {
		uom1, err := catalog.NewUnitOfMeasure("uom-test-thung", companyID, "THUNG_TEST", "Thùng Test", "Thùng 24")
		require.NoError(t, err)
		err = repo.SaveUOM(ctx, uom1)
		require.NoError(t, err)

		uom2, err := catalog.NewUnitOfMeasure("uom-test-lon", companyID, "LON_TEST", "Lon Test", "Lon")
		require.NoError(t, err)
		err = repo.SaveUOM(ctx, uom2)
		require.NoError(t, err)

		fetched, err := repo.GetUOMByCode(ctx, companyID, "THUNG_TEST")
		require.NoError(t, err)
		assert.Equal(t, "Thùng Test", fetched.Name)

		// Conversion
		conv, err := catalog.NewUOMConversion(
			"conv-test-1", companyID, nil,
			uom1.ID, uom2.ID,
			decimal.RequireFromString("24"), catalog.ConversionTypeMultiply,
		)
		require.NoError(t, err)
		err = repo.SaveConversion(ctx, conv)
		require.NoError(t, err)

		fetchedConv, err := repo.GetConversion(ctx, companyID, nil, uom1.ID, uom2.ID)
		require.NoError(t, err)
		assert.Equal(t, "24", fetchedConv.Multiplier.String())
	})

	t.Run("Warehouse CRUD", func(t *testing.T) {
		wh, err := catalog.NewWarehouse(
			"wh-test-1", companyID, nil,
			"KHO_TEST", "Kho Thử Nghiệm", "Hà Nội",
			"acc-cat-1561", "1561",
		)
		require.NoError(t, err)
		err = repo.SaveWarehouse(ctx, wh)
		require.NoError(t, err)

		fetched, err := repo.GetWarehouseByCode(ctx, companyID, "KHO_TEST")
		require.NoError(t, err)
		assert.Equal(t, "Kho Thử Nghiệm", fetched.Name)
	})

	t.Run("BankAccount CRUD", func(t *testing.T) {
		ba, err := catalog.NewBankAccount(
			"ba-test-1", companyID, nil,
			"888888888888", "Techcombank", "TCB", "Chi nhánh 1", "VND",
			"acc-cat-1121", "11211",
		)
		require.NoError(t, err)
		err = repo.SaveBankAccount(ctx, ba)
		require.NoError(t, err)

		fetched, err := repo.GetBankAccountByNumber(ctx, companyID, "888888888888")
		require.NoError(t, err)
		assert.Equal(t, "Techcombank", fetched.BankName)
	})

	t.Run("Customer CRUD", func(t *testing.T) {
		cust, err := catalog.NewCustomer(
			"cust-test-1", companyID, "KH_TEST", "Khách Hàng Test", "0100109106",
			"Hà Nội", "0901234567", "kh@test.vn", "Nguyễn A",
			30, decimal.RequireFromString("150000000"), true,
			"acc-cat-131", "1311",
		)
		require.NoError(t, err)
		err = repo.SaveCustomer(ctx, cust)
		require.NoError(t, err)

		fetched, err := repo.GetCustomerByCode(ctx, companyID, "KH_TEST")
		require.NoError(t, err)
		assert.Equal(t, "150000000", fetched.CreditLimit.String())
	})

	t.Run("Vendor CRUD", func(t *testing.T) {
		v, err := catalog.NewVendor(
			"vendor-test-1", companyID, "NCC_TEST", "Nhà Cung Cấp Test", "0100681592",
			"Hà Nội", "0912345678", "ncc@test.vn", "Trần B",
			"19039999999", "Vietcombank", "Hanoi",
			45, "acc-cat-331", "3311",
		)
		require.NoError(t, err)
		err = repo.SaveVendor(ctx, v)
		require.NoError(t, err)

		fetched, err := repo.GetVendorByCode(ctx, companyID, "NCC_TEST")
		require.NoError(t, err)
		assert.Equal(t, "Nhà Cung Cấp Test", fetched.Name)
	})

	t.Run("Item CRUD", func(t *testing.T) {
		inv := "acc-cat-1561"
		item, err := catalog.NewItem(
			"item-test-1", companyID, "VT_TEST", "Vật Tư Test", "893000000001",
			catalog.ItemTypeMerchandise, "uom-test-thung", nil, &inv,
			"acc-cat-632", "acc-cat-5111",
			decimal.RequireFromString("10"),
			decimal.RequireFromString("20000"),
			decimal.RequireFromString("25000"),
		)
		require.NoError(t, err)
		err = repo.SaveItem(ctx, item)
		require.NoError(t, err)

		fetched, err := repo.GetItemByCode(ctx, companyID, "VT_TEST")
		require.NoError(t, err)
		assert.Equal(t, "25000", fetched.StandardSalePrice.String())
	})

	t.Run("Employee CRUD", func(t *testing.T) {
		emp, err := catalog.NewEmployee(
			"emp-test-1", companyID, nil,
			"NV_TEST", "Nhân Viên Test", "Phòng Tài Chính", "Kế toán",
			"001090999999", "8012345678", "0123456789",
			decimal.RequireFromString("18000000"), decimal.RequireFromString("1.2"),
			"19036888888888", "Techcombank",
			"acc-cat-141", "acc-cat-334",
		)
		require.NoError(t, err)
		err = repo.SaveEmployee(ctx, emp)
		require.NoError(t, err)

		fetched, err := repo.GetEmployeeByCode(ctx, companyID, "NV_TEST")
		require.NoError(t, err)
		assert.Equal(t, "18000000", fetched.BaseSalary.String())
	})
}

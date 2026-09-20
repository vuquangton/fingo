package repository_test

import (
	"context"
	"database/sql"
	"testing"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"fingo/internal/adapter/mariadb/repository"
	"fingo/internal/domain/gl"
	"fingo/internal/domain/sales"
)

func TestSalesRepo_LiveMariaDB(t *testing.T) {
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

	var companyID string
	err = db.QueryRowContext(ctx, "SELECT id FROM company_profile WHERE is_active = TRUE LIMIT 1").Scan(&companyID)
	if err != nil || companyID == "" {
		companyID = "comp-sales-test"
		_, err = db.ExecContext(ctx, "INSERT INTO company_profile (id, tax_code, legal_name, address, legal_representative, chief_accountant, tax_authority_code, tax_authority_name, created_at, updated_at) VALUES (?, '0101243154', 'FinGo Sales Test', 'Hanoi', 'GD', 'KTT', '101', 'CCT', ?, ?)", companyID, now, now)
		require.NoError(t, err)
	}

	getAccountID := func(code, name, nature, category string) string {
		var id string
		err := db.QueryRowContext(ctx, "SELECT id FROM accounts WHERE company_profile_id = ? AND code = ?", companyID, code).Scan(&id)
		if err == nil && id != "" {
			return id
		}
		id = "acc-sr-" + code
		_, _ = db.ExecContext(ctx, "INSERT INTO accounts (id, company_profile_id, code, name, nature, category, is_leaf, is_active, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, 1, 1, NOW(), NOW())", id, companyID, code, name, nature, category)
		return id
	}

	arAccID := getAccountID("131", "Phải thu khách hàng", "DEBIT", "ASSET")
	revAccID := getAccountID("5111", "Doanh thu bán hàng", "CREDIT", "REVENUE")
	vatAccID := getAccountID("33311", "Thuế GTGT phải nộp", "CREDIT", "LIABILITY")

	// Ensure a customer exists
	var customerID string
	err = db.QueryRowContext(ctx, "SELECT id FROM customers WHERE company_profile_id = ? LIMIT 1", companyID).Scan(&customerID)
	if err != nil || customerID == "" {
		customerID = uuid.New().String()
		_, err = db.ExecContext(ctx, `INSERT INTO customers (id, company_profile_id, code, name, is_active, created_at, updated_at)
			VALUES (?, ?, 'CUST-TEST-01', 'Khách hàng Minh Anh', TRUE, ?, ?)`, customerID, companyID, now, now)
		require.NoError(t, err)
	}

	// Ensure a UOM and Item exists
	var uomID string
	err = db.QueryRowContext(ctx, "SELECT id FROM unit_of_measures WHERE company_profile_id = ? LIMIT 1", companyID).Scan(&uomID)
	if err != nil || uomID == "" {
		uomID = uuid.New().String()
		_, err = db.ExecContext(ctx, `INSERT INTO unit_of_measures (id, company_profile_id, code, name, is_active, created_at, updated_at)
			VALUES (?, ?, 'CAI', 'Cái', TRUE, ?, ?)`, uomID, companyID, now, now)
		require.NoError(t, err)
	}

	var itemID string
	err = db.QueryRowContext(ctx, "SELECT id FROM items WHERE company_profile_id = ? LIMIT 1", companyID).Scan(&itemID)
	if err != nil || itemID == "" {
		itemID = uuid.New().String()
		_, err = db.ExecContext(ctx, `INSERT INTO items (id, company_profile_id, code, name, item_type, uom_id, is_active, created_at, updated_at)
			VALUES (?, ?, 'ITEM-SALES-01', 'Sản phẩm mẫu bán hàng', 'MERCHANDISE', ?, TRUE, ?, ?)`, itemID, companyID, uomID, now, now)
		require.NoError(t, err)
	}

	voucherRepo := repository.NewVoucherRepo(db)
	salesRepo := repository.NewSalesRepo(db)

	t.Run("Save and Retrieve Sales Invoice with Lines", func(t *testing.T) {
		vID := uuid.New().String()
		vNo := "BH-" + vID[:8]
		v, err := gl.NewVoucher(gl.CreateVoucherParams{
			ID:               vID,
			CompanyProfileID: companyID,
			VoucherNo:        vNo,
			VoucherDate:      now,
			PostedDate:       now,
			VoucherType:      gl.VoucherTypeSales,
			Description:      "Bán hàng hóa xuất hóa đơn GTGT",
			CreatedBy:        "tester",
			Lines: []gl.VoucherLine{
				{
					ID:                uuid.New().String(),
					LineOrder:         1,
					DebitAccountID:    arAccID,
					CreditAccountID:   revAccID,
					DebitAccountCode:  "131",
					CreditAccountCode: "5111",
					AmountVND:         decimal.NewFromInt(19000000), // Net revenue
					Note:              "Doanh thu thuần",
				},
				{
					ID:                uuid.New().String(),
					LineOrder:         2,
					DebitAccountID:    arAccID,
					CreditAccountID:   vatAccID,
					DebitAccountCode:  "131",
					CreditAccountCode: "33311",
					AmountVND:         decimal.NewFromInt(1900000), // 10% VAT
					Note:              "Thuế GTGT đầu ra 10%",
				},
			},
		})
		require.NoError(t, err)
		require.NoError(t, voucherRepo.SaveVoucher(ctx, v))

		si, err := sales.NewSalesInvoice(sales.CreateSalesInvoiceParams{
			ID:                 uuid.New().String(),
			VoucherID:          v.ID,
			CompanyProfileID:   companyID,
			CustomerID:         customerID,
			InvoiceTemplate:    "1",
			InvoiceSeries:      "C26TAA",
			InvoiceNo:          "00000888",
			InvoiceDate:        now,
			DueDate:            now.AddDate(0, 1, 0),
			PaymentMethod:      "CK",
			IsStockOutwardAuto: true,
			Lines: []sales.CreateSalesInvoiceLineParams{
				{
					LineOrder:       1,
					ItemID:          itemID,
					DebitAccountID:  arAccID,
					CreditAccountID: revAccID,
					Quantity:        decimal.NewFromInt(10),
					UnitPriceVND:    decimal.NewFromInt(2000000), // 20M gross
					DiscountRate:    decimal.NewFromInt(5),       // 1M discount
					VATRate:         decimal.NewFromInt(10),      // 1.9M VAT
					Note:            "Xuất hàng sỉ",
				},
			},
		})
		require.NoError(t, err)

		err = salesRepo.SaveSalesInvoice(ctx, si)
		require.NoError(t, err)

		fetched, err := salesRepo.GetSalesInvoiceByID(ctx, si.ID)
		require.NoError(t, err)
		assert.Equal(t, si.ID, fetched.ID)
		assert.Equal(t, "20000000", fetched.SubtotalVND.String())
		assert.Equal(t, "1000000", fetched.DiscountVND.String())
		assert.Equal(t, "1900000", fetched.VATAmountVND.String())
		assert.Equal(t, "20900000", fetched.TotalAmountVND.String())
		require.Len(t, fetched.Lines, 1)
		assert.Equal(t, "00000888", fetched.InvoiceNo)

		fetchedByV, err := salesRepo.GetSalesInvoiceByVoucherID(ctx, v.ID)
		require.NoError(t, err)
		assert.Equal(t, si.ID, fetchedByV.ID)

		// Test UpdateEInvoiceStatus
		cqtCode := "CQT-M-999"
		err = salesRepo.UpdateEInvoiceStatus(ctx, si.ID, sales.EInvoiceStatusCQTAccepted, &cqtCode)
		require.NoError(t, err)

		updated, err := salesRepo.GetSalesInvoiceByID(ctx, si.ID)
		require.NoError(t, err)
		assert.Equal(t, sales.EInvoiceStatusCQTAccepted, updated.EInvoiceStatus)
		assert.Equal(t, &cqtCode, updated.EInvoiceCodeCQT)
	})
}

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
	"fingo/internal/domain/purchase"
)

func TestPurchaseRepo_LiveMariaDB(t *testing.T) {
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
		companyID = "comp-purchase-test"
		_, err = db.ExecContext(ctx, "INSERT INTO company_profile (id, tax_code, legal_name, address, legal_representative, chief_accountant, tax_authority_code, tax_authority_name, created_at, updated_at) VALUES (?, '0101243153', 'FinGo Purchase Test', 'Hanoi', 'GD', 'KTT', '101', 'CCT', ?, ?)", companyID, now, now)
		require.NoError(t, err)
	}

	getAccountID := func(code, name, nature, category string) string {
		var id string
		err := db.QueryRowContext(ctx, "SELECT id FROM accounts WHERE company_profile_id = ? AND code = ?", companyID, code).Scan(&id)
		if err == nil && id != "" {
			return id
		}
		id = "acc-pr-" + code
		_, _ = db.ExecContext(ctx, "INSERT INTO accounts (id, company_profile_id, code, name, nature, category, is_leaf, is_active, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, 1, 1, NOW(), NOW())", id, companyID, code, name, nature, category)
		return id
	}

	apAccID := getAccountID("331", "Phải trả người bán", "CREDIT", "LIABILITY")
	invAccID := getAccountID("1561", "Hàng hóa tồn kho", "DEBIT", "ASSET")
	vatAccID := getAccountID("13311", "Thuế GTGT đầu vào", "DEBIT", "ASSET")

	// Ensure a vendor exists
	var vendorID string
	err = db.QueryRowContext(ctx, "SELECT id FROM vendors WHERE company_profile_id = ? LIMIT 1", companyID).Scan(&vendorID)
	if err != nil || vendorID == "" {
		vendorID = uuid.New().String()
		_, err = db.ExecContext(ctx, `INSERT INTO vendors (id, company_profile_id, code, name, is_active, created_at, updated_at)
			VALUES (?, ?, 'VEND-TEST-01', 'Nhà cung cấp Thăng Long', TRUE, ?, ?)`, vendorID, companyID, now, now)
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
			VALUES (?, ?, 'ITEM-TEST-01', 'Mặt hàng mẫu', 'MERCHANDISE', ?, TRUE, ?, ?)`, itemID, companyID, uomID, now, now)
		require.NoError(t, err)
	}

	voucherRepo := repository.NewVoucherRepo(db)
	purchaseRepo := repository.NewPurchaseRepo(db)

	t.Run("Save and Retrieve Purchase Invoice with Lines", func(t *testing.T) {
		vID := uuid.New().String()
		vNo := "MH-" + vID[:8]
		v, err := gl.NewVoucher(gl.CreateVoucherParams{
			ID:               vID,
			CompanyProfileID: companyID,
			VoucherNo:        vNo,
			VoucherDate:      now,
			PostedDate:       now,
			VoucherType:      gl.VoucherTypePurchase,
			Description:      "Mua hàng hóa nhập kho hóa đơn NCC",
			CreatedBy:        "tester",
			Lines: []gl.VoucherLine{
				{
					ID:                uuid.New().String(),
					LineOrder:         1,
					DebitAccountID:    invAccID,
					CreditAccountID:   apAccID,
					DebitAccountCode:  "1561",
					CreditAccountCode: "331",
					AmountVND:         decimal.NewFromInt(10000000),
					Note:              "Tiền hàng",
				},
				{
					ID:                uuid.New().String(),
					LineOrder:         2,
					DebitAccountID:    vatAccID,
					CreditAccountID:   apAccID,
					DebitAccountCode:  "13311",
					CreditAccountCode: "331",
					AmountVND:         decimal.NewFromInt(1000000),
					Note:              "Thuế GTGT 10%",
				},
			},
		})
		require.NoError(t, err)
		require.NoError(t, voucherRepo.SaveVoucher(ctx, v))

		pi, err := purchase.NewPurchaseInvoice(purchase.CreatePurchaseInvoiceParams{
			ID:                uuid.New().String(),
			VoucherID:         v.ID,
			CompanyProfileID:  companyID,
			VendorID:          vendorID,
			InvoiceTemplate:   "1",
			InvoiceSeries:     "C26TAA",
			InvoiceNo:         "00000999",
			InvoiceDate:       now,
			DueDate:           now.AddDate(0, 1, 0),
			IsStockInwardAuto: true,
			Lines: []purchase.CreatePurchaseInvoiceLineParams{
				{
					LineOrder:       1,
					ItemID:          itemID,
					DebitAccountID:  invAccID,
					CreditAccountID: apAccID,
					Quantity:        decimal.NewFromInt(10),
					UnitPriceVND:    decimal.NewFromInt(1000000),
					VATRate:         decimal.NewFromInt(10),
					Note:            "Nhập kho lô hàng số 1",
				},
			},
		})
		require.NoError(t, err)

		err = purchaseRepo.SavePurchaseInvoice(ctx, pi)
		require.NoError(t, err)

		fetched, err := purchaseRepo.GetPurchaseInvoiceByID(ctx, pi.ID)
		require.NoError(t, err)
		assert.Equal(t, pi.ID, fetched.ID)
		assert.Equal(t, "10000000", fetched.SubtotalVND.String())
		assert.Equal(t, "1000000", fetched.VATAmountVND.String())
		assert.Equal(t, "11000000", fetched.TotalAmountVND.String())
		require.Len(t, fetched.Lines, 1)
		assert.Equal(t, "00000999", fetched.InvoiceNo)

		fetchedByV, err := purchaseRepo.GetPurchaseInvoiceByVoucherID(ctx, v.ID)
		require.NoError(t, err)
		assert.Equal(t, pi.ID, fetchedByV.ID)

		// Test UpdatePaymentStatus
		err = purchaseRepo.UpdatePaymentStatus(ctx, pi.ID, purchase.PaymentStatusPartiallyPaid, decimal.NewFromInt(5000000))
		require.NoError(t, err)

		updated, err := purchaseRepo.GetPurchaseInvoiceByID(ctx, pi.ID)
		require.NoError(t, err)
		assert.Equal(t, purchase.PaymentStatusPartiallyPaid, updated.PaymentStatus)
		assert.Equal(t, "5000000", updated.PaidAmountVND.String())
	})
}

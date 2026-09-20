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
	"fingo/internal/domain/cash"
	"fingo/internal/domain/gl"
)

func TestCashRepo_LiveMariaDB(t *testing.T) {
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
		companyID = "comp-cash-test"
		_, err = db.ExecContext(ctx, "INSERT INTO company_profile (id, tax_code, legal_name, address, legal_representative, chief_accountant, tax_authority_code, tax_authority_name, created_at, updated_at) VALUES (?, '0101243151', 'FinGo Cash Test', 'Hanoi', 'GD', 'KTT', '101', 'CCT', ?, ?)", companyID, now, now)
		require.NoError(t, err)
	}

	getAccountID := func(code, name, nature, category string) string {
		var id string
		err := db.QueryRowContext(ctx, "SELECT id FROM accounts WHERE code = ?", code).Scan(&id)
		if err == nil {
			return id
		}
		id = uuid.New().String()
		_, err = db.ExecContext(ctx, `INSERT INTO accounts (id, code, name, nature, category, is_leaf, is_active, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, TRUE, TRUE, ?, ?)`, id, code, name, nature, category, now, now)
		require.NoError(t, err)
		return id
	}

	cashAccID := getAccountID("1111", "Tiền mặt VND", "DEBIT", "ASSET")
	bankAccID := getAccountID("1121", "Tiền gửi ngân hàng VND", "DEBIT", "ASSET")

	voucherRepo := repository.NewVoucherRepo(db)
	cashRepo := repository.NewCashRepo(db)

	t.Run("Create and retrieve Cash Receipt linked to voucher", func(t *testing.T) {
		vID := uuid.New().String()
		vNo := "PT-" + vID[:8]
		v, err := gl.NewVoucher(gl.CreateVoucherParams{
			ID:               vID,
			CompanyProfileID: companyID,
			VoucherNo:        vNo,
			VoucherDate:      now,
			PostedDate:       now,
			VoucherType:      gl.VoucherTypeCashReceipt,
			Description:      "Thu tiền bán hàng nhập quỹ",
			CreatedBy:        "tester",
			Lines: []gl.VoucherLine{
				{
					ID:                uuid.New().String(),
					LineOrder:         1,
					DebitAccountID:    cashAccID,
					CreditAccountID:   bankAccID,
					DebitAccountCode:  "1111",
					CreditAccountCode: "1121",
					AmountVND:         decimal.NewFromInt(10000000),
					Note:              "Thu nợ bằng tiền mặt",
				},
			},
		})
		require.NoError(t, err)
		require.NoError(t, voucherRepo.SaveVoucher(ctx, v))

		cr, err := cash.NewCashReceipt(cash.CreateCashReceiptParams{
			ID:                    uuid.New().String(),
			VoucherID:             v.ID,
			CompanyProfileID:      companyID,
			PayerName:             "Nguyễn Văn An",
			PayerAddress:          "Hà Nội",
			Reason:                "Thu nợ khách hàng",
			CashAccountID:         cashAccID,
			TotalAmountVND:        decimal.NewFromInt(10000000),
			AccompanyingDocuments: "Hóa đơn số 001",
		})
		require.NoError(t, err)

		err = cashRepo.SaveCashReceipt(ctx, cr)
		require.NoError(t, err)

		fetched, err := cashRepo.GetCashReceiptByID(ctx, cr.ID)
		require.NoError(t, err)
		assert.Equal(t, cr.ID, fetched.ID)
		assert.Equal(t, cr.PayerName, fetched.PayerName)
		assert.Equal(t, "10000000", fetched.TotalAmountVND.String())

		fetchedByV, err := cashRepo.GetCashReceiptByVoucherID(ctx, v.ID)
		require.NoError(t, err)
		assert.Equal(t, cr.ID, fetchedByV.ID)
	})

	t.Run("Create and retrieve Cash Payment linked to voucher", func(t *testing.T) {
		vID := uuid.New().String()
		vNo := "PC-" + vID[:8]
		v, err := gl.NewVoucher(gl.CreateVoucherParams{
			ID:               vID,
			CompanyProfileID: companyID,
			VoucherNo:        vNo,
			VoucherDate:      now,
			PostedDate:       now,
			VoucherType:      gl.VoucherTypeCashPayment,
			Description:      "Chi tiền mặt thanh toán dịch vụ",
			CreatedBy:        "tester",
			Lines: []gl.VoucherLine{
				{
					ID:                uuid.New().String(),
					LineOrder:         1,
					DebitAccountID:    bankAccID,
					CreditAccountID:   cashAccID,
					DebitAccountCode:  "1121",
					CreditAccountCode: "1111",
					AmountVND:         decimal.NewFromInt(6000000),
					Note:              "Chi tiền khẩn cấp",
				},
			},
		})
		require.NoError(t, err)
		require.NoError(t, voucherRepo.SaveVoucher(ctx, v))

		cp, err := cash.NewCashPayment(cash.CreateCashPaymentParams{
			ID:                    uuid.New().String(),
			VoucherID:             v.ID,
			CompanyProfileID:      companyID,
			ReceiverName:          "Công ty NCC Beta",
			ReceiverAddress:       "Đà Nẵng",
			Reason:                "Chi trả tiền hàng hóa đơn số 99",
			CashAccountID:         cashAccID,
			TotalAmountVND:            decimal.NewFromInt(6000000),
			RequiresNonCashCompliance: true,
			IsNonCashOverride:         true,
			ConfirmationCode:          cash.NonCashConfirmationCode,
			OverrideReason:            "Giám đốc duyệt chi gấp",
			AccompanyingDocuments:     "Phiếu đề nghị thanh toán số 45",
		})
		require.NoError(t, err)

		err = cashRepo.SaveCashPayment(ctx, cp)
		require.NoError(t, err)

		fetched, err := cashRepo.GetCashPaymentByID(ctx, cp.ID)
		require.NoError(t, err)
		assert.Equal(t, cp.ID, fetched.ID)
		assert.Equal(t, cp.ReceiverName, fetched.ReceiverName)
		assert.True(t, fetched.IsNonCashOverride)
		assert.Equal(t, "Giám đốc duyệt chi gấp", fetched.OverrideReason)

		fetchedByV, err := cashRepo.GetCashPaymentByVoucherID(ctx, v.ID)
		require.NoError(t, err)
		assert.Equal(t, cp.ID, fetchedByV.ID)
	})
}

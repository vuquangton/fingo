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

func TestBankRepo_LiveMariaDB(t *testing.T) {
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
		companyID = "comp-bank-test"
		_, err = db.ExecContext(ctx, "INSERT INTO company_profile (id, tax_code, legal_name, address, legal_representative, chief_accountant, tax_authority_code, tax_authority_name, created_at, updated_at) VALUES (?, '0101243152', 'FinGo Bank Test', 'Hanoi', 'GD', 'KTT', '101', 'CCT', ?, ?)", companyID, now, now)
		require.NoError(t, err)
	}

	getAccountID := func(code, name, nature, category string) string {
		var id string
		err := db.QueryRowContext(ctx, "SELECT id FROM accounts WHERE company_profile_id = ? AND code = ?", companyID, code).Scan(&id)
		if err == nil && id != "" {
			return id
		}
		id = "acc-br-" + code
		_, _ = db.ExecContext(ctx, "INSERT INTO accounts (id, company_profile_id, code, name, nature, category, is_leaf, is_active, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, 1, 1, NOW(), NOW())", id, companyID, code, name, nature, category)
		return id
	}

	bankGLAccID := getAccountID("1121", "Tiền gửi ngân hàng VND", "DEBIT", "ASSET")
	arAccID := getAccountID("131", "Phải thu khách hàng", "DEBIT", "ASSET")

	// Ensure currency VND exists
	_, _ = db.ExecContext(ctx, `INSERT IGNORE INTO currencies (company_profile_id, code, name, symbol, exchange_rate, is_base, is_active, created_at, updated_at)
		VALUES (?, 'VND', 'Việt Nam Đồng', 'đ', 1.0, TRUE, TRUE, ?, ?)`, companyID, now, now)

	// Ensure a bank_account exists
	var bankAccountID string
	err = db.QueryRowContext(ctx, "SELECT id FROM bank_accounts WHERE company_profile_id = ? LIMIT 1", companyID).Scan(&bankAccountID)
	if err != nil || bankAccountID == "" {
		bankAccountID = uuid.New().String()
		_, err = db.ExecContext(ctx, `INSERT INTO bank_accounts (id, company_profile_id, account_number, bank_name, bank_code, currency_code, gl_account_id, is_active, created_at, updated_at)
			VALUES (?, ?, '1012345678', 'Vietcombank', 'VCB', 'VND', ?, TRUE, ?, ?)`, bankAccountID, companyID, bankGLAccID, now, now)
		require.NoError(t, err)
	}

	voucherRepo := repository.NewVoucherRepo(db)
	bankRepo := repository.NewBankRepo(db)

	t.Run("Create and retrieve Bank Transaction (Credit Advice) linked to voucher", func(t *testing.T) {
		vID := uuid.New().String()
		vNo := "BC-" + vID[:8]
		v, err := gl.NewVoucher(gl.CreateVoucherParams{
			ID:               vID,
			CompanyProfileID: companyID,
			VoucherNo:        vNo,
			VoucherDate:      now,
			PostedDate:       now,
			VoucherType:      gl.VoucherTypeBankReceipt,
			Description:      "Khách hàng chuyển khoản thanh toán",
			CreatedBy:        "tester",
			Lines: []gl.VoucherLine{
				{
					ID:                uuid.New().String(),
					LineOrder:         1,
					DebitAccountID:    bankGLAccID,
					CreditAccountID:   arAccID,
					DebitAccountCode:  "1121",
					CreditAccountCode: "131",
					AmountVND:         decimal.NewFromInt(50000000),
					Note:              "Báo có VCB",
				},
			},
		})
		require.NoError(t, err)
		require.NoError(t, voucherRepo.SaveVoucher(ctx, v))

		bt, err := cash.NewBankTransaction(cash.CreateBankTransactionParams{
			ID:               uuid.New().String(),
			VoucherID:        v.ID,
			CompanyProfileID: companyID,
			BankAccountID:    bankAccountID,
			TransactionType:  cash.BankTransactionTypeCreditAdvice,
			Counterparty: cash.BankCounterpartyInfo{
				AccountNo: "9988776655",
				BankName:  "Techcombank",
				Name:      "Công ty CP Sao Mai",
				Reference: "REF-VCB-2026-001",
			},
			FeeAmountVND:    decimal.Zero,
			VatFeeAmountVND: decimal.Zero,
			TotalAmountVND:  decimal.NewFromInt(50000000),
		})
		require.NoError(t, err)

		err = bankRepo.SaveBankTransaction(ctx, bt)
		require.NoError(t, err)

		fetched, err := bankRepo.GetBankTransactionByID(ctx, bt.ID)
		require.NoError(t, err)
		assert.Equal(t, bt.ID, fetched.ID)
		assert.Equal(t, cash.BankTransactionTypeCreditAdvice, fetched.TransactionType)
		assert.Equal(t, "50000000", fetched.TotalAmountVND.String())
		assert.Equal(t, "REF-VCB-2026-001", fetched.Counterparty.Reference)

		fetchedByV, err := bankRepo.GetBankTransactionByVoucherID(ctx, v.ID)
		require.NoError(t, err)
		assert.Equal(t, bt.ID, fetchedByV.ID)
	})
}

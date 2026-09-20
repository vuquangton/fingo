package cash_test

import (
	"context"
	"database/sql"
	"io"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"fingo/internal/domain/cash"
	"fingo/internal/domain/catalog"
	"fingo/internal/domain/gl"
	spineDomain "fingo/internal/domain/spine"
	usecaseCash "fingo/internal/usecase/cash"
	usecaseGL "fingo/internal/usecase/gl"
	"fingo/pkg/logger"
)

// Mock Bank Account Repo
type mockBankAccountRepo struct {
	accounts map[string]*catalog.BankAccount
}

func (m *mockBankAccountRepo) GetBankAccountByID(ctx context.Context, id string) (*catalog.BankAccount, error) {
	ba, ok := m.accounts[id]
	if !ok {
		return nil, sql.ErrNoRows
	}
	return ba, nil
}

func setupBankTestEnv() (*usecaseCash.BankUseCase, *cash.BankRepositoryStub) {
	bankRepo := cash.NewBankRepositoryStub()
	glRepo := &mockGLRepo{vouchers: make(map[string]*gl.Voucher)}
	accRepo := &mockAccountRepo{
		accounts: map[string]*spineDomain.Account{
			"acc-1121":  {ID: "acc-1121", Code: "1121", Name: "Tiền gửi ngân hàng VND", IsLeaf: true, IsActive: true},
			"acc-1122":  {ID: "acc-1122", Code: "1122", Name: "Tiền gửi ngân hàng Ngoại tệ", IsLeaf: true, IsActive: true},
			"acc-131":   {ID: "acc-131", Code: "131", Name: "Phải thu của khách hàng", IsLeaf: true, IsActive: true},
			"acc-331":   {ID: "acc-331", Code: "331", Name: "Phải trả cho người bán", IsLeaf: true, IsActive: true},
			"acc-6421":  {ID: "acc-6421", Code: "6421", Name: "Chi phí dịch vụ mua ngoài", IsLeaf: true, IsActive: true},
			"acc-13311": {ID: "acc-13311", Code: "13311", Name: "Thuế GTGT đầu vào được khấu trừ", IsLeaf: true, IsActive: true},
		},
	}
	bankAccRepo := &mockBankAccountRepo{
		accounts: map[string]*catalog.BankAccount{
			"ba-vcb-01": {
				ID:            "ba-vcb-01",
				AccountNumber: "1012345678",
				BankName:      "Vietcombank",
				GLAccountID:   "acc-1121",
				CurrencyCode:  "VND",
				IsActive:      true,
			},
			"ba-vcb-usd": {
				ID:            "ba-vcb-usd",
				AccountNumber: "1012345999",
				BankName:      "Vietcombank USD",
				GLAccountID:   "acc-1122",
				CurrencyCode:  "USD",
				IsActive:      true,
			},
		},
	}
	compRepo := &mockCompanyRepo{}
	log := logger.New(logger.Config{Writer: io.Discard})

	glUC := usecaseGL.NewGLUseCase(glRepo, compRepo, accRepo, log)
	bankUC := usecaseCash.NewBankUseCase(bankRepo, bankAccRepo, accRepo, glUC, log)
	return bankUC, bankRepo
}

func TestBankUseCase_Workflow(t *testing.T) {
	uc, _ := setupBankTestEnv()
	ctx := context.Background()
	txDate := time.Date(2026, 3, 20, 0, 0, 0, 0, time.UTC)
	costCenter := "CC_FINANCE"

	t.Run("Create valid Credit Advice (Báo có VCB)", func(t *testing.T) {
		cmd := usecaseCash.CreateBankCreditAdviceCommand{
			CompanyProfileID: "comp-01",
			VoucherNo:        "BC-2026-0001",
			VoucherDate:      txDate,
			PostedDate:       txDate,
			BankAccountID:    "ba-vcb-01",
			Counterparty: cash.BankCounterpartyInfo{
				AccountNo: "9988776655",
				BankName:  "Techcombank",
				Name:      "Công ty CP Sao Mai",
				Reference: "REF-VCB-001",
			},
			Description:     "Khách hàng thanh toán tiền hàng",
			TotalAmountVND:  decimal.NewFromInt(50000000),
			CreditAccountID: "acc-131",
			CreatedBy:       "ktt",
		}

		bt, v, err := uc.CreateCreditAdvice(ctx, cmd)
		require.NoError(t, err)
		assert.NotEmpty(t, bt.ID)
		assert.Equal(t, cash.BankTransactionTypeCreditAdvice, bt.TransactionType)
		assert.Equal(t, "50000000", bt.TotalAmountVND.String())
		assert.Equal(t, gl.VoucherTypeBankReceipt, v.VoucherType)
		assert.Len(t, v.Lines, 1)

		// Test Get
		fetched, err := uc.GetBankTransaction(ctx, bt.ID)
		require.NoError(t, err)
		assert.Equal(t, bt.ID, fetched.ID)
	})

	t.Run("Create valid Transfer Order with Banking Fee and VAT Fee auto-split", func(t *testing.T) {
		vendorID := "vendor-01"
		cmd := usecaseCash.CreateBankPaymentCommand{
			CompanyProfileID: "comp-01",
			VoucherNo:        "UNC-2026-0001",
			VoucherDate:      txDate,
			PostedDate:       txDate,
			BankAccountID:    "ba-vcb-01",
			TransactionType:  cash.BankTransactionTypeTransferOrder,
			Counterparty: cash.BankCounterpartyInfo{
				AccountNo: "1122334455",
				BankName:  "BIDV",
				Name:      "Công ty TNHH Vật Tư Kim Khí",
				Reference: "UNC-VCB-001",
			},
			Description:        "Chuyển khoản thanh toán tiền hàng NCC",
			PrincipalAmountVND: decimal.NewFromInt(100000000),
			DebitAccountID:     "acc-331",
			VendorID:           &vendorID,
			FeeAmountVND:       decimal.NewFromInt(20000),
			FeeAccountID:       "acc-6421",
			VatFeeAmountVND:    decimal.NewFromInt(2000),
			VatFeeAccountID:    "acc-13311",
			CostCenterID:       &costCenter,
			CreatedBy:          "ktt",
		}

		bt, v, err := uc.CreateBankPayment(ctx, cmd)
		require.NoError(t, err)
		assert.NotEmpty(t, bt.ID)
		assert.Equal(t, cash.BankTransactionTypeTransferOrder, bt.TransactionType)
		assert.Equal(t, "100022000", bt.TotalAmountVND.String())
		assert.Equal(t, gl.VoucherTypeBankPayment, v.VoucherType)

		// Assert 3 voucher lines: Principal (331), Bank fee (6421), VAT fee (13311)
		require.Len(t, v.Lines, 3)
		assert.Equal(t, "100000000", v.Lines[0].AmountVND.String())
		assert.Equal(t, "acc-331", v.Lines[0].DebitAccountID)
		assert.Equal(t, "20000", v.Lines[1].AmountVND.String())
		assert.Equal(t, "acc-6421", v.Lines[1].DebitAccountID)
		assert.Equal(t, "2000", v.Lines[2].AmountVND.String())
		assert.Equal(t, "acc-13311", v.Lines[2].DebitAccountID)
	})

	t.Run("Create Credit Advice with compound split lines", func(t *testing.T) {
		cmd := usecaseCash.CreateBankCreditAdviceCommand{
			CompanyProfileID: "comp-01",
			VoucherNo:        "BC-2026-SPLIT",
			VoucherDate:      txDate,
			PostedDate:       txDate,
			BankAccountID:    "ba-vcb-01",
			Description:      "Báo có thu tiền nhiều hóa đơn",
			TotalAmountVND:   decimal.NewFromInt(30000000),
			Lines: []usecaseCash.CreateBankCreditAdviceLineCommand{
				{CreditAccountID: "acc-131", AmountVND: decimal.NewFromInt(15000000), Note: "HD 01"},
				{CreditAccountID: "acc-131", AmountVND: decimal.NewFromInt(15000000), Note: "HD 02"},
			},
			CreatedBy: "ktt",
		}

		bt, v, err := uc.CreateCreditAdvice(ctx, cmd)
		require.NoError(t, err)
		assert.Equal(t, "30000000", bt.TotalAmountVND.String())
		assert.Len(t, v.Lines, 2)
	})

	t.Run("Create Bank Payment with compound debit lines", func(t *testing.T) {
		cmd := usecaseCash.CreateBankPaymentCommand{
			CompanyProfileID:   "comp-01",
			VoucherNo:          "BN-2026-SPLIT",
			VoucherDate:        txDate,
			PostedDate:         txDate,
			BankAccountID:      "ba-vcb-01",
			TransactionType:    cash.BankTransactionTypeDebitAdvice,
			Description:        "Báo nợ ngân hàng chi phí",
			PrincipalAmountVND: decimal.NewFromInt(20000000),
			CostCenterID:       &costCenter,
			Lines: []usecaseCash.CreateBankPaymentLineCommand{
				{DebitAccountID: "acc-6421", AmountVND: decimal.NewFromInt(10000000), CostCenterID: &costCenter, Note: "Chi phi 1"},
				{DebitAccountID: "acc-6421", AmountVND: decimal.NewFromInt(10000000), CostCenterID: &costCenter, Note: "Chi phi 2"},
			},
			CreatedBy: "ktt",
		}

		bt, v, err := uc.CreateBankPayment(ctx, cmd)
		require.NoError(t, err)
		assert.Equal(t, "20000000", bt.TotalAmountVND.String())
		assert.Len(t, v.Lines, 2)
	})

	t.Run("Reject line sum mismatch", func(t *testing.T) {
		cmd := usecaseCash.CreateBankCreditAdviceCommand{
			CompanyProfileID: "comp-01",
			VoucherNo:        "BC-2026-MISMATCH",
			VoucherDate:      txDate,
			PostedDate:       txDate,
			BankAccountID:    "ba-vcb-01",
			TotalAmountVND:   decimal.NewFromInt(30000000),
			Lines: []usecaseCash.CreateBankCreditAdviceLineCommand{
				{CreditAccountID: "acc-131", AmountVND: decimal.NewFromInt(10000000)}, // Sum 10M != Total 30M
			},
		}
		_, _, err := uc.CreateCreditAdvice(ctx, cmd)
		require.ErrorIs(t, err, cash.ErrPrincipalMismatch)
	})

	t.Run("Reject CREDIT_ADVICE in CreateBankPayment", func(t *testing.T) {
		cmd := usecaseCash.CreateBankPaymentCommand{
			CompanyProfileID:   "comp-01",
			BankAccountID:      "ba-vcb-01",
			TransactionType:    cash.BankTransactionTypeCreditAdvice,
			PrincipalAmountVND: decimal.NewFromInt(1000000),
		}
		_, _, err := uc.CreateBankPayment(ctx, cmd)
		require.ErrorIs(t, err, cash.ErrInvalidTransactionType)
	})

	t.Run("Reject missing fee account when fee is positive", func(t *testing.T) {
		cmd := usecaseCash.CreateBankPaymentCommand{
			CompanyProfileID:   "comp-01",
			BankAccountID:      "ba-vcb-01",
			TransactionType:    cash.BankTransactionTypeTransferOrder,
			PrincipalAmountVND: decimal.NewFromInt(1000000),
			FeeAmountVND:       decimal.NewFromInt(10000),
			FeeAccountID:       "", // missing
		}
		_, _, err := uc.CreateBankPayment(ctx, cmd)
		require.ErrorIs(t, err, cash.ErrFeeAccountRequired)
	})

	t.Run("Reject missing VAT fee account when VAT fee is positive", func(t *testing.T) {
		cmd := usecaseCash.CreateBankPaymentCommand{
			CompanyProfileID:   "comp-01",
			BankAccountID:      "ba-vcb-01",
			TransactionType:    cash.BankTransactionTypeTransferOrder,
			PrincipalAmountVND: decimal.NewFromInt(1000000),
			FeeAmountVND:       decimal.NewFromInt(10000),
			FeeAccountID:       "acc-6421",
			VatFeeAmountVND:    decimal.NewFromInt(1000),
			VatFeeAccountID:    "", // missing
		}
		_, _, err := uc.CreateBankPayment(ctx, cmd)
		require.ErrorIs(t, err, cash.ErrVatFeeAccountRequired)
	})

	t.Run("Enforce Multi-Currency Seam (INV-OPS-06)", func(t *testing.T) {
		// 1121 must use VND
		cmdVNDMismatch := usecaseCash.CreateBankCreditAdviceCommand{
			CompanyProfileID: "comp-01",
			BankAccountID:    "ba-vcb-01", // linked to 1121
			CurrencyCode:     "USD",       // Mismatch
			TotalAmountVND:   decimal.NewFromInt(1000000),
		}
		_, _, err := uc.CreateCreditAdvice(ctx, cmdVNDMismatch)
		require.ErrorIs(t, err, cash.ErrBaseCurrencyMismatch)

		// 1122 must use foreign currency with exchange rate > 0
		cmdUSDMismatch := usecaseCash.CreateBankCreditAdviceCommand{
			CompanyProfileID: "comp-01",
			BankAccountID:    "ba-vcb-usd", // linked to 1122
			CurrencyCode:     "USD",
			ExchangeRate:     decimal.Zero, // Missing rate
			TotalAmountVND:   decimal.NewFromInt(1000000),
		}
		_, _, err = uc.CreateCreditAdvice(ctx, cmdUSDMismatch)
		require.ErrorIs(t, err, cash.ErrForeignCurrencyMismatch)
	})

	t.Run("Bank account not found returns error", func(t *testing.T) {
		cmdCA := usecaseCash.CreateBankCreditAdviceCommand{
			CompanyProfileID: "comp-01",
			BankAccountID:    "ba-missing",
		}
		_, _, err := uc.CreateCreditAdvice(ctx, cmdCA)
		require.ErrorIs(t, err, usecaseCash.ErrBankAccountNotFound)
	})

	t.Run("Idempotent submission returns existing record", func(t *testing.T) {
		idemp := "idemp-bank-01"
		cmd := usecaseCash.CreateBankCreditAdviceCommand{
			CompanyProfileID: "comp-01",
			VoucherNo:        "BC-2026-IDEMP",
			VoucherDate:      txDate,
			PostedDate:       txDate,
			BankAccountID:    "ba-vcb-01",
			Description:      "Báo có",
			TotalAmountVND:   decimal.NewFromInt(1000000),
			CreditAccountID:  "acc-131",
			IdempotencyKey:   &idemp,
			CreatedBy:        "ktt",
		}

		bt1, v1, err := uc.CreateCreditAdvice(ctx, cmd)
		require.NoError(t, err)

		bt2, v2, err := uc.CreateCreditAdvice(ctx, cmd)
		require.NoError(t, err)
		assert.Equal(t, bt1.ID, bt2.ID)
		assert.Equal(t, v1.ID, v2.ID)
	})

	t.Run("GetBankTransaction not found", func(t *testing.T) {
		_, err := uc.GetBankTransaction(ctx, "non-existent")
		require.ErrorIs(t, err, usecaseCash.ErrBankTransactionNotFound)
	})
}

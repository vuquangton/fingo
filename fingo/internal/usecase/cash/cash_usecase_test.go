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
	"fingo/internal/domain/gl"
	spineDomain "fingo/internal/domain/spine"
	"fingo/internal/domain/system"
	usecaseCash "fingo/internal/usecase/cash"
	usecaseGL "fingo/internal/usecase/gl"
	"fingo/pkg/logger"
)

// Mock Cash Repository
type mockCashRepo struct {
	receipts          map[string]*cash.CashReceipt
	receiptsByVoucher map[string]*cash.CashReceipt
	payments          map[string]*cash.CashPayment
	paymentsByVoucher map[string]*cash.CashPayment
}

func newMockCashRepo() *mockCashRepo {
	return &mockCashRepo{
		receipts:          make(map[string]*cash.CashReceipt),
		receiptsByVoucher: make(map[string]*cash.CashReceipt),
		payments:          make(map[string]*cash.CashPayment),
		paymentsByVoucher: make(map[string]*cash.CashPayment),
	}
}

func (m *mockCashRepo) SaveCashReceipt(ctx context.Context, cr *cash.CashReceipt) error {
	m.receipts[cr.ID] = cr
	m.receiptsByVoucher[cr.VoucherID] = cr
	return nil
}

func (m *mockCashRepo) GetCashReceiptByID(ctx context.Context, id string) (*cash.CashReceipt, error) {
	cr, ok := m.receipts[id]
	if !ok {
		return nil, sql.ErrNoRows
	}
	return cr, nil
}

func (m *mockCashRepo) GetCashReceiptByVoucherID(ctx context.Context, voucherID string) (*cash.CashReceipt, error) {
	cr, ok := m.receiptsByVoucher[voucherID]
	if !ok {
		return nil, sql.ErrNoRows
	}
	return cr, nil
}

func (m *mockCashRepo) SaveCashPayment(ctx context.Context, cp *cash.CashPayment) error {
	m.payments[cp.ID] = cp
	m.paymentsByVoucher[cp.VoucherID] = cp
	return nil
}

func (m *mockCashRepo) GetCashPaymentByID(ctx context.Context, id string) (*cash.CashPayment, error) {
	cp, ok := m.payments[id]
	if !ok {
		return nil, sql.ErrNoRows
	}
	return cp, nil
}

func (m *mockCashRepo) GetCashPaymentByVoucherID(ctx context.Context, voucherID string) (*cash.CashPayment, error) {
	cp, ok := m.paymentsByVoucher[voucherID]
	if !ok {
		return nil, sql.ErrNoRows
	}
	return cp, nil
}

// Mock GL Repo
type mockGLRepo struct {
	vouchers map[string]*gl.Voucher
}

func (m *mockGLRepo) SaveVoucher(ctx context.Context, v *gl.Voucher) error {
	m.vouchers[v.ID] = v
	return nil
}

func (m *mockGLRepo) GetVoucherByID(ctx context.Context, id string) (*gl.Voucher, error) {
	v, ok := m.vouchers[id]
	if !ok {
		return nil, sql.ErrNoRows
	}
	return v, nil
}

func (m *mockGLRepo) GetVoucherByNo(ctx context.Context, companyID string, vType gl.VoucherType, voucherNo string) (*gl.Voucher, error) {
	for _, v := range m.vouchers {
		if v.CompanyProfileID == companyID && v.VoucherType == vType && v.VoucherNo == voucherNo {
			return v, nil
		}
	}
	return nil, sql.ErrNoRows
}

func (m *mockGLRepo) GetVoucherByIdempotencyKey(ctx context.Context, companyID, idempotencyKey string) (*gl.Voucher, error) {
	for _, v := range m.vouchers {
		if v.CompanyProfileID == companyID && v.IdempotencyKey != nil && *v.IdempotencyKey == idempotencyKey {
			return v, nil
		}
	}
	return nil, sql.ErrNoRows
}

func (m *mockGLRepo) UpdateVoucherStatus(ctx context.Context, id string, status gl.VoucherStatus) error {
	v, ok := m.vouchers[id]
	if !ok {
		return sql.ErrNoRows
	}
	v.Status = status
	return nil
}

func (m *mockGLRepo) ListVouchersByPeriod(ctx context.Context, companyID string, fromDate, toDate time.Time) ([]gl.Voucher, error) {
	return nil, nil
}

// Mock Account Repo
type mockAccountRepo struct {
	accounts map[string]*spineDomain.Account
}

func (m *mockAccountRepo) GetAccountByID(ctx context.Context, id string) (*spineDomain.Account, error) {
	acc, ok := m.accounts[id]
	if !ok {
		return nil, sql.ErrNoRows
	}
	return acc, nil
}

// Mock Company Repo
type mockCompanyRepo struct{}

func (m *mockCompanyRepo) GetProfile(ctx context.Context) (*system.ProductionCompanyProfile, error) {
	return &system.ProductionCompanyProfile{
		ID:       "comp-01",
		LockDate: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
	}, nil
}

func setupCashTestEnv() (*usecaseCash.CashUseCase, *mockCashRepo) {
	cashRepo := newMockCashRepo()
	glRepo := &mockGLRepo{vouchers: make(map[string]*gl.Voucher)}
	accRepo := &mockAccountRepo{
		accounts: map[string]*spineDomain.Account{
			"acc-1111": {ID: "acc-1111", Code: "1111", Name: "Tiền mặt VND", IsLeaf: true, IsActive: true},
			"acc-1121": {ID: "acc-1121", Code: "1121", Name: "Tiền gửi ngân hàng VND", IsLeaf: true, IsActive: true},
			"acc-131":  {ID: "acc-131", Code: "131", Name: "Phải thu của khách hàng", IsLeaf: true, IsActive: true},
			"acc-331":  {ID: "acc-331", Code: "331", Name: "Phải trả cho người bán", IsLeaf: true, IsActive: true},
			"acc-141":  {ID: "acc-141", Code: "141", Name: "Tạm ứng", IsLeaf: true, IsActive: true},
			"acc-6421": {ID: "acc-6421", Code: "6421", Name: "Chi phí bán hàng", IsLeaf: true, IsActive: true},
			"acc-non-cash": {ID: "acc-non-cash", Code: "1561", Name: "Hàng hóa", IsLeaf: true, IsActive: true},
		},
	}
	compRepo := &mockCompanyRepo{}
	log := logger.New(logger.Config{Writer: io.Discard})

	glUC := usecaseGL.NewGLUseCase(glRepo, compRepo, accRepo, log)
	cashUC := usecaseCash.NewCashUseCase(cashRepo, glUC, accRepo, log)
	return cashUC, cashRepo
}

func TestCashUseCase_Workflow(t *testing.T) {
	uc, _ := setupCashTestEnv()
	ctx := context.Background()
	txDate := time.Date(2026, 3, 20, 0, 0, 0, 0, time.UTC)
	costCenter := "CC_ADMIN"

	t.Run("Create valid Cash Receipt (01-TT)", func(t *testing.T) {
		cmd := usecaseCash.CreateCashReceiptCommand{
			CompanyProfileID:      "comp-01",
			VoucherNo:             "PT-2026-0001",
			VoucherDate:           txDate,
			PostedDate:            txDate,
			PayerName:             "Công ty Đại Phát",
			PayerAddress:          "Hà Nội",
			Reason:                "Thu tiền khách hàng trả nợ",
			CashAccountID:         "acc-1111",
			CreditAccountID:       "acc-131",
			TotalAmountVND:        decimal.NewFromInt(25000000),
			AccompanyingDocuments: "Hóa đơn GTGT số 123",
			CreatedBy:             "ktt",
		}

		cr, v, err := uc.CreateCashReceipt(ctx, cmd)
		require.NoError(t, err)
		assert.NotEmpty(t, cr.ID)
		assert.Equal(t, "25000000", cr.TotalAmountVND.String())
		assert.Equal(t, gl.VoucherTypeCashReceipt, v.VoucherType)
		assert.Equal(t, "PT-2026-0001", v.VoucherNo)

		// Test Get
		fetched, err := uc.GetCashReceipt(ctx, cr.ID)
		require.NoError(t, err)
		assert.Equal(t, cr.ID, fetched.ID)
	})

	t.Run("Create Cash Receipt fails on invalid cash account (not class 111)", func(t *testing.T) {
		cmd := usecaseCash.CreateCashReceiptCommand{
			CompanyProfileID: "comp-01",
			VoucherNo:        "PT-2026-0002",
			VoucherDate:      txDate,
			PostedDate:       txDate,
			PayerName:        "Công ty Đại Phát",
			Reason:           "Thu tiền gửi ngân hàng",
			CashAccountID:    "acc-1121", // 1121 is bank, not 111
			CreditAccountID:  "acc-131",
			TotalAmountVND:   decimal.NewFromInt(1000000),
			CreatedBy:        "ktt",
		}

		_, _, err := uc.CreateCashReceipt(ctx, cmd)
		require.ErrorIs(t, err, usecaseCash.ErrInvalidCashAccount)
	})

	t.Run("Create valid Cash Payment below 5M without override (02-TT)", func(t *testing.T) {
		cmd := usecaseCash.CreateCashPaymentCommand{
			CompanyProfileID:    "comp-01",
			VoucherNo:           "PC-2026-0001",
			VoucherDate:         txDate,
			PostedDate:          txDate,
			ReceiverName:        "Nguyễn Văn Bình",
			Reason:              "Chi mua văn phòng phẩm tiếp khách",
			CashAccountID:       "acc-1111",
			DebitAccountID:      "acc-6421",
			TotalAmountVND:      decimal.NewFromInt(3500000),
			CostCenterID:              &costCenter,
			RequiresNonCashCompliance: true,
			CreatedBy:                 "ktt",
		}

		cp, v, err := uc.CreateCashPayment(ctx, cmd)
		require.NoError(t, err)
		assert.NotEmpty(t, cp.ID)
		assert.Equal(t, "3500000", cp.TotalAmountVND.String())
		assert.Equal(t, gl.VoucherTypeCashPayment, v.VoucherType)

		// Test Get
		fetched, err := uc.GetCashPayment(ctx, cp.ID)
		require.NoError(t, err)
		assert.Equal(t, cp.ID, fetched.ID)
	})

	t.Run("Reject commercial Cash Payment >= 5M without override (Decree 181/2025)", func(t *testing.T) {
		cmd := usecaseCash.CreateCashPaymentCommand{
			CompanyProfileID:    "comp-01",
			VoucherNo:           "PC-2026-0002",
			VoucherDate:         txDate,
			PostedDate:          txDate,
			ReceiverName:        "Công ty TNHH Song Long",
			Reason:              "Chi trả tiền hàng hóa đơn số 999",
			CashAccountID:       "acc-1111",
			DebitAccountID:      "acc-331",
			TotalAmountVND:      decimal.NewFromInt(12000000),
			RequiresNonCashCompliance: true,
			IsNonCashOverride:         false,
			CreatedBy:                 "ktt",
		}

		_, _, err := uc.CreateCashPayment(ctx, cmd)
		require.ErrorIs(t, err, cash.ErrNonCashThresholdExceeded)
	})

	t.Run("Reject commercial Cash Payment >= 5M with override but wrong confirmation code", func(t *testing.T) {
		cmd := usecaseCash.CreateCashPaymentCommand{
			CompanyProfileID:          "comp-01",
			VoucherNo:                 "PC-2026-0003-NO-CODE",
			VoucherDate:               txDate,
			PostedDate:                txDate,
			ReceiverName:              "Công ty TNHH Song Long",
			Reason:                    "Chi trả tiền hàng",
			CashAccountID:             "acc-1111",
			DebitAccountID:            "acc-331",
			TotalAmountVND:            decimal.NewFromInt(12000000),
			RequiresNonCashCompliance: true,
			IsNonCashOverride:         true,
			ConfirmationCode:          "INVALID",
			OverrideReason:            "Chi khẩn",
			CreatedBy:                 "ktt",
		}

		_, _, err := uc.CreateCashPayment(ctx, cmd)
		require.ErrorIs(t, err, cash.ErrChiefAccountantConfirmationRequired)
	})

	t.Run("Allow commercial Cash Payment >= 5M with override and confirmation code", func(t *testing.T) {
		cmd := usecaseCash.CreateCashPaymentCommand{
			CompanyProfileID:          "comp-01",
			VoucherNo:                 "PC-2026-0003",
			VoucherDate:               txDate,
			PostedDate:                txDate,
			ReceiverName:              "Công ty TNHH Song Long",
			Reason:                    "Chi trả tiền hàng khẩn cấp",
			CashAccountID:             "acc-1111",
			DebitAccountID:            "acc-331",
			TotalAmountVND:            decimal.NewFromInt(12000000),
			RequiresNonCashCompliance: true,
			IsNonCashOverride:         true,
			ConfirmationCode:          cash.NonCashConfirmationCode,
			OverrideReason:            "Giám đốc phê duyệt thanh toán tiền mặt khẩn cấp",
			CreatedBy:                 "ktt",
		}

		cp, v, err := uc.CreateCashPayment(ctx, cmd)
		require.NoError(t, err)
		assert.True(t, cp.IsNonCashOverride)
		assert.Equal(t, "Giám đốc phê duyệt thanh toán tiền mặt khẩn cấp", cp.OverrideReason)
		assert.Equal(t, gl.VoucherTypeCashPayment, v.VoucherType)
	})

	t.Run("Allow non-commercial Cash Payment >= 5M with EmployeeID (TK 141 advance)", func(t *testing.T) {
		empID := "emp-001"
		cmd := usecaseCash.CreateCashPaymentCommand{
			CompanyProfileID:          "comp-01",
			VoucherNo:                 "PC-2026-0004",
			VoucherDate:               txDate,
			PostedDate:                txDate,
			ReceiverName:              "Trần Văn Hùng",
			Reason:                    "Tạm ứng công tác phí miền Trung",
			CashAccountID:             "acc-1111",
			DebitAccountID:            "acc-141",
			EmployeeID:                &empID,
			TotalAmountVND:            decimal.NewFromInt(15000000),
			RequiresNonCashCompliance: false,
			CreatedBy:                 "ktt",
		}

		cp, v, err := uc.CreateCashPayment(ctx, cmd)
		require.NoError(t, err)
		assert.False(t, cp.IsNonCashOverride)
		assert.Equal(t, "15000000", cp.TotalAmountVND.String())
		assert.Equal(t, &empID, v.Lines[0].EmployeeID)
	})

	t.Run("Create Cash Receipt with compound split lines (Revenue + VAT)", func(t *testing.T) {
		cmd := usecaseCash.CreateCashReceiptCommand{
			CompanyProfileID: "comp-01",
			VoucherNo:        "PT-2026-SPLIT",
			VoucherDate:      txDate,
			PostedDate:       txDate,
			PayerName:        "Khách vãng lai",
			Reason:           "Bán hàng thu tiền ngay (5111 + 33311)",
			CashAccountID:    "acc-1111",
			TotalAmountVND:   decimal.NewFromInt(11000000),
			Lines: []usecaseCash.CreateCashReceiptLineCommand{
				{
					CreditAccountID: "acc-131",
					AmountVND:       decimal.NewFromInt(10000000),
					Note:            "Doanh thu bán lẻ",
				},
				{
					CreditAccountID: "acc-131",
					AmountVND:       decimal.NewFromInt(1000000),
					Note:            "Thuế GTGT 10%",
				},
			},
			CreatedBy: "ktt",
		}

		cr, v, err := uc.CreateCashReceipt(ctx, cmd)
		require.NoError(t, err)
		assert.Equal(t, "11000000", cr.TotalAmountVND.String())
		assert.Len(t, v.Lines, 2)
	})

	t.Run("Idempotent submission returns existing Cash Receipt and Voucher", func(t *testing.T) {
		idemp := "idemp-cash-rec-01"
		cmd := usecaseCash.CreateCashReceiptCommand{
			CompanyProfileID: "comp-01",
			VoucherNo:        "PT-2026-IDEMP",
			VoucherDate:      txDate,
			PostedDate:       txDate,
			PayerName:        "Công ty Delta",
			Reason:           "Thu tiền khách hàng",
			CashAccountID:    "acc-1111",
			CreditAccountID:  "acc-131",
			TotalAmountVND:   decimal.NewFromInt(5000000),
			IdempotencyKey:   &idemp,
			CreatedBy:        "ktt",
		}

		cr1, v1, err := uc.CreateCashReceipt(ctx, cmd)
		require.NoError(t, err)

		cr2, v2, err := uc.CreateCashReceipt(ctx, cmd)
		require.NoError(t, err)
		assert.Equal(t, cr1.ID, cr2.ID)
		assert.Equal(t, v1.ID, v2.ID)
	})

	t.Run("Query not found error paths", func(t *testing.T) {
		_, err := uc.GetCashReceipt(ctx, "non-existent")
		require.ErrorIs(t, err, usecaseCash.ErrCashReceiptNotFound)

		_, err = uc.GetCashPayment(ctx, "non-existent")
		require.ErrorIs(t, err, usecaseCash.ErrCashPaymentNotFound)
	})

	t.Run("Account not found error paths", func(t *testing.T) {
		cmdReceipt := usecaseCash.CreateCashReceiptCommand{
			CompanyProfileID: "comp-01",
			CashAccountID:    "acc-not-found",
		}
		_, _, err := uc.CreateCashReceipt(ctx, cmdReceipt)
		require.ErrorIs(t, err, usecaseGL.ErrAccountNotFound)

		cmdPayment := usecaseCash.CreateCashPaymentCommand{
			CompanyProfileID: "comp-01",
			CashAccountID:    "acc-not-found",
		}
		_, _, err = uc.CreateCashPayment(ctx, cmdPayment)
		require.ErrorIs(t, err, usecaseGL.ErrAccountNotFound)
	})

	t.Run("Create Cash Payment fails on invalid cash account (not class 111)", func(t *testing.T) {
		cmdPayment := usecaseCash.CreateCashPaymentCommand{
			CompanyProfileID: "comp-01",
			CashAccountID:    "acc-1121", // bank
		}
		_, _, err := uc.CreateCashPayment(ctx, cmdPayment)
		require.ErrorIs(t, err, usecaseCash.ErrInvalidCashAccount)
	})

	t.Run("Idempotent submission returns existing Cash Payment and Voucher", func(t *testing.T) {
		idemp := "idemp-cash-pay-01"
		cmd := usecaseCash.CreateCashPaymentCommand{
			CompanyProfileID: "comp-01",
			VoucherNo:        "PC-2026-IDEMP",
			VoucherDate:      txDate,
			PostedDate:       txDate,
			ReceiverName:     "Công ty Delta",
			Reason:           "Chi tạm ứng",
			CashAccountID:    "acc-1111",
			DebitAccountID:   "acc-141",
			TotalAmountVND:   decimal.NewFromInt(2000000),
			IdempotencyKey:   &idemp,
			CreatedBy:        "ktt",
		}

		cp1, v1, err := uc.CreateCashPayment(ctx, cmd)
		require.NoError(t, err)

		cp2, v2, err := uc.CreateCashPayment(ctx, cmd)
		require.NoError(t, err)
		assert.Equal(t, cp1.ID, cp2.ID)
		assert.Equal(t, v1.ID, v2.ID)
	})
}

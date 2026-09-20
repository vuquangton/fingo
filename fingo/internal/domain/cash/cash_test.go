package cash_test

import (
	"context"
	"database/sql"
	"testing"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"fingo/internal/domain/cash"
)

func TestCashReceipt_Validation(t *testing.T) {
	t.Run("Valid cash receipt passes validation", func(t *testing.T) {
		cr, err := cash.NewCashReceipt(cash.CreateCashReceiptParams{
			ID:               "cr-001",
			VoucherID:        "v-001",
			CompanyProfileID: "comp-01",
			PayerName:        "Nguyễn Văn A",
			PayerAddress:     "123 Lê Lợi, Q1, TP.HCM",
			Reason:           "Thu tiền bán hàng trực tiếp",
			CashAccountID:    "acc-1111",
			TotalAmountVND:   decimal.NewFromInt(15000000),
		})
		require.NoError(t, err)
		assert.Equal(t, "cr-001", cr.ID)
		assert.Equal(t, "15000000", cr.TotalAmountVND.String())
	})

	t.Run("Reject zero or negative amount", func(t *testing.T) {
		_, err := cash.NewCashReceipt(cash.CreateCashReceiptParams{
			ID:               "cr-002",
			VoucherID:        "v-002",
			CompanyProfileID: "comp-01",
			PayerName:        "Nguyễn Văn A",
			Reason:           "Thu tiền",
			CashAccountID:    "acc-1111",
			TotalAmountVND:   decimal.Zero,
		})
		require.ErrorIs(t, err, cash.ErrInvalidCashAmount)

		_, err = cash.NewCashReceipt(cash.CreateCashReceiptParams{
			ID:               "cr-003",
			VoucherID:        "v-003",
			CompanyProfileID: "comp-01",
			PayerName:        "Nguyễn Văn A",
			Reason:           "Thu tiền",
			CashAccountID:    "acc-1111",
			TotalAmountVND:   decimal.NewFromInt(-1000),
		})
		require.ErrorIs(t, err, cash.ErrInvalidCashAmount)
	})

	t.Run("Reject empty payer name", func(t *testing.T) {
		_, err := cash.NewCashReceipt(cash.CreateCashReceiptParams{
			ID:               "cr-004",
			VoucherID:        "v-004",
			CompanyProfileID: "comp-01",
			PayerName:        "",
			Reason:           "Thu tiền",
			CashAccountID:    "acc-1111",
			TotalAmountVND:   decimal.NewFromInt(100000),
		})
		require.ErrorIs(t, err, cash.ErrMissingPayerOrReceiver)
	})

	t.Run("Reject empty reason", func(t *testing.T) {
		_, err := cash.NewCashReceipt(cash.CreateCashReceiptParams{
			ID:               "cr-005",
			VoucherID:        "v-005",
			CompanyProfileID: "comp-01",
			PayerName:        "Nguyễn Văn A",
			Reason:           "",
			CashAccountID:    "acc-1111",
			TotalAmountVND:   decimal.NewFromInt(100000),
		})
		require.ErrorIs(t, err, cash.ErrMissingReason)
	})

	t.Run("Reject empty cash account", func(t *testing.T) {
		_, err := cash.NewCashReceipt(cash.CreateCashReceiptParams{
			ID:               "cr-006",
			VoucherID:        "v-006",
			CompanyProfileID: "comp-01",
			PayerName:        "Nguyễn Văn A",
			Reason:           "Thu tiền",
			CashAccountID:    "",
			TotalAmountVND:   decimal.NewFromInt(100000),
		})
		require.ErrorIs(t, err, cash.ErrMissingCashAccount)
	})
}

func TestCashPayment_Validation(t *testing.T) {
	t.Run("Valid cash payment below 5M passes without warning", func(t *testing.T) {
		cp, err := cash.NewCashPayment(cash.CreateCashPaymentParams{
			ID:                        "cp-001",
			VoucherID:                 "v-001",
			CompanyProfileID:          "comp-01",
			ReceiverName:              "Trần Thị B",
			ReceiverAddress:           "456 Hai Bà Trưng",
			Reason:                    "Chi mua văn phòng phẩm",
			CashAccountID:             "acc-1111",
			TotalAmountVND:            decimal.NewFromInt(4999999),
			RequiresNonCashCompliance: true,
		})
		require.NoError(t, err)
		assert.Equal(t, "cp-001", cp.ID)
		assert.False(t, cp.IsNonCashOverride)
	})

	t.Run("Enforce Decree 181/2025 non-cash gate >= 5M for commercial payment (INV-OPS-05)", func(t *testing.T) {
		// Exactly 5,000,000 VND without override fails
		_, err := cash.NewCashPayment(cash.CreateCashPaymentParams{
			ID:                        "cp-002",
			VoucherID:                 "v-002",
			CompanyProfileID:          "comp-01",
			ReceiverName:              "Công ty TNHH NCC Alpha",
			Reason:                    "Chi trả tiền hàng nhà cung cấp hóa đơn HD01",
			CashAccountID:             "acc-1111",
			TotalAmountVND:            decimal.NewFromInt(5000000),
			RequiresNonCashCompliance: true,
			IsNonCashOverride:         false,
		})
		require.ErrorIs(t, err, cash.ErrNonCashThresholdExceeded)

		// Over 5,000,000 VND without override fails
		_, err = cash.NewCashPayment(cash.CreateCashPaymentParams{
			ID:                        "cp-003",
			VoucherID:                 "v-003",
			CompanyProfileID:          "comp-01",
			ReceiverName:              "Công ty TNHH NCC Alpha",
			Reason:                    "Chi trả tiền mua máy móc",
			CashAccountID:             "acc-1111",
			TotalAmountVND:            decimal.NewFromInt(20000000),
			RequiresNonCashCompliance: true,
			IsNonCashOverride:         false,
		})
		require.ErrorIs(t, err, cash.ErrNonCashThresholdExceeded)
	})

	t.Run("Require Chief Accountant confirmation code for >= 5M override", func(t *testing.T) {
		_, err := cash.NewCashPayment(cash.CreateCashPaymentParams{
			ID:                        "cp-004-no-code",
			VoucherID:                 "v-004",
			CompanyProfileID:          "comp-01",
			ReceiverName:              "Công ty TNHH NCC Alpha",
			Reason:                    "Chi trả tiền hàng",
			CashAccountID:             "acc-1111",
			TotalAmountVND:            decimal.NewFromInt(15000000),
			RequiresNonCashCompliance: true,
			IsNonCashOverride:         true,
			ConfirmationCode:          "WRONG_CODE",
			OverrideReason:            "Chi khẩn",
		})
		require.ErrorIs(t, err, cash.ErrChiefAccountantConfirmationRequired)
	})

	t.Run("Allow >= 5M commercial cash payment with explicit override and confirmation code", func(t *testing.T) {
		cp, err := cash.NewCashPayment(cash.CreateCashPaymentParams{
			ID:                        "cp-004",
			VoucherID:                 "v-004",
			CompanyProfileID:          "comp-01",
			ReceiverName:              "Công ty TNHH NCC Alpha",
			Reason:                    "Chi trả tiền hàng nhà cung cấp HD01",
			CashAccountID:             "acc-1111",
			TotalAmountVND:            decimal.NewFromInt(15000000),
			RequiresNonCashCompliance: true,
			IsNonCashOverride:         true,
			ConfirmationCode:          cash.NonCashConfirmationCode,
			OverrideReason:            "Giám đốc và Kế toán trưởng phê duyệt chi tiền mặt khẩn cấp",
		})
		require.NoError(t, err)
		assert.True(t, cp.IsNonCashOverride)
		assert.NotEmpty(t, cp.OverrideReason)
	})

	t.Run("Reject override without explicit override reason", func(t *testing.T) {
		_, err := cash.NewCashPayment(cash.CreateCashPaymentParams{
			ID:                        "cp-005",
			VoucherID:                 "v-005",
			CompanyProfileID:          "comp-01",
			ReceiverName:              "Công ty TNHH NCC Alpha",
			Reason:                    "Chi trả tiền hàng",
			CashAccountID:             "acc-1111",
			TotalAmountVND:            decimal.NewFromInt(15000000),
			RequiresNonCashCompliance: true,
			IsNonCashOverride:         true,
			ConfirmationCode:          cash.NonCashConfirmationCode,
			OverrideReason:            "", // Blank
		})
		require.ErrorIs(t, err, cash.ErrOverrideReasonRequired)
	})

	t.Run("Non-commercial cash payment (e.g. employee advance/TK 141) >= 5M allowed without override", func(t *testing.T) {
		cp, err := cash.NewCashPayment(cash.CreateCashPaymentParams{
			ID:                        "cp-006",
			VoucherID:                 "v-006",
			CompanyProfileID:          "comp-01",
			ReceiverName:              "Lê Văn C",
			Reason:                    "Tạm ứng công tác phí Hà Nội (TK 141)",
			CashAccountID:             "acc-1111",
			TotalAmountVND:            decimal.NewFromInt(10000000),
			RequiresNonCashCompliance: false, // Non-commercial
		})
		require.NoError(t, err)
		assert.Equal(t, "10000000", cp.TotalAmountVND.String())
	})

	t.Run("Reject empty receiver, reason, or cash account", func(t *testing.T) {
		_, err := cash.NewCashPayment(cash.CreateCashPaymentParams{
			ID:               "cp-007",
			VoucherID:        "v-007",
			CompanyProfileID: "comp-01",
			ReceiverName:     "",
			Reason:           "Chi tiền",
			CashAccountID:    "acc-1111",
			TotalAmountVND:   decimal.NewFromInt(100000),
		})
		require.ErrorIs(t, err, cash.ErrMissingPayerOrReceiver)

		_, err = cash.NewCashPayment(cash.CreateCashPaymentParams{
			ID:               "cp-008",
			VoucherID:        "v-008",
			CompanyProfileID: "comp-01",
			ReceiverName:     "Lê Văn C",
			Reason:           "",
			CashAccountID:    "acc-1111",
			TotalAmountVND:   decimal.NewFromInt(100000),
		})
		require.ErrorIs(t, err, cash.ErrMissingReason)

		_, err = cash.NewCashPayment(cash.CreateCashPaymentParams{
			ID:               "cp-009",
			VoucherID:        "v-009",
			CompanyProfileID: "comp-01",
			ReceiverName:     "Lê Văn C",
			Reason:           "Chi tiền",
			CashAccountID:    "",
			TotalAmountVND:   decimal.NewFromInt(100000),
		})
		require.ErrorIs(t, err, cash.ErrMissingCashAccount)
	})
}

func TestCashRepositoryStub(t *testing.T) {
	ctx := context.Background()
	stub := cash.NewCashRepositoryStub()

	cr, err := cash.NewCashReceipt(cash.CreateCashReceiptParams{
		ID:               "cr-stub-1",
		VoucherID:        "v-stub-1",
		CompanyProfileID: "comp-01",
		PayerName:        "Nguyễn A",
		Reason:           "Thu nợ",
		CashAccountID:    "acc-1111",
		TotalAmountVND:   decimal.NewFromInt(1000000),
	})
	require.NoError(t, err)
	require.NoError(t, stub.SaveCashReceipt(ctx, cr))

	fetchedCR, err := stub.GetCashReceiptByID(ctx, "cr-stub-1")
	require.NoError(t, err)
	assert.Equal(t, cr.ID, fetchedCR.ID)

	fetchedCRByV, err := stub.GetCashReceiptByVoucherID(ctx, "v-stub-1")
	require.NoError(t, err)
	assert.Equal(t, cr.ID, fetchedCRByV.ID)

	cp, err := cash.NewCashPayment(cash.CreateCashPaymentParams{
		ID:               "cp-stub-1",
		VoucherID:        "v-stub-2",
		CompanyProfileID: "comp-01",
		ReceiverName:     "Trần B",
		Reason:           "Chi văn phòng phẩm",
		CashAccountID:    "acc-1111",
		TotalAmountVND:   decimal.NewFromInt(500000),
	})
	require.NoError(t, err)
	require.NoError(t, stub.SaveCashPayment(ctx, cp))

	fetchedCP, err := stub.GetCashPaymentByID(ctx, "cp-stub-1")
	require.NoError(t, err)
	assert.Equal(t, cp.ID, fetchedCP.ID)

	fetchedCPByV, err := stub.GetCashPaymentByVoucherID(ctx, "v-stub-2")
	require.NoError(t, err)
	assert.Equal(t, cp.ID, fetchedCPByV.ID)

	// Not found paths
	_, err = stub.GetCashReceiptByID(ctx, "non-existent")
	require.ErrorIs(t, err, sql.ErrNoRows)

	_, err = stub.GetCashReceiptByVoucherID(ctx, "non-existent")
	assert.True(t, err != nil)

	_, err = stub.GetCashPaymentByID(ctx, "non-existent")
	assert.True(t, err != nil)

	_, err = stub.GetCashPaymentByVoucherID(ctx, "non-existent")
	assert.True(t, err != nil)
}

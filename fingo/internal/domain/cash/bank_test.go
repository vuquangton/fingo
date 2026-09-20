package cash_test

import (
	"context"
	"testing"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"fingo/internal/domain/cash"
)

func TestBankTransaction_Validation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		params      cash.CreateBankTransactionParams
		expectedErr error
	}{
		{
			name: "Valid Credit Advice passes validation",
			params: cash.CreateBankTransactionParams{
				ID:               "bt-001",
				VoucherID:        "v-001",
				CompanyProfileID: "comp-01",
				BankAccountID:    "bank-acc-01",
				TransactionType:  cash.BankTransactionTypeCreditAdvice,
				Counterparty: cash.BankCounterpartyInfo{
					AccountNo: "1012345678",
					BankName:  "Vietcombank",
					Name:      "Công ty Alpha",
					Reference: "BC-2026-001",
				},
				FeeAmountVND:    decimal.Zero,
				VatFeeAmountVND: decimal.Zero,
				TotalAmountVND:  decimal.NewFromInt(50000000),
			},
			expectedErr: nil,
		},
		{
			name: "Valid Transfer Order with banking fee and VAT passes validation",
			params: cash.CreateBankTransactionParams{
				ID:               "bt-002",
				VoucherID:        "v-002",
				CompanyProfileID: "comp-01",
				BankAccountID:    "bank-acc-01",
				TransactionType:  cash.BankTransactionTypeTransferOrder,
				Counterparty: cash.BankCounterpartyInfo{
					AccountNo: "987654321",
					BankName:  "BIDV",
					Name:      "Công ty Beta",
					Reference: "UNC-2026-001",
				},
				FeeAmountVND:    decimal.NewFromInt(20000),
				VatFeeAmountVND: decimal.NewFromInt(2000),
				TotalAmountVND:  decimal.NewFromInt(100000000),
			},
			expectedErr: nil,
		},
		{
			name: "Reject zero total amount",
			params: cash.CreateBankTransactionParams{
				ID:               "bt-003",
				VoucherID:        "v-003",
				CompanyProfileID: "comp-01",
				BankAccountID:    "bank-acc-01",
				TransactionType:  cash.BankTransactionTypeDebitAdvice,
				TotalAmountVND:   decimal.Zero,
			},
			expectedErr: cash.ErrInvalidBankAmount,
		},
		{
			name: "Reject negative total amount",
			params: cash.CreateBankTransactionParams{
				ID:               "bt-004",
				VoucherID:        "v-004",
				CompanyProfileID: "comp-01",
				BankAccountID:    "bank-acc-01",
				TransactionType:  cash.BankTransactionTypeDebitAdvice,
				TotalAmountVND:   decimal.NewFromInt(-500),
			},
			expectedErr: cash.ErrInvalidBankAmount,
		},
		{
			name: "Reject negative bank fee",
			params: cash.CreateBankTransactionParams{
				ID:               "bt-005",
				VoucherID:        "v-005",
				CompanyProfileID: "comp-01",
				BankAccountID:    "bank-acc-01",
				TransactionType:  cash.BankTransactionTypeTransferOrder,
				TotalAmountVND:   decimal.NewFromInt(1000000),
				FeeAmountVND:     decimal.NewFromInt(-1000),
			},
			expectedErr: cash.ErrNegativeBankFee,
		},
		{
			name: "Reject negative VAT fee",
			params: cash.CreateBankTransactionParams{
				ID:               "bt-006",
				VoucherID:        "v-006",
				CompanyProfileID: "comp-01",
				BankAccountID:    "bank-acc-01",
				TransactionType:  cash.BankTransactionTypeTransferOrder,
				TotalAmountVND:   decimal.NewFromInt(1000000),
				VatFeeAmountVND:  decimal.NewFromInt(-100),
			},
			expectedErr: cash.ErrNegativeBankFee,
		},
		{
			name: "Reject empty bank account ID",
			params: cash.CreateBankTransactionParams{
				ID:               "bt-007",
				VoucherID:        "v-007",
				CompanyProfileID: "comp-01",
				BankAccountID:    "",
				TransactionType:  cash.BankTransactionTypeCreditAdvice,
				TotalAmountVND:   decimal.NewFromInt(1000000),
			},
			expectedErr: cash.ErrMissingBankAccount,
		},
		{
			name: "Reject invalid transaction type",
			params: cash.CreateBankTransactionParams{
				ID:               "bt-008",
				VoucherID:        "v-008",
				CompanyProfileID: "comp-01",
				BankAccountID:    "bank-acc-01",
				TransactionType:  cash.BankTransactionType("INVALID_TYPE"),
				TotalAmountVND:   decimal.NewFromInt(1000000),
			},
			expectedErr: cash.ErrInvalidTransactionType,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			bt, err := cash.NewBankTransaction(tt.params)
			if tt.expectedErr != nil {
				require.ErrorIs(t, err, tt.expectedErr)
				assert.Nil(t, bt)
			} else {
				require.NoError(t, err)
				assert.NotNil(t, bt)
				assert.Equal(t, tt.params.ID, bt.ID)
			}
		})
	}
}

func TestBankRepositoryStub(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	stub := cash.NewBankRepositoryStub()

	bt, err := cash.NewBankTransaction(cash.CreateBankTransactionParams{
		ID:               "bt-stub-1",
		VoucherID:        "v-stub-1",
		CompanyProfileID: "comp-01",
		BankAccountID:    "bank-acc-01",
		TransactionType:  cash.BankTransactionTypeCreditAdvice,
		TotalAmountVND:   decimal.NewFromInt(10000000),
	})
	require.NoError(t, err)
	require.NoError(t, stub.SaveBankTransaction(ctx, bt))

	fetched, err := stub.GetBankTransactionByID(ctx, "bt-stub-1")
	require.NoError(t, err)
	assert.Equal(t, bt.ID, fetched.ID)

	fetchedByV, err := stub.GetBankTransactionByVoucherID(ctx, "v-stub-1")
	require.NoError(t, err)
	assert.Equal(t, bt.ID, fetchedByV.ID)

	// Not found paths return domain sentinel ErrBankTransactionNotFound
	_, err = stub.GetBankTransactionByID(ctx, "non-existent")
	require.ErrorIs(t, err, cash.ErrBankTransactionNotFound)

	_, err = stub.GetBankTransactionByVoucherID(ctx, "non-existent")
	require.ErrorIs(t, err, cash.ErrBankTransactionNotFound)
}

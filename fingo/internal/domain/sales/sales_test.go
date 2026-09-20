package sales_test

import (
	"context"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"fingo/internal/domain/sales"
)

func TestSalesInvoice_Validation(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 3, 20, 0, 0, 0, 0, time.UTC)
	dueDate := now.AddDate(0, 1, 0)
	whID := "wh-01"

	t.Run("Valid sales invoice with discount and 10% VAT passes math", func(t *testing.T) {
		si, err := sales.NewSalesInvoice(sales.CreateSalesInvoiceParams{
			ID:                "si-001",
			VoucherID:         "v-001",
			CompanyProfileID:  "comp-01",
			CustomerID:        "cust-01",
			InvoiceTemplate:   "1",
			InvoiceSeries:     "C26TAA",
			InvoiceNo:         "00000001",
			InvoiceDate:       now,
			DueDate:           dueDate,
			PaymentMethod:     "CK",
			IsStockOutwardAuto: true,
			Lines: []sales.CreateSalesInvoiceLineParams{
				{
					LineOrder:       1,
					ItemID:          "item-01",
					WarehouseID:     &whID,
					DebitAccountID:  "acc-131",
					CreditAccountID: "acc-5111",
					Quantity:        decimal.NewFromInt(10),
					UnitPriceVND:    decimal.NewFromInt(2000000), // Gross: 20,000,000
					DiscountRate:    decimal.NewFromInt(5),       // 5% discount = 1,000,000 -> Net: 19,000,000
					VATRate:         decimal.NewFromInt(10),      // 10% of 19,000,000 = 1,900,000
					Note:            "Bán hàng sỉ",
				},
			},
		})
		require.NoError(t, err)
		assert.Equal(t, "20000000", si.SubtotalVND.String())
		assert.Equal(t, "1000000", si.DiscountVND.String())
		assert.Equal(t, "1900000", si.VATAmountVND.String())
		// Total: (20M - 1M) + 1.9M = 20,900,000
		assert.Equal(t, "20900000", si.TotalAmountVND.String())
		assert.Equal(t, sales.EInvoiceStatusDraft, si.EInvoiceStatus)
		assert.Len(t, si.Lines, 1)
	})

	t.Run("Valid sales invoice with 8% VAT (Resolution 204/2025)", func(t *testing.T) {
		si, err := sales.NewSalesInvoice(sales.CreateSalesInvoiceParams{
			ID:               "si-002",
			VoucherID:        "v-002",
			CompanyProfileID: "comp-01",
			CustomerID:       "cust-01",
			InvoiceSeries:    "C26TAA",
			InvoiceNo:        "00000002",
			InvoiceDate:      now,
			DueDate:          dueDate,
			Lines: []sales.CreateSalesInvoiceLineParams{
				{
					ItemID:          "item-01",
					DebitAccountID:  "acc-131",
					CreditAccountID: "acc-5111",
					Quantity:        decimal.NewFromInt(100),
					UnitPriceVND:    decimal.NewFromInt(100000), // 10,000,000
					VATRate:         decimal.NewFromInt(8),      // 8% = 800,000
				},
			},
		})
		require.NoError(t, err)
		assert.Equal(t, "10000000", si.SubtotalVND.String())
		assert.Equal(t, "800000", si.VATAmountVND.String())
		assert.Equal(t, "10800000", si.TotalAmountVND.String())
	})

	t.Run("Reject invalid discount rate or invalid VAT rate", func(t *testing.T) {
		_, err := sales.NewSalesInvoice(sales.CreateSalesInvoiceParams{
			ID:               "si-003",
			VoucherID:        "v-003",
			CompanyProfileID: "comp-01",
			CustomerID:       "cust-01",
			InvoiceNo:        "00000003",
			InvoiceDate:      now,
			DueDate:          dueDate,
			Lines: []sales.CreateSalesInvoiceLineParams{
				{
					ItemID:          "item-01",
					DebitAccountID:  "acc-131",
					CreditAccountID: "acc-5111",
					Quantity:        decimal.NewFromInt(10),
					UnitPriceVND:    decimal.NewFromInt(100000),
					DiscountRate:    decimal.NewFromInt(105), // Invalid: > 100%
					VATRate:         decimal.NewFromInt(10),
				},
			},
		})
		require.ErrorIs(t, err, sales.ErrInvalidDiscountRate)

		_, err = sales.NewSalesInvoice(sales.CreateSalesInvoiceParams{
			ID:               "si-004",
			VoucherID:        "v-004",
			CompanyProfileID: "comp-01",
			CustomerID:       "cust-01",
			InvoiceNo:        "00000004",
			InvoiceDate:      now,
			DueDate:          dueDate,
			Lines: []sales.CreateSalesInvoiceLineParams{
				{
					ItemID:          "item-01",
					DebitAccountID:  "acc-131",
					CreditAccountID: "acc-5111",
					Quantity:        decimal.NewFromInt(10),
					UnitPriceVND:    decimal.NewFromInt(100000),
					VATRate:         decimal.NewFromInt(12), // Invalid: not 0, 5, 8, 10, -1
				},
			},
		})
		require.ErrorIs(t, err, sales.ErrInvalidVATRate)
	})

	t.Run("Reject zero quantity or negative unit price", func(t *testing.T) {
		_, err := sales.NewSalesInvoice(sales.CreateSalesInvoiceParams{
			ID:               "si-005",
			VoucherID:        "v-005",
			CompanyProfileID: "comp-01",
			CustomerID:       "cust-01",
			InvoiceNo:        "00000005",
			InvoiceDate:      now,
			DueDate:          dueDate,
			Lines: []sales.CreateSalesInvoiceLineParams{
				{
					ItemID:          "item-01",
					DebitAccountID:  "acc-131",
					CreditAccountID: "acc-5111",
					Quantity:        decimal.Zero,
					UnitPriceVND:    decimal.NewFromInt(100000),
				},
			},
		})
		require.ErrorIs(t, err, sales.ErrInvalidLineQuantity)

		_, err = sales.NewSalesInvoice(sales.CreateSalesInvoiceParams{
			ID:               "si-006",
			VoucherID:        "v-006",
			CompanyProfileID: "comp-01",
			CustomerID:       "cust-01",
			InvoiceNo:        "00000006",
			InvoiceDate:      now,
			DueDate:          dueDate,
			Lines: []sales.CreateSalesInvoiceLineParams{
				{
					ItemID:          "item-01",
					DebitAccountID:  "acc-131",
					CreditAccountID: "acc-5111",
					Quantity:        decimal.NewFromInt(10),
					UnitPriceVND:    decimal.NewFromInt(-100),
				},
			},
		})
		require.ErrorIs(t, err, sales.ErrInvalidLineUnitPrice)
	})

	t.Run("Reject empty customer ID or empty invoice no", func(t *testing.T) {
		_, err := sales.NewSalesInvoice(sales.CreateSalesInvoiceParams{
			ID:               "si-007",
			VoucherID:        "v-007",
			CompanyProfileID: "comp-01",
			CustomerID:       "",
			InvoiceNo:        "00000007",
			InvoiceDate:      now,
			DueDate:          dueDate,
			Lines: []sales.CreateSalesInvoiceLineParams{
				{
					ItemID:          "item-01",
					DebitAccountID:  "acc-131",
					CreditAccountID: "acc-5111",
					Quantity:        decimal.NewFromInt(10),
					UnitPriceVND:    decimal.NewFromInt(100000),
				},
			},
		})
		require.ErrorIs(t, err, sales.ErrMissingCustomerID)

		_, err = sales.NewSalesInvoice(sales.CreateSalesInvoiceParams{
			ID:               "si-008",
			VoucherID:        "v-008",
			CompanyProfileID: "comp-01",
			CustomerID:       "cust-01",
			InvoiceNo:        "",
			InvoiceDate:      now,
			DueDate:          dueDate,
			Lines: []sales.CreateSalesInvoiceLineParams{
				{
					ItemID:          "item-01",
					DebitAccountID:  "acc-131",
					CreditAccountID: "acc-5111",
					Quantity:        decimal.NewFromInt(10),
					UnitPriceVND:    decimal.NewFromInt(100000),
				},
			},
		})
		require.ErrorIs(t, err, sales.ErrMissingInvoiceNo)
	})

	t.Run("Reject invoice without lines", func(t *testing.T) {
		_, err := sales.NewSalesInvoice(sales.CreateSalesInvoiceParams{
			ID:               "si-009",
			VoucherID:        "v-009",
			CompanyProfileID: "comp-01",
			CustomerID:       "cust-01",
			InvoiceNo:        "00000009",
			InvoiceDate:      now,
			DueDate:          dueDate,
			Lines:            nil,
		})
		require.ErrorIs(t, err, sales.ErrZeroInvoiceLines)
	})
}

func TestSalesRepositoryStub(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	stub := sales.NewSalesRepositoryStub()

	now := time.Now()
	si, err := sales.NewSalesInvoice(sales.CreateSalesInvoiceParams{
		ID:               "si-stub-1",
		VoucherID:        "v-stub-1",
		CompanyProfileID: "comp-01",
		CustomerID:       "cust-01",
		InvoiceNo:        "HD-001",
		InvoiceDate:      now,
		DueDate:          now.AddDate(0, 1, 0),
		Lines: []sales.CreateSalesInvoiceLineParams{
			{
				ItemID:          "item-01",
				DebitAccountID:  "acc-131",
				CreditAccountID: "acc-5111",
				Quantity:        decimal.NewFromInt(5),
				UnitPriceVND:    decimal.NewFromInt(100000),
				VATRate:         decimal.NewFromInt(10),
			},
		},
	})
	require.NoError(t, err)
	require.NoError(t, stub.SaveSalesInvoice(ctx, si))

	fetched, err := stub.GetSalesInvoiceByID(ctx, "si-stub-1")
	require.NoError(t, err)
	assert.Equal(t, si.ID, fetched.ID)
	assert.Len(t, fetched.Lines, 1)

	fetchedByV, err := stub.GetSalesInvoiceByVoucherID(ctx, "v-stub-1")
	require.NoError(t, err)
	assert.Equal(t, si.ID, fetchedByV.ID)

	// Update EInvoice status
	cqtCode := "CQT-123456"
	require.NoError(t, stub.UpdateEInvoiceStatus(ctx, si.ID, sales.EInvoiceStatusCQTAccepted, &cqtCode))
	updated, err := stub.GetSalesInvoiceByID(ctx, si.ID)
	require.NoError(t, err)
	assert.Equal(t, sales.EInvoiceStatusCQTAccepted, updated.EInvoiceStatus)
	assert.Equal(t, &cqtCode, updated.EInvoiceCodeCQT)

	// Not found paths
	_, err = stub.GetSalesInvoiceByID(ctx, "non-existent")
	require.ErrorIs(t, err, sales.ErrSalesInvoiceNotFound)

	_, err = stub.GetSalesInvoiceByVoucherID(ctx, "non-existent")
	require.ErrorIs(t, err, sales.ErrSalesInvoiceNotFound)
}

package purchase_test

import (
	"context"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"fingo/internal/domain/purchase"
)

func TestPurchaseInvoice_Validation(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 3, 20, 0, 0, 0, 0, time.UTC)
	dueDate := now.AddDate(0, 1, 0)
	whID := "wh-01"

	t.Run("Valid purchase invoice with 10% VAT passes math and validation", func(t *testing.T) {
		pi, err := purchase.NewPurchaseInvoice(purchase.CreatePurchaseInvoiceParams{
			ID:                 "pi-001",
			VoucherID:          "v-001",
			CompanyProfileID:   "comp-01",
			VendorID:           "vendor-01",
			InvoiceTemplate:    "1",
			InvoiceSeries:      "C26TAA",
			InvoiceNo:          "00000123",
			InvoiceDate:        now,
			DueDate:            dueDate,
			IsStockInwardAuto:  true,
			Lines: []purchase.CreatePurchaseInvoiceLineParams{
				{
					LineOrder:       1,
					ItemID:          "item-01",
					WarehouseID:     &whID,
					DebitAccountID:  "acc-1561",
					CreditAccountID: "acc-331",
					Quantity:        decimal.NewFromInt(10),
					UnitPriceVND:    decimal.NewFromInt(2000000), // 2,000,000 * 10 = 20,000,000
					VATRate:         decimal.NewFromInt(10),       // 10% = 2,000,000
					Note:            "Máy in văn phòng",
				},
				{
					LineOrder:       2,
					ItemID:          "item-02",
					WarehouseID:     &whID,
					DebitAccountID:  "acc-1561",
					CreditAccountID: "acc-331",
					Quantity:        decimal.NewFromInt(50),
					UnitPriceVND:    decimal.NewFromInt(100000), // 100,000 * 50 = 5,000,000
					VATRate:         decimal.NewFromInt(10),      // 10% = 500,000
					Note:            "Mực in",
				},
			},
		})
		require.NoError(t, err)
		assert.Equal(t, "25000000", pi.SubtotalVND.String())
		assert.Equal(t, "2500000", pi.VATAmountVND.String())
		assert.Equal(t, "27500000", pi.TotalAmountVND.String())
		assert.Equal(t, purchase.PaymentStatusUnpaid, pi.PaymentStatus)
		assert.Len(t, pi.Lines, 2)
	})

	t.Run("Valid 8% VAT rate under Resolution 204/2025 in 2026", func(t *testing.T) {
		pi, err := purchase.NewPurchaseInvoice(purchase.CreatePurchaseInvoiceParams{
			ID:               "pi-002",
			VoucherID:        "v-002",
			CompanyProfileID: "comp-01",
			VendorID:         "vendor-01",
			InvoiceSeries:    "C26TAA",
			InvoiceNo:        "00000124",
			InvoiceDate:      now,
			DueDate:          dueDate,
			Lines: []purchase.CreatePurchaseInvoiceLineParams{
				{
					LineOrder:       1,
					ItemID:          "item-01",
					DebitAccountID:  "acc-1561",
					CreditAccountID: "acc-331",
					Quantity:        decimal.NewFromInt(100),
					UnitPriceVND:    decimal.NewFromInt(100000), // 10,000,000
					VATRate:         decimal.NewFromInt(8),      // 8% = 800,000
				},
			},
		})
		require.NoError(t, err)
		assert.Equal(t, "10000000", pi.SubtotalVND.String())
		assert.Equal(t, "800000", pi.VATAmountVND.String())
		assert.Equal(t, "10800000", pi.TotalAmountVND.String())
	})

	t.Run("Reject invalid VAT rate (e.g. 15%)", func(t *testing.T) {
		_, err := purchase.NewPurchaseInvoice(purchase.CreatePurchaseInvoiceParams{
			ID:               "pi-003",
			VoucherID:        "v-003",
			CompanyProfileID: "comp-01",
			VendorID:         "vendor-01",
			InvoiceNo:        "00000125",
			InvoiceDate:      now,
			DueDate:          dueDate,
			Lines: []purchase.CreatePurchaseInvoiceLineParams{
				{
					ItemID:          "item-01",
					DebitAccountID:  "acc-1561",
					CreditAccountID: "acc-331",
					Quantity:        decimal.NewFromInt(10),
					UnitPriceVND:    decimal.NewFromInt(100000),
					VATRate:         decimal.NewFromInt(15), // Invalid VAT rate
				},
			},
		})
		require.ErrorIs(t, err, purchase.ErrInvalidVATRate)
	})

	t.Run("Reject zero quantity or negative unit price", func(t *testing.T) {
		_, err := purchase.NewPurchaseInvoice(purchase.CreatePurchaseInvoiceParams{
			ID:               "pi-004",
			VoucherID:        "v-004",
			CompanyProfileID: "comp-01",
			VendorID:         "vendor-01",
			InvoiceNo:        "00000126",
			InvoiceDate:      now,
			DueDate:          dueDate,
			Lines: []purchase.CreatePurchaseInvoiceLineParams{
				{
					ItemID:          "item-01",
					DebitAccountID:  "acc-1561",
					CreditAccountID: "acc-331",
					Quantity:        decimal.Zero,
					UnitPriceVND:    decimal.NewFromInt(100000),
				},
			},
		})
		require.ErrorIs(t, err, purchase.ErrInvalidLineQuantity)

		_, err = purchase.NewPurchaseInvoice(purchase.CreatePurchaseInvoiceParams{
			ID:               "pi-005",
			VoucherID:        "v-005",
			CompanyProfileID: "comp-01",
			VendorID:         "vendor-01",
			InvoiceNo:        "00000127",
			InvoiceDate:      now,
			DueDate:          dueDate,
			Lines: []purchase.CreatePurchaseInvoiceLineParams{
				{
					ItemID:          "item-01",
					DebitAccountID:  "acc-1561",
					CreditAccountID: "acc-331",
					Quantity:        decimal.NewFromInt(10),
					UnitPriceVND:    decimal.NewFromInt(-100),
				},
			},
		})
		require.ErrorIs(t, err, purchase.ErrInvalidLineUnitPrice)
	})

	t.Run("Reject empty vendor or empty invoice no", func(t *testing.T) {
		_, err := purchase.NewPurchaseInvoice(purchase.CreatePurchaseInvoiceParams{
			ID:               "pi-006",
			VoucherID:        "v-006",
			CompanyProfileID: "comp-01",
			VendorID:         "",
			InvoiceNo:        "00000128",
			InvoiceDate:      now,
			DueDate:          dueDate,
			Lines: []purchase.CreatePurchaseInvoiceLineParams{
				{
					ItemID:          "item-01",
					DebitAccountID:  "acc-1561",
					CreditAccountID: "acc-331",
					Quantity:        decimal.NewFromInt(10),
					UnitPriceVND:    decimal.NewFromInt(100000),
				},
			},
		})
		require.ErrorIs(t, err, purchase.ErrMissingVendorID)

		_, err = purchase.NewPurchaseInvoice(purchase.CreatePurchaseInvoiceParams{
			ID:               "pi-007",
			VoucherID:        "v-007",
			CompanyProfileID: "comp-01",
			VendorID:         "vendor-01",
			InvoiceNo:        "",
			InvoiceDate:      now,
			DueDate:          dueDate,
			Lines: []purchase.CreatePurchaseInvoiceLineParams{
				{
					ItemID:          "item-01",
					DebitAccountID:  "acc-1561",
					CreditAccountID: "acc-331",
					Quantity:        decimal.NewFromInt(10),
					UnitPriceVND:    decimal.NewFromInt(100000),
				},
			},
		})
		require.ErrorIs(t, err, purchase.ErrMissingInvoiceNo)
	})

	t.Run("Reject invoice without lines", func(t *testing.T) {
		_, err := purchase.NewPurchaseInvoice(purchase.CreatePurchaseInvoiceParams{
			ID:               "pi-008",
			VoucherID:        "v-008",
			CompanyProfileID: "comp-01",
			VendorID:         "vendor-01",
			InvoiceNo:        "00000129",
			InvoiceDate:      now,
			DueDate:          dueDate,
			Lines:            nil,
		})
		require.ErrorIs(t, err, purchase.ErrZeroInvoiceLines)
	})
}

func TestPurchaseRepositoryStub(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	stub := purchase.NewPurchaseRepositoryStub()

	now := time.Now()
	pi, err := purchase.NewPurchaseInvoice(purchase.CreatePurchaseInvoiceParams{
		ID:               "pi-stub-1",
		VoucherID:        "v-stub-1",
		CompanyProfileID: "comp-01",
		VendorID:         "vendor-01",
		InvoiceNo:        "HD-001",
		InvoiceDate:      now,
		DueDate:          now.AddDate(0, 1, 0),
		Lines: []purchase.CreatePurchaseInvoiceLineParams{
			{
				LineOrder:       1,
				ItemID:          "item-01",
				DebitAccountID:  "acc-1561",
				CreditAccountID: "acc-331",
				Quantity:        decimal.NewFromInt(5),
				UnitPriceVND:    decimal.NewFromInt(100000),
				VATRate:         decimal.NewFromInt(10),
			},
		},
	})
	require.NoError(t, err)
	require.NoError(t, stub.SavePurchaseInvoice(ctx, pi))

	fetched, err := stub.GetPurchaseInvoiceByID(ctx, "pi-stub-1")
	require.NoError(t, err)
	assert.Equal(t, pi.ID, fetched.ID)
	assert.Len(t, fetched.Lines, 1)

	fetchedByV, err := stub.GetPurchaseInvoiceByVoucherID(ctx, "v-stub-1")
	require.NoError(t, err)
	assert.Equal(t, pi.ID, fetchedByV.ID)

	// Update payment status
	require.NoError(t, stub.UpdatePaymentStatus(ctx, pi.ID, purchase.PaymentStatusPaid, decimal.NewFromInt(550000)))
	updated, err := stub.GetPurchaseInvoiceByID(ctx, pi.ID)
	require.NoError(t, err)
	assert.Equal(t, purchase.PaymentStatusPaid, updated.PaymentStatus)
	assert.Equal(t, "550000", updated.PaidAmountVND.String())

	// Not found paths
	_, err = stub.GetPurchaseInvoiceByID(ctx, "non-existent")
	require.ErrorIs(t, err, purchase.ErrPurchaseInvoiceNotFound)

	_, err = stub.GetPurchaseInvoiceByVoucherID(ctx, "non-existent")
	require.ErrorIs(t, err, purchase.ErrPurchaseInvoiceNotFound)
}

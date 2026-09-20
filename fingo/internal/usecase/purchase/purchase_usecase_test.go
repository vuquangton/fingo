package purchase_test

import (
	"context"
	"database/sql"
	"io"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"fingo/internal/domain/gl"
	"fingo/internal/domain/purchase"
	spineDomain "fingo/internal/domain/spine"
	"fingo/internal/domain/system"
	glUseCase "fingo/internal/usecase/gl"
	usecasePurchase "fingo/internal/usecase/purchase"
	"fingo/pkg/logger"
)

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

func setupPurchaseTestEnv() (*usecasePurchase.PurchaseUseCase, *purchase.PurchaseRepositoryStub) {
	purchaseRepo := purchase.NewPurchaseRepositoryStub()
	glRepo := &mockGLRepo{vouchers: make(map[string]*gl.Voucher)}
	accRepo := &mockAccountRepo{
		accounts: map[string]*spineDomain.Account{
			"acc-1561":  {ID: "acc-1561", Code: "1561", Name: "Hàng hóa", IsLeaf: true, IsActive: true},
			"acc-331":   {ID: "acc-331", Code: "331", Name: "Phải trả cho người bán", IsLeaf: true, IsActive: true},
			"acc-13311": {ID: "acc-13311", Code: "13311", Name: "Thuế GTGT đầu vào", IsLeaf: true, IsActive: true},
		},
	}
	compRepo := &mockCompanyRepo{}
	log := logger.New(logger.Config{Writer: io.Discard})

	glUC := glUseCase.NewGLUseCase(glRepo, compRepo, accRepo, log)
	purchaseUC := usecasePurchase.NewPurchaseUseCase(purchaseRepo, glUC, log)
	return purchaseUC, purchaseRepo
}

func TestPurchaseUseCase_Workflow(t *testing.T) {
	t.Parallel()
	uc, _ := setupPurchaseTestEnv()
	ctx := context.Background()
	now := time.Date(2026, 3, 20, 0, 0, 0, 0, time.UTC)
	dueDate := now.AddDate(0, 1, 0)
	whID := "wh-01"

	t.Run("Create valid Purchase Invoice with 10% VAT", func(t *testing.T) {
		cmd := usecasePurchase.CreatePurchaseInvoiceCommand{
			CompanyProfileID:  "comp-01",
			VoucherNo:         "MH-2026-0001",
			VoucherDate:       now,
			PostedDate:        now,
			VendorID:          "vendor-01",
			InvoiceTemplate:   "1",
			InvoiceSeries:     "C26TAA",
			InvoiceNo:         "00001001",
			InvoiceDate:       now,
			DueDate:           dueDate,
			VATAccountID:      "acc-13311",
			IsStockInwardAuto: true,
			Lines: []usecasePurchase.CreatePurchaseInvoiceLineCommand{
				{
					LineOrder:       1,
					ItemID:          "item-01",
					WarehouseID:     &whID,
					DebitAccountID:  "acc-1561",
					CreditAccountID: "acc-331",
					Quantity:        decimal.NewFromInt(10),
					UnitPriceVND:    decimal.NewFromInt(1000000), // 10M
					VATRate:         decimal.NewFromInt(10),       // 1M
					Note:            "Mua hàng hóa",
				},
			},
			CreatedBy: "ktt",
		}

		pi, v, err := uc.CreatePurchaseInvoice(ctx, cmd)
		require.NoError(t, err)
		assert.NotEmpty(t, pi.ID)
		assert.Equal(t, "10000000", pi.SubtotalVND.String())
		assert.Equal(t, "1000000", pi.VATAmountVND.String())
		assert.Equal(t, "11000000", pi.TotalAmountVND.String())
		assert.Equal(t, gl.VoucherTypePurchase, v.VoucherType)

		// 2 voucher lines: item line (1561/331) and VAT line (13311/331)
		require.Len(t, v.Lines, 2)
		assert.Equal(t, "10000000", v.Lines[0].AmountVND.String())
		assert.Equal(t, "acc-1561", v.Lines[0].DebitAccountID)
		assert.Equal(t, "1000000", v.Lines[1].AmountVND.String())
		assert.Equal(t, "acc-13311", v.Lines[1].DebitAccountID)

		// Test Get
		fetched, err := uc.GetPurchaseInvoice(ctx, pi.ID)
		require.NoError(t, err)
		assert.Equal(t, pi.ID, fetched.ID)
	})

	t.Run("Create valid Purchase Invoice with 0% VAT (no VAT voucher line)", func(t *testing.T) {
		cmd := usecasePurchase.CreatePurchaseInvoiceCommand{
			CompanyProfileID:  "comp-01",
			VoucherNo:         "MH-2026-0002",
			VoucherDate:       now,
			PostedDate:        now,
			VendorID:          "vendor-01",
			InvoiceNo:         "00001002",
			InvoiceDate:       now,
			DueDate:           dueDate,
			IsStockInwardAuto: true,
			Lines: []usecasePurchase.CreatePurchaseInvoiceLineCommand{
				{
					LineOrder:       1,
					ItemID:          "item-01",
					DebitAccountID:  "acc-1561",
					CreditAccountID: "acc-331",
					Quantity:        decimal.NewFromInt(5),
					UnitPriceVND:    decimal.NewFromInt(2000000), // 10M
					VATRate:         decimal.Zero,                // 0% VAT
				},
			},
			CreatedBy: "ktt",
		}

		pi, v, err := uc.CreatePurchaseInvoice(ctx, cmd)
		require.NoError(t, err)
		assert.Equal(t, "10000000", pi.SubtotalVND.String())
		assert.Equal(t, "0", pi.VATAmountVND.String())
		assert.Equal(t, "10000000", pi.TotalAmountVND.String())

		// Only 1 voucher line since VAT is zero
		require.Len(t, v.Lines, 1)
	})

	t.Run("Reject missing VAT account when invoice has VAT", func(t *testing.T) {
		cmd := usecasePurchase.CreatePurchaseInvoiceCommand{
			CompanyProfileID:  "comp-01",
			VoucherNo:         "MH-2026-0003",
			VoucherDate:       now,
			PostedDate:        now,
			VendorID:          "vendor-01",
			InvoiceNo:         "00001003",
			InvoiceDate:       now,
			DueDate:           dueDate,
			VATAccountID:      "", // Missing
			IsStockInwardAuto: true,
			Lines: []usecasePurchase.CreatePurchaseInvoiceLineCommand{
				{
					LineOrder:       1,
					ItemID:          "item-01",
					DebitAccountID:  "acc-1561",
					CreditAccountID: "acc-331",
					Quantity:        decimal.NewFromInt(10),
					UnitPriceVND:    decimal.NewFromInt(1000000),
					VATRate:         decimal.NewFromInt(10),
				},
			},
			CreatedBy: "ktt",
		}

		_, _, err := uc.CreatePurchaseInvoice(ctx, cmd)
		require.ErrorIs(t, err, usecasePurchase.ErrVATAccountRequired)
	})

	t.Run("Idempotent submission returns existing record", func(t *testing.T) {
		idemp := "idemp-purchase-01"
		cmd := usecasePurchase.CreatePurchaseInvoiceCommand{
			CompanyProfileID: "comp-01",
			VoucherNo:        "MH-2026-IDEMP",
			VoucherDate:      now,
			PostedDate:       now,
			VendorID:         "vendor-01",
			InvoiceNo:        "00001004",
			InvoiceDate:      now,
			DueDate:          dueDate,
			VATAccountID:     "acc-13311",
			Lines: []usecasePurchase.CreatePurchaseInvoiceLineCommand{
				{
					ItemID:          "item-01",
					DebitAccountID:  "acc-1561",
					CreditAccountID: "acc-331",
					Quantity:        decimal.NewFromInt(1),
					UnitPriceVND:    decimal.NewFromInt(500000),
					VATRate:         decimal.NewFromInt(8),
				},
			},
			IdempotencyKey: &idemp,
			CreatedBy:      "ktt",
		}

		pi1, v1, err := uc.CreatePurchaseInvoice(ctx, cmd)
		require.NoError(t, err)

		pi2, v2, err := uc.CreatePurchaseInvoice(ctx, cmd)
		require.NoError(t, err)
		assert.Equal(t, pi1.ID, pi2.ID)
		assert.Equal(t, v1.ID, v2.ID)
	})

	t.Run("GetPurchaseInvoice not found error", func(t *testing.T) {
		_, err := uc.GetPurchaseInvoice(ctx, "non-existent")
		require.ErrorIs(t, err, purchase.ErrPurchaseInvoiceNotFound)
	})
}

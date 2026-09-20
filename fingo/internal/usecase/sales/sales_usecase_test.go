package sales_test

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
	"fingo/internal/domain/sales"
	spineDomain "fingo/internal/domain/spine"
	"fingo/internal/domain/system"
	glUseCase "fingo/internal/usecase/gl"
	usecaseSales "fingo/internal/usecase/sales"
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

func setupSalesTestEnv() (*usecaseSales.SalesUseCase, *sales.SalesRepositoryStub) {
	salesRepo := sales.NewSalesRepositoryStub()
	glRepo := &mockGLRepo{vouchers: make(map[string]*gl.Voucher)}
	accRepo := &mockAccountRepo{
		accounts: map[string]*spineDomain.Account{
			"acc-131":   {ID: "acc-131", Code: "131", Name: "Phải thu của khách hàng", IsLeaf: true, IsActive: true},
			"acc-5111":  {ID: "acc-5111", Code: "5111", Name: "Doanh thu bán hàng hóa", IsLeaf: true, IsActive: true},
			"acc-33311": {ID: "acc-33311", Code: "33311", Name: "Thuế GTGT đầu ra phải nộp", IsLeaf: true, IsActive: true},
		},
	}
	compRepo := &mockCompanyRepo{}
	log := logger.New(logger.Config{Writer: io.Discard})

	glUC := glUseCase.NewGLUseCase(glRepo, compRepo, accRepo, log)
	salesUC := usecaseSales.NewSalesUseCase(salesRepo, glUC, log)
	return salesUC, salesRepo
}

func TestSalesUseCase_Workflow(t *testing.T) {
	t.Parallel()
	uc, _ := setupSalesTestEnv()
	ctx := context.Background()
	now := time.Date(2026, 3, 20, 0, 0, 0, 0, time.UTC)
	dueDate := now.AddDate(0, 1, 0)
	whID := "wh-01"

	t.Run("Create valid Sales Invoice with discount and 10% VAT", func(t *testing.T) {
		cmd := usecaseSales.CreateSalesInvoiceCommand{
			CompanyProfileID:   "comp-01",
			VoucherNo:          "BH-2026-0001",
			VoucherDate:        now,
			PostedDate:         now,
			CustomerID:         "cust-01",
			InvoiceTemplate:    "1",
			InvoiceSeries:      "C26TAA",
			InvoiceNo:          "00000010",
			InvoiceDate:        now,
			DueDate:            dueDate,
			PaymentMethod:      "CK",
			VATAccountID:       "acc-33311",
			IsStockOutwardAuto: true,
			Lines: []usecaseSales.CreateSalesInvoiceLineCommand{
				{
					LineOrder:       1,
					ItemID:          "item-01",
					WarehouseID:     &whID,
					DebitAccountID:  "acc-131",
					CreditAccountID: "acc-5111",
					Quantity:        decimal.NewFromInt(10),
					UnitPriceVND:    decimal.NewFromInt(2000000), // 20M gross
					DiscountRate:    decimal.NewFromInt(5),       // 1M discount -> 19M net
					VATRate:         decimal.NewFromInt(10),      // 1.9M VAT
					Note:            "Bán hàng sỉ",
				},
			},
			CreatedBy: "ktt",
		}

		si, v, err := uc.CreateSalesInvoice(ctx, cmd)
		require.NoError(t, err)
		assert.NotEmpty(t, si.ID)
		assert.Equal(t, "20000000", si.SubtotalVND.String())
		assert.Equal(t, "1000000", si.DiscountVND.String())
		assert.Equal(t, "1900000", si.VATAmountVND.String())
		assert.Equal(t, "20900000", si.TotalAmountVND.String())
		assert.Equal(t, gl.VoucherTypeSales, v.VoucherType)

		// 2 voucher lines: net revenue (131/5111) and output VAT (131/33311)
		require.Len(t, v.Lines, 2)
		assert.Equal(t, "19000000", v.Lines[0].AmountVND.String())
		assert.Equal(t, "acc-5111", v.Lines[0].CreditAccountID)
		assert.Equal(t, "1900000", v.Lines[1].AmountVND.String())
		assert.Equal(t, "acc-33311", v.Lines[1].CreditAccountID)

		// Test Get
		fetched, err := uc.GetSalesInvoice(ctx, si.ID)
		require.NoError(t, err)
		assert.Equal(t, si.ID, fetched.ID)
	})

	t.Run("Create valid Sales Invoice with 0% VAT (no VAT voucher line)", func(t *testing.T) {
		cmd := usecaseSales.CreateSalesInvoiceCommand{
			CompanyProfileID:   "comp-01",
			VoucherNo:          "BH-2026-0002",
			VoucherDate:        now,
			PostedDate:         now,
			CustomerID:         "cust-01",
			InvoiceNo:          "00000020",
			InvoiceDate:        now,
			DueDate:            dueDate,
			IsStockOutwardAuto: true,
			Lines: []usecaseSales.CreateSalesInvoiceLineCommand{
				{
					LineOrder:       1,
					ItemID:          "item-01",
					DebitAccountID:  "acc-131",
					CreditAccountID: "acc-5111",
					Quantity:        decimal.NewFromInt(5),
					UnitPriceVND:    decimal.NewFromInt(1000000), // 5M
					VATRate:         decimal.Zero,                // 0% VAT
				},
			},
			CreatedBy: "ktt",
		}

		si, v, err := uc.CreateSalesInvoice(ctx, cmd)
		require.NoError(t, err)
		assert.Equal(t, "5000000", si.SubtotalVND.String())
		assert.Equal(t, "0", si.VATAmountVND.String())
		assert.Equal(t, "5000000", si.TotalAmountVND.String())

		// Only 1 voucher line since VAT is zero
		require.Len(t, v.Lines, 1)
	})

	t.Run("Reject missing VAT account when invoice has output VAT", func(t *testing.T) {
		cmd := usecaseSales.CreateSalesInvoiceCommand{
			CompanyProfileID:   "comp-01",
			VoucherNo:          "BH-2026-0003",
			VoucherDate:        now,
			PostedDate:         now,
			CustomerID:         "cust-01",
			InvoiceNo:          "00000030",
			InvoiceDate:        now,
			DueDate:            dueDate,
			VATAccountID:       "", // Missing
			IsStockOutwardAuto: true,
			Lines: []usecaseSales.CreateSalesInvoiceLineCommand{
				{
					LineOrder:       1,
					ItemID:          "item-01",
					DebitAccountID:  "acc-131",
					CreditAccountID: "acc-5111",
					Quantity:        decimal.NewFromInt(10),
					UnitPriceVND:    decimal.NewFromInt(1000000),
					VATRate:         decimal.NewFromInt(10),
				},
			},
			CreatedBy: "ktt",
		}

		_, _, err := uc.CreateSalesInvoice(ctx, cmd)
		require.ErrorIs(t, err, usecaseSales.ErrVATAccountRequired)
	})

	t.Run("Idempotent submission returns existing record", func(t *testing.T) {
		idemp := "idemp-sales-01"
		cmd := usecaseSales.CreateSalesInvoiceCommand{
			CompanyProfileID: "comp-01",
			VoucherNo:        "BH-2026-IDEMP",
			VoucherDate:      now,
			PostedDate:       now,
			CustomerID:       "cust-01",
			InvoiceNo:        "00000040",
			InvoiceDate:      now,
			DueDate:          dueDate,
			VATAccountID:     "acc-33311",
			Lines: []usecaseSales.CreateSalesInvoiceLineCommand{
				{
					ItemID:          "item-01",
					DebitAccountID:  "acc-131",
					CreditAccountID: "acc-5111",
					Quantity:        decimal.NewFromInt(1),
					UnitPriceVND:    decimal.NewFromInt(500000),
					VATRate:         decimal.NewFromInt(8),
				},
			},
			IdempotencyKey: &idemp,
			CreatedBy:      "ktt",
		}

		si1, v1, err := uc.CreateSalesInvoice(ctx, cmd)
		require.NoError(t, err)

		si2, v2, err := uc.CreateSalesInvoice(ctx, cmd)
		require.NoError(t, err)
		assert.Equal(t, si1.ID, si2.ID)
		assert.Equal(t, v1.ID, v2.ID)
	})

	t.Run("GetSalesInvoice not found error", func(t *testing.T) {
		_, err := uc.GetSalesInvoice(ctx, "non-existent")
		require.ErrorIs(t, err, sales.ErrSalesInvoiceNotFound)
	})
}

package gl_test

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
	spineDomain "fingo/internal/domain/spine"
	"fingo/internal/domain/system"
	usecaseGL "fingo/internal/usecase/gl"
	"fingo/pkg/logger"
)

// Mock GL Repository
type mockGLRepo struct {
	vouchers map[string]*gl.Voucher
}

func newMockGLRepo() *mockGLRepo {
	return &mockGLRepo{
		vouchers: make(map[string]*gl.Voucher),
	}
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
	var result []gl.Voucher
	for _, v := range m.vouchers {
		if v.CompanyProfileID == companyID && !v.VoucherDate.Before(fromDate) && !v.VoucherDate.After(toDate) {
			result = append(result, *v)
		}
	}
	return result, nil
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
type mockCompanyRepo struct {
	profile *system.ProductionCompanyProfile
}

func (m *mockCompanyRepo) GetProfile(ctx context.Context) (*system.ProductionCompanyProfile, error) {
	return m.profile, nil
}

func setupGLTestEnv() (*usecaseGL.GLUseCase, *mockGLRepo, *mockAccountRepo, *mockCompanyRepo) {
	glRepo := newMockGLRepo()
	accRepo := &mockAccountRepo{
		accounts: map[string]*spineDomain.Account{
			"acc-1111": {
				ID:       "acc-1111",
				Code:     "1111",
				Name:     "Tiền mặt VND",
				IsLeaf:   true,
				IsActive: true,
			},
			"acc-1121": {
				ID:       "acc-1121",
				Code:     "1121",
				Name:     "Tiền gửi ngân hàng VND",
				IsLeaf:   true,
				IsActive: true,
			},
			"acc-112": {
				ID:       "acc-112",
				Code:     "112",
				Name:     "Tiền gửi ngân hàng (Parent)",
				IsLeaf:   false, // Parent non-leaf
				IsActive: true,
			},
			"acc-6421": {
				ID:       "acc-6421",
				Code:     "6421",
				Name:     "Chi phí bán hàng",
				IsLeaf:   true,
				IsActive: true,
			},
		},
	}
	compRepo := &mockCompanyRepo{
		profile: &system.ProductionCompanyProfile{
			ID:       "comp-01",
			LockDate: time.Date(2026, 1, 31, 0, 0, 0, 0, time.UTC),
		},
	}

	uc := usecaseGL.NewGLUseCase(glRepo, compRepo, accRepo, logger.New(logger.Config{Writer: io.Discard}))
	return uc, glRepo, accRepo, compRepo
}

func TestGLUseCase_Workflow(t *testing.T) {
	uc, _, _, _ := setupGLTestEnv()
	ctx := context.Background()
	vDate := time.Date(2026, 3, 20, 0, 0, 0, 0, time.UTC)
	costCenter := "CC_SALES"

	t.Run("Create valid voucher successfully", func(t *testing.T) {
		v, err := uc.CreateVoucher(ctx, usecaseGL.CreateVoucherCommand{
			CompanyProfileID: "comp-01",
			VoucherNo:        "PKT-2026-0001",
			VoucherDate:      vDate,
			VoucherType:      gl.VoucherTypeGeneral,
			Description:      "Rút tiền gửi về nhập quỹ tiền mặt",
			CreatedBy:        "ktt_lead",
			Lines: []usecaseGL.CreateVoucherLineCommand{
				{
					DebitAccountID:  "acc-1111",
					CreditAccountID: "acc-1121",
					AmountVND:       decimal.NewFromInt(50000000),
					Note:            "Rút tiền ngân hàng về quỹ",
				},
			},
		})

		require.NoError(t, err)
		assert.Equal(t, "PKT-2026-0001", v.VoucherNo)
		assert.Equal(t, gl.VoucherStatusDraft, v.Status)
		assert.Equal(t, "50000000", v.TotalDebit.String())
		assert.Equal(t, "50000000", v.TotalCredit.String())

		// Test Post
		err = uc.PostVoucher(ctx, v.ID)
		require.NoError(t, err)

		fetched, err := uc.GetVoucherByID(ctx, v.ID)
		require.NoError(t, err)
		assert.Equal(t, gl.VoucherStatusPosted, fetched.Status)

		// Test Storno Reversal
		revVoucher, err := uc.ReverseVoucher(ctx, v.ID, "PKT-2026-0001-REV", vDate.AddDate(0, 0, 1), "ktt_lead")
		require.NoError(t, err)
		assert.Equal(t, "-50000000", revVoucher.TotalDebit.String())
		assert.Equal(t, gl.VoucherStatusDraft, revVoucher.Status)

		// Test Cancel
		err = uc.CancelVoucher(ctx, revVoucher.ID)
		require.NoError(t, err)

		cancelled, err := uc.GetVoucherByID(ctx, revVoucher.ID)
		require.NoError(t, err)
		assert.Equal(t, gl.VoucherStatusCancelled, cancelled.Status)
	})

	t.Run("Reject voucher on or before lock date (INV-OPS-04)", func(t *testing.T) {
		lockedDate := time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC)
		_, err := uc.CreateVoucher(ctx, usecaseGL.CreateVoucherCommand{
			CompanyProfileID: "comp-01",
			VoucherDate:      lockedDate,
			Lines: []usecaseGL.CreateVoucherLineCommand{
				{
					DebitAccountID:  "acc-1111",
					CreditAccountID: "acc-1121",
					AmountVND:       decimal.NewFromInt(1000000),
				},
			},
		})
		require.ErrorIs(t, err, gl.ErrPeriodLocked)
	})

	t.Run("Reject non-leaf account posting (INV-OPS-02)", func(t *testing.T) {
		_, err := uc.CreateVoucher(ctx, usecaseGL.CreateVoucherCommand{
			CompanyProfileID: "comp-01",
			VoucherDate:      vDate,
			Lines: []usecaseGL.CreateVoucherLineCommand{
				{
					DebitAccountID:  "acc-1111",
					CreditAccountID: "acc-112", // Parent non-leaf account
					AmountVND:       decimal.NewFromInt(1000000),
				},
			},
		})
		require.ErrorIs(t, err, usecaseGL.ErrNonLeafPostingAccount)
	})

	t.Run("Reject non-existent account", func(t *testing.T) {
		_, err := uc.CreateVoucher(ctx, usecaseGL.CreateVoucherCommand{
			CompanyProfileID: "comp-01",
			VoucherDate:      vDate,
			Lines: []usecaseGL.CreateVoucherLineCommand{
				{
					DebitAccountID:  "acc-non-existent",
					CreditAccountID: "acc-1121",
					AmountVND:       decimal.NewFromInt(1000000),
				},
			},
		})
		require.ErrorIs(t, err, usecaseGL.ErrAccountNotFound)
	})

	t.Run("Query methods and error paths", func(t *testing.T) {
		_, err := uc.GetVoucherByID(ctx, "non-existent-id")
		require.ErrorIs(t, err, usecaseGL.ErrVoucherNotFound)

		_, err = uc.GetVoucherByNo(ctx, "comp-01", gl.VoucherTypeGeneral, "NO-SUCH-NO")
		require.ErrorIs(t, err, usecaseGL.ErrVoucherNotFound)

		err = uc.PostVoucher(ctx, "non-existent-id")
		require.ErrorIs(t, err, usecaseGL.ErrVoucherNotFound)

		err = uc.CancelVoucher(ctx, "non-existent-id")
		require.ErrorIs(t, err, usecaseGL.ErrVoucherNotFound)

		_, err = uc.ReverseVoucher(ctx, "non-existent-id", "REV-01", vDate, "ktt")
		require.ErrorIs(t, err, usecaseGL.ErrVoucherNotFound)
	})

	t.Run("List vouchers by period", func(t *testing.T) {
		list, err := uc.ListVouchersByPeriod(ctx, "comp-01", vDate.AddDate(0, 0, -10), vDate.AddDate(0, 0, 10))
		require.NoError(t, err)
		assert.NotEmpty(t, list)
	})

	t.Run("Enforce cost center on expense account (INV-OPS-03)", func(t *testing.T) {
		_, err := uc.CreateVoucher(ctx, usecaseGL.CreateVoucherCommand{
			CompanyProfileID: "comp-01",
			VoucherDate:      vDate,
			Lines: []usecaseGL.CreateVoucherLineCommand{
				{
					DebitAccountID:  "acc-6421",
					CreditAccountID: "acc-1111",
					AmountVND:       decimal.NewFromInt(5000000),
					CostCenterID:    nil, // Missing
				},
			},
		})
		require.ErrorIs(t, err, gl.ErrCostCenterRequired)

		// With cost center
		v, err := uc.CreateVoucher(ctx, usecaseGL.CreateVoucherCommand{
			CompanyProfileID: "comp-01",
			VoucherDate:      vDate,
			Lines: []usecaseGL.CreateVoucherLineCommand{
				{
					DebitAccountID:  "acc-6421",
					CreditAccountID: "acc-1111",
					AmountVND:       decimal.NewFromInt(5000000),
					CostCenterID:    &costCenter,
				},
			},
		})
		require.NoError(t, err)
		assert.Equal(t, "5000000", v.TotalDebit.String())
	})

	t.Run("Credit account error paths", func(t *testing.T) {
		// Credit account non-existent
		_, err := uc.CreateVoucher(ctx, usecaseGL.CreateVoucherCommand{
			CompanyProfileID: "comp-01",
			VoucherDate:      vDate,
			Lines: []usecaseGL.CreateVoucherLineCommand{
				{
					DebitAccountID:  "acc-1111",
					CreditAccountID: "acc-missing-credit",
					AmountVND:       decimal.NewFromInt(1000000),
				},
			},
		})
		require.ErrorIs(t, err, usecaseGL.ErrAccountNotFound)

		// Credit account non-leaf
		_, err = uc.CreateVoucher(ctx, usecaseGL.CreateVoucherCommand{
			CompanyProfileID: "comp-01",
			VoucherDate:      vDate,
			Lines: []usecaseGL.CreateVoucherLineCommand{
				{
					DebitAccountID:  "acc-1111",
					CreditAccountID: "acc-112",
					AmountVND:       decimal.NewFromInt(1000000),
				},
			},
		})
		require.ErrorIs(t, err, usecaseGL.ErrNonLeafPostingAccount)

		// Zero amount
		_, err = uc.CreateVoucher(ctx, usecaseGL.CreateVoucherCommand{
			CompanyProfileID: "comp-01",
			VoucherDate:      vDate,
			Lines: []usecaseGL.CreateVoucherLineCommand{
				{
					DebitAccountID:  "acc-1111",
					CreditAccountID: "acc-1121",
					AmountVND:       decimal.Zero,
				},
			},
		})
		require.ErrorIs(t, err, gl.ErrZeroVoucherAmount)
	})

	t.Run("Status transition rejection", func(t *testing.T) {
		v, err := uc.CreateVoucher(ctx, usecaseGL.CreateVoucherCommand{
			CompanyProfileID: "comp-01",
			VoucherDate:      vDate,
			Lines: []usecaseGL.CreateVoucherLineCommand{
				{
					DebitAccountID:  "acc-1111",
					CreditAccountID: "acc-1121",
					AmountVND:       decimal.NewFromInt(1000000),
				},
			},
		})
		require.NoError(t, err)

		// Post twice
		require.NoError(t, uc.PostVoucher(ctx, v.ID))
		err = uc.PostVoucher(ctx, v.ID)
		require.ErrorIs(t, err, gl.ErrVoucherAlreadyPosted)

		// Cancel twice
		v2, err := uc.CreateVoucher(ctx, usecaseGL.CreateVoucherCommand{
			CompanyProfileID: "comp-01",
			VoucherDate:      vDate,
			Lines: []usecaseGL.CreateVoucherLineCommand{
				{
					DebitAccountID:  "acc-1111",
					CreditAccountID: "acc-1121",
					AmountVND:       decimal.NewFromInt(1000000),
				},
			},
		})
		require.NoError(t, err)

		require.NoError(t, uc.CancelVoucher(ctx, v2.ID))
		err = uc.CancelVoucher(ctx, v2.ID)
		require.ErrorIs(t, err, gl.ErrVoucherAlreadyCancelled)
	})

	t.Run("Idempotency returns existing voucher on duplicate submission (INV-OPS-14)", func(t *testing.T) {
		idempKey := "idemp-test-01"
		cmd := usecaseGL.CreateVoucherCommand{
			CompanyProfileID: "comp-01",
			VoucherDate:      vDate,
			IdempotencyKey:   &idempKey,
			Lines: []usecaseGL.CreateVoucherLineCommand{
				{
					DebitAccountID:  "acc-1111",
					CreditAccountID: "acc-1121",
					AmountVND:       decimal.NewFromInt(1000000),
				},
			},
		}
		v1, err := uc.CreateVoucher(ctx, cmd)
		require.NoError(t, err)
		v2, err := uc.CreateVoucher(ctx, cmd)
		require.NoError(t, err)
		assert.Equal(t, v1.ID, v2.ID)
	})
}

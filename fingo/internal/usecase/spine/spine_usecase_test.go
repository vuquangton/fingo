package spine_test

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"testing"
	"time"

	domain "fingo/internal/domain/spine"
	usecase "fingo/internal/usecase/spine"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- In-Memory Stubs ---

type memoryCurrencyRepo struct {
	mu         sync.RWMutex
	currencies map[string]*domain.Currency
}

func newMemoryCurrencyRepo() *memoryCurrencyRepo {
	return &memoryCurrencyRepo{currencies: make(map[string]*domain.Currency)}
}

func (m *memoryCurrencyRepo) SaveCurrency(ctx context.Context, c *domain.Currency) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.currencies[c.Code] = c
	return nil
}

func (m *memoryCurrencyRepo) GetCurrencyByCode(ctx context.Context, companyID, code string) (*domain.Currency, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	c, ok := m.currencies[code]
	if !ok {
		return nil, sql.ErrNoRows
	}
	return c, nil
}

func (m *memoryCurrencyRepo) GetBaseCurrency(ctx context.Context, companyID string) (*domain.Currency, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, c := range m.currencies {
		if c.IsBase {
			return c, nil
		}
	}
	return nil, sql.ErrNoRows
}

func (m *memoryCurrencyRepo) ListCurrencies(ctx context.Context, companyID string) ([]domain.Currency, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	list := make([]domain.Currency, 0, len(m.currencies))
	for _, c := range m.currencies {
		list = append(list, *c)
	}
	return list, nil
}

type memoryExchangeRateRepo struct {
	mu    sync.RWMutex
	rates map[string]*domain.ExchangeRate
}

func newMemoryExchangeRateRepo() *memoryExchangeRateRepo {
	return &memoryExchangeRateRepo{rates: make(map[string]*domain.ExchangeRate)}
}

func (m *memoryExchangeRateRepo) SaveExchangeRate(ctx context.Context, rate *domain.ExchangeRate) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	k := fmt.Sprintf("%s:%s", rate.CurrencyCode, rate.RateType)
	m.rates[k] = rate
	return nil
}

func (m *memoryExchangeRateRepo) GetEffectiveRate(ctx context.Context, companyID, currCode string, rateDate time.Time, rType domain.RateType) (*domain.ExchangeRate, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	k := fmt.Sprintf("%s:%s", currCode, rType)
	r, ok := m.rates[k]
	if !ok {
		return nil, sql.ErrNoRows
	}
	return r, nil
}

func (m *memoryExchangeRateRepo) ListRatesByDate(ctx context.Context, companyID string, rateDate time.Time) ([]domain.ExchangeRate, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	list := make([]domain.ExchangeRate, 0, len(m.rates))
	for _, r := range m.rates {
		list = append(list, *r)
	}
	return list, nil
}

type memoryAccountRepo struct {
	mu       sync.RWMutex
	accounts map[string]*domain.Account
}

func newMemoryAccountRepo() *memoryAccountRepo {
	return &memoryAccountRepo{accounts: make(map[string]*domain.Account)}
}

func (m *memoryAccountRepo) SaveAccount(ctx context.Context, acc *domain.Account) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.accounts[acc.Code] = acc
	return nil
}

func (m *memoryAccountRepo) GetAccountByID(ctx context.Context, id string) (*domain.Account, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, a := range m.accounts {
		if a.ID == id {
			return a, nil
		}
	}
	return nil, sql.ErrNoRows
}

func (m *memoryAccountRepo) GetAccountByCode(ctx context.Context, companyID, code string) (*domain.Account, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	a, ok := m.accounts[code]
	if !ok {
		return nil, sql.ErrNoRows
	}
	return a, nil
}

func (m *memoryAccountRepo) ListAccounts(ctx context.Context, companyID string) ([]domain.Account, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	list := make([]domain.Account, 0, len(m.accounts))
	for _, a := range m.accounts {
		list = append(list, *a)
	}
	return list, nil
}

func (m *memoryAccountRepo) ListChildAccounts(ctx context.Context, parentID string) ([]domain.Account, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	list := make([]domain.Account, 0)
	for _, a := range m.accounts {
		if a.ParentID != nil && *a.ParentID == parentID {
			list = append(list, *a)
		}
	}
	return list, nil
}

func (m *memoryAccountRepo) UpdateLeafStatus(ctx context.Context, id string, isLeaf bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, a := range m.accounts {
		if a.ID == id {
			a.IsLeaf = isLeaf
			return nil
		}
	}
	return sql.ErrNoRows
}

type memoryPeriodRepo struct {
	mu      sync.RWMutex
	periods map[string]*domain.AccountingPeriod
}

func newMemoryPeriodRepo() *memoryPeriodRepo {
	return &memoryPeriodRepo{periods: make(map[string]*domain.AccountingPeriod)}
}

func (m *memoryPeriodRepo) SaveFiscalYear(ctx context.Context, fy *domain.FiscalYear) error {
	return nil
}

func (m *memoryPeriodRepo) GetFiscalYearByYear(ctx context.Context, companyID string, year int) (*domain.FiscalYear, error) {
	return nil, nil
}

func (m *memoryPeriodRepo) SavePeriod(ctx context.Context, p *domain.AccountingPeriod) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.periods[p.ID] = p
	return nil
}

func (m *memoryPeriodRepo) GetPeriodByDate(ctx context.Context, companyID string, date time.Time) (*domain.AccountingPeriod, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, p := range m.periods {
		if !date.Before(p.StartDate) && !date.After(p.EndDate) {
			return p, nil
		}
	}
	return nil, sql.ErrNoRows
}

func (m *memoryPeriodRepo) ListPeriodsByFiscalYear(ctx context.Context, fyID string) ([]domain.AccountingPeriod, error) {
	return nil, nil
}

func (m *memoryPeriodRepo) UpdatePeriodLock(ctx context.Context, id string, lockDate time.Time, status domain.PeriodStatus) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	p, ok := m.periods[id]
	if !ok {
		return sql.ErrNoRows
	}
	p.LockDate = lockDate
	p.Status = status
	return nil
}

type memoryCostDimensionRepo struct {
	mu sync.RWMutex
}

func newMemoryCostDimensionRepo() *memoryCostDimensionRepo {
	return &memoryCostDimensionRepo{}
}

func (m *memoryCostDimensionRepo) SaveCostCenter(ctx context.Context, cc *domain.CostCenter) error {
	return nil
}
func (m *memoryCostDimensionRepo) GetCostCenterByCode(ctx context.Context, companyID, code string) (*domain.CostCenter, error) {
	return nil, nil
}
func (m *memoryCostDimensionRepo) ListCostCenters(ctx context.Context, companyID string) ([]domain.CostCenter, error) {
	return nil, nil
}
func (m *memoryCostDimensionRepo) SaveExpenseItem(ctx context.Context, ei *domain.ExpenseItem) error {
	return nil
}
func (m *memoryCostDimensionRepo) GetExpenseItemByCode(ctx context.Context, companyID, code string) (*domain.ExpenseItem, error) {
	return nil, nil
}
func (m *memoryCostDimensionRepo) ListExpenseItems(ctx context.Context, companyID string) ([]domain.ExpenseItem, error) {
	return nil, nil
}

// --- Test Suite ---

func TestSpineUseCase_CreateSubAccount(t *testing.T) {
	t.Parallel()

	currRepo := newMemoryCurrencyRepo()
	fxRepo := newMemoryExchangeRateRepo()
	accRepo := newMemoryAccountRepo()
	periodRepo := newMemoryPeriodRepo()
	costRepo := newMemoryCostDimensionRepo()

	uc := usecase.NewSpineUseCase(currRepo, fxRepo, accRepo, periodRepo, costRepo)
	ctx := context.Background()

	companyID := "comp-01"

	// 1. Seed Parent Account 111 (Initially leaf)
	parentID := "acc-p-111"
	parentAcc, err := domain.NewAccount(parentID, companyID, "111", "Tiền mặt", nil, 1, domain.NatureDebit, domain.CategoryAsset, true)
	require.NoError(t, err)
	_ = accRepo.SaveAccount(ctx, parentAcc)

	t.Run("Fails if parent does not exist", func(t *testing.T) {
		_, err := uc.CreateSubAccount(ctx, usecase.CreateSubAccountCommand{
			CompanyProfileID:  companyID,
			ParentAccountCode: "999",
			SubAccountCode:    "9991",
			Name:              "Nonexistent",
			Nature:            domain.NatureDebit,
			Category:          domain.CategoryAsset,
		})
		require.ErrorIs(t, err, usecase.ErrParentAccountNotFound)
	})

	t.Run("Successfully creates child and marks parent is_leaf = false", func(t *testing.T) {
		subAcc, err := uc.CreateSubAccount(ctx, usecase.CreateSubAccountCommand{
			CompanyProfileID:  companyID,
			ParentAccountCode: "111",
			SubAccountCode:    "1111",
			Name:              "Tiền Việt Nam",
			Nature:            domain.NatureDebit,
			Category:          domain.CategoryAsset,
		})
		require.NoError(t, err)
		assert.Equal(t, "1111", subAcc.Code)
		assert.True(t, subAcc.IsLeaf)
		assert.Equal(t, 2, subAcc.AccountLevel)

		// Verify parent is now is_leaf = false
		updatedParent, err := accRepo.GetAccountByCode(ctx, companyID, "111")
		require.NoError(t, err)
		assert.False(t, updatedParent.IsLeaf, "Parent account must become non-leaf once child is added")
	})

	t.Run("Idempotent submission returns existing account without error", func(t *testing.T) {
		subAcc, err := uc.CreateSubAccount(ctx, usecase.CreateSubAccountCommand{
			IdempotencyKey:    "idem-key-1111",
			CompanyProfileID:  companyID,
			ParentAccountCode: "111",
			SubAccountCode:    "1111",
			Name:              "Tiền Việt Nam",
			Nature:            domain.NatureDebit,
			Category:          domain.CategoryAsset,
		})
		require.NoError(t, err)
		assert.Equal(t, "1111", subAcc.Code)
	})
}

func TestSpineUseCase_ExchangeRates_And_Revaluation(t *testing.T) {
	t.Parallel()

	currRepo := newMemoryCurrencyRepo()
	fxRepo := newMemoryExchangeRateRepo()
	accRepo := newMemoryAccountRepo()
	periodRepo := newMemoryPeriodRepo()
	costRepo := newMemoryCostDimensionRepo()

	uc := usecase.NewSpineUseCase(currRepo, fxRepo, accRepo, periodRepo, costRepo)
	ctx := context.Background()

	companyID := "comp-01"

	// Seed Base Currency VND and Foreign Currency USD
	_ = currRepo.SaveCurrency(ctx, &domain.Currency{
		Code: "VND", CompanyProfileID: companyID, IsBase: true, DecimalPlaces: 0,
	})
	_ = currRepo.SaveCurrency(ctx, &domain.Currency{
		Code: "USD", CompanyProfileID: companyID, IsBase: false, DecimalPlaces: 2,
	})

	// Seed USD Exchange Rate (25,450 VND/USD)
	d := time.Date(2026, 3, 31, 0, 0, 0, 0, time.UTC)
	_ = fxRepo.SaveExchangeRate(ctx, &domain.ExchangeRate{
		ID: "fx-01", CompanyProfileID: companyID, CurrencyCode: "USD",
		RateDate: d, RateType: domain.RateTypeBuyTransfer, Rate: decimal.RequireFromString("25450.0"),
	})

	t.Run("Base currency rate is always exactly 1.0", func(t *testing.T) {
		rate, err := uc.GetEffectiveExchangeRate(ctx, companyID, "VND", d, domain.RateTypeBuyTransfer)
		require.NoError(t, err)
		assert.Equal(t, "1", rate.String())
	})

	t.Run("Foreign currency rate resolves stored rate", func(t *testing.T) {
		rate, err := uc.GetEffectiveExchangeRate(ctx, companyID, "USD", d, domain.RateTypeBuyTransfer)
		require.NoError(t, err)
		assert.Equal(t, "25450", rate.String())
	})

	t.Run("RevalueForeignCurrency: Unrealized Gain (IsGain = true)", func(t *testing.T) {
		// 10,000 USD cash, book value 250,000,000 VND -> Revalued: 254,500,000 VND -> Gain: +4,500,000 VND
		res, err := uc.RevalueForeignCurrency(ctx, usecase.RevalueForeignCurrencyCommand{
			CompanyProfileID: companyID,
			AccountCode:      "1112",
			CurrencyCode:     "USD",
			ForeignAmount:    decimal.NewFromInt(10_000),
			BookValueVND:     decimal.NewFromInt(250_000_000),
			RevaluationDate:  d,
			RateType:         domain.RateTypeBuyTransfer,
		})
		require.NoError(t, err)
		assert.True(t, res.IsGain)
		assert.Equal(t, "4500000", res.DiffVND.String())
		assert.Equal(t, "1112", res.DebitAccount)
		assert.Equal(t, "4131", res.CreditAccount)
	})

	t.Run("RevalueForeignCurrency: Unrealized Loss (IsGain = false)", func(t *testing.T) {
		// 10,000 USD cash, book value 256,000,000 VND -> Revalued: 254,500,000 VND -> Loss: -1,500,000 VND
		res, err := uc.RevalueForeignCurrency(ctx, usecase.RevalueForeignCurrencyCommand{
			CompanyProfileID: companyID,
			AccountCode:      "1112",
			CurrencyCode:     "USD",
			ForeignAmount:    decimal.NewFromInt(10_000),
			BookValueVND:     decimal.NewFromInt(256_000_000),
			RevaluationDate:  d,
			RateType:         domain.RateTypeBuyTransfer,
		})
		require.NoError(t, err)
		assert.False(t, res.IsGain)
		assert.Equal(t, "1500000", res.DiffVND.String())
		assert.Equal(t, "4131", res.DebitAccount)
		assert.Equal(t, "1112", res.CreditAccount)
	})

	t.Run("RevalueForeignCurrency: Liability account (TK 331) rate increase is Loss", func(t *testing.T) {
		// 10,000 USD debt, book value 250,000,000 VND -> Revalued: 254,500,000 VND -> Company owes more -> Loss
		res, err := uc.RevalueForeignCurrency(ctx, usecase.RevalueForeignCurrencyCommand{
			CompanyProfileID: companyID,
			AccountCode:      "331",
			CurrencyCode:     "USD",
			ForeignAmount:    decimal.NewFromInt(10_000),
			BookValueVND:     decimal.NewFromInt(250_000_000),
			RevaluationDate:  d,
			RateType:         domain.RateTypeBuyTransfer,
		})
		require.NoError(t, err)
		assert.False(t, res.IsGain)
		assert.Equal(t, "4500000", res.DiffVND.String())
		assert.Equal(t, "4131", res.DebitAccount)
		assert.Equal(t, "331", res.CreditAccount)
	})
}

func TestSpineUseCase_ValidateVoucherDimensions(t *testing.T) {
	t.Parallel()

	currRepo := newMemoryCurrencyRepo()
	fxRepo := newMemoryExchangeRateRepo()
	accRepo := newMemoryAccountRepo()
	periodRepo := newMemoryPeriodRepo()
	costRepo := newMemoryCostDimensionRepo()

	uc := usecase.NewSpineUseCase(currRepo, fxRepo, accRepo, periodRepo, costRepo)
	ctx := context.Background()

	companyID := "comp-01"

	// Seed Period 1 (Tháng 01/2026, LockDate: 2026-01-31)
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 1, 31, 0, 0, 0, 0, time.UTC)
	lock := time.Date(2026, 1, 31, 0, 0, 0, 0, time.UTC)
	_ = periodRepo.SavePeriod(ctx, &domain.AccountingPeriod{
		ID: "p-01", CompanyProfileID: companyID, StartDate: start, EndDate: end, LockDate: lock,
	})

	// Seed Accounts: Parent 111, Leaf 1111, Expense 6422 (requires both dimensions)
	_ = accRepo.SaveAccount(ctx, &domain.Account{
		Code: "111", Name: "Tiền mặt", IsLeaf: false, IsActive: true,
	})
	_ = accRepo.SaveAccount(ctx, &domain.Account{
		Code: "1111", Name: "Tiền VN", IsLeaf: true, IsActive: true,
	})
	_ = accRepo.SaveAccount(ctx, &domain.Account{
		Code: "6422", Name: "Chi phí QLDN", IsLeaf: true, IsActive: true,
		RequiresCostCenter: true, RequiresExpenseItem: true,
	})

	t.Run("Rejects voucher when date is on or before period lock date (INV-SPINE-04)", func(t *testing.T) {
		vDateLocked := time.Date(2026, 1, 20, 0, 0, 0, 0, time.UTC)
		err := uc.ValidateVoucherDimensions(ctx, companyID, vDateLocked, []usecase.VoucherLineAllocation{
			{AccountCode: "1111"},
		})
		require.ErrorIs(t, err, domain.ErrPeriodLocked)
	})

	t.Run("Rejects posting to parent account (INV-SPINE-01)", func(t *testing.T) {
		vDateAllowed := time.Date(2026, 2, 5, 0, 0, 0, 0, time.UTC)
		err := uc.ValidateVoucherDimensions(ctx, companyID, vDateAllowed, []usecase.VoucherLineAllocation{
			{AccountCode: "111"},
		})
		require.ErrorIs(t, err, domain.ErrPostingToParentAccount)
	})

	t.Run("Rejects posting without mandatory expense dimensions (INV-SPINE-07)", func(t *testing.T) {
		vDateAllowed := time.Date(2026, 2, 5, 0, 0, 0, 0, time.UTC)
		err := uc.ValidateVoucherDimensions(ctx, companyID, vDateAllowed, []usecase.VoucherLineAllocation{
			{AccountCode: "6422", CostCenterID: nil, ExpenseItemID: nil},
		})
		require.Error(t, err)
	})

	t.Run("Accepts valid voucher lines with compliant dimensions", func(t *testing.T) {
		vDateAllowed := time.Date(2026, 2, 5, 0, 0, 0, 0, time.UTC)
		ccID := "cc-01"
		eiID := "ei-01"
		err := uc.ValidateVoucherDimensions(ctx, companyID, vDateAllowed, []usecase.VoucherLineAllocation{
			{AccountCode: "1111"},
			{AccountCode: "6422", CostCenterID: &ccID, ExpenseItemID: &eiID},
		})
		require.NoError(t, err)
	})

	t.Run("Rejects line with non-existent account", func(t *testing.T) {
		vDateAllowed := time.Date(2026, 2, 5, 0, 0, 0, 0, time.UTC)
		err := uc.ValidateVoucherDimensions(ctx, companyID, vDateAllowed, []usecase.VoucherLineAllocation{
			{AccountCode: "9999"},
		})
		require.ErrorIs(t, err, usecase.ErrAccountNotFound)
	})
}

func TestSpineUseCase_PeriodLockDate_And_Logger(t *testing.T) {
	t.Parallel()

	currRepo := newMemoryCurrencyRepo()
	fxRepo := newMemoryExchangeRateRepo()
	accRepo := newMemoryAccountRepo()
	periodRepo := newMemoryPeriodRepo()
	costRepo := newMemoryCostDimensionRepo()

	uc := usecase.NewSpineUseCase(currRepo, fxRepo, accRepo, periodRepo, costRepo)
	ctx := context.Background()

	companyID := "comp-01"

	// Seed period
	periodID := "p-lock-01"
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 1, 31, 0, 0, 0, 0, time.UTC)
	_ = periodRepo.SavePeriod(ctx, &domain.AccountingPeriod{
		ID: periodID, CompanyProfileID: companyID, StartDate: start, EndDate: end, LockDate: end,
	})

	t.Run("SetPeriodLockDate updates period lock", func(t *testing.T) {
		newLock := time.Date(2026, 2, 15, 0, 0, 0, 0, time.UTC)
		err := uc.SetPeriodLockDate(ctx, companyID, periodID, newLock)
		require.NoError(t, err)
	})

	t.Run("SetPeriodLockDate fails on nonexistent period", func(t *testing.T) {
		newLock := time.Date(2026, 2, 15, 0, 0, 0, 0, time.UTC)
		err := uc.SetPeriodLockDate(ctx, companyID, "nonexistent-p", newLock)
		require.Error(t, err)
	})

	t.Run("WithLogger coverage", func(t *testing.T) {
		ucWithLogger := uc.WithLogger(nil)
		assert.NotNil(t, ucWithLogger)
	})
}

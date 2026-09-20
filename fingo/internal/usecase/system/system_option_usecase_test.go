package system_test

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"testing"
	"time"

	domain "fingo/internal/domain/system"
	usecase "fingo/internal/usecase/system"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type memorySystemOptionRepo struct {
	mu      sync.RWMutex
	options map[string]*domain.SystemOption
	history []domain.SystemConfigHistory
}

func newMemorySystemOptionRepo() *memorySystemOptionRepo {
	return &memorySystemOptionRepo{
		options: make(map[string]*domain.SystemOption),
		history: make([]domain.SystemConfigHistory, 0),
	}
}

func (m *memorySystemOptionRepo) makeKey(companyID string, branchID *string, key string) string {
	b := "COMPANY"
	if branchID != nil && *branchID != "" {
		b = *branchID
	}
	return fmt.Sprintf("%s:%s:%s", companyID, b, key)
}

func (m *memorySystemOptionRepo) UpsertOption(ctx context.Context, opt *domain.SystemOption) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	k := m.makeKey(opt.CompanyProfileID, opt.BranchID, opt.OptionKey)
	m.options[k] = opt
	return nil
}

func (m *memorySystemOptionRepo) GetOption(ctx context.Context, companyID string, branchID *string, key string) (*domain.SystemOption, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	k := m.makeKey(companyID, branchID, key)
	opt, ok := m.options[k]
	if !ok {
		return nil, sql.ErrNoRows
	}
	return opt, nil
}

func (m *memorySystemOptionRepo) GetEffectiveOption(ctx context.Context, companyID string, branchID *string, key string) (*domain.SystemOption, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if branchID != nil && *branchID != "" {
		branchKey := m.makeKey(companyID, branchID, key)
		if opt, ok := m.options[branchKey]; ok {
			return opt, nil
		}
	}
	compKey := m.makeKey(companyID, nil, key)
	if opt, ok := m.options[compKey]; ok {
		return opt, nil
	}
	return nil, sql.ErrNoRows
}

func (m *memorySystemOptionRepo) ListOptionsByCompany(ctx context.Context, companyID string) ([]domain.SystemOption, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	res := make([]domain.SystemOption, 0)
	for _, opt := range m.options {
		if opt.CompanyProfileID == companyID {
			res = append(res, *opt)
		}
	}
	return res, nil
}

func (m *memorySystemOptionRepo) ListOptionsByCategory(ctx context.Context, companyID string, category domain.OptionCategory) ([]domain.SystemOption, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	res := make([]domain.SystemOption, 0)
	for _, opt := range m.options {
		if opt.CompanyProfileID == companyID && opt.Category == category {
			res = append(res, *opt)
		}
	}
	return res, nil
}

func (m *memorySystemOptionRepo) RecordHistory(ctx context.Context, history *domain.SystemConfigHistory) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.history = append(m.history, *history)
	return nil
}

func (m *memorySystemOptionRepo) ListHistory(ctx context.Context, companyID, key string, limit int32) ([]domain.SystemConfigHistory, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	res := make([]domain.SystemConfigHistory, 0)
	for i := len(m.history) - 1; i >= 0; i-- {
		h := m.history[i]
		if h.CompanyProfileID == companyID && h.OptionKey == key {
			res = append(res, h)
			if int32(len(res)) >= limit {
				break
			}
		}
	}
	return res, nil
}

type memoryVoucherRepo struct {
	mu      sync.Mutex
	configs map[string]*domain.VoucherNumberingConfig
}

func newMemoryVoucherRepo() *memoryVoucherRepo {
	return &memoryVoucherRepo{
		configs: make(map[string]*domain.VoucherNumberingConfig),
	}
}

func (m *memoryVoucherRepo) UpsertConfig(ctx context.Context, cfg *domain.VoucherNumberingConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.configs[cfg.VoucherType] = cfg
	return nil
}

func (m *memoryVoucherRepo) GetConfigByID(ctx context.Context, id string) (*domain.VoucherNumberingConfig, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, c := range m.configs {
		if c.ID == id {
			return c, nil
		}
	}
	return nil, sql.ErrNoRows
}

func (m *memoryVoucherRepo) GetEffectiveConfig(ctx context.Context, companyID string, branchID *string, voucherType string) (*domain.VoucherNumberingConfig, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if c, ok := m.configs[voucherType]; ok {
		return c, nil
	}
	return nil, sql.ErrNoRows
}

func (m *memoryVoucherRepo) NextSequence(ctx context.Context, companyID string, branchID *string, voucherType string, voucherDate time.Time) (string, *domain.VoucherNumberingConfig, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	cfg, ok := m.configs[voucherType]
	if !ok {
		return "", nil, fmt.Errorf("config for %s not found", voucherType)
	}
	num := cfg.NextNumber(voucherDate)
	return num, cfg, nil
}

func (m *memoryVoucherRepo) ListConfigs(ctx context.Context, companyID string) ([]domain.VoucherNumberingConfig, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	res := make([]domain.VoucherNumberingConfig, 0, len(m.configs))
	for _, c := range m.configs {
		res = append(res, *c)
	}
	return res, nil
}

func TestSystemOptionUseCase_GetEffectiveOption(t *testing.T) {
	t.Parallel()

	optRepo := newMemorySystemOptionRepo()
	voucherRepo := newMemoryVoucherRepo()
	uc := usecase.NewSystemOptionUseCase(optRepo, voucherRepo)
	ctx := context.Background()

	t.Run("Returns factory default when not found in DB", func(t *testing.T) {
		opt, err := uc.GetEffectiveOption(ctx, "comp-01", nil, "NON_CASH_PAYMENT_THRESHOLD")
		require.NoError(t, err)
		assert.Equal(t, "5000000", opt.OptionValue)
		assert.Equal(t, domain.DataTypeDecimal, opt.DataType)
	})

	t.Run("Returns persisted DB option and caches result", func(t *testing.T) {
		branchID := "branch-01"
		err := optRepo.UpsertOption(ctx, &domain.SystemOption{
			ID:               "opt-b-1",
			CompanyProfileID: "comp-01",
			BranchID:         &branchID,
			Category:         domain.OptionCategoryInventory,
			OptionKey:        "ALLOW_NEGATIVE_INVENTORY",
			OptionValue:      domain.PolicyWarn,
			DataType:         domain.DataTypeString,
			ScopeLevel:       domain.ScopeBranch,
		})
		require.NoError(t, err)

		opt, err := uc.GetEffectiveOption(ctx, "comp-01", &branchID, "ALLOW_NEGATIVE_INVENTORY")
		require.NoError(t, err)
		assert.Equal(t, domain.PolicyWarn, opt.OptionValue)

		// Second call hit cache
		optCached, err := uc.GetEffectiveOption(ctx, "comp-01", &branchID, "ALLOW_NEGATIVE_INVENTORY")
		require.NoError(t, err)
		assert.Same(t, opt, optCached)
	})
}

func TestSystemOptionUseCase_UpdateOption_Validations(t *testing.T) {
	t.Parallel()

	optRepo := newMemorySystemOptionRepo()
	voucherRepo := newMemoryVoucherRepo()
	uc := usecase.NewSystemOptionUseCase(optRepo, voucherRepo)
	ctx := context.Background()

	t.Run("Rejects LIFO inventory costing method under VAS 02 and Circular 99", func(t *testing.T) {
		err := uc.UpdateOption(ctx, usecase.UpdateOptionCommand{
			CompanyProfileID: "comp-01",
			Category:         domain.OptionCategoryInventory,
			OptionKey:        "COSTING_METHOD",
			NewValue:         "LIFO",
			DataType:         domain.DataTypeString,
			ScopeLevel:       domain.ScopeCompany,
			UpdatedBy:        "admin",
		})
		require.ErrorIs(t, err, domain.ErrLifoProhibited)
	})

	t.Run("Rejects invalid boolean format", func(t *testing.T) {
		err := uc.UpdateOption(ctx, usecase.UpdateOptionCommand{
			CompanyProfileID: "comp-01",
			Category:         domain.OptionCategoryVoucher,
			OptionKey:        "REQUIRE_MAKER_CHECKER",
			NewValue:         "YES",
			DataType:         domain.DataTypeBoolean,
			ScopeLevel:       domain.ScopeCompany,
			UpdatedBy:        "admin",
		})
		require.ErrorIs(t, err, domain.ErrInvalidOptionValue)
	})

	t.Run("Rejects invalid decimal number", func(t *testing.T) {
		err := uc.UpdateOption(ctx, usecase.UpdateOptionCommand{
			CompanyProfileID: "comp-01",
			Category:         domain.OptionCategoryCashBank,
			OptionKey:        "NON_CASH_PAYMENT_THRESHOLD",
			NewValue:         "abc_5000",
			DataType:         domain.DataTypeDecimal,
			ScopeLevel:       domain.ScopeCompany,
			UpdatedBy:        "admin",
		})
		require.ErrorIs(t, err, domain.ErrInvalidOptionValue)
	})

	t.Run("Rejects update when option is marked read-only", func(t *testing.T) {
		_ = optRepo.UpsertOption(ctx, &domain.SystemOption{
			ID:               "readonly-01",
			CompanyProfileID: "comp-01",
			OptionKey:        "PROTECTED_SYS_KEY",
			OptionValue:      "IMMUTABLE",
			DataType:         domain.DataTypeString,
			IsReadonly:       true,
		})

		err := uc.UpdateOption(ctx, usecase.UpdateOptionCommand{
			CompanyProfileID: "comp-01",
			Category:         domain.OptionCategoryGeneral,
			OptionKey:        "PROTECTED_SYS_KEY",
			NewValue:         "NEW_VAL",
			DataType:         domain.DataTypeString,
			ScopeLevel:       domain.ScopeCompany,
			UpdatedBy:        "admin",
		})
		require.ErrorIs(t, err, domain.ErrOptionReadonly)
	})

	t.Run("Successfully updates option, logs history, and invalidates cache", func(t *testing.T) {
		reason := "Thay đổi chính sách tồn kho"
		ip := "127.0.0.1"
		err := uc.UpdateOption(ctx, usecase.UpdateOptionCommand{
			CompanyProfileID: "comp-01",
			Category:         domain.OptionCategoryInventory,
			OptionKey:        "ALLOW_NEGATIVE_INVENTORY",
			NewValue:         domain.PolicyAllow,
			DataType:         domain.DataTypeString,
			ScopeLevel:       domain.ScopeCompany,
			UpdatedBy:        "chief_accountant",
			Reason:           &reason,
			ClientIP:         &ip,
		})
		require.NoError(t, err)

		opt, err := uc.GetEffectiveOption(ctx, "comp-01", nil, "ALLOW_NEGATIVE_INVENTORY")
		require.NoError(t, err)
		assert.Equal(t, domain.PolicyAllow, opt.OptionValue)

		hist, err := optRepo.ListHistory(ctx, "comp-01", "ALLOW_NEGATIVE_INVENTORY", 5)
		require.NoError(t, err)
		require.NotEmpty(t, hist)
		assert.Equal(t, domain.PolicyAllow, hist[0].NewValue)
	})
}

func TestSystemOptionUseCase_StatutoryValidations(t *testing.T) {
	t.Parallel()

	optRepo := newMemorySystemOptionRepo()
	voucherRepo := newMemoryVoucherRepo()
	uc := usecase.NewSystemOptionUseCase(optRepo, voucherRepo)
	ctx := context.Background()

	t.Run("ValidatePaymentCompliance: Cash payment at 5M threshold triggers statutory warning", func(t *testing.T) {
		warn, err := uc.ValidatePaymentCompliance(ctx, "comp-01", nil, true, decimal.NewFromInt(5_000_000))
		require.NoError(t, err)
		assert.True(t, warn)
	})

	t.Run("ValidatePaymentCompliance: Bank payment at 10M is compliant (no warning)", func(t *testing.T) {
		warn, err := uc.ValidatePaymentCompliance(ctx, "comp-01", nil, false, decimal.NewFromInt(10_000_000))
		require.NoError(t, err)
		assert.False(t, warn)
	})

	t.Run("ValidateStockOutward: Negative stock under DISALLOW triggers error", func(t *testing.T) {
		err := uc.ValidateStockOutward(ctx, "comp-01", nil, decimal.NewFromInt(10), decimal.NewFromInt(15))
		require.ErrorIs(t, err, domain.ErrNegativeStockBlocked)
	})

	t.Run("ValidateCashPayment: Negative cash vault under DISALLOW triggers error", func(t *testing.T) {
		err := uc.ValidateCashPayment(ctx, "comp-01", nil, decimal.NewFromInt(2_000_000), decimal.NewFromInt(3_000_000))
		require.ErrorIs(t, err, domain.ErrNegativeCashBlocked)
	})

	t.Run("ValidateVat8PercentRate: 8% VAT rate before 2027-01-01 is permitted", func(t *testing.T) {
		invoiceDate := time.Date(2026, 12, 15, 0, 0, 0, 0, time.UTC)
		err := uc.ValidateVat8PercentRate(ctx, "comp-01", nil, invoiceDate, decimal.NewFromFloat(0.08))
		require.NoError(t, err)
	})

	t.Run("ValidateVat8PercentRate: 8% VAT rate on/after 2027-01-01 MUST FAIL (ErrVatRateExpired)", func(t *testing.T) {
		invoiceDate := time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC)
		err := uc.ValidateVat8PercentRate(ctx, "comp-01", nil, invoiceDate, decimal.NewFromFloat(0.08))
		require.ErrorIs(t, err, domain.ErrVatRateExpired)
	})

	t.Run("ValidateVat8PercentRate: 10% VAT rate after 2026-12-31 is unaffected", func(t *testing.T) {
		invoiceDate := time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC)
		err := uc.ValidateVat8PercentRate(ctx, "comp-01", nil, invoiceDate, decimal.NewFromFloat(0.10))
		require.NoError(t, err)
	})
}

func TestSystemOptionUseCase_GenerateVoucherNumber(t *testing.T) {
	t.Parallel()

	optRepo := newMemorySystemOptionRepo()
	voucherRepo := newMemoryVoucherRepo()
	uc := usecase.NewSystemOptionUseCase(optRepo, voucherRepo)
	ctx := context.Background()

	d := time.Date(2026, 3, 20, 0, 0, 0, 0, time.UTC)
	_ = voucherRepo.UpsertConfig(ctx, &domain.VoucherNumberingConfig{
		ID:              "v-cfg-1",
		VoucherType:     "PAYMENT_VOUCHER",
		Prefix:          "PC",
		Pattern:         "{PREFIX}-{YYYY}{MM}-{SEQUENCE:05}",
		ResetFrequency:  domain.ResetFrequencyMonthly,
		CurrentSequence: 5,
		LastResetDate:   time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC),
	})
	num, err := uc.GenerateVoucherNumber(ctx, "comp-01", nil, "PAYMENT_VOUCHER", d)
	require.NoError(t, err)
	assert.Equal(t, "PC-202603-00006", num)

	// Error path: non-existent voucher type
	_, err = uc.GenerateVoucherNumber(ctx, "comp-01", nil, "NON_EXISTENT", d)
	require.Error(t, err)

	// WithLogger coverage
	ucWithLogger := uc.WithLogger(nil)
	assert.NotNil(t, ucWithLogger)
}

func TestSystemOptionUseCase_ValidateOptionValue_EdgeCases(t *testing.T) {
	t.Parallel()

	optRepo := newMemorySystemOptionRepo()
	voucherRepo := newMemoryVoucherRepo()
	uc := usecase.NewSystemOptionUseCase(optRepo, voucherRepo)
	ctx := context.Background()

	t.Run("Valid integer accepted", func(t *testing.T) {
		err := uc.UpdateOption(ctx, usecase.UpdateOptionCommand{
			CompanyProfileID: "comp-01",
			Category:         domain.OptionCategoryGeneral,
			OptionKey:        "CURRENCY_DECIMALS",
			NewValue:         "2",
			DataType:         domain.DataTypeInt,
			ScopeLevel:       domain.ScopeCompany,
			UpdatedBy:        "admin",
		})
		require.NoError(t, err)
	})

	t.Run("Invalid integer rejected", func(t *testing.T) {
		err := uc.UpdateOption(ctx, usecase.UpdateOptionCommand{
			CompanyProfileID: "comp-01",
			Category:         domain.OptionCategoryGeneral,
			OptionKey:        "CURRENCY_DECIMALS",
			NewValue:         "invalid_int",
			DataType:         domain.DataTypeInt,
			ScopeLevel:       domain.ScopeCompany,
			UpdatedBy:        "admin",
		})
		require.ErrorIs(t, err, domain.ErrInvalidOptionValue)
	})

	t.Run("Invalid costing method rejected", func(t *testing.T) {
		err := uc.UpdateOption(ctx, usecase.UpdateOptionCommand{
			CompanyProfileID: "comp-01",
			Category:         domain.OptionCategoryInventory,
			OptionKey:        "COSTING_METHOD",
			NewValue:         "INVALID_COSTING",
			DataType:         domain.DataTypeString,
			ScopeLevel:       domain.ScopeCompany,
			UpdatedBy:        "admin",
		})
		require.ErrorIs(t, err, domain.ErrInvalidOptionValue)
	})

	t.Run("Invalid negative inventory policy rejected", func(t *testing.T) {
		err := uc.UpdateOption(ctx, usecase.UpdateOptionCommand{
			CompanyProfileID: "comp-01",
			Category:         domain.OptionCategoryInventory,
			OptionKey:        "ALLOW_NEGATIVE_INVENTORY",
			NewValue:         "INVALID_POLICY",
			DataType:         domain.DataTypeString,
			ScopeLevel:       domain.ScopeCompany,
			UpdatedBy:        "admin",
		})
		require.ErrorIs(t, err, domain.ErrInvalidOptionValue)
	})

	t.Run("Invalid negative cash policy rejected (ALLOW is forbidden for cash)", func(t *testing.T) {
		err := uc.UpdateOption(ctx, usecase.UpdateOptionCommand{
			CompanyProfileID: "comp-01",
			Category:         domain.OptionCategoryCashBank,
			OptionKey:        "ALLOW_NEGATIVE_CASH",
			NewValue:         "ALLOW",
			DataType:         domain.DataTypeString,
			ScopeLevel:       domain.ScopeCompany,
			UpdatedBy:        "admin",
		})
		require.ErrorIs(t, err, domain.ErrInvalidOptionValue)
	})
}

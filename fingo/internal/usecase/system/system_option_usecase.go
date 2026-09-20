package system

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"sync"
	"time"

	domain "fingo/internal/domain/system"
	"fingo/pkg/logger"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// Default factory options catalog per SPEC-04 Section 4 Authoritative Key Registry
var factoryDefaults = map[string]*domain.SystemOption{
	"DECIMAL_SEPARATOR": {
		OptionKey:    "DECIMAL_SEPARATOR",
		OptionValue:  ",",
		DataType:     domain.DataTypeString,
		DefaultValue: ",",
		Category:     domain.OptionCategoryGeneral,
		ScopeLevel:   domain.ScopeGlobal,
	},
	"THOUSANDS_SEPARATOR": {
		OptionKey:    "THOUSANDS_SEPARATOR",
		OptionValue:  ".",
		DataType:     domain.DataTypeString,
		DefaultValue: ".",
		Category:     domain.OptionCategoryGeneral,
		ScopeLevel:   domain.ScopeGlobal,
	},
	"CURRENCY_DECIMALS": {
		OptionKey:    "CURRENCY_DECIMALS",
		OptionValue:  "0",
		DataType:     domain.DataTypeInt,
		DefaultValue: "0",
		Category:     domain.OptionCategoryGeneral,
		ScopeLevel:   domain.ScopeGlobal,
	},
	"FOREIGN_CURRENCY_DECIMALS": {
		OptionKey:    "FOREIGN_CURRENCY_DECIMALS",
		OptionValue:  "2",
		DataType:     domain.DataTypeInt,
		DefaultValue: "2",
		Category:     domain.OptionCategoryGeneral,
		ScopeLevel:   domain.ScopeGlobal,
	},
	"ALLOW_NEGATIVE_INVENTORY": {
		OptionKey:    "ALLOW_NEGATIVE_INVENTORY",
		OptionValue:  domain.PolicyDisallow,
		DataType:     domain.DataTypeString,
		DefaultValue: domain.PolicyDisallow,
		Category:     domain.OptionCategoryInventory,
		ScopeLevel:   domain.ScopeCompany,
	},
	"COSTING_METHOD": {
		OptionKey:    "COSTING_METHOD",
		OptionValue:  "MOVING_WEIGHTED_AVG",
		DataType:     domain.DataTypeString,
		DefaultValue: "MOVING_WEIGHTED_AVG",
		Category:     domain.OptionCategoryInventory,
		ScopeLevel:   domain.ScopeCompany,
	},
	"COSTING_SCOPE": {
		OptionKey:    "COSTING_SCOPE",
		OptionValue:  "WAREHOUSE",
		DataType:     domain.DataTypeString,
		DefaultValue: "WAREHOUSE",
		Category:     domain.OptionCategoryInventory,
		ScopeLevel:   domain.ScopeCompany,
	},
	"ALLOW_NEGATIVE_CASH": {
		OptionKey:    "ALLOW_NEGATIVE_CASH",
		OptionValue:  domain.PolicyDisallow,
		DataType:     domain.DataTypeString,
		DefaultValue: domain.PolicyDisallow,
		Category:     domain.OptionCategoryCashBank,
		ScopeLevel:   domain.ScopeCompany,
	},
	"NON_CASH_PAYMENT_THRESHOLD": {
		OptionKey:    "NON_CASH_PAYMENT_THRESHOLD",
		OptionValue:  "5000000",
		DataType:     domain.DataTypeDecimal,
		DefaultValue: "5000000",
		Category:     domain.OptionCategoryCashBank,
		ScopeLevel:   domain.ScopeCompany,
	},
	"AUTO_POST_ON_SAVE": {
		OptionKey:    "AUTO_POST_ON_SAVE",
		OptionValue:  "FALSE",
		DataType:     domain.DataTypeBoolean,
		DefaultValue: "FALSE",
		Category:     domain.OptionCategoryVoucher,
		ScopeLevel:   domain.ScopeCompany,
	},
	"REQUIRE_MAKER_CHECKER": {
		OptionKey:    "REQUIRE_MAKER_CHECKER",
		OptionValue:  "TRUE",
		DataType:     domain.DataTypeBoolean,
		DefaultValue: "TRUE",
		Category:     domain.OptionCategoryVoucher,
		ScopeLevel:   domain.ScopeCompany,
	},
	"VAT_8_PERCENT_ACTIVE": {
		OptionKey:    "VAT_8_PERCENT_ACTIVE",
		OptionValue:  "TRUE",
		DataType:     domain.DataTypeBoolean,
		DefaultValue: "TRUE",
		Category:     domain.OptionCategoryTax,
		ScopeLevel:   domain.ScopeGlobal,
	},
	"VAT_8_PERCENT_EXPIRY_DATE": {
		OptionKey:    "VAT_8_PERCENT_EXPIRY_DATE",
		OptionValue:  "2026-12-31",
		DataType:     domain.DataTypeString,
		DefaultValue: "2026-12-31",
		Category:     domain.OptionCategoryTax,
		ScopeLevel:   domain.ScopeGlobal,
	},
	"ALLOW_BACKDATED_VOUCHERS": {
		OptionKey:    "ALLOW_BACKDATED_VOUCHERS",
		OptionValue:  "FALSE",
		DataType:     domain.DataTypeBoolean,
		DefaultValue: "FALSE",
		Category:     domain.OptionCategoryClosing,
		ScopeLevel:   domain.ScopeCompany,
	},
	"SESSION_TIMEOUT_MINUTES": {
		OptionKey:    "SESSION_TIMEOUT_MINUTES",
		OptionValue:  "15",
		DataType:     domain.DataTypeInt,
		DefaultValue: "15",
		Category:     domain.OptionCategorySecurity,
		ScopeLevel:   domain.ScopeGlobal,
	},
	"MAX_FAILED_LOGIN_ATTEMPTS": {
		OptionKey:    "MAX_FAILED_LOGIN_ATTEMPTS",
		OptionValue:  "5",
		DataType:     domain.DataTypeInt,
		DefaultValue: "5",
		Category:     domain.OptionCategorySecurity,
		ScopeLevel:   domain.ScopeGlobal,
	},
}

type UpdateOptionCommand struct {
	CompanyProfileID string                `json:"company_profile_id"`
	BranchID         *string               `json:"branch_id,omitempty"`
	Category         domain.OptionCategory `json:"category"`
	OptionKey        string                `json:"option_key"`
	NewValue         string                `json:"new_value"`
	DataType         domain.OptionDataType `json:"data_type"`
	ScopeLevel       domain.ScopeLevel     `json:"scope_level"`
	Description      string                `json:"description"`
	UpdatedBy        string                `json:"updated_by"`
	Reason           *string               `json:"reason,omitempty"`
	ClientIP         *string               `json:"client_ip,omitempty"`
}

type SystemOptionUseCase struct {
	optRepo     domain.SystemOptionRepository
	voucherRepo domain.VoucherNumberingRepository
	logger      *logger.Logger
	cacheMu     sync.RWMutex
	cache       map[string]*domain.SystemOption
}

func NewSystemOptionUseCase(
	optRepo domain.SystemOptionRepository,
	voucherRepo domain.VoucherNumberingRepository,
) *SystemOptionUseCase {
	return &SystemOptionUseCase{
		optRepo:     optRepo,
		voucherRepo: voucherRepo,
		logger:      logger.New(logger.Config{Level: slog.LevelInfo, Format: logger.FormatJSON}),
		cache:       make(map[string]*domain.SystemOption),
	}
}

func (u *SystemOptionUseCase) WithLogger(l *logger.Logger) *SystemOptionUseCase {
	if l != nil {
		u.logger = l
	}
	return u
}

func (u *SystemOptionUseCase) cacheKey(companyID string, branchID *string, key string) string {
	b := "GLOBAL"
	if branchID != nil && *branchID != "" {
		b = *branchID
	}
	return fmt.Sprintf("%s:%s:%s", companyID, b, key)
}

func (u *SystemOptionUseCase) GetEffectiveOption(ctx context.Context, companyID string, branchID *string, key string) (*domain.SystemOption, error) {
	cKey := u.cacheKey(companyID, branchID, key)

	u.cacheMu.RLock()
	cached, found := u.cache[cKey]
	u.cacheMu.RUnlock()
	if found {
		return cached, nil
	}

	opt, err := u.optRepo.GetEffectiveOption(ctx, companyID, branchID, key)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			if def, ok := factoryDefaults[key]; ok {
				return def, nil
			}
		}
		return nil, fmt.Errorf("failed to get effective option (%s): %w", key, err)
	}

	u.cacheMu.Lock()
	u.cache[cKey] = opt
	u.cacheMu.Unlock()

	return opt, nil
}

func (u *SystemOptionUseCase) UpdateOption(ctx context.Context, cmd UpdateOptionCommand) error {
	if err := validateOptionValue(cmd.DataType, cmd.OptionKey, cmd.NewValue); err != nil {
		return err
	}

	// Check if existing option is read-only
	existing, err := u.optRepo.GetOption(ctx, cmd.CompanyProfileID, cmd.BranchID, cmd.OptionKey)
	var oldValue *string
	defVal := cmd.NewValue

	if err == nil && existing != nil {
		if existing.IsReadonly {
			return domain.ErrOptionReadonly
		}
		val := existing.OptionValue
		oldValue = &val
		defVal = existing.DefaultValue
	} else if def, ok := factoryDefaults[cmd.OptionKey]; ok {
		defVal = def.DefaultValue
	}

	optID := uuid.New().String()
	if existing != nil {
		optID = existing.ID
	}

	opt := &domain.SystemOption{
		ID:               optID,
		CompanyProfileID: cmd.CompanyProfileID,
		BranchID:         cmd.BranchID,
		Category:         cmd.Category,
		OptionKey:        cmd.OptionKey,
		OptionValue:      cmd.NewValue,
		DataType:         cmd.DataType,
		DefaultValue:     defVal,
		ScopeLevel:       cmd.ScopeLevel,
		Description:      cmd.Description,
		IsReadonly:       false,
		IsEncrypted:      false,
		UpdatedAt:        time.Now(),
		UpdatedBy:        cmd.UpdatedBy,
	}

	if err := u.optRepo.UpsertOption(ctx, opt); err != nil {
		u.logger.Error(ctx, "failed to upsert system option",
			slog.String("key", cmd.OptionKey),
			slog.String("error", err.Error()),
		)
		return fmt.Errorf("failed to upsert option (%s): %w", cmd.OptionKey, err)
	}

	// Record audit history
	hist := &domain.SystemConfigHistory{
		ID:               uuid.New().String(),
		CompanyProfileID: cmd.CompanyProfileID,
		OptionKey:        cmd.OptionKey,
		OldValue:         oldValue,
		NewValue:         cmd.NewValue,
		ChangedBy:        cmd.UpdatedBy,
		ChangedAt:        time.Now(),
		Reason:           cmd.Reason,
		ClientIP:         cmd.ClientIP,
	}
	if err := u.optRepo.RecordHistory(ctx, hist); err != nil {
		u.logger.Warn(ctx, "failed to record option audit history",
			slog.String("key", cmd.OptionKey),
			slog.String("error", err.Error()),
		)
	}

	// Invalidate cache
	u.invalidateCache(cmd.CompanyProfileID, cmd.OptionKey)

	u.logger.Info(ctx, "system option successfully updated",
		slog.String("company_id", cmd.CompanyProfileID),
		slog.String("key", cmd.OptionKey),
		slog.String("new_value", cmd.NewValue),
		slog.String("updated_by", cmd.UpdatedBy),
	)

	return nil
}

func (u *SystemOptionUseCase) invalidateCache(companyID, key string) {
	u.cacheMu.Lock()
	defer u.cacheMu.Unlock()

	prefix := fmt.Sprintf("%s:", companyID)
	suffix := fmt.Sprintf(":%s", key)
	for k := range u.cache {
		if strings.HasPrefix(k, prefix) && strings.HasSuffix(k, suffix) {
			delete(u.cache, k)
		}
	}
}

// GenerateVoucherNumber atomic continuous sequence allocation
func (u *SystemOptionUseCase) GenerateVoucherNumber(
	ctx context.Context,
	companyID string,
	branchID *string,
	voucherType string,
	voucherDate time.Time,
) (string, error) {
	num, _, err := u.voucherRepo.NextSequence(ctx, companyID, branchID, voucherType, voucherDate)
	if err != nil {
		u.logger.Error(ctx, "failed to generate voucher sequence",
			slog.String("voucher_type", voucherType),
			slog.String("error", err.Error()),
		)
		return "", fmt.Errorf("failed to generate voucher number for %s: %w", voucherType, err)
	}
	return num, nil
}

// ValidatePaymentCompliance enforces Luật Thuế GTGT 2024 (5M VND rule)
func (u *SystemOptionUseCase) ValidatePaymentCompliance(
	ctx context.Context,
	companyID string,
	branchID *string,
	isCash bool,
	amount decimal.Decimal,
) (bool, error) {
	thresholdOpt, err := u.GetEffectiveOption(ctx, companyID, branchID, "NON_CASH_PAYMENT_THRESHOLD")
	threshold := decimal.NewFromInt(5_000_000)
	if err == nil && thresholdOpt != nil {
		parsed, parseErr := decimal.NewFromString(thresholdOpt.OptionValue)
		if parseErr == nil {
			threshold = parsed
		}
	}

	return domain.ValidateNonCashPaymentThreshold(amount, isCash, threshold)
}

// ValidateStockOutward enforces VAS 02 and Circular 99 negative stock invariant
func (u *SystemOptionUseCase) ValidateStockOutward(
	ctx context.Context,
	companyID string,
	branchID *string,
	currentStock, dispatchQty decimal.Decimal,
) error {
	policyOpt, err := u.GetEffectiveOption(ctx, companyID, branchID, "ALLOW_NEGATIVE_INVENTORY")
	policy := domain.PolicyDisallow
	if err == nil && policyOpt != nil {
		policy = strings.ToUpper(policyOpt.OptionValue)
	}

	return domain.CheckNegativeInventory(currentStock, dispatchQty, policy)
}

// ValidateCashPayment enforces Law on Accounting cash balance protection
func (u *SystemOptionUseCase) ValidateCashPayment(
	ctx context.Context,
	companyID string,
	branchID *string,
	currentBalance, paymentAmount decimal.Decimal,
) error {
	policyOpt, err := u.GetEffectiveOption(ctx, companyID, branchID, "ALLOW_NEGATIVE_CASH")
	policy := domain.PolicyDisallow
	if err == nil && policyOpt != nil {
		policy = strings.ToUpper(policyOpt.OptionValue)
	}

	return domain.CheckNegativeCash(currentBalance, paymentAmount, policy)
}

// ValidateVat8PercentRate enforces BR-OPT-07: Resolution 204/2025/QH15 & Decree 174/2025/ND-CP
// Rejects 8% VAT rate on and after expiry date (e.g. 2027-01-01)
func (u *SystemOptionUseCase) ValidateVat8PercentRate(
	ctx context.Context,
	companyID string,
	branchID *string,
	invoiceDate time.Time,
	vatRate decimal.Decimal,
) error {
	eightPercent := decimal.NewFromFloat(0.08)
	if !vatRate.Equal(eightPercent) {
		return nil
	}

	opt, err := u.GetEffectiveOption(ctx, companyID, branchID, "VAT_8_PERCENT_EXPIRY_DATE")
	expiryDateStr := "2026-12-31"
	if err == nil && opt != nil && opt.OptionValue != "" {
		expiryDateStr = opt.OptionValue
	}

	expiry, parseErr := time.Parse("2006-01-02", expiryDateStr)
	if parseErr != nil {
		expiry = time.Date(2026, 12, 31, 23, 59, 59, 0, time.UTC)
	} else {
		expiry = time.Date(expiry.Year(), expiry.Month(), expiry.Day(), 23, 59, 59, 0, time.UTC)
	}

	if invoiceDate.After(expiry) {
		return domain.ErrVatRateExpired
	}
	return nil
}

func validateOptionValue(dataType domain.OptionDataType, key, val string) error {
	switch dataType {
	case domain.DataTypeBoolean:
		u := strings.ToUpper(strings.TrimSpace(val))
		if u != "TRUE" && u != "FALSE" {
			return fmt.Errorf("%w: boolean must be TRUE or FALSE", domain.ErrInvalidOptionValue)
		}
	case domain.DataTypeInt:
		if _, err := strconv.ParseInt(strings.TrimSpace(val), 10, 64); err != nil {
			return fmt.Errorf("%w: int parse failure: %v", domain.ErrInvalidOptionValue, err)
		}
	case domain.DataTypeDecimal:
		if _, err := decimal.NewFromString(strings.TrimSpace(val)); err != nil {
			return fmt.Errorf("%w: decimal parse failure: %v", domain.ErrInvalidOptionValue, err)
		}
	}

	switch key {
	case "COSTING_METHOD":
		u := strings.ToUpper(strings.TrimSpace(val))
		if u == "LIFO" {
			return domain.ErrLifoProhibited
		}
		if u != "FIFO" && u != "MOVING_WEIGHTED_AVG" && u != "PERIODIC_AVG" {
			return fmt.Errorf("%w: invalid costing method %s", domain.ErrInvalidOptionValue, val)
		}
	case "ALLOW_NEGATIVE_INVENTORY":
		u := strings.ToUpper(strings.TrimSpace(val))
		if u != domain.PolicyDisallow && u != domain.PolicyWarn && u != domain.PolicyAllow {
			return fmt.Errorf("%w: invalid negative inventory policy %s", domain.ErrInvalidOptionValue, val)
		}
	case "ALLOW_NEGATIVE_CASH":
		u := strings.ToUpper(strings.TrimSpace(val))
		if u != domain.PolicyDisallow && u != domain.PolicyWarn {
			return fmt.Errorf("%w: invalid negative cash policy %s", domain.ErrInvalidOptionValue, val)
		}
	}

	return nil
}

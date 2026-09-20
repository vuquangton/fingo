package spine

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	domain "fingo/internal/domain/spine"
	"fingo/pkg/logger"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

var (
	ErrParentAccountNotFound = errors.New("parent account not found")
	ErrBaseCurrencyFixedRate = errors.New("base currency rate is always fixed to 1.0")
	ErrAccountNotFound       = errors.New("account not found")
	ErrPeriodNotFound        = errors.New("accounting period not found")
)

type CreateSubAccountCommand struct {
	IdempotencyKey      string                 `json:"idempotency_key,omitempty"`
	CompanyProfileID    string                 `json:"company_profile_id"`
	ParentAccountCode   string                 `json:"parent_account_code"`
	SubAccountCode      string                 `json:"sub_account_code"`
	Name                string                 `json:"name"`
	EnglishName         string                 `json:"english_name,omitempty"`
	Nature              domain.AccountNature   `json:"nature"`
	Category            domain.AccountCategory `json:"category"`
	RequiresPartner     bool                   `json:"requires_partner"`
	RequiresBankAccount bool                   `json:"requires_bank_account"`
	RequiresCostCenter  bool                   `json:"requires_cost_center"`
	RequiresExpenseItem bool                   `json:"requires_expense_item"`
}

type RevalueForeignCurrencyCommand struct {
	CompanyProfileID  string          `json:"company_profile_id"`
	AccountCode       string          `json:"account_code"`
	CurrencyCode      string          `json:"currency_code"`
	ForeignAmount     decimal.Decimal `json:"foreign_amount"`
	BookValueVND      decimal.Decimal `json:"book_value_vnd"`
	RevaluationDate   time.Time       `json:"revaluation_date"`
	RateType          domain.RateType `json:"rate_type"`
}

type RevaluationResult struct {
	CurrencyCode   string          `json:"currency_code"`
	ForeignAmount  decimal.Decimal `json:"foreign_amount"`
	BookValueVND   decimal.Decimal `json:"book_value_vnd"`
	ExchangeRate   decimal.Decimal `json:"exchange_rate"`
	RevaluedVND    decimal.Decimal `json:"revalued_vnd"`
	DiffVND        decimal.Decimal `json:"diff_vnd"`
	IsGain         bool            `json:"is_gain"`
	DebitAccount   string          `json:"debit_account"`
	CreditAccount  string          `json:"credit_account"`
}

type VoucherLineAllocation struct {
	AccountCode   string
	CostCenterID  *string
	ExpenseItemID *string
}

type SpineUseCase struct {
	currRepo   domain.CurrencyRepository
	fxRepo     domain.ExchangeRateRepository
	accRepo    domain.AccountRepository
	periodRepo domain.PeriodRepository
	costRepo   domain.CostDimensionRepository
	logger     *logger.Logger
}

func NewSpineUseCase(
	currRepo domain.CurrencyRepository,
	fxRepo domain.ExchangeRateRepository,
	accRepo domain.AccountRepository,
	periodRepo domain.PeriodRepository,
	costRepo domain.CostDimensionRepository,
) *SpineUseCase {
	return &SpineUseCase{
		currRepo:   currRepo,
		fxRepo:     fxRepo,
		accRepo:    accRepo,
		periodRepo: periodRepo,
		costRepo:   costRepo,
		logger:     logger.New(logger.Config{Level: slog.LevelInfo, Format: logger.FormatJSON}),
	}
}

func (u *SpineUseCase) WithLogger(l *logger.Logger) *SpineUseCase {
	if l != nil {
		u.logger = l
	}
	return u
}

// CreateSubAccount creates a child account under an existing parent, marking the parent is_leaf = false
func (u *SpineUseCase) CreateSubAccount(ctx context.Context, cmd CreateSubAccountCommand) (*domain.Account, error) {
	// Idempotency: If account already exists under same code, return it directly
	if existing, err := u.accRepo.GetAccountByCode(ctx, cmd.CompanyProfileID, cmd.SubAccountCode); err == nil && existing != nil {
		u.logger.Info(ctx, "sub-account already exists (idempotent submission)",
			slog.String("company_id", cmd.CompanyProfileID),
			slog.String("code", cmd.SubAccountCode),
		)
		return existing, nil
	}

	parent, err := u.accRepo.GetAccountByCode(ctx, cmd.CompanyProfileID, cmd.ParentAccountCode)
	if err != nil {
		return nil, fmt.Errorf("%w: %s", ErrParentAccountNotFound, cmd.ParentAccountCode)
	}

	subID := uuid.New().String()
	subAcc, err := domain.NewAccount(
		subID, cmd.CompanyProfileID, cmd.SubAccountCode, cmd.Name,
		&parent.ID, parent.AccountLevel+1, cmd.Nature, cmd.Category, true,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create sub account domain entity: %w", err)
	}

	subAcc.EnglishName = cmd.EnglishName
	subAcc.RequiresPartner = cmd.RequiresPartner
	subAcc.RequiresBankAccount = cmd.RequiresBankAccount
	subAcc.RequiresCostCenter = cmd.RequiresCostCenter
	subAcc.RequiresExpenseItem = cmd.RequiresExpenseItem

	if err := u.accRepo.SaveAccount(ctx, subAcc); err != nil {
		return nil, fmt.Errorf("failed to save sub-account: %w", err)
	}

	// If parent was leaf, update parent to non-leaf
	if parent.IsLeaf {
		if err := u.accRepo.UpdateLeafStatus(ctx, parent.ID, false); err != nil {
			u.logger.Warn(ctx, "failed to update parent leaf status",
				slog.String("parent_id", parent.ID),
				slog.String("error", err.Error()),
			)
		}
	}

	u.logger.Info(ctx, "sub-account successfully created",
		slog.String("company_id", cmd.CompanyProfileID),
		slog.String("code", cmd.SubAccountCode),
		slog.String("parent_code", cmd.ParentAccountCode),
	)

	return subAcc, nil
}

// GetEffectiveExchangeRate resolves exchange rate with base currency awareness
func (u *SpineUseCase) GetEffectiveExchangeRate(
	ctx context.Context,
	companyID string,
	currencyCode string,
	rateDate time.Time,
	rateType domain.RateType,
) (decimal.Decimal, error) {
	curr, err := u.currRepo.GetCurrencyByCode(ctx, companyID, currencyCode)
	if err != nil {
		return decimal.Zero, fmt.Errorf("currency not found (%s): %w", currencyCode, err)
	}
	if curr.IsBase {
		return decimal.NewFromInt(1), nil
	}

	rateObj, err := u.fxRepo.GetEffectiveRate(ctx, companyID, currencyCode, rateDate, rateType)
	if err != nil {
		return decimal.Zero, fmt.Errorf("effective exchange rate not found for %s on %s: %w", currencyCode, rateDate.Format("2006-01-02"), err)
	}

	return rateObj.Rate, nil
}

// SetPeriodLockDate updates the transaction barrier date for an accounting period
func (u *SpineUseCase) SetPeriodLockDate(
	ctx context.Context,
	companyID string,
	periodID string,
	lockDate time.Time,
) error {
	if err := u.periodRepo.UpdatePeriodLock(ctx, periodID, lockDate, domain.PeriodStatusHardLocked); err != nil {
		return fmt.Errorf("failed to update period lock: %w", err)
	}

	u.logger.Info(ctx, "accounting period lock date updated",
		slog.String("company_id", companyID),
		slog.String("period_id", periodID),
		slog.String("lock_date", lockDate.Format("2006-01-02")),
		slog.String("status", string(domain.PeriodStatusHardLocked)),
	)

	return nil
}

// RevalueForeignCurrency calculates VAS 10 unrealized foreign exchange difference
func (u *SpineUseCase) RevalueForeignCurrency(
	ctx context.Context,
	cmd RevalueForeignCurrencyCommand,
) (*RevaluationResult, error) {
	rate, err := u.GetEffectiveExchangeRate(ctx, cmd.CompanyProfileID, cmd.CurrencyCode, cmd.RevaluationDate, cmd.RateType)
	if err != nil {
		return nil, fmt.Errorf("cannot revalue foreign currency: %w", err)
	}

	// Determine account category from account repo or code prefix
	category := domain.CategoryAsset
	if acc, err := u.accRepo.GetAccountByCode(ctx, cmd.CompanyProfileID, cmd.AccountCode); err == nil && acc != nil {
		category = acc.Category
	} else if strings.HasPrefix(cmd.AccountCode, "3") {
		category = domain.CategoryLiability
	}

	// Compute book rate from book value and foreign amount: bookRate = bookVal / amountFC
	bookRate := decimal.Zero
	if !cmd.ForeignAmount.IsZero() {
		bookRate = cmd.BookValueVND.Div(cmd.ForeignAmount)
	}

	domainRes := domain.ComputeFXRevaluation(
		cmd.AccountCode, category, cmd.CurrencyCode,
		cmd.ForeignAmount, bookRate, rate, 0,
	)

	result := &RevaluationResult{
		CurrencyCode:  cmd.CurrencyCode,
		ForeignAmount: cmd.ForeignAmount,
		BookValueVND:  cmd.BookValueVND,
		ExchangeRate:  rate,
		RevaluedVND:   domainRes.RevaluedValue,
		DiffVND:       domainRes.Diff.Abs(),
		IsGain:        domainRes.IsGain,
		DebitAccount:  domainRes.DebitAccount,
		CreditAccount: domainRes.CreditAccount,
	}

	u.logger.Info(ctx, "foreign currency revaluation computed",
		slog.String("currency", cmd.CurrencyCode),
		slog.String("rate", rate.String()),
		slog.String("diff", domainRes.Diff.String()),
		slog.Bool("is_gain", result.IsGain),
	)

	return result, nil
}

// ValidateVoucherDimensions validates INV-SPINE-01, INV-SPINE-04, and INV-SPINE-07
func (u *SpineUseCase) ValidateVoucherDimensions(
	ctx context.Context,
	companyID string,
	voucherDate time.Time,
	lines []VoucherLineAllocation,
) error {
	// 1. Period Lock Date Check
	period, err := u.periodRepo.GetPeriodByDate(ctx, companyID, voucherDate)
	if err == nil && period != nil {
		if lockErr := domain.CheckPeriodLock(voucherDate, period.LockDate); lockErr != nil {
			return lockErr
		}
	}

	// 2. Validate line dimensions
	for _, l := range lines {
		acc, accErr := u.accRepo.GetAccountByCode(ctx, companyID, l.AccountCode)
		if accErr != nil {
			return fmt.Errorf("%w: %s", ErrAccountNotFound, l.AccountCode)
		}

		// Leaf check
		if leafErr := domain.ValidatePostingAccount(acc); leafErr != nil {
			return leafErr
		}

		// Allocation check
		if allocErr := domain.ValidateLineAllocation(acc, l.CostCenterID, l.ExpenseItemID); allocErr != nil {
			return allocErr
		}
	}

	return nil
}

package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	sqlc "fingo/internal/adapter/mariadb/sqlc"
	"fingo/internal/domain/spine"

	"github.com/shopspring/decimal"
)

// SpineRepo implements the Accounting Spine repositories
type SpineRepo struct {
	db      *sql.DB
	queries *sqlc.Queries
}

func NewSpineRepo(db *sql.DB) *SpineRepo {
	return &SpineRepo{
		db:      db,
		queries: sqlc.New(db),
	}
}

// ======================== CURRENCY ========================

func (r *SpineRepo) SaveCurrency(ctx context.Context, c *spine.Currency) error {
	arg := sqlc.UpsertCurrencyParams{
		Code:             c.Code,
		CompanyProfileID: c.CompanyProfileID,
		Name:             c.Name,
		Symbol:           c.Symbol,
		DecimalPlaces:    c.DecimalPlaces,
		IsBase:           c.IsBase,
		IsActive:         c.IsActive,
	}
	if err := r.queries.UpsertCurrency(ctx, arg); err != nil {
		return fmt.Errorf("failed to upsert currency (%s): %w", c.Code, err)
	}
	return nil
}

func (r *SpineRepo) GetCurrencyByCode(ctx context.Context, companyID, code string) (*spine.Currency, error) {
	row, err := r.queries.GetCurrencyByCode(ctx, sqlc.GetCurrencyByCodeParams{
		CompanyProfileID: companyID,
		Code:             code,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get currency (%s): %w", code, err)
	}
	return &spine.Currency{
		Code:             row.Code,
		CompanyProfileID: row.CompanyProfileID,
		Name:             row.Name,
		Symbol:           row.Symbol,
		DecimalPlaces:    row.DecimalPlaces,
		IsBase:           row.IsBase,
		IsActive:         row.IsActive,
		CreatedAt:        row.CreatedAt,
		UpdatedAt:        row.UpdatedAt,
	}, nil
}

func (r *SpineRepo) GetBaseCurrency(ctx context.Context, companyID string) (*spine.Currency, error) {
	row, err := r.queries.GetBaseCurrency(ctx, companyID)
	if err != nil {
		return nil, fmt.Errorf("failed to get base currency: %w", err)
	}
	return &spine.Currency{
		Code:             row.Code,
		CompanyProfileID: row.CompanyProfileID,
		Name:             row.Name,
		Symbol:           row.Symbol,
		DecimalPlaces:    row.DecimalPlaces,
		IsBase:           row.IsBase,
		IsActive:         row.IsActive,
		CreatedAt:        row.CreatedAt,
		UpdatedAt:        row.UpdatedAt,
	}, nil
}

func (r *SpineRepo) ListCurrencies(ctx context.Context, companyID string) ([]spine.Currency, error) {
	rows, err := r.queries.ListCurrenciesByCompany(ctx, companyID)
	if err != nil {
		return nil, fmt.Errorf("failed to list currencies: %w", err)
	}
	list := make([]spine.Currency, 0, len(rows))
	for _, row := range rows {
		list = append(list, spine.Currency{
			Code:             row.Code,
			CompanyProfileID: row.CompanyProfileID,
			Name:             row.Name,
			Symbol:           row.Symbol,
			DecimalPlaces:    row.DecimalPlaces,
			IsBase:           row.IsBase,
			IsActive:         row.IsActive,
			CreatedAt:        row.CreatedAt,
			UpdatedAt:        row.UpdatedAt,
		})
	}
	return list, nil
}

// ======================== EXCHANGE RATE ========================

func (r *SpineRepo) SaveExchangeRate(ctx context.Context, rate *spine.ExchangeRate) error {
	arg := sqlc.UpsertExchangeRateParams{
		ID:               rate.ID,
		CompanyProfileID: rate.CompanyProfileID,
		CurrencyCode:     rate.CurrencyCode,
		RateDate:         rate.RateDate,
		RateType:         sqlc.ExchangeRatesRateType(rate.RateType),
		Rate:             rate.Rate.StringFixed(6),
		SourceBank:       rate.SourceBank,
		CreatedBy:        rate.CreatedBy,
	}
	if err := r.queries.UpsertExchangeRate(ctx, arg); err != nil {
		return fmt.Errorf("failed to upsert exchange rate (%s): %w", rate.CurrencyCode, err)
	}
	return nil
}

func (r *SpineRepo) GetEffectiveRate(ctx context.Context, companyID, currCode string, rateDate time.Time, rType spine.RateType) (*spine.ExchangeRate, error) {
	row, err := r.queries.GetEffectiveExchangeRate(ctx, sqlc.GetEffectiveExchangeRateParams{
		CompanyProfileID: companyID,
		CurrencyCode:     currCode,
		RateType:         sqlc.ExchangeRatesRateType(rType),
		RateDate:         rateDate,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get effective rate (%s on %s): %w", currCode, rateDate.Format("2006-01-02"), err)
	}

	rateDec, err := decimal.NewFromString(row.Rate)
	if err != nil {
		return nil, fmt.Errorf("corrupt decimal rate '%s' in db for currency %s: %w", row.Rate, currCode, err)
	}
	return &spine.ExchangeRate{
		ID:               row.ID,
		CompanyProfileID: row.CompanyProfileID,
		CurrencyCode:     row.CurrencyCode,
		RateDate:         row.RateDate,
		RateType:         spine.RateType(row.RateType),
		Rate:             rateDec,
		SourceBank:       row.SourceBank,
		CreatedAt:        row.CreatedAt,
		CreatedBy:        row.CreatedBy,
	}, nil
}

func (r *SpineRepo) ListRatesByDate(ctx context.Context, companyID string, rateDate time.Time) ([]spine.ExchangeRate, error) {
	rows, err := r.queries.ListExchangeRatesByDate(ctx, sqlc.ListExchangeRatesByDateParams{
		CompanyProfileID: companyID,
		RateDate:         rateDate,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list exchange rates for date %s: %w", rateDate.Format("2006-01-02"), err)
	}
	list := make([]spine.ExchangeRate, 0, len(rows))
	for _, row := range rows {
		rateDec, err := decimal.NewFromString(row.Rate)
		if err != nil {
			return nil, fmt.Errorf("corrupt decimal rate '%s' in db for currency %s: %w", row.Rate, row.CurrencyCode, err)
		}
		list = append(list, spine.ExchangeRate{
			ID:               row.ID,
			CompanyProfileID: row.CompanyProfileID,
			CurrencyCode:     row.CurrencyCode,
			RateDate:         row.RateDate,
			RateType:         spine.RateType(row.RateType),
			Rate:             rateDec,
			SourceBank:       row.SourceBank,
			CreatedAt:        row.CreatedAt,
			CreatedBy:        row.CreatedBy,
		})
	}
	return list, nil
}

// ======================== CHART OF ACCOUNTS ========================

func mapAccountRowToDomain(row sqlc.Account) spine.Account {
	return spine.Account{
		ID:                  row.ID,
		CompanyProfileID:    row.CompanyProfileID,
		Code:                row.Code,
		Name:                row.Name,
		EnglishName:         fromNullString(row.EnglishName),
		ParentID:            nullStringToPtr(row.ParentID),
		AccountLevel:        int(row.AccountLevel),
		Nature:              spine.AccountNature(row.Nature),
		Category:            spine.AccountCategory(row.Category),
		IsLeaf:              row.IsLeaf,
		IsForeignCurrency:   row.IsForeignCurrency,
		RequiresPartner:     row.RequiresPartner,
		RequiresBankAccount: row.RequiresBankAccount,
		RequiresCostCenter:  row.RequiresCostCenter,
		RequiresExpenseItem: row.RequiresExpenseItem,
		IsActive:            row.IsActive,
		CreatedAt:           row.CreatedAt,
		UpdatedAt:           row.UpdatedAt,
	}
}

func (r *SpineRepo) SaveAccount(ctx context.Context, acc *spine.Account) error {
	arg := sqlc.UpsertAccountParams{
		ID:                  acc.ID,
		CompanyProfileID:    acc.CompanyProfileID,
		Code:                acc.Code,
		Name:                acc.Name,
		EnglishName:         toNullString(acc.EnglishName),
		ParentID:            ptrToNullString(acc.ParentID),
		AccountLevel:        int32(acc.AccountLevel),
		Nature:              sqlc.AccountsNature(acc.Nature),
		Category:            sqlc.AccountsCategory(acc.Category),
		IsLeaf:              acc.IsLeaf,
		IsForeignCurrency:   acc.IsForeignCurrency,
		RequiresPartner:     acc.RequiresPartner,
		RequiresBankAccount: acc.RequiresBankAccount,
		RequiresCostCenter:  acc.RequiresCostCenter,
		RequiresExpenseItem: acc.RequiresExpenseItem,
		IsActive:            acc.IsActive,
	}
	if err := r.queries.UpsertAccount(ctx, arg); err != nil {
		return fmt.Errorf("failed to upsert account (%s): %w", acc.Code, err)
	}
	return nil
}

func (r *SpineRepo) GetAccountByID(ctx context.Context, id string) (*spine.Account, error) {
	row, err := r.queries.GetAccountByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get account by id (%s): %w", id, err)
	}
	acc := mapAccountRowToDomain(row)
	return &acc, nil
}

func (r *SpineRepo) GetAccountByCode(ctx context.Context, companyID, code string) (*spine.Account, error) {
	row, err := r.queries.GetAccountByCode(ctx, sqlc.GetAccountByCodeParams{
		CompanyProfileID: companyID,
		Code:             code,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get account by code (%s): %w", code, err)
	}
	acc := mapAccountRowToDomain(row)
	return &acc, nil
}

func (r *SpineRepo) ListAccounts(ctx context.Context, companyID string) ([]spine.Account, error) {
	rows, err := r.queries.ListAccountsByCompany(ctx, companyID)
	if err != nil {
		return nil, fmt.Errorf("failed to list accounts: %w", err)
	}
	list := make([]spine.Account, 0, len(rows))
	for _, row := range rows {
		list = append(list, mapAccountRowToDomain(row))
	}
	return list, nil
}

func (r *SpineRepo) ListChildAccounts(ctx context.Context, parentID string) ([]spine.Account, error) {
	rows, err := r.queries.ListChildAccounts(ctx, ptrToNullString(&parentID))
	if err != nil {
		return nil, fmt.Errorf("failed to list child accounts for parent %s: %w", parentID, err)
	}
	list := make([]spine.Account, 0, len(rows))
	for _, row := range rows {
		list = append(list, mapAccountRowToDomain(row))
	}
	return list, nil
}

func (r *SpineRepo) UpdateLeafStatus(ctx context.Context, id string, isLeaf bool) error {
	if err := r.queries.UpdateAccountLeafStatus(ctx, sqlc.UpdateAccountLeafStatusParams{
		IsLeaf: isLeaf,
		ID:     id,
	}); err != nil {
		return fmt.Errorf("failed to update account leaf status (%s): %w", id, err)
	}
	return nil
}

// ======================== FISCAL YEAR & PERIOD ========================

func (r *SpineRepo) SaveFiscalYear(ctx context.Context, fy *spine.FiscalYear) error {
	arg := sqlc.UpsertFiscalYearParams{
		ID:               fy.ID,
		CompanyProfileID: fy.CompanyProfileID,
		Year:             int32(fy.Year),
		StartDate:        fy.StartDate,
		EndDate:          fy.EndDate,
		Status:           sqlc.FiscalYearsStatus(fy.Status),
	}
	if err := r.queries.UpsertFiscalYear(ctx, arg); err != nil {
		return fmt.Errorf("failed to upsert fiscal year (%d): %w", fy.Year, err)
	}
	return nil
}

func (r *SpineRepo) GetFiscalYearByYear(ctx context.Context, companyID string, year int) (*spine.FiscalYear, error) {
	row, err := r.queries.GetFiscalYearByYear(ctx, sqlc.GetFiscalYearByYearParams{
		CompanyProfileID: companyID,
		Year:             int32(year),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get fiscal year (%d): %w", year, err)
	}
	return &spine.FiscalYear{
		ID:               row.ID,
		CompanyProfileID: row.CompanyProfileID,
		Year:             int(row.Year),
		StartDate:        row.StartDate,
		EndDate:          row.EndDate,
		Status:           spine.FiscalYearStatus(row.Status),
		CreatedAt:        row.CreatedAt,
		UpdatedAt:        row.UpdatedAt,
	}, nil
}

func (r *SpineRepo) SavePeriod(ctx context.Context, p *spine.AccountingPeriod) error {
	arg := sqlc.UpsertAccountingPeriodParams{
		ID:               p.ID,
		FiscalYearID:     p.FiscalYearID,
		CompanyProfileID: p.CompanyProfileID,
		PeriodNumber:     int32(p.PeriodNumber),
		Name:             p.Name,
		StartDate:        p.StartDate,
		EndDate:          p.EndDate,
		LockDate:         p.LockDate,
		Status:           sqlc.AccountingPeriodsStatus(p.Status),
	}
	if err := r.queries.UpsertAccountingPeriod(ctx, arg); err != nil {
		return fmt.Errorf("failed to upsert accounting period (%d): %w", p.PeriodNumber, err)
	}
	return nil
}

func (r *SpineRepo) GetPeriodByDate(ctx context.Context, companyID string, date time.Time) (*spine.AccountingPeriod, error) {
	row, err := r.queries.GetPeriodByDate(ctx, sqlc.GetPeriodByDateParams{
		CompanyProfileID: companyID,
		StartDate:        date,
		EndDate:          date,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get period for date %s: %w", date.Format("2006-01-02"), err)
	}
	return &spine.AccountingPeriod{
		ID:               row.ID,
		FiscalYearID:     row.FiscalYearID,
		CompanyProfileID: row.CompanyProfileID,
		PeriodNumber:     int(row.PeriodNumber),
		Name:             row.Name,
		StartDate:        row.StartDate,
		EndDate:          row.EndDate,
		LockDate:         row.LockDate,
		Status:           spine.PeriodStatus(row.Status),
		CreatedAt:        row.CreatedAt,
		UpdatedAt:        row.UpdatedAt,
	}, nil
}

func (r *SpineRepo) ListPeriodsByFiscalYear(ctx context.Context, fyID string) ([]spine.AccountingPeriod, error) {
	rows, err := r.queries.ListPeriodsByFiscalYear(ctx, fyID)
	if err != nil {
		return nil, fmt.Errorf("failed to list periods for fiscal year %s: %w", fyID, err)
	}
	list := make([]spine.AccountingPeriod, 0, len(rows))
	for _, row := range rows {
		list = append(list, spine.AccountingPeriod{
			ID:               row.ID,
			FiscalYearID:     row.FiscalYearID,
			CompanyProfileID: row.CompanyProfileID,
			PeriodNumber:     int(row.PeriodNumber),
			Name:             row.Name,
			StartDate:        row.StartDate,
			EndDate:          row.EndDate,
			LockDate:         row.LockDate,
			Status:           spine.PeriodStatus(row.Status),
			CreatedAt:        row.CreatedAt,
			UpdatedAt:        row.UpdatedAt,
		})
	}
	return list, nil
}

func (r *SpineRepo) UpdatePeriodLock(ctx context.Context, id string, lockDate time.Time, status spine.PeriodStatus) error {
	if err := r.queries.UpdatePeriodLock(ctx, sqlc.UpdatePeriodLockParams{
		LockDate: lockDate,
		Status:   sqlc.AccountingPeriodsStatus(status),
		ID:       id,
	}); err != nil {
		return fmt.Errorf("failed to update period lock (%s): %w", id, err)
	}
	return nil
}

// ======================== COST CENTERS & EXPENSE ITEMS ========================

func (r *SpineRepo) SaveCostCenter(ctx context.Context, cc *spine.CostCenter) error {
	arg := sqlc.UpsertCostCenterParams{
		ID:               cc.ID,
		CompanyProfileID: cc.CompanyProfileID,
		BranchID:         ptrToNullString(cc.BranchID),
		Code:             cc.Code,
		Name:             cc.Name,
		ParentID:         ptrToNullString(cc.ParentID),
		IsLeaf:           cc.IsLeaf,
		IsActive:         cc.IsActive,
	}
	if err := r.queries.UpsertCostCenter(ctx, arg); err != nil {
		return fmt.Errorf("failed to upsert cost center (%s): %w", cc.Code, err)
	}
	return nil
}

func (r *SpineRepo) GetCostCenterByCode(ctx context.Context, companyID, code string) (*spine.CostCenter, error) {
	row, err := r.queries.GetCostCenterByCode(ctx, sqlc.GetCostCenterByCodeParams{
		CompanyProfileID: companyID,
		Code:             code,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get cost center (%s): %w", code, err)
	}
	return &spine.CostCenter{
		ID:               row.ID,
		CompanyProfileID: row.CompanyProfileID,
		BranchID:         nullStringToPtr(row.BranchID),
		Code:             row.Code,
		Name:             row.Name,
		ParentID:         nullStringToPtr(row.ParentID),
		IsLeaf:           row.IsLeaf,
		IsActive:         row.IsActive,
		CreatedAt:        row.CreatedAt,
		UpdatedAt:        row.UpdatedAt,
	}, nil
}

func (r *SpineRepo) ListCostCenters(ctx context.Context, companyID string) ([]spine.CostCenter, error) {
	rows, err := r.queries.ListCostCentersByCompany(ctx, companyID)
	if err != nil {
		return nil, fmt.Errorf("failed to list cost centers: %w", err)
	}
	list := make([]spine.CostCenter, 0, len(rows))
	for _, row := range rows {
		list = append(list, spine.CostCenter{
			ID:               row.ID,
			CompanyProfileID: row.CompanyProfileID,
			BranchID:         nullStringToPtr(row.BranchID),
			Code:             row.Code,
			Name:             row.Name,
			ParentID:         nullStringToPtr(row.ParentID),
			IsLeaf:           row.IsLeaf,
			IsActive:         row.IsActive,
			CreatedAt:        row.CreatedAt,
			UpdatedAt:        row.UpdatedAt,
		})
	}
	return list, nil
}

func (r *SpineRepo) SaveExpenseItem(ctx context.Context, ei *spine.ExpenseItem) error {
	arg := sqlc.UpsertExpenseItemParams{
		ID:               ei.ID,
		CompanyProfileID: ei.CompanyProfileID,
		Code:             ei.Code,
		Name:             ei.Name,
		Category:         string(ei.Category),
		ParentID:         ptrToNullString(ei.ParentID),
		IsLeaf:           ei.IsLeaf,
		IsActive:         ei.IsActive,
	}
	if err := r.queries.UpsertExpenseItem(ctx, arg); err != nil {
		return fmt.Errorf("failed to upsert expense item (%s): %w", ei.Code, err)
	}
	return nil
}

func (r *SpineRepo) GetExpenseItemByCode(ctx context.Context, companyID, code string) (*spine.ExpenseItem, error) {
	row, err := r.queries.GetExpenseItemByCode(ctx, sqlc.GetExpenseItemByCodeParams{
		CompanyProfileID: companyID,
		Code:             code,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get expense item (%s): %w", code, err)
	}
	return &spine.ExpenseItem{
		ID:               row.ID,
		CompanyProfileID: row.CompanyProfileID,
		Code:             row.Code,
		Name:             row.Name,
		Category:         spine.ExpenseCategory(row.Category),
		ParentID:         nullStringToPtr(row.ParentID),
		IsLeaf:           row.IsLeaf,
		IsActive:         row.IsActive,
		CreatedAt:        row.CreatedAt,
		UpdatedAt:        row.UpdatedAt,
	}, nil
}

func (r *SpineRepo) ListExpenseItems(ctx context.Context, companyID string) ([]spine.ExpenseItem, error) {
	rows, err := r.queries.ListExpenseItemsByCompany(ctx, companyID)
	if err != nil {
		return nil, fmt.Errorf("failed to list expense items: %w", err)
	}
	list := make([]spine.ExpenseItem, 0, len(rows))
	for _, row := range rows {
		list = append(list, spine.ExpenseItem{
			ID:               row.ID,
			CompanyProfileID: row.CompanyProfileID,
			Code:             row.Code,
			Name:             row.Name,
			Category:         spine.ExpenseCategory(row.Category),
			ParentID:         nullStringToPtr(row.ParentID),
			IsLeaf:           row.IsLeaf,
			IsActive:         row.IsActive,
			CreatedAt:        row.CreatedAt,
			UpdatedAt:        row.UpdatedAt,
		})
	}
	return list, nil
}

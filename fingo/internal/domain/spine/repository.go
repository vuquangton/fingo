package spine

import (
	"context"
	"time"
)

// CurrencyRepository defines the persistence seam for currencies
type CurrencyRepository interface {
	SaveCurrency(ctx context.Context, c *Currency) error
	GetCurrencyByCode(ctx context.Context, companyID, code string) (*Currency, error)
	GetBaseCurrency(ctx context.Context, companyID string) (*Currency, error)
	ListCurrencies(ctx context.Context, companyID string) ([]Currency, error)
}

// ExchangeRateRepository defines the persistence seam for daily exchange rates
type ExchangeRateRepository interface {
	SaveExchangeRate(ctx context.Context, rate *ExchangeRate) error
	GetEffectiveRate(ctx context.Context, companyID, currCode string, rateDate time.Time, rType RateType) (*ExchangeRate, error)
	ListRatesByDate(ctx context.Context, companyID string, rateDate time.Time) ([]ExchangeRate, error)
}

// AccountRepository defines the persistence seam for Chart of Accounts
type AccountRepository interface {
	SaveAccount(ctx context.Context, acc *Account) error
	GetAccountByID(ctx context.Context, id string) (*Account, error)
	GetAccountByCode(ctx context.Context, companyID, code string) (*Account, error)
	ListAccounts(ctx context.Context, companyID string) ([]Account, error)
	ListChildAccounts(ctx context.Context, parentID string) ([]Account, error)
	UpdateLeafStatus(ctx context.Context, id string, isLeaf bool) error
}

// PeriodRepository defines the persistence seam for FiscalYears and AccountingPeriods
type PeriodRepository interface {
	SaveFiscalYear(ctx context.Context, fy *FiscalYear) error
	GetFiscalYearByYear(ctx context.Context, companyID string, year int) (*FiscalYear, error)
	SavePeriod(ctx context.Context, p *AccountingPeriod) error
	GetPeriodByDate(ctx context.Context, companyID string, date time.Time) (*AccountingPeriod, error)
	ListPeriodsByFiscalYear(ctx context.Context, fyID string) ([]AccountingPeriod, error)
	UpdatePeriodLock(ctx context.Context, id string, lockDate time.Time, status PeriodStatus) error
}

// CostDimensionRepository defines persistence for CostCenters and ExpenseItems
type CostDimensionRepository interface {
	SaveCostCenter(ctx context.Context, cc *CostCenter) error
	GetCostCenterByCode(ctx context.Context, companyID, code string) (*CostCenter, error)
	ListCostCenters(ctx context.Context, companyID string) ([]CostCenter, error)
	SaveExpenseItem(ctx context.Context, ei *ExpenseItem) error
	GetExpenseItemByCode(ctx context.Context, companyID, code string) (*ExpenseItem, error)
	ListExpenseItems(ctx context.Context, companyID string) ([]ExpenseItem, error)
}

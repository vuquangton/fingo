package spine

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/shopspring/decimal"
)

var (
	ErrInvalidCurrencyCode       = errors.New("currency code must be a valid 3-letter ISO-4217 code")
	ErrBaseCurrencyCannotHaveRate = errors.New("base currency cannot have an exchange rate other than 1.0")
	ErrNegativeExchangeRate       = errors.New("exchange rate must be strictly positive")
	ErrInvalidDecimalPlaces      = errors.New("decimal places must be between 0 and 6")
)

type RateType string

const (
	RateTypeBuyTransfer  RateType = "BUY_TRANSFER"
	RateTypeSellTransfer RateType = "SELL_TRANSFER"
	RateTypeCentralSBV   RateType = "CENTRAL_SBV"
)

// Currency represents an ISO 4217 currency definition within a company profile
type Currency struct {
	Code             string    `json:"code"`
	CompanyProfileID string    `json:"company_profile_id"`
	Name             string    `json:"name"`
	Symbol           string    `json:"symbol"`
	DecimalPlaces    int32     `json:"decimal_places"`
	IsBase           bool      `json:"is_base"`
	IsActive         bool      `json:"is_active"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

func NewCurrency(code, companyID, name, symbol string, decimalPlaces int32, isBase bool) (*Currency, error) {
	code = strings.ToUpper(strings.TrimSpace(code))
	if len(code) != 3 {
		return nil, fmt.Errorf("%w: '%s'", ErrInvalidCurrencyCode, code)
	}
	if decimalPlaces < 0 || decimalPlaces > 6 {
		return nil, fmt.Errorf("%w: %d", ErrInvalidDecimalPlaces, decimalPlaces)
	}

	return &Currency{
		Code:             code,
		CompanyProfileID: companyID,
		Name:             name,
		Symbol:           symbol,
		DecimalPlaces:    decimalPlaces,
		IsBase:           isBase,
		IsActive:         true,
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
	}, nil
}

// ExchangeRate defines a point-in-time conversion rate between a foreign currency and base currency
type ExchangeRate struct {
	ID               string          `json:"id"`
	CompanyProfileID string          `json:"company_profile_id"`
	CurrencyCode     string          `json:"currency_code"`
	RateDate         time.Time       `json:"rate_date"`
	RateType         RateType        `json:"rate_type"`
	Rate             decimal.Decimal `json:"rate"`
	SourceBank       string          `json:"source_bank"`
	CreatedAt        time.Time       `json:"created_at"`
	CreatedBy        string          `json:"created_by"`
}

func NewExchangeRate(
	id, companyID, currCode string,
	rateDate time.Time,
	rType RateType,
	rate decimal.Decimal,
	sourceBank, createdBy string,
) (*ExchangeRate, error) {
	currCode = strings.ToUpper(strings.TrimSpace(currCode))
	if len(currCode) != 3 {
		return nil, fmt.Errorf("%w: '%s'", ErrInvalidCurrencyCode, currCode)
	}
	if rate.LessThanOrEqual(decimal.Zero) {
		return nil, fmt.Errorf("%w: %s", ErrNegativeExchangeRate, rate.String())
	}
	if sourceBank == "" {
		sourceBank = "VIETCOMBANK"
	}

	return &ExchangeRate{
		ID:               id,
		CompanyProfileID: companyID,
		CurrencyCode:     currCode,
		RateDate:         rateDate,
		RateType:         rType,
		Rate:             rate,
		SourceBank:       sourceBank,
		CreatedAt:        time.Now(),
		CreatedBy:        createdBy,
	}, nil
}

// ConvertCurrency converts a foreign currency amount to base currency (or vice versa) with rounding
func ConvertCurrency(amountFC, rate decimal.Decimal, decimals int32) decimal.Decimal {
	if decimals < 0 {
		decimals = 0
	}
	converted := amountFC.Mul(rate)
	return converted.RoundBank(decimals)
}

// RevaluationResult represents the VAS 10 unrealized FX revaluation calculation outcome.
type RevaluationResult struct {
	AccountCode   string          `json:"account_code"`
	CurrencyCode  string          `json:"currency_code"`
	BookValue     decimal.Decimal `json:"book_value"`
	RevaluedValue decimal.Decimal `json:"revalued_value"`
	Diff          decimal.Decimal `json:"diff"`
	IsGain        bool            `json:"is_gain"`
	DebitAccount  string          `json:"debit_account"`
	CreditAccount string          `json:"credit_account"`
}

// ComputeFXRevaluation evaluates VAS 10 unrealized exchange gain/loss based on account category.
// Statutory Rule:
// - Monetary Assets (TK 1112, 1122, 131): Revaluation increase (diff > 0) is GAIN (Nợ 1112/1122/131, Có 4131).
// - Monetary Liabilities (TK 331, 341): Revaluation increase (diff > 0) is LOSS (Nợ 4131, Có 331/341).
func ComputeFXRevaluation(accountCode string, category AccountCategory, currencyCode string, amountFC, bookRate, currentRate decimal.Decimal, decimals int32) RevaluationResult {
	bookVal := ConvertCurrency(amountFC, bookRate, decimals)
	revalVal := ConvertCurrency(amountFC, currentRate, decimals)
	diff := revalVal.Sub(bookVal)

	res := RevaluationResult{
		AccountCode:   accountCode,
		CurrencyCode:  currencyCode,
		BookValue:     bookVal,
		RevaluedValue: revalVal,
		Diff:          diff,
	}

	isLiability := category == CategoryLiability || strings.HasPrefix(accountCode, "3")

	if isLiability {
		if diff.IsPositive() {
			// Liability value rose: statutory loss
			res.IsGain = false
			res.DebitAccount = "4131"
			res.CreditAccount = accountCode
		} else if diff.IsNegative() {
			// Liability value dropped: statutory gain
			res.IsGain = true
			res.DebitAccount = accountCode
			res.CreditAccount = "4131"
		}
	} else {
		if diff.IsPositive() {
			// Asset value rose: statutory gain
			res.IsGain = true
			res.DebitAccount = accountCode
			res.CreditAccount = "4131"
		} else if diff.IsNegative() {
			// Asset value dropped: statutory loss
			res.IsGain = false
			res.DebitAccount = "4131"
			res.CreditAccount = accountCode
		}
	}

	return res
}


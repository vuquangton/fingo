package spine_test

import (
	"testing"
	"time"

	"fingo/internal/domain/spine"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCurrency_Creation_And_Conversion(t *testing.T) {
	t.Parallel()

	t.Run("Valid Currency creation", func(t *testing.T) {
		c, err := spine.NewCurrency("VND", "comp-1", "Đồng Việt Nam", "₫", 0, true)
		require.NoError(t, err)
		assert.Equal(t, "VND", c.Code)
		assert.True(t, c.IsBase)
		assert.Equal(t, int32(0), c.DecimalPlaces)
	})

	t.Run("Invalid Currency code rejected", func(t *testing.T) {
		_, err := spine.NewCurrency("US", "comp-1", "Dollar", "$", 2, false)
		require.ErrorIs(t, err, spine.ErrInvalidCurrencyCode)

		_, err = spine.NewCurrency("TOOLONG", "comp-1", "Dollar", "$", 2, false)
		require.ErrorIs(t, err, spine.ErrInvalidCurrencyCode)
	})

	t.Run("Invalid decimal places rejected", func(t *testing.T) {
		_, err := spine.NewCurrency("USD", "comp-1", "US Dollar", "$", -1, false)
		require.ErrorIs(t, err, spine.ErrInvalidDecimalPlaces)

		_, err = spine.NewCurrency("USD", "comp-1", "US Dollar", "$", 7, false)
		require.ErrorIs(t, err, spine.ErrInvalidDecimalPlaces)
	})

	t.Run("ConvertCurrency rounding precision", func(t *testing.T) {
		// 100.55 USD at 25450.50 VND/USD -> 2559047.775 VND -> Banker's rounded to 0 decimal: 2559048
		amountFC := decimal.RequireFromString("100.55")
		rate := decimal.RequireFromString("25450.50")
		convertedVND := spine.ConvertCurrency(amountFC, rate, 0)
		assert.Equal(t, "2559048", convertedVND.String())

		// Convert to 2 decimal places (e.g. EUR)
		rateEUR := decimal.RequireFromString("1.0825")
		convertedEUR := spine.ConvertCurrency(amountFC, rateEUR, 2)
		assert.Equal(t, "108.85", convertedEUR.String())
	})
}

func TestExchangeRate_Creation(t *testing.T) {
	t.Parallel()

	t.Run("Valid ExchangeRate creation", func(t *testing.T) {
		d := time.Date(2026, 3, 20, 0, 0, 0, 0, time.UTC)
		r, err := spine.NewExchangeRate(
			"fx-1", "comp-1", "usd", d,
			spine.RateTypeBuyTransfer, decimal.RequireFromString("25450.0"),
			"VIETCOMBANK", "user-1",
		)
		require.NoError(t, err)
		assert.Equal(t, "USD", r.CurrencyCode)
		assert.Equal(t, spine.RateTypeBuyTransfer, r.RateType)
	})

	t.Run("Negative or zero ExchangeRate rejected", func(t *testing.T) {
		d := time.Date(2026, 3, 20, 0, 0, 0, 0, time.UTC)
		_, err := spine.NewExchangeRate(
			"fx-2", "comp-1", "USD", d,
			spine.RateTypeBuyTransfer, decimal.Zero,
			"", "user-1",
		)
		require.ErrorIs(t, err, spine.ErrNegativeExchangeRate)

		_, err = spine.NewExchangeRate(
			"fx-3", "comp-1", "USD", d,
			spine.RateTypeBuyTransfer, decimal.RequireFromString("-25000"),
			"", "user-1",
		)
		require.ErrorIs(t, err, spine.ErrNegativeExchangeRate)
	})
}

func TestAccount_Validation_And_DualBalance(t *testing.T) {
	t.Parallel()

	t.Run("Valid Account creation", func(t *testing.T) {
		acc, err := spine.NewAccount(
			"acc-1", "comp-1", "1111", "Tiền Việt Nam",
			nil, 2, spine.NatureDebit, spine.CategoryAsset, true,
		)
		require.NoError(t, err)
		assert.Equal(t, "1111", acc.Code)
		assert.True(t, acc.IsLeaf)
	})

	t.Run("Invalid Account code rejected", func(t *testing.T) {
		_, err := spine.NewAccount(
			"acc-2", "comp-1", "11", "Short",
			nil, 1, spine.NatureDebit, spine.CategoryAsset, true,
		)
		require.ErrorIs(t, err, spine.ErrInvalidAccountCode)
	})

	t.Run("Circular self-parent hierarchy rejected", func(t *testing.T) {
		selfID := "acc-3"
		_, err := spine.NewAccount(
			selfID, "comp-1", "111", "Cash",
			&selfID, 1, spine.NatureDebit, spine.CategoryAsset, false,
		)
		require.ErrorIs(t, err, spine.ErrCircularAccountHierarchy)
	})

	t.Run("ValidatePostingAccount rejects parent accounts (INV-SPINE-01)", func(t *testing.T) {
		parentAcc := &spine.Account{
			Code:     "111",
			Name:     "Tiền mặt",
			IsLeaf:   false,
			IsActive: true,
		}
		err := spine.ValidatePostingAccount(parentAcc)
		require.ErrorIs(t, err, spine.ErrPostingToParentAccount)
	})

	t.Run("ValidatePostingAccount rejects inactive accounts", func(t *testing.T) {
		inactiveLeaf := &spine.Account{
			Code:     "1111",
			Name:     "Tiền VN",
			IsLeaf:   true,
			IsActive: false,
		}
		err := spine.ValidatePostingAccount(inactiveLeaf)
		require.ErrorIs(t, err, spine.ErrInactiveAccount)
	})

	t.Run("ValidatePostingAccount allows active leaf account", func(t *testing.T) {
		activeLeaf := &spine.Account{
			Code:     "1111",
			Name:     "Tiền VN",
			IsLeaf:   true,
			IsActive: true,
		}
		err := spine.ValidatePostingAccount(activeLeaf)
		require.NoError(t, err)
	})

	t.Run("SeparateDualBalances preserves independent Debit and Credit totals (INV-SPINE-02)", func(t *testing.T) {
		// Customer subledger balances in Account 131:
		// Cust A: owes 100M (+100)
		// Cust B: advanced 30M (-30)
		// Cust C: owes 50M (+50)
		// Cust D: advanced 10M (-10)
		partnerBalances := map[string]decimal.Decimal{
			"cust-a": decimal.NewFromInt(100_000_000),
			"cust-b": decimal.NewFromInt(-30_000_000),
			"cust-c": decimal.NewFromInt(50_000_000),
			"cust-d": decimal.NewFromInt(-10_000_000),
		}

		debitSum, creditSum := spine.SeparateDualBalances(partnerBalances)
		assert.Equal(t, "150000000", debitSum.String(), "Total Debit (Asset) must equal exactly 150M")
		assert.Equal(t, "40000000", creditSum.String(), "Total Credit (Liability) must equal exactly 40M without netting")
	})
}

func TestPeriod_Lock_Barrier(t *testing.T) {
	t.Parallel()

	t.Run("Valid FiscalYear and Period creation", func(t *testing.T) {
		start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
		end := time.Date(2026, 12, 31, 0, 0, 0, 0, time.UTC)
		fy, err := spine.NewFiscalYear("fy-1", "comp-1", 2026, start, end)
		require.NoError(t, err)
		assert.Equal(t, 2026, fy.Year)

		pEnd := time.Date(2026, 1, 31, 0, 0, 0, 0, time.UTC)
		p, err := spine.NewAccountingPeriod("p-1", fy.ID, "comp-1", 1, "Tháng 01/2026", start, pEnd, pEnd)
		require.NoError(t, err)
		assert.Equal(t, 1, p.PeriodNumber)
	})

	t.Run("Invalid FiscalYear date range rejected", func(t *testing.T) {
		start := time.Date(2026, 12, 31, 0, 0, 0, 0, time.UTC)
		end := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
		_, err := spine.NewFiscalYear("fy-2", "comp-1", 2026, start, end)
		require.ErrorIs(t, err, spine.ErrInvalidFiscalYearDate)
	})

	t.Run("CheckPeriodLock: voucher date on or before lock date MUST BLOCK (INV-SPINE-04)", func(t *testing.T) {
		lockDate := time.Date(2026, 1, 31, 23, 59, 59, 0, time.UTC)

		// Before lock date
		vDateBefore := time.Date(2026, 1, 15, 10, 0, 0, 0, time.UTC)
		err := spine.CheckPeriodLock(vDateBefore, lockDate)
		require.ErrorIs(t, err, spine.ErrPeriodLocked)

		// Exact calendar lock date
		vDateExact := time.Date(2026, 1, 31, 8, 30, 0, 0, time.UTC)
		err = spine.CheckPeriodLock(vDateExact, lockDate)
		require.ErrorIs(t, err, spine.ErrPeriodLocked)

		// After lock date -> ALLOWED
		vDateAfter := time.Date(2026, 2, 1, 9, 0, 0, 0, time.UTC)
		err = spine.CheckPeriodLock(vDateAfter, lockDate)
		require.NoError(t, err)
	})
}

func TestCostCenter_And_ExpenseItem(t *testing.T) {
	t.Parallel()

	t.Run("Valid CostCenter creation", func(t *testing.T) {
		cc, err := spine.NewCostCenter("cc-1", "comp-1", nil, "PHONG_KD", "Phòng Kinh Doanh", nil, true)
		require.NoError(t, err)
		assert.Equal(t, "PHONG_KD", cc.Code)
		assert.True(t, cc.IsLeaf)
	})

	t.Run("CostCenter self-parent circular chain rejected", func(t *testing.T) {
		selfID := "cc-2"
		_, err := spine.NewCostCenter(selfID, "comp-1", nil, "PHONG_KT", "Phòng Kế Toán", &selfID, true)
		require.ErrorIs(t, err, spine.ErrCircularParentChain)
	})

	t.Run("Valid ExpenseItem creation", func(t *testing.T) {
		ei, err := spine.NewExpenseItem("ei-1", "comp-1", "VPP", "Văn phòng phẩm", spine.ExpenseCategoryMaterial, nil, true)
		require.NoError(t, err)
		assert.Equal(t, "VPP", ei.Code)
		assert.Equal(t, spine.ExpenseCategoryMaterial, ei.Category)
	})

	t.Run("ValidateLineAllocation enforces mandatory dimensions (INV-SPINE-07)", func(t *testing.T) {
		accExpense := &spine.Account{
			Code:                "6422",
			Name:                "Chi phí quản lý",
			RequiresExpenseItem: true,
			RequiresCostCenter:  true,
		}

		// Missing expense item
		ccID := "cc-01"
		err := spine.ValidateLineAllocation(accExpense, &ccID, nil)
		require.ErrorIs(t, err, spine.ErrMissingExpenseItem)

		// Missing cost center
		eiID := "ei-01"
		err = spine.ValidateLineAllocation(accExpense, nil, &eiID)
		require.ErrorIs(t, err, spine.ErrMissingCostCenter)

		// Both provided -> ALLOWED
		err = spine.ValidateLineAllocation(accExpense, &ccID, &eiID)
		require.NoError(t, err)

		// Nil account
		err = spine.ValidateLineAllocation(nil, &ccID, &eiID)
		require.Error(t, err)
	})

	t.Run("CostCenter and ExpenseItem constructor edge cases", func(t *testing.T) {
		_, err := spine.NewCostCenter("cc-3", "comp-1", nil, "X", "Too short", nil, true)
		require.ErrorIs(t, err, spine.ErrInvalidCode)

		_, err = spine.NewExpenseItem("ei-2", "comp-1", "Y", "Too short", spine.ExpenseCategoryMaterial, nil, true)
		require.ErrorIs(t, err, spine.ErrInvalidCode)

		selfID := "ei-3"
		_, err = spine.NewExpenseItem(selfID, "comp-1", "VPP_SELF", "Self Parent", spine.ExpenseCategoryMaterial, &selfID, true)
		require.ErrorIs(t, err, spine.ErrCircularParentChain)
	})
}

func TestDomainSpine_Additional_Branch_Coverage(t *testing.T) {
	t.Parallel()

	t.Run("ConvertCurrency negative decimals defaults to 0", func(t *testing.T) {
		res := spine.ConvertCurrency(decimal.RequireFromString("10.55"), decimal.NewFromInt(1), -1)
		assert.Equal(t, "11", res.String())
	})

	t.Run("NewExchangeRate invalid currency code", func(t *testing.T) {
		d := time.Date(2026, 3, 20, 0, 0, 0, 0, time.UTC)
		_, err := spine.NewExchangeRate("fx-x", "c-1", "US", d, spine.RateTypeBuyTransfer, decimal.NewFromInt(25000), "", "u1")
		require.ErrorIs(t, err, spine.ErrInvalidCurrencyCode)
	})

	t.Run("NewAccount level defaults to 1 when level < 1", func(t *testing.T) {
		acc, err := spine.NewAccount("a-x", "c-1", "1111", "Cash", nil, 0, spine.NatureDebit, spine.CategoryAsset, true)
		require.NoError(t, err)
		assert.Equal(t, 1, acc.AccountLevel)
	})

	t.Run("ValidatePostingAccount nil account", func(t *testing.T) {
		err := spine.ValidatePostingAccount(nil)
		require.Error(t, err)
	})

	t.Run("NewAccountingPeriod invalid period number or inverted dates", func(t *testing.T) {
		start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
		end := time.Date(2026, 1, 31, 0, 0, 0, 0, time.UTC)
		_, err := spine.NewAccountingPeriod("p-x", "fy-1", "c-1", 0, "M0", start, end, end)
		require.ErrorIs(t, err, spine.ErrInvalidPeriodNumber)

		_, err = spine.NewAccountingPeriod("p-x", "fy-1", "c-1", 14, "M14", start, end, end)
		require.ErrorIs(t, err, spine.ErrInvalidPeriodNumber)

		_, err = spine.NewAccountingPeriod("p-x", "fy-1", "c-1", 1, "M1", end, start, end)
		require.ErrorIs(t, err, spine.ErrInvalidPeriodDates)
	})

	t.Run("CheckPeriodLock zero lock date is allowed", func(t *testing.T) {
		vDate := time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC)
		err := spine.CheckPeriodLock(vDate, time.Time{})
		require.NoError(t, err)
	})

	t.Run("ComputeFXRevaluation Asset Gain and Loss", func(t *testing.T) {
		// Asset 1112: Rate goes up (25000 -> 25500) -> GAIN
		resGain := spine.ComputeFXRevaluation(
			"1112", spine.CategoryAsset, "USD",
			decimal.RequireFromString("1000"), decimal.RequireFromString("25000"), decimal.RequireFromString("25500"), 0,
		)
		assert.True(t, resGain.IsGain)
		assert.Equal(t, "1112", resGain.DebitAccount)
		assert.Equal(t, "4131", resGain.CreditAccount)
		assert.Equal(t, "500000", resGain.Diff.String())

		// Asset 1112: Rate goes down (25500 -> 25000) -> LOSS
		resLoss := spine.ComputeFXRevaluation(
			"1112", spine.CategoryAsset, "USD",
			decimal.RequireFromString("1000"), decimal.RequireFromString("25500"), decimal.RequireFromString("25000"), 0,
		)
		assert.False(t, resLoss.IsGain)
		assert.Equal(t, "4131", resLoss.DebitAccount)
		assert.Equal(t, "1112", resLoss.CreditAccount)
	})

	t.Run("ComputeFXRevaluation Liability Gain and Loss (TK 331)", func(t *testing.T) {
		// Liability 331: Rate goes up (25000 -> 25500) -> Company owes more -> LOSS
		resLoss := spine.ComputeFXRevaluation(
			"331", spine.CategoryLiability, "USD",
			decimal.RequireFromString("1000"), decimal.RequireFromString("25000"), decimal.RequireFromString("25500"), 0,
		)
		assert.False(t, resLoss.IsGain)
		assert.Equal(t, "4131", resLoss.DebitAccount)
		assert.Equal(t, "331", resLoss.CreditAccount)
		assert.Equal(t, "500000", resLoss.Diff.String())

		// Liability 331: Rate goes down (25500 -> 25000) -> Company owes less -> GAIN
		resGain := spine.ComputeFXRevaluation(
			"331", spine.CategoryLiability, "USD",
			decimal.RequireFromString("1000"), decimal.RequireFromString("25500"), decimal.RequireFromString("25000"), 0,
		)
		assert.True(t, resGain.IsGain)
		assert.Equal(t, "331", resGain.DebitAccount)
		assert.Equal(t, "4131", resGain.CreditAccount)
	})

	t.Run("IsStatutoryNominalExpenseAccount check", func(t *testing.T) {
		assert.True(t, spine.IsStatutoryNominalExpenseAccount("154"))
		assert.True(t, spine.IsStatutoryNominalExpenseAccount("6421"))
		assert.True(t, spine.IsStatutoryNominalExpenseAccount("627"))
		assert.False(t, spine.IsStatutoryNominalExpenseAccount("1111"))
		assert.False(t, spine.IsStatutoryNominalExpenseAccount("131"))
	})
}


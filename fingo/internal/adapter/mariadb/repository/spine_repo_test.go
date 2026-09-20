package repository_test

import (
	"context"
	"testing"
	"time"

	"fingo/internal/adapter/mariadb/repository"
	"fingo/internal/domain/spine"
	"fingo/internal/domain/system"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSpineRepo_Integration(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	companyRepo := repository.NewCompanyProfileRepo(db)
	spineRepo := repository.NewSpineRepo(db)
	ctx := context.Background()

	// 1. Ensure company exists
	var companyID string
	activeComp, err := companyRepo.GetProfile(ctx)
	if err == nil && activeComp != nil {
		companyID = activeComp.ID
	} else {
		companyID = "test-cp-spine-01"
		comp, err := system.NewProductionCompanyProfile(system.CreateCompanyProfileParams{
			ID:                  companyID,
			TaxCode:             "0101243150",
			LegalName:           "CÔNG TY TEST SPINE REPO",
			Address:             "Hà Nội",
			LegalRepresentative: "Giám Đốc",
			ChiefAccountant:     "Kế Toán Trưởng",
			TaxAuthorityCode:    "10500",
			TaxAuthorityName:    "Cục Thuế Cầu Giấy",
			Regime:              system.RegimeCircular133,
			VATMethod:           system.VATMethodDeduction,
			CostingMethod:       system.CostingMovingWeighted,
			BusinessType:        system.BusinessTrading,
		})
		require.NoError(t, err)
		require.NoError(t, companyRepo.SaveProfile(ctx, comp))
	}

	t.Run("Currencies and Exchange Rates CRUD", func(t *testing.T) {
		// Save base currency VND
		vnd, err := spine.NewCurrency("VND", companyID, "Đồng Việt Nam", "₫", 0, true)
		require.NoError(t, err)
		err = spineRepo.SaveCurrency(ctx, vnd)
		require.NoError(t, err)

		// Save foreign currency USD
		usd, err := spine.NewCurrency("USD", companyID, "Đô la Mỹ", "$", 2, false)
		require.NoError(t, err)
		err = spineRepo.SaveCurrency(ctx, usd)
		require.NoError(t, err)

		// Verify GetBaseCurrency
		baseCurr, err := spineRepo.GetBaseCurrency(ctx, companyID)
		require.NoError(t, err)
		assert.Equal(t, "VND", baseCurr.Code)
		assert.True(t, baseCurr.IsBase)

		// Save Exchange Rate
		d := time.Date(2026, 3, 20, 0, 0, 0, 0, time.UTC)
		rate, err := spine.NewExchangeRate(
			"fx-int-01", companyID, "USD", d,
			spine.RateTypeBuyTransfer, decimal.RequireFromString("25450.0"),
			"VIETCOMBANK", "test-user",
		)
		require.NoError(t, err)
		err = spineRepo.SaveExchangeRate(ctx, rate)
		require.NoError(t, err)

		// Query Effective Rate (exact date or past fallback)
		fetchedRate, err := spineRepo.GetEffectiveRate(ctx, companyID, "USD", d, spine.RateTypeBuyTransfer)
		require.NoError(t, err)
		assert.Equal(t, "25450", fetchedRate.Rate.String())
	})

	t.Run("Chart of Accounts Hierarchy CRUD", func(t *testing.T) {
		parentID := "acc-p-111"
		childID := "acc-c-1111"

		// Clean up previous runs
		_, _ = db.Exec("DELETE FROM accounts WHERE company_profile_id = ? AND code IN ('111', '1111')", companyID)

		// 1. Create Parent Account 111
		pAcc, err := spine.NewAccount(
			parentID, companyID, "111", "Tiền mặt",
			nil, 1, spine.NatureDebit, spine.CategoryAsset, false,
		)
		require.NoError(t, err)
		err = spineRepo.SaveAccount(ctx, pAcc)
		require.NoError(t, err)

		// 2. Create Child Account 1111
		cAcc, err := spine.NewAccount(
			childID, companyID, "1111", "Tiền Việt Nam",
			&parentID, 2, spine.NatureDebit, spine.CategoryAsset, true,
		)
		require.NoError(t, err)
		err = spineRepo.SaveAccount(ctx, cAcc)
		require.NoError(t, err)

		// 3. Query Account by Code
		fetchedChild, err := spineRepo.GetAccountByCode(ctx, companyID, "1111")
		require.NoError(t, err)
		assert.Equal(t, "1111", fetchedChild.Code)
		assert.True(t, fetchedChild.IsLeaf)
		require.NotNil(t, fetchedChild.ParentID)
		assert.Equal(t, parentID, *fetchedChild.ParentID)

		// 4. Query Child Accounts of 111
		children, err := spineRepo.ListChildAccounts(ctx, parentID)
		require.NoError(t, err)
		assert.NotEmpty(t, children)
		assert.Equal(t, "1111", children[0].Code)
	})

	t.Run("Fiscal Year and Accounting Period with Lock Date", func(t *testing.T) {
		fyID := "fy-int-2026"
		periodID := "p-int-2026-01"

		_, _ = db.Exec("DELETE FROM accounting_periods WHERE company_profile_id = ?", companyID)
		_, _ = db.Exec("DELETE FROM fiscal_years WHERE company_profile_id = ?", companyID)

		// 1. Create Fiscal Year 2026
		start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
		end := time.Date(2026, 12, 31, 0, 0, 0, 0, time.UTC)
		fy, err := spine.NewFiscalYear(fyID, companyID, 2026, start, end)
		require.NoError(t, err)
		err = spineRepo.SaveFiscalYear(ctx, fy)
		require.NoError(t, err)

		// 2. Create Period 1 (Tháng 01/2026)
		pEnd := time.Date(2026, 1, 31, 0, 0, 0, 0, time.UTC)
		lock := time.Date(2026, 1, 31, 0, 0, 0, 0, time.UTC)
		p, err := spine.NewAccountingPeriod(periodID, fyID, companyID, 1, "Tháng 01/2026", start, pEnd, lock)
		require.NoError(t, err)
		err = spineRepo.SavePeriod(ctx, p)
		require.NoError(t, err)

		// 3. Query Period by Date (e.g. 2026-01-15)
		queryDate := time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC)
		fetchedP, err := spineRepo.GetPeriodByDate(ctx, companyID, queryDate)
		require.NoError(t, err)
		assert.Equal(t, 1, fetchedP.PeriodNumber)

		// 4. Update Period Lock
		newLock := time.Date(2026, 2, 10, 0, 0, 0, 0, time.UTC)
		err = spineRepo.UpdatePeriodLock(ctx, periodID, newLock, spine.PeriodStatusSoftLocked)
		require.NoError(t, err)
	})

	t.Run("Cost Center and Expense Item CRUD", func(t *testing.T) {
		ccID := "cc-int-01"
		eiID := "ei-int-01"

		_, _ = db.Exec("DELETE FROM cost_centers WHERE company_profile_id = ?", companyID)
		_, _ = db.Exec("DELETE FROM expense_items WHERE company_profile_id = ?", companyID)

		// Cost center
		cc, err := spine.NewCostCenter(ccID, companyID, nil, "TTP_KD", "Phòng Kinh Doanh", nil, true)
		require.NoError(t, err)
		err = spineRepo.SaveCostCenter(ctx, cc)
		require.NoError(t, err)

		fetchedCC, err := spineRepo.GetCostCenterByCode(ctx, companyID, "TTP_KD")
		require.NoError(t, err)
		assert.Equal(t, "TTP_KD", fetchedCC.Code)

		// Expense item
		ei, err := spine.NewExpenseItem(eiID, companyID, "CP_VPP", "Văn phòng phẩm", spine.ExpenseCategoryMaterial, nil, true)
		require.NoError(t, err)
		err = spineRepo.SaveExpenseItem(ctx, ei)
		require.NoError(t, err)

		fetchedEI, err := spineRepo.GetExpenseItemByCode(ctx, companyID, "CP_VPP")
		require.NoError(t, err)
		assert.Equal(t, "CP_VPP", fetchedEI.Code)
	})
}

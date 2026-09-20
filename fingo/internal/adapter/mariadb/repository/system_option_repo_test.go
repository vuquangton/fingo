package repository_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"fingo/internal/adapter/mariadb/repository"
	"fingo/internal/domain/system"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSystemOptionRepo_CRUD_And_Hierarchy(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	companyRepo := repository.NewCompanyProfileRepo(db)
	orgRepo := repository.NewBranchOrgUnitRepo(db)
	optRepo := repository.NewSystemOptionRepo(db)
	ctx := context.Background()

	// 1. Ensure company exists
	var companyID string
	activeComp, err := companyRepo.GetProfile(ctx)
	if err == nil && activeComp != nil {
		companyID = activeComp.ID
	} else {
		companyID = "test-cp-opt-01"
		comp, err := system.NewProductionCompanyProfile(system.CreateCompanyProfileParams{
			ID:                  companyID,
			TaxCode:             "0101243150",
			LegalName:           "CÔNG TY TEST OPTION REPO",
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

	// 2. Ensure test branch exists
	branchID := "test-branch-opt-01"
	_, _ = db.Exec("DELETE FROM branch_org_units WHERE id = ?", branchID)
	branch, err := system.NewBranchOrgUnit(system.CreateBranchOrgUnitParams{
		ID:                   branchID,
		CompanyProfileID:     companyID,
		ParentCompanyTaxCode: "0101243150",
		TaxCode:              "0101243150-001",
		Code:                 "CN-OPT-01",
		Name:                 "Chi nhánh Cấu hình",
		UnitType:             system.OrgUnitBranch,
		AccountingGovernance: system.GovDependent,
		TaxFilingMechanism:   system.TaxFilingCentralized,
		ProvinceCityCode:     "01",
		Address:              "123 Cầu Giấy, Hà Nội",
	})
	require.NoError(t, err)
	_ = orgRepo.CreateOrgUnit(ctx, branch)

	// 3. Upsert Company-Level Option
	compOpt := &system.SystemOption{
		ID:               "opt-comp-01",
		CompanyProfileID: companyID,
		BranchID:         nil,
		Category:         system.OptionCategoryCashBank,
		OptionKey:        "NON_CASH_PAYMENT_THRESHOLD",
		OptionValue:      "5000000",
		DataType:         system.DataTypeDecimal,
		DefaultValue:     "5000000",
		ScopeLevel:       system.ScopeCompany,
		Description:      "Ngưỡng thanh toán không dùng tiền mặt",
		UpdatedBy:        "test-user",
	}
	err = optRepo.UpsertOption(ctx, compOpt)
	require.NoError(t, err)

	// 4. Query Company-Level Option
	fetchedCompOpt, err := optRepo.GetOption(ctx, companyID, nil, "NON_CASH_PAYMENT_THRESHOLD")
	require.NoError(t, err)
	assert.Equal(t, "5000000", fetchedCompOpt.OptionValue)
	assert.Nil(t, fetchedCompOpt.BranchID)

	// 5. Effective Option without branch should yield company value
	effComp, err := optRepo.GetEffectiveOption(ctx, companyID, nil, "NON_CASH_PAYMENT_THRESHOLD")
	require.NoError(t, err)
	assert.Equal(t, "5000000", effComp.OptionValue)

	// 6. Upsert Branch-Level Override Option
	branchOpt := &system.SystemOption{
		ID:               "opt-branch-01",
		CompanyProfileID: companyID,
		BranchID:         &branchID,
		Category:         system.OptionCategoryCashBank,
		OptionKey:        "NON_CASH_PAYMENT_THRESHOLD",
		OptionValue:      "10000000",
		DataType:         system.DataTypeDecimal,
		DefaultValue:     "5000000",
		ScopeLevel:       system.ScopeBranch,
		Description:      "Ngưỡng thanh toán riêng của chi nhánh",
		UpdatedBy:        "test-user",
	}
	err = optRepo.UpsertOption(ctx, branchOpt)
	require.NoError(t, err)

	// 7. Effective Option with branchID must return Branch Override (10000000)
	effBranch, err := optRepo.GetEffectiveOption(ctx, companyID, &branchID, "NON_CASH_PAYMENT_THRESHOLD")
	require.NoError(t, err)
	assert.Equal(t, "10000000", effBranch.OptionValue)
	require.NotNil(t, effBranch.BranchID)
	assert.Equal(t, branchID, *effBranch.BranchID)

	// 8. Record and List Audit History
	_, _ = db.Exec("DELETE FROM system_config_history WHERE company_profile_id = ?", companyID)
	oldVal := "5000000"
	reason := "Nghị định 181 điều chỉnh ngưỡng"
	clientIP := "192.168.1.50"
	hist := &system.SystemConfigHistory{
		ID:               "hist-opt-01",
		CompanyProfileID: companyID,
		OptionKey:        "NON_CASH_PAYMENT_THRESHOLD",
		OldValue:         &oldVal,
		NewValue:         "10000000",
		ChangedBy:        "test-user",
		Reason:           &reason,
		ClientIP:         &clientIP,
	}
	err = optRepo.RecordHistory(ctx, hist)
	require.NoError(t, err)

	historyList, err := optRepo.ListHistory(ctx, companyID, "NON_CASH_PAYMENT_THRESHOLD", 10)
	require.NoError(t, err)
	assert.NotEmpty(t, historyList)
	assert.Equal(t, "10000000", historyList[0].NewValue)
}

func TestVoucherNumberingRepo_Concurrent_NextSequence(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	companyRepo := repository.NewCompanyProfileRepo(db)
	voucherRepo := repository.NewVoucherNumberingRepo(db)
	ctx := context.Background()

	var companyID string
	activeComp, err := companyRepo.GetProfile(ctx)
	if err == nil && activeComp != nil {
		companyID = activeComp.ID
	} else {
		companyID = "test-cp-vouch-01"
		comp, err := system.NewProductionCompanyProfile(system.CreateCompanyProfileParams{
			ID:                  companyID,
			TaxCode:             "0101243150",
			LegalName:           "CÔNG TY TEST VOUCHER REPO",
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

	// Seed voucher numbering configuration
	cfgID := "cfg-pt-test-01"
	voucherType := "CASH_RECEIPT"
	initialDate := time.Date(2026, 3, 20, 0, 0, 0, 0, time.UTC)

	cfg := &system.VoucherNumberingConfig{
		ID:               cfgID,
		CompanyProfileID: companyID,
		BranchID:         nil,
		VoucherType:      voucherType,
		Prefix:           "PT",
		Pattern:          "{PREFIX}-{YYYY}{MM}-{SEQUENCE:05}",
		ResetFrequency:   system.ResetFrequencyMonthly,
		CurrentSequence:  0,
		LastResetDate:    initialDate,
	}
	err = voucherRepo.UpsertConfig(ctx, cfg)
	require.NoError(t, err)

	// Test 1: Sequential generation
	num1, updatedCfg, err := voucherRepo.NextSequence(ctx, companyID, nil, voucherType, initialDate)
	require.NoError(t, err)
	assert.Equal(t, "PT-202603-00001", num1)
	assert.Equal(t, int64(1), updatedCfg.CurrentSequence)

	num2, updatedCfg2, err := voucherRepo.NextSequence(ctx, companyID, nil, voucherType, initialDate)
	require.NoError(t, err)
	assert.Equal(t, "PT-202603-00002", num2)
	assert.Equal(t, int64(2), updatedCfg2.CurrentSequence)

	// Test 2: Concurrency Test (Pessimistic locking ensures strictly unique, gapless sequences)
	concurrentCount := 10
	var wg sync.WaitGroup
	var mu sync.Mutex
	generatedNumbers := make([]string, 0, concurrentCount)

	wg.Add(concurrentCount)
	for i := 0; i < concurrentCount; i++ {
		go func() {
			defer wg.Done()
			num, _, seqErr := voucherRepo.NextSequence(ctx, companyID, nil, voucherType, initialDate)
			if seqErr == nil {
				mu.Lock()
				generatedNumbers = append(generatedNumbers, num)
				mu.Unlock()
			}
		}()
	}
	wg.Wait()

	assert.Equal(t, concurrentCount, len(generatedNumbers))

	// Verify all generated numbers are distinct (no collision)
	uniqueMap := make(map[string]bool)
	for _, n := range generatedNumbers {
		uniqueMap[n] = true
	}
	assert.Equal(t, len(generatedNumbers), len(uniqueMap), "Voucher numbers generated concurrently must be unique")
}

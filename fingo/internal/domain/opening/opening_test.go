package opening_test

import (
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"fingo/internal/domain/opening"
)

func TestOpeningBatch_Equilibrium(t *testing.T) {
	t.Parallel()

	asOfDate := time.Date(2025, 12, 31, 0, 0, 0, 0, time.UTC)

	t.Run("Valid balanced trial balance satisfies INV-OPEN-01", func(t *testing.T) {
		batch := opening.NewOpeningBatch("b-1", "comp-1", asOfDate, "Cutover 2026")
		batch.Accounts = []opening.AccountOpeningBalance{
			{AccountCode: "1111", DebitAmountVND: decimal.RequireFromString("100000000"), CreditAmountVND: decimal.Zero},
			{AccountCode: "1121", DebitAmountVND: decimal.RequireFromString("200000000"), CreditAmountVND: decimal.Zero},
			{AccountCode: "4111", DebitAmountVND: decimal.Zero, CreditAmountVND: decimal.RequireFromString("300000000")},
		}

		err := batch.ValidateEquilibrium()
		require.NoError(t, err)
		assert.True(t, batch.TotalDebit.Equal(decimal.RequireFromString("300000000")))
		assert.True(t, batch.TotalCredit.Equal(decimal.RequireFromString("300000000")))
	})

	t.Run("Unbalanced trial balance returns ErrTrialBalanceUnbalanced", func(t *testing.T) {
		batch := opening.NewOpeningBatch("b-2", "comp-1", asOfDate, "Unbalanced")
		batch.Accounts = []opening.AccountOpeningBalance{
			{AccountCode: "1111", DebitAmountVND: decimal.RequireFromString("100000000"), CreditAmountVND: decimal.Zero},
			{AccountCode: "4111", DebitAmountVND: decimal.Zero, CreditAmountVND: decimal.RequireFromString("90000000")},
		}

		err := batch.ValidateEquilibrium()
		require.ErrorIs(t, err, opening.ErrTrialBalanceUnbalanced)
	})

	t.Run("P&L nominal account with balance returns ErrInvalidOpeningAccount", func(t *testing.T) {
		batch := opening.NewOpeningBatch("b-3", "comp-1", asOfDate, "P&L illegal")
		batch.Accounts = []opening.AccountOpeningBalance{
			{AccountCode: "1111", DebitAmountVND: decimal.RequireFromString("50000000"), CreditAmountVND: decimal.Zero},
			{AccountCode: "5111", DebitAmountVND: decimal.Zero, CreditAmountVND: decimal.RequireFromString("50000000")},
		}

		err := batch.ValidateEquilibrium()
		require.ErrorIs(t, err, opening.ErrInvalidOpeningAccount)

		// Expense account 642
		batch.Accounts = []opening.AccountOpeningBalance{
			{AccountCode: "6421", DebitAmountVND: decimal.RequireFromString("50000000"), CreditAmountVND: decimal.Zero},
			{AccountCode: "4111", DebitAmountVND: decimal.Zero, CreditAmountVND: decimal.RequireFromString("50000000")},
		}
		err = batch.ValidateEquilibrium()
		require.ErrorIs(t, err, opening.ErrInvalidOpeningAccount)
	})
}

func TestOpeningBatch_FullReconciliation(t *testing.T) {
	t.Parallel()

	asOfDate := time.Date(2025, 12, 31, 0, 0, 0, 0, time.UTC)

	createReconciledBatch := func() *opening.OpeningBatch {
		b := opening.NewOpeningBatch("batch-rec", "comp-1", asOfDate, "Full cutover")

		// GL Accounts
		b.Accounts = []opening.AccountOpeningBalance{
			{AccountCode: "1111", DebitAmountVND: decimal.RequireFromString("100000000"), CreditAmountVND: decimal.Zero},
			{AccountCode: "1311", DebitAmountVND: decimal.RequireFromString("250000000"), CreditAmountVND: decimal.RequireFromString("20000000")},
			{AccountCode: "1561", DebitAmountVND: decimal.RequireFromString("400000000"), CreditAmountVND: decimal.Zero},
			{AccountCode: "2111", DebitAmountVND: decimal.RequireFromString("1000000000"), CreditAmountVND: decimal.Zero},
			{AccountCode: "2141", DebitAmountVND: decimal.Zero, CreditAmountVND: decimal.RequireFromString("300000000")},
			{AccountCode: "3311", DebitAmountVND: decimal.RequireFromString("15000000"), CreditAmountVND: decimal.RequireFromString("180000000")},
			{AccountCode: "4111", DebitAmountVND: decimal.Zero, CreditAmountVND: decimal.RequireFromString("1265000000")},
		}

		// Customer Subledger (Matches TK 131: Dr 250M, Cr 20M)
		b.Customers = []opening.CustomerOpeningBalance{
			{CustomerCode: "KH001", DebitAmountVND: decimal.RequireFromString("250000000"), CreditAmountVND: decimal.Zero},
			{CustomerCode: "KH002", DebitAmountVND: decimal.Zero, CreditAmountVND: decimal.RequireFromString("20000000")},
		}

		// Vendor Subledger (Matches TK 331: Dr 15M, Cr 180M)
		b.Vendors = []opening.VendorOpeningBalance{
			{VendorCode: "NCC001", DebitAmountVND: decimal.Zero, CreditAmountVND: decimal.RequireFromString("180000000")},
			{VendorCode: "NCC002", DebitAmountVND: decimal.RequireFromString("15000000"), CreditAmountVND: decimal.Zero},
		}

		// Inventory Subledger (Matches TK 1561: 400M)
		b.Inventory = []opening.InventoryOpeningBalance{
			{
				WarehouseCode:  "KHO_TONG",
				ItemCode:       "ITEM_01",
				Quantity:       decimal.RequireFromString("1000"),
				UnitCost:       decimal.RequireFromString("400000"),
				TotalAmountVND: decimal.RequireFromString("400000000"),
			},
		}

		// Fixed Asset Subledger (Matches TK 211: 1,000M; TK 214: 300M)
		b.Assets = []opening.AssetOpeningBalance{
			{
				AssetCode:               "TS001",
				AssetName:               "Xe tải Isuzu",
				OriginalCost:            decimal.RequireFromString("1000000000"),
				AccumulatedDepreciation: decimal.RequireFromString("300000000"),
				NetBookValue:            decimal.RequireFromString("700000000"),
				UsefulLifeMonths:        60,
				RemainingLifeMonths:     42,
				MonthlyDepreciation:     decimal.RequireFromString("16666667"),
			},
		}

		return b
	}

	t.Run("Reconciliation passes when all subledgers match GL exactly", func(t *testing.T) {
		batch := createReconciledBatch()
		report, err := batch.Reconcile()
		require.NoError(t, err)
		assert.True(t, report.AllReconciled)
		assert.True(t, report.TrialBalanceBalanced)
		assert.True(t, report.CustomerARReconciled)
		assert.True(t, report.VendorAPReconciled)
		assert.True(t, report.InventoryReconciled)
		assert.True(t, report.FixedAssetsReconciled)
		assert.Equal(t, opening.BatchStatusValidated, batch.Status)
	})

	t.Run("Reconciliation fails when Customer AR subledger mismatches TK 131", func(t *testing.T) {
		batch := createReconciledBatch()
		// Corrupt customer balance
		batch.Customers[0].DebitAmountVND = decimal.RequireFromString("240000000") // 10M mismatch

		report, err := batch.Reconcile()
		require.ErrorIs(t, err, opening.ErrSubledgerReconciliationFailed)
		assert.False(t, report.AllReconciled)
		assert.False(t, report.CustomerARReconciled)
	})

	t.Run("Reconciliation fails when Vendor AP subledger mismatches TK 331", func(t *testing.T) {
		batch := createReconciledBatch()
		batch.Vendors[0].CreditAmountVND = decimal.RequireFromString("170000000") // 10M mismatch

		report, err := batch.Reconcile()
		require.ErrorIs(t, err, opening.ErrSubledgerReconciliationFailed)
		assert.False(t, report.VendorAPReconciled)
	})

	t.Run("Reconciliation fails when Inventory subledger mismatches TK 15x", func(t *testing.T) {
		batch := createReconciledBatch()
		batch.Inventory[0].TotalAmountVND = decimal.RequireFromString("390000000") // 10M mismatch

		report, err := batch.Reconcile()
		require.ErrorIs(t, err, opening.ErrSubledgerReconciliationFailed)
		assert.False(t, report.InventoryReconciled)
	})

	t.Run("Reconciliation fails when Fixed Asset schedule mismatches TK 211 or 214", func(t *testing.T) {
		batch := createReconciledBatch()
		batch.Assets[0].OriginalCost = decimal.RequireFromString("900000000") // 100M mismatch

		report, err := batch.Reconcile()
		require.ErrorIs(t, err, opening.ErrSubledgerReconciliationFailed)
		assert.False(t, report.FixedAssetsReconciled)
	})

	t.Run("Reconciliation flags invalid inventory quantity or asset useful life", func(t *testing.T) {
		batch := createReconciledBatch()
		batch.Inventory[0].Quantity = decimal.Zero
		batch.Assets[0].UsefulLifeMonths = 0

		report, err := batch.Reconcile()
		require.Error(t, err)
		assert.False(t, report.AllReconciled)
	})
}

func TestOpeningBatch_Commit_And_Lock(t *testing.T) {
	t.Parallel()

	asOfDate := time.Date(2025, 12, 31, 0, 0, 0, 0, time.UTC)

	t.Run("Commit successfully locks batch and sets timestamp and user", func(t *testing.T) {
		b := opening.NewOpeningBatch("batch-rec", "comp-1", asOfDate, "Full cutover")
		b.Accounts = []opening.AccountOpeningBalance{
			{AccountCode: "1111", DebitAmountVND: decimal.RequireFromString("100000000")},
			{AccountCode: "4111", CreditAmountVND: decimal.RequireFromString("100000000")},
		}

		err := b.Commit("chief-accountant-user")
		require.NoError(t, err)
		assert.Equal(t, opening.BatchStatusCommitted, b.Status)
		require.NotNil(t, b.CommittedAt)
		require.NotNil(t, b.CommittedBy)
		assert.Equal(t, "chief-accountant-user", *b.CommittedBy)

		// Re-commit fails
		err = b.Commit("chief-accountant-user")
		require.ErrorIs(t, err, opening.ErrBatchAlreadyCommitted)

		// Lock committed batch succeeds
		err = b.Lock()
		require.NoError(t, err)
		assert.Equal(t, opening.BatchStatusLocked, b.Status)
	})

	t.Run("Commit fails if reconciliation fails", func(t *testing.T) {
		b := opening.NewOpeningBatch("batch-err", "comp-1", asOfDate, "Unbalanced")
		b.Accounts = []opening.AccountOpeningBalance{
			{AccountCode: "1111", DebitAmountVND: decimal.RequireFromString("100000000")},
			{AccountCode: "4111", CreditAmountVND: decimal.RequireFromString("90000000")},
		}

		err := b.Commit("user-1")
		require.ErrorIs(t, err, opening.ErrBatchNotValidated)
	})

	t.Run("Lock fails if batch is not committed", func(t *testing.T) {
		b := opening.NewOpeningBatch("batch-draft", "comp-1", asOfDate, "Draft")
		err := b.Lock()
		require.Error(t, err)
	})
}

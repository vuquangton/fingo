package repository_test

import (
	"context"
	"database/sql"
	"testing"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"fingo/internal/adapter/mariadb/repository"
	"fingo/internal/domain/gl"
)

func TestVoucherRepo_LiveMariaDB(t *testing.T) {
	dsn := "dev:123456@tcp(127.0.0.1:3306)/fingo?parseTime=true"
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Skipf("skipping live MariaDB test: %v", err)
		return
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		t.Skipf("skipping live MariaDB test (cannot ping): %v", err)
		return
	}

	ctx := context.Background()
	now := time.Now()

	// 1. Reuse existing company
	var companyID string
	err = db.QueryRowContext(ctx, "SELECT id FROM company_profile WHERE is_active = TRUE LIMIT 1").Scan(&companyID)
	if err != nil || companyID == "" {
		companyID = "comp-voucher-test"
		_, err = db.ExecContext(ctx, "INSERT INTO company_profile (id, tax_code, legal_name, address, legal_representative, chief_accountant, tax_authority_code, tax_authority_name, created_at, updated_at) VALUES (?, '0101243150', 'FinGo Voucher Test', 'Hanoi', 'GD', 'KTT', '101', 'CCT', ?, ?)", companyID, now, now)
		require.NoError(t, err)
	}

	getAccountID := func(code, name, nature, category string) string {
		var id string
		err := db.QueryRowContext(ctx, "SELECT id FROM accounts WHERE company_profile_id = ? AND code = ?", companyID, code).Scan(&id)
		if err == nil && id != "" {
			return id
		}
		id = "acc-vr-" + code
		_, _ = db.ExecContext(ctx, "INSERT INTO accounts (id, company_profile_id, code, name, nature, category, is_leaf, is_active, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, 1, 1, NOW(), NOW())", id, companyID, code, name, nature, category)
		return id
	}

	acc6421ID := getAccountID("6421", "Chi phi ban hang", "DEBIT", "EXPENSE")
	acc2141ID := getAccountID("2141", "Hao mon TSCD", "CREDIT", "ASSET")

	// 3. Ensure test cost center exists
	var ccID string
	err = db.QueryRowContext(ctx, "SELECT id FROM cost_centers WHERE company_profile_id = ? AND code = 'CC_SALES'", companyID).Scan(&ccID)
	if err != nil || ccID == "" {
		ccID = "cc-vr-sales"
		_, _ = db.ExecContext(ctx, "INSERT INTO cost_centers (id, company_profile_id, code, name, is_active, created_at, updated_at) VALUES (?, ?, 'CC_SALES', 'Chi phi BH', 1, ?, ?)", ccID, companyID, now, now)
	}

	repo := repository.NewVoucherRepo(db)
	vDate := time.Date(2026, 3, 20, 0, 0, 0, 0, time.UTC)
	vID := "v-live-test-" + time.Now().Format("150405")
	vNo := "PKT-" + time.Now().Format("150405")

	// Cleanup on finish
	defer func() {
		_, _ = db.ExecContext(ctx, "DELETE FROM vouchers WHERE id = ?", vID)
	}()

	t.Run("Create and Read Voucher with Lines", func(t *testing.T) {
		v, err := gl.NewVoucher(gl.CreateVoucherParams{
			ID:               vID,
			CompanyProfileID: companyID,
			VoucherNo:        vNo,
			VoucherDate:      vDate,
			VoucherType:      gl.VoucherTypeGeneral,
			Description:      "Live test general voucher",
			CreatedBy:        "ktt_test",
			Lines: []gl.VoucherLine{
				{
					DebitAccountID:    acc6421ID,
					CreditAccountID:   acc2141ID,
					DebitAccountCode:  "6421",
					CreditAccountCode: "2141",
					AmountVND:         decimal.NewFromInt(12500000),
					CostCenterID:      &ccID,
					Note:              "Khau hao xe ban hang",
				},
			},
		})
		require.NoError(t, err)

		// Save to DB
		err = repo.SaveVoucher(ctx, v)
		require.NoError(t, err)

		// Fetch by ID
		fetched, err := repo.GetVoucherByID(ctx, vID)
		require.NoError(t, err)
		assert.Equal(t, vID, fetched.ID)
		assert.Equal(t, vNo, fetched.VoucherNo)
		assert.Equal(t, gl.VoucherStatusDraft, fetched.Status)
		assert.Equal(t, "12500000", fetched.TotalDebit.String())
		assert.Equal(t, "12500000", fetched.TotalCredit.String())
		require.Len(t, fetched.Lines, 1)
		assert.Equal(t, "6421", fetched.Lines[0].DebitAccountCode)
		assert.Equal(t, "2141", fetched.Lines[0].CreditAccountCode)
		assert.Equal(t, "12500000", fetched.Lines[0].AmountVND.String())
		assert.Equal(t, ccID, *fetched.Lines[0].CostCenterID)

		// Fetch by No
		byNo, err := repo.GetVoucherByNo(ctx, companyID, gl.VoucherTypeGeneral, vNo)
		require.NoError(t, err)
		assert.Equal(t, vID, byNo.ID)

		// Update status to POSTED
		err = repo.UpdateVoucherStatus(ctx, vID, gl.VoucherStatusPosted)
		require.NoError(t, err)

		updated, err := repo.GetVoucherByID(ctx, vID)
		require.NoError(t, err)
		assert.Equal(t, gl.VoucherStatusPosted, updated.Status)

		// List by period
		list, err := repo.ListVouchersByPeriod(ctx, companyID, vDate.AddDate(0, 0, -1), vDate.AddDate(0, 0, 1))
		require.NoError(t, err)
		assert.NotEmpty(t, list)
	})
}

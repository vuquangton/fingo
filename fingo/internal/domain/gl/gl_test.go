package gl_test

import (
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"fingo/internal/domain/gl"
)

func ptr(s string) *string {
	return &s
}

func TestVoucher_DomainInvariants(t *testing.T) {
	vDate := time.Date(2026, 3, 20, 0, 0, 0, 0, time.UTC)
	costCenter := "CC_SALES"

	t.Run("Create valid voucher with balanced lines", func(t *testing.T) {
		v, err := gl.NewVoucher(gl.CreateVoucherParams{
			ID:               "v-001",
			CompanyProfileID: "comp-01",
			VoucherNo:        "PKT-2026-0001",
			VoucherDate:      vDate,
			VoucherType:      gl.VoucherTypeGeneral,
			Description:      "Trích khấu hao TSCĐ tháng 3",
			CreatedBy:        "ktt_test",
			Lines: []gl.VoucherLine{
				{
					DebitAccountID:    "acc-642",
					CreditAccountID:   "acc-214",
					DebitAccountCode:  "6421",
					CreditAccountCode: "2141",
					AmountVND:         decimal.NewFromInt(15000000),
					CostCenterID:      &costCenter,
					Note:              "Khấu hao bộ phận bán hàng",
				},
			},
		})

		require.NoError(t, err)
		assert.Equal(t, gl.VoucherStatusDraft, v.Status)
		assert.Equal(t, "15000000", v.TotalDebit.String())
		assert.Equal(t, "15000000", v.TotalCredit.String())
		require.NoError(t, v.ValidateBalance())

		// Test Post
		err = v.Post()
		require.NoError(t, err)
		assert.Equal(t, gl.VoucherStatusPosted, v.Status)

		// Cannot post again
		err = v.Post()
		require.ErrorIs(t, err, gl.ErrVoucherAlreadyPosted)
	})

	t.Run("Reject missing mandatory fields", func(t *testing.T) {
		_, err := gl.NewVoucher(gl.CreateVoucherParams{
			CompanyProfileID: "",
			VoucherNo:        "PKT-01",
			VoucherDate:      vDate,
		})
		require.Error(t, err)

		_, err = gl.NewVoucher(gl.CreateVoucherParams{
			CompanyProfileID: "comp-01",
			VoucherNo:        "",
			VoucherDate:      vDate,
		})
		require.Error(t, err)

		_, err = gl.NewVoucher(gl.CreateVoucherParams{
			CompanyProfileID: "comp-01",
			VoucherNo:        "PKT-01",
			VoucherDate:      time.Time{},
		})
		require.Error(t, err)
	})

	t.Run("Reject zero amount line", func(t *testing.T) {
		v, err := gl.NewVoucher(gl.CreateVoucherParams{
			ID:               "v-002",
			CompanyProfileID: "comp-01",
			VoucherNo:        "PKT-0002",
			VoucherDate:      vDate,
		})
		require.NoError(t, err)

		err = v.AddLine(gl.VoucherLine{
			DebitAccountID:    "acc-111",
			CreditAccountID:   "acc-112",
			DebitAccountCode:  "1111",
			CreditAccountCode: "1121",
			AmountVND:         decimal.Zero,
		})
		require.ErrorIs(t, err, gl.ErrZeroVoucherAmount)
	})

	t.Run("Reject identical debit and credit accounts", func(t *testing.T) {
		v, err := gl.NewVoucher(gl.CreateVoucherParams{
			ID:               "v-003",
			CompanyProfileID: "comp-01",
			VoucherNo:        "PKT-0003",
			VoucherDate:      vDate,
		})
		require.NoError(t, err)

		err = v.AddLine(gl.VoucherLine{
			DebitAccountID:    "acc-111",
			CreditAccountID:   "acc-111",
			DebitAccountCode:  "1111",
			CreditAccountCode: "1111",
			AmountVND:         decimal.NewFromInt(1000000),
		})
		require.ErrorIs(t, err, gl.ErrSameDebitCreditAccount)
	})

	t.Run("Reject missing account ID", func(t *testing.T) {
		v, err := gl.NewVoucher(gl.CreateVoucherParams{
			ID:               "v-004",
			CompanyProfileID: "comp-01",
			VoucherNo:        "PKT-0004",
			VoucherDate:      vDate,
		})
		require.NoError(t, err)

		err = v.AddLine(gl.VoucherLine{
			DebitAccountID:    "",
			CreditAccountID:   "acc-111",
			DebitAccountCode:  "",
			CreditAccountCode: "1111",
			AmountVND:         decimal.NewFromInt(1000000),
		})
		require.ErrorIs(t, err, gl.ErrAccountRequired)
	})

	t.Run("INV-OPS-03: Cost center mandatory for cost/expense accounts", func(t *testing.T) {
		v, err := gl.NewVoucher(gl.CreateVoucherParams{
			ID:               "v-005",
			CompanyProfileID: "comp-01",
			VoucherNo:        "PKT-0005",
			VoucherDate:      vDate,
		})
		require.NoError(t, err)

		// 642 without cost center -> rejected
		err = v.AddLine(gl.VoucherLine{
			DebitAccountID:    "acc-642",
			CreditAccountID:   "acc-111",
			DebitAccountCode:  "6422",
			CreditAccountCode: "1111",
			AmountVND:         decimal.NewFromInt(5000000),
			CostCenterID:      nil,
		})
		require.ErrorIs(t, err, gl.ErrCostCenterRequired)

		// 642 with cost center -> accepted
		err = v.AddLine(gl.VoucherLine{
			DebitAccountID:    "acc-642",
			CreditAccountID:   "acc-111",
			DebitAccountCode:  "6422",
			CreditAccountCode: "1111",
			AmountVND:         decimal.NewFromInt(5000000),
			CostCenterID:      &costCenter,
		})
		require.NoError(t, err)
	})

	t.Run("Reject posting with empty lines", func(t *testing.T) {
		v, err := gl.NewVoucher(gl.CreateVoucherParams{
			ID:               "v-006",
			CompanyProfileID: "comp-01",
			VoucherNo:        "PKT-0006",
			VoucherDate:      vDate,
		})
		require.NoError(t, err)
		err = v.Post()
		require.ErrorIs(t, err, gl.ErrVoucherEmptyLines)
	})

	t.Run("Cancel lifecycle transitions", func(t *testing.T) {
		v, err := gl.NewVoucher(gl.CreateVoucherParams{
			ID:               "v-007",
			CompanyProfileID: "comp-01",
			VoucherNo:        "PKT-0007",
			VoucherDate:      vDate,
			Lines: []gl.VoucherLine{
				{
					DebitAccountID:    "acc-111",
					CreditAccountID:   "acc-112",
					DebitAccountCode:  "1111",
					CreditAccountCode: "1121",
					AmountVND:         decimal.NewFromInt(2000000),
				},
			},
		})
		require.NoError(t, err)

		err = v.Cancel()
		require.NoError(t, err)
		assert.Equal(t, gl.VoucherStatusCancelled, v.Status)

		// Cannot cancel again
		err = v.Cancel()
		require.ErrorIs(t, err, gl.ErrVoucherAlreadyCancelled)

		// Cannot post cancelled voucher
		err = v.Post()
		require.ErrorIs(t, err, gl.ErrVoucherAlreadyCancelled)
	})

	t.Run("Storno reversing voucher generation", func(t *testing.T) {
		v, err := gl.NewVoucher(gl.CreateVoucherParams{
			ID:               "v-008",
			CompanyProfileID: "comp-01",
			VoucherNo:        "PKT-0008",
			VoucherDate:      vDate,
			Lines: []gl.VoucherLine{
				{
					DebitAccountID:    "acc-111",
					CreditAccountID:   "acc-131",
					DebitAccountCode:  "1111",
					CreditAccountCode: "1311",
					AmountFC:          decimal.NewFromInt(100),
					AmountVND:         decimal.NewFromInt(2500000),
					Note:              "Thu tiền nhầm",
				},
			},
		})
		require.NoError(t, err)
		require.NoError(t, v.Post())

		revDate := time.Date(2026, 3, 21, 0, 0, 0, 0, time.UTC)
		reversal, err := v.CreateReversingVoucher("v-008-rev", "PKT-0008-REV", revDate, "ktt_lead")
		require.NoError(t, err)

		assert.Equal(t, "v-008-rev", reversal.ID)
		assert.Equal(t, "PKT-0008-REV", reversal.VoucherNo)
		assert.Equal(t, gl.VoucherStatusDraft, reversal.Status)
		require.Len(t, reversal.Lines, 1)

		// Storno negative amounts
		assert.Equal(t, "-2500000", reversal.Lines[0].AmountVND.String())
		assert.Equal(t, "-100", reversal.Lines[0].AmountFC.String())
		assert.Contains(t, reversal.Lines[0].Note, "Reversal of PKT-0008")
		assert.Equal(t, "-2500000", reversal.TotalDebit.String())
		assert.Equal(t, "-2500000", reversal.TotalCredit.String())
	})
}

package reports

import (
	"testing"
	"time"

	"fingo/internal/domain/gl"
	"fingo/internal/money"
)

// What the client owns equals what it owes together with what is left for its
// owners — at the end of a closed period, where the result has been carried.
func TestBalanceSheet_Balances(t *testing.T) {
	f := newFixture(t)
	sheet := f.balanceSheet(t, january.ID)

	if !sheet.Balanced() {
		t.Errorf("assets %s, liabilities %s, equity %s: %s != %s",
			sheet.TotalAssets, sheet.TotalLiabilities, sheet.TotalEquity,
			sheet.TotalAssets, sheet.TotalLiabilities.Add(sheet.TotalEquity))
	}

	if want := money.FromInt(7_400_000); !sheet.TotalAssets.Equal(want) {
		t.Errorf("total assets = %s, want %s", sheet.TotalAssets, want)
	}
	if want := money.FromInt(2_000_000); !sheet.TotalLiabilities.Equal(want) {
		t.Errorf("total liabilities = %s, want %s", sheet.TotalLiabilities, want)
	}
	if want := money.FromInt(5_400_000); !sheet.TotalEquity.Equal(want) {
		t.Errorf("total equity = %s, want %s", sheet.TotalEquity, want)
	}

	// A closed period has carried its result: nothing is left standing in the
	// profit and loss accounts, and the same figure sits in equity.
	if !sheet.Result.IsZero() {
		t.Errorf("a closed period still reports an unclosed result of %s", sheet.Result)
	}
}

// A balance sheet is a position at an instant, so it carries every line up to
// the period's end and not only the period's own movements.
func TestBalanceSheet_CarriesEveryLineUpToTheDate(t *testing.T) {
	f := newFixture(t)
	sheet := f.balanceSheet(t, january.ID)

	cases := map[string]struct {
		section []BalanceSheetRow
		want    int64
	}{
		"cash paid in before the period": {sheet.Assets, 5_000_000},
		"stock bought and partly sold":   {sheet.Assets, 1_400_000},
		"a receivable raised":            {sheet.Assets, 1_000_000},
		"a supplier owed":                {sheet.Liabilities, 2_000_000},
		"capital paid in":                {sheet.Equity, 5_000_000},
		"the year's profit":              {sheet.Equity, 400_000},
	}

	codes := map[string]string{
		"cash paid in before the period": "111",
		"stock bought and partly sold":   "156",
		"a receivable raised":            "131",
		"a supplier owed":                "331",
		"capital paid in":                "4111",
		"the year's profit":              "4212",
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			row := sheetRow(t, tc.section, codes[name])
			if want := money.FromInt(tc.want); !row.Amount.Equal(want) {
				t.Errorf("%s = %s, want %s", codes[name], row.Amount, want)
			}
		})
	}
}

// A period nobody has closed yet still balances: what has been earned and not
// carried sits in equity, because that is where it belongs until a close moves
// it.
func TestBalanceSheet_CarriesAnUnclosedResultIntoEquity(t *testing.T) {
	f := newFixture(t)
	sheet := f.balanceSheet(t, february.ID)

	if want := money.FromInt(500_000); !sheet.Result.Equal(want) {
		t.Errorf("unclosed result = %s, want %s", sheet.Result, want)
	}
	if !sheet.Balanced() {
		t.Errorf("assets %s, liabilities %s, equity %s (including the result)",
			sheet.TotalAssets, sheet.TotalLiabilities, sheet.TotalEquity)
	}

	// Equity is the owners' capital, the profit carried at the close, and the
	// profit still standing in the profit and loss accounts.
	want := money.FromInt(5_000_000 + 400_000 + 500_000)
	if !sheet.TotalEquity.Equal(want) {
		t.Errorf("total equity = %s, want %s", sheet.TotalEquity, want)
	}
}

// A contra-asset carries its balance on the side an asset normally does not —
// accumulated depreciation is the ordinary one — and it is still an asset. The
// report classifies by what the chart says the account is, not by which side the
// figure landed on.
func TestBalanceSheet_HoldsAContraAssetAsANegativeAsset(t *testing.T) {
	f := newFixture(t)

	f.journal.lines = append(f.journal.lines,
		Line{
			VoucherID:   "v-depreciation",
			VoucherDate: time.Date(2026, 1, 25, 0, 0, 0, 0, time.UTC),
			AccountCode: "6422",
			Side:        gl.SideDebit,
			Amount:      money.FromInt(100_000),
		},
		Line{
			VoucherID:   "v-depreciation",
			VoucherDate: time.Date(2026, 1, 25, 0, 0, 0, 0, time.UTC),
			AccountCode: "2141",
			Side:        gl.SideCredit,
			Amount:      money.FromInt(100_000),
		},
	)

	sheet := f.balanceSheet(t, january.ID)

	accumulated := sheetRow(t, sheet.Assets, "2141")
	if want := money.FromInt(-100_000); !accumulated.Amount.Equal(want) {
		t.Errorf("2141 = %s, want %s: a credit balance on an asset account", accumulated.Amount, want)
	}
	for _, row := range sheet.Liabilities {
		if row.AccountCode == "2141" {
			t.Error("depreciation accumulated on an asset is reported as something the client owes")
		}
	}

	// And the identity still holds with a negative asset in the section.
	if !sheet.Balanced() {
		t.Errorf("assets %s, liabilities %s, equity %s",
			sheet.TotalAssets, sheet.TotalLiabilities, sheet.TotalEquity)
	}
}

// A balance sheet holds what a client owns, owes and is owed for. The accounts
// it earned and spent through are not on it: their net is stated once, as the
// result.
func TestBalanceSheet_LeavesTheProfitAndLossAccountsOff(t *testing.T) {
	f := newFixture(t)
	sheet := f.balanceSheet(t, february.ID)

	for _, section := range [][]BalanceSheetRow{sheet.Assets, sheet.Liabilities, sheet.Equity} {
		for _, row := range section {
			switch row.AccountCode {
			case "5111", "632", "911":
				t.Errorf("%s is a profit and loss account and is a row of the balance sheet", row.AccountCode)
			}
			if row.SourceVoucherCount == 0 {
				t.Errorf("%s has no voucher behind it", row.AccountCode)
			}
		}
	}
}

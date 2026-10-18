package gl_test

import (
	"testing"

	"fingo/internal/domain/gl"
)

// The TT133 chart of accounts is 49 accounts, taken from Phụ lục 1 of Circular
// 133/2016/TT-BTC — read from the official Công báo text-layer PDF (Công báo
// số 1103+1104 ngày 12-10-2016, printed pages 77-82), not from a vendor copy.
//
// Vendor and aggregator copies of this appendix contain cell-merge typos: they
// print "5111 5112 5111 5118", "4111 4111 4118" and "2141 2142 2141 2147". The
// Công báo gives 5111/5112/5113/5118, 4111/4112/4118, 2141/2142/2143/2147.
// These tests exist to hold the official values.
//
// Circular 46/2025/TT-BTC did NOT change Phụ lục 1. It replaced Điều 10, making
// the chart self-modifiable by the enterprise instead of requiring Ministry
// approval. No account was added, renamed or removed.
func TestTT133Chart_IsALegalChartOf49Accounts(t *testing.T) {
	accounts := gl.TT133Chart()

	coa := gl.NewChartOfAccounts()
	for _, a := range accounts {
		if err := coa.Add(a); err != nil {
			t.Fatalf("seed account %q is malformed: %v", a.Code, err)
		}
	}
	if err := coa.Validate(); err != nil {
		t.Fatalf("the seed chart is not a legal chart: %v", err)
	}

	var topLevel int
	for _, a := range accounts {
		if a.ParentCode == "" {
			topLevel++
		}
	}
	if topLevel != 49 {
		t.Fatalf("cấp-1 accounts = %d, want 49 (Phụ lục 1, Số TT 01-49)", topLevel)
	}
}

// The accounts a trading SME actually posts to. If any of these is missing the
// trading cycle cannot be recorded at all.
func TestTT133Chart_HasTheTradingAccounts(t *testing.T) {
	coa := gl.NewChartOfAccounts()
	for _, a := range gl.TT133Chart() {
		if err := coa.Add(a); err != nil {
			t.Fatalf("seed account %q is malformed: %v", a.Code, err)
		}
	}

	want := []string{
		"111", "1111", "1112", // cash
		"112", "1121", // bank
		"131",                 // receivable
		"133", "1331", "1332", // deductible input VAT
		"156",                  // goods
		"331",                  // payable
		"333", "3331", "33311", // output VAT
		"334", "338", "3382", "3383", "3384", "3385", // payroll liabilities
		"421", "4211", "4212", // retained earnings
		"511", "5111", // revenue
		"611",                 // goods purchase — EXISTS under 133
		"632",                 // cost of goods sold
		"642", "6421", "6422", // operating expenses
		"711", "811", "821", // other income/expense, CIT
		"911",                 // profit determination
		"229", "2293", "2294", // provisions
	}

	for _, code := range want {
		if _, ok := coa.ByCode(code); !ok {
			t.Errorf("account %s is missing from the TT133 chart", code)
		}
	}
}

// Accounts that exist under other regimes but NOT under 133/2016. Adding any of
// these would be importing another circular's chart.
func TestTT133Chart_ExcludesAccountsFromOtherRegimes(t *testing.T) {
	coa := gl.NewChartOfAccounts()
	for _, a := range gl.TT133Chart() {
		if err := coa.Add(a); err != nil {
			t.Fatalf("seed account %q is malformed: %v", a.Code, err)
		}
	}

	// 521 and its cấp 2 exist in the 99/2025 chart, not in 133. Under 133
	// revenue deductions are debited straight to TK 511.
	// 621/622/627 and 641 are likewise absent from Phụ lục 1.
	// 1562 is absent: TK 156 has no prescribed cấp 2 under 133.
	for _, code := range []string{"521", "5211", "5212", "5213", "621", "622", "627", "641", "1562"} {
		if _, ok := coa.ByCode(code); ok {
			t.Errorf("account %s must NOT be in the TT133 chart", code)
		}
	}
}

// Nature follows the account class, with contra accounts carrying the opposite
// side. This is a documented project rule, not a citation — the circular lists
// accounts by class, not by normal balance.
func TestTT133Chart_AssignsNatureByClass(t *testing.T) {
	coa := gl.NewChartOfAccounts()
	for _, a := range gl.TT133Chart() {
		if err := coa.Add(a); err != nil {
			t.Fatalf("seed account %q is malformed: %v", a.Code, err)
		}
	}

	cases := map[string]gl.AccountNature{
		"111": gl.NatureDebit,  // cash
		"131": gl.NatureDual,   // receivable can flip to credit
		"331": gl.NatureDual,   // payable can flip to debit
		"214": gl.NatureCredit, // contra-asset: accumulated depreciation
		"229": gl.NatureCredit, // contra-asset: provisions against assets
		"419": gl.NatureDebit,  // contra-equity: treasury shares
		"411": gl.NatureCredit, // owner capital
		"511": gl.NatureCredit, // revenue
		"632": gl.NatureDebit,  // cost of sales
		"911": gl.NatureDual,   // profit determination
	}

	for code, want := range cases {
		got, ok := coa.ByCode(code)
		if !ok {
			t.Errorf("account %s missing", code)
			continue
		}
		if got.Nature != want {
			t.Errorf("account %s nature = %q, want %q", code, got.Nature, want)
		}
	}
}

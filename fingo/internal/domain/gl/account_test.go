package gl_test

import (
	"testing"

	"fingo/internal/domain/gl"
)

// These tests exercise the chart-of-accounts STRUCTURE and its invariants.
// Account numbers appearing here are illustrative only: the statutory seed for
// the TT133 regime is pending question Q3 in docs/legal-basis.md (the TT133
// chart of accounts as amended by Circular 46/2025/TT-BTC has not been read
// from primary text). Nothing here asserts that a given account exists in law.

func TestChartOfAccounts_RegistersAndFindsByCode(t *testing.T) {
	coa := gl.NewChartOfAccounts()

	err := coa.Add(gl.Account{
		Code:   "111",
		Name:   "Tiền mặt",
		Type:   gl.AccountTypeAsset,
		Nature: gl.NatureDebit,
	})
	if err != nil {
		t.Fatalf("adding a well-formed asset account failed: %v", err)
	}

	got, ok := coa.ByCode("111")
	if !ok {
		t.Fatal("account 111 not found after Add")
	}
	if got.Name != "Tiền mặt" {
		t.Fatalf("name = %q, want %q", got.Name, "Tiền mặt")
	}
	if !got.IsActive {
		t.Fatal("a newly added account must be active by default")
	}
}

// The code is how the ledger refers to an account, so a duplicate would make
// every posting that names it ambiguous.
func TestChartOfAccounts_RejectsDuplicateCode(t *testing.T) {
	coa := gl.NewChartOfAccounts()

	first := gl.Account{Code: "111", Name: "Tiền mặt", Type: gl.AccountTypeAsset, Nature: gl.NatureDebit}
	if err := coa.Add(first); err != nil {
		t.Fatalf("first Add failed: %v", err)
	}

	second := gl.Account{Code: "111", Name: "Tiền Việt Nam", Type: gl.AccountTypeAsset, Nature: gl.NatureDebit}
	if err := coa.Add(second); err == nil {
		t.Fatal("duplicate account code must be rejected, got nil")
	}

	// The rejected add must not have overwritten the original.
	got, _ := coa.ByCode("111")
	if got.Name != "Tiền mặt" {
		t.Fatalf("surviving name = %q, want %q — the rejected add overwrote the account", got.Name, "Tiền mặt")
	}
}

// Note: type/nature pairing is deliberately NOT validated. Contra accounts are
// legitimate — accumulated depreciation is an ASSET account carrying a CREDIT
// balance, and revenue deductions are REVENUE accounts carrying DEBIT
// balances. Enforcing a pairing would reject real charts.
func TestChartOfAccounts_RejectsMalformedAccounts(t *testing.T) {
	cases := map[string]gl.Account{
		"empty code": {
			Code: "", Name: "Tiền mặt", Type: gl.AccountTypeAsset, Nature: gl.NatureDebit,
		},
		"non-numeric code": {
			Code: "111a", Name: "Tiền mặt", Type: gl.AccountTypeAsset, Nature: gl.NatureDebit,
		},
		"code with whitespace": {
			Code: "1 11", Name: "Tiền mặt", Type: gl.AccountTypeAsset, Nature: gl.NatureDebit,
		},
		"empty name": {
			Code: "111", Name: "", Type: gl.AccountTypeAsset, Nature: gl.NatureDebit,
		},
		"unknown type": {
			Code: "111", Name: "Tiền mặt", Type: gl.AccountType("MYSTERY"), Nature: gl.NatureDebit,
		},
		"unknown nature": {
			Code: "111", Name: "Tiền mặt", Type: gl.AccountTypeAsset, Nature: gl.AccountNature("MYSTERY"),
		},
	}

	for name, a := range cases {
		t.Run(name, func(t *testing.T) {
			coa := gl.NewChartOfAccounts()
			if err := coa.Add(a); err == nil {
				t.Fatalf("%s must be rejected, got nil", name)
			}
			if _, found := coa.ByCode(a.Code); found {
				t.Fatalf("%s must not be registered", name)
			}
		})
	}
}

// A contra account is legal even though its nature looks "wrong" for its type.
// This test exists to stop a future contributor adding a pairing rule.
func TestChartOfAccounts_AcceptsContraAccounts(t *testing.T) {
	coa := gl.NewChartOfAccounts()

	// Accumulated depreciation: an ASSET account with a CREDIT nature.
	err := coa.Add(gl.Account{
		Code: "214", Name: "Hao mòn tài sản cố định",
		Type: gl.AccountTypeAsset, Nature: gl.NatureCredit,
	})
	if err != nil {
		t.Fatalf("a contra-asset account must be accepted, got: %v", err)
	}
}

func addAccount(t *testing.T, coa *gl.ChartOfAccounts, code, name, parent string) {
	t.Helper()
	err := coa.Add(gl.Account{
		Code:       code,
		Name:       name,
		ParentCode: parent,
		Type:       gl.AccountTypeAsset,
		Nature:     gl.NatureDebit,
	})
	if err != nil {
		t.Fatalf("Add(%s) failed: %v", code, err)
	}
}

// A well-formed hierarchy validates. Codes may be added child-before-parent:
// a seed file's order must not matter.
func TestChartOfAccounts_ValidateAcceptsWellFormedHierarchy(t *testing.T) {
	coa := gl.NewChartOfAccounts()

	addAccount(t, coa, "11111", "Tiền Việt Nam tại quỹ", "1111")
	addAccount(t, coa, "111", "Tiền mặt", "")
	addAccount(t, coa, "1111", "Tiền Việt Nam", "111")

	if err := coa.Validate(); err != nil {
		t.Fatalf("well-formed hierarchy must validate, got: %v", err)
	}
}

// A parent reference that resolves to nothing means part of the chart is
// floating, and a report that rolls up the tree would silently lose it.
func TestChartOfAccounts_ValidateRejectsMissingParent(t *testing.T) {
	coa := gl.NewChartOfAccounts()
	addAccount(t, coa, "1111", "Tiền Việt Nam", "111") // 111 never added

	if err := coa.Validate(); err == nil {
		t.Fatal("an account referencing a missing parent must be rejected, got nil")
	}
}

// A cycle would make any roll-up of the tree non-terminating.
func TestChartOfAccounts_ValidateRejectsCycle(t *testing.T) {
	coa := gl.NewChartOfAccounts()
	addAccount(t, coa, "111", "Tiền mặt", "1111")
	addAccount(t, coa, "1111", "Tiền Việt Nam", "111")

	if err := coa.Validate(); err == nil {
		t.Fatal("a parent cycle must be rejected, got nil")
	}
}

// An account cannot be its own parent.
func TestChartOfAccounts_ValidateRejectsSelfParent(t *testing.T) {
	coa := gl.NewChartOfAccounts()
	addAccount(t, coa, "111", "Tiền mặt", "111")

	if err := coa.Validate(); err == nil {
		t.Fatal("a self-parenting account must be rejected, got nil")
	}
}

func seedTT133Chart(t *testing.T) *gl.ChartOfAccounts {
	t.Helper()

	coa := gl.NewChartOfAccounts()
	for _, a := range gl.TT133Chart() {
		if err := coa.Add(a); err != nil {
			t.Fatalf("seed %s: %v", a.Code, err)
		}
	}

	return coa
}

// A heading is not a posting target. Postability is derived from the chart
// rather than stored, so it stays true when the chart changes.
func TestChartOfAccounts_IsPostableDistinguishesHeadingsFromLeaves(t *testing.T) {
	coa := seedTT133Chart(t)

	cases := map[string]bool{
		"131":   true,  // receivable — a leaf
		"156":   true,  // goods
		"5111":  true,  // revenue, a prescribed cấp 2
		"33311": true,  // output VAT, cấp 3 beneath 3331
		"911":   true,  // profit determination
		"111":   false, // heading over 1111 and 1112
		"511":   false, // heading over 5111–5118
		"3331":  false, // heading over 33311 and 33312
		"642":   false, // heading over 6421 and 6422
		"9999":  false, // not in the chart at all
	}

	for code, want := range cases {
		if got := coa.IsPostable(code); got != want {
			t.Errorf("IsPostable(%s) = %v, want %v", code, got, want)
		}
	}
}

// TT 46/2025/TT-BTC lets an enterprise modify its own chart. Renaming an account
// must not disturb whether it is active.
func TestChartOfAccounts_UpdateChangesPresentationOnly(t *testing.T) {
	coa := seedTT133Chart(t)

	renamed := gl.Account{
		Code: "156", Name: "Hàng hóa (đổi tên)",
		Type: gl.AccountTypeAsset, Nature: gl.NatureDebit,
	}
	if err := coa.Update(renamed); err != nil {
		t.Fatalf("Update: %v", err)
	}

	got, _ := coa.ByCode("156")
	if got.Name != "Hàng hóa (đổi tên)" {
		t.Fatalf("name = %q, want it updated", got.Name)
	}
	if !got.IsActive {
		t.Fatal("an update must not deactivate an account")
	}

	ghost := gl.Account{Code: "9999", Name: "Ghost", Type: gl.AccountTypeAsset, Nature: gl.NatureDebit}
	if err := coa.Update(ghost); err == nil {
		t.Fatal("updating an account that does not exist must be refused, got nil")
	}
}

// Deactivating stops new postings; the account and its history remain, because
// historic balances refer to it.
func TestChartOfAccounts_DeactivateStopsPostings(t *testing.T) {
	coa := seedTT133Chart(t)

	if err := coa.Deactivate("156"); err != nil {
		t.Fatalf("Deactivate: %v", err)
	}

	if coa.IsPostable("156") {
		t.Fatal("a deactivated account must not accept postings")
	}
	if _, ok := coa.ByCode("156"); !ok {
		t.Fatal("a deactivated account must still be readable — its history depends on it")
	}
	if err := coa.Deactivate("9999"); err == nil {
		t.Fatal("deactivating an account that does not exist must be refused, got nil")
	}
}

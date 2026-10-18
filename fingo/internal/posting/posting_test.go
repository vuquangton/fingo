package posting_test

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"fingo/internal/domain/document"
	"fingo/internal/domain/gl"
	"fingo/internal/posting"
)

// rule builds a rule for a decision, so the tests read as the decisions they are.
func rule(id string, docType document.Kind, side gl.EntrySide, accountType gl.AccountType, code string) posting.Rule {
	return posting.Rule{
		ID:          id,
		DocType:     docType,
		Side:        side,
		AccountType: accountType,
		AccountCode: code,
	}
}

func salesDebitAsset(code string) posting.Rule {
	return rule("sales.receivable", document.KindSalesInvoice, gl.SideDebit, gl.AccountTypeAsset, code)
}

// ---------------------------------------------------------------------------
// Resolution
// ---------------------------------------------------------------------------

// Resolution happy path: the sales cycle the engine records today.
func TestResolve_AnswersTheSalesCycle(t *testing.T) {
	set, err := posting.Load(posting.TT133Defaults())
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	cases := []struct {
		side        gl.EntrySide
		accountType gl.AccountType
		want        string
	}{
		{gl.SideDebit, gl.AccountTypeAsset, "131"},     // phải thu của khách hàng
		{gl.SideCredit, gl.AccountTypeRevenue, "5111"}, // doanh thu bán hàng hóa
		{gl.SideCredit, gl.AccountTypeLiability, "33311"},
		{gl.SideDebit, gl.AccountTypeExpense, "632"}, // giá vốn hàng bán
		{gl.SideCredit, gl.AccountTypeAsset, "156"},  // hàng hóa
	}

	for _, c := range cases {
		got := set.Resolve(document.KindSalesInvoice, c.side, c.accountType)
		if got != c.want {
			t.Errorf("Resolve(sales, %s, %s) = %q, want %q", c.side, c.accountType, got, c.want)
		}
	}
}

// The two asset decisions on a sale are told apart by their side, which is the
// whole reason side is part of the key.
func TestResolve_SeparatesTheTwoAssetDecisions(t *testing.T) {
	set, err := posting.Load(posting.TT133Defaults())
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	debit := set.Resolve(document.KindSalesInvoice, gl.SideDebit, gl.AccountTypeAsset)
	credit := set.Resolve(document.KindSalesInvoice, gl.SideCredit, gl.AccountTypeAsset)

	if debit == credit {
		t.Fatalf("both asset decisions resolved to %q; the receivable and the inventory "+
			"relief cannot be the same account", debit)
	}
	if debit != "131" || credit != "156" {
		t.Errorf("debit asset = %q, credit asset = %q, want 131 and 156", debit, credit)
	}
}

func TestLookup_ReportsAMissingDecisionWithoutPanicking(t *testing.T) {
	set, err := posting.Load(posting.TT133Defaults())
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if _, ok := set.Lookup(document.KindSalesInvoice, gl.SideDebit, gl.AccountTypeAsset); !ok {
		t.Error("the receivable decision must be found")
	}

	// A decision the defaults do not answer: purchases cannot be keyed this way
	// yet — see the note on Requirements.
	if _, ok := set.Lookup(document.KindPurchaseInvoice, gl.SideDebit, gl.AccountTypeAsset); ok {
		t.Error("a purchase decision must not resolve against the sales defaults")
	}
}

// ---------------------------------------------------------------------------
// Failing at boot, not at runtime
// ---------------------------------------------------------------------------

// Missing rule → startup error, not runtime panic.
func TestLoad_RefusesAMissingRule(t *testing.T) {
	// Every sales decision except the receivable.
	incomplete := posting.TT133Defaults()[1:]

	_, err := posting.Load(incomplete)
	if err == nil {
		t.Fatal("a set missing a required decision must be refused, got nil")
	}
	if !errors.Is(err, posting.ErrUndefined) {
		t.Fatalf("error = %v, want ErrUndefined", err)
	}
}

// Ambiguous rule (same docType+side+accountType, two codes) → boot error.
func TestLoad_RefusesAnAmbiguousRule(t *testing.T) {
	rules := append(posting.TT133Defaults(), salesDebitAsset("1388"))

	_, err := posting.Load(rules)
	if err == nil {
		t.Fatal("two codes for one decision must be refused, got nil")
	}
	if !errors.Is(err, posting.ErrAmbiguous) {
		t.Fatalf("error = %v, want ErrAmbiguous", err)
	}

	// The error names both codes, because the person reading it at boot has to
	// know which two rules to reconcile.
	if !strings.Contains(err.Error(), "131") || !strings.Contains(err.Error(), "1388") {
		t.Errorf("error %q should name both accounts", err)
	}
}

// The same decision answered twice with the same code is not ambiguous, only
// redundant.
func TestLoad_AcceptsTheSameAnswerTwice(t *testing.T) {
	rules := append(posting.TT133Defaults(), salesDebitAsset("131"))

	if _, err := posting.Load(rules); err != nil {
		t.Fatalf("a repeated answer must be accepted: %v", err)
	}
}

// Resolve panics when a set has no answer — but a set from Load cannot be in
// that state, which is the point of the boot check.
//
// The set here is built deliberately without requirements, which is the only way
// to reach the panic. That is what makes the panic a programming error rather
// than an operational one.
func TestResolve_PanicsWhenTheSetWasNotCheckedForCompleteness(t *testing.T) {
	unchecked, err := posting.LoadFor(posting.TT133Defaults(), nil)
	if err != nil {
		t.Fatalf("LoadFor with no requirements: %v", err)
	}

	defer func() {
		if recover() == nil {
			t.Fatal("resolving a decision with no rule must panic")
		}
	}()

	unchecked.Resolve(document.KindPurchaseInvoice, gl.SideDebit, gl.AccountTypeAsset)
}

// The panic message has to say which decision was unanswerable, or the person
// reading the stack trace learns nothing.
func TestResolve_ThePanicNamesTheDecision(t *testing.T) {
	unchecked, err := posting.LoadFor(posting.TT133Defaults(), nil)
	if err != nil {
		t.Fatalf("LoadFor: %v", err)
	}

	defer func() {
		recovered := recover()
		if recovered == nil {
			t.Fatal("expected a panic")
		}

		message := recovered.(string)
		for _, part := range []string{"PURCHASE_INVOICE", "DEBIT", "ASSET"} {
			if !strings.Contains(message, part) {
				t.Errorf("panic %q should name %s", message, part)
			}
		}
	}()

	unchecked.Resolve(document.KindPurchaseInvoice, gl.SideDebit, gl.AccountTypeAsset)
}

func TestLoad_RefusesAMalformedRule(t *testing.T) {
	cases := map[string]posting.Rule{
		"no id":                rule("", document.KindSalesInvoice, gl.SideDebit, gl.AccountTypeAsset, "131"),
		"no code":              rule("x", document.KindSalesInvoice, gl.SideDebit, gl.AccountTypeAsset, ""),
		"blank code":           rule("x", document.KindSalesInvoice, gl.SideDebit, gl.AccountTypeAsset, "  "),
		"unknown document":     rule("x", document.Kind("MYSTERY"), gl.SideDebit, gl.AccountTypeAsset, "131"),
		"unknown side":         rule("x", document.KindSalesInvoice, gl.EntrySide("SIDEWAYS"), gl.AccountTypeAsset, "131"),
		"no side":              rule("x", document.KindSalesInvoice, gl.EntrySide(""), gl.AccountTypeAsset, "131"),
		"unknown account type": rule("x", document.KindSalesInvoice, gl.SideDebit, gl.AccountType("MYSTERY"), "131"),
	}

	for name, r := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := posting.LoadFor([]posting.Rule{r}, nil); err == nil {
				t.Fatalf("%s must be refused, got nil", name)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Read-only at runtime
// ---------------------------------------------------------------------------

// Rules are read-only at runtime: there is no way to change one once the books
// are open. The admin UI that edits them is a later, separate act.
func TestRuleSet_IsReadOnly(t *testing.T) {
	forbidden := []string{"set", "add", "put", "update", "delete", "remove", "edit", "clear"}

	for _, typ := range []reflect.Type{
		reflect.TypeOf(posting.RuleSet{}),
		reflect.TypeOf(&posting.RuleSet{}),
	} {
		for i := 0; i < typ.NumMethod(); i++ {
			name := typ.Method(i).Name
			lower := strings.ToLower(name)

			// Resolve and Lookup read; Rules reads. Nothing else should exist.
			switch name {
			case "Resolve", "Lookup", "Rules":
				continue
			}

			for _, word := range forbidden {
				if strings.Contains(lower, word) {
					t.Errorf("%s has method %s; rules are read-only at runtime", typ, name)
				}
			}
		}
	}
}

// The set hands out a copy, so a caller cannot edit the rules by keeping the
// slice it was given.
func TestRules_ReturnsACopy(t *testing.T) {
	set, err := posting.Load(posting.TT133Defaults())
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	first := set.Rules()
	if len(first) == 0 {
		t.Fatal("no rules came back")
	}

	first[0].AccountCode = "9999"

	if got := set.Resolve(first[0].DocType, first[0].Side, first[0].AccountType); got == "9999" {
		t.Error("editing the returned slice changed the loaded rules")
	}
}

// ---------------------------------------------------------------------------
// The contract with the engine
// ---------------------------------------------------------------------------

// Every required decision is expressible, and Load accepts the defaults for all
// of them. This is the assertion that the engine's vocabulary and the rule key
// agree.
func TestRequirements_AreAnsweredByTheDefaults(t *testing.T) {
	if len(posting.Requirements) == 0 {
		t.Fatal("the engine must declare the decisions it makes")
	}

	set, err := posting.Load(posting.TT133Defaults())
	if err != nil {
		t.Fatalf("the defaults do not cover the requirements: %v", err)
	}

	for _, key := range posting.Requirements {
		if _, ok := set.Lookup(key.DocType, key.Side, key.AccountType); !ok {
			t.Errorf("no default answers %s %s %s", key.DocType, key.Side, key.AccountType)
		}
	}
}

// Every default points at an account the statutory chart actually has, and can
// post to. A rule naming an account the chart does not carry would be a
// mapping to nothing — and it would only show up as a failed posting.
func TestTT133Defaults_NameAccountsInTheChart(t *testing.T) {
	chart := gl.NewChartOfAccounts()
	for _, a := range gl.TT133Chart() {
		if err := chart.Add(a); err != nil {
			t.Fatalf("seed the chart with %s: %v", a.Code, err)
		}
	}

	for _, r := range posting.TT133Defaults() {
		if !chart.IsPostable(r.AccountCode) {
			t.Errorf("rule %s names %s, which is not a postable account in the TT133 chart",
				r.ID, r.AccountCode)
		}
	}
}

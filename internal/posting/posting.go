// Package posting holds the rules that decide which account receives a debit or
// a credit.
//
// The engine's use cases describe what happened — a sale, a purchase, a receipt
// — and these rules say where it goes. Keeping the mapping here rather than as
// constants inside each use case is what lets a client's own chart be honoured:
// Circular 46/2025/TT-BTC lets an enterprise modify its chart, so "revenue goes
// to 5111" is a default and not a fact.
//
// The rules are loaded once, at boot, and checked then. A missing or
// contradictory rule is a fault in the configuration, and the time to find a
// fault is when the books are opened — not half-way through a month, with a
// voucher half-built.
package posting

import (
	"errors"
	"fmt"
	"strings"

	"fingo/internal/domain/document"
	"fingo/internal/domain/gl"
)

var (
	// ErrAmbiguous reports two rules answering one decision with different
	// accounts.
	ErrAmbiguous = errors.New("two posting rules answer the same decision")

	// ErrUndefined reports a decision the engine makes that no rule answers.
	ErrUndefined = errors.New("no posting rule answers a required decision")
)

// Key identifies a decision: for this kind of document, on this side, for this
// kind of account, which account?
//
// The key is exactly what it is, and no more. It expresses the sales cycle
// completely, and it does NOT express a purchase invoice: inventory (TK 156) and
// deductible input VAT (TK 1331) are both asset accounts debited by the same
// document, so they collide on (purchase, debit, asset). Purchases need a
// further dimension — the role the line plays — before they can be keyed this
// way. Sales are what the engine records today, which is why Requirements lists
// only those.
type Key struct {
	DocType     document.Kind
	Side        gl.EntrySide
	AccountType gl.AccountType
}

func (k Key) String() string {
	return fmt.Sprintf("%s/%s/%s", k.DocType, k.Side, k.AccountType)
}

// Rule is one answer to one decision.
type Rule struct {
	ID          string
	DocType     document.Kind
	Side        gl.EntrySide
	AccountType gl.AccountType
	AccountCode string
}

// Key is the decision this rule answers.
func (r Rule) Key() Key {
	return Key{DocType: r.DocType, Side: r.Side, AccountType: r.AccountType}
}

// Validate reports whether the rule could answer anything.
func (r Rule) Validate() error {
	if strings.TrimSpace(r.ID) == "" {
		return errors.New("a posting rule needs an id")
	}
	if !r.DocType.Valid() {
		return fmt.Errorf("posting rule %s names document type %q, which is not one of the known kinds",
			r.ID, r.DocType)
	}
	if !validSide(r.Side) {
		return fmt.Errorf("posting rule %s names side %q; a posting is a debit or a credit",
			r.ID, r.Side)
	}
	if !r.AccountType.Valid() {
		return fmt.Errorf("posting rule %s names account type %q, which is not one of the five types",
			r.ID, r.AccountType)
	}
	if strings.TrimSpace(r.AccountCode) == "" {
		return fmt.Errorf("posting rule %s names no account", r.ID)
	}

	return nil
}

// validSide reports whether a side is a debit or a credit.
//
// gl.EntrySide carries no Valid of its own: it is a closed set of two, and the
// ledger only ever writes it by constant. A rule comes from configuration, where
// nothing makes that true.
func validSide(side gl.EntrySide) bool {
	return side == gl.SideDebit || side == gl.SideCredit
}

// Requirements lists every posting decision the engine makes today.
//
// It is the contract between the documents and the chart of accounts. The engine
// cannot record a document whose decisions are unanswered, so a chart that
// leaves one out is a configuration fault — and Load turns it into a boot error
// rather than a surprise at month end.
var Requirements = []Key{
	{DocType: document.KindSalesInvoice, Side: gl.SideDebit, AccountType: gl.AccountTypeAsset},
	{DocType: document.KindSalesInvoice, Side: gl.SideCredit, AccountType: gl.AccountTypeRevenue},
	{DocType: document.KindSalesInvoice, Side: gl.SideCredit, AccountType: gl.AccountTypeLiability},
	{DocType: document.KindSalesInvoice, Side: gl.SideDebit, AccountType: gl.AccountTypeExpense},
	{DocType: document.KindSalesInvoice, Side: gl.SideCredit, AccountType: gl.AccountTypeAsset},
}

// TT133Defaults is the sales-cycle mapping from Phụ lục 1 of Circular
// 133/2016/TT-BTC — and the accounts internal/usecase/sales posts to today.
//
// These are defaults and not facts: Circular 46/2025/TT-BTC lets an enterprise
// modify its own chart, so a client may point any of them elsewhere. That is the
// reason they live here as data rather than as constants in the use case.
func TT133Defaults() []Rule {
	return []Rule{
		{
			ID: "sales-receivable", DocType: document.KindSalesInvoice,
			Side: gl.SideDebit, AccountType: gl.AccountTypeAsset, AccountCode: "131",
		},
		{
			ID: "sales-revenue", DocType: document.KindSalesInvoice,
			Side: gl.SideCredit, AccountType: gl.AccountTypeRevenue, AccountCode: "5111",
		},
		{
			ID: "sales-output-vat", DocType: document.KindSalesInvoice,
			Side: gl.SideCredit, AccountType: gl.AccountTypeLiability, AccountCode: "33311",
		},
		{
			ID: "sales-cost", DocType: document.KindSalesInvoice,
			Side: gl.SideDebit, AccountType: gl.AccountTypeExpense, AccountCode: "632",
		},
		{
			ID: "sales-inventory", DocType: document.KindSalesInvoice,
			Side: gl.SideCredit, AccountType: gl.AccountTypeAsset, AccountCode: "156",
		},
	}
}

// RuleSet is a checked set of rules.
//
// It offers no way to change one. Rules are read-only at runtime; an admin UI
// that edits them is a later, separate act that would write them down and
// reload.
type RuleSet struct {
	byKey map[Key]string
	rules []Rule
}

// Load builds a rule set and checks it against everything the engine needs.
func Load(rules []Rule) (*RuleSet, error) {
	return LoadFor(rules, Requirements)
}

// LoadFor builds a rule set against a stated set of required decisions, so a
// caller — or a test — can say what is needed.
//
// It refuses a set that answers one decision twice with different accounts, and
// one that leaves a required decision unanswered. Both are configuration faults
// and both are cheaper to find here than at the point of posting.
func LoadFor(rules []Rule, required []Key) (*RuleSet, error) {
	byKey := make(map[Key]string, len(rules))

	for i, r := range rules {
		if err := r.Validate(); err != nil {
			return nil, fmt.Errorf("rule %d: %w", i, err)
		}

		key := r.Key()

		if existing, already := byKey[key]; already && existing != r.AccountCode {
			return nil, fmt.Errorf("%w: %s is both %s and %s",
				ErrAmbiguous, key, existing, r.AccountCode)
		}

		byKey[key] = r.AccountCode
	}

	for _, key := range required {
		if _, answered := byKey[key]; !answered {
			return nil, fmt.Errorf("%w: %s", ErrUndefined, key)
		}
	}

	held := make([]Rule, len(rules))
	copy(held, rules)

	return &RuleSet{byKey: byKey, rules: held}, nil
}

// Resolve returns the account code for a posting decision.
//
// It panics when the set has no answer. A set built by Load cannot be in that
// state, because Load refuses to produce one that leaves a required decision
// unanswered — so the panic is a programming error, reachable only by building a
// set without checking it. Failing loudly there is the point: the alternative is
// posting to the wrong account and finding out at year end.
func (s *RuleSet) Resolve(docType document.Kind, side gl.EntrySide, accountType gl.AccountType) string {
	key := Key{DocType: docType, Side: side, AccountType: accountType}

	code, answered := s.byKey[key]
	if !answered {
		panic(fmt.Sprintf("posting: no rule answers %s", key))
	}

	return code
}

// Lookup is the form for a caller that wants to ask whether a decision is
// covered rather than assert that it is.
func (s *RuleSet) Lookup(
	docType document.Kind,
	side gl.EntrySide,
	accountType gl.AccountType,
) (string, bool) {
	code, answered := s.byKey[Key{DocType: docType, Side: side, AccountType: accountType}]

	return code, answered
}

// Rules returns the rules the set was built from, as a copy.
func (s *RuleSet) Rules() []Rule {
	out := make([]Rule, len(s.rules))
	copy(out, s.rules)

	return out
}

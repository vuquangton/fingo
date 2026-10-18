package gl

import (
	"errors"
	"fmt"
	"sort"
	"strings"
)

// The chart of accounts: the set of accounts a client's books are kept in.
//
// Structural rules only. Which specific accounts exist is a matter of the
// accounting regime (Circular 133/2016 for SMEs, Circular 99/2025 for the
// general regime) and is NOT encoded here — see docs/legal-basis.md.

type AccountType string

const (
	AccountTypeAsset     AccountType = "ASSET"
	AccountTypeLiability AccountType = "LIABILITY"
	AccountTypeEquity    AccountType = "EQUITY"
	AccountTypeRevenue   AccountType = "REVENUE"
	AccountTypeExpense   AccountType = "EXPENSE"
)

func (t AccountType) Valid() bool {
	switch t {
	case AccountTypeAsset, AccountTypeLiability, AccountTypeEquity,
		AccountTypeRevenue, AccountTypeExpense:
		return true
	}
	return false
}

// AccountNature is the side on which an account normally carries its balance.
//
// Deliberately NOT constrained against AccountType. Contra accounts are real:
// accumulated depreciation is an ASSET account carrying a CREDIT balance, and
// revenue deductions are REVENUE accounts carrying DEBIT balances. A pairing
// rule would reject legitimate charts.
type AccountNature string

const (
	NatureDebit  AccountNature = "DEBIT"
	NatureCredit AccountNature = "CREDIT"
	NatureDual   AccountNature = "DUAL"
)

func (n AccountNature) Valid() bool {
	switch n {
	case NatureDebit, NatureCredit, NatureDual:
		return true
	}
	return false
}

// Account is one account in the chart.
//
// Accounts are identified by their code — the number an accountant uses, such
// as "111" or "1111". Hierarchy is expressed by ParentCode, not by a database
// identity: the accountant's language is the domain language here.
type Account struct {
	Code       string
	Name       string
	ParentCode string
	Type       AccountType
	Nature     AccountNature
	IsActive   bool
}

// ChartOfAccounts holds one client's accounts.
type ChartOfAccounts struct {
	byCode map[string]Account
}

func NewChartOfAccounts() *ChartOfAccounts {
	return &ChartOfAccounts{byCode: make(map[string]Account)}
}

// Add registers an account. A newly added account is active.
func (c *ChartOfAccounts) Add(a Account) error {
	if err := validateAccount(a); err != nil {
		return err
	}
	if _, exists := c.byCode[a.Code]; exists {
		return fmt.Errorf("account %s already exists", a.Code)
	}

	a.IsActive = true
	c.byCode[a.Code] = a

	return nil
}

// Load registers an account read back from storage, preserving its active flag.
//
// Distinct from Add, which registers a NEW account and activates it. Loading a
// deactivated account must not revive it, or a chart read back from the database
// would silently re-enable accounts the client had switched off.
func (c *ChartOfAccounts) Load(a Account) error {
	if err := validateAccount(a); err != nil {
		return err
	}
	if _, exists := c.byCode[a.Code]; exists {
		return fmt.Errorf("account %s already exists", a.Code)
	}

	c.byCode[a.Code] = a

	return nil
}

// ByCode looks an account up by its code.
func (c *ChartOfAccounts) ByCode(code string) (Account, bool) {
	a, ok := c.byCode[code]
	return a, ok
}

// IsPostable reports whether an account can carry a posting: it must exist, be
// active, and have no sub-accounts.
//
// A parent is a heading, not a posting target. TK 511 exists to hold 5111–5118;
// posting to 511 itself would leave the sub-accounts unreconcilable with their
// own total. This is derived from the chart rather than stored as a flag, so it
// cannot drift out of step when the chart changes.
func (c *ChartOfAccounts) IsPostable(code string) bool {
	account, ok := c.byCode[code]
	if !ok || !account.IsActive {
		return false
	}

	for _, other := range c.byCode {
		if other.ParentCode == code {
			return false
		}
	}

	return true
}

// Update replaces an existing account's details.
//
// TT 46/2025/TT-BTC lets an enterprise modify its own chart without Ministry
// approval, so this is a supported operation rather than a maintenance back
// door. The code identifies the account and is not changed here; the active
// flag is managed by Deactivate so an update cannot silently revive an account.
func (c *ChartOfAccounts) Update(a Account) error {
	existing, ok := c.byCode[a.Code]
	if !ok {
		return fmt.Errorf("account %s does not exist", a.Code)
	}
	if err := validateAccount(a); err != nil {
		return err
	}

	a.IsActive = existing.IsActive
	c.byCode[a.Code] = a

	return nil
}

// Deactivate stops an account accepting new postings, without disturbing the
// entries already made against it. Historic balances must survive, so the
// account is never deleted.
func (c *ChartOfAccounts) Deactivate(code string) error {
	account, ok := c.byCode[code]
	if !ok {
		return fmt.Errorf("account %s does not exist", code)
	}

	account.IsActive = false
	c.byCode[code] = account

	return nil
}

// Validate checks the chart as a whole: every parent reference resolves, and
// no account is its own ancestor. Add deliberately does not check parents, so
// that a seed file's order does not matter; the hierarchy is settled here.
//
// Codes are visited in sorted order so the reported error is deterministic.
func (c *ChartOfAccounts) Validate() error {
	for _, code := range c.sortedCodes() {
		parent := c.byCode[code].ParentCode
		if parent == "" {
			continue
		}
		if _, ok := c.byCode[parent]; !ok {
			return fmt.Errorf("account %s references missing parent %s", code, parent)
		}
	}

	for _, code := range c.sortedCodes() {
		seen := make(map[string]bool)
		for cur := code; cur != ""; {
			if seen[cur] {
				return fmt.Errorf("account hierarchy contains a cycle at %s", cur)
			}
			seen[cur] = true

			a, ok := c.byCode[cur]
			if !ok {
				break
			}
			cur = a.ParentCode
		}
	}

	return nil
}

func (c *ChartOfAccounts) sortedCodes() []string {
	codes := make([]string, 0, len(c.byCode))
	for code := range c.byCode {
		codes = append(codes, code)
	}
	sort.Strings(codes)
	return codes
}

func validateAccount(a Account) error {
	if strings.TrimSpace(a.Code) == "" {
		return errors.New("account code is required")
	}
	if !isDigits(a.Code) {
		return fmt.Errorf("account code %q must be numeric", a.Code)
	}
	if strings.TrimSpace(a.Name) == "" {
		return fmt.Errorf("account %s must have a name", a.Code)
	}
	if !a.Type.Valid() {
		return fmt.Errorf("account %s has an unknown type %q", a.Code, a.Type)
	}
	if !a.Nature.Valid() {
		return fmt.Errorf("account %s has an unknown nature %q", a.Code, a.Nature)
	}
	return nil
}

func isDigits(s string) bool {
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

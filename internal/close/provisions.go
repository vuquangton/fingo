package close

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	"fingo/internal/domain/gl"
	"fingo/internal/money"
	"fingo/internal/usecase/ledger"
)

// ErrInvalidProvision is a provision calculation that could not be made: no
// accounts, no age bands, an asset whose useful life is not a positive number of
// months, a cost that is less than its residual, or a rate that is not a
// percentage.
var ErrInvalidProvision = errors.New("close: invalid provision")

// hundred is the divisor a percentage is read against.
var hundred = money.FromInt(100)

// DepreciableAsset is one asset a close depreciates.
//
// It carries what the accumulated depreciation already is rather than a start
// date, because that is the figure the charge depends on: the asset is
// depreciated until (cost − residual) is exhausted, and how many months that
// takes is the schedule's business, not the close's.
type DepreciableAsset struct {
	ID   string
	Name string

	// Cost is the original cost (nguyên giá — TK 211), and Residual the amount
	// the asset is expected to be worth at the end of its life. The depreciable
	// base is the difference.
	Cost     money.Dong
	Residual money.Dong

	// UsefulLifeMonths is the life the base is spread over.
	UsefulLifeMonths int

	// Accumulated is the depreciation charged against the asset so far
	// (hao mòn lũy kế — TK 214). The charge stops when the base is reached.
	Accumulated money.Dong

	// ExpenseAccount is where the period's charge is expensed, and
	// AccumulationAccount where it is accumulated. Empty means the provision
	// accounts' defaults are used.
	//
	// Per asset rather than per close: a delivery van and an office server are
	// both depreciated, and only one of them belongs in selling costs.
	ExpenseAccount      string
	AccumulationAccount string
}

// Assets answers which assets a client depreciates.
type Assets interface {
	// Depreciable returns the assets in service at an instant. An asset whose
	// depreciation has not started, or which has been disposed of, is not one of
	// them.
	Depreciable(ctx context.Context, asOf time.Time) ([]DepreciableAsset, error)
}

// Receivable is one amount a customer still owes, and when it fell due.
type Receivable struct {
	DocumentID string
	PartnerID  string

	// DueDate is when the amount fell due. A zero due date is an amount that is
	// not yet overdue, and is aged as current.
	DueDate time.Time

	// Amount is what is still outstanding on it — the document's total less
	// whatever has been allocated to it.
	Amount money.Dong
}

// Receivables answers what a client's customers still owe, and when it fell due.
type Receivables interface {
	// Outstanding returns the amounts owed to the client at an instant.
	Outstanding(ctx context.Context, asOf time.Time) ([]Receivable, error)
}

// ProvisionAccounts names the accounts a close's provision entries post to.
//
// All four are configured rather than fixed, for the reason every other account
// in this engine is: TT 46/2025/TT-BTC lets an enterprise modify its own chart,
// so "accumulated depreciation is 2141" is the statutory default and not a fact
// about the client.
//
// Every one of them must be a leaf the chart will accept a posting to. 214
// (hao mòn tài sản cố định) and 229 (dự phòng tổn thất tài sản) are headings
// with sub-accounts, and are exactly the kind of account this rule exists for:
// they are summaries, read by the balance sheet and written by nothing but the
// close — the close names 2141 and 2293 under them. A code that names a heading
// is refused by the chart when the entries are posted, because the ledger will
// not let a heading carry a posting.
type ProvisionAccounts struct {
	// DepreciationExpense is where the period's depreciation is expensed, and
	// AccumulatedDepreciation where it accumulates.
	DepreciationExpense     string
	AccumulatedDepreciation string

	// BadDebtExpense is where the period's bad debt provision is expensed, and
	// BadDebtProvision where the provision accumulates.
	BadDebtExpense   string
	BadDebtProvision string
}

// TT133ProvisionAccounts is the TT133 provision pair — Phụ lục 1 of Circular
// 133/2016/TT-BTC and Article 6 of Circular 48/2019/TT-BTC.
//
// 642 itself is a heading (6421 selling, 6422 administration), so the expense
// lands on its administration leaf. The accumulation accounts are the leaves
// under the headings 214 and 229.
func TT133ProvisionAccounts() ProvisionAccounts {
	return ProvisionAccounts{
		DepreciationExpense:     "6422",
		AccumulatedDepreciation: "2141",
		BadDebtExpense:          "6422",
		BadDebtProvision:        "2293",
	}
}

// Validate reports whether the four accounts could carry provision entries.
func (a ProvisionAccounts) Validate() error {
	// A slice rather than a map, so the account a client is told about first is
	// the same one every time.
	for _, named := range []struct{ name, code string }{
		{"depreciation expense", a.DepreciationExpense},
		{"accumulated depreciation", a.AccumulatedDepreciation},
		{"bad debt expense", a.BadDebtExpense},
		{"bad debt provision", a.BadDebtProvision},
	} {
		if named.code == "" {
			return fmt.Errorf("%w: no %s account", ErrInvalidProvision, named.name)
		}
	}

	// An entry that debits the account it credits moves nothing. Both pairs are
	// checked, so a chart that maps depreciation onto its own accumulation
	// account is refused here rather than posted as a wash.
	if a.DepreciationExpense == a.AccumulatedDepreciation {
		return fmt.Errorf("%w: depreciation and its accumulation are both %s",
			ErrInvalidProvision, a.DepreciationExpense)
	}
	if a.BadDebtExpense == a.BadDebtProvision {
		return fmt.Errorf("%w: the bad debt provision and its expense are both %s",
			ErrInvalidProvision, a.BadDebtExpense)
	}

	return nil
}

// AgingBand is one age band of receivables and the percentage provided against
// it.
//
// Bands are ordered youngest first and are contiguous: a band covers days from
// its own FromDays up to the day before the next band's, and the last band
// covers everything older. No overlap can be expressed, which is the point — two
// bands over one amount would provide against it twice, and a gap would leave
// some of what is owed unprovided for with nothing to say so.
type AgingBand struct {
	Name string

	// FromDays is the first day of overdue this band covers, inclusive. Overdue
	// days are counted from the due date to the close's date and are never
	// negative: an amount that is not yet due is aged as current.
	FromDays int

	// Percent is the provision as a percentage of the band, so 5 is five per
	// cent.
	Percent money.Dong
}

// BadDebtPolicy is how much of each age band of receivables is provided against.
type BadDebtPolicy struct {
	Bands []AgingBand
}

// DefaultBadDebtPolicy is the age bands an aging report is kept in, with nothing
// provided against any of them.
//
// The rates are configuration, and this is what a client has before they have
// configured any: bands that age the receivables and provide against none of
// them. It is a real policy — a client who provides against nothing is not the
// same as a client whose provision is unknown.
func DefaultBadDebtPolicy() BadDebtPolicy {
	return BadDebtPolicy{Bands: []AgingBand{
		{Name: "current", FromDays: 0},
		{Name: "1-30 days", FromDays: 1},
		{Name: "31-60 days", FromDays: 31},
		{Name: "61-90 days", FromDays: 61},
		{Name: "over 90 days", FromDays: 91},
	}}
}

// Circular48BadDebtPolicy is the provision schedule of Article 6 of Circular
// 48/2019/TT-BTC: 30% from six months to a year, 50% to two years, 70% to three,
// 90% to four, and everything older in full.
//
// The statute counts months and this counts days, because a close works in dates
// and a month is not a fixed number of them: the bands are the day-count
// stand-ins for the statute's month boundaries. Circular 48 expires no later
// than 31/12/2026 (see docs/requirements-v1.md, row 17), so this is a default
// with an end date rather than a permanent rule.
func Circular48BadDebtPolicy() BadDebtPolicy {
	return BadDebtPolicy{Bands: []AgingBand{
		{Name: "up to 6 months", FromDays: 0},
		{Name: "6-12 months", FromDays: 181, Percent: money.FromInt(30)},
		{Name: "1-2 years", FromDays: 366, Percent: money.FromInt(50)},
		{Name: "2-3 years", FromDays: 731, Percent: money.FromInt(70)},
		{Name: "3-4 years", FromDays: 1096, Percent: money.FromInt(90)},
		{Name: "over 4 years", FromDays: 1461, Percent: money.FromInt(100)},
	}}
}

// Validate reports whether the bands could be applied to a set of receivables.
func (p BadDebtPolicy) Validate() error {
	if len(p.Bands) == 0 {
		return fmt.Errorf("%w: a bad debt policy needs at least one age band", ErrInvalidProvision)
	}

	for i, band := range p.Bands {
		if band.Name == "" {
			return fmt.Errorf("%w: age band %d has no name", ErrInvalidProvision, i+1)
		}
		if band.Percent.IsNegative() || band.Percent.GreaterThan(hundred) {
			return fmt.Errorf("%w: age band %s provides %s per cent of what it covers",
				ErrInvalidProvision, band.Name, band.Percent)
		}
		switch {
		case i == 0 && band.FromDays != 0:
			// The first band has to catch what is not yet overdue, or an amount
			// due tomorrow falls into no band at all.
			return fmt.Errorf("%w: the youngest band %s starts at %d days overdue, not 0",
				ErrInvalidProvision, band.Name, band.FromDays)
		case i > 0 && band.FromDays <= p.Bands[i-1].FromDays:
			return fmt.Errorf("%w: age band %s starts at %d days overdue, which does not come after %s",
				ErrInvalidProvision, band.Name, band.FromDays, p.Bands[i-1].Name)
		}
	}

	return nil
}

// bandIndex returns the band an amount that many days overdue falls in. The
// policy must have been validated.
func (p BadDebtPolicy) bandIndex(daysOverdue int) int {
	index := 0

	for i, band := range p.Bands {
		if daysOverdue < band.FromDays {
			break
		}
		index = i
	}

	return index
}

// BadDebtCharge is what one age band was provided against at a close.
type BadDebtCharge struct {
	Band        string
	Outstanding money.Dong
	Percent     money.Dong
	Amount      money.Dong
}

// DepreciationCharge is what one asset was depreciated by at a close.
type DepreciationCharge struct {
	AssetID string
	Name    string

	// Amount is the charge for the period, Months how many monthly rates it
	// covers, and Accumulated what the asset stands at after it.
	Amount      money.Dong
	Months      int
	Accumulated money.Dong
}

// ProvisionSummary is what a close provided for at a period's end: the
// depreciation it charged and the bad debt it provided against, item by item.
//
// It is recorded on the close report because the entries alone do not say why:
// a voucher shows 6422 debited 100, and only this says which asset, over how
// many months, and how far through its life it now is.
type ProvisionSummary struct {
	Depreciation []DepreciationCharge
	BadDebt      []BadDebtCharge

	DepreciationTotal money.Dong
	BadDebtTotal      money.Dong

	// Total is what the provisions charged to the period's expenses, and is the
	// amount the closing entries then carry to retained earnings along with
	// everything else the period spent.
	Total money.Dong
}

// Provisions is the period-end provision calculation a close performs.
//
// It is opt-in: a close that has none provides for nothing and behaves exactly
// as it did before. A close that has one reads the assets it depreciates and the
// receivables it provides against, and posts the entries for both in the same
// voucher as the closing entries.
type Provisions struct {
	// Assets and Receivables are where the facts come from. Both are required:
	// a client with no fixed assets supplies an empty list, so that a wiring
	// mistake reads as a mistake rather than as nothing to do.
	Assets      Assets
	Receivables Receivables

	// Charts is the client's chart of accounts. Required, because every account
	// a provision posts to has to be one the chart will accept an entry to, and
	// one of the right kind: a close that found out at posting time would have
	// written its snapshot before the entry it recorded ever landed.
	Charts ledger.ChartSource

	// Accounts and BadDebt are what the calculation posts to and how much.
	Accounts ProvisionAccounts
	BadDebt  BadDebtPolicy
}

// Validate reports whether the calculation could be performed at all.
func (p Provisions) Validate() error {
	if p.Assets == nil {
		return fmt.Errorf("%w: a provision calculation needs the assets it depreciates", ErrInvalidProvision)
	}
	if p.Receivables == nil {
		return fmt.Errorf("%w: a provision calculation needs the receivables it provides against", ErrInvalidProvision)
	}
	if p.Charts == nil {
		return fmt.Errorf("%w: a provision calculation needs the chart its accounts are checked against", ErrInvalidProvision)
	}
	if err := p.Accounts.Validate(); err != nil {
		return err
	}

	return p.BadDebt.Validate()
}

// provisionConfig is everything a provision calculation is told beyond the facts
// it is given: the accounts it posts to, how much it provides, what it may not
// touch, and the chart that says whether it may touch them.
type provisionConfig struct {
	accounts ProvisionAccounts
	badDebt  BadDebtPolicy
	closing  ClosingAccounts
	chart    *gl.ChartOfAccounts
}

// checkProvisionPair reports whether a provision may post to the two accounts it
// names: the one it expenses, and the one it accumulates into.
//
// The expense has to be a profit and loss account and the accumulation must not
// be one. That is what keeps the promise a close makes — that every profit and
// loss account is left at zero. An asset whose depreciation is accumulated into
// 6421 credits an expense account the close does not clear, and the period is
// then closed with a profit and loss account standing at something other than
// nothing, which nothing later would notice.
func checkProvisionPair(expense, accumulation, what string, config provisionConfig) error {
	switch {
	case expense == "" || accumulation == "":
		return fmt.Errorf("%w: %s posts to no account", ErrInvalidProvision, what)
	case expense == accumulation:
		return fmt.Errorf("%w: %s expenses %s and accumulates into it, which moves nothing",
			ErrInvalidProvision, what, expense)
	case expense == config.closing.Result || accumulation == config.closing.Result:
		return fmt.Errorf("%w: %s posts to %s, which is where the close determines the period's result",
			ErrInvalidProvision, what, config.closing.Result)
	case expense == config.closing.Retained || accumulation == config.closing.Retained:
		return fmt.Errorf("%w: %s posts to %s, which is where the close carries the period's result",
			ErrInvalidProvision, what, config.closing.Retained)
	}

	if config.chart == nil {
		return nil
	}

	expenseAccount, known := config.chart.ByCode(expense)
	if !known || !config.chart.IsPostable(expense) {
		return fmt.Errorf("%w: %s expenses %s, which the chart will not accept a posting to",
			ErrInvalidProvision, what, expense)
	}
	if expenseAccount.Type != gl.AccountTypeExpense {
		return fmt.Errorf("%w: %s expenses %s, which is a %s account",
			ErrInvalidProvision, what, expense, expenseAccount.Type)
	}

	accumulationAccount, known := config.chart.ByCode(accumulation)
	if !known || !config.chart.IsPostable(accumulation) {
		return fmt.Errorf("%w: %s accumulates into %s, which the chart will not accept a posting to",
			ErrInvalidProvision, what, accumulation)
	}
	if accumulationAccount.Type == gl.AccountTypeExpense || accumulationAccount.Type == gl.AccountTypeRevenue {
		return fmt.Errorf("%w: %s accumulates into %s, which is a %s account the close would not clear",
			ErrInvalidProvision, what, accumulation, accumulationAccount.Type)
	}

	return nil
}

// provision is what one close provides for: the entries it posts, the expenses
// those entries charge, and the summary it records.
type provision struct {
	lines    []gl.VoucherLine
	expenses []AccountBalance
	summary  ProvisionSummary
}

// provisionsFor turns a period's assets and receivables into the entries a close
// posts for them.
//
// It is a pure function of what it is given — the facts, how many months the
// period covers, the accounts, the policy — so the arithmetic can be read and
// tested without a ledger, a calendar or a database.
func provisionsFor(
	assets []DepreciableAsset,
	receivables []Receivable,
	months int,
	asOf time.Time,
	config provisionConfig,
) (provision, error) {
	accounts, badDebt := config.accounts, config.badDebt

	if err := accounts.Validate(); err != nil {
		return provision{}, err
	}
	if err := badDebt.Validate(); err != nil {
		return provision{}, err
	}

	out := provision{summary: ProvisionSummary{
		Depreciation:      make([]DepreciationCharge, 0, len(assets)),
		BadDebt:           make([]BadDebtCharge, 0, len(badDebt.Bands)),
		DepreciationTotal: money.Zero,
		BadDebtTotal:      money.Zero,
		Total:             money.Zero,
	}}

	// Sorted so that two closes over the same assets produce the same voucher.
	ordered := make([]DepreciableAsset, len(assets))
	copy(ordered, assets)
	sort.SliceStable(ordered, func(i, j int) bool { return ordered[i].ID < ordered[j].ID })

	for _, asset := range ordered {
		charge, err := depreciationFor(asset, months)
		if err != nil {
			return provision{}, err
		}
		if charge.IsZero() {
			// Fully depreciated, or nothing left to spread: an asset that has
			// been written down does not keep producing entries.
			continue
		}

		expense := asset.ExpenseAccount
		if expense == "" {
			expense = accounts.DepreciationExpense
		}
		accumulation := asset.AccumulationAccount
		if accumulation == "" {
			accumulation = accounts.AccumulatedDepreciation
		}

		// An asset names its own accounts, so the accounts are checked per asset
		// and not once for the close: what a van is depreciated into is not what
		// a server is, and neither is exempt from the rule.
		what := "the depreciation of " + describe(asset)
		if err := checkProvisionPair(expense, accumulation, what, config); err != nil {
			return provision{}, err
		}

		note := "Depreciation of " + describe(asset)
		out.lines = append(out.lines,
			gl.VoucherLine{AccountCode: expense, Side: gl.SideDebit, Amount: charge, Note: note},
			gl.VoucherLine{AccountCode: accumulation, Side: gl.SideCredit, Amount: charge, Note: note},
		)
		out.expenses = append(out.expenses, expenseBalance(expense, charge))

		accumulated := asset.Accumulated.Add(charge)
		out.summary.Depreciation = append(out.summary.Depreciation, DepreciationCharge{
			AssetID:     asset.ID,
			Name:        asset.Name,
			Amount:      charge,
			Months:      months,
			Accumulated: accumulated,
		})
		out.summary.DepreciationTotal = out.summary.DepreciationTotal.Add(charge)
	}

	charges, err := badDebtCharges(receivables, asOf, badDebt)
	if err != nil {
		return provision{}, err
	}

	if len(charges) > 0 {
		if err := checkProvisionPair(accounts.BadDebtExpense, accounts.BadDebtProvision,
			"the bad debt provision", config); err != nil {
			return provision{}, err
		}
	}

	for _, charge := range charges {
		out.summary.BadDebt = append(out.summary.BadDebt, charge)
		if charge.Amount.IsZero() {
			// The band holds something, and the policy provides nothing against
			// it. It belongs in the summary and nowhere near the ledger.
			continue
		}

		note := fmt.Sprintf("Bad debt provision, %s (%s%% of %s owing)",
			charge.Band, charge.Percent, charge.Outstanding)
		out.lines = append(out.lines,
			gl.VoucherLine{AccountCode: accounts.BadDebtExpense, Side: gl.SideDebit, Amount: charge.Amount, Note: note},
			gl.VoucherLine{AccountCode: accounts.BadDebtProvision, Side: gl.SideCredit, Amount: charge.Amount, Note: note},
		)
		out.expenses = append(out.expenses, expenseBalance(accounts.BadDebtExpense, charge.Amount))
		out.summary.BadDebtTotal = out.summary.BadDebtTotal.Add(charge.Amount)
	}

	out.summary.Total = out.summary.DepreciationTotal.Add(out.summary.BadDebtTotal)

	return out, nil
}

// depreciationFor is the charge one close makes on one asset: the straight-line
// monthly rate for each month the period covers, and never more than what is
// left of the depreciable base.
//
// Truncating to whole đồng is what the rate costs: VND has no subunit, so a base
// that does not divide evenly by the life cannot be charged in equal whole-đồng
// instalments. The odd đồng is left to a later period, which the cap above turns
// into a final charge smaller than the rate — the asset lands exactly on its
// residual, and never past it.
func depreciationFor(asset DepreciableAsset, months int) (money.Dong, error) {
	rate, err := monthlyRate(asset)
	if err != nil {
		return money.Zero, err
	}
	if months < 1 {
		months = 1
	}

	remaining := asset.Cost.Sub(asset.Residual).Sub(asset.Accumulated)
	if !remaining.IsPositive() {
		return money.Zero, nil
	}

	charge := rate.Mul(money.FromInt(int64(months)))
	if charge.GreaterThan(remaining) {
		// The life is nearly over and this charge would pass the base. What is
		// left is the charge: the asset lands exactly on its residual, and never
		// past it.
		charge = remaining
	}

	return charge.Truncate(0), nil
}

// monthlyRate is (cost − residual) / useful life, in whole đồng.
//
// A rate of nothing is raised to one đồng. A base smaller than the life in
// months — an asset that costs less than a đồng for every month it is expected
// to last — would otherwise truncate to a rate of zero and never be written down
// at all. One đồng a month takes the same base over the same life, and the cap
// in depreciationFor still stops it there.
func monthlyRate(asset DepreciableAsset) (money.Dong, error) {
	switch {
	case asset.ID == "":
		return money.Zero, fmt.Errorf("%w: an asset to depreciate has no id", ErrInvalidProvision)
	case asset.UsefulLifeMonths <= 0:
		return money.Zero, fmt.Errorf("%w: asset %s has a useful life of %d months",
			ErrInvalidProvision, asset.ID, asset.UsefulLifeMonths)
	case !asset.Cost.IsPositive():
		return money.Zero, fmt.Errorf("%w: asset %s costs %s", ErrInvalidProvision, asset.ID, asset.Cost)
	case asset.Residual.IsNegative() || asset.Residual.GreaterThan(asset.Cost):
		return money.Zero, fmt.Errorf("%w: asset %s costs %s and is expected to be worth %s at the end of its life",
			ErrInvalidProvision, asset.ID, asset.Cost, asset.Residual)
	}

	rate := asset.Cost.Sub(asset.Residual).
		Div(money.FromInt(int64(asset.UsefulLifeMonths))).
		Truncate(0)
	if rate.IsZero() {
		rate = money.FromInt(1)
	}

	return rate, nil
}

// badDebtCharges ages the receivables into the policy's bands and works out what
// each band is provided against.
//
// Only bands that hold something are reported: a band nobody falls into is not a
// provision of nothing, it is an absence of one.
func badDebtCharges(receivables []Receivable, asOf time.Time, policy BadDebtPolicy) ([]BadDebtCharge, error) {
	if err := policy.Validate(); err != nil {
		return nil, err
	}

	outstanding := make([]money.Dong, len(policy.Bands))

	for _, receivable := range receivables {
		if !receivable.Amount.IsPositive() {
			// A credit balance, or a document with nothing left on it, is not
			// something to provide against.
			continue
		}

		band := policy.bandIndex(daysOverdue(receivable.DueDate, asOf))
		outstanding[band] = outstanding[band].Add(receivable.Amount)
	}

	charges := make([]BadDebtCharge, 0, len(policy.Bands))
	for i, band := range policy.Bands {
		if outstanding[i].IsZero() {
			continue
		}

		charges = append(charges, BadDebtCharge{
			Band:        band.Name,
			Outstanding: outstanding[i],
			Percent:     band.Percent,
			Amount:      outstanding[i].Mul(band.Percent).Div(hundred).Truncate(0),
		})
	}

	return charges, nil
}

// daysOverdue is how many days an amount is past its due date, counted in whole
// days and never negative.
func daysOverdue(due, asOf time.Time) int {
	if due.IsZero() {
		return 0
	}

	days := int(dayOf(asOf).Sub(dayOf(due)).Hours() / 24)
	if days < 0 {
		return 0
	}

	return days
}

// monthsSpanned is how many calendar months a period touches.
//
// Depreciation is a monthly charge, and a period is a stretch of dates rather
// than a month: a close of a one-month period charges one monthly rate, which is
// the ordinary case, and a close of a quarter charges three.
func monthsSpanned(from, to time.Time) int {
	start, end := from.UTC(), to.UTC()

	months := (end.Year()-start.Year())*12 + int(end.Month()) - int(start.Month()) + 1
	if months < 1 {
		return 1
	}

	return months
}

// dayOf drops the time of day, in UTC.
//
// Comparing instants would age an amount due at 09:00 on the closing date as
// though part of a day had passed, and one due at 23:00 as though none had: a
// period is a range of dates, exactly as internal/period holds it.
func dayOf(t time.Time) time.Time {
	u := t.UTC()

	return time.Date(u.Year(), u.Month(), u.Day(), 0, 0, 0, 0, time.UTC)
}

// expenseBalance is one provision's charge as a balance the closing entries can
// carry: the expense account the provision debited, and by how much.
//
// It is what ties the two halves of a close together. The provision entries and
// the closing entries are one voucher, and the closing entries must carry the
// expenses the provisions just created — read before them, the period's expenses
// would be short by exactly the provisions, and the profit and loss accounts
// would be left standing at that amount instead of at zero.
func expenseBalance(account string, amount money.Dong) AccountBalance {
	return AccountBalance{AccountCode: account, Type: gl.AccountTypeExpense, Amount: amount}
}

// withProvisions returns the period's balances with the provisions' expenses
// folded into them.
//
// An account the period already carries as something other than an expense is
// refused rather than folded into. The close would clear that account twice —
// once as the type it is, and once as the expense the provision made it — and
// what was left standing afterwards would be the difference between the two,
// silently.
func withProvisions(balances []AccountBalance, expenses []AccountBalance) ([]AccountBalance, error) {
	if len(expenses) == 0 {
		return balances, nil
	}

	out := make([]AccountBalance, len(balances))
	copy(out, balances)

	for _, expense := range expenses {
		found := -1

		for i := range out {
			if out[i].AccountCode == expense.AccountCode {
				found = i

				break
			}
		}

		if found < 0 {
			out = append(out, expense)

			continue
		}

		if out[found].Type != gl.AccountTypeExpense {
			return nil, fmt.Errorf(
				"%w: account %s carries the period's %s and a provision posts to it as an expense",
				ErrInvalidProvision, expense.AccountCode, out[found].Type)
		}

		out[found].Amount = out[found].Amount.Add(expense.Amount)
	}

	return out, nil
}

// nothingLeft reports whether a period's profit and loss accounts are all at
// zero: whether anything has been posted into the period since it was closed.
//
// It is the difference between a close being run again because the lock failed —
// the entries are in, the books are empty, and the snapshot on file is the truth
// — and one being run again after somebody posted into the period, which would
// leave whatever they posted behind in accounts the close had already emptied.
func nothingLeft(balances []AccountBalance) bool {
	for _, balance := range balances {
		if !balance.Amount.IsZero() {
			return false
		}
	}

	return true
}

// describe names an asset for a ledger note: what a bookkeeper would recognise
// it by, with the id as a fallback.
func describe(asset DepreciableAsset) string {
	if asset.Name != "" {
		return asset.Name
	}

	return asset.ID
}

package close

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"fingo/internal/domain/gl"
	"fingo/internal/money"
)

// testAssets is a fixed set of assets, stated outright.
type testAssets struct {
	assets []DepreciableAsset
	err    error
}

func (a *testAssets) Depreciable(context.Context, time.Time) ([]DepreciableAsset, error) {
	if a.err != nil {
		return nil, a.err
	}

	return a.assets, nil
}

// testReceivables is a fixed set of amounts owed, stated outright.
type testReceivables struct {
	receivables []Receivable
	err         error
}

func (r *testReceivables) Outstanding(context.Context, time.Time) ([]Receivable, error) {
	if r.err != nil {
		return nil, r.err
	}

	return r.receivables, nil
}

// overduePolicy is the band policy the tests provide under: nothing until an
// amount is more than ninety days late, then a stated percentage of it.
func overduePolicy(percent int64) BadDebtPolicy {
	policy := DefaultBadDebtPolicy()
	for i := range policy.Bands {
		if policy.Bands[i].Name == "over 90 days" {
			policy.Bands[i].Percent = money.FromInt(percent)
		}
	}

	return policy
}

// testCharts is the client's chart of accounts, as a close reads it to check
// that a provision posts only to accounts it will accept.
type testCharts struct {
	chart *gl.ChartOfAccounts
	err   error
}

func (c testCharts) Chart(context.Context) (*gl.ChartOfAccounts, error) {
	if c.err != nil {
		return nil, c.err
	}

	return c.chart, nil
}

// withProvisions makes the fixture's close one that provides for the stated
// assets and receivables, checked against the TT133 chart.
func (f *fixture) withProvisions(t *testing.T, assets *testAssets, receivables *testReceivables, policy BadDebtPolicy) {
	t.Helper()

	f.configureProvisions(t, Provisions{
		Assets:      assets,
		Receivables: receivables,
		Charts:      testCharts{chart: f.ledger.rules.Chart},
		Accounts:    TT133ProvisionAccounts(),
		BadDebt:     policy,
	})
}

// configureProvisions wires a provision calculation the test has built itself —
// including the ones that are wrong on purpose.
func (f *fixture) configureProvisions(t *testing.T, provisions Provisions) {
	t.Helper()

	service, err := f.service.WithProvisions(provisions)
	if err != nil {
		t.Fatalf("WithProvisions: %v", err)
	}

	f.service = service
}

// stretch widens the fixture's period to end later, so a test can close a
// stretch longer than a month.
func (f *fixture) stretch(end time.Time) *fixture {
	f.january.EndDate = end
	f.periods.periods[0].EndDate = end

	return f
}

// asset is a 12,000 asset with a 120-month life: 100 a month, the arithmetic the
// specification names.
func asset() DepreciableAsset {
	return DepreciableAsset{
		ID:               "FA-001",
		Name:             "Laptop",
		Cost:             money.FromInt(12_000),
		UsefulLifeMonths: 120,
	}
}

// A 12,000 asset over 120 months is 100 a month.
func TestDepreciationFor_IsTheStraightLineMonthlyRate(t *testing.T) {
	charge, err := depreciationFor(asset(), 1)
	if err != nil {
		t.Fatalf("depreciationFor: %v", err)
	}

	if want := money.FromInt(100); charge.Cmp(want) != 0 {
		t.Errorf("monthly depreciation = %s, want %s", charge, want)
	}
}

// The rate is charged once for each month the period covers, so a close of a
// quarter charges three rates.
func TestDepreciationFor_ChargesAMonthlyRateForEachMonthOfThePeriod(t *testing.T) {
	charge, err := depreciationFor(asset(), 3)
	if err != nil {
		t.Fatalf("depreciationFor: %v", err)
	}

	if want := money.FromInt(300); charge.Cmp(want) != 0 {
		t.Errorf("quarterly depreciation = %s, want %s", charge, want)
	}
}

// Residual is not depreciated: it is what the asset is expected to be worth at
// the end of its life.
func TestDepreciationFor_SpreadsOnlyWhatIsAboveTheResidual(t *testing.T) {
	worn := asset()
	worn.Residual = money.FromInt(1_200)
	worn.UsefulLifeMonths = 108 // (12,000 − 1,200) / 108 = 100

	charge, err := depreciationFor(worn, 1)
	if err != nil {
		t.Fatalf("depreciationFor: %v", err)
	}

	if want := money.FromInt(100); charge.Cmp(want) != 0 {
		t.Errorf("monthly depreciation = %s, want %s", charge, want)
	}
}

func TestDepreciationFor_NeverDepreciatesPastTheBase(t *testing.T) {
	cases := map[string]struct {
		accumulated money.Dong
		months      int
		want        int64
	}{
		"the last month takes what is left": {money.FromInt(11_950), 1, 50},
		"an overcharge is cut to the base":  {money.FromInt(11_950), 3, 50},
		"fully depreciated":                 {money.FromInt(12_000), 1, 0},
		"depreciated past the base":         {money.FromInt(13_000), 1, 0},
		"nothing above the residual":        {money.FromInt(0), 1, 100},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			worn := asset()
			worn.Accumulated = tc.accumulated

			charge, err := depreciationFor(worn, tc.months)
			if err != nil {
				t.Fatalf("depreciationFor: %v", err)
			}
			if got, want := charge, money.FromInt(tc.want); got.Cmp(want) != 0 {
				t.Errorf("charge = %s, want %s", got, want)
			}
		})
	}
}

// A base that does not divide evenly is left whole: VND has no subunit. A rate
// that truncated to nothing is raised to one đồng a month rather than spread
// over no periods at all — the asset still comes down to its residual.
func TestDepreciationFor_ChargesAWholeNumberOfDong(t *testing.T) {
	cheap := DepreciableAsset{
		ID:               "FA-002",
		Cost:             money.FromInt(100),
		UsefulLifeMonths: 120,
	}

	charge, err := depreciationFor(cheap, 1)
	if err != nil {
		t.Fatalf("depreciationFor: %v", err)
	}
	if want := money.FromInt(1); charge.Cmp(want) != 0 {
		t.Errorf("charge = %s, want %s — 100 over 120 months is not zero a month", charge, want)
	}

	// And it finishes: by the end of the hundredth month there is nothing left.
	spent := cheap
	spent.Accumulated = money.FromInt(100)
	if charge, err = depreciationFor(spent, 1); err != nil {
		t.Fatalf("depreciationFor: %v", err)
	}
	if !charge.IsZero() {
		t.Errorf("charge on a written-down asset = %s, want 0", charge)
	}

	fractional := DepreciableAsset{
		ID:               "FA-003",
		Cost:             money.FromInt(1_000),
		UsefulLifeMonths: 3,
	}

	charge, err = depreciationFor(fractional, 1)
	if err != nil {
		t.Fatalf("depreciationFor: %v", err)
	}
	if want := money.FromInt(333); charge.Cmp(want) != 0 {
		t.Errorf("charge = %s, want %s — 1,000 over 3 months has no exact whole-đồng rate", charge, want)
	}
}

func TestDepreciationFor_RefusesAnAssetThatCouldNotBeDepreciated(t *testing.T) {
	cases := map[string]func() DepreciableAsset{
		"no id":         func() DepreciableAsset { a := asset(); a.ID = ""; return a },
		"no life":       func() DepreciableAsset { a := asset(); a.UsefulLifeMonths = 0; return a },
		"no cost":       func() DepreciableAsset { a := asset(); a.Cost = money.Zero; return a },
		"negative cost": func() DepreciableAsset { a := asset(); a.Cost = money.FromInt(-1); return a },
		"residual above cost": func() DepreciableAsset {
			a := asset()
			a.Residual = money.FromInt(12_001)
			return a
		},
		"negative residual": func() DepreciableAsset {
			a := asset()
			a.Residual = money.FromInt(-1)
			return a
		},
		// Fractional đồng used to be rejected here. Since ADR 0008 an amount is
		// money.Dong — an integer — so a fractional asset cost cannot be
		// expressed, and the three cases that used to prove the check are gone
		// with the check itself.
	}

	for name, spoil := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := depreciationFor(spoil(), 1); !errors.Is(err, ErrInvalidProvision) {
				t.Fatalf("error = %v, want ErrInvalidProvision", err)
			}
		})
	}
}

// The provision is the stated percentage of each band: five per cent of the
// million that is more than ninety days late is fifty thousand.
func TestBadDebtCharges_ProvidesTheBandRateAgainstWhatIsInIt(t *testing.T) {
	asOf := januaryEnd
	late := Receivable{
		DocumentID: "inv-1",
		DueDate:    asOf.AddDate(0, 0, -120),
		Amount:     money.FromInt(1_000_000),
	}

	charges, err := badDebtCharges([]Receivable{late}, asOf, overduePolicy(5))
	if err != nil {
		t.Fatalf("badDebtCharges: %v", err)
	}

	if len(charges) != 1 {
		t.Fatalf("charges = %+v, want the overdue band alone", charges)
	}
	if charges[0].Band != "over 90 days" {
		t.Errorf("band = %q, want over 90 days", charges[0].Band)
	}
	if want := money.FromInt(1_000_000); charges[0].Outstanding.Cmp(want) != 0 {
		t.Errorf("outstanding = %s, want %s", charges[0].Outstanding, want)
	}
	if want := money.FromInt(50_000); charges[0].Amount.Cmp(want) != 0 {
		t.Errorf("provision = %s, want 5%% of %s = %s", charges[0].Amount, charges[0].Outstanding, want)
	}
}

// An amount is aged by whole days against the close's date, and one that is not
// yet due is aged as current rather than as negative days overdue.
func TestBadDebtCharges_AgesByWholeDaysAndFloorsAtZero(t *testing.T) {
	asOf := januaryEnd
	policy := DefaultBadDebtPolicy()

	cases := map[string]struct {
		due  time.Time
		band string
	}{
		"due today":        {asOf, "current"},
		"due tomorrow":     {asOf.AddDate(0, 0, 1), "current"},
		"no due date":      {time.Time{}, "current"},
		"one day late":     {asOf.AddDate(0, 0, -1), "1-30 days"},
		"thirty days late": {asOf.AddDate(0, 0, -30), "1-30 days"},
		"thirty-one days":  {asOf.AddDate(0, 0, -31), "31-60 days"},
		"ninety days late": {asOf.AddDate(0, 0, -90), "61-90 days"},
		"ninety-one days":  {asOf.AddDate(0, 0, -91), "over 90 days"},
		"two years old":    {asOf.AddDate(-2, 0, 0), "over 90 days"},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			charges, err := badDebtCharges(
				[]Receivable{{DocumentID: "inv-1", DueDate: tc.due, Amount: money.FromInt(500)}},
				asOf, policy,
			)
			if err != nil {
				t.Fatalf("badDebtCharges: %v", err)
			}
			if len(charges) != 1 || charges[0].Band != tc.band {
				t.Fatalf("charges = %+v, want %q alone", charges, tc.band)
			}
		})
	}
}

// A band nobody is in is not a provision of nothing, it is an absence of one,
// and an amount that has been settled or overpaid is not something to provide
// against.
func TestBadDebtCharges_LeavesOutWhatIsNotOwed(t *testing.T) {
	asOf := januaryEnd
	policy := overduePolicy(5)

	charges, err := badDebtCharges([]Receivable{
		{DocumentID: "paid", DueDate: asOf.AddDate(0, 0, -120), Amount: money.Zero},
		{DocumentID: "overpaid", DueDate: asOf.AddDate(0, 0, -120), Amount: money.FromInt(-50)},
		{DocumentID: "fresh", DueDate: asOf, Amount: money.FromInt(700)},
	}, asOf, policy)
	if err != nil {
		t.Fatalf("badDebtCharges: %v", err)
	}

	if len(charges) != 1 || charges[0].Band != "current" {
		t.Fatalf("charges = %+v, want the current band alone", charges)
	}
	if !charges[0].Amount.IsZero() {
		t.Errorf("provision against a band at no rate = %s, want 0", charges[0].Amount)
	}
}

func TestBadDebtPolicy_RefusesBandsThatOverlapOrLeaveTheYoungestOut(t *testing.T) {
	cases := map[string]BadDebtPolicy{
		"no bands": {},
		"youngest band starts late": {Bands: []AgingBand{
			{Name: "1-30", FromDays: 1},
		}},
		"bands out of order": {Bands: []AgingBand{
			{Name: "current", FromDays: 0},
			{Name: "over 90", FromDays: 91},
			{Name: "31-60", FromDays: 31},
		}},
		"two bands from the same day": {Bands: []AgingBand{
			{Name: "current", FromDays: 0},
			{Name: "late", FromDays: 0},
		}},
		"no name": {Bands: []AgingBand{
			{Name: "current", FromDays: 0},
			{FromDays: 91},
		}},
		"more than everything": {Bands: []AgingBand{
			{Name: "current", FromDays: 0},
			{Name: "over 90", FromDays: 91, Percent: money.FromInt(101)},
		}},
		"less than nothing": {Bands: []AgingBand{
			{Name: "current", FromDays: 0},
			{Name: "over 90", FromDays: 91, Percent: money.FromInt(-1)},
		}},
	}

	for name, policy := range cases {
		t.Run(name, func(t *testing.T) {
			if err := policy.Validate(); !errors.Is(err, ErrInvalidProvision) {
				t.Fatalf("error = %v, want ErrInvalidProvision", err)
			}
		})
	}

	for _, good := range []BadDebtPolicy{DefaultBadDebtPolicy(), Circular48BadDebtPolicy()} {
		if err := good.Validate(); err != nil {
			t.Errorf("a stated policy is refused: %v", err)
		}
	}
}

func TestProvisionAccounts_RefuseAnIncompleteOrSelfCancellingPair(t *testing.T) {
	accounts := TT133ProvisionAccounts()

	cases := map[string]ProvisionAccounts{
		"no depreciation expense": {AccumulatedDepreciation: "2141", BadDebtExpense: "6422", BadDebtProvision: "2293"},
		"no accumulation":         {DepreciationExpense: "6422", BadDebtExpense: "6422", BadDebtProvision: "2293"},
		"no bad debt expense":     {DepreciationExpense: "6422", AccumulatedDepreciation: "2141", BadDebtProvision: "2293"},
		"no bad debt provision":   {DepreciationExpense: "6422", AccumulatedDepreciation: "2141", BadDebtExpense: "6422"},
		"depreciation into itself": {
			DepreciationExpense: "2141", AccumulatedDepreciation: "2141",
			BadDebtExpense: "6422", BadDebtProvision: "2293",
		},
		"bad debt into itself": {
			DepreciationExpense: "6422", AccumulatedDepreciation: "2141",
			BadDebtExpense: "2293", BadDebtProvision: "2293",
		},
	}

	for name, spoilt := range cases {
		t.Run(name, func(t *testing.T) {
			if err := spoilt.Validate(); !errors.Is(err, ErrInvalidProvision) {
				t.Fatalf("error = %v, want ErrInvalidProvision", err)
			}
		})
	}

	if err := accounts.Validate(); err != nil {
		t.Fatalf("the TT133 provision pair is refused: %v", err)
	}
}

// The provision accounts are summaries the close writes and nothing else does:
// 214 and 229 are headings with sub-accounts, so the close names the leaves
// under them. A heading cannot carry a posting at all — the chart sees to that.
func TestTT133ProvisionAccounts_NameAccountsTheChartWillAcceptAPostingTo(t *testing.T) {
	chart := gl.NewChartOfAccounts()
	for _, account := range gl.TT133Chart() {
		if err := chart.Add(account); err != nil {
			t.Fatalf("seed chart with %s: %v", account.Code, err)
		}
	}

	accounts := TT133ProvisionAccounts()
	for _, code := range []string{
		accounts.DepreciationExpense,
		accounts.AccumulatedDepreciation,
		accounts.BadDebtExpense,
		accounts.BadDebtProvision,
	} {
		if !chart.IsPostable(code) {
			t.Errorf("the provision account %s cannot be posted to", code)
		}
	}

	for _, summary := range []string{"214", "229", "642"} {
		if chart.IsPostable(summary) {
			t.Errorf("the summary account %s accepts a posting, and it is a heading", summary)
		}
	}
}

// A close that depreciates an asset posts the charge against the asset's expense
// and accumulation accounts, and carries it into the period's result in the same
// voucher.
func TestClosePeriod_DepreciatesAnAssetForThePeriod(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t).trade()
	f.withProvisions(t, &testAssets{assets: []DepreciableAsset{asset()}}, &testReceivables{}, overduePolicy(5))

	report := f.closeIt(ctx, t)

	if len(report.Provisions.Depreciation) != 1 {
		t.Fatalf("provisions = %+v, want one depreciated asset", report.Provisions)
	}
	charge := report.Provisions.Depreciation[0]
	if want := money.FromInt(100); charge.Amount.Cmp(want) != 0 {
		t.Errorf("depreciation = %s, want %s for a 12,000 asset over 120 months", charge.Amount, want)
	}
	if want := money.FromInt(100); charge.Accumulated.Cmp(want) != 0 {
		t.Errorf("accumulated = %s, want %s", charge.Accumulated, want)
	}
	if charge.AssetID != "FA-001" || charge.Months != 1 {
		t.Errorf("charge = %+v, want FA-001 over one month", charge)
	}

	// The charge is an expense of the period: it is expensed, accumulated, and
	// then carried out of the profit and loss accounts like everything else.
	if got, want := f.ledger.balance("2141"), money.FromInt(-100); got.Cmp(want) != 0 {
		t.Errorf("accumulated depreciation = %s, want %s", got, want)
	}
	if got := f.ledger.balance("6422"); !got.IsZero() {
		t.Errorf("the depreciation expense stands at %s after the close, want 0", got)
	}
	if got, want := f.ledger.balance("4212"), money.FromInt(-399_900); got.Cmp(want) != 0 {
		t.Errorf("retained earnings = %s, want %s — the period's profit after depreciation", got, want)
	}
}

// A period that covers three months charges three monthly rates.
func TestClosePeriod_ChargesAMonthlyRateForEachMonthOfAPeriod(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t).trade()
	f.stretch(time.Date(2026, 3, 31, 0, 0, 0, 0, time.UTC))
	f.withProvisions(t, &testAssets{assets: []DepreciableAsset{asset()}}, &testReceivables{}, overduePolicy(5))

	report := f.closeIt(ctx, t)

	if want := money.FromInt(300); report.Provisions.DepreciationTotal.Cmp(want) != 0 {
		t.Errorf("depreciation over a quarter = %s, want %s", report.Provisions.DepreciationTotal, want)
	}
}

// Five per cent of what is more than ninety days late.
func TestClosePeriod_ProvidesAgainstOverdueReceivables(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t).trade()
	f.withProvisions(t, &testAssets{}, &testReceivables{receivables: []Receivable{{
		DocumentID: "inv-1",
		PartnerID:  "cust-1",
		DueDate:    januaryEnd.AddDate(0, 0, -120),
		Amount:     money.FromInt(1_000_000),
	}}}, overduePolicy(5))

	report := f.closeIt(ctx, t)

	if want := money.FromInt(50_000); report.Provisions.BadDebtTotal.Cmp(want) != 0 {
		t.Errorf("bad debt provision = %s, want 5%% of 1,000,000 = %s", report.Provisions.BadDebtTotal, want)
	}
	if len(report.Provisions.BadDebt) != 1 || report.Provisions.BadDebt[0].Band != "over 90 days" {
		t.Errorf("bad debt detail = %+v, want the overdue band alone", report.Provisions.BadDebt)
	}

	if got, want := f.ledger.balance("2293"), money.FromInt(-50_000); got.Cmp(want) != 0 {
		t.Errorf("the bad debt provision account = %s, want %s", got, want)
	}
	if got := f.ledger.balance("6422"); !got.IsZero() {
		t.Errorf("the bad debt expense stands at %s after the close, want 0", got)
	}
	if got, want := f.ledger.balance("4212"), money.FromInt(-350_000); got.Cmp(want) != 0 {
		t.Errorf("retained earnings = %s, want %s — the period's profit after the provision", got, want)
	}
}

// The provisions and the closing entries are one voucher, and therefore one
// transaction: a period cannot end up with a provision charged and its result
// not carried, or the other way round.
func TestClosePeriod_PostsProvisionsAndClosingEntriesInOneVoucher(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t).trade()
	f.withProvisions(t,
		&testAssets{assets: []DepreciableAsset{asset()}},
		&testReceivables{receivables: []Receivable{{
			DocumentID: "inv-1",
			DueDate:    januaryEnd.AddDate(0, 0, -120),
			Amount:     money.FromInt(1_000_000),
		}}},
		overduePolicy(5),
	)

	f.closeIt(ctx, t)

	if len(f.ledger.vouchers) != 1 {
		t.Fatalf("postings = %v, want one voucher carrying everything the close wrote", f.ledger.vouchers)
	}
	if f.ledger.vouchers[0] != closeVoucherID {
		t.Errorf("voucher = %s, want %s", f.ledger.vouchers[0], closeVoucherID)
	}

	voucher := f.ledger.voucher(closeVoucherID)
	assertLine(t, voucher, "2141", gl.SideCredit, 100)     // depreciation accumulated
	assertLine(t, voucher, "2293", gl.SideCredit, 50_000)  // bad debt provided
	assertLine(t, voucher, "4212", gl.SideCredit, 349_900) // and the result carried out
	assertLine(t, voucher, "5111", gl.SideDebit, 1_000_000)
	assertResultAccountEmpty(t, voucher)
}

// Every profit and loss account is still at zero after a close that provided:
// the provisions are part of the period's expenses, not a separate act.
func TestClosePeriod_LeavesEveryProfitAndLossAccountAtZeroAfterProviding(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t).trade()
	f.withProvisions(t,
		&testAssets{assets: []DepreciableAsset{asset()}},
		&testReceivables{receivables: []Receivable{{
			DocumentID: "inv-1",
			DueDate:    januaryEnd.AddDate(0, 0, -120),
			Amount:     money.FromInt(1_000_000),
		}}},
		overduePolicy(5),
	)

	f.closeIt(ctx, t)

	for _, code := range []string{"5111", "632", "6422", "911"} {
		if got := f.ledger.balance(code); !got.IsZero() {
			t.Errorf("account %s stands at %s after the close, want 0", code, got)
		}
	}

	balances, err := f.ledger.Balances(ctx, f.january)
	if err != nil {
		t.Fatalf("Balances: %v", err)
	}
	if len(balances) != 0 {
		t.Errorf("the period still carries profit and loss balances: %+v", balances)
	}
}

// An asset that has been written down produces no entry, and a period with
// nothing outstanding produces no bad debt entry either: the close is then a
// close and nothing else.
func TestClosePeriod_ProvidesNothingForAnAssetAlreadyWrittenDown(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t).trade()

	worn := asset()
	worn.Accumulated = worn.Cost
	f.withProvisions(t, &testAssets{assets: []DepreciableAsset{worn}}, &testReceivables{}, overduePolicy(5))

	report := f.closeIt(ctx, t)

	if len(report.Provisions.Depreciation) != 0 || !report.Provisions.Total.IsZero() {
		t.Errorf("provisions = %+v, want none", report.Provisions)
	}
	if got := f.ledger.balance("2141"); !got.IsZero() {
		t.Errorf("accumulated depreciation = %s, want 0", got)
	}
}

// The provision accounts are summaries: a heading cannot carry a posting, so a
// close configured with 214 rather than 2141 is refused — and refused before
// anything is written, because the chart is read before any entry is built.
func TestClosePeriod_RefusesASummaryAccountItCannotPostTo(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t).trade()

	accounts := TT133ProvisionAccounts()
	accounts.AccumulatedDepreciation = "214"

	f.configureProvisions(t, Provisions{
		Assets:      &testAssets{assets: []DepreciableAsset{asset()}},
		Receivables: &testReceivables{},
		Charts:      testCharts{chart: f.ledger.rules.Chart},
		Accounts:    accounts,
		BadDebt:     overduePolicy(5),
	})

	_, err := f.service.ClosePeriod(ctx, f.command())
	if !errors.Is(err, ErrInvalidProvision) {
		t.Fatalf("error = %v, want ErrInvalidProvision", err)
	}
	if !strings.Contains(err.Error(), "214") || !strings.Contains(err.Error(), "posting to") {
		t.Errorf("error = %v, want it to say 214 will not take a posting", err)
	}
	if f.periods.find(f.january.ID).Locked {
		t.Error("the period was locked although the provisions could not be posted")
	}
	if _, err := f.reports.Get(ctx, f.january.ID); !errors.Is(err, ErrNoReport) {
		t.Errorf("a report was stored for a close that never happened: %v", err)
	}
}

func TestWithProvisions_RefusesACalculationThatCouldNotBeMade(t *testing.T) {
	f := newFixture(t)
	charts := testCharts{chart: f.ledger.rules.Chart}

	cases := map[string]Provisions{
		"no assets": {
			Receivables: &testReceivables{},
			Charts:      charts,
			Accounts:    TT133ProvisionAccounts(),
			BadDebt:     DefaultBadDebtPolicy(),
		},
		"no receivables": {
			Assets:   &testAssets{},
			Charts:   charts,
			Accounts: TT133ProvisionAccounts(),
			BadDebt:  DefaultBadDebtPolicy(),
		},
		"no chart": {
			Assets:      &testAssets{},
			Receivables: &testReceivables{},
			Accounts:    TT133ProvisionAccounts(),
			BadDebt:     DefaultBadDebtPolicy(),
		},
		"no bands": {
			Assets:      &testAssets{},
			Receivables: &testReceivables{},
			Charts:      charts,
			Accounts:    TT133ProvisionAccounts(),
		},
		"accounts that name nothing": {
			Assets:      &testAssets{},
			Receivables: &testReceivables{},
			Charts:      charts,
			BadDebt:     DefaultBadDebtPolicy(),
		},
		"a provision through the result account": {
			Assets:      &testAssets{},
			Receivables: &testReceivables{},
			Charts:      charts,
			Accounts: ProvisionAccounts{
				DepreciationExpense: "911", AccumulatedDepreciation: "2141",
				BadDebtExpense: "6422", BadDebtProvision: "2293",
			},
			BadDebt: DefaultBadDebtPolicy(),
		},
		"a provision into retained earnings": {
			Assets:      &testAssets{},
			Receivables: &testReceivables{},
			Accounts: ProvisionAccounts{
				DepreciationExpense: "6422", AccumulatedDepreciation: "4212",
				BadDebtExpense: "6422", BadDebtProvision: "2293",
			},
			BadDebt: DefaultBadDebtPolicy(),
		},
	}

	for name, provisions := range cases {
		t.Run(name, func(t *testing.T) {
			service, err := f.service.WithProvisions(provisions)
			if err == nil {
				t.Fatalf("a calculation that could not be made was accepted: %+v", service)
			}
			if !errors.Is(err, ErrInvalidProvision) {
				t.Fatalf("error = %v, want ErrInvalidProvision", err)
			}
		})
	}
}

// A close with no provision calculation is exactly what it was before there was
// one: it provides for nothing and posts one voucher of closing entries.
func TestClosePeriod_ProvidesNothingWhenNoCalculationIsConfigured(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t).trade()

	report := f.closeIt(ctx, t)

	if len(report.Provisions.Depreciation) != 0 || len(report.Provisions.BadDebt) != 0 ||
		!report.Provisions.Total.IsZero() {
		t.Errorf("provisions = %+v, want none", report.Provisions)
	}
	if len(f.ledger.vouchers) != 1 {
		t.Errorf("postings = %v, want the one closing voucher", f.ledger.vouchers)
	}
}

// A provision that names an account the period carries as something other than
// an expense is refused: the close would clear that account twice and leave the
// difference standing.
func TestClosePeriod_RefusesAProvisionIntoAnAccountThatIsNotAnExpense(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t).trade()

	accounts := TT133ProvisionAccounts()
	accounts.DepreciationExpense = "5111" // a revenue account, and one the period has earned from

	f.configureProvisions(t, Provisions{
		Assets:      &testAssets{assets: []DepreciableAsset{asset()}},
		Receivables: &testReceivables{},
		Charts:      testCharts{chart: f.ledger.rules.Chart},
		Accounts:    accounts,
		BadDebt:     overduePolicy(5),
	})

	if _, err := f.service.ClosePeriod(ctx, f.command()); !errors.Is(err, ErrInvalidProvision) {
		t.Fatalf("error = %v, want ErrInvalidProvision", err)
	}
	if f.ledger.posts != 0 {
		t.Errorf("the ledger holds %d posting(s), want none", f.ledger.posts)
	}
	if got, want := f.ledger.balance("5111"), money.FromInt(-1_000_000); got.Cmp(want) != 0 {
		t.Errorf("revenue = %s, want it untouched at %s", got, want)
	}
}

// An asset names its own accounts, so each asset's accounts are checked in their
// own right. Accumulating into a profit and loss account would credit an account
// the close does not clear, and the period would close with it standing at
// something other than nothing.
func TestClosePeriod_RefusesAnAssetThatAccumulatesIntoAProfitAndLossAccount(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t).trade()

	van := asset()
	van.AccumulationAccount = "6421" // a selling cost, not an accumulation account

	f.withProvisions(t, &testAssets{assets: []DepreciableAsset{van}}, &testReceivables{}, overduePolicy(5))

	_, err := f.service.ClosePeriod(ctx, f.command())
	if !errors.Is(err, ErrInvalidProvision) {
		t.Fatalf("error = %v, want ErrInvalidProvision", err)
	}
	if got := f.ledger.balance("6421"); !got.IsZero() {
		t.Errorf("6421 stands at %s, want nothing: the close refused before posting", got)
	}
	if f.periods.find(f.january.ID).Locked {
		t.Error("the period was locked although the provisions were refused")
	}
}

// An asset accumulating into retained earnings would write its depreciation
// straight into equity: the period's result would be right by accident and the
// account it belongs in would be wrong for good.
func TestClosePeriod_RefusesAnAssetThatAccumulatesIntoRetainedEarnings(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t).trade()

	van := asset()
	van.AccumulationAccount = "4212"

	f.withProvisions(t, &testAssets{assets: []DepreciableAsset{van}}, &testReceivables{}, overduePolicy(5))

	if _, err := f.service.ClosePeriod(ctx, f.command()); !errors.Is(err, ErrInvalidProvision) {
		t.Fatalf("error = %v, want ErrInvalidProvision", err)
	}
	if got := f.ledger.balance("4212"); !got.IsZero() {
		t.Errorf("retained earnings = %s, want nothing: the close refused before posting", got)
	}
}

// An asset that expenses and accumulates into one account moves nothing, and
// would be reported as having provided for something it did not.
func TestClosePeriod_RefusesAnAssetThatExpensesIntoItsOwnAccumulationAccount(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t).trade()

	van := asset()
	van.ExpenseAccount = "2141"
	van.AccumulationAccount = "2141"

	f.withProvisions(t, &testAssets{assets: []DepreciableAsset{van}}, &testReceivables{}, overduePolicy(5))

	if _, err := f.service.ClosePeriod(ctx, f.command()); !errors.Is(err, ErrInvalidProvision) {
		t.Fatalf("error = %v, want ErrInvalidProvision", err)
	}
	if got := f.ledger.balance("2141"); !got.IsZero() {
		t.Errorf("2141 stands at %s, want nothing", got)
	}
}

// A close that provided and then failed to lock is run again: the assets must
// not be depreciated a second time, and the snapshot of the provisions must
// survive the retry.
func TestClosePeriod_CanBeRetriedAfterTheLockFailsWithProvisions(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t).trade()
	f.withProvisions(t, &testAssets{assets: []DepreciableAsset{asset()}}, &testReceivables{}, overduePolicy(5))
	f.periods.failLock = errors.New("the database went away")

	if _, err := f.service.ClosePeriod(ctx, f.command()); err == nil {
		t.Fatal("the close reported success though the period was never locked")
	}

	report := f.closeIt(ctx, t)

	if len(f.ledger.vouchers) != 1 {
		t.Errorf("postings = %v, want the one voucher the first attempt posted", f.ledger.vouchers)
	}
	if got, want := f.ledger.balance("2141"), money.FromInt(-100); got.Cmp(want) != 0 {
		t.Errorf("accumulated depreciation = %s, want %s — depreciated once, not twice", got, want)
	}
	if got, want := f.ledger.balance("4212"), money.FromInt(-399_900); got.Cmp(want) != 0 {
		t.Errorf("retained earnings = %s, want %s", got, want)
	}
	if want := money.FromInt(100); report.Provisions.DepreciationTotal.Cmp(want) != 0 {
		t.Errorf("the retried close reports %s of depreciation, want the %s that was charged",
			report.Provisions.DepreciationTotal, want)
	}
	if !f.periods.find(f.january.ID).Locked {
		t.Error("the retry did not lock the period")
	}
}

// An asset may name its own accounts — a delivery van is a selling cost and a
// server is an administration one — and the close falls back to the provision
// accounts only for the assets that name none.
func TestClosePeriod_DepreciatesAnAssetIntoTheAccountsItNames(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	f.ledger.seed("5111", gl.SideCredit, 1_000_000, time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC))

	van := asset()
	van.ID = "FA-VAN"
	van.Name = "Delivery van"
	van.ExpenseAccount = "6421"      // chi phí bán hàng, not the 6422 default
	van.AccumulationAccount = "2142" // and its own accumulation account

	f.withProvisions(t, &testAssets{assets: []DepreciableAsset{van}}, &testReceivables{}, overduePolicy(5))

	f.closeIt(ctx, t)

	voucher := f.ledger.voucher(closeVoucherID)
	assertLine(t, voucher, "6421", gl.SideDebit, 100)
	assertLine(t, voucher, "2142", gl.SideCredit, 100)

	if amount, found := lineOn(voucher, "6422", gl.SideDebit); found {
		t.Errorf("the voucher expenses %s to 6422, want nothing: the asset named its own account", amount)
	}

	if got, want := f.ledger.balance("2142"), money.FromInt(-100); got.Cmp(want) != 0 {
		t.Errorf("accumulated depreciation = %s, want %s", got, want)
	}
	if got := f.ledger.balance("6421"); !got.IsZero() {
		t.Errorf("the account the asset named stands at %s after the close, want 0", got)
	}
	if got := f.ledger.balance("6422"); !got.IsZero() {
		t.Errorf("6422 stands at %s, want 0 — the asset named its own expense account", got)
	}
}

// A provision expensed to an account the period has already spent from adds to
// what that account stands at. Carried separately it would be two rows for one
// account, and the closing entries would clear it twice.
func TestClosePeriod_AddsAProvisionToAnAccountThePeriodAlreadyUsed(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	f.ledger.seed("5111", gl.SideCredit, 1_000_000, time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC))
	f.ledger.seed("6422", gl.SideDebit, 500, time.Date(2026, 1, 16, 0, 0, 0, 0, time.UTC))
	f.withProvisions(t, &testAssets{assets: []DepreciableAsset{asset()}}, &testReceivables{}, overduePolicy(5))

	report := f.closeIt(ctx, t)

	// 500 of administration expense, and 100 of depreciation into the same
	// account: the period spent 600, and that is what it carries.
	if want := money.FromInt(999_400); report.NetProfit.Cmp(want) != 0 {
		t.Errorf("net profit = %s, want %s", report.NetProfit, want)
	}
	if got := f.ledger.balance("6422"); !got.IsZero() {
		t.Errorf("the expense account stands at %s after the close, want 0", got)
	}
	if got, want := f.ledger.balance("2141"), money.FromInt(-100); got.Cmp(want) != 0 {
		t.Errorf("accumulated depreciation = %s, want %s", got, want)
	}

	rows := 0
	for _, expense := range report.Expenses {
		if expense.AccountCode != "6422" {
			continue
		}

		rows++
		if want := money.FromInt(600); expense.Amount.Cmp(want) != 0 {
			t.Errorf("6422 in the snapshot = %s, want %s", expense.Amount, want)
		}
	}
	if rows != 1 {
		t.Errorf("the snapshot holds 6422 %d times, want once", rows)
	}
}

// The report keeps the breakdown the entries cannot: which asset, over how many
// months, and which band at what rate.
func TestClosePeriod_RecordsTheProvisionBreakdownOnTheReport(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t).trade()
	f.withProvisions(t,
		&testAssets{assets: []DepreciableAsset{asset()}},
		&testReceivables{receivables: []Receivable{{
			DocumentID: "inv-1",
			DueDate:    januaryEnd.AddDate(0, 0, -120),
			Amount:     money.FromInt(1_000_000),
		}}},
		overduePolicy(5),
	)

	f.closeIt(ctx, t)

	recorded, err := f.service.Report(ctx, f.january.ID)
	if err != nil {
		t.Fatalf("Report: %v", err)
	}

	if want := money.FromInt(50_100); recorded.Provisions.Total.Cmp(want) != 0 {
		t.Errorf("provisions totalled = %s, want 100 of depreciation and 50,000 of bad debt = %s",
			recorded.Provisions.Total, want)
	}
	if got := recorded.Provisions.Depreciation[0].Name; got != "Laptop" {
		t.Errorf("depreciated asset = %q, want Laptop", got)
	}
	if got := recorded.Provisions.BadDebt[0].Percent; got.Cmp(money.FromInt(5)) != 0 {
		t.Errorf("band rate = %s%%, want 5%%", got)
	}

	// The provisions are part of the period's expenses, so the snapshot of the
	// period shows them, once, alongside everything else it spent.
	rows := 0
	for _, expense := range recorded.Expenses {
		if expense.AccountCode != "6422" {
			continue
		}

		rows++
		if want := money.FromInt(50_100); expense.Amount.Cmp(want) != 0 {
			t.Errorf("6422 in the snapshot = %s, want %s", expense.Amount, want)
		}
	}
	if rows != 1 {
		t.Errorf("the snapshot's expenses = %+v, want 6422 among them once", recorded.Expenses)
	}
}

// The default accounts are checked the same way an asset's own are. An
// accumulated depreciation account pointing at an expense account leaves that
// account standing after the close, exactly as the per-asset form does.
func TestClosePeriod_RefusesADefaultAccumulationAccountThatIsAProfitAndLossAccount(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t).trade()

	accounts := TT133ProvisionAccounts()
	accounts.AccumulatedDepreciation = "6421"

	f.configureProvisions(t, Provisions{
		Assets:      &testAssets{assets: []DepreciableAsset{asset()}},
		Receivables: &testReceivables{},
		Charts:      testCharts{chart: f.ledger.rules.Chart},
		Accounts:    accounts,
		BadDebt:     overduePolicy(5),
	})

	if _, err := f.service.ClosePeriod(ctx, f.command()); !errors.Is(err, ErrInvalidProvision) {
		t.Fatalf("error = %v, want ErrInvalidProvision", err)
	}
	if got := f.ledger.balance("6421"); !got.IsZero() {
		t.Errorf("6421 stands at %s, want nothing", got)
	}
}

// Depreciation is a monthly charge and a period is a stretch of dates, so the
// number of calendar months the period touches is what the rate is multiplied
// by.
func TestMonthsSpanned_CountsTheCalendarMonthsAPeriodTouches(t *testing.T) {
	day := func(year int, month time.Month, day int) time.Time {
		return time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
	}

	cases := map[string]struct {
		from, to time.Time
		want     int
	}{
		"one month":                      {day(2026, 1, 1), day(2026, 1, 31), 1},
		"a quarter":                      {day(2026, 1, 1), day(2026, 3, 31), 3},
		"across the year":                {day(2025, 11, 1), day(2026, 2, 28), 4},
		"a single day":                   {day(2026, 1, 15), day(2026, 1, 15), 1},
		"the whole of a year":            {day(2026, 1, 1), day(2026, 12, 31), 12},
		"a period that has it backwards": {day(2026, 3, 1), day(2026, 1, 1), 1},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			if got := monthsSpanned(tc.from, tc.to); got != tc.want {
				t.Errorf("monthsSpanned(%s, %s) = %d, want %d",
					tc.from.Format(time.DateOnly), tc.to.Format(time.DateOnly), got, tc.want)
			}
		})
	}
}

// An amount is aged by whole days in UTC, which is the instant the ledger keeps
// and the day internal/period compares: a due date that arrives in another zone
// is read where the books are.
func TestDaysOverdue_CountsWholeDaysInUTC(t *testing.T) {
	asOf := time.Date(2026, 1, 31, 0, 0, 0, 0, time.UTC)

	cases := map[string]struct {
		due  time.Time
		want int
	}{
		"midnight UTC": {time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), 30},
		"the same instant in another zone": {
			time.Date(2026, 1, 1, 7, 0, 0, 0, time.FixedZone("+07", 7*3600)), 30,
		},
		"late in the closing day": {
			time.Date(2026, 1, 31, 23, 0, 0, 0, time.UTC), 0,
		},
		"not yet due": {time.Date(2026, 2, 5, 0, 0, 0, 0, time.UTC), 0},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			if got := daysOverdue(tc.due, asOf); got != tc.want {
				t.Errorf("daysOverdue(%s, %s) = %d, want %d",
					tc.due, asOf.Format(time.DateOnly), got, tc.want)
			}
		})
	}
}

// The statutory schedule, band by band: what the whole aging policy is for is
// the boundary between one rate and the next.
func TestCircular48BadDebtPolicy_ProvidesByHowLateAnAmountIs(t *testing.T) {
	asOf := januaryEnd
	policy := Circular48BadDebtPolicy()

	cases := map[string]struct {
		daysLate int
		percent  int64
	}{
		"a month late":            {30, 0},
		"the last day at nothing": {180, 0},
		"the first day at thirty": {181, 30},
		"under a year":            {365, 30},
		"a year in":               {366, 50},
		"under two years":         {730, 50},
		"two years in":            {731, 70},
		"under three years":       {1095, 70},
		"three years in":          {1096, 90},
		"under four years":        {1460, 90},
		"four years in":           {1461, 100},
		"a decade late":           {3650, 100},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			charges, err := badDebtCharges([]Receivable{{
				DocumentID: "inv-1",
				DueDate:    asOf.AddDate(0, 0, -tc.daysLate),
				Amount:     money.FromInt(1_000_000),
			}}, asOf, policy)
			if err != nil {
				t.Fatalf("badDebtCharges: %v", err)
			}
			if len(charges) != 1 {
				t.Fatalf("charges = %+v, want one band", charges)
			}

			want := money.FromInt(1_000_000).Mul(money.FromInt(tc.percent)).Div(hundred)
			if charges[0].Amount.Cmp(want) != 0 {
				t.Errorf("%d days late provides %s, want %s at %d per cent",
					tc.daysLate, charges[0].Amount, want, tc.percent)
			}
			if charges[0].Percent.Cmp(money.FromInt(tc.percent)) != 0 {
				t.Errorf("%d days late is banded at %s%%, want %d%%",
					tc.daysLate, charges[0].Percent, tc.percent)
			}
		})
	}
}

// A close says why it could not provide: an asset register that is down is not
// the same as a policy nobody configured, and neither is "no provisions".
func TestClosePeriod_ReportsWhyItCouldNotProvide(t *testing.T) {
	ctx := context.Background()

	cases := map[string]struct {
		assets      Assets
		receivables Receivables
		want        string
	}{
		"the assets cannot be read": {
			assets:      &testAssets{err: errors.New("the asset register is down")},
			receivables: &testReceivables{},
			want:        "asset register is down",
		},
		"the receivables cannot be read": {
			assets:      &testAssets{},
			receivables: &testReceivables{err: errors.New("the AR book is locked")},
			want:        "AR book is locked",
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			f := newFixture(t).trade()

			f.configureProvisions(t, Provisions{
				Assets:      tc.assets,
				Receivables: tc.receivables,
				Charts:      testCharts{chart: f.ledger.rules.Chart},
				Accounts:    TT133ProvisionAccounts(),
				BadDebt:     overduePolicy(5),
			})

			_, err := f.service.ClosePeriod(ctx, f.command())
			if err == nil {
				t.Fatal("the close provided although it could not read what it provides for")
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error = %v, want it to mention %q", err, tc.want)
			}
			if f.ledger.posts != 0 {
				t.Errorf("the ledger holds %d posting(s), want none", f.ledger.posts)
			}
		})
	}
}

// The chart is asked for its accounts once per close, and a chart that cannot be
// read stops the close before anything is written.
func TestClosePeriod_ReportsAChartThatCouldNotBeRead(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t).trade()

	f.configureProvisions(t, Provisions{
		Assets:      &testAssets{assets: []DepreciableAsset{asset()}},
		Receivables: &testReceivables{},
		Charts:      testCharts{err: errors.New("the chart is unreadable")},
		Accounts:    TT133ProvisionAccounts(),
		BadDebt:     overduePolicy(5),
	})

	_, err := f.service.ClosePeriod(ctx, f.command())
	if err == nil || !strings.Contains(err.Error(), "chart is unreadable") {
		t.Fatalf("error = %v, want the chart's own failure", err)
	}
	if _, err := f.reports.Get(ctx, f.january.ID); !errors.Is(err, ErrNoReport) {
		t.Errorf("a report was stored for a close that never ran: %v", err)
	}
}

// A close keeps its own copy of the policy it was configured with: a caller who
// changes the bands afterwards does not change what the close provides.
func TestWithProvisions_KeepsItsOwnCopyOfThePolicy(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t).trade()

	policy := overduePolicy(5)
	f.withProvisions(t, &testAssets{assets: []DepreciableAsset{asset()}}, &testReceivables{receivables: []Receivable{{
		DocumentID: "inv-1",
		DueDate:    januaryEnd.AddDate(0, 0, -120),
		Amount:     money.FromInt(1_000_000),
	}}}, policy)

	// The caller's copy moves under the close's feet.
	policy.Bands[len(policy.Bands)-1].Percent = money.FromInt(50)

	report := f.closeIt(ctx, t)

	if want := money.FromInt(50_000); report.Provisions.BadDebtTotal.Cmp(want) != 0 {
		t.Errorf("bad debt provision = %s, want the %s the close was configured with",
			report.Provisions.BadDebtTotal, want)
	}
}

// The third review item: the pair check is one rule with several ways to break
// it, and each of them leaves the period's profit and loss accounts standing at
// something after the close says they are clear.
func TestCheckProvisionPair_RefusesWhatACloseMustNotPostTo(t *testing.T) {
	config := provisionConfig{
		accounts: TT133ProvisionAccounts(),
		badDebt:  DefaultBadDebtPolicy(),
		closing:  TT133ClosingAccounts(),
		chart:    tt133Chart(t),
	}

	cases := map[string]struct{ expense, accumulation, want string }{
		"no expense account":                     {"", "2141", "posts to no account"},
		"no accumulation account":                {"6422", "", "posts to no account"},
		"one account for both":                   {"2141", "2141", "moves nothing"},
		"the expense is the result account":      {"911", "2141", "period's result"},
		"the accumulation is the result account": {"6422", "911", "period's result"},
		"the expense is retained earnings":       {"4212", "2141", "period's result"},
		"into retained earnings":                 {"6422", "4212", "period's result"},
		"an account the chart does not hold":     {"9999", "2141", "posting to"},
		"a heading as the expense":               {"642", "2141", "posting to"},
		"a heading as the accumulation":          {"6422", "214", "posting to"},
		"an asset as the expense":                {"2141", "2142", "ASSET"},
		"a revenue account as the accumulation":  {"6422", "5111", "REVENUE"},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			err := checkProvisionPair(tc.expense, tc.accumulation, "the depreciation of FA-001", config)
			if !errors.Is(err, ErrInvalidProvision) {
				t.Fatalf("error = %v, want ErrInvalidProvision", err)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error = %v, want it to mention %q", err, tc.want)
			}
		})
	}

	if err := checkProvisionPair("6422", "2141", "the depreciation of FA-001", config); err != nil {
		t.Errorf("the TT133 pair is refused: %v", err)
	}

	// Without a chart there is nothing to check the accounts against, and the
	// pair is taken as given.
	config.chart = nil
	if err := checkProvisionPair("6422", "2141", "the depreciation of FA-001", config); err != nil {
		t.Errorf("a pair with no chart to check it against is refused: %v", err)
	}
}

// The close clears the accounts the balances port reports, and a provision
// expenses into one of them. Where the two disagree about what an account is,
// the close refuses rather than clearing half of it.
func TestWithProvisions_RefusesAnAccountTheBooksCarryAsSomethingElse(t *testing.T) {
	books := []AccountBalance{{AccountCode: "6422", Type: gl.AccountTypeAsset, Amount: money.FromInt(50)}}
	expenses := []AccountBalance{{AccountCode: "6422", Type: gl.AccountTypeExpense, Amount: money.FromInt(100)}}

	if _, err := withProvisions(books, expenses); !errors.Is(err, ErrInvalidProvision) {
		t.Fatalf("error = %v, want ErrInvalidProvision", err)
	}

	// Where they agree, the provision adds to what the account stands at
	// instead of appearing beside it as a second row for one account.
	folded, err := withProvisions(
		[]AccountBalance{{AccountCode: "6422", Type: gl.AccountTypeExpense, Amount: money.FromInt(50)}},
		expenses,
	)
	if err != nil {
		t.Fatalf("withProvisions: %v", err)
	}
	if len(folded) != 1 {
		t.Fatalf("folded balances = %+v, want one row", folded)
	}
	if want := money.FromInt(150); folded[0].Amount.Cmp(want) != 0 {
		t.Errorf("the folded account = %s, want %s", folded[0].Amount, want)
	}
}

// A ledger note names an asset by what a bookkeeper would recognise it by, and
// falls back to its id when it has no name.
func TestDescribe_NamesAnAssetByItsNameOrItsID(t *testing.T) {
	if got := describe(asset()); got != "Laptop" {
		t.Errorf("describe = %q, want Laptop", got)
	}

	unnamed := asset()
	unnamed.Name = ""
	if got := describe(unnamed); got != "FA-001" {
		t.Errorf("describe = %q, want FA-001", got)
	}
}

// tt133Chart is the chart of accounts as a close reads it.
func tt133Chart(t *testing.T) *gl.ChartOfAccounts {
	t.Helper()

	chart := gl.NewChartOfAccounts()
	for _, account := range gl.TT133Chart() {
		if err := chart.Add(account); err != nil {
			t.Fatalf("seed chart with %s: %v", account.Code, err)
		}
	}

	return chart
}

// A stored snapshot is a snapshot: neither what a caller held when it wrote one,
// nor what a caller is handed back, can change it.
func TestMemoryReports_KeepsTheSnapshotItWasGiven(t *testing.T) {
	ctx := context.Background()
	reports := NewMemoryReports()

	report := Report{
		PeriodID: "2026-01",
		Revenues: []AccountBalance{{AccountCode: "5111"}},
		Expenses: []AccountBalance{{AccountCode: "632"}},
		Provisions: ProvisionSummary{
			Depreciation: []DepreciationCharge{{AssetID: "FA-001", Name: "Laptop"}},
			BadDebt:      []BadDebtCharge{{Band: "over 90 days", Percent: money.FromInt(5)}},
		},
	}
	if err := reports.Save(ctx, report); err != nil {
		t.Fatalf("Save: %v", err)
	}

	// What the caller still holds is not what was stored.
	report.Revenues[0].AccountCode = "mutated"
	report.Provisions.Depreciation[0].AssetID = "mutated"
	report.Provisions.BadDebt[0].Band = "mutated"

	got, err := reports.Get(ctx, "2026-01")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	assertSnapshotIntact(t, got)

	// And what a reader is handed is not either.
	got.Provisions.Depreciation[0].AssetID = "mutated"
	got.Provisions.BadDebt[0].Band = "mutated"
	got.Revenues[0].AccountCode = "mutated"

	again, err := reports.Get(ctx, "2026-01")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	assertSnapshotIntact(t, again)
}

func assertSnapshotIntact(t *testing.T, report Report) {
	t.Helper()

	if report.Revenues[0].AccountCode != "5111" {
		t.Errorf("revenue in the snapshot = %s, want 5111", report.Revenues[0].AccountCode)
	}
	if report.Provisions.Depreciation[0].AssetID != "FA-001" {
		t.Errorf("depreciated asset in the snapshot = %s, want FA-001", report.Provisions.Depreciation[0].AssetID)
	}
	if report.Provisions.BadDebt[0].Band != "over 90 days" {
		t.Errorf("band in the snapshot = %s, want over 90 days", report.Provisions.BadDebt[0].Band)
	}
}

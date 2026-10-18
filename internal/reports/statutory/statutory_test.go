package statutory

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"fingo/internal/domain/gl"
	"fingo/internal/money"
	"fingo/internal/reports"
	"fingo/internal/vat"
	"fingo/pkg/logger"
)

var (
	januaryStart = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	januaryEnd   = time.Date(2026, 1, 31, 0, 0, 0, 0, time.UTC)
	generatedAt  = time.Date(2026, 2, 1, 9, 0, 0, 0, time.UTC)
)

// testLedger is the reports a filing is computed from, stated outright.
//
// It is a fixture and not a book: internal/reports sums the lines, and what this
// package reads is the three reports that come out of it. Stating them here
// keeps a failure in this package distinguishable from a failure in that one.
type testLedger struct {
	trial  reports.TrialBalance
	profit reports.ProfitAndLoss
	sheet  reports.BalanceSheet
	err    error
}

func (l *testLedger) TrialBalance(context.Context, string) (reports.TrialBalance, error) {
	if l.err != nil {
		return reports.TrialBalance{}, l.err
	}

	return l.trial, nil
}

func (l *testLedger) ProfitAndLoss(context.Context, string) (reports.ProfitAndLoss, error) {
	if l.err != nil {
		return reports.ProfitAndLoss{}, l.err
	}

	return l.profit, nil
}

func (l *testLedger) BalanceSheet(context.Context, string) (reports.BalanceSheet, error) {
	if l.err != nil {
		return reports.BalanceSheet{}, l.err
	}

	return l.sheet, nil
}

// testFacts is what the ledger does not hold.
type testFacts struct {
	pots     VATPots
	turnover money.Dong
	err      error
}

func (f *testFacts) VATPots(context.Context, ReportingPeriod) (VATPots, error) {
	if f.err != nil {
		return VATPots{}, f.err
	}

	return f.pots, nil
}

func (f *testFacts) YearTurnover(context.Context, ReportingPeriod) (money.Dong, error) {
	if f.err != nil {
		return money.Zero, f.err
	}

	return f.turnover, nil
}

// dong builds an amount; the helper is named dong rather than money so it does
// not shadow the money package at every call site.
func dong(amount int64) money.Dong { return money.FromInt(amount) }

// fixture is a service over a month that exercises every statement: VAT at both
// rates and in the 8% window, a profit, input VAT carried in, and corporate tax
// paid in part.
type fixture struct {
	service *Service
	facts   *testFacts
	log     *bytes.Buffer
}

func newFixture(t *testing.T) *fixture {
	t.Helper()

	ledger := &testLedger{
		trial: reports.TrialBalance{
			PeriodID:  "2026-01",
			StartDate: januaryStart,
			EndDate:   januaryEnd,
			Rows: []reports.TrialBalanceRow{
				// Carried in from December: input VAT not yet credited.
				{AccountCode: accountInputVAT, AccountName: "Thuế GTGT được khấu trừ", OpeningDebit: dong(5_000_000), PeriodDebit: dong(20_000_000)},
				// The period's output VAT, as the books recorded it: the tax on
				// every band below and nothing else, so that the filing's own
				// reconciliation is clean.
				{AccountCode: accountOutputVAT, AccountName: "Thuế GTGT đầu ra", PeriodCredit: dong(49_000_000)},
				// Corporate income tax paid during the year.
				{AccountCode: accountCITPayable, AccountName: "Thuế thu nhập doanh nghiệp", PeriodDebit: dong(5_000_000)},
			},
			OpeningDebit: dong(5_000_000), OpeningCredit: dong(5_000_000),
			PeriodDebit: dong(25_000_000), PeriodCredit: dong(54_000_000),
			ClosingDebit: dong(25_000_000), ClosingCredit: dong(54_000_000),
		},
		profit: reports.ProfitAndLoss{
			PeriodID: "2026-01",
			// The bands below add up to this: 100m at 5%, 400m at 10% and 50m in
			// the 8% window.
			TotalRevenue:  dong(550_000_000),
			TotalExpenses: dong(300_000_000),
			NetProfit:     dong(250_000_000),
		},
		sheet: reports.BalanceSheet{
			PeriodID:         "2026-01",
			AsOf:             januaryEnd,
			TotalAssets:      dong(700_000_000),
			TotalLiabilities: dong(200_000_000),
			TotalEquity:      dong(500_000_000),
		},
	}

	facts := &testFacts{
		pots: VATPots{
			ByRate: []VATBand{
				{Rate: rateFivePercent, Net: dong(100_000_000), Tax: dong(5_000_000)},
				{Rate: rateTenPercent, Net: dong(400_000_000), Tax: dong(40_000_000)},
				{Rate: rateEightPercent, Net: dong(50_000_000), Tax: dong(4_000_000)},
			},
		},
		// Above 3bn and below 50bn: the 17% band.
		turnover: dong(4_000_000_000),
	}

	held := &bytes.Buffer{}
	service, err := NewService(Config{
		Ledger: ledger,
		Facts:  facts,
		Regime: RegimeTT133,
		Log:    logger.New(logger.Config{Level: slog.LevelWarn, Writer: held, Format: logger.FormatText}),
		Now:    func() time.Time { return generatedAt },
	})
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}

	return &fixture{service: service, facts: facts, log: held}
}

func (f *fixture) generate(t *testing.T) Output {
	t.Helper()

	out, err := f.service.Generate(context.Background(), "2026-01")
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	return out
}

// ledger is the fixture's book, typed, so a test can change what the books hold.
func (f *fixture) ledger() *testLedger {
	return f.service.ledger.(*testLedger)
}

// statement returns one statement of the output.
func statement(t *testing.T, out Output, code string) Statement {
	t.Helper()

	for _, held := range out.Statements {
		if held.Code == code {
			return held
		}
	}

	t.Fatalf("the filing holds no %s: %+v", code, codes(out))

	return Statement{}
}

func codes(out Output) []string {
	held := make([]string, 0, len(out.Statements))
	for _, one := range out.Statements {
		held = append(held, one.Code)
	}

	return held
}

// line returns one line of a statement by its indicator, or the first line whose
// label contains a phrase when there is no indicator.
func line(t *testing.T, held Statement, indicator string) LineItem {
	t.Helper()

	for _, item := range held.Lines {
		if item.Indicator == indicator {
			return item
		}
	}

	t.Fatalf("%s holds no line %q: %+v", held.Code, indicator, held.Lines)

	return LineItem{}
}

func lineWith(t *testing.T, held Statement, phrase string) LineItem {
	t.Helper()

	for _, item := range held.Lines {
		if strings.Contains(item.Label, phrase) {
			return item
		}
	}

	t.Fatalf("%s holds no line containing %q", held.Code, phrase)

	return LineItem{}
}

func amount(t *testing.T, item LineItem, want int64) {
	t.Helper()

	if got := item.Amount; !got.Equal(dong(want)) {
		t.Errorf("%s %s = %s, want %d", item.Statement, item.Indicator, got, want)
	}
}

// The filing holds the statements the regime calls for.
func TestGenerate_ProducesTheStatementsTheRegimeCallsFor(t *testing.T) {
	out := newFixture(t).generate(t)

	want := []string{"01/GTGT", codeVATReduction, "03/TNDN", "B01-DN", "B02-DN"}
	if got := codes(out); !equal(got, want) {
		t.Errorf("the filing holds %v, want %v", got, want)
	}

	if out.Period.ID != "2026-01" || out.Period.Regime != RegimeTT133 {
		t.Errorf("period = %+v, want 2026-01 under TT133", out.Period)
	}
	if out.Period.StartDate != "2026-01-01" || out.Period.EndDate != "2026-01-31" {
		t.Errorf("period dates = %s to %s, want the month", out.Period.StartDate, out.Period.EndDate)
	}
	if !out.GeneratedAt.Equal(generatedAt) {
		t.Errorf("generated at %v, want %v", out.GeneratedAt, generatedAt)
	}
}

// No line is filed with a mã số. The file records the numbering against a
// secondary source and leaves it open in Q5, Q2 and Q8, and its own rule is not
// to build on what it has not resolved.
func TestGenerate_FilesNoIndicatorNumber(t *testing.T) {
	out := newFixture(t).generate(t)

	for _, item := range out.Lines() {
		if item.Indicator != "" {
			t.Errorf("%s %s is filed under mã số %q, and the file has not resolved the numbering",
				item.Statement, item.Label, item.Indicator)
		}
	}

	// What the file does record is named in the warning, so a filer who accepts
	// the secondary numbering can write it in by hand.
	held := statement(t, out, "01/GTGT")

	named := false
	for _, warning := range held.Warnings {
		if warning.Question == "Q5" && strings.Contains(warning.Item, "[22]") && strings.Contains(warning.Item, "[43]") {
			named = true
		}
	}
	if !named {
		t.Errorf("the return does not name the mã số it withheld: %+v", held.Warnings)
	}
}

// Every statement says what its numbering is: which question blocks it, and
// where the numbering is only a secondary source's.
func TestGenerate_SaysWhatItsNumberingIs(t *testing.T) {
	out := newFixture(t).generate(t)

	wantQuestion := map[string]string{
		"01/GTGT":        "Q5",
		"03/TNDN":        "Q8",
		"B01-DN":         "Q2",
		"B02-DN":         "Q2",
		codeVATReduction: "Q8",
	}

	for code, question := range wantQuestion {
		held := statement(t, out, code)

		asked := false
		for _, warning := range held.Warnings {
			if warning.Question == question && strings.Contains(warning.Item, "mã số") {
				asked = true
			}
		}
		if !asked {
			t.Errorf("%s does not warn that %s leaves its numbering open: %+v", code, question, held.Warnings)
		}
	}
}

// Every line of every statement names the instrument that authorises it.
func TestGenerate_EveryLineHasALegalBasis(t *testing.T) {
	out := newFixture(t).generate(t)

	lines := out.Lines()
	if len(lines) == 0 {
		t.Fatal("the filing holds no lines at all")
	}

	for _, item := range lines {
		if strings.TrimSpace(item.Reference) == "" {
			t.Errorf("%s %s carries no legal basis", item.Statement, item.Label)
		}
		if item.Confidence != Primary && item.Confidence != Secondary {
			t.Errorf("%s %s rests on %q, which is not a resolved basis", item.Statement, item.Label, item.Confidence)
		}
		if item.Label == "" {
			t.Errorf("%s has a line with no label", item.Statement)
		}
	}
}

// A line the legal basis has not resolved is not filed. It is warned about
// instead, naming what is missing — and every unresolved line in the catalogue
// leaves exactly one warning behind it.
func TestGenerate_NoUnresolvedLineAppears(t *testing.T) {
	f := newFixture(t)
	out := f.generate(t)

	for _, item := range out.Lines() {
		if item.Confidence == Unresolved {
			t.Errorf("%s %s is unresolved and was filed anyway", item.Statement, item.Label)
		}
	}

	// Nothing the catalogue marks unresolved reaches a statement or a total.
	for _, spec := range catalogueSpecs(t) {
		for _, line := range spec.lines {
			if !line.unresolved {
				continue
			}

			held := statement(t, out, spec.code)
			if !dropsLine(held, line) {
				t.Errorf("%s dropped %q and the filing does not say so: %+v", spec.code, line.label, held.Warnings)
			}
		}
	}

	// And the things left out are named, so a filing says what it does not say.
	for _, phrase := range []string{
		"provisional-payment exemption threshold",
		"reduction after the window closes",
		"indicator not described",
	} {
		if !warned(out, phrase) {
			t.Errorf("nothing in the warnings is about %q: %+v", phrase, out.Warnings)
		}
	}

	// Every warning belongs to a statement and says something: a warning with an
	// empty statement cannot be tied to the form that left the line out.
	for _, warning := range out.Warnings {
		if warning.Statement == "" {
			t.Errorf("a warning belongs to no statement: %+v", warning)
		}
		if warning.Reason == "" || warning.Item == "" {
			t.Errorf("a warning says nothing: %+v", warning)
		}
	}

	if !strings.Contains(f.log.String(), "left out") {
		t.Errorf("nothing was logged: %q", f.log.String())
	}
}

// dropsLine reports whether a statement carries a warning for a line it left
// out.
func dropsLine(held Statement, line lineSpec) bool {
	for _, warning := range held.Warnings {
		if strings.Contains(warning.Item, line.indicator) && strings.Contains(warning.Item, line.label) {
			return true
		}
	}

	return false
}

func warned(out Output, phrase string) bool {
	for _, warning := range out.Warnings {
		if strings.Contains(warning.Item, phrase) || strings.Contains(warning.Reason, phrase) {
			return true
		}
	}

	return false
}

func answers(question string, warnings []Warning) bool {
	for _, warning := range warnings {
		if warning.Question == question {
			return true
		}
	}

	return false
}

// The VAT return is the month's VAT arithmetic: what was carried in, what was
// credited, what is payable and what is carried out.
func TestGenerate_ComputesTheVATReturn(t *testing.T) {
	out := newFixture(t).generate(t)
	returned := statement(t, out, "01/GTGT")

	amount(t, lineWith(t, returned, "credit carried in"), 5_000_000)
	amount(t, lineWith(t, returned, "deductible input VAT"), 20_000_000)
	amount(t, lineWith(t, returned, "turnover at 5%"), 100_000_000)
	amount(t, lineWith(t, returned, "output VAT at 5%"), 5_000_000)
	amount(t, lineWith(t, returned, "turnover at 10%"), 400_000_000)
	amount(t, lineWith(t, returned, "output VAT at 10%"), 40_000_000)
	amount(t, lineWith(t, returned, "VAT payable for the period"), 24_000_000) // 49,000,000 output less 25,000,000 credited
	amount(t, lineWith(t, returned, "credit carried out"), 0)
}

// What is not subject to VAT, and what is supplied at 0%, are stated as they are
// invoiced.
func TestGenerate_StatesWhatThePeriodSoldWithoutVAT(t *testing.T) {
	f := newFixture(t)
	f.facts.pots.NonTaxable = dong(30_000_000)
	f.facts.pots.ZeroRated = dong(70_000_000)

	returned := statement(t, f.generate(t), "01/GTGT")

	amount(t, lineWith(t, returned, "turnover not subject to VAT"), 30_000_000)
	amount(t, lineWith(t, returned, "turnover at 0%"), 70_000_000)
}

// A month whose input absorbs its output carries the rest forward and pays
// nothing.
func TestGenerate_CarriesVATForwardWhenInputExceedsOutput(t *testing.T) {
	f := newFixture(t)

	// The output account holds less than the credit available.
	for i := range f.ledger().trial.Rows {
		if f.ledger().trial.Rows[i].AccountCode == accountOutputVAT {
			f.ledger().trial.Rows[i].PeriodCredit = dong(10_000_000)
		}
	}

	returned := statement(t, f.generate(t), "01/GTGT")

	amount(t, lineWith(t, returned, "VAT payable for the period"), 0)
	amount(t, lineWith(t, returned, "credit carried out"), 15_000_000) // 25,000,000 available less 10,000,000 output
}

// The reduced-VAT appendix is filed when the period sold in the 8% band, and not
// otherwise: an appendix of zeroes is not a filing.
func TestGenerate_FilesTheReducedVATAppendixOnlyWhenTheBandIsUsed(t *testing.T) {
	f := newFixture(t)
	if !filed(f.generate(t), codeVATReduction) {
		t.Fatal("the appendix is missing although the month sold in the 8% band")
	}

	f.facts.pots = VATPots{ByRate: []VATBand{
		{Rate: rateTenPercent, Net: dong(400_000_000), Tax: dong(40_000_000)},
	}}

	if filed(f.generate(t), codeVATReduction) {
		t.Error("the appendix is filed although nothing was sold in the reduced band")
	}
}

func filed(out Output, code string) bool {
	for _, held := range out.Statements {
		if held.Code == code {
			return true
		}
	}

	return false
}

// The appendix states what the window gives up: the two points between the
// standard ten and the eight applied.
func TestGenerate_StatesTheVATTheWindowGivesUp(t *testing.T) {
	out := newFixture(t).generate(t)
	appendix := statement(t, out, codeVATReduction)

	amount(t, lineWith(t, appendix, "turnover in the reduced band"), 50_000_000)
	amount(t, lineWith(t, appendix, "VAT reduced"), 1_000_000)
}

// The corporate income tax finalisation: the year's turnover selects the rate,
// the tax follows from the profit, and what is still owed against the 80% test
// is stated.
func TestGenerate_ComputesTheCorporateIncomeTax(t *testing.T) {
	out := newFixture(t).generate(t)
	cit := statement(t, out, "03/TNDN")

	amount(t, lineWith(t, cit, "turnover for the year"), 4_000_000_000)
	amount(t, lineWith(t, cit, "accounting profit before tax"), 250_000_000)
	amount(t, lineWith(t, cit, "at the applicable rate"), 42_500_000) // 17% of 250,000,000
	amount(t, lineWith(t, cit, "provisional corporate income tax paid"), 5_000_000)
	amount(t, lineWith(t, cit, "shortfall against the 80% test"), 29_000_000) // 80% of 42.5m less 5m

	if got := lineWith(t, cit, "at the applicable rate").Note; !strings.Contains(got, "15%") {
		t.Errorf("the rate line reads %q, want it to name the rate the turnover selected", got)
	}
}

// A loss is taxed at nothing, and the 80% test asks for nothing.
func TestGenerate_TaxesALossAtNothing(t *testing.T) {
	f := newFixture(t)
	f.ledger().profit.NetProfit = dong(-50_000_000)

	cit := statement(t, f.generate(t), "03/TNDN")

	amount(t, lineWith(t, cit, "accounting profit before tax"), -50_000_000)
	amount(t, lineWith(t, cit, "at the applicable rate"), 0)
	amount(t, lineWith(t, cit, "shortfall against the 80% test"), 0)
}

// The financial statements are named and cited for the regime the books are kept
// under, and a regime with no form set is refused rather than filed under
// whichever name was nearest.
func TestGenerate_NamesTheFormsForTheRegime(t *testing.T) {
	under133 := newFixture(t).generate(t)
	if got := statement(t, under133, "B01-DN").Name; got != "Bảng cân đối kế toán" {
		t.Errorf("under TT133 the balance sheet is called %q", got)
	}
	if got := statement(t, under133, "B01-DN").Authority; !strings.Contains(got, "133/2016/TT-BTC") {
		t.Errorf("under TT133 the balance sheet cites %q, want Circular 133", got)
	}

	f := newFixture(t)
	if _, err := NewService(Config{
		Ledger: f.service.ledger,
		Facts:  f.facts,
		Regime: RegimeTT99,
		Now:    func() time.Time { return generatedAt },
	}); err != nil {
		t.Fatalf("NewService under TT99: %v", err)
	}

	under99 := fixtureUnder(t, RegimeTT99).generate(t)
	if got := statement(t, under99, "B01-DN").Name; got != "Báo cáo tình hình tài chính" {
		t.Errorf("under TT99 the balance sheet is called %q, want the renamed form", got)
	}
	if got := statement(t, under99, "B01-DN").Authority; !strings.Contains(got, "99/2025/TT-BTC") {
		t.Errorf("under TT99 the balance sheet cites %q, want Circular 99", got)
	}

	if _, err := NewService(Config{
		Ledger: f.service.ledger,
		Facts:  f.facts,
		Regime: "IFRS",
	}); !errors.Is(err, ErrUnknownRegime) {
		t.Fatalf("error = %v, want ErrUnknownRegime: the form set follows the regime", err)
	}
}

func fixtureUnder(t *testing.T, regime string) *fixture {
	t.Helper()

	f := newFixture(t)

	service, err := NewService(Config{
		Ledger: f.service.ledger,
		Facts:  f.facts,
		Regime: regime,
		Now:    func() time.Time { return generatedAt },
	})
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}

	f.service = service

	return f
}

// A filing says the numbering it could not resolve, rather than inventing one.
func TestGenerate_WarnsAboutUnresolvedNumbering(t *testing.T) {
	out := newFixture(t).generate(t)

	asked := map[string]string{"01/GTGT": "Q5", "03/TNDN": "Q8", "B01-DN": "Q2", "B02-DN": "Q2"}

	for code, question := range asked {
		held := statement(t, out, code)

		answered := false
		for _, warning := range held.Warnings {
			if warning.Question == question && strings.HasPrefix(warning.Item, "mã số") {
				answered = true
			}
		}
		if !answered {
			t.Errorf("%s carries no warning that %s leaves its mã số unresolved: %+v", code, question, held.Warnings)
		}
	}
}

func TestGenerate_RefusesABookItCannotRead(t *testing.T) {
	f := newFixture(t)
	f.ledger().err = errors.New("the ledger is unreadable")

	if _, err := f.service.Generate(context.Background(), "2026-01"); err == nil {
		t.Fatal("a filing was produced from a book nobody could read")
	}
}

func TestNewService_RefusesAnIncompleteConfiguration(t *testing.T) {
	f := newFixture(t)

	cases := map[string]Config{
		"no ledger":                 {Facts: f.facts, Regime: RegimeTT133},
		"no facts":                  {Ledger: f.service.ledger, Regime: RegimeTT133},
		"no regime":                 {Ledger: f.service.ledger, Facts: f.facts},
		"a regime with no form set": {Ledger: f.service.ledger, Facts: f.facts, Regime: "micro"},
	}

	for name, cfg := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := NewService(cfg); err == nil {
				t.Fatal("an incomplete service was built")
			}
		})
	}
}

// The catalogue cites instruments, articles, forms and accounts, and every one
// of them is something docs/legal-basis.md holds.
//
// This is the repository's own rule — "any financial rule, rate, form code or
// account number in this repository must trace to a row in this file" — checked
// rather than asserted. It is deliberately blunt: it matches the tokens, so a
// citation the file does not carry fails here even when the sentence around it
// reads well.
func TestCatalogue_CitesOnlyWhatTheLegalBasisHolds(t *testing.T) {
	basis := readLegalBasis(t)

	instruments := regexp.MustCompile(`\d+/\d{4}/(TT-BTC|QH1[0-9]|NĐ-CP)`)
	articles := regexp.MustCompile(`Điều \d+`)

	type citation struct {
		where string
		text  string
	}

	// A legal basis names an instrument, and any article it pinpoints is one the
	// file places. A form code is a token the file names. Neither check is
	// applied to the warnings, which are prose about what is *missing* and so
	// cite nothing by design.
	bases := make([]citation, 0)
	formCodes := make([]string, 0)

	for _, spec := range catalogueSpecs(t) {
		bases = append(bases, citation{spec.code + " (authority)", spec.authority})
		formCodes = append(formCodes, spec.code)

		for _, item := range spec.lines {
			bases = append(bases, citation{spec.code + " " + item.label, item.reference})
		}
	}

	if len(bases) < 20 {
		t.Fatalf("the catalogue holds %d citations, which is too few for this test to mean anything", len(bases))
	}

	for _, held := range bases {
		found := instruments.FindAllString(held.text, -1)
		if len(found) == 0 {
			t.Errorf("%s cites %q, which names no instrument", held.where, held.text)

			continue
		}

		for _, instrument := range found {
			if !strings.Contains(basis, instrument) {
				t.Errorf("%s cites %s, which docs/legal-basis.md does not record", held.where, instrument)
			}
		}

		for _, article := range articles.FindAllString(held.text, -1) {
			if !strings.Contains(basis, article) {
				t.Errorf("%s cites %s, which docs/legal-basis.md does not place", held.where, article)
			}
		}
	}

	// A filing is handed out under a form code, and every code is the file's —
	// including the appendix, whose only code is the name the file gives it.
	for _, code := range formCodes {
		if !strings.Contains(basis, code) {
			t.Errorf("the filing carries the form %q, which docs/legal-basis.md does not name", code)
		}
	}

	// And the chart the accounts come from is the file's chart.
	if !strings.Contains(basis, "Phụ lục 1") {
		t.Error("docs/legal-basis.md no longer records the TT133 chart, and the filing's accounts come from it")
	}
}

// catalogueSpecs is every statement the service can produce, under both regimes
// and in both states of the VAT window, so that the traceability test covers all
// of them.
func catalogueSpecs(t *testing.T) []statementSpec {
	t.Helper()

	specs := []statementSpec{
		vatReturnCatalogue(),
		vatReductionCatalogue(true),
		vatReductionCatalogue(false),
		citCatalogue(),
	}

	for _, regime := range []string{RegimeTT133, RegimeTT99} {
		held, err := financialStatementsCatalogue(regime)
		if err != nil {
			t.Fatalf("financialStatementsCatalogue(%s): %v", regime, err)
		}

		specs = append(specs, held...)
	}

	return specs
}

func readLegalBasis(t *testing.T) string {
	t.Helper()

	// Walk up to the repository root instead of counting directory levels. The
	// v2 port moved the module from fingo/ to the repository root, and a fixed
	// ".." chain was a fact about layout, not about the legal register. This is
	// a layout fix, not a behaviour change: every assertion in this file still
	// reads the same file's contents.
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}

	for range 8 {
		held, err := os.ReadFile(filepath.Join(dir, "docs", "legal-basis.md"))
		if err == nil {
			return string(held)
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}

	t.Fatalf("docs/legal-basis.md not found in any directory above the test's own")
	return ""
}

// The accounts a filing reads are the chart's accounts.
func TestCatalogue_AccountsExistInTheChart(t *testing.T) {
	chart := gl.NewChartOfAccounts()
	for _, account := range gl.TT133Chart() {
		if err := chart.Add(account); err != nil {
			t.Fatalf("seed chart with %s: %v", account.Code, err)
		}
	}

	for _, code := range []string{accountInputVAT, accountOutputVAT, accountCITPayable} {
		account, known := chart.ByCode(code)
		if !known {
			t.Errorf("the filing reads TK %s, which the chart does not hold", code)

			continue
		}
		if !chart.IsPostable(code) {
			t.Errorf("the filing reads TK %s, which cannot be posted to, so nothing is ever on it", code)
		}
		if account.Name == "" {
			t.Errorf("TK %s has no name", code)
		}
	}
}

// The rates the catalogue applies are the ones the file states.
func TestCatalogue_RatesAreTheOnesTheFileStates(t *testing.T) {
	basis := readLegalBasis(t)

	for _, stated := range []string{"Standard 10%", "reduced list", "8% window runs to 31/12/2026"} {
		if !strings.Contains(basis, stated) {
			t.Errorf("docs/legal-basis.md no longer says %q, and the rates here come from it", stated)
		}
	}

	if rateFivePercent != 500 || rateEightPercent != 800 || rateTenPercent != 1000 {
		t.Errorf("the rates are %d/%d/%d basis points, want 500/800/1000",
			rateFivePercent, rateEightPercent, rateTenPercent)
	}
	if standardRateDelta != rateTenPercent-rateEightPercent {
		t.Errorf("the window gives up %d basis points and the difference between the rates is %d",
			standardRateDelta, rateTenPercent-rateEightPercent)
	}
}

func equal(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}

	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}

	return true
}

// A VATPots with a band is what the appendix needs; a rate the file does not
// name is not a band the filing can file.
func TestVATPots_UnknownRatesAreNotBanded(t *testing.T) {
	f := figures{vat: VATPots{ByRate: []VATBand{{Rate: vat.BasisPoints(1234), Net: dong(1), Tax: dong(1)}}}}

	if band := f.vatBand(rateTenPercent); !band.Net.IsZero() || !band.Tax.IsZero() {
		t.Errorf("a rate nobody filed produced the 10%% band: %+v", band)
	}
}

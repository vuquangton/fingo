package statutory

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"time"

	"fingo/internal/money"
	"fingo/internal/reports"
	"fingo/internal/vat"
	"fingo/pkg/logger"
)

// The accounting regimes this package has a form set for.
//
// There are four tracks in Vietnamese law — SME, general, micro and IFRS — and
// this package knows the two the repository actually names. The others are
// refused rather than filed under whichever name was nearest.
const (
	// RegimeTT133 is the SME regime the repository declares: Circular
	// 133/2016/TT-BTC as amended by TT 46/2025/TT-BTC.
	RegimeTT133 = "TT133"

	// RegimeTT99 is the general regime in force from FY2026.
	RegimeTT99 = "TT99"
)

// The VAT rates this package applies, in basis points, from
// docs/legal-basis.md: the standard 10%, the 5% reduced list, and the 8% window
// that runs to 31/12/2026.
//
// They live here as data rather than in internal/vat because that package
// carries what a rate *is* — a row with an ID, a number of basis points and a
// period of effect — and not which rates the law prescribes.
const (
	rateFivePercent   vat.BasisPoints = 500
	rateEightPercent  vat.BasisPoints = 800
	rateTenPercent    vat.BasisPoints = 1000
	standardRateDelta vat.BasisPoints = 200 // what the 8% window gives up
)

// VATBand is what one VAT rate was charged on, and how much tax it carried.
type VATBand struct {
	Rate vat.BasisPoints
	Net  money.Dong
	Tax  money.Dong
}

// VATPots is what the period's invoices say about VAT, by treatment.
type VATPots struct {
	// NonTaxable and ZeroRated are net values: what the period sold without VAT
	// being chargeable, and what it exported or supplied at 0%.
	NonTaxable money.Dong
	ZeroRated  money.Dong

	// ByRate is one band per rate that appeared.
	ByRate []VATBand
}

// Facts is what the statements need and the ledger does not hold.
//
// Two things live here, and both are facts about something other than a voucher
// line. The rate a sale was taxed at is a fact about the invoice: a voucher line
// carries an account and an amount, and the posting rules send every sales VAT
// to the same output account. The year's turnover is a fact about the year,
// which is what selects the corporate income tax rate.
type Facts interface {
	// VATPots returns the period's VAT by treatment.
	VATPots(ctx context.Context, p ReportingPeriod) (VATPots, error)

	// YearTurnover returns the turnover the corporate income tax rate is
	// selected by.
	YearTurnover(ctx context.Context, p ReportingPeriod) (money.Dong, error)
}

// Ledger is the books a filing is computed from.
//
// *reports.Service satisfies it. The statements are summed from posted voucher
// lines by that package, and nothing here reads a stored balance.
type Ledger interface {
	TrialBalance(ctx context.Context, periodID string) (reports.TrialBalance, error)
	ProfitAndLoss(ctx context.Context, periodID string) (reports.ProfitAndLoss, error)
	BalanceSheet(ctx context.Context, periodID string) (reports.BalanceSheet, error)
}

// Config is what a statutory service is built from.
type Config struct {
	// Ledger and Facts are required.
	Ledger Ledger
	Facts  Facts

	// Regime is the accounting regime the books are kept under. Required: it
	// decides which forms are filed.
	Regime string

	// Log records what a filing left out. Nil discards it — the warnings are in
	// the output as well, so nothing is lost by a caller that does not want
	// them on the console.
	Log *logger.Logger

	// Now is the clock the run is stamped with. Nil means time.Now.
	Now func() time.Time
}

// Service produces statutory filings.
type Service struct {
	ledger Ledger
	facts  Facts
	regime string
	log    *logger.Logger
	now    func() time.Time
}

// NewService builds the service.
func NewService(cfg Config) (*Service, error) {
	switch {
	case cfg.Ledger == nil:
		return nil, fmt.Errorf("%w: a filing needs the books it is computed from", ErrInvalidService)
	case cfg.Facts == nil:
		return nil, fmt.Errorf("%w: a filing needs the facts the ledger does not hold (VAT by treatment, the year's turnover)", ErrInvalidService)
	}

	switch cfg.Regime {
	case RegimeTT133, RegimeTT99:
	default:
		return nil, fmt.Errorf("%w: %q", ErrUnknownRegime, cfg.Regime)
	}

	log := cfg.Log
	if log == nil {
		log = logger.New(logger.Config{Level: slog.LevelWarn, Writer: io.Discard, Format: logger.FormatText})
	}

	now := cfg.Now
	if now == nil {
		now = func() time.Time { return time.Now().UTC() }
	}

	return &Service{ledger: cfg.Ledger, facts: cfg.Facts, regime: cfg.Regime, log: log, now: now}, nil
}

// Generate produces every statement the period calls for.
//
// The period's dates come from the books themselves — the trial balance states
// the period it was read for — so a filing cannot be dated differently from the
// report it was computed from.
func (s *Service) Generate(ctx context.Context, periodID string) (Output, error) {
	trial, err := s.ledger.TrialBalance(ctx, periodID)
	if err != nil {
		return Output{}, fmt.Errorf("read the trial balance for %s: %w", periodID, err)
	}

	period := ReportingPeriod{
		ID:        periodID,
		StartDate: trial.StartDate.UTC().Format(time.DateOnly),
		EndDate:   trial.EndDate.UTC().Format(time.DateOnly),
		Regime:    s.regime,
	}

	profit, err := s.ledger.ProfitAndLoss(ctx, periodID)
	if err != nil {
		return Output{}, fmt.Errorf("read the profit and loss report for %s: %w", periodID, err)
	}

	sheet, err := s.ledger.BalanceSheet(ctx, periodID)
	if err != nil {
		return Output{}, fmt.Errorf("read the balance sheet for %s: %w", periodID, err)
	}

	pots, err := s.facts.VATPots(ctx, period)
	if err != nil {
		return Output{}, fmt.Errorf("read the period's VAT by treatment: %w", err)
	}

	turnover, err := s.facts.YearTurnover(ctx, period)
	if err != nil {
		return Output{}, fmt.Errorf("read the year's turnover: %w", err)
	}

	assembled := figures{
		period:    period,
		trial:     trial,
		profit:    profit,
		sheet:     sheet,
		vat:       pots,
		vatReturn: computeVAT(trial, pots),
		turnover:  turnover,
		citPaid:   movement(trial, accountCITPayable, false),
	}

	// The rate bands and the ledger's output VAT are two accounts of the same
	// tax. Where they disagree the payable follows the ledger, and the
	// difference is said rather than smoothed over.
	specs := []statementSpec{vatReturnCatalogue()}
	if warnings := assembled.vatReconciliation(); len(warnings) > 0 {
		specs[0].warnings = append(specs[0].warnings, warnings...)
	}

	// The reduced-VAT appendix is filed when the period actually sold in the
	// band, and it follows the return it is an appendix to. An appendix of
	// zeroes is not a filing, it is noise. The reduction itself is stated only
	// where the window was open for the period.
	if assembled.vatBand(rateEightPercent).Net.IsPositive() {
		specs = append(specs, vatReductionCatalogue(assembled.withinVATWindow()))
	}

	specs = append(specs, citCatalogue())

	statements, err := financialStatementsCatalogue(s.regime)
	if err != nil {
		return Output{}, err
	}

	specs = append(specs, statements...)

	out := Output{
		Period:      period,
		GeneratedAt: s.now().UTC(),
		Statements:  make([]Statement, 0, len(specs)),
		Warnings:    make([]Warning, 0),
	}

	for _, spec := range specs {
		statement := s.build(ctx, spec, assembled)
		out.Statements = append(out.Statements, statement)
		out.Warnings = append(out.Warnings, statement.Warnings...)
	}

	// A filing that cannot hold up as one is not returned. This is the last
	// place a missing legal basis can be caught before it reaches an auditor.
	if err := out.Validate(); err != nil {
		return Output{}, err
	}

	return out, nil
}

// build turns a statement's specification and its figures into a statement,
// leaving out every line the legal basis has not resolved and saying so.
func (s *Service) build(ctx context.Context, spec statementSpec, f figures) Statement {
	statement := Statement{
		Code:       spec.code,
		Name:       spec.name,
		Authority:  spec.authority,
		Confidence: spec.confidence,
		Lines:      make([]LineItem, 0, len(spec.lines)),
		Warnings:   make([]Warning, 0, len(spec.warnings)+2),
	}

	numbered := spec.numbered()

	// The numbering warning comes first: it is about the statement as a whole,
	// and it is what a reader has to know before reading a line that carries no
	// mã số.
	if !numbered {
		item := "mã số"
		if spec.withheld != "" {
			item = "mã số " + spec.withheld
		}

		statement.Warnings = append(statement.Warnings, Warning{
			Item:     item,
			Question: spec.indicatorQuestion,
			Reason:   spec.indicatorReason,
		})
	}

	for _, warning := range spec.warnings {
		// By index, so that the statement the warning belongs to is written into
		// the warning and not into a copy of it.
		statement.Warnings = append(statement.Warnings, warning)
	}

	for i := range statement.Warnings {
		statement.Warnings[i].Statement = spec.code
		s.warn(ctx, statement.Warnings[i])
	}

	for _, line := range spec.lines {
		if line.unresolved {
			warning := Warning{
				Statement: spec.code,
				Item:      warningItem(line),
				Question:  line.question,
				Reason:    line.reason,
			}
			statement.Warnings = append(statement.Warnings, warning)
			s.warn(ctx, warning)

			continue
		}

		indicator := line.indicator
		if !numbered {
			// The file records the number against a secondary source and leaves
			// the numbering open. What it does not resolve is not written on a
			// line that is going to a tax authority.
			indicator = ""
		}

		statement.Lines = append(statement.Lines, LineItem{
			Statement:  spec.code,
			Indicator:  indicator,
			Label:      line.label,
			Reference:  line.reference,
			Confidence: line.confidence,
			Amount:     line.amount(f),
			Note:       line.note,
		})
	}

	return statement
}

// warningItem names a dropped line by its indicator where it has one, so that
// two warnings about two indicators are not the same sentence twice.
func warningItem(line lineSpec) string {
	if line.indicator == "" {
		return line.label
	}

	return line.indicator + " — " + line.label
}

// warn records a warning in the log. It is in the output too; this is for the
// operator who wants to know what the filing just left out.
func (s *Service) warn(ctx context.Context, warning Warning) {
	s.log.Warn(ctx, "statutory line left out",
		slog.String("statement", warning.Statement),
		slog.String("item", warning.Item),
		slog.String("question", warning.Question),
		slog.String("reason", warning.Reason),
	)
}

// vatReturn is the arithmetic of the 01/GTGT return.
type vatReturn struct {
	CarriedIn   money.Dong // what the last period left to credit
	InputCredit money.Dong // the period's deductible input VAT
	OutputTax   money.Dong // the period's output VAT
	Payable     money.Dong // output less credit and input, when positive
	CarriedOut  money.Dong // the rest, carried to the next period
}

// computeVAT works out what the return states.
//
// The output VAT is taken from the ledger — the movement on TK 33311 — and not
// from the rates the invoices were taxed at, because the payable must be what
// the books say was charged. The rate bands are reported beside it, and
// vatReconciliation is what says so when the two do not agree.
func computeVAT(trial reports.TrialBalance, pots VATPots) vatReturn {
	carriedIn := openingBalance(trial, accountInputVAT)
	if carriedIn.IsNegative() {
		// Input VAT carried on the credit side is not a credit carried in; it is
		// a book that has been reversed past zero. It is not netted into the
		// return, and vatReconciliation says so.
		carriedIn = money.Zero
	}

	out := vatReturn{
		CarriedIn:   carriedIn,
		InputCredit: movement(trial, accountInputVAT, false),
		OutputTax:   movement(trial, accountOutputVAT, true),
	}

	available := out.CarriedIn.Add(out.InputCredit)

	if out.OutputTax.GreaterThan(available) {
		out.Payable = out.OutputTax.Sub(available)
	} else {
		out.CarriedOut = available.Sub(out.OutputTax)
	}

	return out
}

// figures is everything the catalogue's lines are computed from.
type figures struct {
	period ReportingPeriod

	trial  reports.TrialBalance
	profit reports.ProfitAndLoss
	sheet  reports.BalanceSheet

	vat       VATPots
	vatReturn vatReturn
	turnover  money.Dong
	citPaid   money.Dong
}

// vatBand returns the band for a rate, or an empty one when the period sold
// nothing at it. A band that did not happen is a zero box on the form.
func (f figures) vatBand(rate vat.BasisPoints) VATBand {
	band := VATBand{Rate: rate}

	for _, held := range f.vat.ByRate {
		if held.Rate != rate {
			continue
		}

		band.Net = band.Net.Add(held.Net)
		band.Tax = band.Tax.Add(held.Tax)
	}

	return band
}

// vatReconciliation names the differences between what the invoices say the tax
// was and what the ledger holds, so that a filer clears them instead of filing
// a payable nobody can tie back.
func (f figures) vatReconciliation() []Warning {
	out := make([]Warning, 0, 2)

	banded := f.bandedTax()
	if !banded.Equal(f.vatReturn.OutputTax) {
		out = append(out, Warning{
			Statement: "01/GTGT",
			Item:      "thuế GTGT đầu ra (output VAT)",
			Reason: fmt.Sprintf("%s: the rate bands carry %s and TK %s holds %s.",
				reasonOutputVAT, banded, accountOutputVAT, f.vatReturn.OutputTax),
		})
	}

	if openingBalance(f.trial, accountInputVAT).IsNegative() {
		out = append(out, Warning{
			Statement: "01/GTGT",
			Item:      "thuế GTGT đầu vào (input VAT)",
			Reason: fmt.Sprintf("TK %s carries a credit balance of %s, so nothing is carried in: input VAT carried on the credit side is a book reversed past zero, not a credit to net off.",
				accountInputVAT, openingBalance(f.trial, accountInputVAT).Neg()),
		})
	}

	return out
}

// bandedTax is what the reported rate bands carry in total.
func (f figures) bandedTax() money.Dong {
	total := money.Zero

	for _, held := range f.vat.ByRate {
		total = total.Add(held.Tax)
	}

	return total
}

// withinVATWindow reports whether the period ends inside the 8% window, which
// docs/legal-basis.md closes on 31/12/2026.
func (f figures) withinVATWindow() bool {
	closed, err := time.Parse(time.DateOnly, vatWindowCloses)
	if err != nil {
		return false
	}

	end, err := time.Parse(time.DateOnly, f.period.EndDate)
	if err != nil {
		return false
	}

	return !end.After(closed)
}

// vatWindowCloses is the last day of the 8% window of Nghị quyết 204/2025/QH15,
// as docs/legal-basis.md states it.
const vatWindowCloses = "2026-12-31"

// citRate is the rate the year's turnover selects, in basis points:
// 15% at or below 3bn, 17% above 3bn up to 50bn, 20% above that.
func (f figures) citRate() vat.BasisPoints {
	switch {
	case f.turnover.LessThanOrEqual(money.FromInt(3_000_000_000)):
		return 1500
	case f.turnover.LessThanOrEqual(money.FromInt(50_000_000_000)):
		return 1700
	default:
		return 2000
	}
}

// citLiability is the tax on the period's profit at the applicable rate. A loss
// is taxed at nothing.
func (f figures) citLiability() money.Dong {
	if !f.profit.NetProfit.IsPositive() {
		return money.Zero
	}

	return roundDong(f.profit.NetProfit.
		Mul(money.FromInt(int64(f.citRate()))).
		Div(money.FromInt(int64(vat.Whole))))
}

// citShortfall is what the provisional payments are short of the 80% the
// administration expects, and nothing when they are not short.
func (f figures) citShortfall() money.Dong {
	required := roundDong(f.citLiability().
		Mul(money.FromInt(80)).
		Div(money.FromInt(100)))

	if f.citPaid.GreaterThanOrEqual(required) {
		return money.Zero
	}

	return required.Sub(f.citPaid)
}

// roundDong rounds a computed amount to whole đồng, half away from zero.
//
// It is the direction internal/vat rounds a tax on an amount in, and a second
// answer to "which way does half go" is a second answer to a question that has
// one.
func roundDong(amount money.Dong) money.Dong {
	return amount.Round(0)
}

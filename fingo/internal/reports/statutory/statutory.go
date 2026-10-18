// Package statutory produces the filings a client's books have to answer to: the
// VAT return, the corporate income tax finalisation and the annual financial
// statements — as structured JSON for filing, and as a PDF for the archive.
//
// # Every line traces to docs/legal-basis.md
//
// A line item names the instrument that authorises it, quoted the way
// docs/legal-basis.md writes it, and carries the confidence that file gives it:
// P (verified against primary text), S (a named secondary source), or ? (not
// resolved). TestCatalogue_CitesOnlyWhatTheLegalBasisHolds reads the file and
// checks every reference in this package appears in it, so a line item cannot
// cite an authority the repository has not recorded.
//
// # A ? line is never emitted
//
// The rule is the file's own: "A row marked ? must not be built on. Resolve it
// first." An unresolved line is left out of both outputs and reported as a
// warning naming what is missing and which open question blocks it, so a filing
// says what it does not contain rather than being quietly short.
//
// The same rule reaches the indicator numbers, and there it bites hardest. The
// file records the 01/GTGT mapping — [22] carried in, [23]–[25] input, [26]
// non-taxable, [29] 0%, [30]/[31] 5%, [32]/[33] 10%, [40] payable, [43] carried
// out — against a named secondary source, and in the same breath leaves those
// numbers open: "Remaining ?: … the 01/GTGT indicator numbers (Phụ lục I of TT
// 89/2026 was not readable)" (Q5). It does the same to the financial statements
// (Q2) and, by not recording them at all, to 03/TNDN.
//
// A line the rule forbids building on is not built on, so **no line in this
// package is filed with a mã số**. What the file does record is kept in the
// source and named in the statement's warning, so a filer who accepts the
// secondary numbering can write it in by hand — and the filing says, on its
// face, that they had to.
//
// # Where the figures come from
//
// From internal/reports, which sums posted voucher lines: nothing here reads a
// stored balance, and nothing here is a rate or a threshold that is not in
// docs/legal-basis.md. What the ledger cannot say — the rate a sale was taxed at
// is a fact about the invoice, and the year's turnover is a fact about the year
// — is asked of Facts instead. See Service.
//
// # Regimes are not interchangeable
//
// The form set follows the accounting regime the books are kept under, and the
// file is explicit that a change to one regime is not a change to another: "Do
// not port changes between regimes." The financial statements are therefore
// named and cited for the regime the service was built with, and an unknown
// regime is refused rather than filed under whichever name was handy.
package statutory

import (
	"errors"
	"time"

	"github.com/shopspring/decimal"
)

// Errors a caller may need to tell apart.
var (
	// ErrInvalidService is a statutory service built without what it needs.
	ErrInvalidService = errors.New("statutory: incomplete service")

	// ErrUnknownRegime is a regime the form set has nothing to say about. The
	// regime decides which forms are filed, so it cannot be guessed.
	ErrUnknownRegime = errors.New("statutory: no form set is known for that accounting regime")

	// ErrInvalidOutput is output that does not hold up: a line with no legal
	// basis, or a line the legal basis has not resolved.
	ErrInvalidOutput = errors.New("statutory: invalid output")
)

// Confidence is how well a line item traces to law, as docs/legal-basis.md marks
// it.
type Confidence string

const (
	// Primary is verified against primary text: the instrument body, the
	// gazette, the ministry or the tax authority.
	Primary Confidence = "P"

	// Secondary is verified against a named secondary source citing the
	// instrument — a professional firm, a vendor, state-owned press.
	Secondary Confidence = "S"

	// Unresolved is not resolved, and must not be built on.
	Unresolved Confidence = "?"
)

// LineItem is one line of a statement.
type LineItem struct {
	// Statement is the form the line belongs to, so a line can be read on its
	// own and still say where it comes from.
	Statement string `json:"statement"`

	// Indicator is the mã số the form prescribes, where the legal basis has
	// resolved one. Empty when the numbering is unresolved, which is what the
	// statement's warning is about.
	Indicator string `json:"indicator"`

	// Label is the line as the form names it, in Vietnamese, with a short
	// English gloss where the name alone would not place it.
	Label string `json:"label"`

	// Reference is the legal basis: the instrument and the provision, written
	// the way docs/legal-basis.md writes them.
	Reference string `json:"reference"`

	// Confidence is the file's mark for that basis.
	Confidence Confidence `json:"confidence"`

	// Amount is in whole đồng. It is written as a decimal string rather than a
	// JSON number, because a JSON number is a float in most parsers and an
	// amount that rounds is an amount that is wrong.
	Amount decimal.Decimal `json:"amount"`

	// Note is anything a reader of this line has to know: which rate applied,
	// which question blocks the mã số, what the line is derived from.
	Note string `json:"note,omitempty"`
}

// Statement is one filing.
type Statement struct {
	// Code is the form: 01/GTGT, 03/TNDN, B01-DN.
	Code string `json:"code"`

	Name string `json:"name"`

	// Authority is the instrument the form itself comes from.
	Authority string `json:"authority"`

	// Confidence is the file's mark for that authority.
	Confidence Confidence `json:"confidence"`

	Lines []LineItem `json:"lines"`

	// Warnings are what this statement left out or could not state — an
	// unresolved line, or a form whose indicator numbers are not resolved.
	Warnings []Warning `json:"warnings"`
}

// Warning is something a filing does not contain, and why.
type Warning struct {
	Statement string `json:"statement"`

	// Item is the line that was left out, or the subject the warning is about.
	Item string `json:"item"`

	// Question is the open question in docs/legal-basis.md that blocks it — Q2,
	// Q5, Q8 — where one does.
	Question string `json:"question,omitempty"`

	// Reason is what is missing, in the file's own terms.
	Reason string `json:"reason"`
}

// ReportingPeriod names what a filing covers.
type ReportingPeriod struct {
	ID        string `json:"id"`
	StartDate string `json:"start_date"`
	EndDate   string `json:"end_date"`

	// Regime is the accounting regime the books are kept under. The form set
	// follows it.
	Regime string `json:"regime"`
}

// Output is everything one run produced.
//
// It is what is filed and what is archived, and it carries its own warnings: a
// caller that ignores them still holds a document that says what it left out.
type Output struct {
	Period ReportingPeriod `json:"period"`

	// GeneratedAt is when the run happened, in UTC.
	GeneratedAt time.Time `json:"generated_at"`

	Statements []Statement `json:"statements"`

	// Warnings repeats every statement's warnings, so a filer can read what is
	// missing without walking the statements.
	Warnings []Warning `json:"warnings"`
}

// Lines returns every line of every statement, in filing order.
func (o Output) Lines() []LineItem {
	out := make([]LineItem, 0, len(o.Statements))

	for _, statement := range o.Statements {
		out = append(out, statement.Lines...)
	}

	return out
}

// Validate reports whether the output holds up as a filing.
//
// Two things are checked, and they are the two the whole package exists for:
// every line names the instrument that authorises it, and no line rests on
// something docs/legal-basis.md has not resolved. An output that fails either is
// not filed — it is a bug in the catalogue, caught here rather than by an
// auditor.
func (o Output) Validate() error {
	for _, statement := range o.Statements {
		if statement.Code == "" {
			return errors.New("statutory: a statement has no form code")
		}
		if statement.Authority == "" {
			return errors.New("statutory: statement " + statement.Code + " names no authority")
		}

		for _, line := range statement.Lines {
			switch {
			case line.Reference == "":
				return errLine(line, "a line carries no legal basis")
			case line.Confidence == "":
				return errLine(line, "a line carries no confidence")
			case line.Confidence == Unresolved:
				return errLine(line, "a line rests on something docs/legal-basis.md has not resolved")
			case line.Label == "":
				return errLine(line, "a line has no label")
			}
		}
	}

	return nil
}

func errLine(line LineItem, why string) error {
	return errors.New("statutory: " + line.Statement + " " + line.Indicator + " " + line.Label + ": " + why)
}

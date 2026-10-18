package statutory

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/shopspring/decimal"

	"fingo/internal/reports"
)

// What was paid is what was debited to the tax account. The year-end accrual is
// a credit to the same account, and a net movement would turn a payment into a
// negative one and the 80% shortfall into a figure nobody could act on.
func TestGenerate_TaxPaidIsWhatWasPaidNotWhatIsOwed(t *testing.T) {
	f := newFixture(t)

	for i := range f.ledger().trial.Rows {
		if f.ledger().trial.Rows[i].AccountCode != accountCITPayable {
			continue
		}

		// The liability accrued at the year end, on the same account.
		f.ledger().trial.Rows[i].PeriodCredit = money(29_000_000)
	}

	cit := statement(t, f.generate(t), "03/TNDN")

	amount(t, lineWith(t, cit, "provisional corporate income tax paid"), 5_000_000)
	amount(t, lineWith(t, cit, "shortfall against the 80% test"), 29_000_000) // 80% of 42.5m less the 5m paid
}

// The 8% window closes on 31/12/2026, and no instrument extending it was found:
// a period after it states the turnover that was taxed at 8%, and no reduction.
func TestGenerate_StatesNoReductionAfterTheWindowCloses(t *testing.T) {
	f := newFixture(t)
	f.ledger().trial.StartDate = time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC)
	f.ledger().trial.EndDate = time.Date(2027, 1, 31, 0, 0, 0, 0, time.UTC)

	out := f.generate(t)
	appendix := statement(t, out, codeVATReduction)

	amount(t, lineWith(t, appendix, "turnover in the reduced band"), 50_000_000)

	for _, line := range appendix.Lines {
		if strings.Contains(line.Label, "VAT reduced") {
			t.Errorf("a period after the window states a reduction of %s", line.Amount)
		}
	}

	said := false
	for _, warning := range appendix.Warnings {
		if strings.Contains(warning.Reason, "31/12/2026") {
			said = true
		}
	}
	if !said {
		t.Errorf("nothing says the window had closed: %+v", appendix.Warnings)
	}
}

// The rate bands and the ledger are two accounts of the same tax. Where they
// disagree the payable follows the ledger and the difference is stated, because
// a filer has to clear it rather than discover it at an assessment.
func TestGenerate_WarnsWhenTheRateBandsAndTheLedgerDisagree(t *testing.T) {
	f := newFixture(t)

	for i := range f.ledger().trial.Rows {
		if f.ledger().trial.Rows[i].AccountCode == accountOutputVAT {
			f.ledger().trial.Rows[i].PeriodCredit = money(90_000_000)
		}
	}

	returned := statement(t, f.generate(t), "01/GTGT")

	// The payable is the ledger's, not the bands'.
	amount(t, lineWith(t, returned, "VAT payable for the period"), 65_000_000)

	said := 0
	for _, warning := range returned.Warnings {
		if warning.Item == "thuế GTGT đầu ra (output VAT)" {
			said++

			if !strings.Contains(warning.Reason, "49000000") || !strings.Contains(warning.Reason, "90000000") {
				t.Errorf("the warning does not name both figures: %s", warning.Reason)
			}
		}
	}
	if said != 1 {
		t.Errorf("the disagreement produced %d warnings, want one", said)
	}
}

// Input VAT carried on the credit side is a book reversed past zero, not a credit
// to net off. It is not netted into the return, and the filing says so.
func TestGenerate_WarnsAboutACreditBalanceOnInputVAT(t *testing.T) {
	f := newFixture(t)
	f.facts.pots.NonTaxable = decimal.Zero

	for i := range f.ledger().trial.Rows {
		if f.ledger().trial.Rows[i].AccountCode == accountInputVAT {
			f.ledger().trial.Rows[i].OpeningDebit = decimal.Zero
			f.ledger().trial.Rows[i].OpeningCredit = money(7_000_000)
		}
	}

	returned := statement(t, f.generate(t), "01/GTGT")

	amount(t, lineWith(t, returned, "credit carried in"), 0)

	said := false
	for _, warning := range returned.Warnings {
		if warning.Item == "thuế GTGT đầu vào (input VAT)" && strings.Contains(warning.Reason, "7000000") {
			said = true
		}
	}
	if !said {
		t.Errorf("a credit balance on input VAT passed without a word: %+v", returned.Warnings)
	}
}

// The finalisation mixes the year's turnover with the period's profit, and says
// so: a month is not a year, and a filer has to know which figure is which.
func TestGenerate_SaysWhichPeriodTheCITFiguresCover(t *testing.T) {
	cit := statement(t, newFixture(t).generate(t), "03/TNDN")

	said := false
	for _, warning := range cit.Warnings {
		if strings.Contains(warning.Item, "kỳ tính thuế") && strings.Contains(warning.Reason, "full year") {
			said = true
		}
	}
	if !said {
		t.Errorf("nothing says which period the figures cover: %+v", cit.Warnings)
	}
}

// A row too long for the page is folded rather than run off it: the warnings are
// what the archive exists to keep, and they are the longest lines there are.
func TestPDF_FoldsLinesThatWouldRunOffThePage(t *testing.T) {
	out := newFixture(t).generate(t)

	narrow := layout{width: 300, height: 800, margin: 20, leading: 12, bodySize: 9}
	for _, line := range out.pdfLines(narrow) {
		if len([]rune(line)) > narrow.columns() {
			t.Errorf("a line of %d characters does not fit a page of %d: %q",
				len([]rune(line)), narrow.columns(), line)
		}
	}

	// And what is folded is still all there.
	joined := strings.Join(out.pdfLines(narrow), " ")
	for _, wanted := range []string{"exemption threshold", "31/12/2026", "mã số"} {
		if !strings.Contains(joined, wanted) {
			t.Errorf("folding lost %q", wanted)
		}
	}
}

func TestWrapLine_BreaksOnWordsAndKeepsTheIndent(t *testing.T) {
	cases := map[string]struct {
		line   string
		budget int
		want   []string
	}{
		"it fits": {"short", 20, []string{"short"}},
		"it does not": {
			"aaaa bbbb cccc dddd",
			10,
			[]string{"aaaa bbbb", "cccc dddd"},
		},
		"the indent is kept": {
			"    aaaa bbbb cccc",
			15,
			[]string{"    aaaa bbbb", "    cccc"},
		},
		"a word longer than the page is cut": {
			"xxxxxxxxxxxxxxxxxxxxxx y",
			12,
			[]string{"xxxxxxxxxxxx", "xxxxxxxxxx y"},
		},
		"a page too narrow to fold": {"anything", 4, []string{"anything"}},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := wrapLine(tc.line, tc.budget)
			if len(got) != len(tc.want) {
				t.Fatalf("wrapLine made %d lines %q, want %d", len(got), got, len(tc.want))
			}

			for i := range got {
				if got[i] != tc.want[i] {
					t.Errorf("line %d = %q, want %q", i+1, got[i], tc.want[i])
				}
			}

			// Nothing is lost but the spaces a fold ate — unless a word had to
			// be cut, which is what a word longer than the page gets.
			if longestWord(tc.line) <= tc.budget {
				if strings.Join(strings.Fields(strings.Join(got, " ")), " ") !=
					strings.Join(strings.Fields(tc.line), " ") {
					t.Errorf("folding changed the words: %q from %q", got, tc.line)
				}
			} else if strings.Join(strings.Fields(strings.Join(got, " ")), "") !=
				strings.Join(strings.Fields(tc.line), "") {
				t.Errorf("folding lost characters: %q from %q", got, tc.line)
			}
		})
	}
}

// longestWord is the longest run of non-space characters in a line.
func longestWord(line string) int {
	longest := 0

	for _, word := range strings.Fields(line) {
		if len(word) > longest {
			longest = len(word)
		}
	}

	return longest
}

// A string that looks like it could break the document is escaped, not obeyed.
func TestPDF_SurvivesAStringThatLooksLikeTheDocument(t *testing.T) {
	out := newFixture(t).generate(t)

	out.Period.ID = `2026-01") Tj ET BT /F1 9 Tf (endstream` + "\n" + `% x \ 1 0 obj (x`

	document, err := out.PDF()
	if err != nil {
		t.Fatalf("PDF: %v", err)
	}

	pages := declaredPageCount(t, document)
	if pages < 1 {
		t.Fatalf("the document declares %d pages", pages)
	}

	// One stream per page, the page tree's own stream count included in neither:
	// the hostile text must not have started or ended one.
	if started := bytes.Count(document, []byte("\nstream\n")); started != pages {
		t.Errorf("the document holds %d content streams and %d pages: the text forged one", started, pages)
	}
	if ended := bytes.Count(document, []byte("\nendstream")); ended != pages {
		t.Errorf("the document ends %d streams and holds %d pages: the text closed one", ended, pages)
	}

	// The parentheses and the backslash reach the page escaped.
	if !bytes.Contains(document, []byte(`\)`)) {
		t.Error("the closing parenthesis in the text is not escaped")
	}
	if !bytes.Contains(document, []byte(`\\`)) {
		t.Error("the backslash in the text is not escaped")
	}
}

// The validator refuses a schema node this validator would have to guess at.
func TestJSONSchema_RefusesARefBesideOtherKeywords(t *testing.T) {
	compiled, err := compiledSchema()
	if err != nil {
		t.Fatalf("compiledSchema: %v", err)
	}

	node := map[string]any{
		"$ref":     "#/$defs/line",
		"required": []any{"amount"},
	}

	if err := compiled.validate(map[string]any{}, node, "$.test"); err == nil {
		t.Fatal("a $ref beside other keywords was silently accepted")
	}
}

// The filing's own schema refuses what the spec forbids, one rule at a time.
func TestValidateJSON_RefusesWhereTheSchemaIsSilent(t *testing.T) {
	f := newFixture(t)

	cases := map[string]func(*Output){
		"a warning that belongs to no statement": func(o *Output) {
			o.Warnings[0].Statement = ""
		},
		"an amount with a fractional đồng": func(o *Output) {
			o.Statements[0].Lines[0].Amount = decimal.RequireFromString("1.50")
		},
	}

	for name, spoil := range cases {
		t.Run(name, func(t *testing.T) {
			out := f.generate(t)
			out.Statements = append([]Statement{}, out.Statements...)
			out.Statements[0].Lines = append([]LineItem{}, out.Statements[0].Lines...)
			out.Warnings = append([]Warning{}, out.Warnings...)
			spoil(&out)

			document, err := out.JSON()
			if err == nil {
				t.Fatalf("the filing was rendered and validated: %s", firstLineOf(document))
			}
		})
	}

	// A generated-at that is not a time is refused where it can be seen: in a
	// document, because a time.Time always renders as one.
	document, err := f.generate(t).JSON()
	if err != nil {
		t.Fatalf("JSON: %v", err)
	}

	var held map[string]any
	if err := json.Unmarshal(document, &held); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	held["generated_at"] = ""

	broken, err := json.Marshal(held)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if err := ValidateJSON(broken); err == nil {
		t.Error("a filing with no generated-at passed validation")
	}
}

func firstLineOf(document []byte) string {
	line, _, _ := strings.Cut(string(document), "\n")

	return line
}

// The bands the filing adds up are the bands the ledger holds, which is what
// makes the reconciliation meaningful rather than decorative.
func TestFigures_BandedTaxAddsEveryBand(t *testing.T) {
	f := figures{vat: VATPots{ByRate: []VATBand{
		{Rate: rateFivePercent, Tax: money(5)},
		{Rate: rateTenPercent, Tax: money(40)},
		{Rate: rateEightPercent, Tax: money(4)},
	}}}

	if want := money(49); !f.bandedTax().Equal(want) {
		t.Errorf("banded tax = %s, want %s", f.bandedTax(), want)
	}
}

// A trial balance with no row for an account is a zero box, not a missing one.
func TestLedger_ReadsAnAccountTheBooksDoNotHold(t *testing.T) {
	trial := reports.TrialBalance{Rows: []reports.TrialBalanceRow{
		{AccountCode: "111", PeriodDebit: money(5)},
	}}

	if got := movement(trial, "9999", false); !got.IsZero() {
		t.Errorf("movement on an account with no row = %s, want 0", got)
	}
	if got := openingBalance(trial, "9999"); !got.IsZero() {
		t.Errorf("opening balance of an account with no row = %s, want 0", got)
	}
	if got := accountMovement(trial, "9999"); !got.IsZero() {
		t.Errorf("account movement of an account with no row = %s, want 0", got)
	}

	if got := movement(trial, "111", false); !got.Equal(money(5)) {
		t.Errorf("movement on 111 = %s, want 5", got)
	}
}

// A rate nobody filed is not a box on the return.
func TestVATPots_ARateThatWasNotUsedIsNotABand(t *testing.T) {
	f := figures{vat: VATPots{ByRate: []VATBand{{Rate: rateEightPercent, Net: money(1)}}}}

	if band := f.vatBand(rateFivePercent); !band.Net.IsZero() || !band.Tax.IsZero() {
		t.Errorf("a rate nobody used produced a band: %+v", band)
	}
}

// The rounding is the repository's: half away from zero, the direction
// internal/vat rounds a tax in.
func TestRoundDong_GoesHalfAwayFromZero(t *testing.T) {
	cases := map[string]string{
		"0.5":  "1",
		"-0.5": "-1",
		"1.4":  "1",
		"2.5":  "3",
		"49":   "49",
	}

	for amount, want := range cases {
		got := roundDong(decimal.RequireFromString(amount))
		if got.String() != want {
			t.Errorf("roundDong(%s) = %s, want %s", amount, got, want)
		}
	}

	// The reduction of a band that rounds to nothing is nothing, and one that
	// rounds up is one đồng: truncating would have lost the odd đồng the
	// repository's rounding keeps.
	if got := vatReduction(money(49)); !got.Equal(money(1)) {
		t.Errorf("the reduction on 49 đồng = %s, want 1: 0.98 đồng rounds away from zero", got)
	}
	if got := vatReduction(money(75)); !got.Equal(money(2)) {
		t.Errorf("the reduction on 75 đồng = %s, want 2: 1.5 đồng rounds away from zero", got)
	}
}

func TestVATReduction_IsTheDifferenceTheWindowGivesUp(t *testing.T) {
	if got := vatReduction(money(50_000_000)); !got.Equal(money(1_000_000)) {
		t.Errorf("the reduction on 50,000,000 = %s, want 1,000,000", got)
	}
	if standardRateDelta != rateTenPercent-rateEightPercent {
		t.Errorf("the window gives up %d basis points and the rates differ by %d",
			standardRateDelta, rateTenPercent-rateEightPercent)
	}
}

func TestWithinVATWindow_EndsOnTheLastDayOfTheWindow(t *testing.T) {
	cases := map[string]bool{
		"2026-12-30": true,
		"2026-12-31": true,
		"2027-01-01": false,
		"2025-06-30": true,
		"not a date": false,
	}

	for end, want := range cases {
		f := figures{period: ReportingPeriod{EndDate: end}}
		if got := f.withinVATWindow(); got != want {
			t.Errorf("a period ending %s is within the window: %v, want %v", end, got, want)
		}
	}
}

var _ = fmt.Sprintf

package statutory

import (
	"context"
	"errors"
	"fingo/internal/money"
	"testing"
)

// The corporate income tax rate is the one the year's turnover selects.
func TestCITRate_FollowsTheTurnoverBands(t *testing.T) {
	cases := map[string]struct {
		turnover int64
		want     int32
	}{
		"a small enterprise":      {2_000_000_000, 1500},
		"exactly at the band":     {3_000_000_000, 1500},
		"just over it":            {3_000_000_001, 1700},
		"at the upper band":       {50_000_000_000, 1700},
		"just over the upper one": {50_000_000_001, 2000},
		"a large enterprise":      {900_000_000_000, 2000},
		"nothing at all":          {0, 1500},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			f := figures{turnover: money.FromInt(tc.turnover)}
			if got := f.citRate(); int32(got) != tc.want {
				t.Errorf("a turnover of %d selects %d basis points, want %d", tc.turnover, got, tc.want)
			}
		})
	}
}

// A fact the ledger does not hold and the caller cannot supply stops the filing:
// a return computed without it would be a guess with a form number on it.
func TestGenerate_StopsWhenAFactCannotBeRead(t *testing.T) {
	f := newFixture(t)
	f.facts.err = errors.New("the invoice book is unreadable")

	if _, err := f.service.Generate(context.Background(), "2026-01"); err == nil {
		t.Fatal("a filing was produced without the facts it needs")
	}
}

// Output that cannot hold up as a filing is refused, and says which rule it
// broke.
func TestOutput_ValidateRefusesWhatIsNotAFiling(t *testing.T) {
	good := Output{
		Period: ReportingPeriod{ID: "2026-01", StartDate: "2026-01-01", EndDate: "2026-01-31", Regime: RegimeTT133},
		Statements: []Statement{{
			Code:      "01/GTGT",
			Name:      "Tờ khai thuế GTGT",
			Authority: authorityVATReturn,
			Lines: []LineItem{{
				Statement:  "01/GTGT",
				Label:      "Thuế GTGT phải nộp",
				Reference:  authorityVATReturn,
				Confidence: Secondary,
			}},
		}},
	}

	if err := good.Validate(); err != nil {
		t.Fatalf("a filing that holds up was refused: %v", err)
	}

	cases := map[string]func(*Output){
		"a statement with no code":      func(o *Output) { o.Statements[0].Code = "" },
		"a statement with no authority": func(o *Output) { o.Statements[0].Authority = "" },
		"a line with no label":          func(o *Output) { o.Statements[0].Lines[0].Label = "" },
		"a line with no basis":          func(o *Output) { o.Statements[0].Lines[0].Reference = "" },
		"a line with no confidence":     func(o *Output) { o.Statements[0].Lines[0].Confidence = "" },
		"a line the basis has not resolved": func(o *Output) {
			o.Statements[0].Lines[0].Confidence = Unresolved
		},
	}

	for name, spoil := range cases {
		t.Run(name, func(t *testing.T) {
			// A fresh filing each time: the spoilers edit in place, and one
			// test's damage must not reach another's.
			broken := goodFiling()
			spoil(&broken)

			if err := broken.Validate(); err == nil {
				t.Fatal("output that is not a filing passed validation")
			}
		})
	}
}

// goodFiling is one statement with one line.
func goodFiling() Output {
	return Output{
		Period: ReportingPeriod{ID: "2026-01", StartDate: "2026-01-01", EndDate: "2026-01-31", Regime: RegimeTT133},
		Statements: []Statement{{
			Code:      "01/GTGT",
			Name:      "Tờ khai thuế GTGT",
			Authority: authorityVATReturn,
			Lines: []LineItem{{
				Statement:  "01/GTGT",
				Label:      "Thuế GTGT phải nộp",
				Reference:  authorityVATReturn,
				Confidence: Secondary,
			}},
		}},
	}
}

// The typographic characters this package writes reach the page as characters a
// standard font has.
func TestASCII_MapsThePunctuationThisPackageWrites(t *testing.T) {
	cases := map[string]string{
		"b – c":   "b - c",
		"b — c":   "b - c",
		"b − c":   "b - c",
		"‘a’":     "'a'",
		"“a”":     `"a"`,
		"and so…": "and so.",
		"≤ 3bn":   "< 3bn",
		"≥ 6mo":   "> 6mo",
		"2 × 3":   "2 x 3",
		"¿qué?":   "?que?",
	}

	for written, want := range cases {
		if got := ascii(written); got != want {
			t.Errorf("ascii(%q) = %q, want %q", written, got, want)
		}
	}
}

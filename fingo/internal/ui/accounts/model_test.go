package accounts_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"fingo/internal/domain/gl"
	"fingo/internal/ui/accounts"
)

// chart is a Source backed by a fixed slice, so the model can be tested without
// a database.
type chart struct {
	accounts []gl.Account
	err      error
}

func (c chart) Accounts(context.Context) ([]gl.Account, error) {
	return c.accounts, c.err
}

func sample() []gl.Account {
	// Deliberately out of order, and deliberately carrying a contra account:
	// hao mòn TSCĐ (214) is an ASSET account that carries a CREDIT balance.
	return []gl.Account{
		{Code: "131", Name: "Phải thu khách hàng", Type: gl.AccountTypeAsset, Nature: gl.NatureDebit, IsActive: true},
		{Code: "111", Name: "Tiền mặt", Type: gl.AccountTypeAsset, Nature: gl.NatureDebit, IsActive: true},
		{Code: "214", Name: "Hao mòn tài sản cố định", Type: gl.AccountTypeAsset, Nature: gl.NatureCredit, IsActive: true},
		{Code: "331", Name: "Phải trả cho người bán", Type: gl.AccountTypeLiability, Nature: gl.NatureCredit, IsActive: true},
		{Code: "511", Name: "Doanh thu bán hàng", Type: gl.AccountTypeRevenue, Nature: gl.NatureCredit, IsActive: true},
		{Code: "911", Name: "Xác định kết quả kinh doanh", Type: gl.AccountTypeEquity, Nature: gl.NatureDual, IsActive: true},
	}
}

func loaded(t *testing.T, src accounts.Source) *accounts.Model {
	t.Helper()

	m := &accounts.Model{}
	if err := m.Load(t.Context(), src); err != nil {
		t.Fatalf("Load: %v", err)
	}

	return m
}

// TestModel_OrdersByCode — an accountant reads a chart by account number, and
// the numbers are the order. Sorting them as strings is right: "111" before
// "1121" before "131" is what a chart of accounts *is*.
func TestModel_OrdersByCode(t *testing.T) {
	m := loaded(t, chart{accounts: sample()})

	var codes []string
	for _, r := range m.Rows() {
		codes = append(codes, r.Code)
	}

	want := []string{"111", "131", "214", "331", "511", "911"}
	if strings.Join(codes, ",") != strings.Join(want, ",") {
		t.Errorf("codes = %v, want %v", codes, want)
	}
}

// TestModel_SpeaksTheAccountantsWords — the enum is the domain's; the screen is
// the operator's. Rendering "CREDIT" into a column headed "Tính chất" tells an
// accountant nothing they can use.
func TestModel_SpeaksTheAccountantsWords(t *testing.T) {
	m := loaded(t, chart{accounts: sample()})

	byCode := map[string]accounts.Row{}
	for _, r := range m.Rows() {
		byCode[r.Code] = r
	}

	tests := []struct{ code, kind, nature string }{
		{"111", "Tài sản", "Dư Nợ"},
		{"331", "Nợ phải trả", "Dư Có"},
		{"511", "Doanh thu", "Dư Có"},
		{"911", "Vốn chủ sở hữu", "Lưỡng tính"},
	}

	for _, tt := range tests {
		t.Run(tt.code, func(t *testing.T) {
			got, ok := byCode[tt.code]
			if !ok {
				t.Fatalf("%s is missing from the rows", tt.code)
			}
			if got.Kind != tt.kind {
				t.Errorf("%s kind = %q, want %q", tt.code, got.Kind, tt.kind)
			}
			if got.Nature != tt.nature {
				t.Errorf("%s nature = %q, want %q", tt.code, got.Nature, tt.nature)
			}
		})
	}
}

// TestModel_AContraAccountKeepsItsOwnNature — 214 is an asset that carries a
// credit balance. The screen must show what the account does, not what its type
// usually does, or an accountant reading the chart is told the wrong thing.
func TestModel_AContraAccountKeepsItsOwnNature(t *testing.T) {
	m := loaded(t, chart{accounts: sample()})

	for _, r := range m.Rows() {
		if r.Code != "214" {
			continue
		}

		if r.Kind != "Tài sản" {
			t.Errorf("214 kind = %q, want Tài sản", r.Kind)
		}
		if r.Nature != "Dư Có" {
			t.Errorf("214 nature = %q, want Dư Có — it is a contra account", r.Nature)
		}

		return
	}

	t.Fatal("214 is missing from the rows")
}

// TestModel_FiltersOnCodeAndOnName — the two things an accountant has in hand
// when looking for an account: the number, or what it is called.
func TestModel_FiltersOnCodeAndOnName(t *testing.T) {
	tests := []struct {
		name  string
		query string
		want  []string
	}{
		{"an account number", "131", []string{"131"}},
		{"a prefix of a number", "1", []string{"111", "131"}},
		{"a word from the name", "phải trả", []string{"331"}},
		{"a word in the middle", "khách", []string{"131"}},
		{"no diacritics at all", "phai thu", []string{"131"}},
		{"no diacritics and no case", "HAO MON", []string{"214"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := loaded(t, chart{accounts: sample()})
			m.Filter(tt.query)

			var codes []string
			for _, r := range m.Rows() {
				codes = append(codes, r.Code)
			}

			if strings.Join(codes, ",") != strings.Join(tt.want, ",") {
				t.Errorf("Filter(%q) gave %v, want %v", tt.query, codes, tt.want)
			}
		})
	}
}

// TestModel_AnEmptyChartAndAnEmptySearchSayDifferentThings — this is the rule
// the whole product runs on: never an empty table.
//
// "This client has no accounts" and "nothing matches what you typed" are
// different facts, and an operator who cannot tell them apart will conclude the
// application is broken, or worse, that the account does not exist.
func TestModel_AnEmptyChartAndAnEmptySearchSayDifferentThings(t *testing.T) {
	noAccounts := loaded(t, chart{})
	if len(noAccounts.Rows()) != 0 {
		t.Fatalf("a chart with no accounts produced %d rows", len(noAccounts.Rows()))
	}
	if noAccounts.Empty() == "" {
		t.Error("an empty chart says nothing")
	}

	filtered := loaded(t, chart{accounts: sample()})
	filtered.Filter("zzzz")

	if len(filtered.Rows()) != 0 {
		t.Fatalf("a search for zzzz produced %d rows", len(filtered.Rows()))
	}
	if filtered.Empty() == "" {
		t.Error("a search with no matches says nothing")
	}
	if filtered.Empty() == noAccounts.Empty() {
		t.Errorf("both states read %q, so an operator cannot tell them apart", filtered.Empty())
	}
}

// TestModel_ClearingTheFilterBringsEverythingBack — a filter is not a deletion.
func TestModel_ClearingTheFilterBringsEverythingBack(t *testing.T) {
	m := loaded(t, chart{accounts: sample()})

	m.Filter("131")
	if len(m.Rows()) != 1 {
		t.Fatalf("after filtering, %d rows", len(m.Rows()))
	}

	m.Filter("")
	if len(m.Rows()) != len(sample()) {
		t.Errorf("after clearing the filter, %d rows, want %d", len(m.Rows()), len(sample()))
	}

	m.Filter("   ")
	if len(m.Rows()) != len(sample()) {
		t.Errorf("a filter of spaces gave %d rows, want %d — spaces are not a search", len(m.Rows()), len(sample()))
	}
}

// TestModel_SummaryStatesWhatIsShown — the operator needs to know whether they
// are looking at the whole chart or part of it.
func TestModel_SummaryStatesWhatIsShown(t *testing.T) {
	m := loaded(t, chart{accounts: sample()})

	if got := m.Summary(); !strings.Contains(got, "6") {
		t.Errorf("the unfiltered summary %q does not state the total", got)
	}

	m.Filter("phải")
	got := m.Summary()
	if !strings.Contains(got, "2") {
		t.Errorf("the filtered summary %q does not state how many are shown", got)
	}
	if !strings.Contains(got, "6") {
		t.Errorf("the filtered summary %q does not state the total", got)
	}
}

// TestModel_LoadReportsAFailure — a screen that swallows the error and renders
// an empty chart has told the operator their books are empty.
func TestModel_LoadReportsAFailure(t *testing.T) {
	boom := errors.New("the database is not reachable")

	m := &accounts.Model{}
	err := m.Load(t.Context(), chart{err: boom})

	if err == nil {
		t.Fatal("Load = nil, want the source's error")
	}
	if !errors.Is(err, boom) {
		t.Errorf("Load = %v, want it to wrap %v", err, boom)
	}
}

// TestModel_AnAccountNumberMatchesByPrefixNotAnywhere — a product decision, and
// one that looks like a bug from the other side.
//
// An accountant types "1" to see the 1xx accounts. If the number is matched as a
// substring, the answer is 111, 131, 214, 511 and 911 — every account with a 1
// anywhere in it — and the group they asked for is lost among accounts they did
// not. Matching by prefix is what makes a chart of accounts searchable by its
// own structure.
//
// It is pinned by name because "the filter is too strict" is the obvious-looking
// change, and it makes the screen worse.
func TestModel_AnAccountNumberMatchesByPrefixNotAnywhere(t *testing.T) {
	m := loaded(t, chart{accounts: sample()})
	m.Filter("1")

	var codes []string
	for _, r := range m.Rows() {
		codes = append(codes, r.Code)
	}

	got := strings.Join(codes, ",")
	if got != "111,131" {
		t.Errorf("filtering on %q gave %s, want 111,131 — 214, 511 and 911 merely contain a 1", "1", got)
	}

	// And a digit that no account number starts with finds nothing at all,
	// even though 111 contains it.
	m.Filter("11")
	if got := len(m.Rows()); got != 1 {
		t.Errorf("filtering on %q gave %d rows, want 1", "11", got)
	}
}

// TestModel_TableIsWhatTheViewDraws — the seam between the screen's rules and
// the widget that paints them.
//
// The view is given column headings, rows of strings, and the sentence to show
// when there are no rows. Everything decided here is asserted above; everything
// the view does with it is drawing. That is the whole reason the model is
// testable and the shell is not.
func TestModel_TableIsWhatTheViewDraws(t *testing.T) {
	m := loaded(t, chart{accounts: sample()})

	headers, rows, empty := m.Table()

	want := []string{"Mã", "Tên tài khoản", "Loại", "Tính chất"}
	if strings.Join(headers, "|") != strings.Join(want, "|") {
		t.Errorf("headers = %v, want %v", headers, want)
	}

	if len(rows) != len(m.Rows()) {
		t.Fatalf("the table has %d rows and the model has %d", len(rows), len(m.Rows()))
	}

	for i, r := range m.Rows() {
		got := strings.Join(rows[i], "|")
		want := strings.Join([]string{r.Code, r.Name, r.Kind, r.Nature}, "|")
		if got != want {
			t.Errorf("row %d = %q, want %q", i, got, want)
		}
	}

	// Every row has exactly as many cells as there are headings, or the view
	// draws a table with a column missing.
	for i, r := range rows {
		if len(r) != len(headers) {
			t.Errorf("row %d has %d cells and there are %d headings", i, len(r), len(headers))
		}
	}

	if empty != m.Empty() {
		t.Errorf("the table's empty message is %q, want %q", empty, m.Empty())
	}
}

// TestModel_TableFollowsTheFilter — what the view draws must be what the filter
// left, or the search box changes a count nobody can see.
func TestModel_TableFollowsTheFilter(t *testing.T) {
	m := loaded(t, chart{accounts: sample()})
	m.Filter("phai thu")

	_, rows, _ := m.Table()

	if len(rows) != 1 {
		t.Fatalf("after filtering, the table has %d rows, want 1", len(rows))
	}
	if rows[0][0] != "131" {
		t.Errorf("the table shows %q, want 131", rows[0][0])
	}
}

// TestModel_TableOnAnEmptyChartStillHasHeadings — a view given no headings
// cannot draw a header row, and an operator who filters down to nothing would
// watch the columns disappear.
func TestModel_TableOnAnEmptyChartStillHasHeadings(t *testing.T) {
	m := loaded(t, chart{})

	headers, rows, empty := m.Table()

	if len(headers) == 0 {
		t.Error("an empty chart produced no headings")
	}
	if len(rows) != 0 {
		t.Errorf("an empty chart produced %d rows", len(rows))
	}
	if empty == "" {
		t.Error("an empty chart produced no message")
	}
}

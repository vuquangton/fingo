package web_test

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"fingo/web"
	"fingo/web/books"
)

// The dashboard's position figures are the reports, read at a glance.
//
// Each figure is asserted against the page that owns it rather than against a
// number written down here. "Profit to date" is only worth showing if it is the
// same profit the profit and loss report states, and a hard-coded expectation
// would go on passing after the two had drifted apart.
//
// The figures are read from the data attributes, not from the text. The pages
// render money twice on purpose, and a test that un-formatted "1.100.000" back
// into a number would be testing its own parser.
func TestE2E_TheDashboardAgreesWithTheReports(t *testing.T) {
	server, client := stack(t)

	// Empty books are a position of zero, and the page says so rather than
	// leaving a reader to work out that nothing has happened yet.
	_, body := get(t, client, server.URL+"/")

	for _, name := range []string{"data-cash-total", "data-receivable", "data-payable", "data-profit"} {
		if got := attr(t, body, name); got != 0 {
			t.Fatalf("%s is %d on empty books, want 0", name, got)
		}
	}

	mustContain(t, body, "Needs attention")
	mustContain(t, body, "Nothing has been posted yet.")

	// Sell 1.000.000 net at 10% on credit: 1.100.000 owed to us, and revenue is
	// the only thing the result is made of so far.
	status, body := post(t, client, server.URL+"/documents/invoice", url.Values{
		"customer":        {"Công ty Bình Minh"},
		"date":            {theDate},
		"net_amount":      {"1.000.000"},
		"vat_rate_id":     {"VAT10"},
		"description":     {"Bán hàng"},
		"idempotency_key": {"dash-1"},
	})
	if status != http.StatusOK {
		t.Fatalf("creating the invoice answered %d:\n%s", status, body)
	}

	id := invoiceID(t, body)

	_, body = get(t, client, server.URL+"/")

	if got := attr(t, body, "data-receivable"); got != 1_100_000 {
		t.Fatalf("owed to us is %d, want 1100000", got)
	}
	if got := attr(t, body, "data-cash-total"); got != 0 {
		t.Fatalf("cash is %d, want 0: a credit sale moves no money", got)
	}
	if got := attr(t, body, "data-payable"); got != 0 {
		t.Fatalf("we owe is %d, want 0: selling to somebody owes nobody", got)
	}

	// The result on the dashboard is the one the report states.
	dashboardProfit := attr(t, body, "data-profit")

	_, profitAndLoss := get(t, client, server.URL+"/reports/profit-and-loss")
	if want := attr(t, profitAndLoss, "data-net-profit"); dashboardProfit != want {
		t.Fatalf("the dashboard reports a profit of %d; the profit and loss report states %d",
			dashboardProfit, want)
	}

	// Nothing has been paid, so the one thing the books can prove is
	// outstanding is the money they are owed.
	mustContain(t, body, "Invoices outstanding")

	// Pay it in full, against the invoice. Naming what the money settles is
	// what clears the receivable; an unapplied receipt would leave the debt
	// standing, which is the whole reason that field exists.
	status, body = post(t, client, server.URL+"/documents/payment", url.Values{
		"customer":        {"Công ty Bình Minh"},
		"date":            {theDate},
		"amount":          {"1.100.000"},
		"account_code":    {"1111"},
		"against":         {id},
		"idempotency_key": {"dash-2"},
	})
	if status != http.StatusOK {
		t.Fatalf("recording the payment answered %d:\n%s", status, body)
	}

	_, body = get(t, client, server.URL+"/")

	if got := attr(t, body, "data-cash-total"); got != 1_100_000 {
		t.Fatalf("cash is %d, want 1100000", got)
	}
	if got := attr(t, body, "data-receivable"); got != 0 {
		t.Fatalf("owed to us is %d, want 0 once the invoice is paid", got)
	}
}

// A period that has ended and was never locked is work, and the dashboard is
// where an operator should find out about it.
//
// The books seed exactly one period, covering the year the clock is in, so this
// cannot be reached by reading them at a later date — a second period has to be
// added, the way an operator adds one. The set has to stay contiguous, which is
// what makes "the previous year" the only period that can be in this state.
func TestE2E_TheDashboardFlagsAPeriodThatEndedUnlocked(t *testing.T) {
	server, client := stackAt(t, time.Date(2028, time.June, 1, 9, 0, 0, 0, time.UTC))

	// Nothing is outstanding while the only period still covers today.
	_, body := get(t, client, server.URL+"/")
	mustContain(t, body, "Nothing the ledger can see is outstanding.")

	status, body := post(t, client, server.URL+"/settings/periods", url.Values{
		"id":              {"FY2027"},
		"from":            {"2027-01-01"},
		"to":              {"2027-12-31"},
		"idempotency_key": {"dash-period-1"},
	})
	if status != http.StatusOK {
		t.Fatalf("adding the period answered %d:\n%s", status, body)
	}

	_, body = get(t, client, server.URL+"/")

	// FY2027 ended five months ago and is still open to postings, so an entry
	// dated back into it would change accounts somebody has already read.
	mustContain(t, body, "Periods ended but not locked")
}

// stackAt is stack with the books told a different day.
//
// A second entry point rather than a change to stack, because the fixtures'
// fixed date cannot reach the one rule on the dashboard that depends on the
// calendar: a period that has ended and is still unlocked.
func stackAt(t *testing.T, day time.Time) (*httptest.Server, *http.Client) {
	t.Helper()

	ledger, err := books.New(books.WithClock(func() time.Time { return day }))
	if err != nil {
		t.Fatalf("books.New: %v", err)
	}

	server, err := web.NewServer(ledger)
	if err != nil {
		t.Fatalf("web.NewServer: %v", err)
	}

	httpServer := httptest.NewServer(server.Handler())
	t.Cleanup(httpServer.Close)

	client := httpServer.Client()
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }

	return httpServer, client
}

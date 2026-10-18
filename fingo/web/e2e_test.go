package web_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"fingo/web"
	"fingo/web/books"
)

// theDay is the date the books are told it is. Fixed, so the period the books
// seed and the dates the tests post to cannot drift apart.
var theDay = time.Date(2026, time.March, 15, 9, 0, 0, 0, time.UTC)

// theDate is the business date every document in these tests carries.
const theDate = "2026-03-01"

// stack builds the whole application — books, templates, routes, middleware —
// behind a real HTTP server, and returns a client that does not follow
// redirects.
//
// A real server rather than a recorder: the flow under test is a browser's, and
// it includes things a recorder does not exercise such as the idempotency key
// being lifted out of a form body.
func stack(t *testing.T) (*httptest.Server, *http.Client) {
	t.Helper()

	ledger, err := books.New(books.WithClock(func() time.Time { return theDay }))
	if err != nil {
		t.Fatalf("books.New: %v", err)
	}

	var minted int

	server, err := web.NewServer(ledger, web.WithKeyFunc(func() string {
		minted++

		return "k-" + strconv.Itoa(minted)
	}))
	if err != nil {
		t.Fatalf("web.NewServer: %v", err)
	}

	httpServer := httptest.NewServer(server.Handler())
	t.Cleanup(httpServer.Close)

	client := httpServer.Client()
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }

	return httpServer, client
}

// send makes a request and returns the status and the body.
//
// form is nil for a GET. The request carries no HX-Request header and no
// Idempotency-Key header, which is exactly what a browser with scripts turned
// off sends — the whole no-JavaScript claim is that this is enough.
func send(t *testing.T, c *http.Client, method, target string, form url.Values) (int, string) {
	t.Helper()

	var body io.Reader
	if form != nil {
		body = strings.NewReader(form.Encode())
	}

	req, err := http.NewRequest(method, target, body)
	if err != nil {
		t.Fatalf("new request %s %s: %v", method, target, err)
	}
	if form != nil {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}

	resp, err := c.Do(req)
	if err != nil {
		t.Fatalf("%s %s: %v", method, target, err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read %s %s: %v", method, target, err)
	}

	return resp.StatusCode, string(raw)
}

func get(t *testing.T, c *http.Client, target string) (int, string) {
	t.Helper()

	return send(t, c, http.MethodGet, target, nil)
}

func post(t *testing.T, c *http.Client, target string, form url.Values) (int, string) {
	t.Helper()

	return send(t, c, http.MethodPost, target, form)
}

// attr reads a data attribute the page carries alongside a figure.
//
// The pages render money twice on purpose: once as text a person reads, and once
// as the number in a data attribute. A test that had to un-format "1.100.000"
// back into a number would be testing its own parser.
func attr(t *testing.T, body, name string) int64 {
	t.Helper()

	pattern := regexp.MustCompile(regexp.QuoteMeta(name) + `="(-?[0-9]+)"`)

	match := pattern.FindStringSubmatch(body)
	if match == nil {
		t.Fatalf("%s is not on the page:\n%s", name, body)
	}

	value, err := strconv.ParseInt(match[1], 10, 64)
	if err != nil {
		t.Fatalf("%s = %q, which is not a number", name, match[1])
	}

	return value
}

func mustContain(t *testing.T, body, want string) {
	t.Helper()

	if !strings.Contains(body, want) {
		t.Fatalf("the page does not contain %q:\n%s", want, body)
	}
}

// invoiceID pulls the first invoice link out of a page.
func invoiceID(t *testing.T, body string) string {
	t.Helper()

	match := regexp.MustCompile(`/documents/(inv-[0-9]+)`).FindStringSubmatch(body)
	if match == nil {
		t.Fatalf("no invoice is linked from the page:\n%s", body)
	}

	return match[1]
}

// The whole of the first flow, driven the way a browser drives it: create an
// invoice, find it in the list, open it, pay it, and watch the status move.
func TestE2E_CreateInvoiceFindItInTheListAndPayIt(t *testing.T) {
	server, client := stack(t)

	status, body := get(t, client, server.URL+"/")
	if status != http.StatusOK {
		t.Fatalf("the dashboard answered %d", status)
	}
	mustContain(t, body, "Dashboard")
	mustContain(t, body, "FY2026")

	// Create the invoice. 1.000.000 net at 10% is 1.100.000.
	status, body = post(t, client, server.URL+"/documents/invoice", url.Values{
		"customer":        {"Công ty Bình Minh"},
		"date":            {theDate},
		"net_amount":      {"1.000.000"},
		"vat_rate_id":     {"VAT10"},
		"description":     {"Bán hàng"},
		"idempotency_key": {"inv-create-1"},
	})
	if status != http.StatusOK {
		t.Fatalf("creating the invoice answered %d:\n%s", status, body)
	}
	mustContain(t, body, "The invoice was recorded and posted.")
	mustContain(t, body, "Công ty Bình Minh")

	id := invoiceID(t, body)

	// It is in the list, open, and worth what the tax made it worth.
	status, body = get(t, client, server.URL+"/documents?type=invoice")
	if status != http.StatusOK {
		t.Fatalf("the document list answered %d", status)
	}
	mustContain(t, body, id)
	mustContain(t, body, "1.100.000")
	mustContain(t, body, `data-status="open"`)

	// Open it.
	status, body = get(t, client, server.URL+"/documents/"+id)
	if status != http.StatusOK {
		t.Fatalf("the document answered %d", status)
	}
	mustContain(t, body, `data-status="open"`)
	if got := attr(t, body, "data-outstanding"); got != 1_100_000 {
		t.Fatalf("outstanding is %d, want 1100000", got)
	}

	// The pay form is pre-filled with what is owed.
	status, body = get(t, client, server.URL+"/documents/"+id+"/pay")
	if status != http.StatusOK {
		t.Fatalf("the pay form answered %d", status)
	}
	mustContain(t, body, `value="1.100.000"`)
	mustContain(t, body, `name="against" value="`+id+`"`)

	// Pay it in full, into cash.
	status, body = post(t, client, server.URL+"/documents/payment", url.Values{
		"customer":        {"Công ty Bình Minh"},
		"date":            {theDate},
		"amount":          {"1.100.000"},
		"account_code":    {"1111"},
		"against":         {id},
		"idempotency_key": {"inv-pay-1"},
	})
	if status != http.StatusOK {
		t.Fatalf("paying answered %d:\n%s", status, body)
	}
	mustContain(t, body, "The payment was recorded and applied.")

	// The status moved, and nothing is outstanding any more.
	mustContain(t, body, `data-status="paid"`)
	if got := attr(t, body, "data-outstanding"); got != 0 {
		t.Fatalf("outstanding after paying is %d, want 0", got)
	}

	// And it is still paid when it is read back from scratch.
	_, body = get(t, client, server.URL+"/documents/"+id)
	mustContain(t, body, `data-status="paid"`)
	if got := attr(t, body, "data-outstanding"); got != 0 {
		t.Fatalf("outstanding on a fresh read is %d, want 0", got)
	}
}

// A part payment moves the status to partially paid and leaves the rest owed.
func TestE2E_APartPaymentLeavesTheRestOwed(t *testing.T) {
	server, client := stack(t)

	_, body := post(t, client, server.URL+"/documents/invoice", url.Values{
		"customer":        {"Công ty Đông Á"},
		"date":            {theDate},
		"net_amount":      {"2.000.000"},
		"idempotency_key": {"part-1"},
	})

	id := invoiceID(t, body)

	_, body = post(t, client, server.URL+"/documents/payment", url.Values{
		"customer":        {"Công ty Đông Á"},
		"date":            {theDate},
		"amount":          {"500.000"},
		"account_code":    {"1111"},
		"against":         {id},
		"idempotency_key": {"part-2"},
	})
	mustContain(t, body, `data-status="partially_paid"`)

	if got := attr(t, body, "data-outstanding"); got != 1_500_000 {
		t.Fatalf("outstanding is %d, want 1500000", got)
	}
}

// The second flow: the trial balance a person is shown adds up on both sides.
func TestE2E_TrialBalanceShowsDebitsEqualCredits(t *testing.T) {
	server, client := stack(t)

	// Post something with two credits and a debit, which is the shape that a
	// single-sided check would pass by accident.
	_, body := post(t, client, server.URL+"/documents/invoice", url.Values{
		"customer":        {"Công ty Bình Minh"},
		"date":            {theDate},
		"net_amount":      {"1.000.000"},
		"vat_rate_id":     {"VAT10"},
		"idempotency_key": {"tb-1"},
	})
	mustContain(t, body, "The invoice was recorded and posted.")

	// And a journal entry, so the report is summing more than one voucher.
	journal := url.Values{"date": {theDate}, "description": {"Nộp tiền vào ngân hàng"}, "idempotency_key": {"tb-2"}}
	journal.Set("account_1", "1121")
	journal.Set("side_1", "debit")
	journal.Set("amount_1", "400.000")
	journal.Set("account_2", "1111")
	journal.Set("side_2", "credit")
	journal.Set("amount_2", "400.000")

	status, body := post(t, client, server.URL+"/documents/journal", journal)
	if status != http.StatusOK {
		t.Fatalf("the journal answered %d:\n%s", status, body)
	}
	mustContain(t, body, "The journal entry was recorded and posted.")

	status, body = get(t, client, server.URL+"/reports/trial-balance")
	if status != http.StatusOK {
		t.Fatalf("the trial balance answered %d", status)
	}

	debits := attr(t, body, "data-total-debits")
	credits := attr(t, body, "data-total-credits")

	// Non-zero first: 0 == 0 would be a true statement about an empty report
	// and would prove nothing.
	if debits == 0 || credits == 0 {
		t.Fatalf("the trial balance totals are %d and %d; nothing was posted", debits, credits)
	}
	if debits != credits {
		t.Fatalf("debits %d do not equal credits %d", debits, credits)
	}
	if want := int64(1_100_000 + 400_000); debits != want {
		t.Fatalf("period debits are %d, want %d", debits, want)
	}

	// The closing columns balance too, and the page says so.
	closingDebits := attr(t, body, "data-total-closing-debits")
	closingCredits := attr(t, body, "data-total-closing-credits")
	if closingDebits != closingCredits {
		t.Fatalf("closing debits %d do not equal closing credits %d", closingDebits, closingCredits)
	}
	mustContain(t, body, `data-balanced="1"`)

	// The same figures are on the page as text, so the claim is about what a
	// person is shown and not only about an attribute.
	mustContain(t, body, "1.500.000")
}

// Every form has to be usable with scripts turned off. That means a real action,
// a real method, a key the server supplied, and an answer that is a whole
// document.
func TestE2E_EveryFormWorksWithoutJavaScript(t *testing.T) {
	server, client := stack(t)

	forms := []struct {
		name   string
		page   string
		action string
		form   url.Values
	}{
		{
			name:   "invoice",
			page:   "/documents/new?type=invoice",
			action: "/documents/invoice",
			form: url.Values{
				"customer":        {"Công ty Bình Minh"},
				"date":            {theDate},
				"net_amount":      {"500.000"},
				"vat_rate_id":     {"VAT10"},
				"idempotency_key": {"nojs-invoice"},
			},
		},
		{
			name:   "payment",
			page:   "/documents/new?type=payment",
			action: "/documents/payment",
			form: url.Values{
				"customer":        {"Công ty Bình Minh"},
				"date":            {theDate},
				"amount":          {"250.000"},
				"account_code":    {"1111"},
				"idempotency_key": {"nojs-payment"},
			},
		},
		{
			name:   "journal",
			page:   "/documents/new?type=journal",
			action: "/documents/journal",
			form:   buildJournal(),
		},
		{
			name:   "account",
			page:   "/accounts/new",
			action: "/accounts",
			form: url.Values{
				// 9999 is not in the TT133 chart, so the form is adding
				// something rather than colliding with the seed.
				"code":            {"9999"},
				"name":            {"Chi phí trả trước"},
				"type":            {"ASSET"},
				"nature":          {"DEBIT"},
				"idempotency_key": {"nojs-account"},
			},
		},
		{
			name:   "period",
			page:   "/settings",
			action: "/settings/periods",
			form: url.Values{
				"id":              {"FY2027"},
				"from":            {"2027-01-01"},
				"to":              {"2027-12-31"},
				"idempotency_key": {"nojs-period"},
			},
		},
	}

	for _, tc := range forms {
		t.Run(tc.name, func(t *testing.T) {
			// The page renders without scripts, and the form is a real form:
			// a method and an action a browser can submit on its own.
			status, body := get(t, client, server.URL+tc.page)
			if status != http.StatusOK {
				t.Fatalf("%s answered %d", tc.page, status)
			}
			mustContain(t, body, `method="post"`)
			mustContain(t, body, `action="`+tc.action+`"`)
			mustContain(t, body, `name="idempotency_key"`)
			mustContain(t, body, "<!doctype html>")

			// Submitting it with no HX-Request header answers with a whole
			// document, which is what a browser with no HTMX needs.
			status, body = post(t, client, server.URL+tc.action, tc.form)
			if status != http.StatusOK {
				t.Fatalf("%s answered %d:\n%s", tc.action, status, body)
			}
			mustContain(t, body, "<!doctype html>")
			mustContain(t, body, `data-flash="1"`)
		})
	}
}

// Without JavaScript the server still has to hand every form a usable key, and
// the form has to carry it.
func TestE2E_TheServerSuppliesTheIdempotencyKey(t *testing.T) {
	server, client := stack(t)

	_, body := get(t, client, server.URL+"/documents/new?type=invoice")

	match := regexp.MustCompile(`name="idempotency_key" value="([^"]+)"`).FindStringSubmatch(body)
	if match == nil {
		t.Fatalf("the invoice form carries no idempotency key:\n%s", body)
	}
	if strings.TrimSpace(match[1]) == "" {
		t.Fatal("the invoice form carries an empty idempotency key")
	}

	// And the page asks the browser for a better one where it can.
	mustContain(t, body, "crypto.randomUUID")
}

// The key is what makes a repeated write safe. Sending the same request twice —
// a double-click, a retry — must not write a second invoice.
func TestE2E_ARepeatedWriteIsAnsweredFromTheFirst(t *testing.T) {
	server, client := stack(t)

	form := url.Values{
		"customer":        {"Công ty Bình Minh"},
		"date":            {theDate},
		"net_amount":      {"750.000"},
		"idempotency_key": {"same-key-twice"},
	}

	status, first := post(t, client, server.URL+"/documents/invoice", form)
	if status != http.StatusOK {
		t.Fatalf("the first write answered %d", status)
	}

	status, second := post(t, client, server.URL+"/documents/invoice", form)
	if status != http.StatusOK {
		t.Fatalf("the repeat answered %d", status)
	}

	if first != second {
		t.Fatal("the repeat was answered with something other than the first answer")
	}

	// One invoice, not two.
	_, list := get(t, client, server.URL+"/documents?type=invoice")

	if got := strings.Count(list, "/documents/inv-"); got != 1 {
		t.Fatalf("the list holds %d invoices, want 1:\n%s", got, list)
	}
}

// A refused write says what was wrong, in the books' own words, and hands the
// form back filled in.
func TestE2E_ARefusedWriteShowsTheErrorVerbatim(t *testing.T) {
	server, client := stack(t)

	// A journal that does not balance.
	form := buildJournal()
	form.Set("amount_2", "300.000")
	form.Set("idempotency_key", "unbalanced")

	status, body := post(t, client, server.URL+"/documents/journal", form)
	if status != http.StatusUnprocessableEntity {
		t.Fatalf("an unbalanced journal answered %d, want 422:\n%s", status, body)
	}

	mustContain(t, body, `data-error="1"`)
	// The message names both sides, which is the whole point of showing it
	// rather than a code.
	mustContain(t, body, "debits")
	mustContain(t, body, "credits")
	mustContain(t, body, "400.000")
	mustContain(t, body, "300.000")

	// The form came back with what was typed still in it.
	mustContain(t, body, `name="amount_1" value="400.000"`)
}

// buildJournal is a balanced two-line entry moving cash to the bank.
func buildJournal() url.Values {
	form := url.Values{
		"date":            {theDate},
		"description":     {"Nộp tiền vào ngân hàng"},
		"idempotency_key": {"journal-1"},
	}
	form.Set("account_1", "1121")
	form.Set("side_1", "debit")
	form.Set("amount_1", "400.000")
	form.Set("account_2", "1111")
	form.Set("side_2", "credit")
	form.Set("amount_2", "400.000")

	return form
}

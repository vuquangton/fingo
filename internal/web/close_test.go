package web_test

import (
	"net/http"
	"net/url"
	"strings"
	"testing"

	"io"

	"github.com/jackc/pgx/v5"
)

// currentPeriod returns the id of the client's only period.
func (h *harness) currentPeriod(t *testing.T, clientID string) string {
	t.Helper()

	var id string

	// Scoped: an unscoped query is filtered to nothing by row-level security, so
	// the fixture must read through the same door the application does.
	if err := h.storeOf(t).InClient(t.Context(), h.scopeOf(t, clientID), func(tx pgx.Tx) error {
		return tx.QueryRow(t.Context(),
			`SELECT id::text FROM accounting_periods ORDER BY start_date DESC LIMIT 1`).Scan(&id)
	}); err != nil {
		t.Fatalf("read the period: %v", err)
	}
	return id
}

func TestCloseScreenListsThePeriod(t *testing.T) {
	h := newHarness(t)
	email := newEmail()
	h.user(t, email, "correct-horse", "admin")
	h.login(t, email, "correct-horse")
	client := h.newClient(t)
	h.pickClient(t, client)

	body := h.body(t, "/close")

	for _, want := range []string{"Khoá sổ kỳ kế toán", "Đang mở", "Khoá sổ kỳ này"} {
		if !strings.Contains(body, want) {
			t.Fatalf("the close screen does not contain %q\n%s", want, body)
		}
	}
}

func TestCloseCarriesTheResultAndLocksThePeriod(t *testing.T) {
	h := newHarness(t)
	email := newEmail()
	h.user(t, email, "correct-horse", "admin")
	h.login(t, email, "correct-horse")
	client := h.newClient(t)
	h.pickClient(t, client)
	h.postSale(t)

	// Give the client a year to close, the way the close screen does.
	h.body(t, "/close")
	periodID := h.currentPeriod(t, client)

	resp, err := h.client.PostForm(h.server.URL+"/close", url.Values{"period_id": {periodID}})
	if err != nil {
		t.Fatalf("close: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("POST /close = %d, want 200 after the redirect\n%s", resp.StatusCode, readAll(t, resp))
	}

	body := readAll(t, resp)
	if !strings.Contains(body, "Đã khoá sổ") {
		t.Fatalf("the close did not report success:\n%s", body)
	}

	// The result is carried, so every revenue account stands at zero.
	st := h.storeOf(t)
	scope := h.scopeOf(t, client)

	var revenueNet int64
	if err := st.InClient(t.Context(), scope, func(tx pgx.Tx) error {
		return tx.QueryRow(t.Context(), `
			SELECT COALESCE(SUM(CASE WHEN l.side = 'CREDIT' THEN l.amount ELSE -l.amount END), 0)
			  FROM voucher_lines l
			 JOIN accounts a ON a.client_id = l.client_id AND a.code = l.account_code
			 WHERE a.account_type = 'REVENUE'`).Scan(&revenueNet)
	}); err != nil {
		t.Fatalf("read revenue: %v", err)
	}
	if revenueNet != 0 {
		t.Fatalf("revenue still stands at %d after the close, want 0", revenueNet)
	}

	var locked bool
	if err := st.InClient(t.Context(), scope, func(tx pgx.Tx) error {
		return tx.QueryRow(t.Context(),
			`SELECT EXISTS (SELECT 1 FROM accounting_period_locks WHERE period_id = $1)`,
			periodID).Scan(&locked)
	}); err != nil {
		t.Fatalf("read the lock: %v", err)
	}
	if !locked {
		t.Fatal("the period was not locked")
	}

	var report int
	if err := st.InClient(t.Context(), scope, func(tx pgx.Tx) error {
		return tx.QueryRow(t.Context(),
			`SELECT count(*) FROM period_close_reports WHERE period_id = $1`, periodID).Scan(&report)
	}); err != nil {
		t.Fatalf("read the report: %v", err)
	}
	if report != 1 {
		t.Fatalf("closing reports = %d, want 1", report)
	}
}

func TestClosedPeriodRefusesANewVoucher(t *testing.T) {
	h := newHarness(t)
	email := newEmail()
	h.user(t, email, "correct-horse", "admin")
	h.login(t, email, "correct-horse")
	client := h.newClient(t)
	h.pickClient(t, client)
	h.postSale(t)

	h.body(t, "/close")
	periodID := h.currentPeriod(t, client)

	resp, err := h.client.PostForm(h.server.URL+"/close", url.Values{"period_id": {periodID}})
	if err != nil {
		t.Fatalf("close: %v", err)
	}
	_ = resp.Body.Close()

	// The period is now closed: a voucher dated inside it must be refused.
	post, err := h.client.PostForm(h.server.URL+"/entry", url.Values{
		"date":        {"2026-03-20"},
		"description": {"Bán thêm"},
		"account":     {"131", "5111"},
		"side":        {"DEBIT", "CREDIT"},
		"amount":      {"500.000", "500.000"},
	})
	if err != nil {
		t.Fatalf("post entry: %v", err)
	}
	defer func() { _ = post.Body.Close() }()

	if post.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("posting into a closed period = %d, want 422", post.StatusCode)
	}

	body := readAll(t, post)
	if !strings.Contains(strings.ToLower(body), "closed") && !strings.Contains(body, "khoá") {
		t.Fatalf("the refusal does not name the closed period:\n%s", body)
	}
}

func readAll(t *testing.T, resp *http.Response) string {
	t.Helper()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	return string(raw)
}

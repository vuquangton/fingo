package web_test

import (
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
)

// TestRootSendsAVisitorToTheSignInPage is the defect the drift report found: the
// operator guide says "open the address and sign in", and the address answered 404.
func TestRootSendsAVisitorToTheSignInPage(t *testing.T) {
	h := newHarness(t)

	resp, err := h.client.Get(h.server.URL + "/")
	if err != nil {
		t.Fatalf("get root: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET / = %d, want 200 after the redirect", resp.StatusCode)
	}
	if !strings.HasSuffix(resp.Request.URL.Path, "/login") {
		t.Fatalf("GET / landed on %s, want /login", resp.Request.URL.Path)
	}

	body := readAll(t, resp)
	for _, want := range []string{"Đăng nhập", "email", "password", "FinGo"} {
		if !strings.Contains(body, want) {
			t.Fatalf("the sign-in page does not contain %q:\n%s", want, body)
		}
	}
}

func TestLoginPageSignsInWithARedirect(t *testing.T) {
	h := newHarness(t)
	email := newEmail()
	h.user(t, email, "correct-horse", "accountant")

	// A browser sends Accept: text/html; http.Client does not, so the header is
	// set explicitly to exercise the form path rather than the API path.
	form := url.Values{"email": {email}, "password": {"correct-horse"}}
	req, err := http.NewRequest(http.MethodPost, h.server.URL+"/login", strings.NewReader(form.Encode()))
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "text/html")

	resp, err := h.client.Do(req)
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("browser login = %d, want 200 after the redirect", resp.StatusCode)
	}
	if !strings.HasSuffix(resp.Request.URL.Path, "/clients") {
		t.Fatalf("login landed on %s, want /clients", resp.Request.URL.Path)
	}
}

func TestLoginPageSaysNothingAboutWhichHalfWasWrong(t *testing.T) {
	h := newHarness(t)
	email := newEmail()
	h.user(t, email, "correct-horse", "accountant")

	attempt := func(address, password string) (int, string) {
		form := url.Values{"email": {address}, "password": {password}}
		req, err := http.NewRequest(http.MethodPost, h.server.URL+"/login", strings.NewReader(form.Encode()))
		if err != nil {
			t.Fatalf("request: %v", err)
		}
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("Accept", "text/html")

		resp, err := h.client.Do(req)
		if err != nil {
			t.Fatalf("login: %v", err)
		}
		defer func() { _ = resp.Body.Close() }()

		return resp.StatusCode, readAll(t, resp)
	}

	wrongPassword, bodyA := attempt(email, "wrong-password")
	unknownEmail, bodyB := attempt("nobody@example.invalid", "wrong-password")

	if wrongPassword != http.StatusUnauthorized || unknownEmail != http.StatusUnauthorized {
		t.Fatalf("statuses %d / %d, want 401 for both", wrongPassword, unknownEmail)
	}
	if bodyA != bodyB {
		t.Fatalf("the two failures render differently, which reveals which addresses exist:\n%s\n---\n%s", bodyA, bodyB)
	}
}

func TestPeriodLockWritesAnAuditRow(t *testing.T) {
	h := newHarness(t)
	email := newEmail()
	h.user(t, email, "correct-horse", "admin")
	h.login(t, email, "correct-horse")
	client := h.newClient(t)
	h.pickClient(t, client)

	// The period has to exist before anything can be posted into it.
	var periodID string
	if err := h.storeOf(t).InClient(t.Context(), h.scopeOf(t, client), func(tx pgx.Tx) error {
		return tx.QueryRow(t.Context(), `
			INSERT INTO accounting_periods (client_id, name, start_date, end_date)
			VALUES ($1, 'Năm 2026', '2026-01-01', '2026-12-31')
			RETURNING id::text`, client).Scan(&periodID)
	}); err != nil {
		t.Fatalf("period: %v", err)
	}

	// A balanced voucher, then close the period.
	sale, err := h.client.PostForm(h.server.URL+"/entry", url.Values{
		"date":        {"2026-03-15"},
		"description": {"Bút toán"},
		"account":     {"1111", "4111"},
		"side":        {"DEBIT", "CREDIT"},
		"amount":      {"1.000.000", "1.000.000"},
	})
	if err != nil {
		t.Fatalf("entry: %v", err)
	}
	_ = sale.Body.Close()

	resp, err := h.client.PostForm(h.server.URL+"/close", url.Values{
		"period_id": {periodID},
	})
	if err != nil {
		t.Fatalf("close: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("close = %d\n%s", resp.StatusCode, readAll(t, resp))
	}

	var locks int
	if err := h.storeOf(t).InClient(t.Context(), h.scopeOf(t, client), func(tx pgx.Tx) error {
		return tx.QueryRow(t.Context(),
			`SELECT count(*) FROM audit_log WHERE action = 'PERIOD_LOCK' AND entity_id = $1`,
			periodID).Scan(&locks)
	}); err != nil {
		t.Fatalf("read audit: %v", err)
	}

	if locks != 1 {
		t.Fatalf("period lock audit rows = %d, want 1", locks)
	}
}

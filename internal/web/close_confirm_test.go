package web_test

import (
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
)

// TestClosePreviewPostsNothingAndConfirmLocks is C5: a close is a two-step act, and
// the preview must not write a voucher or lock the period.
func TestClosePreviewPostsNothingAndConfirmLocks(t *testing.T) {
	h := newHarness(t)
	email := newEmail()
	h.user(t, email, "correct-horse", "admin")
	h.login(t, email, "correct-horse")
	client := h.newClient(t)
	h.pickClient(t, client)

	h.postEntry(t, "2026-03-15", "Bút toán", "1111", "4111", "1.000.000")

	periodID := h.ensurePeriod(t, client)

	before := h.countVouchers(t, client)

	preview, err := h.client.PostForm(h.server.URL+"/close/preview", url.Values{"period_id": {periodID}})
	if err != nil {
		t.Fatalf("preview: %v", err)
	}
	defer func() { _ = preview.Body.Close() }()
	if preview.StatusCode != http.StatusOK {
		t.Fatalf("preview = %d\n%s", preview.StatusCode, readAll(t, preview))
	}

	body := readAll(t, preview)
	if !strings.Contains(body, "xem trước") {
		t.Fatalf("the preview page does not say it is a preview:\n%s", body)
	}
	if after := h.countVouchers(t, client); after != before {
		t.Fatalf("preview posted %d voucher(s)", after-before)
	}

	token := field(t, body, "token")
	if token == "" {
		t.Fatal("the preview did not issue a token")
	}

	confirm, err := h.client.PostForm(h.server.URL+"/close/confirm", url.Values{
		"token": {token}, "period_id": {periodID}, "confirm": {"01/2026"},
	})
	if err != nil {
		t.Fatalf("confirm: %v", err)
	}
	defer func() { _ = confirm.Body.Close() }()
	if confirm.StatusCode != http.StatusOK {
		t.Fatalf("confirm = %d\n%s", confirm.StatusCode, readAll(t, confirm))
	}

	var locked bool
	if err := h.storeOf(t).InClient(t.Context(), h.scopeOf(t, client), func(tx pgx.Tx) error {
		return tx.QueryRow(t.Context(),
			`SELECT EXISTS (SELECT 1 FROM accounting_period_locks WHERE period_id = $1)`, periodID).Scan(&locked)
	}); err != nil {
		t.Fatalf("read lock: %v", err)
	}
	if !locked {
		t.Fatal("confirm did not lock the period")
	}

	// The token is one-time: replaying the same confirmation is refused.
	replay, err := h.client.PostForm(h.server.URL+"/close/confirm", url.Values{
		"token": {token}, "period_id": {periodID}, "confirm": {"01/2026"},
	})
	if err != nil {
		t.Fatalf("replay: %v", err)
	}
	defer func() { _ = replay.Body.Close() }()
	if replay.StatusCode == http.StatusOK {
		t.Fatal("a used close token was accepted twice")
	}
}

// TestCloseConfirmRefusesAStaleToken covers the data-changed case.
func TestCloseConfirmRefusesAStaleToken(t *testing.T) {
	h := newHarness(t)
	email := newEmail()
	h.user(t, email, "correct-horse", "admin")
	h.login(t, email, "correct-horse")
	client := h.newClient(t)
	h.pickClient(t, client)

	h.postEntry(t, "2026-03-10", "Bút toán 1", "1111", "4111", "1.000.000")
	periodID := h.ensurePeriod(t, client)

	preview, err := h.client.PostForm(h.server.URL+"/close/preview", url.Values{"period_id": {periodID}})
	if err != nil {
		t.Fatalf("preview: %v", err)
	}
	body := readAll(t, preview)
	token := field(t, body, "token")

	// Something changes after the operator looked at the preview.
	h.postEntry(t, "2026-03-20", "Bút toán 2", "6422", "331", "500.000")

	confirm, err := h.client.PostForm(h.server.URL+"/close/confirm", url.Values{
		"token": {token}, "period_id": {periodID}, "confirm": {"01/2026"},
	})
	if err != nil {
		t.Fatalf("confirm: %v", err)
	}
	defer func() { _ = confirm.Body.Close() }()

	if confirm.StatusCode != http.StatusConflict {
		t.Fatalf("stale confirm = %d, want 409", confirm.StatusCode)
	}
}

// TestCloseConfirmRequiresTheTypedPeriod checks the deliberate act.
func TestCloseConfirmRequiresTheTypedPeriod(t *testing.T) {
	h := newHarness(t)
	email := newEmail()
	h.user(t, email, "correct-horse", "admin")
	h.login(t, email, "correct-horse")
	client := h.newClient(t)
	h.pickClient(t, client)

	h.postEntry(t, "2026-03-10", "Bút toán", "1111", "4111", "1.000.000")
	periodID := h.ensurePeriod(t, client)

	preview, err := h.client.PostForm(h.server.URL+"/close/preview", url.Values{"period_id": {periodID}})
	if err != nil {
		t.Fatalf("preview: %v", err)
	}
	token := field(t, readAll(t, preview), "token")

	confirm, err := h.client.PostForm(h.server.URL+"/close/confirm", url.Values{
		"token": {token}, "period_id": {periodID}, "confirm": {"2026-03"},
	})
	if err != nil {
		t.Fatalf("confirm: %v", err)
	}
	defer func() { _ = confirm.Body.Close() }()

	if confirm.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("wrong typed period = %d, want 422", confirm.StatusCode)
	}
}

func (h *harness) countVouchers(t *testing.T, clientID string) int {
	t.Helper()

	var n int
	if err := h.storeOf(t).InClient(t.Context(), h.scopeOf(t, clientID), func(tx pgx.Tx) error {
		return tx.QueryRow(t.Context(), `SELECT count(*) FROM vouchers`).Scan(&n)
	}); err != nil {
		t.Fatalf("count vouchers: %v", err)
	}
	return n
}

// ensurePeriod returns the client's first period, creating 2026 if none exists.
// /entry posts into a period but does not create one.
func (h *harness) ensurePeriod(t *testing.T, clientID string) string {
	t.Helper()

	var id string
	if err := h.storeOf(t).InClient(t.Context(), h.scopeOf(t, clientID), func(tx pgx.Tx) error {
		err := tx.QueryRow(t.Context(),
			`SELECT id::text FROM accounting_periods ORDER BY start_date LIMIT 1`).Scan(&id)
		if err == nil {
			return nil
		}
		return tx.QueryRow(t.Context(), `
			INSERT INTO accounting_periods (client_id, name, start_date, end_date)
			VALUES ($1, 'Năm 2026', '2026-01-01', '2026-12-31')
			RETURNING id::text`, clientID).Scan(&id)
	}); err != nil {
		t.Fatalf("ensure period: %v", err)
	}
	return id
}

// field extracts a hidden input value from a rendered page.
func field(t *testing.T, body, name string) string {
	t.Helper()

	marker := `name="` + name + `" value="`
	i := strings.Index(body, marker)
	if i < 0 {
		return ""
	}
	rest := body[i+len(marker):]
	j := strings.Index(rest, `"`)
	if j < 0 {
		return ""
	}
	return rest[:j]
}

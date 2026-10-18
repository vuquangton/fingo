package web_test

import (
	"crypto/rand"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"

	"fingo/internal/store"
	"fingo/internal/testdb"
	"fingo/internal/web"
)

// pickClient selects a client for the session, the way an operator does.
func (h *harness) pickClient(t *testing.T, id string) {
	t.Helper()

	resp, err := h.client.PostForm(h.server.URL+"/clients/"+id+"/select", url.Values{})
	if err != nil {
		t.Fatalf("select client: %v", err)
	}
	_ = resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("select client = %d, want 204", resp.StatusCode)
	}
}

func TestEntryScreenRendersTheChart(t *testing.T) {
	h := newHarness(t)
	email := newEmail()
	h.user(t, email, "correct-horse", web.RoleAccountant)
	client := h.newClient(t)

	h.login(t, email, "correct-horse")
	h.pickClient(t, client)

	resp := h.get(t, "/entry")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /entry = %d, want 200", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	for _, want := range []string{"Chứng từ", "5111", "Ghi sổ"} {
		if !strings.Contains(string(body), want) {
			t.Fatalf("the entry page does not contain %q", want)
		}
	}
}

func TestEntryScreenPostsAVoucher(t *testing.T) {
	h := newHarness(t)
	email := newEmail()
	h.user(t, email, "correct-horse", web.RoleAccountant)
	client := h.newClient(t)

	h.login(t, email, "correct-horse")
	h.pickClient(t, client)

	resp, err := h.client.PostForm(h.server.URL+"/entry", url.Values{
		"date":        {"2026-03-15"},
		"description": {"Bán hàng cho khách"},
		"account":     {"131", "5111", "33311"},
		"side":        {"DEBIT", "CREDIT", "CREDIT"},
		"amount":      {"1.100.000", "1.000.000", "100.000"},
	})
	if err != nil {
		t.Fatalf("post entry: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("POST /entry = %d, want 200 after the redirect\n%s", resp.StatusCode, body)
	}

	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "Đã ghi sổ") {
		t.Fatalf("the success notice is missing:\n%s", body)
	}

	st := h.storeOf(t)
	scope := h.scopeOf(t, client)

	var vouchers, lines int
	if err := st.InClient(t.Context(), scope, func(tx pgx.Tx) error {
		if err := tx.QueryRow(t.Context(), `SELECT count(*) FROM vouchers`).Scan(&vouchers); err != nil {
			return err
		}
		return tx.QueryRow(t.Context(), `SELECT count(*) FROM voucher_lines`).Scan(&lines)
	}); err != nil {
		t.Fatalf("count: %v", err)
	}

	if vouchers != 1 || lines != 3 {
		t.Fatalf("vouchers=%d lines=%d, want 1/3", vouchers, lines)
	}
}

func TestEntryScreenRefusesAnUnbalancedForm(t *testing.T) {
	h := newHarness(t)
	email := newEmail()
	h.user(t, email, "correct-horse", web.RoleAccountant)
	client := h.newClient(t)

	h.login(t, email, "correct-horse")
	h.pickClient(t, client)

	resp, err := h.client.PostForm(h.server.URL+"/entry", url.Values{
		"date":        {"2026-03-15"},
		"description": {"Sai số tiền"},
		"account":     {"131", "5111"},
		"side":        {"DEBIT", "CREDIT"},
		"amount":      {"1.100.000", "1.000.000"},
	})
	if err != nil {
		t.Fatalf("post entry: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("unbalanced form = %d, want 422", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(strings.ToLower(string(body)), "balance") {
		t.Fatalf("the refusal does not name the balance:\n%s", body)
	}
}

func (h *harness) login(t *testing.T, email, password string) {
	t.Helper()

	resp := h.post(t, "/login", url.Values{"email": {email}, "password": {password}})
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("login = %d, want 204", resp.StatusCode)
	}
}

// newClient registers a client and gives it the statutory chart.
func (h *harness) newClient(t *testing.T) string {
	t.Helper()

	id := newUUID(t)
	if _, err := h.pool.Exec(t.Context(),
		`INSERT INTO clients (id, name, tax_id) VALUES ($1, $2, $3)`,
		id, "Entry client", "ENT-"+id); err != nil {
		t.Fatalf("insert client: %v", err)
	}

	st := h.storeOf(t)
	scope := h.scopeOf(t, id)
	if err := st.InClient(t.Context(), scope, func(tx pgx.Tx) error {
		_, err := tx.Exec(t.Context(), `SELECT provision_client_chart($1)`, id)
		return err
	}); err != nil {
		t.Fatalf("provision chart: %v", err)
	}
	return id
}

func (h *harness) storeOf(t *testing.T) *store.Store {
	t.Helper()

	cfg := testdb.Load(t)
	st, err := store.Open(t.Context(), store.Config{DSN: cfg.DSN()})
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(st.Close)
	return st
}

func (h *harness) scopeOf(t *testing.T, id string) store.Scope {
	t.Helper()

	scope, err := store.NewScope(id)
	if err != nil {
		t.Fatalf("scope: %v", err)
	}
	return scope
}

func newUUID(t *testing.T) string {
	t.Helper()

	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		t.Fatalf("rand: %v", err)
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

package web_test

import (
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
)

// journalVoucherNo returns the number of the only posted voucher.
func (h *harness) journalVoucherNo(t *testing.T, clientID string) string {
	t.Helper()

	var no string
	if err := h.storeOf(t).InClient(t.Context(), h.scopeOf(t, clientID), func(tx pgx.Tx) error {
		return tx.QueryRow(t.Context(),
			`SELECT voucher_no FROM vouchers ORDER BY created_at LIMIT 1`).Scan(&no)
	}); err != nil {
		t.Fatalf("read the voucher: %v", err)
	}
	return no
}

func TestReversalMirrorsTheVoucher(t *testing.T) {
	h := newHarness(t)
	email := newEmail()
	h.user(t, email, "correct-horse", "accountant")
	h.login(t, email, "correct-horse")
	client := h.newClient(t)
	h.pickClient(t, client)

	// A voucher with two lines, posted through the entry screen.
	entry, err := h.client.PostForm(h.server.URL+"/entry", url.Values{
		"date":        {"2026-03-15"},
		"description": {"Bút toán nhập sai"},
		"account":     {"1111", "4111"},
		"side":        {"DEBIT", "CREDIT"},
		"amount":      {"2.000.000", "2.000.000"},
	})
	if err != nil {
		t.Fatalf("post entry: %v", err)
	}
	_ = entry.Body.Close()

	if entry.StatusCode != http.StatusOK {
		t.Fatalf("entry = %d", entry.StatusCode)
	}

	voucherNo := h.journalVoucherNo(t, client)

	resp, err := h.client.PostForm(h.server.URL+"/reverse", url.Values{
		"voucher_no": {voucherNo},
		"date":       {"2026-03-20"},
	})
	if err != nil {
		t.Fatalf("reverse: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("POST /reverse = %d\n%s", resp.StatusCode, readAll(t, resp))
	}

	body := readAll(t, resp)
	if !strings.Contains(body, "Đã ghi sổ bút toán đảo") {
		t.Fatalf("the success notice is missing:\n%s", body)
	}

	st := h.storeOf(t)
	scope := h.scopeOf(t, client)

	var vouchers int
	var net int64
	var reversedDescription string
	if err := st.InClient(t.Context(), scope, func(tx pgx.Tx) error {
		if err := tx.QueryRow(t.Context(), `SELECT count(*) FROM vouchers`).Scan(&vouchers); err != nil {
			return err
		}
		if err := tx.QueryRow(t.Context(), `
			SELECT COALESCE(SUM(CASE WHEN side = 'DEBIT' THEN amount ELSE -amount END), 0)
			  FROM voucher_lines`).Scan(&net); err != nil {
			return err
		}
		return tx.QueryRow(t.Context(), `
			SELECT description FROM vouchers WHERE description <> 'Bút toán nhập sai'`).Scan(&reversedDescription)
	}); err != nil {
		t.Fatalf("read the books: %v", err)
	}

	if vouchers != 2 {
		t.Fatalf("vouchers = %d, want 2 (original + mirror)", vouchers)
	}
	if net != 0 {
		t.Fatalf("the books net to %d after the reversal, want 0", net)
	}
	if !strings.HasPrefix(reversedDescription, "Đảo bút toán ") {
		t.Fatalf("the mirror is not labelled: %q", reversedDescription)
	}
}

func TestSecondReversalIsRefused(t *testing.T) {
	h := newHarness(t)
	email := newEmail()
	h.user(t, email, "correct-horse", "accountant")
	h.login(t, email, "correct-horse")
	client := h.newClient(t)
	h.pickClient(t, client)

	entry, err := h.client.PostForm(h.server.URL+"/entry", url.Values{
		"date":        {"2026-03-15"},
		"description": {"Bút toán"},
		"account":     {"1111", "4111"},
		"side":        {"DEBIT", "CREDIT"},
		"amount":      {"1.000.000", "1.000.000"},
	})
	if err != nil {
		t.Fatalf("post entry: %v", err)
	}
	_ = entry.Body.Close()

	voucherNo := h.journalVoucherNo(t, client)

	first, err := h.client.PostForm(h.server.URL+"/reverse", url.Values{
		"voucher_no": {voucherNo}, "date": {"2026-03-20"},
	})
	if err != nil {
		t.Fatalf("reverse: %v", err)
	}
	_ = first.Body.Close()

	second, err := h.client.PostForm(h.server.URL+"/reverse", url.Values{
		"voucher_no": {voucherNo}, "date": {"2026-03-21"},
	})
	if err != nil {
		t.Fatalf("reverse again: %v", err)
	}
	defer func() { _ = second.Body.Close() }()

	if second.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("second reversal = %d, want 422", second.StatusCode)
	}

	body := readAll(t, second)
	if !strings.Contains(strings.ToLower(body), "already") && !strings.Contains(body, "đã đảo") {
		t.Fatalf("the refusal does not explain itself:\n%s", body)
	}
}

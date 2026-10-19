package web_test

import (
	"net/http"
	"net/url"
	"testing"

	"github.com/jackc/pgx/v5"
)

// postCreditNote returns the response for a return of qty on line 1.
func (h *harness) postCreditNote(t *testing.T, invoiceID string, qty string) *http.Response {
	t.Helper()

	resp, err := h.client.PostForm(h.server.URL+"/credit-note", url.Values{
		"invoice_id": {invoiceID},
		"date":       {"2026-05-15"},
		"reason":     {"Hàng lỗi"},
		"qty_1":      {qty},
	})
	if err != nil {
		t.Fatalf("credit note: %v", err)
	}
	return resp
}

func (h *harness) invoiceDocID(t *testing.T, clientID string) string {
	t.Helper()

	var id string
	if err := h.storeOf(t).InClient(t.Context(), h.scopeOf(t, clientID), func(tx pgx.Tx) error {
		return tx.QueryRow(t.Context(), `
			SELECT id::text FROM documents WHERE kind = 'SALES_INVOICE'
			 ORDER BY created_at DESC LIMIT 1`).Scan(&id)
	}); err != nil {
		t.Fatalf("read invoice: %v", err)
	}
	return id
}

// TestCreditNoteReversesRevenueVatAndTheReceivable is C6's core posting.
func TestCreditNoteReversesRevenueVatAndTheReceivable(t *testing.T) {
	h := newHarness(t)
	email := newEmail()
	h.user(t, email, "correct-horse", "accountant")
	h.login(t, email, "correct-horse")
	client := h.newClient(t)
	h.pickClient(t, client)

	h.sellOne(t, client, "Công ty Trả Lại", "500.000") // 2 × 500.000 + 10% = 1.100.000
	invoiceID := h.invoiceDocID(t, client)

	resp := h.postCreditNote(t, invoiceID, "2")
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("credit note = %d\n%s", resp.StatusCode, readAll(t, resp))
	}

	var reduction, vat, receivable int64
	var status string
	var allocated int64
	if err := h.storeOf(t).InClient(t.Context(), h.scopeOf(t, client), func(tx pgx.Tx) error {
		if err := tx.QueryRow(t.Context(), `
			SELECT COALESCE(SUM(CASE WHEN account_code = '5213'  AND side = 'DEBIT'  THEN amount END), 0),
			       COALESCE(SUM(CASE WHEN account_code = '33311' AND side = 'DEBIT'  THEN amount END), 0),
			       COALESCE(SUM(CASE WHEN account_code = '131'   AND side = 'CREDIT' THEN amount END), 0)
			  FROM voucher_lines`).Scan(&reduction, &vat, &receivable); err != nil {
			return err
		}
		return tx.QueryRow(t.Context(),
			`SELECT status, allocated FROM documents WHERE id = $1`, invoiceID).Scan(&status, &allocated)
	}); err != nil {
		t.Fatalf("read: %v", err)
	}

	if reduction != 1_000_000 {
		t.Fatalf("revenue deduction 5213 = %d, want 1,000,000", reduction)
	}
	// The return reverses the output VAT of what came back: 100.000.
	if vat != 100_000 {
		t.Fatalf("33311 debit = %d, want 100,000 (VAT on the returned goods)", vat)
	}
	// The sale's 131 debit (1.100.000) plus the return's 131 credit (1.100.000).
	if receivable != 1_100_000 {
		t.Fatalf("131 credit = %d, want 1,100,000", receivable)
	}
	if status != "SETTLED" || allocated != 1_100_000 {
		t.Fatalf("invoice = %s / %d, want SETTLED / 1,100,000", status, allocated)
	}
}

// TestCreditNoteRefusesReturningMoreThanWasSold is the over-return guard.
func TestCreditNoteRefusesReturningMoreThanWasSold(t *testing.T) {
	h := newHarness(t)
	email := newEmail()
	h.user(t, email, "correct-horse", "accountant")
	h.login(t, email, "correct-horse")
	client := h.newClient(t)
	h.pickClient(t, client)

	h.sellOne(t, client, "Khách trả quá", "500.000")
	invoiceID := h.invoiceDocID(t, client)

	resp := h.postCreditNote(t, invoiceID, "3")
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("over-return = %d, want 422", resp.StatusCode)
	}
}

// TestCreditNotePartialLeavesTheInvoicePartiallyCredited checks partial quantities.
func TestCreditNotePartialLeavesTheInvoicePartiallyCredited(t *testing.T) {
	h := newHarness(t)
	email := newEmail()
	h.user(t, email, "correct-horse", "accountant")
	h.login(t, email, "correct-horse")
	client := h.newClient(t)
	h.pickClient(t, client)

	h.sellOne(t, client, "Khách trả một nửa", "500.000")
	invoiceID := h.invoiceDocID(t, client)

	resp := h.postCreditNote(t, invoiceID, "1")
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("partial credit note = %d\n%s", resp.StatusCode, readAll(t, resp))
	}

	var allocated int64
	var status string
	if err := h.storeOf(t).InClient(t.Context(), h.scopeOf(t, client), func(tx pgx.Tx) error {
		return tx.QueryRow(t.Context(),
			`SELECT allocated, status FROM documents WHERE id = $1`, invoiceID).Scan(&allocated, &status)
	}); err != nil {
		t.Fatalf("read: %v", err)
	}

	// One of two units returned: 550.000 credited, 550.000 still owed.
	if allocated != 550_000 || status != "PARTIAL" {
		t.Fatalf("after partial return: %d / %s, want 550,000 / PARTIAL", allocated, status)
	}
}

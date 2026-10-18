package web_test

import (
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
)

// postBill records a 1.000.000 supplier bill at 10% and returns its document id.
func (h *harness) postBill(t *testing.T, clientID, supplierID string) string {
	t.Helper()

	resp, err := h.client.PostForm(h.server.URL+"/purchase", url.Values{
		"partner_id":      {supplierID},
		"date":            {"2026-03-10"},
		"description":     {"Phí điện tháng 3"},
		"expense_account": {"6422"},
		"net":             {"1.000.000"},
	})
	if err != nil {
		t.Fatalf("post bill: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("POST /purchase = %d\n%s", resp.StatusCode, readAll(t, resp))
	}

	var id string
	if err := h.storeOf(t).InClient(t.Context(), h.scopeOf(t, clientID), func(tx pgx.Tx) error {
		return tx.QueryRow(t.Context(),
			`SELECT id::text FROM documents WHERE kind = 'PURCHASE_BILL' ORDER BY created_at DESC LIMIT 1`).Scan(&id)
	}); err != nil {
		t.Fatalf("read the bill: %v", err)
	}
	return id
}

func TestPaymentSettlesTheSupplierBill(t *testing.T) {
	h := newHarness(t)
	email := newEmail()
	h.user(t, email, "correct-horse", "accountant")
	h.login(t, email, "correct-horse")
	client := h.newClient(t)
	h.pickClient(t, client)

	supplierID := h.addSupplier(t, client, "Điện lực Sài Gòn", "0301234567")
	billID := h.postBill(t, client, supplierID)

	body := h.body(t, "/payment")
	if !strings.Contains(body, "Điện lực Sài Gòn") || !strings.Contains(body, "1.100.000") {
		t.Fatalf("the payment screen does not show the open bill:\n%s", body)
	}

	resp, err := h.client.PostForm(h.server.URL+"/payment", url.Values{
		"document_id": {billID},
		"amount":      {"1.100.000"},
		"date":        {"2026-03-25"},
	})
	if err != nil {
		t.Fatalf("pay: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("POST /payment = %d\n%s", resp.StatusCode, readAll(t, resp))
	}

	st := h.storeOf(t)
	scope := h.scopeOf(t, client)

	var status string
	var allocated, payableDebit, cashCredit int64
	if err := st.InClient(t.Context(), scope, func(tx pgx.Tx) error {
		if err := tx.QueryRow(t.Context(),
			`SELECT status, allocated FROM documents WHERE id = $1`, billID).Scan(&status, &allocated); err != nil {
			return err
		}
		return tx.QueryRow(t.Context(), `
			SELECT COALESCE(SUM(CASE WHEN account_code = '331'  AND side = 'DEBIT'  THEN amount END), 0),
			       COALESCE(SUM(CASE WHEN account_code = '1111' AND side = 'CREDIT' THEN amount END), 0)
			  FROM voucher_lines`).Scan(&payableDebit, &cashCredit)
	}); err != nil {
		t.Fatalf("read the books: %v", err)
	}

	if status != "SETTLED" || allocated != 1_100_000 {
		t.Fatalf("bill = %s / %d, want SETTLED / 1,100,000", status, allocated)
	}
	if payableDebit != 1_100_000 || cashCredit != 1_100_000 {
		t.Fatalf("331 debit %d / 1111 credit %d, want 1,100,000 each", payableDebit, cashCredit)
	}
}

func TestPaymentRefusesASettledBill(t *testing.T) {
	h := newHarness(t)
	email := newEmail()
	h.user(t, email, "correct-horse", "accountant")
	h.login(t, email, "correct-horse")
	client := h.newClient(t)
	h.pickClient(t, client)

	supplierID := h.addSupplier(t, client, "Nhà cung cấp A", "")
	billID := h.postBill(t, client, supplierID)

	first, err := h.client.PostForm(h.server.URL+"/payment", url.Values{
		"document_id": {billID}, "amount": {"1.100.000"}, "date": {"2026-03-25"},
	})
	if err != nil {
		t.Fatalf("pay: %v", err)
	}
	_ = first.Body.Close()

	second, err := h.client.PostForm(h.server.URL+"/payment", url.Values{
		"document_id": {billID}, "amount": {"1.100.000"}, "date": {"2026-03-26"},
	})
	if err != nil {
		t.Fatalf("pay again: %v", err)
	}
	defer func() { _ = second.Body.Close() }()

	if second.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("paying a settled bill = %d, want 422", second.StatusCode)
	}
}

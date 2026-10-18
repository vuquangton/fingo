package web_test

import (
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
)

// sellOne posts one 1.100.000 invoice and returns its document id.
func (h *harness) sellOne(t *testing.T, clientID, partnerName string, unitPrice string) string {
	t.Helper()

	h.addPartner(t, partnerName, "")
	partnerID := h.partnerID(t, clientID, partnerName)

	resp, err := h.client.PostForm(h.server.URL+"/sales", url.Values{
		"partner_id":  {partnerID},
		"date":        {"2026-03-15"},
		"description": {"Bán hàng"},
		"item":        {"HH-01", "", ""},
		"quantity":    {"2", "1", "1"},
		"unit_price":  {unitPrice, "", ""},
	})
	if err != nil {
		t.Fatalf("post sale: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("POST /sales = %d\n%s", resp.StatusCode, readAll(t, resp))
	}

	var id string
	if err := h.storeOf(t).InClient(t.Context(), h.scopeOf(t, clientID), func(tx pgx.Tx) error {
		return tx.QueryRow(t.Context(),
			`SELECT id::text FROM documents WHERE status = 'OPEN' ORDER BY created_at DESC LIMIT 1`).Scan(&id)
	}); err != nil {
		t.Fatalf("read the invoice: %v", err)
	}
	return id
}

func TestReceiptSettlesTheInvoice(t *testing.T) {
	h := newHarness(t)
	email := newEmail()
	h.user(t, email, "correct-horse", "accountant")
	h.login(t, email, "correct-horse")
	client := h.newClient(t)
	h.pickClient(t, client)

	invoiceID := h.sellOne(t, client, "Công ty Đông Á", "500.000")

	body := h.body(t, "/receipt")
	if !strings.Contains(body, "Công ty Đông Á") || !strings.Contains(body, "1.100.000") {
		t.Fatalf("the receipt screen does not show the open invoice:\n%s", body)
	}

	resp, err := h.client.PostForm(h.server.URL+"/receipt", url.Values{
		"document_id": {invoiceID},
		"amount":      {"1.100.000"},
		"date":        {"2026-03-20"},
	})
	if err != nil {
		t.Fatalf("post receipt: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("POST /receipt = %d\n%s", resp.StatusCode, readAll(t, resp))
	}

	st := h.storeOf(t)
	scope := h.scopeOf(t, client)

	var status string
	var allocated int64
	var allocations int
	var cash int64
	if err := st.InClient(t.Context(), scope, func(tx pgx.Tx) error {
		if err := tx.QueryRow(t.Context(),
			`SELECT status, allocated FROM documents WHERE id = $1`, invoiceID).Scan(&status, &allocated); err != nil {
			return err
		}
		if err := tx.QueryRow(t.Context(),
			`SELECT count(*) FROM allocations WHERE document_id = $1`, invoiceID).Scan(&allocations); err != nil {
			return err
		}
		return tx.QueryRow(t.Context(),
			`SELECT COALESCE(SUM(CASE WHEN account_code = '1111' AND side = 'DEBIT' THEN amount END), 0)
			   FROM voucher_lines`).Scan(&cash)
	}); err != nil {
		t.Fatalf("read the books: %v", err)
	}

	if status != "SETTLED" || allocated != 1_100_000 {
		t.Fatalf("document = %s / %d, want SETTLED / 1,100,000", status, allocated)
	}
	if allocations != 1 {
		t.Fatalf("allocations = %d, want 1", allocations)
	}
	if cash != 1_100_000 {
		t.Fatalf("cash debit = %d, want 1,100,000", cash)
	}
}

func TestOverpaymentBecomesACustomerCredit(t *testing.T) {
	h := newHarness(t)
	email := newEmail()
	h.user(t, email, "correct-horse", "accountant")
	h.login(t, email, "correct-horse")
	client := h.newClient(t)
	h.pickClient(t, client)

	invoiceID := h.sellOne(t, client, "Khách sộp", "500.000")

	resp, err := h.client.PostForm(h.server.URL+"/receipt", url.Values{
		"document_id": {invoiceID},
		"amount":      {"2.000.000"},
		"date":        {"2026-03-20"},
	})
	if err != nil {
		t.Fatalf("post receipt: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("overpayment was refused: %d\n%s", resp.StatusCode, readAll(t, resp))
	}

	var credits int
	var creditAmount int64
	if err := h.storeOf(t).InClient(t.Context(), h.scopeOf(t, client), func(tx pgx.Tx) error {
		if err := tx.QueryRow(t.Context(), `SELECT count(*) FROM document_credits`).Scan(&credits); err != nil {
			return err
		}
		return tx.QueryRow(t.Context(),
			`SELECT COALESCE(MAX(amount), 0) FROM document_credits`).Scan(&creditAmount)
	}); err != nil {
		t.Fatalf("read credits: %v", err)
	}

	if credits != 1 {
		t.Fatalf("credits = %d, want 1", credits)
	}
	if creditAmount != 900_000 {
		t.Fatalf("credit = %d, want 900,000", creditAmount)
	}
}

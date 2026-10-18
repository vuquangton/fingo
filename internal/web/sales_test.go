package web_test

import (
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
)

// addPartner creates a customer through the screen, the way an operator does.
func (h *harness) addPartner(t *testing.T, name, taxID string) {
	t.Helper()

	resp, err := h.client.PostForm(h.server.URL+"/partners", url.Values{
		"name": {name}, "tax_id": {taxID}, "kind": {"CUSTOMER"},
	})
	if err != nil {
		t.Fatalf("add partner: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("add partner = %d", resp.StatusCode)
	}
}

func (h *harness) partnerID(t *testing.T, clientID, name string) string {
	t.Helper()

	var id string
	if err := h.storeOf(t).InClient(t.Context(), h.scopeOf(t, clientID), func(tx pgx.Tx) error {
		return tx.QueryRow(t.Context(), `SELECT id::text FROM partners WHERE name = $1`, name).Scan(&id)
	}); err != nil {
		t.Fatalf("read partner: %v", err)
	}
	return id
}

func TestSalesScreenNeedsAPartnerFirst(t *testing.T) {
	h := newHarness(t)
	email := newEmail()
	h.user(t, email, "correct-horse", "accountant")
	h.login(t, email, "correct-horse")
	h.pickClient(t, h.newClient(t))

	body := h.body(t, "/sales")

	if !strings.Contains(body, "Chưa có khách hàng") {
		t.Fatalf("the sales screen does not send the operator to the partner list first:\n%s", body)
	}
}

func TestSalesScreenPostsAnInvoice(t *testing.T) {
	h := newHarness(t)
	email := newEmail()
	h.user(t, email, "correct-horse", "accountant")
	h.login(t, email, "correct-horse")
	client := h.newClient(t)
	h.pickClient(t, client)

	h.addPartner(t, "Công ty Đông Á", "0101234567")
	partnerID := h.partnerID(t, client, "Công ty Đông Á")

	resp, err := h.client.PostForm(h.server.URL+"/sales", url.Values{
		"partner_id":  {partnerID},
		"date":        {"2026-03-15"},
		"description": {"Bán hàng cho Đông Á"},
		"item":        {"HH-01", "", ""},
		"quantity":    {"2", "1", "1"},
		"unit_price":  {"500.000", "", ""},
	})
	if err != nil {
		t.Fatalf("post sale: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("POST /sales = %d, want 200 after the redirect\n%s", resp.StatusCode, readAll(t, resp))
	}

	body := readAll(t, resp)
	if !strings.Contains(body, "Đã ghi sổ hóa đơn") {
		t.Fatalf("the success notice is missing:\n%s", body)
	}

	// The ledger holds the three postings, and the receivable is open.
	st := h.storeOf(t)
	scope := h.scopeOf(t, client)

	var ar, revenue, vat int64
	var openDocuments int
	if err := st.InClient(t.Context(), scope, func(tx pgx.Tx) error {
		if err := tx.QueryRow(t.Context(), `
			SELECT
			  COALESCE(SUM(CASE WHEN account_code = '131'   AND side = 'DEBIT'  THEN amount END), 0),
			  COALESCE(SUM(CASE WHEN account_code = '5111'  AND side = 'CREDIT' THEN amount END), 0),
			  COALESCE(SUM(CASE WHEN account_code = '33311' AND side = 'CREDIT' THEN amount END), 0)
			  FROM voucher_lines`).Scan(&ar, &revenue, &vat); err != nil {
			return err
		}
		return tx.QueryRow(t.Context(),
			`SELECT count(*) FROM documents WHERE status = 'OPEN'`).Scan(&openDocuments)
	}); err != nil {
		t.Fatalf("read the books: %v", err)
	}

	if ar != 1_100_000 || revenue != 1_000_000 || vat != 100_000 {
		t.Fatalf("postings: 131=%d 5111=%d 33311=%d, want 1100000/1000000/100000",
			ar, revenue, vat)
	}
	if openDocuments != 1 {
		t.Fatalf("open receivables = %d, want 1", openDocuments)
	}
}

func TestSalesScreenRefusesAnEmptyInvoice(t *testing.T) {
	h := newHarness(t)
	email := newEmail()
	h.user(t, email, "correct-horse", "accountant")
	h.login(t, email, "correct-horse")
	client := h.newClient(t)
	h.pickClient(t, client)

	h.addPartner(t, "Khách A", "")
	partnerID := h.partnerID(t, client, "Khách A")

	resp, err := h.client.PostForm(h.server.URL+"/sales", url.Values{
		"partner_id": {partnerID},
		"date":       {"2026-03-15"},
		"item":       {"", ""},
		"quantity":   {"1", "1"},
		"unit_price": {"", ""},
	})
	if err != nil {
		t.Fatalf("post sale: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("empty invoice = %d, want 422", resp.StatusCode)
	}
}

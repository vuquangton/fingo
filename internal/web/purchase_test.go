package web_test

import (
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
)

// addSupplier creates a supplier through the partner screen.
func (h *harness) addSupplier(t *testing.T, clientID, name, taxID string) string {
	t.Helper()

	resp, err := h.client.PostForm(h.server.URL+"/partners", url.Values{
		"name": {name}, "tax_id": {taxID}, "kind": {"SUPPLIER"},
	})
	if err != nil {
		t.Fatalf("add supplier: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("add supplier = %d", resp.StatusCode)
	}

	var id string
	if err := h.storeOf(t).InClient(t.Context(), h.scopeOf(t, clientID), func(tx pgx.Tx) error {
		return tx.QueryRow(t.Context(), `SELECT id::text FROM partners WHERE name = $1`, name).Scan(&id)
	}); err != nil {
		t.Fatalf("read supplier: %v", err)
	}
	return id
}

func TestPurchaseBillPostsCostInputVATAndPayable(t *testing.T) {
	h := newHarness(t)
	email := newEmail()
	h.user(t, email, "correct-horse", "accountant")
	h.login(t, email, "correct-horse")
	client := h.newClient(t)
	h.pickClient(t, client)

	supplierID := h.addSupplier(t, client, "Điện lực Sài Gòn", "0301234567")

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

	body := readAll(t, resp)
	if !strings.Contains(body, "Đã ghi sổ hóa đơn mua vào") {
		t.Fatalf("the success notice is missing:\n%s", body)
	}

	st := h.storeOf(t)
	scope := h.scopeOf(t, client)

	var expense, inputVAT, payable int64
	var billKind string
	var billTotal int64
	if err := st.InClient(t.Context(), scope, func(tx pgx.Tx) error {
		if err := tx.QueryRow(t.Context(), `
			SELECT
			  COALESCE(SUM(CASE WHEN account_code = '6422' AND side = 'DEBIT'  THEN amount END), 0),
			  COALESCE(SUM(CASE WHEN account_code = '1331' AND side = 'DEBIT'  THEN amount END), 0),
			  COALESCE(SUM(CASE WHEN account_code = '331'  AND side = 'CREDIT' THEN amount END), 0)
			  FROM voucher_lines`).Scan(&expense, &inputVAT, &payable); err != nil {
			return err
		}
		return tx.QueryRow(t.Context(),
			`SELECT kind, total FROM documents WHERE status = 'OPEN' ORDER BY created_at DESC LIMIT 1`).
			Scan(&billKind, &billTotal)
	}); err != nil {
		t.Fatalf("read the books: %v", err)
	}

	if expense != 1_000_000 {
		t.Fatalf("TK 6422 debit = %d, want 1,000,000", expense)
	}
	if inputVAT != 100_000 {
		t.Fatalf("TK 1331 debit = %d, want 100,000", inputVAT)
	}
	if payable != 1_100_000 {
		t.Fatalf("TK 331 credit = %d, want 1,100,000", payable)
	}
	if billKind != "PURCHASE_BILL" || billTotal != 1_100_000 {
		t.Fatalf("payable document = %s / %d, want PURCHASE_BILL / 1,100,000", billKind, billTotal)
	}
}

func TestPurchaseBillNeedsASupplier(t *testing.T) {
	h := newHarness(t)
	email := newEmail()
	h.user(t, email, "correct-horse", "accountant")
	h.login(t, email, "correct-horse")
	h.pickClient(t, h.newClient(t))

	resp, err := h.client.PostForm(h.server.URL+"/purchase", url.Values{
		"partner_id":      {""},
		"date":            {"2026-03-10"},
		"expense_account": {"6422"},
		"net":             {"1.000.000"},
	})
	if err != nil {
		t.Fatalf("post bill: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("bill with no supplier = %d, want 422", resp.StatusCode)
	}
}

package web_test

import (
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
)

// postEntry posts a simple two-line voucher through the screen.
func (h *harness) postEntry(t *testing.T, date, description, debit, credit string, amount string) {
	t.Helper()

	resp, err := h.client.PostForm(h.server.URL+"/entry", url.Values{
		"date": {date}, "description": {description},
		"account": {debit, credit}, "side": {"DEBIT", "CREDIT"},
		"amount": {amount, amount},
	})
	if err != nil {
		t.Fatalf("post entry: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("entry = %d\n%s", resp.StatusCode, readAll(t, resp))
	}
}

// TestOpeningStockSeedsTheWarehouse is C2: a trader's 156 opening balance must
// become quantities at a cost, or the first sale has no stock and no COGS.
func TestOpeningStockSeedsTheWarehouse(t *testing.T) {
	h := newHarness(t)
	email := newEmail()
	h.user(t, email, "correct-horse", "accountant")
	h.login(t, email, "correct-horse")
	client := h.newClient(t)
	h.pickClient(t, client)

	// The opening 156 balance the stock import must reconcile to.
	h.postEntry(t, "2026-05-01", "Số dư đầu kỳ 156", "156", "4111", "120.000.000")

	resp, err := h.client.PostForm(h.server.URL+"/opening/stock", url.Values{
		"date":      {"2026-05-01"},
		"sku":       {"HH-A"},
		"name":      {"Hàng A"},
		"unit":      {"cái"},
		"qty":       {"300"},
		"unit_cost": {"400.000"},
	})
	if err != nil {
		t.Fatalf("opening stock: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("POST /opening/stock = %d\n%s", resp.StatusCode, readAll(t, resp))
	}

	// Stock exists at the opening cost.
	stock := h.body(t, "/stock")
	for _, want := range []string{"HH-A", "300", "120.000.000"} {
		if !strings.Contains(stock, want) {
			t.Fatalf("stock list missing %q:\n%s", want, stock)
		}
	}

	// A sale of 100 costs 40.000.000 at the opening average.
	h.addPartner(t, "Minh Long", "")
	partnerID := h.partnerID(t, client, "Minh Long")

	sale, err := h.client.PostForm(h.server.URL+"/sales", url.Values{
		"partner_id": {partnerID}, "date": {"2026-05-10"}, "description": {"Bán 100"},
		"item": {"HH-A", "", ""}, "quantity": {"100", "1", "1"},
		"unit_price": {"700.000", "", ""}, "vat_rate": {"vat10", "vat10", "vat10"},
	})
	if err != nil {
		t.Fatalf("sale: %v", err)
	}
	defer func() { _ = sale.Body.Close() }()
	if sale.StatusCode != http.StatusOK {
		t.Fatalf("sale = %d\n%s", sale.StatusCode, readAll(t, sale))
	}

	var cogs int64
	if err := h.storeOf(t).InClient(t.Context(), h.scopeOf(t, client), func(tx pgx.Tx) error {
		return tx.QueryRow(t.Context(), `
			SELECT COALESCE(SUM(CASE WHEN account_code = '632' AND side = 'DEBIT' THEN amount END), 0)
			  FROM voucher_lines`).Scan(&cogs)
	}); err != nil {
		t.Fatalf("read COGS: %v", err)
	}
	if cogs != 40_000_000 {
		t.Fatalf("COGS = %d, want 40,000,000 (100 × 400,000)", cogs)
	}

	// Re-import after activity is refused.
	again, err := h.client.PostForm(h.server.URL+"/opening/stock", url.Values{
		"date": {"2026-05-01"}, "sku": {"HH-A"}, "name": {"Hàng A"}, "unit": {"cái"},
		"qty": {"300"}, "unit_cost": {"400.000"},
	})
	if err != nil {
		t.Fatalf("re-import: %v", err)
	}
	defer func() { _ = again.Body.Close() }()
	if again.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("re-import = %d, want 422", again.StatusCode)
	}
}

// TestOpeningStockMustEqualTheOpening156 reconciles the import to the account; a
// mismatch is refused with the difference stated, not silently accepted.
func TestOpeningStockMustEqualTheOpening156(t *testing.T) {
	h := newHarness(t)
	email := newEmail()
	h.user(t, email, "correct-horse", "accountant")
	h.login(t, email, "correct-horse")
	client := h.newClient(t)
	h.pickClient(t, client)

	h.postEntry(t, "2026-05-01", "Số dư đầu kỳ 156", "156", "4111", "100.000.000")

	resp, err := h.client.PostForm(h.server.URL+"/opening/stock", url.Values{
		"date": {"2026-05-01"}, "sku": {"HH-A"}, "name": {"Hàng A"}, "unit": {"cái"},
		"qty": {"300"}, "unit_cost": {"400.000"},
	})
	if err != nil {
		t.Fatalf("opening stock: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("mismatched import = %d, want 422\n%s", resp.StatusCode, readAll(t, resp))
	}
}

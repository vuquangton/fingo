package web_test

import (
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
)

// receiveStock books qty units of an item at unitCost.
func (h *harness) receiveStock(t *testing.T, clientID, itemID, qty, unitCost, date, source string) {
	t.Helper()

	supplierID := newUUID(t)
	if err := h.storeOf(t).InClient(t.Context(), h.scopeOf(t, clientID), func(tx pgx.Tx) error {
		_, err := tx.Exec(t.Context(),
			`INSERT INTO partners (id, client_id, kind, name, tax_id) VALUES ($1, $2, 'SUPPLIER', $3, $4)`,
			supplierID, clientID, "NCC "+supplierID[:8], "NCC-"+supplierID[:8])
		return err
	}); err != nil {
		t.Fatalf("insert supplier: %v", err)
	}

	resp, err := h.client.PostForm(h.server.URL+"/stock/receive", url.Values{
		"item_id": {itemID}, "partner_id": {supplierID}, "qty": {qty}, "unit_cost": {unitCost},
		"date": {date}, "source_doc": {source},
	})
	if err != nil {
		t.Fatalf("receive stock: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("receive stock = %d\n%s", resp.StatusCode, readAll(t, resp))
	}
}

// sellItem posts a sale of qty units of a stock item at unitPrice.
func (h *harness) sellItem(t *testing.T, partnerID, sku string, qty int, unitPrice string) *http.Response {
	t.Helper()

	resp, err := h.client.PostForm(h.server.URL+"/sales", url.Values{
		"partner_id":  {partnerID},
		"date":        {"2026-03-15"},
		"description": {"Bán hàng cho khách"},
		"item":        {sku, "", ""},
		"quantity":    {itoa(qty), "1", "1"},
		"unit_price":  {unitPrice, "", ""},
	})
	if err != nil {
		t.Fatalf("post sale: %v", err)
	}
	return resp
}

func TestSalePostsTheCostOfWhatItSold(t *testing.T) {
	h := newHarness(t)
	email := newEmail()
	h.user(t, email, "correct-horse", "accountant")
	h.login(t, email, "correct-horse")
	client := h.newClient(t)
	h.pickClient(t, client)

	itemID := h.addItem(t, client, "HH-01", "Hàng hóa 01")
	h.receiveStock(t, client, itemID, "10", "100.000", "2026-03-05", "HĐ 0001234")

	h.addPartner(t, "Công ty Đông Á", "")
	partnerID := h.partnerID(t, client, "Công ty Đông Á")

	resp := h.sellItem(t, partnerID, "HH-01", 2, "500.000")
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("POST /sales = %d\n%s", resp.StatusCode, readAll(t, resp))
	}

	st := h.storeOf(t)
	scope := h.scopeOf(t, client)

	var revenue, receivable, cogs, inventoryCredit int64
	var onHand int
	if err := st.InClient(t.Context(), scope, func(tx pgx.Tx) error {
		if err := tx.QueryRow(t.Context(), `
			SELECT
			  COALESCE(SUM(CASE WHEN account_code = '5111' AND side = 'CREDIT' THEN amount END), 0),
			  COALESCE(SUM(CASE WHEN account_code = '131'  AND side = 'DEBIT'  THEN amount END), 0),
			  COALESCE(SUM(CASE WHEN account_code = '632'  AND side = 'DEBIT'  THEN amount END), 0),
			  COALESCE(SUM(CASE WHEN account_code = '156'  AND side = 'CREDIT' THEN amount END), 0)
			  FROM voucher_lines`).Scan(&revenue, &receivable, &cogs, &inventoryCredit); err != nil {
			return err
		}
		return tx.QueryRow(t.Context(),
			`SELECT COALESCE(SUM(CASE WHEN kind = 'RECEIPT' THEN quantity ELSE -quantity END), 0)::int
			   FROM stock_movements`).Scan(&onHand)
	}); err != nil {
		t.Fatalf("read the books: %v", err)
	}

	if revenue != 1_000_000 || receivable != 1_100_000 {
		t.Fatalf("revenue %d / receivable %d, want 1,000,000 / 1,100,000", revenue, receivable)
	}
	if cogs != 200_000 || inventoryCredit != 200_000 {
		t.Fatalf("COGS %d / inventory credit %d, want 200,000 each (2 × 100,000)", cogs, inventoryCredit)
	}
	if onHand != 8 {
		t.Fatalf("stock on hand = %d, want 8", onHand)
	}
}

func TestSaleBeyondStockIsRefusedBeforeItPosts(t *testing.T) {
	h := newHarness(t)
	email := newEmail()
	h.user(t, email, "correct-horse", "accountant")
	h.login(t, email, "correct-horse")
	client := h.newClient(t)
	h.pickClient(t, client)

	itemID := h.addItem(t, client, "HH-02", "Hàng hóa 02")
	h.receiveStock(t, client, itemID, "3", "100.000", "2026-03-05", "HĐ 0001")

	h.addPartner(t, "Khách B", "")
	partnerID := h.partnerID(t, client, "Khách B")

	resp := h.sellItem(t, partnerID, "HH-02", 5, "200.000")
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("selling beyond stock = %d, want 422", resp.StatusCode)
	}

	body := readAll(t, resp)
	if !strings.Contains(strings.ToLower(body), "stock") && !strings.Contains(body, "tồn") {
		t.Fatalf("the refusal does not name the stock shortage:\n%s", body)
	}

	var vouchers int
	if err := h.storeOf(t).InClient(t.Context(), h.scopeOf(t, client), func(tx pgx.Tx) error {
		return tx.QueryRow(t.Context(),
			`SELECT count(*) FROM vouchers v JOIN voucher_lines l ON l.voucher_id = v.id
			  WHERE l.account_code = '5111'`).Scan(&vouchers)
	}); err != nil {
		t.Fatalf("count revenue lines: %v", err)
	}
	if vouchers != 0 {
		t.Fatalf("a refused sale posted %d revenue lines", vouchers)
	}
}

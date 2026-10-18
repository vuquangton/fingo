package web_test

import (
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
)

// addItem stocks an item through the screen and returns its id.
func (h *harness) addItem(t *testing.T, clientID, sku, name string) string {
	t.Helper()

	resp, err := h.client.PostForm(h.server.URL+"/stock/item", url.Values{
		"sku": {sku}, "name": {name}, "unit": {"cái"}, "reorder_point": {"5"},
	})
	if err != nil {
		t.Fatalf("add item: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("add item = %d\n%s", resp.StatusCode, readAll(t, resp))
	}

	var id string
	if err := h.storeOf(t).InClient(t.Context(), h.scopeOf(t, clientID), func(tx pgx.Tx) error {
		return tx.QueryRow(t.Context(), `SELECT id::text FROM items WHERE sku = $1`, sku).Scan(&id)
	}); err != nil {
		t.Fatalf("read item: %v", err)
	}
	return id
}

func TestStockReceiptValuesTheItemAndPostsInventory(t *testing.T) {
	h := newHarness(t)
	email := newEmail()
	h.user(t, email, "correct-horse", "accountant")
	h.login(t, email, "correct-horse")
	client := h.newClient(t)
	h.pickClient(t, client)

	itemID := h.addItem(t, client, "HH-01", "Hàng hóa 01")

	resp, err := h.client.PostForm(h.server.URL+"/stock/receive", url.Values{
		"item_id":    {itemID},
		"qty":        {"10"},
		"unit_cost":  {"100.000"},
		"date":       {"2026-03-05"},
		"source_doc": {"HĐ 0001234"},
	})
	if err != nil {
		t.Fatalf("receive: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("POST /stock/receive = %d\n%s", resp.StatusCode, readAll(t, resp))
	}

	st := h.storeOf(t)
	scope := h.scopeOf(t, client)

	var inventoryDebit, payableCredit int64
	var movements int
	if err := st.InClient(t.Context(), scope, func(tx pgx.Tx) error {
		if err := tx.QueryRow(t.Context(), `
			SELECT COALESCE(SUM(CASE WHEN account_code = '156' AND side = 'DEBIT'  THEN amount END), 0),
			       COALESCE(SUM(CASE WHEN account_code = '331' AND side = 'CREDIT' THEN amount END), 0)
			  FROM voucher_lines`).Scan(&inventoryDebit, &payableCredit); err != nil {
			return err
		}
		return tx.QueryRow(t.Context(), `SELECT count(*) FROM stock_movements`).Scan(&movements)
	}); err != nil {
		t.Fatalf("read the books: %v", err)
	}

	if inventoryDebit != 1_000_000 || payableCredit != 1_000_000 {
		t.Fatalf("156 debit %d / 331 credit %d, want 1,000,000 each", inventoryDebit, payableCredit)
	}
	if movements != 1 {
		t.Fatalf("stock movements = %d, want 1", movements)
	}

	body := h.body(t, "/stock")
	for _, want := range []string{"HH-01", "1.000.000", "100.000"} {
		if !strings.Contains(body, want) {
			t.Fatalf("the stock screen does not show %q:\n%s", want, body)
		}
	}
}

func TestStockReceiptNeedsASourceDocument(t *testing.T) {
	h := newHarness(t)
	email := newEmail()
	h.user(t, email, "correct-horse", "accountant")
	h.login(t, email, "correct-horse")
	client := h.newClient(t)
	h.pickClient(t, client)

	itemID := h.addItem(t, client, "HH-02", "Hàng hóa 02")

	resp, err := h.client.PostForm(h.server.URL+"/stock/receive", url.Values{
		"item_id":    {itemID},
		"qty":        {"5"},
		"unit_cost":  {"100.000"},
		"date":       {"2026-03-05"},
		"source_doc": {""},
	})
	if err != nil {
		t.Fatalf("receive: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("receipt with no source document = %d, want 422", resp.StatusCode)
	}
}

func TestStocktakeAdjustsToTheCountedQuantity(t *testing.T) {
	h := newHarness(t)
	email := newEmail()
	h.user(t, email, "correct-horse", "accountant")
	h.login(t, email, "correct-horse")
	client := h.newClient(t)
	h.pickClient(t, client)

	itemID := h.addItem(t, client, "HH-03", "Hàng hóa 03")
	h.receiveStock(t, itemID, "10", "100.000", "2026-03-05", "HĐ 0009")

	resp, err := h.client.PostForm(h.server.URL+"/stock/count", url.Values{
		"item_id": {itemID}, "counted": {"7"}, "date": {"2026-12-31"}, "source_doc": {"BB kiểm kê 31/12"},
	})
	if err != nil {
		t.Fatalf("stocktake: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("POST /stock/count = %d\n%s", resp.StatusCode, readAll(t, resp))
	}

	st := h.storeOf(t)
	scope := h.scopeOf(t, client)

	var cogs, inventoryCredit int64
	var onHand int
	if err := st.InClient(t.Context(), scope, func(tx pgx.Tx) error {
		if err := tx.QueryRow(t.Context(), `
			SELECT COALESCE(SUM(CASE WHEN account_code = '632' AND side = 'DEBIT'  THEN amount END), 0),
			       COALESCE(SUM(CASE WHEN account_code = '156' AND side = 'CREDIT' THEN amount END), 0)
			  FROM voucher_lines`).Scan(&cogs, &inventoryCredit); err != nil {
			return err
		}
		return tx.QueryRow(t.Context(),
			`SELECT COALESCE(SUM(CASE WHEN kind = 'RECEIPT' THEN quantity ELSE -quantity END), 0)::int
			   FROM stock_movements`).Scan(&onHand)
	}); err != nil {
		t.Fatalf("read the books: %v", err)
	}

	if cogs != 300_000 || inventoryCredit != 300_000 {
		t.Fatalf("shortage cost %d / inventory credit %d, want 300,000 each", cogs, inventoryCredit)
	}
	if onHand != 7 {
		t.Fatalf("stock on hand = %d, want 7", onHand)
	}

	again, err := h.client.PostForm(h.server.URL+"/stock/count", url.Values{
		"item_id": {itemID}, "counted": {"7"}, "date": {"2026-12-31"},
	})
	if err != nil {
		t.Fatalf("stocktake again: %v", err)
	}
	defer func() { _ = again.Body.Close() }()

	if again.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("a count that matches = %d, want 422 (nothing to adjust)", again.StatusCode)
	}
}

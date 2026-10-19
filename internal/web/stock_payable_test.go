package web_test

import (
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
)

// TestStockReceiptCreatesThePayable is C1: a receipt on credit must produce the AP
// document as well as the 331 credit, or the control account and the aging
// subledger disagree from the first purchase.
func TestStockReceiptCreatesThePayable(t *testing.T) {
	h := newHarness(t)
	email := newEmail()
	h.user(t, email, "correct-horse", "accountant")
	h.login(t, email, "correct-horse")
	client := h.newClient(t)
	h.pickClient(t, client)

	supplierID := h.addSupplier(t, client, "Đại Phát", "0312345678")
	itemID := h.addItem(t, client, "HH-A", "Hàng A")

	resp, err := h.client.PostForm(h.server.URL+"/stock/receive", url.Values{
		"item_id": {itemID}, "partner_id": {supplierID},
		"qty": {"100"}, "unit_cost": {"420.000"},
		"date": {"2026-05-10"}, "source_doc": {"PNK-01 / HĐ 0001"},
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

	var docs int
	var total int64
	var partner string
	if err := st.InClient(t.Context(), scope, func(tx pgx.Tx) error {
		if err := tx.QueryRow(t.Context(), `
			SELECT count(*), COALESCE(MAX(total), 0), COALESCE(MAX(partner_id::text), '')
			  FROM documents WHERE kind = 'PURCHASE_BILL' AND status = 'OPEN'`).
			Scan(&docs, &total, &partner); err != nil {
			return err
		}
		return nil
	}); err != nil {
		t.Fatalf("read documents: %v", err)
	}

	if docs != 1 {
		t.Fatalf("AP documents = %d, want 1: the stock receipt posted 331 with no payable", docs)
	}
	if total != 42_000_000 {
		t.Fatalf("AP document total = %d, want 42,000,000", total)
	}
	if partner != supplierID {
		t.Fatalf("AP document partner = %q, want the supplier", partner)
	}

	// The payment screen must offer it, and aging must reconcile to 331.
	payment := h.body(t, "/payment")
	if !strings.Contains(payment, "Đại Phát") || !strings.Contains(payment, "42.000.000") {
		t.Fatalf("the payable is not offered for payment:\n%s", payment)
	}

	aging := h.body(t, "/aging")
	if strings.Contains(aging, "bg-red-lt") {
		t.Fatalf("aging reports a difference against 331 after one clean receipt:\n%s", aging)
	}
}

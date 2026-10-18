package web_test

import (
	"net/http"
	"net/url"
	"strings"
	"testing"
)

func TestVATSummaryReadsOutputAndInputFromTheLedger(t *testing.T) {
	h := newHarness(t)
	email := newEmail()
	h.user(t, email, "correct-horse", "accountant")
	h.login(t, email, "correct-horse")
	client := h.newClient(t)
	h.pickClient(t, client)

	h.addPartner(t, "Khách VAT", "")
	partnerID := h.partnerID(t, client, "Khách VAT")

	sale, err := h.client.PostForm(h.server.URL+"/sales", url.Values{
		"partner_id": {partnerID}, "date": {"2026-05-10"}, "description": {"Bán hàng"},
		"item": {"HH-10", "", ""}, "quantity": {"1", "1", "1"},
		"unit_price": {"1.000.000", "", ""}, "vat_rate": {"vat10", "vat10", "vat10"},
	})
	if err != nil { t.Fatalf("sale: %v", err) }
	_ = sale.Body.Close()

	supplierID := h.addSupplier(t, client, "NCC VAT", "")
	bill, err := h.client.PostForm(h.server.URL+"/purchase", url.Values{
		"partner_id": {supplierID}, "date": {"2026-05-11"}, "description": {"Mua hàng"},
		"expense_account": {"6422"}, "net": {"1.000.000"}, "vat_rate": {"vat5"},
	})
	if err != nil { t.Fatalf("purchase: %v", err) }
	_ = bill.Body.Close()

	resp, err := h.client.Get(h.server.URL + "/reports/vat")
	if err != nil { t.Fatalf("vat: %v", err) }
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /reports/vat = %d", resp.StatusCode)
	}
	body := readAll(t, resp)

	// Output 100.000 (10% of 1.000.000); input 50.000 (5% of 1.000.000);
	// payable 50.000; taxable revenue 1.000.000.
	for _, want := range []string{"100.000", "50.000", "1.000.000", "33311", "1331"} {
		if !strings.Contains(body, want) {
			t.Fatalf("VAT summary missing %q:\n%s", want, body)
		}
	}
}
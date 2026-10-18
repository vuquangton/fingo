package web_test

import (
	"net/http"
	"net/url"
	"testing"

	"github.com/jackc/pgx/v5"
)

func TestPurchaseClaimsInputVATAtTheChosenRate(t *testing.T) {
	h := newHarness(t)
	email := newEmail()
	h.user(t, email, "correct-horse", "accountant")
	h.login(t, email, "correct-horse")
	client := h.newClient(t)
	h.pickClient(t, client)

	supplierID := h.addSupplier(t, client, "Nhà cung cấp 5%", "")

	bill := func(rate, net string) *http.Response {
		t.Helper()
		resp, err := h.client.PostForm(h.server.URL+"/purchase", url.Values{
			"partner_id": {supplierID}, "date": {"2026-05-10"},
			"description": {"Hóa đơn mua"}, "expense_account": {"6422"},
			"net": {net}, "vat_rate": {rate},
		})
		if err != nil {
			t.Fatalf("post bill: %v", err)
		}
		return resp
	}

	first := bill("vat5", "1.000.000")
	defer func() { _ = first.Body.Close() }()
	if first.StatusCode != http.StatusOK {
		t.Fatalf("5%% bill = %d\n%s", first.StatusCode, readAll(t, first))
	}

	second := bill("vat0", "2.000.000")
	defer func() { _ = second.Body.Close() }()
	if second.StatusCode != http.StatusOK {
		t.Fatalf("0%% bill = %d\n%s", second.StatusCode, readAll(t, second))
	}

	var inputVAT, payable int64
	var vatLines int
	if err := h.storeOf(t).InClient(t.Context(), h.scopeOf(t, client), func(tx pgx.Tx) error {
		if err := tx.QueryRow(t.Context(), `
			SELECT COALESCE(SUM(CASE WHEN account_code = '1331' AND side = 'DEBIT'  THEN amount END), 0),
			       COALESCE(SUM(CASE WHEN account_code = '331'  AND side = 'CREDIT' THEN amount END), 0),
			       count(*) FILTER (WHERE account_code = '1331')
			  FROM voucher_lines`).Scan(&inputVAT, &payable, &vatLines); err != nil {
			return err
		}
		return nil
	}); err != nil {
		t.Fatalf("read: %v", err)
	}

	// 5% of 1.000.000 = 50.000; the 0% bill claims nothing and posts no 1331 line.
	if inputVAT != 50_000 {
		t.Fatalf("input VAT = %d, want 50,000", inputVAT)
	}
	if payable != 3_050_000 {
		t.Fatalf("payable = %d, want 3,050,000", payable)
	}
	if vatLines != 1 {
		t.Fatalf("1331 lines = %d, want 1 (a zero-rated bill has no input VAT line)", vatLines)
	}
}

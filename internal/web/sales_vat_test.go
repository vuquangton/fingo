package web_test

import (
	"net/http"
	"net/url"
	"testing"

	"github.com/jackc/pgx/v5"
)

// TestSalePostsEachLinesOwnVATRate proves the rate menu reaches the ledger: the
// engine computes tax per line from the id, so a 5% line must post 5%, not 10%.
func TestSalePostsEachLinesOwnVATRate(t *testing.T) {
	h := newHarness(t)
	email := newEmail()
	h.user(t, email, "correct-horse", "accountant")
	h.login(t, email, "correct-horse")
	client := h.newClient(t)
	h.pickClient(t, client)

	h.addPartner(t, "Công ty Năm Phần Trăm", "")
	partnerID := h.partnerID(t, client, "Công ty Năm Phần Trăm")

	// Two lines: 1.000.000 at 5%, 2.000.000 at 0%.
	resp, err := h.client.PostForm(h.server.URL+"/sales", url.Values{
		"partner_id":  {partnerID},
		"date":        {"2026-05-10"},
		"description": {"Bán hàng nhiều thuế suất"},
		"item":        {"HH-05", "HH-00", ""},
		"quantity":    {"1", "1", "1"},
		"unit_price":  {"1.000.000", "2.000.000", ""},
		"vat_rate":    {"vat5", "vat0", "vat10"},
	})
	if err != nil {
		t.Fatalf("post sale: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("POST /sales = %d\n%s", resp.StatusCode, readAll(t, resp))
	}

	var vat, revenue, receivable int64
	if err := h.storeOf(t).InClient(t.Context(), h.scopeOf(t, client), func(tx pgx.Tx) error {
		if err := tx.QueryRow(t.Context(), `
			SELECT COALESCE(SUM(CASE WHEN account_code = '33311' AND side = 'CREDIT' THEN amount END), 0),
			       COALESCE(SUM(CASE WHEN account_code = '5111'  AND side = 'CREDIT' THEN amount END), 0),
			       COALESCE(SUM(CASE WHEN account_code = '131'   AND side = 'DEBIT'  THEN amount END), 0)
			  FROM voucher_lines`).Scan(&vat, &revenue, &receivable); err != nil {
			return err
		}
		return nil
	}); err != nil {
		t.Fatalf("read the books: %v", err)
	}

	// 5% of 1.000.000 = 50.000; 0% of 2.000.000 = 0.
	if vat != 50_000 {
		t.Fatalf("output VAT = %d, want 50,000 (5%% of 1,000,000, plus 0%% of 2,000,000)", vat)
	}
	if revenue != 3_000_000 {
		t.Fatalf("revenue = %d, want 3,000,000", revenue)
	}
	if receivable != 3_050_000 {
		t.Fatalf("receivable = %d, want 3,050,000", receivable)
	}
}

// TestExemptSalePostsNoOutputVAT is the KCT case: the invoice is exempt, so 33311
// must not move, and the receivable equals the revenue exactly.
func TestExemptSalePostsNoOutputVAT(t *testing.T) {
	h := newHarness(t)
	email := newEmail()
	h.user(t, email, "correct-horse", "accountant")
	h.login(t, email, "correct-horse")
	client := h.newClient(t)
	h.pickClient(t, client)

	h.addPartner(t, "Khách KCT", "")
	partnerID := h.partnerID(t, client, "Khách KCT")

	resp, err := h.client.PostForm(h.server.URL+"/sales", url.Values{
		"partner_id": {partnerID}, "date": {"2026-05-10"}, "description": {"Hàng KCT"},
		"item":       {"KCT-01", "", ""},
		"quantity":   {"1", "1", "1"},
		"unit_price": {"1.000.000", "", ""},
		"vat_rate":   {"kct", "vat10", "vat10"},
	})
	if err != nil {
		t.Fatalf("post sale: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("POST /sales = %d\n%s", resp.StatusCode, readAll(t, resp))
	}

	var vat, revenue, receivable int64
	if err := h.storeOf(t).InClient(t.Context(), h.scopeOf(t, client), func(tx pgx.Tx) error {
		return tx.QueryRow(t.Context(), `
			SELECT COALESCE(SUM(CASE WHEN account_code = '33311' AND side = 'CREDIT' THEN amount END), 0),
			       COALESCE(SUM(CASE WHEN account_code = '5111'  AND side = 'CREDIT' THEN amount END), 0),
			       COALESCE(SUM(CASE WHEN account_code = '131'   AND side = 'DEBIT'  THEN amount END), 0)
			  FROM voucher_lines`).Scan(&vat, &revenue, &receivable)
	}); err != nil {
		t.Fatalf("read: %v", err)
	}

	if vat != 0 {
		t.Fatalf("exempt sale posted %d of output VAT, want 0", vat)
	}
	if revenue != 1_000_000 || receivable != 1_000_000 {
		t.Fatalf("revenue %d / receivable %d, want 1,000,000 each", revenue, receivable)
	}
}

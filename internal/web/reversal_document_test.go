package web_test

import (
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
)

// TestReversingAReceiptReopensTheInvoice is C4: the cash goes back, so the invoice
// must go back to owing it. A reversal that leaves the document settled makes 131
// disagree with the aging subledger for the rest of the year.
func TestReversingAReceiptReopensTheInvoice(t *testing.T) {
	h := newHarness(t)
	email := newEmail()
	h.user(t, email, "correct-horse", "accountant")
	h.login(t, email, "correct-horse")
	client := h.newClient(t)
	h.pickClient(t, client)

	invoiceID := h.sellOne(t, client, "Công ty Đông Á", "500.000")

	receipt, err := h.client.PostForm(h.server.URL+"/receipt", url.Values{
		"document_id": {invoiceID}, "amount": {"1.100.000"}, "date": {"2026-03-20"},
	})
	if err != nil {
		t.Fatalf("receipt: %v", err)
	}
	_ = receipt.Body.Close()
	if receipt.StatusCode != http.StatusOK {
		t.Fatalf("receipt = %d", receipt.StatusCode)
	}

	var receiptNo string
	if err := h.storeOf(t).InClient(t.Context(), h.scopeOf(t, client), func(tx pgx.Tx) error {
		return tx.QueryRow(t.Context(), `
			SELECT voucher_no FROM vouchers WHERE description = 'Thu tiền khách hàng'
			 ORDER BY created_at DESC LIMIT 1`).Scan(&receiptNo)
	}); err != nil {
		t.Fatalf("find receipt voucher: %v", err)
	}

	reverse, err := h.client.PostForm(h.server.URL+"/reverse", url.Values{
		"voucher_no": {receiptNo}, "date": {"2026-03-25"},
	})
	if err != nil {
		t.Fatalf("reverse: %v", err)
	}
	defer func() { _ = reverse.Body.Close() }()
	if reverse.StatusCode != http.StatusOK {
		t.Fatalf("reverse = %d\n%s", reverse.StatusCode, readAll(t, reverse))
	}

	var status string
	var allocated int64
	if err := h.storeOf(t).InClient(t.Context(), h.scopeOf(t, client), func(tx pgx.Tx) error {
		return tx.QueryRow(t.Context(),
			`SELECT status, allocated FROM documents WHERE id = $1`, invoiceID).Scan(&status, &allocated)
	}); err != nil {
		t.Fatalf("read document: %v", err)
	}

	if status != "OPEN" || allocated != 0 {
		t.Fatalf("after reversing the receipt the invoice is %s / %d, want OPEN / 0", status, allocated)
	}

	// The ledger and the subledger agree again.
	aging := h.body(t, "/aging")
	if strings.Contains(aging, "bg-red-lt") {
		t.Fatalf("aging reports a difference after reversing a receipt:\n%s", aging)
	}
}

// TestReversingPartOfAPaymentRestoresTheRemainder checks the partial case.
func TestReversingPartOfAPaymentRestoresTheRemainder(t *testing.T) {
	h := newHarness(t)
	email := newEmail()
	h.user(t, email, "correct-horse", "accountant")
	h.login(t, email, "correct-horse")
	client := h.newClient(t)
	h.pickClient(t, client)

	invoiceID := h.sellOne(t, client, "Khách trả một phần", "500.000")

	resp, err := h.client.PostForm(h.server.URL+"/receipt", url.Values{
		"document_id": {invoiceID}, "amount": {"400.000"}, "date": {"2026-03-20"},
	})
	if err != nil {
		t.Fatalf("receipt: %v", err)
	}
	_ = resp.Body.Close()

	var status string
	var allocated int64
	if err := h.storeOf(t).InClient(t.Context(), h.scopeOf(t, client), func(tx pgx.Tx) error {
		return tx.QueryRow(t.Context(),
			`SELECT status, allocated FROM documents WHERE id = $1`, invoiceID).Scan(&status, &allocated)
	}); err != nil {
		t.Fatalf("read document: %v", err)
	}
	if status != "PARTIAL" || allocated != 400_000 {
		t.Fatalf("before reversal: %s / %d, want PARTIAL / 400,000", status, allocated)
	}

	var receiptNo string
	if err := h.storeOf(t).InClient(t.Context(), h.scopeOf(t, client), func(tx pgx.Tx) error {
		return tx.QueryRow(t.Context(), `
			SELECT voucher_no FROM vouchers WHERE description = 'Thu tiền khách hàng'
			 ORDER BY created_at DESC LIMIT 1`).Scan(&receiptNo)
	}); err != nil {
		t.Fatalf("find receipt voucher: %v", err)
	}

	rev, err := h.client.PostForm(h.server.URL+"/reverse", url.Values{
		"voucher_no": {receiptNo}, "date": {"2026-03-25"},
	})
	if err != nil {
		t.Fatalf("reverse: %v", err)
	}
	defer func() { _ = rev.Body.Close() }()
	if rev.StatusCode != http.StatusOK {
		t.Fatalf("reverse = %d\n%s", rev.StatusCode, readAll(t, rev))
	}

	if err := h.storeOf(t).InClient(t.Context(), h.scopeOf(t, client), func(tx pgx.Tx) error {
		return tx.QueryRow(t.Context(),
			`SELECT status, allocated FROM documents WHERE id = $1`, invoiceID).Scan(&status, &allocated)
	}); err != nil {
		t.Fatalf("read document after reversal: %v", err)
	}
	if status != "OPEN" || allocated != 0 {
		t.Fatalf("after reversing a partial receipt: %s / %d, want OPEN / 0", status, allocated)
	}
}

// TestReversingAnOverpaymentUnwindsTheCredit covers the third case: the receipt
// paid more than the invoice, the excess became a customer credit, and reversing
// the receipt must put the invoice back and reverse the credit.
func TestReversingAnOverpaymentUnwindsTheCredit(t *testing.T) {
	h := newHarness(t)
	email := newEmail()
	h.user(t, email, "correct-horse", "accountant")
	h.login(t, email, "correct-horse")
	client := h.newClient(t)
	h.pickClient(t, client)

	invoiceID := h.sellOne(t, client, "Khách trả thừa", "500.000")

	resp, err := h.client.PostForm(h.server.URL+"/receipt", url.Values{
		"document_id": {invoiceID}, "amount": {"2.000.000"}, "date": {"2026-03-20"},
	})
	if err != nil {
		t.Fatalf("receipt: %v", err)
	}
	_ = resp.Body.Close()

	var receiptNo string
	if err := h.storeOf(t).InClient(t.Context(), h.scopeOf(t, client), func(tx pgx.Tx) error {
		return tx.QueryRow(t.Context(), `
			SELECT voucher_no FROM vouchers WHERE description = 'Thu tiền khách hàng'
			 ORDER BY created_at DESC LIMIT 1`).Scan(&receiptNo)
	}); err != nil {
		t.Fatalf("find receipt: %v", err)
	}

	rev, err := h.client.PostForm(h.server.URL+"/reverse", url.Values{
		"voucher_no": {receiptNo}, "date": {"2026-03-25"},
	})
	if err != nil {
		t.Fatalf("reverse: %v", err)
	}
	defer func() { _ = rev.Body.Close() }()
	if rev.StatusCode != http.StatusOK {
		t.Fatalf("reverse = %d\n%s", rev.StatusCode, readAll(t, rev))
	}

	var credits, reversals int
	var status string
	var allocated int64
	if err := h.storeOf(t).InClient(t.Context(), h.scopeOf(t, client), func(tx pgx.Tx) error {
		if err := tx.QueryRow(t.Context(), `SELECT count(*) FROM document_credits`).Scan(&credits); err != nil {
			return err
		}
		if err := tx.QueryRow(t.Context(), `SELECT count(*) FROM document_credit_reversals`).Scan(&reversals); err != nil {
			return err
		}
		return tx.QueryRow(t.Context(),
			`SELECT status, allocated FROM documents WHERE id = $1`, invoiceID).Scan(&status, &allocated)
	}); err != nil {
		t.Fatalf("read: %v", err)
	}

	if credits != 1 || reversals != 1 {
		t.Fatalf("credits %d / reversals %d, want 1 / 1", credits, reversals)
	}
	if status != "OPEN" || allocated != 0 {
		t.Fatalf("after reversal: %s / %d, want OPEN / 0", status, allocated)
	}
}

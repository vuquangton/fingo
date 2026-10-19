package web_test

import (
	"net/http"
	"net/url"
	"testing"

	"github.com/jackc/pgx/v5"
)

// TestUnresolvedVATWindowNeedsAnAcknowledgement covers C3's mechanism: while a
// flagged 8% window covers the invoice date, a sale cannot post until the operator
// confirms it was reviewed, and the confirmation is written to the audit log.
func TestUnresolvedVATWindowNeedsAnAcknowledgement(t *testing.T) {
	h := newHarness(t)
	email := newEmail()
	h.user(t, email, "correct-horse", "accountant")
	h.login(t, email, "correct-horse")
	client := h.newClient(t)
	h.pickClient(t, client)

	// A practitioner-confirmed window is not invented here: the test enters one to
	// exercise the mechanism, and the default catalog has none.
	if _, err := h.pool.Exec(t.Context(), `
		INSERT INTO vat_rates (id, name, basis_points, effective_from) VALUES ('vat8', '8% test', 800, '2026-01-01')
		ON CONFLICT (id) DO NOTHING`); err != nil {
		t.Fatalf("insert vat8 rate: %v", err)
	}

	if _, err := h.pool.Exec(t.Context(), `
		INSERT INTO vat_unresolved_windows (rate_id, from_date, to_date, note)
		VALUES ('vat8', '2026-01-01', '2026-12-31', 'test window')`); err != nil {
		t.Fatalf("insert window: %v", err)
	}

	h.addPartner(t, "Khách 8%", "")
	partnerID := h.partnerID(t, client, "Khách 8%")

	form := url.Values{
		"partner_id": {partnerID}, "date": {"2026-05-10"}, "description": {"Có thể 8%"},
		"item": {"HH-08", "", ""}, "quantity": {"1", "1", "1"},
		"unit_price": {"1.000.000", "", ""}, "vat_rate": {"vat10", "vat10", "vat10"},
	}

	refused, err := h.client.PostForm(h.server.URL+"/sales", form)
	if err != nil {
		t.Fatalf("post: %v", err)
	}
	defer func() { _ = refused.Body.Close() }()
	if refused.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("unacknowledged sale = %d, want 422", refused.StatusCode)
	}

	form.Set("vat8_ack", "1")
	accepted, err := h.client.PostForm(h.server.URL+"/sales", form)
	if err != nil {
		t.Fatalf("post with ack: %v", err)
	}
	defer func() { _ = accepted.Body.Close() }()
	if accepted.StatusCode != http.StatusOK {
		t.Fatalf("acknowledged sale = %d\n%s", accepted.StatusCode, readAll(t, accepted))
	}

	var acks int
	if err := h.storeOf(t).InClient(t.Context(), h.scopeOf(t, client), func(tx pgx.Tx) error {
		return tx.QueryRow(t.Context(),
			`SELECT count(*) FROM audit_log WHERE action = 'VAT_RATE_ACK'`).Scan(&acks)
	}); err != nil {
		t.Fatalf("read audit: %v", err)
	}
	if acks != 1 {
		t.Fatalf("audit acknowledgements = %d, want 1", acks)
	}
}

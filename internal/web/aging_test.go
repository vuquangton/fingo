package web_test

import (
	"net/http"
	"net/url"
	"strings"
	"testing"
)

func TestAgingBucketsByDaysOverdueAndTiesToTheLedger(t *testing.T) {
	h := newHarness(t)
	email := newEmail()
	h.user(t, email, "correct-horse", "accountant")
	h.login(t, email, "correct-horse")
	client := h.newClient(t)
	h.pickClient(t, client)

	h.addPartner(t, "Công ty Đông Á", "")
	partnerID := h.partnerID(t, client, "Công ty Đông Á")

	sell := func(date string) {
		t.Helper()
		resp, err := h.client.PostForm(h.server.URL+"/sales", url.Values{
			"partner_id":  {partnerID},
			"date":        {date},
			"description": {"Bán hàng"},
			"item":        {"DV-01", "", ""},
			"quantity":    {"1", "1", "1"},
			"unit_price":  {"1.000.000", "", ""},
		})
		if err != nil {
			t.Fatalf("sale %s: %v", date, err)
		}
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("sale %s = %d\n%s", date, resp.StatusCode, readAll(t, resp))
		}
	}

	// Due date is document date + 30, so these land in three different buckets
	// as at 30 June 2026: 119 days, 46 days, and not yet due.
	sell("2026-02-01")
	sell("2026-04-15")
	sell("2026-06-20")

	body := h.body(t, "/aging?as_of=2026-06-30")

	for _, want := range []string{
		"Phải thu khách hàng",
		"khớp",      // subledger equals the 131 control account
		"3.300.000", // three invoices of 1.100.000
		"Công ty Đông Á",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("the aging screen does not show %q:\n%s", want, body)
		}
	}

	// Each invoice is in exactly one bucket, so three buckets of 1.100.000 exist
	// and the "over 90" bucket holds exactly one of them.
	if got := strings.Count(body, "1.100.000"); got < 5 {
		t.Fatalf("expected five occurrences of 1.100.000 (three buckets, totals, ledger), got %d", got)
	}
}

func TestAgingRefusesABadDate(t *testing.T) {
	h := newHarness(t)
	email := newEmail()
	h.user(t, email, "correct-horse", "accountant")
	h.login(t, email, "correct-horse")
	h.pickClient(t, h.newClient(t))

	resp, err := h.client.Get(h.server.URL + "/aging?as_of=not-a-date")
	if err != nil {
		t.Fatalf("aging: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("bad date = %d, want 422", resp.StatusCode)
	}
}

package web_test

import (
	"io"
	"net/url"
	"strings"
	"testing"
)

// postSale posts the one voucher every read-screen test reads back.
func (h *harness) postSale(t *testing.T) {
	t.Helper()

	resp, err := h.client.PostForm(h.server.URL+"/entry", url.Values{
		"date":        {"2026-03-15"},
		"description": {"Bán hàng cho khách"},
		"account":     {"131", "5111", "33311"},
		"side":        {"DEBIT", "CREDIT", "CREDIT"},
		"amount":      {"1.100.000", "1.000.000", "100.000"},
	})
	if err != nil {
		t.Fatalf("post entry: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("post entry = %d\n%s", resp.StatusCode, body)
	}
}

func (h *harness) body(t *testing.T, path string) string {
	t.Helper()

	resp := h.get(t, path)
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("GET %s = %d, want 200\n%s", path, resp.StatusCode, raw)
	}
	return string(raw)
}

func TestAccountsScreenListsTheChart(t *testing.T) {
	h := newHarness(t)
	email := newEmail()
	h.user(t, email, "correct-horse", "accountant")
	h.login(t, email, "correct-horse")
	h.pickClient(t, h.newClient(t))

	body := h.body(t, "/accounts")

	for _, want := range []string{"Hệ thống tài khoản", "5111", "33311", "911"} {
		if !strings.Contains(body, want) {
			t.Fatalf("the chart screen does not contain %q\n%s", want, body)
		}
	}
}

func TestJournalScreenShowsThePostedVoucher(t *testing.T) {
	h := newHarness(t)
	email := newEmail()
	h.user(t, email, "correct-horse", "accountant")
	h.login(t, email, "correct-horse")
	h.pickClient(t, h.newClient(t))
	h.postSale(t)

	body := h.body(t, "/journal")

	for _, want := range []string{"Sổ nhật ký chung", "Bán hàng cho khách", "1.100.000"} {
		if !strings.Contains(body, want) {
			t.Fatalf("the journal does not contain %q\n%s", want, body)
		}
	}
}

func TestTrialBalanceScreenBalances(t *testing.T) {
	h := newHarness(t)
	email := newEmail()
	h.user(t, email, "correct-horse", "accountant")
	h.login(t, email, "correct-horse")
	h.pickClient(t, h.newClient(t))
	h.postSale(t)

	body := h.body(t, "/reports")

	for _, want := range []string{"Bảng cân đối số phát sinh", "5111", "1.100.000", "cân"} {
		if !strings.Contains(body, want) {
			t.Fatalf("the trial balance does not contain %q\n%s", want, body)
		}
	}

	if strings.Contains(body, "KHÔNG cân") {
		t.Fatal("the trial balance reports itself unbalanced after one balanced voucher")
	}
}

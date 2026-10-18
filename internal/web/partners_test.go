package web_test

import (
	"net/http"
	"net/url"
	"strings"
	"testing"
)

func TestPartnersScreenIsEmptyForANewClient(t *testing.T) {
	h := newHarness(t)
	email := newEmail()
	h.user(t, email, "correct-horse", "accountant")
	h.login(t, email, "correct-horse")
	h.pickClient(t, h.newClient(t))

	body := h.body(t, "/partners")

	for _, want := range []string{"Danh mục đối tượng", "Chưa có đối tượng nào"} {
		if !strings.Contains(body, want) {
			t.Fatalf("the partners screen does not contain %q\n%s", want, body)
		}
	}
}

func TestPartnerCanBeAdded(t *testing.T) {
	h := newHarness(t)
	email := newEmail()
	h.user(t, email, "correct-horse", "accountant")
	h.login(t, email, "correct-horse")
	h.pickClient(t, h.newClient(t))

	resp, err := h.client.PostForm(h.server.URL+"/partners", url.Values{
		"name":   {"Công ty Đông Á"},
		"tax_id": {"0101234567"},
		"kind":   {"CUSTOMER"},
	})
	if err != nil {
		t.Fatalf("add partner: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("POST /partners = %d, want 200 after the redirect\n%s", resp.StatusCode, readAll(t, resp))
	}

	body := h.body(t, "/partners")
	for _, want := range []string{"Công ty Đông Á", "0101234567", "Khách hàng"} {
		if !strings.Contains(body, want) {
			t.Fatalf("the list does not contain %q\n%s", want, body)
		}
	}
}

func TestDuplicatePartnerIsRefusedWithAReadableMessage(t *testing.T) {
	h := newHarness(t)
	email := newEmail()
	h.user(t, email, "correct-horse", "accountant")
	h.login(t, email, "correct-horse")
	h.pickClient(t, h.newClient(t))

	form := url.Values{"name": {"Công ty Bình Minh"}, "tax_id": {""}, "kind": {"SUPPLIER"}}

	first, err := h.client.PostForm(h.server.URL+"/partners", form)
	if err != nil {
		t.Fatalf("add partner: %v", err)
	}
	_ = first.Body.Close()

	second, err := h.client.PostForm(h.server.URL+"/partners", form)
	if err != nil {
		t.Fatalf("add partner again: %v", err)
	}
	defer func() { _ = second.Body.Close() }()

	if second.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("duplicate partner = %d, want 422", second.StatusCode)
	}

	body := readAll(t, second)
	if !strings.Contains(body, "đã tồn tại") && !strings.Contains(strings.ToLower(body), "exists") {
		t.Fatalf("the refusal does not explain the duplicate:\n%s", body)
	}
}

func TestPartnerNeedsAName(t *testing.T) {
	h := newHarness(t)
	email := newEmail()
	h.user(t, email, "correct-horse", "accountant")
	h.login(t, email, "correct-horse")
	h.pickClient(t, h.newClient(t))

	resp, err := h.client.PostForm(h.server.URL+"/partners", url.Values{
		"name": {"   "},
		"kind": {"CUSTOMER"},
	})
	if err != nil {
		t.Fatalf("add partner: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("blank name = %d, want 422", resp.StatusCode)
	}
}

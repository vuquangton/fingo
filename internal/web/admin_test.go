package web_test

import (
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
)

func TestAdminCreatesAClientAndOpensItsBooks(t *testing.T) {
	h := newHarness(t)
	admin := newEmail()
	h.user(t, admin, "correct-horse", "admin")
	h.login(t, admin, "correct-horse")

	// Create the client through the screen.
	resp, err := h.client.PostForm(h.server.URL+"/clients", url.Values{
		"name": {"Công ty Mới"}, "tax_id": {"020" + newUUID(t)[:8]},
	})
	if err != nil {
		t.Fatalf("create client: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("POST /clients = %d\n%s", resp.StatusCode, readAll(t, resp))
	}

	body := h.body(t, "/clients")
	if !strings.Contains(body, "Công ty Mới") {
		t.Fatalf("the new client is not listed:\n%s", body)
	}

	// Its chart exists: 123 accounts, scoped to the new client.
	var clientID string
	if err := h.pool.QueryRow(t.Context(),
		`SELECT id::text FROM clients WHERE name = 'Công ty Mới' ORDER BY created_at DESC LIMIT 1`).Scan(&clientID); err != nil {
		t.Fatalf("read the client: %v", err)
	}

	var accounts int
	if err := h.storeOf(t).InClient(t.Context(), h.scopeOf(t, clientID), func(tx pgx.Tx) error {
		return tx.QueryRow(t.Context(), `SELECT count(*) FROM accounts`).Scan(&accounts)
	}); err != nil {
		t.Fatalf("count accounts: %v", err)
	}
	if accounts != 123 {
		t.Fatalf("the new client has %d accounts, want 123", accounts)
	}

	// Switching to it works, and the books are empty.
	switchResp, err := h.client.PostForm(h.server.URL+"/clients/"+clientID+"/select", url.Values{})
	if err != nil {
		t.Fatalf("select client: %v", err)
	}
	_ = switchResp.Body.Close()

	journal := h.body(t, "/journal")
	if !strings.Contains(journal, "Chưa có chứng từ nào") {
		t.Fatalf("a new client's journal is not empty:\n%s", journal)
	}
}

func TestOnlyAnAdminMayCreateAClient(t *testing.T) {
	h := newHarness(t)
	accountant := newEmail()
	h.user(t, accountant, "correct-horse", "accountant")
	h.login(t, accountant, "correct-horse")

	resp, err := h.client.PostForm(h.server.URL+"/clients", url.Values{
		"name": {"Không được"}, "tax_id": {"0000000001"},
	})
	if err != nil {
		t.Fatalf("create client: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("an accountant creating a client = %d, want 403", resp.StatusCode)
	}
}

func TestAdminCreatesAUserWhoCanSignIn(t *testing.T) {
	h := newHarness(t)
	admin := newEmail()
	h.user(t, admin, "correct-horse", "admin")
	h.login(t, admin, "correct-horse")

	newUser := newEmail()

	resp, err := h.client.PostForm(h.server.URL+"/users", url.Values{
		"email":        {newUser},
		"display_name": {"Người dùng mới"},
		"role":         {"viewer"},
		"password":     {"a-long-enough-password"},
	})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("POST /users = %d\n%s", resp.StatusCode, readAll(t, resp))
	}

	// The new account signs in, and a viewer cannot post a sale.
	h.logout(t)
	h.login(t, newUser, "a-long-enough-password")

	sale, err := h.client.PostForm(h.server.URL+"/sales", url.Values{
		"partner_id": {"x"}, "date": {"2026-03-15"}, "item": {"HH"}, "quantity": {"1"}, "unit_price": {"1.000"},
	})
	if err != nil {
		t.Fatalf("post sale: %v", err)
	}
	defer func() { _ = sale.Body.Close() }()

	if sale.StatusCode != http.StatusForbidden {
		t.Fatalf("a viewer posting a sale = %d, want 403", sale.StatusCode)
	}
}

func TestPasswordChangeRequiresTheCurrentPassword(t *testing.T) {
	h := newHarness(t)
	email := newEmail()
	h.user(t, email, "correct-horse", "accountant")
	h.login(t, email, "correct-horse")

	wrong, err := h.client.PostForm(h.server.URL+"/password", url.Values{
		"current": {"not-it"}, "next": {"another-long-password"},
	})
	if err != nil {
		t.Fatalf("change password: %v", err)
	}
	defer func() { _ = wrong.Body.Close() }()

	if wrong.StatusCode != http.StatusForbidden {
		t.Fatalf("wrong current password = %d, want 403", wrong.StatusCode)
	}

	right, err := h.client.PostForm(h.server.URL+"/password", url.Values{
		"current": {"correct-horse"}, "next": {"another-long-password"},
	})
	if err != nil {
		t.Fatalf("change password: %v", err)
	}
	defer func() { _ = right.Body.Close() }()

	if right.StatusCode != http.StatusOK {
		t.Fatalf("change password = %d\n%s", right.StatusCode, readAll(t, right))
	}

	h.logout(t)
	h.login(t, email, "another-long-password")
}

// logout ends the session through the route, so the next login starts clean.
func (h *harness) logout(t *testing.T) {
	t.Helper()

	resp, err := h.client.PostForm(h.server.URL+"/logout", url.Values{})
	if err != nil {
		t.Fatalf("logout: %v", err)
	}
	_ = resp.Body.Close()
}

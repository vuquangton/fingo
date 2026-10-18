package web_test

import (
	"net/http"
	"net/url"
	"testing"
	"time"
)

// TestFailedLoginsAreRecordedAndCostNothing measures what an unauthenticated
// attacker pays for.
//
// An unknown email must not cost an argon2 verification: the account is not
// found, so no hash is computed. That is the property that keeps a password
// spray from turning into a CPU denial of service on an office machine.
func TestFailedLoginsAreRecordedAndCostNothing(t *testing.T) {
	h := newHarness(t)

	const attempts = 30

	// The development database accumulates between runs, so this run counts only
	// its own attempts, by a prefix nothing else uses.
	prefix := "spray-" + newUUID(t)[:8] + "-"

	start := time.Now()
	for i := 0; i < attempts; i++ {
		resp, err := h.client.PostForm(h.server.URL+"/login", url.Values{
			"email":    {prefix + itoa(i) + "@example.invalid"},
			"password": {"wrong-password"},
		})
		if err != nil {
			t.Fatalf("login: %v", err)
		}
		_ = resp.Body.Close()

		if resp.StatusCode != http.StatusUnauthorized {
			t.Fatalf("attempt %d = %d, want 401", i, resp.StatusCode)
		}
	}
	elapsed := time.Since(start)

	// failed_logins is practice-level data, not a client's books.
	var recorded int
	if err := h.pool.QueryRow(t.Context(),
		`SELECT count(*) FROM failed_logins WHERE email LIKE $1`, prefix+"%").Scan(&recorded); err != nil {
		t.Fatalf("read failed logins: %v", err)
	}

	if recorded != attempts {
		t.Fatalf("recorded %d failed logins, want %d", recorded, attempts)
	}

	t.Logf("%d unknown-email attempts in %s (%.1f ms each)", attempts, elapsed,
		float64(elapsed.Milliseconds())/attempts)

	// There is no lockout: the record exists, nothing reads it. Recorded here so
	// the absence is deliberate rather than forgotten.
	if elapsed > 30*time.Second {
		t.Fatalf("%d attempts took %s: unauthenticated work is too expensive", attempts, elapsed)
	}
}

package onboarding_test

import (
	"context"
	"strings"
	"testing"

	"fingo/internal/onboarding"
)

// Nothing listening is the common case on a machine that has not had a database
// installed, and it must be a report rather than an error: "there is no server
// here" is an answer to the question the screen asked.
func TestDetect_ReportsNothingListening(t *testing.T) {
	// Port 1 is reserved and nothing binds it, so this fails immediately rather
	// than waiting out a timeout.
	got := onboarding.Detect(context.Background(), "127.0.0.1", "1")

	if got.Found {
		t.Fatal("a port nothing is listening on was reported as a server")
	}
	if got.Problem == "" {
		t.Error("nothing was found and no reason was given")
	}
	if got.Host != "127.0.0.1" || got.Port != "1" {
		t.Errorf("the report lost the address it was asked about: %+v", got)
	}
}

// An empty address falls back to the local defaults, so a screen can call this
// without knowing them.
func TestDetect_FallsBackToTheLocalDefaults(t *testing.T) {
	got := onboarding.Detect(context.Background(), "", "")

	if got.Host != "127.0.0.1" {
		t.Errorf("host = %q, want the local default", got.Host)
	}
	if got.Port != "3306" {
		t.Errorf("port = %q, want the default", got.Port)
	}
}

// And the case it exists for: a MariaDB that is actually running.
func TestDetect_FindsTheServerOnThisMachine(t *testing.T) {
	cfg := configuration(t)

	got := onboarding.Detect(context.Background(), cfg.Host, cfg.Port)
	if !got.Found {
		t.Fatalf("no server found at %s:%s: %s", cfg.Host, cfg.Port, got.Problem)
	}
	if strings.TrimSpace(got.Version) == "" {
		t.Error("a server was found and its version was not recorded")
	}
	if got.Problem != "" {
		t.Errorf("a server was found and a problem was reported too: %q", got.Problem)
	}

	// The version is shown to an operator, so it is worth seeing what they see.
	t.Logf("detected at %s:%s: %s", got.Host, got.Port, got.Version)

	// MariaDB announces itself as "5.5.5-10.x.y-MariaDB" for the benefit of
	// clients older than it is. That prefix is not the version and must not
	// reach the screen.
	if strings.HasPrefix(got.Version, "5.5.5-") {
		t.Errorf("the compatibility prefix reached the report: %q", got.Version)
	}
}

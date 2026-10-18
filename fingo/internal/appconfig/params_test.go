package appconfig_test

import (
	"strings"
	"testing"

	"fingo/internal/appconfig"
)

// Config returns a copy, and a copy that shares a map is not a copy.
//
// This is not hypothetical. onboarding.open takes the config and adds a dial
// timeout to its parameters; when the map was shared, that write landed on the
// caller's Connection. A probe — which is meant to change nothing — left the
// connection holding a parameter nobody typed, and the next time Connect saved
// it, the settings file recorded it. The symptom was a saved connection with a
// "timeout" in it that no operator had entered.
func TestConnection_ConfigDoesNotShareTheParametersMap(t *testing.T) {
	c, err := appconfig.NewConnection("db.internal", "3306", "dev", "pw", "fingo", "")
	if err != nil {
		t.Fatalf("NewConnection: %v", err)
	}

	before := c.Params()

	cfg := c.Config()
	cfg.Params["timeout"] = "10s"

	if after := c.Params(); after != before {
		t.Errorf("writing to the returned Config changed the connection:\n before %q\n after  %q",
			before, after)
	}
	if strings.Contains(c.Params(), "timeout") {
		t.Errorf("the connection acquired a parameter it was never given: %q", c.Params())
	}
}

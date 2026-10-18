package client_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	"fingo/internal/client"
)

var (
	day1 = time.Date(2026, time.March, 1, 0, 0, 0, 0, time.UTC)
	day2 = time.Date(2026, time.March, 2, 0, 0, 0, 0, time.UTC)
	day3 = time.Date(2026, time.March, 3, 0, 0, 0, 0, time.UTC)
)

// The whole of the lifecycle of a client record: it goes in, comes back,
// changes, and is closed without ever being deleted.
func TestClient_NormalisesWhatIsEntered(t *testing.T) {
	c, err := client.NewClient(
		"  c-1  ",
		"  Công ty Bình Minh  ",
		" 0101.234.567 ",
		day1.Add(9*time.Hour),
	)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	if c.ID != "c-1" {
		t.Errorf("id = %q, want %q", c.ID, "c-1")
	}
	if c.Name != "Công ty Bình Minh" {
		t.Errorf("name = %q, want the trimmed company name", c.Name)
	}
	if c.TaxID != "0101234567" {
		t.Errorf("tax id = %q, want the digits only", c.TaxID)
	}
	if !c.Active {
		t.Error("a newly registered client is active")
	}
	if !c.CreatedAt.Equal(day1.Add(9 * time.Hour)) {
		t.Errorf("created at = %s, want %s", c.CreatedAt, day1.Add(9*time.Hour))
	}
}

// A client is never registered without a date. An import that supplies none gets
// the moment it was registered rather than the zero time, which would sort first
// in every list and read as 0001.
func TestNewClient_StampsACreationInstant(t *testing.T) {
	before := time.Now().UTC()

	c, err := client.NewClient("c-1", "Công ty Bình Minh", "", time.Time{})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	if c.CreatedAt.IsZero() {
		t.Fatal("created at is the zero time")
	}
	if c.CreatedAt.Location() != time.UTC {
		t.Errorf("created at is in %s, want UTC", c.CreatedAt.Location())
	}
	if c.CreatedAt.Before(before.Add(-time.Second)) || c.CreatedAt.After(time.Now().UTC().Add(time.Second)) {
		t.Errorf("created at = %s, which is not now", c.CreatedAt)
	}
}

func TestClient_ValidateRefusesWhatCannotBeStored(t *testing.T) {
	cases := map[string]client.Client{
		"no id":         {Name: "Công ty Bình Minh"},
		"blank id":      {ID: "   ", Name: "Công ty Bình Minh"},
		"no name":       {ID: "c-1"},
		"blank name":    {ID: "c-1", Name: "\t"},
		"a long name":   {ID: "c-1", Name: strings.Repeat("a", client.MaxNameLen+1)},
		"a long tax id": {ID: "c-1", Name: "Công ty Bình Minh", TaxID: strings.Repeat("9", client.MaxTaxIDLen+1)},
	}

	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			if err := c.Validate(); !errors.Is(err, client.ErrInvalidClient) {
				t.Fatalf("Validate() = %v, want ErrInvalidClient", err)
			}
		})
	}
}

// A tax code is the same company written four ways. Without collapsing those,
// one client is onboarded twice and its books are split in two.
func TestNormalizeTaxID_CollapsesTheSameCodeWrittenDifferently(t *testing.T) {
	cases := map[string]string{
		"0101234567":     "0101234567",
		"0101 234 567":   "0101234567",
		"0101.234.567":   "0101234567",
		"0101234567-001": "0101234567-001",
		"  0101234567 ":  "0101234567",
		"":               "",
		"-":              "",
		"MST: none":      "",
	}

	for raw, want := range cases {
		if got := client.NormalizeTaxID(raw); got != want {
			t.Errorf("NormalizeTaxID(%q) = %q, want %q", raw, got, want)
		}
	}
}

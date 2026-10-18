package voucher_test

import (
	"testing"

	"fingo/internal/voucher"
)

// The number as it is written on the document: PREFIX-YYYY-NNNNN.
func TestFormat_RendersAsItIsWrittenOnTheDocument(t *testing.T) {
	cases := []struct {
		prefix voucher.Prefix
		year   int
		number int
		want   string
	}{
		{"JV", 2026, 42, "JV-2026-00042"},
		{"INV", 2026, 1, "INV-2026-00001"},
		{"PO", 2026, 999, "PO-2026-00999"},
		{"JV", 2027, 100000, "JV-2027-100000"},
		{"JV", 2026, 0, "JV-2026-00000"},
	}

	for _, c := range cases {
		if got := voucher.Format(c.prefix, c.year, c.number); got != c.want {
			t.Errorf("Format(%s, %d, %d) = %q, want %q", c.prefix, c.year, c.number, got, c.want)
		}
	}
}

// The width is a minimum, not a limit. Past 99,999 a run does not silently wrap
// back to a short number — which would collide with one already issued.
func TestFormat_DoesNotTruncatePastTheWidth(t *testing.T) {
	got := voucher.Format("INV", 2026, 1234567)

	if got != "INV-2026-1234567" {
		t.Errorf("Format = %q, want INV-2026-1234567", got)
	}
}

func TestPrefix_Valid(t *testing.T) {
	cases := map[string]bool{
		"JV":                true,
		"INV":               true,
		"PO":                true,
		"A":                 true,
		"ABCDEFGHIJKLMNOP":  true, // 16, the longest allowed
		"":                  false,
		"ABCDEFGHIJKLMNOPQ": false, // 17
		"jv":                false, // lower case would make the format ambiguous
		"J V":               false,
		"JV-1":              false, // a hyphen is the separator
		"JV_1":              false,
		"JV/1":              false,
		"TIẾNG":             false,
	}

	for prefix, want := range cases {
		if got := voucher.Prefix(prefix).Valid(); got != want {
			t.Errorf("Prefix(%q).Valid() = %v, want %v", prefix, got, want)
		}
	}
}

// A number is only a number once it is formatted, so the two must agree.
func TestFormat_AgreesWithWhatThePrefixAllows(t *testing.T) {
	for _, prefix := range []voucher.Prefix{"JV", "INV", "PO"} {
		if !prefix.Valid() {
			t.Fatalf("%s should be a valid prefix", prefix)
		}

		got := voucher.Format(prefix, 2026, 7)
		want := string(prefix) + "-2026-00007"
		if got != want {
			t.Errorf("Format(%s, 2026, 7) = %q, want %q", prefix, got, want)
		}
	}
}

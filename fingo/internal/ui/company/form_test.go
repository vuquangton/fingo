package company_test

import (
	"strings"
	"testing"

	"fingo/internal/ui/company"
)

// TestForm_SetTaxCode_AcceptsTheShapesACompanyHas — the Vietnamese mã số thuế
// has two shapes and no others: a ten-digit enterprise code, and a thirteen-digit
// branch code written as the parent's ten digits, a dash, and three more.
//
// Getting this wrong is not cosmetic. The tax code is printed on every
// declaration, and a code the tax authority does not recognise is a filing that
// is rejected after it has been submitted.
func TestForm_SetTaxCode_AcceptsTheShapesACompanyHas(t *testing.T) {
	tests := []struct {
		name  string
		typed string
		want  string
	}{
		{name: "an enterprise code", typed: "0123456789", want: "0123456789"},
		{name: "a branch code", typed: "0123456789-001", want: "0123456789-001"},
		{name: "an untouched field is not a mistake", typed: "", want: ""},
		{name: "with the spaces people type", typed: "  0123456789  ", want: "0123456789"},
		{name: "with spaces around the dash", typed: "0123456789 - 001", want: "0123456789-001"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var f company.Form

			if err := f.SetTaxCode(tt.typed); err != nil {
				t.Fatalf("SetTaxCode(%q): %v", tt.typed, err)
			}
			if got := f.TaxCode(); got != tt.want {
				t.Errorf("TaxCode = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestForm_SetTaxCode_RefusesWhatIsNotATaxCode — refused at the field, while the
// operator is standing in it, rather than at the end of a form.
func TestForm_SetTaxCode_RefusesWhatIsNotATaxCode(t *testing.T) {
	for _, typed := range []string{
		"abc",
		"12345",
		"01234567890",
		"0123456789-01",
		"0123456789-0011",
		"0123456789-abc",
		"0123456789/001",
	} {
		t.Run(typed, func(t *testing.T) {
			var f company.Form

			if err := f.SetTaxCode(typed); err == nil {
				t.Fatalf("SetTaxCode(%q) = nil, want a refusal", typed)
			}
		})
	}
}

// TestForm_SetTaxCode_SaysWhatItWanted — a refusal that does not show the shape
// leaves the operator guessing, and guessing at a tax code is how a wrong one
// reaches a filing.
func TestForm_SetTaxCode_SaysWhatItWanted(t *testing.T) {
	var f company.Form

	err := f.SetTaxCode("12345")
	if err == nil {
		t.Fatal("SetTaxCode(12345) = nil, want a refusal")
	}

	msg := err.Error()
	if !strings.Contains(msg, "10") {
		t.Errorf("refusal %q does not say how long the code should be", msg)
	}
}

// TestForm_SetName_KeepsWhatWasTyped — the name is free text, and the only thing
// the model does to it is trim it.
func TestForm_SetName_KeepsWhatWasTyped(t *testing.T) {
	var f company.Form

	if err := f.SetName("  Công ty TNHH Bình Minh  "); err != nil {
		t.Fatalf("SetName: %v", err)
	}
	if got := f.Name(); got != "Công ty TNHH Bình Minh" {
		t.Errorf("Name = %q, want it trimmed", got)
	}
}

// TestForm_Profile_NeedsAName — a company with no name cannot head a document.
//
// The tax code is deliberately not required here. A newly registered company may
// not have its mã số thuế in hand on the day the books are opened, and refusing
// to let the operator start work until it arrives is refusing to let them work.
func TestForm_Profile_NeedsAName(t *testing.T) {
	var f company.Form

	if _, err := f.Profile(); err == nil {
		t.Fatal("Profile() with no name = nil, want a refusal")
	}

	if err := f.SetName("Công ty Bình Minh"); err != nil {
		t.Fatalf("SetName: %v", err)
	}

	p, err := f.Profile()
	if err != nil {
		t.Fatalf("Profile() with a name: %v", err)
	}
	if p.Name != "Công ty Bình Minh" {
		t.Errorf("Profile().Name = %q", p.Name)
	}
	if p.TaxCode != "" {
		t.Errorf("Profile().TaxCode = %q, want empty for a company that has not given one", p.TaxCode)
	}
}

// TestForm_Profile_CarriesBothFields — the two things a first run collects.
func TestForm_Profile_CarriesBothFields(t *testing.T) {
	var f company.Form

	if err := f.SetName("Công ty Bình Minh"); err != nil {
		t.Fatal(err)
	}
	if err := f.SetTaxCode("0123456789-001"); err != nil {
		t.Fatal(err)
	}

	p, err := f.Profile()
	if err != nil {
		t.Fatalf("Profile: %v", err)
	}

	if p.Name != "Công ty Bình Minh" || p.TaxCode != "0123456789-001" {
		t.Errorf("Profile = %+v, want both fields", p)
	}
}

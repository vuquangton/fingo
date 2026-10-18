package partner_test

import (
	"testing"

	"fingo/internal/domain/partner"
)

func TestNewPartner_TrimsWhatPeoplePaste(t *testing.T) {
	p, err := partner.NewPartner(
		"p-1",
		"  KH001  ",
		"  Công ty TNHH Bình Minh  ",
		"  0123456789  ",
		partner.KindCustomer,
	)
	if err != nil {
		t.Fatalf("NewPartner: %v", err)
	}

	if p.Code != "KH001" {
		t.Errorf("Code = %q, want it trimmed", p.Code)
	}
	if p.Name != "Công ty TNHH Bình Minh" {
		t.Errorf("Name = %q, want it trimmed", p.Name)
	}
	if p.TaxCode != "0123456789" {
		t.Errorf("TaxCode = %q, want it trimmed", p.TaxCode)
	}
	if !p.IsActive {
		t.Error("a new partner is active")
	}
}

// The same business gets written four ways. Without normalising, it is onboarded
// twice and its receivables split across both records.
func TestNormalizeTaxCode_CollapsesTheWaysOneCodeIsWritten(t *testing.T) {
	want := "0123456789-001"

	cases := []string{
		"0123456789-001",
		"0123456789 - 001",
		"0123 456 789-001",
		"0123.456.789-001",
		" 0123456789-001 ",
	}

	for _, raw := range cases {
		if got := partner.NormalizeTaxCode(raw); got != want {
			t.Errorf("NormalizeTaxCode(%q) = %q, want %q", raw, got, want)
		}
	}
}

func TestNormalizeTaxCode_LeavesNothingWhenThereIsNothing(t *testing.T) {
	for _, raw := range []string{"", "   ", "...", "---"} {
		if got := partner.NormalizeTaxCode(raw); got != "" {
			t.Errorf("NormalizeTaxCode(%q) = %q, want empty", raw, got)
		}
	}
}

// A partner with no code or no name is not a partner: nobody could find it and
// nothing could be addressed to it.
func TestNewPartner_RefusesAnIncompletePartner(t *testing.T) {
	cases := map[string]struct {
		code, name string
		kind       partner.Kind
	}{
		"no code":      {"", "Bình Minh", partner.KindCustomer},
		"blank code":   {"   ", "Bình Minh", partner.KindCustomer},
		"no name":      {"KH001", "", partner.KindCustomer},
		"blank name":   {"KH001", "   ", partner.KindCustomer},
		"unknown kind": {"KH001", "Bình Minh", partner.Kind("MYSTERY")},
		"no kind":      {"KH001", "Bình Minh", partner.Kind("")},
	}

	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := partner.NewPartner("p-1", c.code, c.name, "", c.kind); err == nil {
				t.Fatalf("%s must be refused, got nil", name)
			}
		})
	}
}

// A partner who is both a customer and a supplier is one record, not two — they
// are the same balance to offset.
func TestKind_BothIsItsOwnKind(t *testing.T) {
	for _, kind := range []partner.Kind{partner.KindCustomer, partner.KindSupplier, partner.KindBoth} {
		if !kind.Valid() {
			t.Errorf("%s must be valid", kind)
		}
	}

	if partner.Kind("").Valid() {
		t.Error("an empty kind is not valid")
	}
}

// A tax code is optional — a small customer may not have one — so an empty one
// must not stop a partner being created.
func TestNewPartner_AcceptsAPartnerWithoutATaxCode(t *testing.T) {
	p, err := partner.NewPartner("p-2", "KH002", "Khách lẻ", "", partner.KindCustomer)
	if err != nil {
		t.Fatalf("a partner without a tax code must be accepted: %v", err)
	}
	if p.TaxCode != "" {
		t.Errorf("TaxCode = %q, want empty", p.TaxCode)
	}
}

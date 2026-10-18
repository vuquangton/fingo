package partner_test

import (
	"errors"
	"testing"

	"fingo/internal/partner"
)

// The interface the MariaDB adapter is written against is the one the in-memory
// directory satisfies, so a rule proved against one holds for the other.
var _ partner.Repository = (*partner.MemoryRepository)(nil)

func TestNewPartner_TrimsAndNormalisesWhatPeopleType(t *testing.T) {
	p, err := partner.NewPartner(
		"  p-1  ",
		"  Công ty TNHH Bình Minh  ",
		partner.TypeBoth,
		"  0123 456 789  ",
		30,
	)
	if err != nil {
		t.Fatalf("NewPartner: %v", err)
	}

	if p.ID != "p-1" {
		t.Errorf("ID = %q, want it trimmed", p.ID)
	}
	if p.Name != "Công ty TNHH Bình Minh" {
		t.Errorf("Name = %q, want it trimmed", p.Name)
	}
	if p.TaxID != "0123456789" {
		t.Errorf("TaxID = %q, want the separators collapsed", p.TaxID)
	}
	if !p.Active {
		t.Error("a new partner is active")
	}
	if p.PaymentTerms != 30 {
		t.Errorf("PaymentTerms = %d, want 30", p.PaymentTerms)
	}
}

// The same business gets written four ways. Without normalising, it is onboarded
// twice and the duplicate is undetectable — which is what the warning exists to
// catch.
func TestNormalizeTaxID_CollapsesTheWaysOneCodeIsWritten(t *testing.T) {
	want := "0123456789-001"

	cases := []string{
		"0123456789-001",
		"0123456789 - 001",
		"0123 456 789-001",
		"0123.456.789-001",
		" 0123456789-001 ",
		"0123/456/789-001",
	}

	for _, raw := range cases {
		if got := partner.NormalizeTaxID(raw); got != want {
			t.Errorf("NormalizeTaxID(%q) = %q, want %q", raw, got, want)
		}
	}
}

func TestNormalizeTaxID_LeavesNothingWhenThereIsNothing(t *testing.T) {
	for _, raw := range []string{"", "   ", "...", "---", "N/A"} {
		if got := partner.NormalizeTaxID(raw); got != "" {
			t.Errorf("NormalizeTaxID(%q) = %q, want empty", raw, got)
		}
	}
}

// One partner can be both a customer and a supplier, and that is one record with
// two roles rather than two records.
func TestType_BothAnswersYesToBothSides(t *testing.T) {
	both := partner.TypeBoth

	if !both.IsCustomer() {
		t.Error("a partner who is both must be usable on the sales side")
	}
	if !both.IsSupplier() {
		t.Error("a partner who is both must be usable on the purchase side")
	}

	if partner.TypeCustomer.IsSupplier() {
		t.Error("a customer is not a supplier")
	}
	if partner.TypeSupplier.IsCustomer() {
		t.Error("a supplier is not a customer")
	}

	for _, kind := range []partner.Type{partner.TypeCustomer, partner.TypeSupplier, partner.TypeBoth} {
		if !kind.Valid() {
			t.Errorf("%q must be a valid type", kind)
		}
	}
	for _, kind := range []partner.Type{"", "CUSTOMER", "vendor", partner.Type("MYSTERY")} {
		if kind.Valid() {
			t.Errorf("%q must not be a valid type", kind)
		}
	}
}

// A partner that could not be stored at all is refused, and the reason is one a
// caller can branch on.
func TestValidate_RefusesAPartnerThatCouldNotBeStored(t *testing.T) {
	cases := map[string]partner.Partner{
		"no id":               {ID: "", Name: "Bình Minh", Type: partner.TypeCustomer},
		"blank id":            {ID: "   ", Name: "Bình Minh", Type: partner.TypeCustomer},
		"no name":             {ID: "p-1", Name: "", Type: partner.TypeCustomer},
		"blank name":          {ID: "p-1", Name: "   ", Type: partner.TypeCustomer},
		"no type":             {ID: "p-1", Name: "Bình Minh", Type: ""},
		"unknown type":        {ID: "p-1", Name: "Bình Minh", Type: "MYSTERY"},
		"negative terms":      {ID: "p-1", Name: "Bình Minh", Type: partner.TypeBoth, PaymentTerms: -1},
		"absurdly long terms": {ID: "p-1", Name: "Bình Minh", Type: partner.TypeBoth, PaymentTerms: partner.MaxPaymentTerms + 1},
	}

	for name, p := range cases {
		t.Run(name, func(t *testing.T) {
			if err := p.Validate(); !errors.Is(err, partner.ErrInvalidPartner) {
				t.Fatalf("Validate() = %v, want ErrInvalidPartner", err)
			}
		})
	}
}

// Payment on delivery is a term, not an absence of one.
func TestValidate_AcceptsTheEndsOfThePaymentTermRange(t *testing.T) {
	for _, terms := range []int{0, 30, partner.MaxPaymentTerms} {
		p := partner.Partner{ID: "p-1", Name: "Bình Minh", Type: partner.TypeBoth, PaymentTerms: terms}
		if err := p.Validate(); err != nil {
			t.Errorf("payment terms of %d days: %v", terms, err)
		}
	}
}

// A tax id is optional — a small customer may have none — so an empty one must
// not stop a partner being created.
func TestNewPartner_AcceptsAPartnerWithoutATaxID(t *testing.T) {
	p, err := partner.NewPartner("p-2", "Khách lẻ", partner.TypeCustomer, "", 0)
	if err != nil {
		t.Fatalf("a partner without a tax id must be accepted: %v", err)
	}
	if p.TaxID != "" {
		t.Errorf("TaxID = %q, want empty", p.TaxID)
	}
}

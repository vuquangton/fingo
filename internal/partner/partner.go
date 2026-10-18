// Package partner holds the businesses a client trades with.
//
// A partner is the dimension that makes a receivable answerable. "TK 131 is
// 500,000,000" is a number; "TK 131 from Công ty Bình Minh is 500,000,000" is a
// position somebody can act on. TT133 keeps Sổ chi tiết TK 131 by customer,
// which is why the dimension belongs on the journal line rather than beside it.
//
// NOTE: this is the directory model. It coexists with internal/domain/partner,
// which carries a customer code and an address and is what the posting path
// currently uses. They are two models of the same idea and one has to go; that
// consolidation is a separate decision and is not made here. See the note at the
// top of migration 00013.
package partner

import (
	"errors"
	"fmt"
	"strings"
)

// Errors a caller may need to tell apart.
var (
	// ErrInvalidPartner is a partner that could not be stored: no id, no name,
	// an unknown type, or terms outside the range a credit term can take.
	ErrInvalidPartner = errors.New("partner: invalid partner")

	// ErrUnknownPartner is a lookup, update or delete of an id the directory
	// does not hold.
	ErrUnknownPartner = errors.New("partner: no such partner")

	// ErrPartnerExists is a create of an id already in the directory.
	ErrPartnerExists = errors.New("partner: a partner with this id already exists")
)

// MaxPaymentTerms is the longest credit term the directory will store, in days.
//
// Not a legal limit: it is the point past which the number is a typo rather than
// a term. Ten years is already beyond any trade credit, and a bound keeps the
// value inside the column the database stores it in.
const MaxPaymentTerms = 3650

// Type says which side of the books a partner appears on.
//
// Both is a single record, not two: a business that buys from and sells to a
// client is one balance to offset, and splitting it into a customer row and a
// supplier row would hide that.
type Type string

const (
	TypeCustomer Type = "customer"
	TypeSupplier Type = "supplier"
	TypeBoth     Type = "both"
)

// Valid reports whether the type is one of the three the directory knows.
func (t Type) Valid() bool {
	switch t {
	case TypeCustomer, TypeSupplier, TypeBoth:
		return true
	}

	return false
}

// IsCustomer reports whether the partner is on the sales side. A partner who is
// both answers yes here and to IsSupplier: one record, two roles.
func (t Type) IsCustomer() bool { return t == TypeCustomer || t == TypeBoth }

// IsSupplier reports whether the partner is on the purchase side.
func (t Type) IsSupplier() bool { return t == TypeSupplier || t == TypeBoth }

// Partner is a customer, a supplier, or both.
type Partner struct {
	// ID identifies the partner. Stable for the life of the books — usually a
	// UUID — because it is what a journal line refers to.
	ID string

	// Name is the business name (tên đối tác).
	Name string

	// Type says which side of the books the partner appears on.
	Type Type

	// TaxID is Mã số thuế. Optional — a small customer may have none — and
	// normalised, so the same business written four ways is one value and a
	// duplicate is detectable at all.
	TaxID string

	// PaymentTerms is the credit period in days. 0 is payment on delivery.
	PaymentTerms int

	// Active reports whether the partner may be used on new documents. A
	// partner that has been traded with is deactivated rather than deleted, so
	// the history that names it keeps its meaning.
	Active bool
}

// NewPartner builds a partner, trimming and normalising what was entered.
//
// Normalising here rather than at the UI means no other path into the directory
// can skip it — an import, a script, or a future screen all get the same
// treatment. Every repository write normalises again, so a struct literal
// assembled by hand cannot skip it either.
func NewPartner(id, name string, kind Type, taxID string, paymentTerms int) (Partner, error) {
	p := Partner{
		ID:           id,
		Name:         name,
		Type:         kind,
		TaxID:        taxID,
		PaymentTerms: paymentTerms,
		Active:       true,
	}.Normalized()

	if err := p.Validate(); err != nil {
		return Partner{}, err
	}

	return p, nil
}

// Validate reports whether the partner could be stored and used.
func (p Partner) Validate() error {
	if strings.TrimSpace(p.ID) == "" {
		return fmt.Errorf("%w: a partner needs an id", ErrInvalidPartner)
	}
	if strings.TrimSpace(p.Name) == "" {
		return fmt.Errorf("%w: a partner needs a name", ErrInvalidPartner)
	}
	if !p.Type.Valid() {
		return fmt.Errorf("%w: partner type %q is not one of customer, supplier, both", ErrInvalidPartner, p.Type)
	}
	if p.PaymentTerms < 0 {
		return fmt.Errorf("%w: payment terms of %d days are a negative credit period",
			ErrInvalidPartner, p.PaymentTerms)
	}
	if p.PaymentTerms > MaxPaymentTerms {
		return fmt.Errorf("%w: payment terms of %d days exceed the %d-day maximum",
			ErrInvalidPartner, p.PaymentTerms, MaxPaymentTerms)
	}

	return nil
}

// IsCustomer reports whether the partner is on the sales side.
func (p Partner) IsCustomer() bool { return p.Type.IsCustomer() }

// IsSupplier reports whether the partner is on the purchase side.
func (p Partner) IsSupplier() bool { return p.Type.IsSupplier() }

// NormalizeTaxID strips the separators people type into a tax id.
//
// The same business gets written "0123456789", "0123 456 789", "0123.456.789",
// and "0123456789-001" for its branch. Without collapsing those, one customer is
// onboarded several times and their balance splits across the duplicates — which
// stays invisible until somebody chases a receivable that is already half paid.
// It is also what makes a duplicate detectable at all: two strings that differ
// only in punctuation are the same tax id.
//
// Digits survive, and so does the hyphen before a branch number. Everything else
// goes. A string with no digits at all is empty, not punctuation.
func NormalizeTaxID(raw string) string {
	var kept strings.Builder

	for _, r := range raw {
		if (r >= '0' && r <= '9') || r == '-' {
			kept.WriteRune(r)
		}
	}

	out := strings.Trim(kept.String(), "-")
	if !strings.ContainsAny(out, "0123456789") {
		return ""
	}

	return out
}

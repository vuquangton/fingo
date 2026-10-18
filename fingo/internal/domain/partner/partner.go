// Package partner holds the businesses a client trades with.
//
// A partner is the dimension that makes a receivable answerable. "TK 131 is
// 500,000,000" is a number; "TK 131 from Công ty Bình Minh is 500,000,000" is a
// position somebody can act on. TT133 keeps Sổ chi tiết TK 131 by customer,
// which is why the dimension belongs on the journal line rather than beside it.
package partner

import (
	"errors"
	"fmt"
	"strings"
)

// Kind says which side of the books a partner appears on.
//
// Both is a single record, not two: a business that buys and sells to a client
// is one balance to offset, and splitting it would hide that.
type Kind string

const (
	KindCustomer Kind = "CUSTOMER"
	KindSupplier Kind = "SUPPLIER"
	KindBoth     Kind = "BOTH"
)

func (k Kind) Valid() bool {
	switch k {
	case KindCustomer, KindSupplier, KindBoth:
		return true
	}
	return false
}

// Partner is a customer, a supplier, or both.
type Partner struct {
	ID       string
	Code     string // Mã khách hàng / nhà cung cấp
	Name     string
	TaxCode  string // Mã số thuế; optional, and normalised
	Address  string
	Kind     Kind
	IsActive bool
}

// NewPartner builds a partner, trimming and normalising what was entered.
//
// Normalising here rather than at the UI means no other path into the books can
// skip it — an import, a script, or a future screen all get the same treatment.
func NewPartner(id, code, name, taxCode string, kind Kind) (*Partner, error) {
	p := &Partner{
		ID:       id,
		Code:     strings.TrimSpace(code),
		Name:     strings.TrimSpace(name),
		TaxCode:  NormalizeTaxCode(taxCode),
		Kind:     kind,
		IsActive: true,
	}

	if err := p.Validate(); err != nil {
		return nil, err
	}

	return p, nil
}

// Validate reports whether a partner could be found and addressed.
func (p Partner) Validate() error {
	if strings.TrimSpace(p.Code) == "" {
		return errors.New("a partner needs a code: it is how the partner is referred to")
	}
	if strings.TrimSpace(p.Name) == "" {
		return errors.New("a partner needs a name")
	}
	if !p.Kind.Valid() {
		return fmt.Errorf("partner kind %q is not one of the known kinds", p.Kind)
	}

	return nil
}

// NormalizeTaxCode strips the separators people type into a tax code.
//
// The same business gets written "0123456789", "0123 456 789", "0123.456.789",
// and "0123456789-001" for its branch. Without collapsing those, one customer is
// onboarded several times and their balance splits across the duplicates — which
// stays invisible until somebody chases a receivable that is already half paid.
//
// Digits survive, and so does the hyphen before a branch number. Everything else
// goes. A string with no digits at all is empty, not punctuation.
func NormalizeTaxCode(raw string) string {
	var digits strings.Builder

	for _, r := range raw {
		if (r >= '0' && r <= '9') || r == '-' {
			digits.WriteRune(r)
		}
	}

	out := strings.Trim(digits.String(), "-")
	if !strings.ContainsAny(out, "0123456789") {
		return ""
	}

	return out
}

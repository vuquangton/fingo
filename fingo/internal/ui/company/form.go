// Package company holds what a first run collects about the client.
//
// Two fields, and one of them is checked. The name is free text — the only thing
// done to it is trimming — while the mã số thuế has exactly two legal shapes and
// a wrong one is a filing the tax authority rejects after it has been submitted.
package company

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
)

// taxCode is the Vietnamese mã số thuế: ten digits, optionally followed by a
// three-digit branch suffix after a dash.
//
// Thirteen digits with no dash is refused rather than accepted. The canonical
// written form has the dash, and a system that quietly accepts both stores two
// spellings of one code and then cannot match a declaration to a registration.
var taxCode = regexp.MustCompile(`^[0-9]{10}(-[0-9]{3})?$`)

// Profile is what is stored once the form is accepted.
type Profile struct {
	Name    string
	TaxCode string
}

// Form is the first-run company form.
//
// Committed one field at a time, like the entry grid: a refusal belongs at the
// field the operator is standing in, not at the end of the form.
type Form struct {
	name    string
	taxCode string
}

// SetName commits the name field.
func (f *Form) SetName(v string) error {
	f.name = strings.TrimSpace(v)

	return nil
}

// SetTaxCode commits the mã số thuế field.
//
// An empty code is allowed: a newly registered company may not have its mã số
// thuế in hand on the day the books are opened, and refusing to let the operator
// start work until it arrives is refusing to let them work.
func (f *Form) SetTaxCode(v string) error {
	code := canonicalTaxCode(v)

	if code != "" && !taxCode.MatchString(code) {
		return fmt.Errorf(
			"%q is not a mã số thuế: it is 10 digits, or 10 digits and a 3-digit branch suffix",
			strings.TrimSpace(v))
	}

	f.taxCode = code

	return nil
}

// Name is the trimmed company name.
func (f *Form) Name() string { return f.name }

// TaxCode is the canonical mã số thuế, empty when none was given.
func (f *Form) TaxCode() string { return f.taxCode }

// Profile is what to store, once the form is acceptable.
func (f *Form) Profile() (Profile, error) {
	if f.name == "" {
		return Profile{}, errors.New("a company needs a name: it is what heads every document")
	}

	return Profile{Name: f.name, TaxCode: f.taxCode}, nil
}

// canonicalTaxCode strips the spaces people type around the dash, so that
// "0123456789 - 001" and "0123456789-001" are one code and not two.
func canonicalTaxCode(v string) string {
	return strings.ReplaceAll(strings.TrimSpace(v), " ", "")
}

package partner

import "fmt"

// WarningCode names a warning, so a caller can branch on it without matching
// prose that a later edit may reword.
type WarningCode string

// WarningDuplicateTaxID says another partner already carries this tax id.
const WarningDuplicateTaxID WarningCode = "duplicate_tax_id"

// Warning is something an operator should look at that is not a reason to refuse
// what they asked for.
//
// A duplicate tax id is the case this exists for. It is not always wrong: a
// branch files under its head office's code, and one legal entity may be kept
// under two trading names. Refusing the second row would force an operator to
// invent a false tax id to get their work done; accepting it silently would hide
// a genuine double-onboarding until somebody chased a receivable that was
// already half paid. So the row is written and the operator is told.
type Warning struct {
	// Code names the warning.
	Code WarningCode

	// Message is what the operator is shown.
	Message string

	// PartnerID is the row already holding the tax id, so the operator can go
	// and look at it.
	PartnerID string
}

// String renders the warning as the sentence an operator reads.
func (w Warning) String() string { return w.Message }

// CheckDuplicateTaxID returns a warning when another partner already carries
// this tax id, and nil when there is nothing to say.
//
// The comparison is on the normalised value, so "0123 456 789" and
// "0123.456.789" are the same tax id and one of them is not a second business.
// The candidate's own row is skipped, so re-saving a partner under its unchanged
// tax id does not warn about itself.
//
// A partner with no tax id never warns: nothing is being duplicated, and many
// partners may have none.
func CheckDuplicateTaxID(candidate Partner, others []Partner) *Warning {
	taxID := NormalizeTaxID(candidate.TaxID)
	if taxID == "" {
		return nil
	}

	for _, other := range others {
		if other.ID == candidate.ID {
			continue
		}
		if NormalizeTaxID(other.TaxID) != taxID {
			continue
		}

		return &Warning{
			Code:      WarningDuplicateTaxID,
			PartnerID: other.ID,
			Message: fmt.Sprintf(
				"tax id %s is already used by %s (%s); the partner was saved — check whether this is the same business",
				taxID, displayName(other), other.ID),
		}
	}

	return nil
}

// displayName names a partner in a message, falling back to the id when the row
// has no name to show.
func displayName(p Partner) string {
	if p.Name == "" {
		return p.ID
	}

	return p.Name
}

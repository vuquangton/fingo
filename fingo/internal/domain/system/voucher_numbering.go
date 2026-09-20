package system

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type ResetFrequency string

const (
	ResetFrequencyMonthly    ResetFrequency = "MONTHLY"
	ResetFrequencyYearly     ResetFrequency = "YEARLY"
	ResetFrequencyContinuous ResetFrequency = "CONTINUOUS"
)

// VoucherNumberingConfig defines automated continuous sequence numbering rules
type VoucherNumberingConfig struct {
	ID               string         `json:"id"`
	CompanyProfileID string         `json:"company_profile_id"`
	BranchID         *string        `json:"branch_id,omitempty"`
	VoucherType      string         `json:"voucher_type"`
	Prefix           string         `json:"prefix"`
	Pattern          string         `json:"pattern"`
	ResetFrequency   ResetFrequency `json:"reset_frequency"`
	CurrentSequence  int64          `json:"current_sequence"`
	LastResetDate    time.Time      `json:"last_reset_date"`
	UpdatedAt        time.Time      `json:"updated_at"`
}

var sequenceRegex = regexp.MustCompile(`\{(?:SEQUENCE|SEQ):(\d+)\}`)

// FormatVoucherNumber formats a voucher number according to an enterprise pattern template
// Supported tokens:
// {PREFIX}       -> prefix string (e.g. "PT")
// {YYYY}         -> 4-digit year (e.g. "2026")
// {YY}           -> 2-digit year (e.g. "26")
// {MM}           -> 2-digit month (e.g. "03")
// {DD}           -> 2-digit day (e.g. "20")
// {SEQUENCE:N} / {SEQ:N} -> 0-padded sequence of length N (e.g. {SEQUENCE:05} -> "00042")
func FormatVoucherNumber(prefix, pattern string, seq int64, t time.Time) string {
	if pattern == "" {
		pattern = "{PREFIX}-{YYYY}{MM}-{SEQUENCE:05}"
	}

	result := pattern
	result = strings.ReplaceAll(result, "{PREFIX}", prefix)
	result = strings.ReplaceAll(result, "{YYYY}", t.Format("2006"))
	result = strings.ReplaceAll(result, "{YY}", t.Format("06"))
	result = strings.ReplaceAll(result, "{MM}", t.Format("01"))
	result = strings.ReplaceAll(result, "{DD}", t.Format("02"))

	matches := sequenceRegex.FindStringSubmatch(result)
	if len(matches) == 2 {
		width, err := strconv.Atoi(matches[1])
		if err == nil && width > 0 {
			formatSpec := fmt.Sprintf("%%0%dd", width)
			formattedSeq := fmt.Sprintf(formatSpec, seq)
			result = strings.Replace(result, matches[0], formattedSeq, 1)
		}
	} else {
		// Fallback simple sequence
		result = strings.ReplaceAll(result, "{SEQUENCE}", strconv.FormatInt(seq, 10))
		result = strings.ReplaceAll(result, "{SEQ}", strconv.FormatInt(seq, 10))
	}

	return result
}

// NextNumber increments the sequence according to reset frequency and returns the formatted voucher number.
// Out-of-order backdated vouchers will not reset modern sequences.
func (c *VoucherNumberingConfig) NextNumber(voucherDate time.Time) string {
	shouldReset := false
	switch c.ResetFrequency {
	case ResetFrequencyMonthly:
		if c.LastResetDate.IsZero() {
			shouldReset = true
		} else if voucherDate.After(c.LastResetDate) && (voucherDate.Year() > c.LastResetDate.Year() || voucherDate.Month() > c.LastResetDate.Month()) {
			shouldReset = true
		}
	case ResetFrequencyYearly:
		if c.LastResetDate.IsZero() {
			shouldReset = true
		} else if voucherDate.After(c.LastResetDate) && voucherDate.Year() > c.LastResetDate.Year() {
			shouldReset = true
		}
	case ResetFrequencyContinuous:
		shouldReset = false
	}

	if shouldReset {
		c.CurrentSequence = 0
	}

	c.CurrentSequence++
	if c.LastResetDate.IsZero() || voucherDate.After(c.LastResetDate) {
		c.LastResetDate = voucherDate
	}
	c.UpdatedAt = time.Now()

	return FormatVoucherNumber(c.Prefix, c.Pattern, c.CurrentSequence, voucherDate)
}

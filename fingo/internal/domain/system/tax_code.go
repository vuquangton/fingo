package system

import (
	"errors"
	"strings"
	"unicode"
)

var (
	ErrTaxCodeEmpty     = errors.New("tax code cannot be empty")
	ErrTaxCodeLength    = errors.New("tax code must be exactly 10 digits or 13 digits (format: XXXXXXXXXX-YYY)")
	ErrTaxCodeMalformed = errors.New("tax code contains invalid characters")
	ErrTaxCodeChecksum  = errors.New("tax code failed modulo-11 checksum validation per Decree 01/2021/ND-CP")
)

// weights for the first 9 digits per Circular 105/2020/TT-BTC & Decree 01/2021/ND-CP
var taxCodeWeights = [9]int{31, 29, 23, 19, 17, 13, 7, 5, 3}

// ValidateTaxCode validates Vietnamese 10-digit enterprise and 13-digit branch tax codes
func ValidateTaxCode(rawMST string) error {
	mst := strings.TrimSpace(rawMST)
	if mst == "" {
		return ErrTaxCodeEmpty
	}

	var base10 string
	switch len(mst) {
	case 10:
		base10 = mst
	case 14:
		if mst[10] != '-' {
			return ErrTaxCodeLength
		}
		base10 = mst[:10]
		branchSuffix := mst[11:]
		for _, r := range branchSuffix {
			if !unicode.IsDigit(r) {
				return ErrTaxCodeMalformed
			}
		}
	default:
		return ErrTaxCodeLength
	}

	for _, r := range base10 {
		if !unicode.IsDigit(r) {
			return ErrTaxCodeMalformed
		}
	}

	sum := 0
	for i := 0; i < 9; i++ {
		digit := int(base10[i] - '0')
		sum += digit * taxCodeWeights[i]
	}

	remainder := sum % 11
	checkDigit := 10 - remainder
	if checkDigit == 10 {
		checkDigit = 0
	}

	expectedCheckDigit := int(base10[9] - '0')
	if checkDigit != expectedCheckDigit {
		return ErrTaxCodeChecksum
	}

	return nil
}

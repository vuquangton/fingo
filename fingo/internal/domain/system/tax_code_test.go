package system_test

import (
	"testing"

	"fingo/internal/domain/system"
)

func TestValidateTaxCode(t *testing.T) {
	tests := []struct {
		name    string
		taxCode string
		wantErr bool
	}{
		// Valid 10-digit enterprise MSTs
		{"Valid 10-digit MISA", "0101243150", false},
		{"Valid 10-digit FPT", "0101248141", false},
		{"Valid 10-digit with whitespace", " 0101243150 ", false},

		// Valid 13-digit branch MSTs
		{"Valid 13-digit branch", "0101243150-001", false},
		{"Valid 13-digit branch max", "0101248141-999", false},

		// Invalid lengths
		{"Empty string", "", true},
		{"Too short", "01012431", true},
		{"Too long", "010124315012", true},
		{"13 digits without hyphen", "0101243150001", true},

		// Invalid characters
		{"Contains letters in 10-digit", "010124315A", true},
		{"Contains special characters", "010124315@", true},
		{"Branch suffix with letters", "0101243150-00A", true},

		// Invalid Checksum (Modulo-11 violation)
		{"Checksum mismatch", "0101243159", true},
		{"Branch with invalid base checksum", "0101243159-001", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := system.ValidateTaxCode(tt.taxCode)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateTaxCode(%q) error = %v, wantErr %v", tt.taxCode, err, tt.wantErr)
			}
		})
	}
}

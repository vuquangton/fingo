package system_test

import (
	"testing"
	"time"

	"fingo/internal/domain/system"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateNonCashPaymentThreshold(t *testing.T) {
	t.Parallel()

	defaultThreshold := decimal.NewFromInt(5_000_000) // Luật Thuế GTGT 2024 (from 01/07/2025)

	tests := []struct {
		name        string
		amount      decimal.Decimal
		isCash      bool
		threshold   decimal.Decimal
		wantWarning bool
		wantErr     bool
	}{
		{
			name:        "Cash payment below 5M threshold (Compliant)",
			amount:      decimal.NewFromInt(4_999_999),
			isCash:      true,
			threshold:   defaultThreshold,
			wantWarning: false,
			wantErr:     false,
		},
		{
			name:        "Cash payment exactly at 5M threshold (Statutory Warning)",
			amount:      decimal.NewFromInt(5_000_000),
			isCash:      true,
			threshold:   defaultThreshold,
			wantWarning: true,
			wantErr:     false,
		},
		{
			name:        "Cash payment above 5M threshold (Statutory Warning)",
			amount:      decimal.NewFromInt(12_500_000),
			isCash:      true,
			threshold:   defaultThreshold,
			wantWarning: true,
			wantErr:     false,
		},
		{
			name:        "Bank transfer above 5M threshold (Compliant, No Warning)",
			amount:      decimal.NewFromInt(50_000_000),
			isCash:      false,
			threshold:   defaultThreshold,
			wantWarning: false,
			wantErr:     false,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			isWarning, err := system.ValidateNonCashPaymentThreshold(tt.amount, tt.isCash, tt.threshold)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantWarning, isWarning)
		})
	}
}

func TestCheckNegativeInventory(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		currentStock decimal.Decimal
		dispatchQty  decimal.Decimal
		policy       string
		wantErr      bool
		errExpected  error
	}{
		{
			name:         "Stock sufficient under DISALLOW policy",
			currentStock: decimal.NewFromInt(100),
			dispatchQty:  decimal.NewFromInt(40),
			policy:       system.PolicyDisallow,
			wantErr:      false,
		},
		{
			name:         "Stock insufficient under DISALLOW policy -> MUST BLOCK",
			currentStock: decimal.NewFromInt(10),
			dispatchQty:  decimal.NewFromInt(15),
			policy:       system.PolicyDisallow,
			wantErr:      true,
			errExpected:  system.ErrNegativeStockBlocked,
		},
		{
			name:         "Stock insufficient under ALLOW policy -> PERMITTED",
			currentStock: decimal.NewFromInt(10),
			dispatchQty:  decimal.NewFromInt(15),
			policy:       system.PolicyAllow,
			wantErr:      false,
		},
		{
			name:         "Stock insufficient under WARN policy -> PERMITTED with warning state",
			currentStock: decimal.NewFromInt(0),
			dispatchQty:  decimal.NewFromInt(5),
			policy:       system.PolicyWarn,
			wantErr:      false,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := system.CheckNegativeInventory(tt.currentStock, tt.dispatchQty, tt.policy)
			if tt.wantErr {
				require.Error(t, err)
				if tt.errExpected != nil {
					require.ErrorIs(t, err, tt.errExpected)
				}
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestCheckNegativeCash(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		currentBalance decimal.Decimal
		paymentAmount  decimal.Decimal
		policy         string
		wantErr        bool
		errExpected    error
	}{
		{
			name:           "Cash vault sufficient",
			currentBalance: decimal.NewFromInt(10_000_000),
			paymentAmount:  decimal.NewFromInt(3_000_000),
			policy:         system.PolicyDisallow,
			wantErr:        false,
		},
		{
			name:           "Cash vault overdraft under DISALLOW policy -> MUST BLOCK",
			currentBalance: decimal.NewFromInt(2_000_000),
			paymentAmount:  decimal.NewFromInt(2_500_000),
			policy:         system.PolicyDisallow,
			wantErr:        true,
			errExpected:    system.ErrNegativeCashBlocked,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := system.CheckNegativeCash(tt.currentBalance, tt.paymentAmount, tt.policy)
			if tt.wantErr {
				require.Error(t, err)
				if tt.errExpected != nil {
					require.ErrorIs(t, err, tt.errExpected)
				}
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestFormatVoucherNumber(t *testing.T) {
	t.Parallel()

	targetDate := time.Date(2026, 3, 20, 14, 30, 0, 0, time.UTC)

	tests := []struct {
		name     string
		prefix   string
		pattern  string
		seq      int64
		date     time.Time
		expected string
	}{
		{
			name:     "Standard monthly voucher pattern",
			prefix:   "PT",
			pattern:  "{PREFIX}-{YYYY}{MM}-{SEQUENCE:05}",
			seq:      42,
			date:     targetDate,
			expected: "PT-202603-00042",
		},
		{
			name:     "Annual invoice pattern with 6-digit sequence",
			prefix:   "HDBR",
			pattern:  "{PREFIX}-{YY}-{SEQUENCE:06}",
			seq:      142,
			date:     targetDate,
			expected: "HDBR-26-000142",
		},
		{
			name:     "Continuous numbering without date tokens",
			prefix:   "PKT",
			pattern:  "{PREFIX}-{SEQUENCE:04}",
			seq:      7,
			date:     targetDate,
			expected: "PKT-0007",
		},
		{
			name:     "Short SEQ token pattern from wireframe",
			prefix:   "PC",
			pattern:  "{PREFIX}-{YYYY}{MM}-{SEQ:05}",
			seq:      24,
			date:     targetDate,
			expected: "PC-202603-00024",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			res := system.FormatVoucherNumber(tt.prefix, tt.pattern, tt.seq, tt.date)
			assert.Equal(t, tt.expected, res)
		})
	}
}

func TestVoucherNumberingConfig_NextNumber(t *testing.T) {
	t.Parallel()

	t.Run("Monthly reset frequency advances sequence and resets on new month", func(t *testing.T) {
		t.Parallel()
		cfg := &system.VoucherNumberingConfig{
			ID:              "cfg-1",
			VoucherType:     "CASH_RECEIPT",
			Prefix:          "PT",
			Pattern:         "{PREFIX}-{YYYY}{MM}-{SEQUENCE:04}",
			ResetFrequency:  system.ResetFrequencyMonthly,
			CurrentSequence: 0,
		}

		d1 := time.Date(2026, 3, 15, 10, 0, 0, 0, time.UTC)
		num1 := cfg.NextNumber(d1)
		assert.Equal(t, "PT-202603-0001", num1)
		assert.Equal(t, int64(1), cfg.CurrentSequence)

		num2 := cfg.NextNumber(d1)
		assert.Equal(t, "PT-202603-0002", num2)
		assert.Equal(t, int64(2), cfg.CurrentSequence)

		// Next month -> resets to 1
		d2 := time.Date(2026, 4, 1, 9, 0, 0, 0, time.UTC)
		num3 := cfg.NextNumber(d2)
		assert.Equal(t, "PT-202604-0001", num3)
		assert.Equal(t, int64(1), cfg.CurrentSequence)
	})

	t.Run("Yearly reset frequency preserves sequence in same year and resets on new year", func(t *testing.T) {
		t.Parallel()
		cfg := &system.VoucherNumberingConfig{
			ID:              "cfg-2",
			VoucherType:     "SALES_INVOICE",
			Prefix:          "HD",
			Pattern:         "{PREFIX}-{YYYY}-{SEQUENCE:05}",
			ResetFrequency:  system.ResetFrequencyYearly,
			CurrentSequence: 50,
			LastResetDate:   time.Date(2026, 1, 10, 0, 0, 0, 0, time.UTC),
		}

		d1 := time.Date(2026, 5, 20, 0, 0, 0, 0, time.UTC)
		num1 := cfg.NextNumber(d1)
		assert.Equal(t, "HD-2026-00051", num1)
		assert.Equal(t, int64(51), cfg.CurrentSequence)

		// New year -> resets
		d2 := time.Date(2027, 1, 2, 0, 0, 0, 0, time.UTC)
		num2 := cfg.NextNumber(d2)
		assert.Equal(t, "HD-2027-00001", num2)
		assert.Equal(t, int64(1), cfg.CurrentSequence)
	})

	t.Run("Continuous reset frequency never resets", func(t *testing.T) {
		t.Parallel()
		cfg := &system.VoucherNumberingConfig{
			ID:              "cfg-3",
			VoucherType:     "JOURNAL_VOUCHER",
			Prefix:          "PKT",
			Pattern:         "{PREFIX}-{SEQUENCE:04}",
			ResetFrequency:  system.ResetFrequencyContinuous,
			CurrentSequence: 999,
			LastResetDate:   time.Date(2025, 12, 31, 0, 0, 0, 0, time.UTC),
		}

		d1 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
		num1 := cfg.NextNumber(d1)
		assert.Equal(t, "PKT-1000", num1)
		assert.Equal(t, int64(1000), cfg.CurrentSequence)
	})
}

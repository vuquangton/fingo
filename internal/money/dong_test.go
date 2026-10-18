package money_test

import (
	"encoding/json"
	"testing"

	"fingo/internal/money"
)

func TestDongArithmetic(t *testing.T) {
	tests := []struct {
		name string
		got  money.Dong
		want money.Dong
	}{
		{"add", money.FromInt(1000).Add(money.FromInt(250)), money.FromInt(1250)},
		{"sub", money.FromInt(1000).Sub(money.FromInt(250)), money.FromInt(750)},
		{"neg", money.FromInt(1000).Neg(), money.FromInt(-1000)},
		{"abs negative", money.FromInt(-1000).Abs(), money.FromInt(1000)},
		{"abs positive", money.FromInt(1000).Abs(), money.FromInt(1000)},
		{"mul", money.FromInt(120).Mul(money.FromInt(3)), money.FromInt(360)},
		{"div exact", money.FromInt(1200).Div(money.FromInt(3)), money.FromInt(400)},
		{"div truncates toward zero", money.FromInt(1000).Div(money.FromInt(3)), money.FromInt(333)},
		{"div truncates negative toward zero", money.FromInt(-1000).Div(money.FromInt(3)), money.FromInt(-333)},
		{"round is identity", money.FromInt(1234).Round(0), money.FromInt(1234)},
		{"truncate is identity", money.FromInt(1234).Truncate(0), money.FromInt(1234)},
		{"zero value", money.Dong(0), money.Zero},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.got != tt.want {
				t.Fatalf("got %d, want %d", tt.got, tt.want)
			}
		})
	}
}

func TestDongComparisons(t *testing.T) {
	tests := []struct {
		name    string
		left    money.Dong
		right   money.Dong
		wantCmp int
	}{
		{"less", money.FromInt(-5), money.FromInt(3), -1},
		{"equal", money.FromInt(3), money.FromInt(3), 0},
		{"greater", money.FromInt(5), money.FromInt(3), 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.left.Cmp(tt.right); got != tt.wantCmp {
				t.Fatalf("Cmp = %d, want %d", got, tt.wantCmp)
			}
			if got := tt.left.Sign(); got != tt.wantCmp && !(tt.wantCmp == 0 && got == 1) {
				t.Fatalf("Sign = %d, want the sign of the left amount", got)
			}
			if got := tt.left.Equal(tt.right); got != (tt.wantCmp == 0) {
				t.Fatalf("Equal = %v", got)
			}
		})
	}
}

func TestDongDivisionByZeroPanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("division by zero did not panic")
		}
	}()

	_ = money.FromInt(1).Div(money.Zero)
}

func TestDongJSONIsAString(t *testing.T) {
	// An amount crosses HTTP as a string: a JSON number is a double in the
	// browser, and above 2^53 that rounds the figure.
	const big = int64(9_007_199_254_740_993) // 2^53 + 1

	encoded, err := json.Marshal(money.FromInt(big))
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if string(encoded) != `"9007199254740993"` {
		t.Fatalf("encoded = %s, want a quoted integer", encoded)
	}

	var decoded money.Dong
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatalf("unmarshal string: %v", err)
	}
	if decoded != money.FromInt(big) {
		t.Fatalf("round-trip = %d, want %d", decoded, big)
	}

	if err := json.Unmarshal([]byte(`1234`), &decoded); err != nil {
		t.Fatalf("unmarshal number: %v", err)
	}
	if decoded != money.FromInt(1234) {
		t.Fatalf("numeric decode = %d, want 1234", decoded)
	}
}

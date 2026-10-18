package textfold_test

import (
	"testing"

	"fingo/internal/ui/textfold"
)

// TestFold_RemovesTheAccentsPeopleDoNotType — slice P1.1.
//
// An accountant looking for a customer types "no phai tra", not "Nợ phải trả".
// A search box that requires the diacritics is a search box that does not work,
// and the failure is quiet: the list simply comes back empty and the operator
// concludes the customer is not there.
func TestFold_RemovesTheAccentsPeopleDoNotType(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"a plain word is unchanged", "Doanh thu", "doanh thu"},
		{"a base vowel with a tone", "Thuế", "thue"},
		{"a vowel with a shape and a tone", "Nợ phải trả", "no phai tra"},
		{"a breve", "Tài sản ngắn hạn", "tai san ngan han"},
		{"a circumflex", "Chi phí quản lý", "chi phi quan ly"},
		{"a horn", "Tài khoản phải trả", "tai khoan phai tra"},
		{"already folded input is unchanged", "tien mat", "tien mat"},
		{"digits and punctuation survive", "TK 131 - Phải thu", "tk 131 - phai thu"},
		{"an empty string", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := textfold.Fold(tt.in); got != tt.want {
				t.Errorf("Fold(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

// TestFold_HandlesDWithAStroke — and this is the case that makes the whole
// function worth testing rather than assuming.
//
// Vietnamese "đ" (U+0111) is a *letter*, not a d with a combining mark. It does
// not decompose under NFD, so a fold built on Unicode decomposition alone leaves
// it exactly where it was, and "Đơn vị tính" becomes unfindable by anyone typing
// "don vi tinh" — which is everyone.
//
// It has to be mapped by hand, and nothing about the rest of the alphabet
// suggests that.
func TestFold_HandlesDWithAStroke(t *testing.T) {
	tests := []struct{ in, want string }{
		{"Đơn vị tính", "don vi tinh"},
		{"đ", "d"},
		{"Đ", "d"},
		{"Đã khóa sổ", "da khoa so"},
	}

	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			if got := textfold.Fold(tt.in); got != tt.want {
				t.Errorf("Fold(%q) = %q, want %q — đ does not decompose under NFD", tt.in, got, tt.want)
			}
		})
	}
}

// TestFold_CollapsesWhitespace — a search is words, not spacing. An operator who
// pastes a name out of an email brings its line breaks with it.
func TestFold_CollapsesWhitespace(t *testing.T) {
	for _, in := range []string{
		"  Nợ   phải\t trả  ",
		"Nợ phải trả",
		"nợ\nphải trả",
	} {
		if got := textfold.Fold(in); got != "no phai tra" {
			t.Errorf("Fold(%q) = %q, want %q", in, got, "no phai tra")
		}
	}
}

// TestFold_IsIdempotent — folding a folded string must not change it. A search
// box folds the query as the operator types, so folding twice is the normal case
// rather than the exception.
func TestFold_IsIdempotent(t *testing.T) {
	for _, in := range []string{"Nợ phải trả", "Đơn vị tính", "TK 131", ""} {
		once := textfold.Fold(in)
		if twice := textfold.Fold(once); twice != once {
			t.Errorf("Fold(Fold(%q)) = %q, want %q", in, twice, once)
		}
	}
}

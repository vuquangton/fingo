// Package textfold folds Vietnamese text into a form that can be searched
// without typing the diacritics.
//
// It exists because of how the language is actually typed. "Nợ phải trả" is
// written with five diacritics and searched with none: an accountant looking for
// a supplier types "no phai tra". A search box that compares raw runes finds
// nothing, and finds it quietly — the list comes back empty and the operator
// concludes the record is missing.
//
// The fold is applied to both sides of every comparison, so it is idempotent by
// requirement rather than by accident.
package textfold

import (
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

// dWithStroke is Vietnamese "đ" and "Đ" (U+0111, U+0110).
//
// It is a letter of the alphabet, not a "d" carrying a combining mark, so it
// does not decompose under NFD and it survives every decomposition-based fold
// untouched. Left alone, "Đơn vị tính" becomes impossible to find by typing
// "don vi tinh" — which is how everyone types it. It has to be mapped by hand,
// and nothing about the rest of the alphabet suggests that.
const (
	dStrokeLower = '\u0111'
	dStrokeUpper = '\u0110'
)

// Fold returns the searchable form of s: lowercased, without diacritics, with
// runs of whitespace collapsed to single spaces and the ends trimmed.
//
// Call it on both the text and the query. It is safe to call on its own output.
func Fold(s string) string {
	// NFD splits "ế" into "e" + circumflex + acute, which is what makes it
	// possible to drop the marks rather than enumerate every accented rune.
	decomposed := norm.NFD.String(s)

	var b strings.Builder
	b.Grow(len(decomposed))

	// space tracks whether a separator is pending, so that runs collapse and the
	// result is trimmed without a second pass.
	space := false

	for _, r := range decomposed {
		switch {
		case r == dStrokeLower, r == dStrokeUpper:
			// Handled here, before the mark test can see it and keep it.
			if space && b.Len() > 0 {
				b.WriteRune(' ')
			}
			space = false
			b.WriteRune('d')

			continue

		case unicode.Is(unicode.Mn, r):
			// A combining mark. This is the diacritic; drop it.
			continue
		}

		if unicode.IsSpace(r) {
			space = true

			continue
		}

		if space && b.Len() > 0 {
			b.WriteRune(' ')
		}
		space = false

		b.WriteRune(unicode.ToLower(r))
	}

	return b.String()
}

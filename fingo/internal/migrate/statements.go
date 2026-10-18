package migrate

import "strings"

// splitStatements splits a SQL script into the statements it contains.
//
// database/sql sends one statement per Exec unless multiStatements is switched
// on, and switching it on widens what every connection in the process will
// accept. A lexer is smaller than that: it walks the script once and treats a
// semicolon as a separator only where it actually is one — outside a string, an
// identifier and a comment.
//
// The statements returned are slices of the original text, comments and all.
// Nothing is rewritten, so what runs is what the file says.
//
// Deliberately not supported: the mysql client's DELIMITER directive. It exists
// to let a client send a body containing semicolons, and nothing here needs to —
// stored programs are installed separately, which is how this repository already
// treats its triggers (see db/triggers).
func splitStatements(script string) []string {
	var (
		out     []string
		pending = -1 // first byte since the last statement ended, comments included
		content = -1 // first byte that is actually part of the statement
	)

	start := func(i int) {
		if pending < 0 {
			pending = i
		}
	}

	end := func(at int) {
		if content >= 0 {
			if s := strings.TrimSpace(script[pending:at]); s != "" {
				out = append(out, s)
			}
		}

		pending, content = -1, -1
	}

	for i := 0; i < len(script); {
		switch c := script[i]; {
		case c == '\'' || c == '"' || c == '`':
			start(i)
			if content < 0 {
				content = i
			}
			i = skipQuoted(script, i, c)

		case c == '-' && i+1 < len(script) && script[i+1] == '-' && lineCommentEndsAt(script, i+2):
			start(i)
			i = skipToLineEnd(script, i)

		case c == '#':
			start(i)
			i = skipToLineEnd(script, i)

		case c == '/' && i+1 < len(script) && script[i+1] == '*':
			start(i)
			i = skipBlockComment(script, i)

		case c == ';':
			end(i)
			i++

		case c == ' ' || c == '\t' || c == '\n' || c == '\r' || c == '\f' || c == '\v':
			i++

		default:
			start(i)
			if content < 0 {
				content = i
			}
			i++
		}
	}

	end(len(script))

	return out
}

// lineCommentEndsAt reports whether "--" at position i begins a comment.
//
// MySQL requires whitespace (or the end of input) after the two hyphens. Without
// that rule "a--b" would be a comment and the expression would silently lose its
// tail, which is exactly the kind of quiet wrong answer a splitter must not
// produce.
func lineCommentEndsAt(s string, i int) bool {
	if i >= len(s) {
		return true
	}

	switch s[i] {
	case ' ', '\t', '\n', '\r', '\f', '\v':
		return true
	}

	return false
}

// skipQuoted returns the index just past a quoted run that opens at i.
//
// Both escapes MySQL recognises are handled: a backslash escape, and the quote
// doubled to mean itself. Without the second, 'It”s here' would end at the
// fourth quote and everything after it would be misread.
func skipQuoted(s string, i int, quote byte) int {
	for i++; i < len(s); {
		switch s[i] {
		case '\\':
			i += 2
		case quote:
			if i+1 < len(s) && s[i+1] == quote {
				i += 2
				continue
			}

			return i + 1
		default:
			i++
		}
	}

	return len(s)
}

// skipToLineEnd returns the index of the first byte of the next line.
func skipToLineEnd(s string, i int) int {
	if n := strings.IndexByte(s[i:], '\n'); n >= 0 {
		return i + n + 1
	}

	return len(s)
}

// skipBlockComment returns the index just past a /* ... */ comment.
//
// An unterminated comment runs to the end of the script rather than being
// treated as an error here. The database is the authority on whether the SQL is
// valid, and a splitter that guesses is a splitter that hides the real message.
func skipBlockComment(s string, i int) int {
	if n := strings.Index(s[i+2:], "*/"); n >= 0 {
		return i + 2 + n + 2
	}

	return len(s)
}

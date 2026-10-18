package statutory

import (
	"bytes"
	"fmt"
	"strings"
	"time"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

// PDF renders the filing as the document an archive keeps.
//
// # Why the text is ASCII
//
// The archive copy is written with the standard PDF fonts — Helvetica, which a
// reader already has — and those carry no Vietnamese. The filing copy is the
// JSON, which keeps every name as it is written; this one prints the same lines
// transliterated and says so in its own header, because an archive whose text
// renders as boxes is not an archive.
func (o Output) PDF() ([]byte, error) {
	return o.render(a4)
}

// layout is what a page has room for.
type layout struct {
	width, height float64
	margin        float64
	leading       float64
	bodySize      float64
}

// a4 is the page an archive keeps: A4 in points, with room for a header.
var a4 = layout{width: 595, height: 842, margin: 45, leading: 13, bodySize: 8.5}

// linesPerPage is how many body lines fit under one header.
func (l layout) linesPerPage() int {
	room := l.height - 2*l.margin - 4*l.leading

	fit := int(room / l.leading)
	if fit < 1 {
		return 1
	}

	return fit
}

// render writes the output as a PDF.
//
// The catalogue and the page tree are reserved first, so that the pages written
// in between can name the tree they belong to before its body is known.
func (o Output) render(page layout) ([]byte, error) {
	if err := o.Validate(); err != nil {
		return nil, err
	}

	doc := &pdf{}
	catalog := doc.reserve()
	pagesObject := doc.reserve()
	font := doc.object("<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica /Encoding /WinAnsiEncoding >>")

	pages := paginate(o.pdfLines(page), page.linesPerPage())
	kids := make([]string, 0, len(pages))

	for n, body := range pages {
		content := doc.stream(pageContent(body, page, n+1, len(pages)))
		pageObject := doc.object(fmt.Sprintf(
			"<< /Type /Page /Parent %d 0 R /MediaBox [0 0 %.0f %.0f] /Resources << /Font << /F1 %d 0 R >> >> /Contents %d 0 R >>",
			pagesObject, page.width, page.height, font.number, content.number))

		kids = append(kids, fmt.Sprintf("%d 0 R", pageObject.number))
	}

	doc.fill(pagesObject, fmt.Sprintf("<< /Type /Pages /Kids [%s] /Count %d >>",
		strings.Join(kids, " "), len(pages)))
	doc.fill(catalog, fmt.Sprintf("<< /Type /Catalog /Pages %d 0 R >>", pagesObject))

	return doc.bytes()
}

// columns is how many characters of body text fit across the page.
//
// Helvetica's average glyph is about half its point size, and the measurement is
// deliberately pessimistic: a line that just fits is a line whose warning gets
// clipped, and the warnings are what the archive exists to keep.
func (l layout) columns() int {
	if l.bodySize <= 0 {
		return 80
	}

	usable := l.width - 2*l.margin
	if usable <= 0 {
		return 80
	}

	return int(usable / (0.5 * l.bodySize))
}

// pdfLines is the document as lines of text, one string per printed line, each
// already folded to what the page holds.
func (o Output) pdfLines(page layout) []string {
	budget := page.columns()
	out := make([]string, 0, 64)

	emit := func(line string) {
		out = append(out, wrapLine(line, budget)...)
	}

	emit("FinGo — statutory filing")
	emit(fmt.Sprintf("Period %s (%s to %s)   Regime %s   Generated %s UTC",
		o.Period.ID, o.Period.StartDate, o.Period.EndDate, o.Period.Regime,
		o.GeneratedAt.Format(time.RFC3339)))
	emit("Text is transliterated: the archive copy uses standard PDF fonts, which carry no Vietnamese. The JSON filing copy keeps the names as written.")
	emit("")

	// A row is laid out as its label and its amount on one line, with the amount
	// against the right edge; a long label is folded by emit, so the amount
	// moves to the next line rather than off the page.
	row := func(indicator, label, amount string) string {
		if indicator == "" {
			indicator = "—"
		}

		line := fmt.Sprintf("    %-6s %s", indicator, label)

		if pad := budget - len([]rune(line)) - len([]rune(amount)); pad > 0 {
			line += strings.Repeat(" ", pad)
		}

		return line + amount
	}

	for _, statement := range o.Statements {
		emit(fmt.Sprintf("%s  %s", statement.Code, statement.Name))
		emit(fmt.Sprintf("    Authority: %s [%s]", statement.Authority, statement.Confidence))

		for _, line := range statement.Lines {
			emit(row(line.Indicator, line.Label, line.Amount.String()))
			emit(fmt.Sprintf("           %s [%s]", line.Reference, line.Confidence))
		}

		for _, warning := range statement.Warnings {
			emit(fmt.Sprintf("    LEFT OUT: %s — %s", warning.Item, warning.Reason))
		}

		emit("")
	}

	return out
}

// wrapLine folds a line that would run past the page, keeping the indent of the
// line it came from so that a folded row still reads as a row.
//
// A word longer than the page is cut rather than allowed to run: printing half a
// word is bad, printing a line that a reader cannot see the end of is worse.
func wrapLine(line string, budget int) []string {
	// A page too narrow to hold a word is a layout fault, not a line to cut into
	// pieces nobody could read: below this the line is left as it is.
	if budget < 8 || len([]rune(line)) <= budget {
		return []string{line}
	}

	indent := line[:len(line)-len(strings.TrimLeft(line, " "))]
	out := make([]string, 0, 2)
	rest := line

	for len([]rune(rest)) > budget {
		cut := strings.LastIndex(runeSlice(rest, budget), " ")
		if cut < len(indent)+1 {
			cut = len(runeSlice(rest, budget))
		}

		out = append(out, rest[:cut])
		rest = indent + strings.TrimLeft(rest[cut:], " ")
	}

	return append(out, rest)
}

// runeSlice returns the first n runes of a string, so that a fold never lands
// inside a character.
func runeSlice(s string, n int) string {
	held := []rune(s)
	if len(held) <= n {
		return s
	}

	return string(held[:n])
}

// paginate splits lines into pages, repeating nothing: the header is written by
// the renderer on every page, so a page holds body lines only.
func paginate(lines []string, perPage int) [][]string {
	if perPage < 1 {
		perPage = 1
	}

	pages := make([][]string, 0, 1+len(lines)/perPage)

	for len(lines) > 0 {
		take := perPage
		if take > len(lines) {
			take = len(lines)
		}

		pages = append(pages, lines[:take])
		lines = lines[take:]
	}

	if len(pages) == 0 {
		pages = append(pages, nil)
	}

	return pages
}

// pageContent is one page's content stream: a small header, then the lines.
func pageContent(lines []string, page layout, number, total int) string {
	var b strings.Builder
	y := page.height - page.margin

	fmt.Fprintf(&b, "BT /F1 %.1f Tf %.1f TL %.1f %.1f Td ", page.bodySize, page.leading, page.margin, y)

	for _, line := range lines {
		fmt.Fprintf(&b, "(%s) Tj T* ", escapePDF(ascii(line)))
	}

	fmt.Fprintf(&b, "ET BT /F1 %.1f Tf %.1f %.1f Td (Page %d of %d) Tj ET",
		page.bodySize, page.margin, page.margin/2, number, total)

	return b.String()
}

// escapePDF escapes the three characters a PDF string literal cannot carry
// raw.
func escapePDF(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, "(", `\(`)
	s = strings.ReplaceAll(s, ")", `\)`)

	// A line is a line: a control character in a text string would end the
	// stream's readability for no gain.
	return strings.Map(func(r rune) rune {
		if r < 0x20 || r == 0x7f {
			return ' '
		}

		return r
	}, s)
}

// ascii renders a string in the characters the standard PDF fonts carry.
//
// The Vietnamese letters are decomposed and their tone marks dropped, so "Thuế"
// becomes "Thue"; the two letters that are not decompositions — đ and Đ — are
// mapped by hand. What is left is the base Latin alphabet, which every reader
// has.
func ascii(s string) string {
	decomposed := norm.NFD.String(s)

	var b strings.Builder

	for _, r := range decomposed {
		switch {
		case unicode.Is(unicode.Mn, r):
			// A tone or a horn, dropped.
		case r == 'đ':
			b.WriteRune('d')
		case r == 'Đ':
			b.WriteRune('D')
		case r > 0x7f:
			b.WriteRune(basePunctuation(r))
		default:
			b.WriteRune(r)
		}
	}

	return b.String()
}

// basePunctuation maps the typographic characters this package writes to the
// ASCII ones a standard font certainly has.
func basePunctuation(r rune) rune {
	switch r {
	case '–', '—', '−':
		return '-'
	case '‘', '’':
		return '\''
	case '“', '”':
		return '"'
	case '…':
		return '.'
	case '≤':
		return '<'
	case '≥':
		return '>'
	case '×':
		return 'x'
	}

	return '?'
}

// pdf writes the bytes of a PDF, keeping the offsets its cross-reference table
// needs.
type pdf struct {
	buf bytes.Buffer

	// bodies holds each object's body by object number, so an object can be
	// reserved before it is known — the pages name the page tree that names
	// them.
	bodies  []string
	offsets []int
}

// reserve takes the next object number without writing it.
func (p *pdf) reserve() int {
	p.bodies = append(p.bodies, "")
	p.offsets = append(p.offsets, 0)

	return len(p.bodies)
}

// fill gives a reserved object its body.
func (p *pdf) fill(number int, body string) {
	p.bodies[number-1] = body
}

// object writes an object that is already known.
func (p *pdf) object(body string) pdfRef {
	number := p.reserve()
	p.fill(number, body)

	return pdfRef{number}
}

// stream writes an object holding a stream.
func (p *pdf) stream(content string) pdfRef {
	return p.object(fmt.Sprintf("<< /Length %d >>\nstream\n%s\nendstream", len(content), content))
}

// ref is one object's number, rendered as a PDF reference.
type pdfRef struct{ number int }

// bytes assembles the document: header, objects, cross-reference, trailer.
func (p *pdf) bytes() ([]byte, error) {
	p.buf.Reset()

	// The %âãÏÓ marker tells a transfer tool this is binary, which is what the
	// spec asks for and what stops a newline being rewritten underneath the
	// offsets.
	p.buf.WriteString("%PDF-1.7\n%\xe2\xe3\xcf\xd3\n")

	for i, body := range p.bodies {
		p.offsets[i] = p.buf.Len()
		fmt.Fprintf(&p.buf, "%d 0 obj\n%s\nendobj\n", i+1, body)
	}

	xref := p.buf.Len()

	fmt.Fprintf(&p.buf, "xref\n0 %d\n", len(p.bodies)+1)
	p.buf.WriteString("0000000000 65535 f \n")

	for _, offset := range p.offsets {
		fmt.Fprintf(&p.buf, "%010d 00000 n \n", offset)
	}

	fmt.Fprintf(&p.buf, "trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n",
		len(p.bodies)+1, xref)

	return p.buf.Bytes(), nil
}

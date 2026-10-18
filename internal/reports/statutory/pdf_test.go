package statutory

import (
	"bytes"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// The archive copy is a PDF: it starts as one, ends as one, and is not empty.
func TestPDF_RendersADocument(t *testing.T) {
	document, err := newFixture(t).generate(t).PDF()
	if err != nil {
		t.Fatalf("PDF: %v", err)
	}

	if len(document) < 1000 {
		t.Fatalf("the document is %d bytes, which is not a filing", len(document))
	}
	if !bytes.HasPrefix(document, []byte("%PDF-1.7")) {
		t.Errorf("the document does not start with a PDF header: %q", document[:8])
	}
	if !bytes.HasSuffix(document, []byte("%%EOF\n")) {
		t.Errorf("the document does not end with a PDF trailer")
	}
	if !bytes.Contains(document, []byte("/Type /Catalog")) {
		t.Error("the document has no catalogue")
	}
	if !bytes.Contains(document, []byte("/BaseFont /Helvetica")) {
		t.Error("the document names no font, so a reader has nothing to draw with")
	}
}

// The page count the document declares is the number of pages it holds.
func TestPDF_DeclaresThePagesItHolds(t *testing.T) {
	document, err := newFixture(t).generate(t).PDF()
	if err != nil {
		t.Fatalf("PDF: %v", err)
	}

	declared := declaredPageCount(t, document)
	held := bytes.Count(document, []byte("/Type /Page "))

	if held == 0 {
		t.Fatal("the document holds no pages")
	}
	if declared != held {
		t.Errorf("the document declares %d pages and holds %d", declared, held)
	}
	if declared < 2 {
		t.Errorf("the filing fits on %d page(s), and this fixture is long enough to need more", declared)
	}

	// And a reader is told which page it is on.
	if !bytes.Contains(document, []byte("(Page 1 of ")) {
		t.Error("the pages are not numbered")
	}
}

// declaredPageCount reads /Count out of the document's page tree.
func declaredPageCount(t *testing.T, document []byte) int {
	t.Helper()

	held := regexp.MustCompile(`/Type /Pages /Kids \[[^\]]*\] /Count (\d+)`).FindSubmatch(document)
	if held == nil {
		t.Fatal("the document has no page tree")
	}

	count, err := strconv.Atoi(string(held[1]))
	if err != nil {
		t.Fatalf("the page tree's count is not a number: %v", err)
	}

	return count
}

// Every offset the cross-reference table gives points at the object it says it
// does. A document whose offsets are wrong is a document a reader cannot open.
func TestPDF_CrossReferencePointsAtItsObjects(t *testing.T) {
	document, err := newFixture(t).generate(t).PDF()
	if err != nil {
		t.Fatalf("PDF: %v", err)
	}

	start := bytes.LastIndex(document, []byte("startxref"))
	if start < 0 {
		t.Fatal("the document has no startxref")
	}

	var offset int
	if _, err := fmt.Sscanf(string(document[start:]), "startxref\n%d", &offset); err != nil {
		t.Fatalf("startxref does not name an offset: %v", err)
	}
	if offset <= 0 || offset >= len(document) {
		t.Fatalf("startxref points at %d, outside a document of %d bytes", offset, len(document))
	}
	if !bytes.HasPrefix(document[offset:], []byte("xref\n")) {
		t.Fatalf("startxref points at %q, which is not a cross-reference table", snippet(document, offset))
	}

	// 0 <count>\n then one twenty-byte entry per object, the first being the
	// free head.
	table := document[offset:]
	lines := bytes.SplitN(table, []byte("\n"), 3)
	var first, count int
	if _, err := fmt.Sscanf(string(lines[1]), "%d %d", &first, &count); err != nil || first != 0 {
		t.Fatalf("the table does not start at object 0 or does not say how many it holds: %q", lines[1])
	}
	if count < 4 {
		t.Fatalf("the table holds %d objects, which is too few for a PDF", count)
	}

	if !bytes.HasPrefix(lines[2], []byte("0000000000 65535 f \n")) {
		t.Fatalf("the table's first entry is not the free head: %q", lines[2][:20])
	}

	// Object 0 is the free head, so object N's entry sits at N×20 bytes.
	for number := 1; number < count; number++ {
		entry := string(lines[2][number*20 : (number+1)*20])
		var object, generation int
		if _, err := fmt.Sscanf(entry, "%d %d n", &object, &generation); err != nil {
			t.Fatalf("entry %d is not an entry: %q", number, entry)
		}

		want := []byte(fmt.Sprintf("%d 0 obj", number))
		if !bytes.HasPrefix(document[at(t, document, object):], want) {
			t.Errorf("the table points object %d at %d, which holds %q",
				number, object, snippet(document, at(t, document, object)))
		}
	}
}

func at(t *testing.T, document []byte, offset int) int {
	t.Helper()

	if offset < 0 || offset >= len(document) {
		t.Fatalf("offset %d is outside a document of %d bytes", offset, len(document))
	}

	return offset
}

func snippet(document []byte, offset int) string {
	end := offset + 20
	if end > len(document) {
		end = len(document)
	}

	return string(document[offset:end])
}

// The text in the document is ASCII: the standard fonts carry nothing else, so
// anything else would render as boxes in the archive.
func TestPDF_PrintsTextTheStandardFontsCanCarry(t *testing.T) {
	document, err := newFixture(t).generate(t).PDF()
	if err != nil {
		t.Fatalf("PDF: %v", err)
	}

	streams := 0

	// Split on the header's own newline, because "stream" also ends the word
	// "endstream" and splitting on the bare word would cut the marker away.
	for _, held := range strings.Split(string(document), "\nstream\n")[1:] {
		body, _, found := strings.Cut(held, "\nendstream")
		if !found {
			continue
		}

		streams++

		for _, r := range body {
			if r > 0x7f {
				t.Fatalf("a content stream carries %q, which a standard font cannot draw", r)
			}
		}
	}

	if streams == 0 {
		t.Fatal("the document holds no content streams")
	}
}

// The page count follows the content: a shorter page holds fewer lines and needs
// more of them.
func TestPDF_PaginatesByWhatFitsOnThePage(t *testing.T) {
	out := newFixture(t).generate(t)

	full, err := out.render(a4)
	if err != nil {
		t.Fatalf("render on A4: %v", err)
	}

	tight, err := out.render(layout{width: 595, height: 160, margin: 20, leading: 12, bodySize: 8})
	if err != nil {
		t.Fatalf("render on a short page: %v", err)
	}

	if declaredPageCount(t, tight) <= declaredPageCount(t, full) {
		t.Errorf("a page holding %d lines took %d pages and an A4 page took %d",
			layout{height: 160, margin: 20, leading: 12}.linesPerPage(),
			declaredPageCount(t, tight), declaredPageCount(t, full))
	}
}

func TestPaginate_BreaksWhereThePageEnds(t *testing.T) {
	lines := []string{"1", "2", "3", "4", "5"}

	cases := map[string]struct {
		perPage int
		want    []int
	}{
		"an exact fit":     {5, []int{5}},
		"one over":         {4, []int{4, 1}},
		"two pages":        {3, []int{3, 2}},
		"one line a page":  {1, []int{1, 1, 1, 1, 1}},
		"nothing fits":     {0, []int{1, 1, 1, 1, 1}},
		"a page and a bit": {2, []int{2, 2, 1}},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			pages := paginate(lines, tc.perPage)
			if len(pages) != len(tc.want) {
				t.Fatalf("paginate made %d pages, want %d", len(pages), len(tc.want))
			}

			total := 0
			for i, page := range pages {
				if len(page) != tc.want[i] {
					t.Errorf("page %d holds %d lines, want %d", i+1, len(page), tc.want[i])
				}

				total += len(page)
			}
			if total != len(lines) {
				t.Errorf("the pages hold %d lines and there were %d", total, len(lines))
			}
		})
	}

	if pages := paginate(nil, 10); len(pages) != 1 || len(pages[0]) != 0 {
		t.Errorf("an empty document made %d pages, want one empty page", len(pages))
	}
}

// The transliteration is total: every character this package writes reaches the
// page as something a standard font can draw.
func TestASCII_CarriesEveryCharacterTheCatalogueWrites(t *testing.T) {
	out := newFixture(t).generate(t)

	for _, line := range out.pdfLines(a4) {
		if got := ascii(line); strings.ContainsFunc(got, func(r rune) bool { return r > 0x7f }) {
			t.Errorf("%q transliterates to %q, which is not ASCII", line, got)
		}
	}

	cases := map[string]string{
		"Thuế GTGT đầu ra":            "Thue GTGT dau ra",
		"Đơn vị":                      "Don vi",
		"Báo cáo tình hình tài chính": "Bao cao tinh hinh tai chinh",
		"≤ 3 tỷ":                      "< 3 ty",
		"—":                           "-",
	}
	for written, want := range cases {
		if got := ascii(written); got != want {
			t.Errorf("ascii(%q) = %q, want %q", written, got, want)
		}
	}
}

// A string with the three characters a PDF text string cannot carry raw is
// escaped rather than breaking the stream.
func TestEscapePDF_escapesWhatAStringCannotCarry(t *testing.T) {
	got := escapePDF(`a (b) c\d` + "\n")

	if strings.ContainsAny(got, "()\\") && !strings.Contains(got, `\(`) {
		t.Errorf("escapePDF(%q) = %q, which a stream cannot carry", "a (b)", got)
	}
	if strings.Contains(got, "\n") {
		t.Errorf("escapePDF left a newline in %q", got)
	}
}

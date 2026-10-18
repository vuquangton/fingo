package gl_test

import (
	"testing"
	"time"

	"fingo/internal/domain/gl"
	"fingo/internal/domain/gl/gltest"
	"fingo/internal/money"
)

// voucherDate is fixed so assertions stay deterministic.
var voucherDate = time.Date(2026, 3, 14, 0, 0, 0, 0, time.UTC)

// testRules supplies the chart a voucher is posted against.
//
// Posting without rules is not possible, which is the point: the rules cannot be
// forgotten or bypassed.
func testRules(t *testing.T) gl.PostingRules {
	t.Helper()
	return gltest.TT133Rules(t)
}

// newVoucher builds a draft.
//
// The second argument is the document number the old model required. Numbers
// are now allocated by the ledger when a voucher is posted, so it is ignored —
// the call sites keep it because they read as the documents they represent.
func newVoucher(id, _ string, t gl.VoucherType) *gl.Voucher {
	return gl.NewVoucher(id, "Test entry", t, voucherDate)
}

// testNumber is the number a voucher would be posted under in these tests.
func testNumber(t *testing.T, v *gl.Voucher) gl.Number {
	t.Helper()
	return gltest.NumberFor(t, v, 1)
}

func mustAdd(t *testing.T, v *gl.Voucher, account string, side gl.EntrySide, amount int64) {
	t.Helper()
	line := gl.VoucherLine{AccountCode: account, Side: side, Amount: money.FromInt(amount)}
	if err := v.AddLine(line); err != nil {
		t.Fatalf("AddLine(%s, %s, %d) failed: %v", account, side, amount, err)
	}
}

func mustPost(t *testing.T, v *gl.Voucher, who string) {
	t.Helper()
	if err := v.Post(testRules(t), testNumber(t, v), voucherDate, who); err != nil {
		t.Fatalf("Post(%s) failed: %v", v.ID(), err)
	}
}

// A VAT sales invoice is the most common transaction in a trading SME, and it
// is one debit against two credits. It is unrepresentable under the old
// debit/credit pair shape — see docs/adr/0001-journal-line-model.md.
//
// Expected values come from the invoice itself: net 100 + VAT 10 = 110.
func TestVoucher_VATSalesInvoiceIsExpressible(t *testing.T) {
	v := newVoucher("v-1", "HD0001", gl.VoucherTypeSales)

	mustAdd(t, v, "131", gl.SideDebit, 110)   // Nợ 131  — receivable
	mustAdd(t, v, "5111", gl.SideCredit, 100) // Có 5111 — revenue, net of VAT
	mustAdd(t, v, "33311", gl.SideCredit, 10) // Có 33311 — output VAT

	if err := v.ValidateBalance(); err != nil {
		t.Fatalf("VAT sales invoice must balance, got error: %v", err)
	}
	if got, want := len(v.Lines()), 3; got != want {
		t.Fatalf("voucher lines = %d, want %d", got, want)
	}
}

// A voucher whose debit and credit totals disagree must be rejected. 110
// debited against 100 credited is 10 đồng short of double entry.
func TestVoucher_UnbalancedIsRejected(t *testing.T) {
	v := newVoucher("v-2", "PK002", gl.VoucherTypeGeneral)

	mustAdd(t, v, "131", gl.SideDebit, 110)
	mustAdd(t, v, "5111", gl.SideCredit, 100)

	if err := v.ValidateBalance(); err == nil {
		t.Fatal("unbalanced voucher (110 debit vs 100 credit) must be rejected, got nil")
	}
}

// VND has no subunit: an amount is money.Dong, an integer, so a fractional
// amount cannot be constructed into a line at all. The v1 test that asserted a
// rejection here is gone because the state it described is now unrepresentable;
// the refusal lives at the input boundary (format.Parse), not in AddLine.
// Characterization: per-line validation. Rejected at AddLine so an invalid
// line cannot be constructed into a voucher at all.
func TestVoucher_InvalidLinesAreRejected(t *testing.T) {
	cases := map[string]gl.VoucherLine{
		"empty account": {
			AccountCode: "", Side: gl.SideDebit, Amount: money.FromInt(100),
		},
		"zero amount": {
			AccountCode: "111", Side: gl.SideDebit, Amount: money.Zero,
		},
		"negative amount": {
			AccountCode: "111", Side: gl.SideDebit, Amount: money.FromInt(-100),
		},
		"unknown side": {
			AccountCode: "111", Side: gl.EntrySide("SIDEWAYS"), Amount: money.FromInt(100),
		},
	}

	for name, line := range cases {
		t.Run(name, func(t *testing.T) {
			v := newVoucher("v-x", "PKX", gl.VoucherTypeGeneral)
			if err := v.AddLine(line); err == nil {
				t.Fatalf("%s must be rejected, got nil", name)
			}
		})
	}
}

// Voucher-level: a voucher must carry at least one line, and may not be
// one-sided.
func TestVoucher_DegenerateVouchersAreRejected(t *testing.T) {
	t.Run("no lines", func(t *testing.T) {
		v := newVoucher("v-4", "PK004", gl.VoucherTypeGeneral)
		if err := v.ValidateBalance(); err == nil {
			t.Fatal("empty voucher must be rejected, got nil")
		}
	})

	t.Run("one-sided (debit only)", func(t *testing.T) {
		v := newVoucher("v-5", "PK005", gl.VoucherTypeGeneral)
		mustAdd(t, v, "111", gl.SideDebit, 100)
		if err := v.ValidateBalance(); err == nil {
			t.Fatal("one-sided voucher must be rejected, got nil")
		}
	})
}

// An unbalanced voucher must not reach the ledger. Posting is the moment the
// books accept an entry, so it is the last place the invariant can be enforced.
func TestVoucher_CannotPostUnbalanced(t *testing.T) {
	v := newVoucher("v-6", "PK006", gl.VoucherTypeGeneral)

	mustAdd(t, v, "131", gl.SideDebit, 110)
	mustAdd(t, v, "5111", gl.SideCredit, 100)

	if err := v.Post(testRules(t), testNumber(t, v), voucherDate, "operator-1"); err == nil {
		t.Fatal("unbalanced voucher must not post, got nil error")
	}
	if v.IsPosted() {
		t.Fatal("voucher must remain unposted after a rejected Post")
	}
}

// A posted voucher is immutable. Once it is in the ledger the books must not
// change behind an already-filed figure.
func TestVoucher_PostedVoucherIsImmutable(t *testing.T) {
	v := newVoucher("v-7", "PK007", gl.VoucherTypeGeneral)
	mustAdd(t, v, "1111", gl.SideDebit, 100)
	mustAdd(t, v, "5111", gl.SideCredit, 100)

	mustPost(t, v, "operator-1")

	extra := gl.VoucherLine{AccountCode: "6422", Side: gl.SideDebit, Amount: money.FromInt(50)}
	if err := v.AddLine(extra); err == nil {
		t.Fatal("adding a line to a posted voucher must be rejected, got nil")
	}
	if got, want := len(v.Lines()), 2; got != want {
		t.Fatalf("posted voucher line count = %d, want %d", got, want)
	}
}

// Posting is a one-way transition. A second Post would double the entry, so it
// is refused and the original poster is preserved.
func TestVoucher_CannotPostTwice(t *testing.T) {
	v := newVoucher("v-8", "PK008", gl.VoucherTypeGeneral)
	mustAdd(t, v, "1111", gl.SideDebit, 100)
	mustAdd(t, v, "5111", gl.SideCredit, 100)

	mustPost(t, v, "operator-1")

	if err := v.Post(testRules(t), testNumber(t, v), voucherDate, "operator-2"); err == nil {
		t.Fatal("second post must be rejected, got nil")
	}
	if got, want := v.PostedBy(), "operator-1"; got != want {
		t.Fatalf("PostedBy = %q, want %q — original poster must not be overwritten", got, want)
	}
}

// A posted voucher is corrected by a counter-entry, never by editing. The
// reversal mirrors every line: same accounts, opposite sides, same amounts.
func TestVoucher_ReversalMirrorsLines(t *testing.T) {
	v := newVoucher("v-9", "HD0009", gl.VoucherTypeSales)
	mustAdd(t, v, "131", gl.SideDebit, 110)
	mustAdd(t, v, "5111", gl.SideCredit, 100)
	mustAdd(t, v, "33311", gl.SideCredit, 10)
	mustPost(t, v, "operator-1")

	// Reverse builds the counter-entry; it is the caller who admits it to the
	// ledger, through the same door every other posting uses.
	rev, err := v.Reverse("v-10", voucherDate)
	if err != nil {
		t.Fatalf("reversal failed: %v", err)
	}
	if rev.IsPosted() {
		t.Fatal("a reversal must come back unposted for the caller to post")
	}
	if err := rev.Post(testRules(t), testNumber(t, rev), voucherDate, "operator-1"); err != nil {
		t.Fatalf("posting the reversal failed: %v", err)
	}
	if !rev.IsPosted() {
		t.Fatal("reversal must be posted after Post")
	}

	got := rev.Lines()
	if len(got) != 3 {
		t.Fatalf("reversal lines = %d, want 3", len(got))
	}

	// The receivable that was debited 110 must now be credited 110.
	if got[0].AccountCode != "131" || got[0].Side != gl.SideCredit {
		t.Fatalf("line 0 = %s/%s, want 131/CREDIT", got[0].AccountCode, got[0].Side)
	}
	if !got[0].Amount.Equal(money.FromInt(110)) {
		t.Fatalf("line 0 amount = %s, want 110", got[0].Amount)
	}
	// Revenue and output VAT were credited; they must now be debited.
	if got[1].AccountCode != "5111" || got[1].Side != gl.SideDebit {
		t.Fatalf("line 1 = %s/%s, want 5111/DEBIT", got[1].AccountCode, got[1].Side)
	}
	if got[2].AccountCode != "33311" || got[2].Side != gl.SideDebit {
		t.Fatalf("line 2 = %s/%s, want 33311/DEBIT", got[2].AccountCode, got[2].Side)
	}
	if err := rev.ValidateBalance(); err != nil {
		t.Fatalf("reversal must itself balance: %v", err)
	}
}

// Reversing a draft makes no sense: nothing has been admitted to the ledger.
func TestVoucher_CannotReverseDraft(t *testing.T) {
	v := newVoucher("v-11", "PK011", gl.VoucherTypeGeneral)
	mustAdd(t, v, "1111", gl.SideDebit, 100)
	mustAdd(t, v, "5111", gl.SideCredit, 100)

	if _, err := v.Reverse("v-12", voucherDate); err == nil {
		t.Fatal("reversing an unposted voucher must be rejected, got nil")
	}
}

// Posting without rules is not possible. The chart is not optional context a
// caller may omit — that is what makes the rule unbypassable.
func TestVoucher_PostRequiresRules(t *testing.T) {
	v := newVoucher("v-20", "PK020", gl.VoucherTypeGeneral)
	mustAdd(t, v, "1111", gl.SideDebit, 100)
	mustAdd(t, v, "5111", gl.SideCredit, 100)

	if err := v.Post(gl.PostingRules{}, testNumber(t, v), voucherDate, "operator-1"); err == nil {
		t.Fatal("posting with no chart must be refused, got nil")
	}
}

// An account outside the client's chart cannot be posted to, however well the
// voucher balances.
func TestVoucher_PostRefusesAnAccountOutsideTheChart(t *testing.T) {
	v := newVoucher("v-21", "PK021", gl.VoucherTypeGeneral)
	mustAdd(t, v, "9999", gl.SideDebit, 100)
	mustAdd(t, v, "5111", gl.SideCredit, 100)

	if err := v.Post(testRules(t), testNumber(t, v), voucherDate, "operator-1"); err == nil {
		t.Fatal("posting to an account outside the chart must be refused, got nil")
	}
}

// A parent account is a heading, not a posting target: TK 511 exists only to
// hold 5111–5118. Posting to it would leave the sub-accounts unreconcilable.
func TestVoucher_PostRefusesAParentAccount(t *testing.T) {
	v := newVoucher("v-22", "PK022", gl.VoucherTypeGeneral)
	mustAdd(t, v, "131", gl.SideDebit, 100)
	mustAdd(t, v, "511", gl.SideCredit, 100) // 511 has prescribed children

	if err := v.Post(testRules(t), testNumber(t, v), voucherDate, "operator-1"); err == nil {
		t.Fatal("posting to a parent account must be refused, got nil")
	}

	// The same entry against the leaf account is accepted.
	leaf := newVoucher("v-23", "PK023", gl.VoucherTypeGeneral)
	mustAdd(t, leaf, "131", gl.SideDebit, 100)
	mustAdd(t, leaf, "5111", gl.SideCredit, 100)
	mustPost(t, leaf, "operator-1")
}

// A ledger entry without a description cannot be explained to an auditor, so an
// unnamed voucher is refused.
func TestVoucher_PostRefusesAnEmptyDescription(t *testing.T) {
	v := gl.NewVoucher("v-24", "", gl.VoucherTypeGeneral, voucherDate)
	mustAdd(t, v, "1111", gl.SideDebit, 100)
	mustAdd(t, v, "5111", gl.SideCredit, 100)

	if err := v.Post(testRules(t), testNumber(t, v), voucherDate, "operator-1"); err == nil {
		t.Fatal("posting an unnamed voucher must be refused, got nil")
	}
}

// A draft's description can still be corrected; a posted one is part of the
// record and cannot be rewritten.
func TestVoucher_DescriptionIsEditableOnlyWhileUnposted(t *testing.T) {
	v := newVoucher("v-25", "PK025", gl.VoucherTypeGeneral)
	mustAdd(t, v, "1111", gl.SideDebit, 100)
	mustAdd(t, v, "5111", gl.SideCredit, 100)

	if err := v.SetDescription("Bán hàng cho khách A"); err != nil {
		t.Fatalf("SetDescription on a draft: %v", err)
	}
	if got := v.Description(); got != "Bán hàng cho khách A" {
		t.Fatalf("Description = %q, want it updated", got)
	}

	mustPost(t, v, "operator-1")

	if err := v.SetDescription("something else"); err == nil {
		t.Fatal("editing the description of a posted voucher must be refused, got nil")
	}
}

// Rehydrate rebuilds a voucher from stored state.
//
// It is the only route to a posted voucher that does not pass through Post, and
// it re-validates everything it loads: a database holding an unbalanced voucher
// must fail here rather than leak into the domain.
func TestRehydrate_RebuildsAPostedVoucher(t *testing.T) {
	postedAt := time.Date(2026, 3, 14, 10, 30, 0, 0, time.UTC)

	v, err := gl.Rehydrate(gl.PersistedVoucher{
		ID:          "v-100",
		Book:        gl.BookSalesInvoice,
		FiscalYear:  2026,
		Seq:         100,
		VoucherDate: voucherDate,
		VoucherType: gl.VoucherTypeSales,
		Description: "Bán hàng cho khách A",
		IsPosted:    true,
		PostedAt:    postedAt,
		PostedBy:    "operator-1",
		Lines: []gl.VoucherLine{
			{ID: "l-1", AccountCode: "131", Side: gl.SideDebit, Amount: money.FromInt(110)},
			{ID: "l-2", AccountCode: "5111", Side: gl.SideCredit, Amount: money.FromInt(100)},
			{ID: "l-3", AccountCode: "33311", Side: gl.SideCredit, Amount: money.FromInt(10)},
		},
	})
	if err != nil {
		t.Fatalf("rehydrate failed: %v", err)
	}

	if !v.IsPosted() {
		t.Fatal("a stored posted voucher must come back posted")
	}
	if !v.PostedAt().Equal(postedAt) {
		t.Fatalf("PostedAt = %v, want %v", v.PostedAt(), postedAt)
	}
	if v.PostedBy() != "operator-1" {
		t.Fatalf("PostedBy = %q, want %q", v.PostedBy(), "operator-1")
	}
	if v.VoucherNo() != "HD2026-0100" || v.ID() != "v-100" {
		t.Fatalf("identity not preserved: id=%q no=%q", v.ID(), v.VoucherNo())
	}
	if v.Description() != "Bán hàng cho khách A" {
		t.Fatalf("Description = %q, want it preserved", v.Description())
	}
	if got := len(v.Lines()); got != 3 {
		t.Fatalf("lines = %d, want 3", got)
	}
	// Line order must survive the round trip.
	for i, l := range v.Lines() {
		if l.LineOrder != i+1 {
			t.Fatalf("line %d has LineOrder %d, want %d", i, l.LineOrder, i+1)
		}
	}
}

// A draft round-trips without being posted.
func TestRehydrate_KeepsADraftUnposted(t *testing.T) {
	v, err := gl.Rehydrate(gl.PersistedVoucher{
		ID: "v-101",
		// No number: a stored draft never had one.
		VoucherDate: voucherDate,
		VoucherType: gl.VoucherTypeGeneral,
		Lines: []gl.VoucherLine{
			{ID: "l-1", AccountCode: "111", Side: gl.SideDebit, Amount: money.FromInt(100)},
			{ID: "l-2", AccountCode: "511", Side: gl.SideCredit, Amount: money.FromInt(100)},
		},
	})
	if err != nil {
		t.Fatalf("rehydrate failed: %v", err)
	}
	if v.IsPosted() {
		t.Fatal("a stored draft must come back unposted")
	}
}

// Stored state is not trusted. Corruption must surface at the boundary rather
// than become a voucher the domain would never have accepted.
func TestRehydrate_RejectsCorruptStoredState(t *testing.T) {
	cases := map[string]gl.PersistedVoucher{
		"posted but does not balance": {
			ID: "v-102", VoucherDate: voucherDate,
			VoucherType: gl.VoucherTypeGeneral, IsPosted: true,
			PostedAt: voucherDate, PostedBy: "op",
			Lines: []gl.VoucherLine{
				{ID: "l-1", AccountCode: "131", Side: gl.SideDebit, Amount: money.FromInt(110)},
				{ID: "l-2", AccountCode: "511", Side: gl.SideCredit, Amount: money.FromInt(100)},
			},
		},
		"line with no account": {
			ID: "v-104", VoucherDate: voucherDate,
			VoucherType: gl.VoucherTypeGeneral,
			Lines: []gl.VoucherLine{
				{ID: "l-1", AccountCode: "", Side: gl.SideDebit, Amount: money.FromInt(100)},
			},
		},
	}

	for name, p := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := gl.Rehydrate(p); err == nil {
				t.Fatalf("%s must be rejected, got nil", name)
			}
		})
	}
}

// A draft has no number. A number is a place in a book's sequence for a fiscal
// year, and a voucher takes that place only when it is posted.
func TestVoucher_ADraftHasNoNumber(t *testing.T) {
	v := newVoucher("v-40", "HD0040", gl.VoucherTypeSales)
	mustAdd(t, v, "131", gl.SideDebit, 110)
	mustAdd(t, v, "5111", gl.SideCredit, 100)
	mustAdd(t, v, "33311", gl.SideCredit, 10)

	if got := v.VoucherNo(); got != "" {
		t.Fatalf("a draft must have no number, got %q", got)
	}

	mustPost(t, v, "operator-1")

	if got, want := v.VoucherNo(), "HD2026-0001"; got != want {
		t.Fatalf("after posting the number = %q, want %q", got, want)
	}
	if got := v.Number().Book; got != gl.BookSalesInvoice {
		t.Fatalf("number book = %q, want %q — the book follows the voucher type", got, gl.BookSalesInvoice)
	}
}

// A voucher cannot be numbered in another book: a receipt has no place in the
// sales invoice sequence.
func TestVoucher_PostRefusesANumberFromTheWrongBook(t *testing.T) {
	v := newVoucher("v-41", "PK041", gl.VoucherTypeCashReceipt)
	mustAdd(t, v, "1111", gl.SideDebit, 100)
	mustAdd(t, v, "131", gl.SideCredit, 100)

	wrong := gl.Number{Book: gl.BookSalesInvoice, Year: 2026, Seq: 1}
	if err := v.Post(testRules(t), wrong, voucherDate, "operator-1"); err == nil {
		t.Fatal("a number from the wrong book must be refused, got nil")
	}
}

// Nor in another fiscal year: the sequence restarts each year, so a 2026 voucher
// carrying a 2027 number would collide with a real 2027 document.
func TestVoucher_PostRefusesANumberFromTheWrongYear(t *testing.T) {
	v := newVoucher("v-42", "PK042", gl.VoucherTypeGeneral)
	mustAdd(t, v, "1111", gl.SideDebit, 100)
	mustAdd(t, v, "5111", gl.SideCredit, 100)

	wrong := gl.Number{Book: gl.BookGeneral, Year: 2027, Seq: 1}
	if err := v.Post(testRules(t), wrong, voucherDate, "operator-1"); err == nil {
		t.Fatal("a number from the wrong fiscal year must be refused, got nil")
	}
}

// An unrenderable number must never reach a document.
func TestVoucher_PostRefusesAnInvalidNumber(t *testing.T) {
	v := newVoucher("v-43", "PK043", gl.VoucherTypeGeneral)
	mustAdd(t, v, "1111", gl.SideDebit, 100)
	mustAdd(t, v, "5111", gl.SideCredit, 100)

	unusable := gl.Number{Book: gl.BookGeneral, Year: 2026, Seq: 0}
	if err := v.Post(testRules(t), unusable, voucherDate, "operator-1"); err == nil {
		t.Fatal("a zero sequence must be refused, got nil")
	}
}

package gl_test

import (
	"testing"

	"fingo/internal/domain/gl"
)

// A number is written on the document, so it has to read the way a bookkeeper
// writes it: book, fiscal year, padded sequence.
func TestNumber_RendersAsItIsWrittenOnTheDocument(t *testing.T) {
	cases := []struct {
		number gl.Number
		want   string
	}{
		{gl.Number{Book: gl.BookCashReceipt, Year: 2026, Seq: 1}, "PT2026-0001"},
		{gl.Number{Book: gl.BookCashReceipt, Year: 2026, Seq: 42}, "PT2026-0042"},
		{gl.Number{Book: gl.BookSalesInvoice, Year: 2026, Seq: 1234}, "HD2026-1234"},
		{gl.Number{Book: gl.BookPurchaseInvoice, Year: 2026, Seq: 7}, "HDM2026-0007"},
		{gl.Number{Book: gl.BookGeneral, Year: 2027, Seq: 999}, "PK2027-0999"},
	}

	for _, c := range cases {
		if got := c.number.String(); got != c.want {
			t.Errorf("Number%v.String() = %q, want %q", c.number, got, c.want)
		}
	}
}

// The book follows from what kind of document the voucher is, so a caller does
// not choose a series and cannot number a receipt in the sales book.
func TestBookFor_DerivesTheBookFromTheVoucherType(t *testing.T) {
	cases := map[gl.VoucherType]gl.Book{
		gl.VoucherTypeCashReceipt: gl.BookCashReceipt,
		gl.VoucherTypeCashPayment: gl.BookCashPayment,
		gl.VoucherTypeSales:       gl.BookSalesInvoice,
		gl.VoucherTypePurchase:    gl.BookPurchaseInvoice,
		gl.VoucherTypeGeneral:     gl.BookGeneral,
	}

	for voucherType, want := range cases {
		got, err := gl.BookFor(voucherType)
		if err != nil {
			t.Fatalf("BookFor(%s): %v", voucherType, err)
		}
		if got != want {
			t.Errorf("BookFor(%s) = %s, want %s", voucherType, got, want)
		}
	}

	if _, err := gl.BookFor(gl.VoucherType("MYSTERY")); err == nil {
		t.Fatal("an unknown voucher type has no book, so BookFor must fail")
	}
}

// The sequence restarts each fiscal year: a client's 2027 receipts begin at 1,
// they do not continue 2026's run.
func TestNumber_NextContinuesWithinTheYearOnly(t *testing.T) {
	first := gl.Number{Book: gl.BookCashReceipt, Year: 2026, Seq: 1}

	if got := first.Next(); got.Seq != 2 || got.Year != 2026 || got.Book != gl.BookCashReceipt {
		t.Fatalf("Next() = %v, want the same book and year with sequence 2", got)
	}

	// A new year starts a new run.
	yearEnd := gl.Number{Book: gl.BookCashReceipt, Year: 2026, Seq: 500}
	next := gl.Number{Book: yearEnd.Book, Year: 2027, Seq: 1}
	if got := next.String(); got != "PT2027-0001" {
		t.Fatalf("a new fiscal year must start at 1, got %s", got)
	}
}

// A number is a real-world artifact with sequence rules. One that cannot be
// written on a document must be refused rather than rendered.
func TestNumber_RejectsAnUnusableSequence(t *testing.T) {
	cases := map[string]gl.Number{
		"unknown book":  {Book: gl.Book("ZZ"), Year: 2026, Seq: 1},
		"no book":       {Book: "", Year: 2026, Seq: 1},
		"zero sequence": {Book: gl.BookCashReceipt, Year: 2026, Seq: 0},
		"negative seq":  {Book: gl.BookCashReceipt, Year: 2026, Seq: -1},
		"year zero":     {Book: gl.BookCashReceipt, Year: 0, Seq: 1},
	}

	for name, number := range cases {
		t.Run(name, func(t *testing.T) {
			if err := number.Validate(); err == nil {
				t.Fatalf("%s must be refused, got nil", name)
			}
		})
	}

	good := gl.Number{Book: gl.BookCashReceipt, Year: 2026, Seq: 1}
	if err := good.Validate(); err != nil {
		t.Fatalf("a well-formed number must be accepted: %v", err)
	}
}

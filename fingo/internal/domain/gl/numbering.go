package gl

import (
	"errors"
	"fmt"
)

// Book is the series a voucher number belongs to.
//
// A practice numbers each book separately, and each fiscal year restarts the
// run. A receipt and a sales invoice are numbered independently, and
// "PT2026-0001" and "PT2027-0001" are different documents.
type Book string

const (
	BookCashReceipt     Book = "PT"  // phiếu thu
	BookCashPayment     Book = "PC"  // phiếu chi
	BookSalesInvoice    Book = "HD"  // hóa đơn bán hàng
	BookPurchaseInvoice Book = "HDM" // hóa đơn mua hàng
	BookGeneral         Book = "PK"  // phiếu kế toán
)

func (b Book) Valid() bool {
	switch b {
	case BookCashReceipt, BookCashPayment, BookSalesInvoice, BookPurchaseInvoice, BookGeneral:
		return true
	}
	return false
}

// BookFor returns the series a voucher type is numbered in.
//
// Derived rather than chosen by the caller: a receipt cannot be numbered in the
// sales book, so there is no way to get it wrong.
func BookFor(voucherType VoucherType) (Book, error) {
	switch voucherType {
	case VoucherTypeCashReceipt:
		return BookCashReceipt, nil
	case VoucherTypeCashPayment:
		return BookCashPayment, nil
	case VoucherTypeSales:
		return BookSalesInvoice, nil
	case VoucherTypePurchase:
		return BookPurchaseInvoice, nil
	case VoucherTypeGeneral:
		return BookGeneral, nil
	}

	return "", fmt.Errorf("voucher type %q has no number book", voucherType)
}

// Number is a voucher's place in a book's sequence for one fiscal year.
type Number struct {
	Book Book
	Year int
	Seq  int
}

// String renders the number as it is written on the document.
func (n Number) String() string {
	return fmt.Sprintf("%s%d-%04d", n.Book, n.Year, n.Seq)
}

// Next returns the following number in the same book and year.
func (n Number) Next() Number {
	n.Seq++
	return n
}

// Validate reports whether the number can be written on a document.
func (n Number) Validate() error {
	if !n.Book.Valid() {
		return fmt.Errorf("number book %q is not one of the known series", n.Book)
	}
	if n.Year < 1 {
		return errors.New("a voucher number needs a fiscal year")
	}
	if n.Seq < 1 {
		return errors.New("a voucher sequence starts at 1")
	}

	return nil
}

package gl

import (
	"context"
	"errors"
	"time"

	"github.com/shopspring/decimal"
)

type VoucherType string

const (
	VoucherTypeSales       VoucherType = "SALES"
	VoucherTypePurchase    VoucherType = "PURCHASE"
	VoucherTypeCashReceipt VoucherType = "CASH_RECEIPT"
	VoucherTypeCashPayment VoucherType = "CASH_PAYMENT"
	VoucherTypeGeneral     VoucherType = "GENERAL"
)

type VoucherLine struct {
	ID              string
	VoucherID       string
	LineOrder       int
	DebitAccountID  string
	CreditAccountID string
	Amount          decimal.Decimal
	Note            string
}

type Voucher struct {
	ID          string
	VoucherNo   string
	VoucherDate time.Time
	PostedDate  time.Time
	VoucherType VoucherType
	Description string
	IsPosted    bool
	Lines       []VoucherLine
}

func NewVoucherStub(id, voucherNo string, vType VoucherType, vDate time.Time) *Voucher {
	return &Voucher{
		ID:          id,
		VoucherNo:   voucherNo,
		VoucherType: vType,
		VoucherDate: vDate,
		PostedDate:  vDate,
		IsPosted:    false,
		Lines:       make([]VoucherLine, 0),
	}
}

func (v *Voucher) AddLine(line VoucherLine) {
	line.LineOrder = len(v.Lines) + 1
	v.Lines = append(v.Lines, line)
}

func (v *Voucher) ValidateBalance() error {
	if len(v.Lines) == 0 {
		return errors.New("voucher must contain at least one line")
	}
	for _, l := range v.Lines {
		if l.Amount.LessThanOrEqual(decimal.Zero) {
			return errors.New("voucher line amount must be greater than zero")
		}
		if l.DebitAccountID == "" || l.CreditAccountID == "" {
			return errors.New("debit and credit accounts are required")
		}
	}
	return nil
}

type GLRepositoryStub interface {
	SaveVoucher(ctx context.Context, v *Voucher) error
	GetVoucher(ctx context.Context, id string) (*Voucher, error)
}

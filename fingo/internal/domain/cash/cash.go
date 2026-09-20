package cash

import (
	"context"
	"time"

	"github.com/shopspring/decimal"
)

type CashReceipt struct {
	ID          string
	VoucherNo   string
	Date        time.Time
	PayerName   string
	Reason      string
	CustomerID  string
	Amount      decimal.Decimal
	IsPosted    bool
}

type CashPayment struct {
	ID           string
	VoucherNo    string
	Date         time.Time
	ReceiverName string
	Reason       string
	VendorID     string
	Amount       decimal.Decimal
	IsPosted     bool
}

type BankTransactionType string

const (
	BankDeposit BankTransactionType = "DEPOSIT" // Báo có (112)
	BankPayment BankTransactionType = "PAYMENT" // Ủy nhiệm chi / Báo nợ (112)
)

type BankTransaction struct {
	ID            string
	VoucherNo     string
	Date          time.Time
	Type          BankTransactionType
	BankAccountID string
	Amount        decimal.Decimal
	ReferenceNo   string
	IsPosted      bool
}

func NewCashReceiptStub(id, voucherNo string, date time.Time, amount decimal.Decimal) *CashReceipt {
	return &CashReceipt{
		ID:        id,
		VoucherNo: voucherNo,
		Date:      date,
		Amount:    amount,
		IsPosted:  false,
	}
}

type CashRepositoryStub interface {
	SaveCashReceipt(ctx context.Context, cr *CashReceipt) error
	SaveCashPayment(ctx context.Context, cp *CashPayment) error
	SaveBankTransaction(ctx context.Context, bt *BankTransaction) error
}

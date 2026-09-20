package opening

import (
	"context"
	"errors"
	"time"

	"github.com/shopspring/decimal"
)

type AccountOpeningBalance struct {
	AccountCode  string
	DebitAmount  decimal.Decimal
	CreditAmount decimal.Decimal
}

type CustomerOpeningBalance struct {
	CustomerID   string
	DebitAmount  decimal.Decimal
	CreditAmount decimal.Decimal
}

type VendorOpeningBalance struct {
	VendorID     string
	DebitAmount  decimal.Decimal
	CreditAmount decimal.Decimal
}

type InventoryOpeningBalance struct {
	WarehouseID string
	ItemID      string
	Quantity    decimal.Decimal
	UnitCost    decimal.Decimal
}

func (i InventoryOpeningBalance) TotalAmount() decimal.Decimal {
	return i.Quantity.Mul(i.UnitCost)
}

type OpeningBatch struct {
	ID        string
	AsOfDate  time.Time
	Accounts  []AccountOpeningBalance
	Customers []CustomerOpeningBalance
	Vendors   []VendorOpeningBalance
	Inventory []InventoryOpeningBalance
	IsLocked  bool
}

func NewOpeningBatchStub(id string, asOfDate time.Time) *OpeningBatch {
	return &OpeningBatch{
		ID:        id,
		AsOfDate:  asOfDate,
		Accounts:  make([]AccountOpeningBalance, 0),
		Customers: make([]CustomerOpeningBalance, 0),
		Vendors:   make([]VendorOpeningBalance, 0),
		Inventory: make([]InventoryOpeningBalance, 0),
		IsLocked:  false,
	}
}

func (b *OpeningBatch) AddAccountBalance(code string, debit, credit decimal.Decimal) {
	b.Accounts = append(b.Accounts, AccountOpeningBalance{
		AccountCode:  code,
		DebitAmount:  debit,
		CreditAmount: credit,
	})
}

func (b *OpeningBatch) ValidateBalance() error {
	totalDebit := decimal.Zero
	totalCredit := decimal.Zero

	for _, acc := range b.Accounts {
		totalDebit = totalDebit.Add(acc.DebitAmount)
		totalCredit = totalCredit.Add(acc.CreditAmount)
	}

	if totalDebit.Cmp(totalCredit) != 0 {
		return errors.New("opening balances must balance: total debit != total credit")
	}
	return nil
}

type OpeningRepositoryStub interface {
	SaveOpeningBatch(ctx context.Context, batch *OpeningBatch) error
	GetOpeningBatch(ctx context.Context, asOfDate time.Time) (*OpeningBatch, error)
}

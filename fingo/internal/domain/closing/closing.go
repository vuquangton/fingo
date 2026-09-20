package closing

import (
	"context"
	"errors"
	"time"

	"github.com/shopspring/decimal"
)

type SystemLockConfig struct {
	LockDate time.Time
}

func (c SystemLockConfig) CheckVoucherDateAllowed(voucherDate time.Time) error {
	if !c.LockDate.IsZero() && !voucherDate.After(c.LockDate) {
		return errors.New("cannot create or edit vouchers on or before accounting lock date")
	}
	return nil
}

type ClosingAccountBalance struct {
	AccountCode string
	Amount      decimal.Decimal
}

type PeriodClosing struct {
	ID        string
	Year      int
	Month     int
	ClosedAt  time.Time
	Revenues  []ClosingAccountBalance // 511, 515, 711
	Expenses  []ClosingAccountBalance // 632, 635, 641, 642, 811
	IsClosed  bool
}

func NewPeriodClosingStub(id string, year, month int, closedAt time.Time) *PeriodClosing {
	return &PeriodClosing{
		ID:       id,
		Year:     year,
		Month:    month,
		ClosedAt: closedAt,
		Revenues: make([]ClosingAccountBalance, 0),
		Expenses: make([]ClosingAccountBalance, 0),
		IsClosed: false,
	}
}

func (p *PeriodClosing) AddRevenue(code string, amount decimal.Decimal) {
	p.Revenues = append(p.Revenues, ClosingAccountBalance{AccountCode: code, Amount: amount})
}

func (p *PeriodClosing) AddExpense(code string, amount decimal.Decimal) {
	p.Expenses = append(p.Expenses, ClosingAccountBalance{AccountCode: code, Amount: amount})
}

func (p *PeriodClosing) CalculateNetProfit() decimal.Decimal {
	totalRev := decimal.Zero
	for _, r := range p.Revenues {
		totalRev = totalRev.Add(r.Amount)
	}

	totalExp := decimal.Zero
	for _, e := range p.Expenses {
		totalExp = totalExp.Add(e.Amount)
	}

	return totalRev.Sub(totalExp)
}

type ClosingRepositoryStub interface {
	GetLockDate(ctx context.Context) (time.Time, error)
	SetLockDate(ctx context.Context, lockDate time.Time) error
	SavePeriodClosing(ctx context.Context, closing *PeriodClosing) error
}

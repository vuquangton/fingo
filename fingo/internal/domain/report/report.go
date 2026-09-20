package report

import (
	"context"
	"errors"

	"github.com/shopspring/decimal"
)

type TrialBalanceRow struct {
	AccountCode   string
	AccountName   string
	OpeningDebit  decimal.Decimal
	OpeningCredit decimal.Decimal
	PeriodDebit   decimal.Decimal
	PeriodCredit  decimal.Decimal
	ClosingDebit  decimal.Decimal
	ClosingCredit decimal.Decimal
}

type TrialBalance struct {
	Year  int
	Month int
	Rows  []TrialBalanceRow
}

func NewTrialBalanceStub(year, month int) *TrialBalance {
	return &TrialBalance{
		Year:  year,
		Month: month,
		Rows:  make([]TrialBalanceRow, 0),
	}
}

func (t *TrialBalance) AddRow(code, name string, od, oc, pd, pc, cd, cc decimal.Decimal) {
	t.Rows = append(t.Rows, TrialBalanceRow{
		AccountCode:   code,
		AccountName:   name,
		OpeningDebit:  od,
		OpeningCredit: oc,
		PeriodDebit:   pd,
		PeriodCredit:  pc,
		ClosingDebit:  cd,
		ClosingCredit: cc,
	})
}

func (t *TrialBalance) ValidateBalance() error {
	var totalOD, totalOC, totalPD, totalPC, totalCD, totalCC decimal.Decimal

	for _, r := range t.Rows {
		totalOD = totalOD.Add(r.OpeningDebit)
		totalOC = totalOC.Add(r.OpeningCredit)
		totalPD = totalPD.Add(r.PeriodDebit)
		totalPC = totalPC.Add(r.PeriodCredit)
		totalCD = totalCD.Add(r.ClosingDebit)
		totalCC = totalCC.Add(r.ClosingCredit)
	}

	if totalOD.Cmp(totalOC) != 0 || totalPD.Cmp(totalPC) != 0 || totalCD.Cmp(totalCC) != 0 {
		return errors.New("trial balance equation failed: debit totals do not match credit totals")
	}
	return nil
}

type FinancialReportType string

const (
	ReportBalanceSheet     FinancialReportType = "B01_DNN" // Bảng cân đối kế toán
	ReportIncomeStatement  FinancialReportType = "B02_DNN" // Báo cáo kết quả HĐKD
	ReportCashFlowDirect   FinancialReportType = "B03_DNN" // Lưu chuyển tiền tệ
)

type ReportRepositoryStub interface {
	GenerateTrialBalance(ctx context.Context, year, month int) (*TrialBalance, error)
}

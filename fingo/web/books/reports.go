package books

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/shopspring/decimal"

	"fingo/internal/domain/gl"
	"fingo/internal/period"
	"fingo/internal/posting"
	"fingo/internal/reports"
	"fingo/internal/vat"
	"fingo/web"
)

// periodSource adapts the books to the reports' Periods port.
type periodSource struct{ b *Books }

func (p periodSource) List(context.Context) ([]period.Period, error) {
	return append([]period.Period(nil), p.b.periods...), nil
}

// journalSource adapts the ledger to the reports' Journal port.
//
// Every posted line is handed over, not only the ones inside the window the
// report asked for. The window is the report's own rule — it is the thing that
// splits an account into what it carried in and what the period did to it — and
// a source that pre-filtered would be a second, quieter copy of that rule.
type journalSource struct{ b *Books }

func (j journalSource) Lines(context.Context, time.Time, time.Time) ([]reports.Line, error) {
	out := make([]reports.Line, 0, len(j.b.vouchers))

	for _, e := range j.b.vouchers {
		out = append(out, reports.Line{
			VoucherID:   e.VoucherID,
			VoucherDate: e.VoucherDate,
			AccountCode: e.AccountCode,
			Side:        e.Side,
			Amount:      e.Amount,
		})
	}

	return out, nil
}

// chartSource adapts the chart to the reports' ChartSource port.
type chartSource struct{ b *Books }

func (c chartSource) Chart(context.Context) (*gl.ChartOfAccounts, error) { return c.b.chart, nil }

// reporter builds the report service over these books.
func (b *Books) reporter() (*reports.Service, error) {
	service, err := reports.NewService(periodSource{b}, journalSource{b}, chartSource{b}, closingAccounts())
	if err != nil {
		return nil, err
	}

	return service, nil
}

// TrialBalance reads every account's position for the current period.
func (b *Books) TrialBalance(ctx context.Context) (web.TrialBalancePage, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	p, err := b.reportPeriod()
	if err != nil {
		return web.TrialBalancePage{}, err
	}

	service, err := b.reporter()
	if err != nil {
		return web.TrialBalancePage{}, err
	}

	report, err := service.TrialBalance(ctx, p.ID)
	if err != nil {
		return web.TrialBalancePage{}, err
	}

	return web.NewTrialBalancePage(b.periodRow(p), report), nil
}

// ProfitAndLoss reads the current period's revenue and expenses.
func (b *Books) ProfitAndLoss(ctx context.Context) (web.ProfitAndLossPage, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	p, err := b.reportPeriod()
	if err != nil {
		return web.ProfitAndLossPage{}, err
	}

	service, err := b.reporter()
	if err != nil {
		return web.ProfitAndLossPage{}, err
	}

	report, err := service.ProfitAndLoss(ctx, p.ID)
	if err != nil {
		return web.ProfitAndLossPage{}, err
	}

	return web.NewProfitAndLossPage(b.periodRow(p), report), nil
}

// BalanceSheet reads the position at the end of the current period.
func (b *Books) BalanceSheet(ctx context.Context) (web.BalanceSheetPage, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	p, err := b.reportPeriod()
	if err != nil {
		return web.BalanceSheetPage{}, err
	}

	service, err := b.reporter()
	if err != nil {
		return web.BalanceSheetPage{}, err
	}

	report, err := service.BalanceSheet(ctx, p.ID)
	if err != nil {
		return web.BalanceSheetPage{}, err
	}

	return web.NewBalanceSheetPage(b.periodRow(p), report), nil
}

// Dashboard summarises where the books stand.
func (b *Books) Dashboard(ctx context.Context) (web.Dashboard, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	var out web.Dashboard

	p, ok := b.current()
	if !ok {
		return out, nil
	}

	out.Period = b.periodRow(p)
	out.HasPeriod = true

	rows, err := b.rowsLocked(ctx, web.DocumentFilter{})
	if err != nil {
		return out, err
	}

	receivable := int64(0)

	for _, row := range rows {
		if row.Status == "void" || row.Status == "paid" || row.Status == "posted" {
			continue
		}
		if row.Type != "invoice" {
			continue
		}

		out.Open = append(out.Open, row)
		receivable += row.Outstanding.Units
	}
	out.Receivable = web.NewAmount(receivable)

	// The dashboard's cash and balance figures come from the same trial balance
	// the report shows. Summing them separately is how two screens come to
	// disagree about the same money.
	tb, err := b.trialBalanceLocked(ctx, p)
	if err != nil {
		return out, err
	}

	cash := int64(0)

	for _, row := range tb.Rows {
		if !strings.HasPrefix(row.Code, "111") && !strings.HasPrefix(row.Code, "112") {
			continue
		}

		out.Cash = append(out.Cash, web.AccountBalance{
			Code:   row.Code,
			Name:   row.Name,
			Amount: row.ClosingDebit,
		})

		cash += row.ClosingDebit.Units - row.ClosingCredit.Units
	}

	out.CashTotal = web.NewAmount(cash)

	// What is owed to suppliers, off the rows already in hand. TK 331 is a
	// credit account, so the debt is the closing credit less any closing debit:
	// a supplier account carrying a debit is money paid ahead, not a debt.
	payable := int64(0)

	for _, row := range tb.Rows {
		if !strings.HasPrefix(row.Code, "331") {
			continue
		}

		payable += row.ClosingCredit.Units - row.ClosingDebit.Units
	}

	out.Payable = web.NewAmount(payable)

	// The result comes from the profit and loss report the screen shows, not
	// from a second sum over the same lines. Two screens disagreeing about the
	// same profit is the failure this package is arranged to prevent.
	service, err := b.reporter()
	if err != nil {
		return out, err
	}

	result, err := service.ProfitAndLoss(ctx, p.ID)
	if err != nil {
		return out, err
	}

	out.Profit = web.NewAmount(result.NetProfit.IntPart())
	out.Balanced, out.Debits, out.Credits = tb.Balanced, tb.Debits, tb.Credits
	out.Attention = b.attentionLocked(out)
	out.Recent = newest(rows, dashboardRecent)

	return out, nil
}

// dashboardRecent is how many documents the dashboard lists before a reader is
// expected to go to the documents screen for the rest.
const dashboardRecent = 5

// newest caps a list that is already sorted newest first.
func newest(rows []web.DocumentRow, limit int) []web.DocumentRow {
	if len(rows) <= limit {
		return rows
	}

	return rows[:limit]
}

// attentionLocked is the work the books can prove is outstanding.
//
// Three facts and no more, because three is what this ledger holds. An overdue
// figure needs a due date, an unmatched receipt needs to know what a payment
// was against, and a draft needs a state the ledger does not have. None of them
// can be stated without inventing one, and a work list that invents work is
// worse than an empty one.
func (b *Books) attentionLocked(out web.Dashboard) []web.AttentionRow {
	var rows []web.AttentionRow

	if len(out.Open) > 0 {
		rows = append(rows, web.AttentionRow{
			Count:  len(out.Open),
			What:   "Invoices outstanding",
			Why:    out.Receivable.Text + " owed to the business",
			Action: "Open",
			Href:   "/documents?type=invoice",
		})
	}

	// A period that has ended and is not locked still takes postings, so an
	// entry dated back into it changes accounts somebody has already read.
	unlocked := 0

	for _, p := range b.periods {
		if p.Locked || !p.EndDate.Before(b.now()) {
			continue
		}

		unlocked++
	}

	if unlocked > 0 {
		rows = append(rows, web.AttentionRow{
			Count:  unlocked,
			What:   "Periods ended but not locked",
			Why:    "An entry dated back into one of them still posts",
			Action: "Review",
			Href:   "/settings",
		})
	}

	if !out.Balanced {
		rows = append(rows, web.AttentionRow{
			Count:  1,
			What:   "The ledger does not balance",
			Why:    "Debits " + out.Debits.Text + " against credits " + out.Credits.Text,
			Action: "Inspect",
			Href:   "/reports/trial-balance",
		})
	}

	return rows
}

// trialBalanceLocked is TrialBalance for a caller already holding the lock.
func (b *Books) trialBalanceLocked(ctx context.Context, p period.Period) (web.TrialBalancePage, error) {
	service, err := b.reporter()
	if err != nil {
		return web.TrialBalancePage{}, err
	}

	report, err := service.TrialBalance(ctx, p.ID)
	if err != nil {
		return web.TrialBalancePage{}, err
	}

	return web.NewTrialBalancePage(b.periodRow(p), report), nil
}

// reportPeriod returns the period the reports are read for.
func (b *Books) reportPeriod() (period.Period, error) {
	p, ok := b.current()
	if !ok {
		return period.Period{}, fmt.Errorf("the books have no accounting period; add one in Settings")
	}

	return p, nil
}

// Settings returns the periods, the VAT rates and the posting rules.
func (b *Books) Settings(_ context.Context) (web.SettingsPage, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	today := b.now()

	page := web.SettingsPage{Today: today.Format(time.DateOnly)}

	for _, p := range b.periods {
		page.Periods = append(page.Periods, b.periodRow(p))
	}

	for _, rate := range b.vat.List() {
		row := web.VATRateRow{
			ID:      rate.ID,
			Rate:    percent(rate.Rate),
			From:    rate.EffectiveFrom.Format(time.DateOnly),
			Current: covers(rate, today),
		}
		if rate.EffectiveTo != nil {
			row.To = rate.EffectiveTo.Format(time.DateOnly)
		}

		page.VATRates = append(page.VATRates, row)
	}

	for _, rule := range posting.TT133Defaults() {
		page.PostingRules = append(page.PostingRules, web.PostingRuleRow{
			Document: string(rule.DocType),
			Side:     string(rule.Side),
			Account:  string(rule.AccountType),
			Posts:    rule.AccountCode,
		})
	}

	return page, nil
}

// CreatePeriod adds an accounting period.
//
// The calendar does the refusing: it owns what "a complete set of periods"
// means — no overlaps, no gaps — and a UI that checked those itself would be a
// second, weaker copy of that rule.
func (b *Books) CreatePeriod(_ context.Context, in web.PeriodInput) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	candidate := period.Period{
		ID:        strings.TrimSpace(in.ID),
		StartDate: in.From,
		EndDate:   in.To,
	}

	calendar, err := period.NewCalendar(append(append([]period.Period(nil), b.periods...), candidate))
	if err != nil {
		return err
	}

	b.periods = calendar.Periods()

	return nil
}

// CreateVATRate adds a VAT rate.
func (b *Books) CreateVATRate(_ context.Context, in web.VATRateInput) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	rate := vat.VATRate{
		ID:            strings.TrimSpace(in.ID),
		Rate:          vat.BasisPoints(in.RateBasisPoints),
		EffectiveFrom: in.From,
	}

	if !in.To.IsZero() {
		to := in.To
		rate.EffectiveTo = &to
	}

	return b.vat.Create(rate)
}

// percent renders basis points as a percentage: 1000 is 10%.
func percent(bp vat.BasisPoints) string {
	return decimal.NewFromInt(int64(bp)).Div(decimal.NewFromInt(100)).String() + "%"
}

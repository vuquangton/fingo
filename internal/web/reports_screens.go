// Report screens and the one export: P&L, balance sheet, account drill-down,
// and trial balance as an Excel workbook.
//
// Nothing here computes a figure. Each handler asks internal/books, which asks
// internal/reports, which sums the posted lines. The handler formats.
package web

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/xuri/excelize/v2"

	"fingo/internal/books"
	"fingo/internal/format"
	"fingo/internal/money"
	"fingo/internal/period"
	"fingo/internal/reports"
	"fingo/internal/store"
)

// currentBooks resolves the scoped books and the latest period.
func (a *Auth) currentBooks(c *gin.Context) (*books.Books, period.Period, store.Scope, string, bool) {
	scope, clientName, ok := a.requireScope(c)
	if !ok {
		return nil, period.Period{}, store.Scope{}, "", false
	}

	b := books.Open(a.store, scope)

	p, err := b.EnsureCurrentYear(c.Request.Context(), time.Now())
	if err != nil {
		a.renderTable(c, http.StatusInternalServerError, tableView{Title: "Báo cáo", Client: clientName, Note: err.Error()})
		return nil, period.Period{}, store.Scope{}, "", false
	}

	return b, p, scope, clientName, true
}

// ShowProfitAndLoss renders the period result.
func (a *Auth) ShowProfitAndLoss(c *gin.Context) {
	b, p, _, clientName, ok := a.currentBooks(c)
	if !ok {
		return
	}

	pl, err := b.ProfitAndLoss(c.Request.Context(), p.ID)
	if err != nil {
		a.renderTable(c, http.StatusUnprocessableEntity, tableView{Title: "Kết quả kinh doanh", Client: clientName, Note: err.Error()})
		return
	}

	view := tableView{
		Title:    "Kết quả hoạt động kinh doanh",
		Client:   clientName,
		Headings: []string{"Chỉ tiêu", "Tài khoản", "Số tiền"},
		Numeric:  []bool{false, false, true},
		Empty:    "Kỳ này chưa có doanh thu hay chi phí nào.",
	}

	view.Rows = append(view.Rows, []string{"DOANH THU", "", ""})
	for _, r := range pl.Revenue {
		view.Rows = append(view.Rows, []string{"  " + r.AccountName, r.AccountCode, format.Money(r.Amount)})
	}
	view.Rows = append(view.Rows, []string{"Tổng doanh thu", "", format.Money(pl.TotalRevenue)})
	view.Rows = append(view.Rows, []string{"CHI PHÍ", "", ""})
	for _, r := range pl.Expenses {
		view.Rows = append(view.Rows, []string{"  " + r.AccountName, r.AccountCode, format.Money(r.Amount)})
	}
	view.Rows = append(view.Rows, []string{"Tổng chi phí", "", format.Money(pl.TotalExpenses)})
	view.Rows = append(view.Rows, []string{"Lợi nhuận sau thuế (chưa kết chuyển)", "", format.Money(pl.NetProfit)})

	view.Note = fmt.Sprintf("%s · %s → %s · bỏ qua %d chứng từ kết chuyển",
		p.ID, pl.StartDate.Format("02/01/2006"), pl.EndDate.Format("02/01/2006"), pl.ExcludedClosingVouchers)

	a.renderTable(c, http.StatusOK, view)
}

// ShowBalanceSheet renders the position at the end of the period.
func (a *Auth) ShowBalanceSheet(c *gin.Context) {
	b, p, _, clientName, ok := a.currentBooks(c)
	if !ok {
		return
	}

	bs, err := b.BalanceSheet(c.Request.Context(), p.ID)
	if err != nil {
		a.renderTable(c, http.StatusUnprocessableEntity, tableView{Title: "Bảng cân đối kế toán", Client: clientName, Note: err.Error()})
		return
	}

	view := tableView{
		Title:    "Bảng cân đối kế toán",
		Client:   clientName,
		Headings: []string{"Chỉ tiêu", "Tài khoản", "Số tiền"},
		Numeric:  []bool{false, false, true},
		Empty:    "Chưa có số dư nào.",
	}


	view.Rows = append(view.Rows, []string{"TÀI SẢN", "", ""})
	for _, r := range bs.Assets {
		view.Rows = append(view.Rows, []string{"  " + r.AccountName, r.AccountCode, format.Money(r.Amount)})
	}
	view.Rows = append(view.Rows, []string{"Tổng tài sản", "", format.Money(bs.TotalAssets)})

	view.Rows = append(view.Rows, []string{"NỢ PHẢI TRẢ", "", ""})
	for _, r := range bs.Liabilities {
		view.Rows = append(view.Rows, []string{"  " + r.AccountName, r.AccountCode, format.Money(r.Amount)})
	}
	view.Rows = append(view.Rows, []string{"Tổng nợ phải trả", "", format.Money(bs.TotalLiabilities)})

	view.Rows = append(view.Rows, []string{"VỐN CHỦ SỞ HỮU", "", ""})
	for _, r := range bs.Equity {
		view.Rows = append(view.Rows, []string{"  " + r.AccountName, r.AccountCode, format.Money(r.Amount)})
	}
	view.Rows = append(view.Rows, []string{"Tổng vốn chủ sở hữu", "", format.Money(bs.TotalEquity)})

	state := "cân"
	if !bs.Balanced() {
		state = "KHÔNG cân"
	}
	view.Note = fmt.Sprintf("%s · tại %s · %s", p.ID, bs.AsOf.Format("02/01/2006"), state)

	a.renderTable(c, http.StatusOK, view)
}

// ShowLedger lists one account's movements: the far end of a report row.
func (a *Auth) ShowLedger(c *gin.Context) {
	b, p, _, clientName, ok := a.currentBooks(c)
	if !ok {
		return
	}

	account := c.Query("account")
	if account == "" {
		c.Redirect(http.StatusSeeOther, "/reports")
		return
	}

	lines, err := b.AccountLines(c.Request.Context(), account, p.StartDate, p.EndDate)
	if err != nil {
		a.renderTable(c, http.StatusUnprocessableEntity, tableView{Title: "Sổ chi tiết " + account, Client: clientName, Note: err.Error()})
		return
	}

	view := tableView{
		Title:    "Sổ chi tiết tài khoản " + account,
		Client:   clientName,
		Headings: []string{"Ngày", "Chứng từ", "Diễn giải", "Nợ", "Có", "Luỹ kế"},
		Numeric:  []bool{false, false, false, true, true, true},
		Empty:    "Tài khoản này chưa có bút toán nào trong kỳ.",
	}

	running := money.Zero
	for _, l := range lines {
		debit, credit := "", ""
		if l.Side == "DEBIT" {
			running = running.Add(l.Amount)
			debit = format.Money(l.Amount)
		} else {
			running = running.Sub(l.Amount)
			credit = format.Money(l.Amount)
		}
		view.Rows = append(view.Rows, []string{
			l.VoucherDate.Format("02/01/2006"), l.VoucherID, "", debit, credit, format.Money(running),
		})
	}

	view.Note = fmt.Sprintf("%s · %d bút toán", p.ID, len(lines))
	a.renderTable(c, http.StatusOK, view)
}

// ExportTrialBalance writes the trial balance as an .xlsx file.
//
// excelize is the wheel: it owns OOXML, styling and the content type the browser
// downloads. The report itself is still internal/reports.
func (a *Auth) ExportTrialBalance(c *gin.Context) {
	b, p, _, _, ok := a.currentBooks(c)
	if !ok {
		return
	}

	tb, err := b.TrialBalance(c.Request.Context(), p.ID)
	if err != nil {
		c.String(http.StatusUnprocessableEntity, "không lập được bảng cân đối: %v", err)
		return
	}

	sheet, err := trialBalanceSheet(tb)
	if err != nil {
		c.String(http.StatusInternalServerError, "không tạo được tệp Excel: %v", err)
		return
	}

	c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="can-doi-%s.xlsx"`, p.ID))
	c.Header("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	c.Status(http.StatusOK)
	if err := sheet.Write(c.Writer); err != nil {
		c.String(http.StatusInternalServerError, "không ghi được tệp Excel: %v", err)
	}
}

// trialBalanceSheet builds the workbook. The test reads it back with excelize
// rather than asserting on bytes, so the assertion is about the figures.
func trialBalanceSheet(tb reports.TrialBalance) (*excelize.File, error) {
	f := excelize.NewFile()
	const sheet = "CanDoi"

	idx, err := f.NewSheet(sheet)
	if err != nil {
		return nil, fmt.Errorf("new sheet: %w", err)
	}
	f.SetActiveSheet(idx)
	_ = f.DeleteSheet("Sheet1")

	headings := []string{
		"Tài khoản", "Tên", "Nợ đầu kỳ", "Có đầu kỳ",
		"Nợ phát sinh", "Có phát sinh", "Nợ cuối kỳ", "Có cuối kỳ",
	}

	write := func(row int, values []any) error {
		for i, v := range values {
			cell, err := excelize.CoordinatesToCellName(i+1, row)
			if err != nil {
				return err
			}
			if err := f.SetCellValue(sheet, cell, v); err != nil {
				return err
			}
		}
		return nil
	}

	header := make([]any, len(headings))
	for i, h := range headings {
		header[i] = h
	}
	if err := write(1, header); err != nil {
		return nil, err
	}

	for i, r := range tb.Rows {
		if err := write(i+2, []any{
			r.AccountCode, r.AccountName,
			r.OpeningDebit.Int64(), r.OpeningCredit.Int64(),
			r.PeriodDebit.Int64(), r.PeriodCredit.Int64(),
			r.ClosingDebit.Int64(), r.ClosingCredit.Int64(),
		}); err != nil {
			return nil, err
		}
	}

	totalRow := len(tb.Rows) + 2
	if err := write(totalRow, []any{
		"", "Tổng cộng",
		tb.OpeningDebit.Int64(), tb.OpeningCredit.Int64(),
		tb.PeriodDebit.Int64(), tb.PeriodCredit.Int64(),
		tb.ClosingDebit.Int64(), tb.ClosingCredit.Int64(),
	}); err != nil {
		return nil, err
	}

	style, err := f.NewStyle(&excelize.Style{Font: &excelize.Font{Bold: true}})
	if err != nil {
		return nil, err
	}
	if err := f.SetCellStyle(sheet, "A1", "H1", style); err != nil {
		return nil, err
	}
	if err := f.SetColWidth(sheet, "A", "A", 12); err != nil {
		return nil, err
	}
	if err := f.SetColWidth(sheet, "B", "B", 40); err != nil {
		return nil, err
	}
	if err := f.SetColWidth(sheet, "C", "H", 16); err != nil {
		return nil, err
	}

	return f, nil
}

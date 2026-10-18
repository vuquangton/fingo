// Read screens: chart of accounts, journal, trial balance.
//
// Every figure comes from the same rows the ledger wrote. There is no cached
// total and no second query that "just sums it differently": the trial balance is
// internal/reports, and this file only formats what it returns.
package web

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"

	"fingo/internal/books"
	"fingo/internal/format"
	"fingo/internal/money"
)

// tableView is one rendered table.
type tableView struct {
	Title    string
	Client   string
	Note     string
	Headings []string
	Rows     [][]string
	Empty    string
	Numeric  []bool // which columns are amounts, rendered right-aligned
}

func (a *Auth) renderTable(c *gin.Context, status int, view tableView) {
	email, _, _ := UserFrom(c)
	data := struct {
		Email  string
		Client string
		tableView
	}{Email: email, Client: view.Client, tableView: view}

	c.Status(status)
	if err := templates.ExecuteTemplate(c.Writer, "table.html", data); err != nil {
		c.String(http.StatusInternalServerError, "không dựng được trang: %v", err)
	}
}

// ShowAccounts lists the chart.
func (a *Auth) ShowAccounts(c *gin.Context) {
	scope, clientName, ok := a.requireScope(c)
	if !ok {
		return
	}

	view := tableView{
		Title:    "Hệ thống tài khoản",
		Client:   clientName,
		Headings: []string{"Tài khoản", "Tên", "Loại", "Ghi sổ được"},
		Empty:    "Chưa có tài khoản nào.",
	}

	err := a.store.Read(c.Request.Context(), scope, func(tx pgx.Tx) error {
		rows, err := tx.Query(c.Request.Context(), `
			SELECT a.code, a.name, a.account_type,
			       NOT EXISTS (SELECT 1 FROM accounts child WHERE child.parent_code = a.code) AS postable
			  FROM accounts a
			 WHERE a.active
			 ORDER BY a.code`)
		if err != nil {
			return err
		}
		defer rows.Close()

		for rows.Next() {
			var code, name, accountType string
			var postable bool
			if err := rows.Scan(&code, &name, &accountType, &postable); err != nil {
				return err
			}
			mark := "cấp 1"
			if postable {
				mark = "có"
			}
			view.Rows = append(view.Rows, []string{code, name, accountType, mark})
		}
		return rows.Err()
	})
	if err != nil {
		a.renderTable(c, http.StatusInternalServerError, tableView{Title: "Hệ thống tài khoản", Note: err.Error()})
		return
	}

	view.Note = fmt.Sprintf("%d tài khoản đang dùng.", len(view.Rows))
	view.Numeric = []bool{false, false, false, false}
	a.renderTable(c, http.StatusOK, view)
}

// ShowJournal lists posted vouchers with their totals.
func (a *Auth) ShowJournal(c *gin.Context) {
	scope, clientName, ok := a.requireScope(c)
	if !ok {
		return
	}

	view := tableView{
		Title:    "Sổ nhật ký chung",
		Client:   clientName,
		Headings: []string{"Số chứng từ", "Ngày", "Diễn giải", "Số tiền"},
		Empty:    "Chưa có chứng từ nào được ghi sổ.",
		Numeric:  []bool{false, false, false, true},
	}

	err := a.store.Read(c.Request.Context(), scope, func(tx pgx.Tx) error {
		rows, err := tx.Query(c.Request.Context(), `
			SELECT v.voucher_no, v.voucher_date, v.description, COALESCE(SUM(l.amount) FILTER (WHERE l.side = 'DEBIT'), 0)
			  FROM vouchers v
			  JOIN voucher_lines l ON l.voucher_id = v.id
			 WHERE v.posted_at IS NOT NULL
			 GROUP BY v.id, v.voucher_no, v.voucher_date, v.description
			 ORDER BY v.voucher_date, v.voucher_no`)
		if err != nil {
			return err
		}
		defer rows.Close()

		for rows.Next() {
			var no, description string
			var date time.Time
			var total int64
			if err := rows.Scan(&no, &date, &description, &total); err != nil {
				return err
			}
			view.Rows = append(view.Rows, []string{
				no, date.Format("02/01/2006"), description, format.Money(money.FromInt(total)),
			})
		}
		return rows.Err()
	})
	if err != nil {
		a.renderTable(c, http.StatusInternalServerError, tableView{Title: "Sổ nhật ký chung", Client: clientName, Note: err.Error()})
		return
	}

	view.Note = fmt.Sprintf("%d chứng từ đã ghi sổ.", len(view.Rows))
	a.renderTable(c, http.StatusOK, view)
}

// ShowTrialBalance renders the trial balance for the latest period.
func (a *Auth) ShowTrialBalance(c *gin.Context) {
	scope, clientName, ok := a.requireScope(c)
	if !ok {
		return
	}

	b := books.Open(a.store, scope)

	ledgerPeriod, err := b.EnsureCurrentYear(c.Request.Context(), time.Now())
	if err != nil {
		a.renderTable(c, http.StatusInternalServerError, tableView{Title: "Bảng cân đối số phát sinh", Client: clientName, Note: err.Error()})
		return
	}

	tb, err := b.TrialBalance(c.Request.Context(), ledgerPeriod.ID)
	if err != nil {
		a.renderTable(c, http.StatusUnprocessableEntity, tableView{Title: "Bảng cân đối số phát sinh", Client: clientName, Note: err.Error()})
		return
	}

	view := tableView{
		Title:    "Bảng cân đối số phát sinh",
		Client:   clientName,
		Empty:    "Kỳ này chưa có bút toán nào.",
		Headings: []string{"Tài khoản", "Tên", "Nợ đầu kỳ", "Có đầu kỳ", "Nợ phát sinh", "Có phát sinh", "Nợ cuối kỳ", "Có cuối kỳ"},
		Numeric:  []bool{false, false, true, true, true, true, true, true},
	}

	for _, row := range tb.Rows {
		view.Rows = append(view.Rows, []string{
			row.AccountCode,
			row.AccountName,
			format.Money(row.OpeningDebit),
			format.Money(row.OpeningCredit),
			format.Money(row.PeriodDebit),
			format.Money(row.PeriodCredit),
			format.Money(row.ClosingDebit),
			format.Money(row.ClosingCredit),
		})
	}

	state := "cân"
	if !tb.Balanced() {
		state = "KHÔNG cân"
	}
	view.Note = fmt.Sprintf("%s · %d tài khoản · %d chứng từ · %s",
		ledgerPeriod.ID, len(tb.Rows), tb.SourceVoucherCount, state)

	a.renderTable(c, http.StatusOK, view)
}

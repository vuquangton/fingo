// Aging: what is owed, by how long it has been owed.
package web

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"fingo/internal/books"
	"fingo/internal/format"
)

type agingRowView struct {
	Partner   string
	Documents int
	Current   string
	Days30    string
	Days60    string
	Days90    string
	Older     string
	Total     string
}

type agingView struct {
	Title    string
	Control  string
	Rows     []agingRowView
	Current  string
	Days30   string
	Days60   string
	Days90   string
	Older    string
	Total    string
	Ledger   string
	Diff     string
	Balanced bool
}

type agingData struct {
	Email  string
	Client string
	AsOf   string
	Views  []agingView
	Error  string
}

// ShowAging renders receivables and payables aged as of a date.
func (a *Auth) ShowAging(c *gin.Context) {
	scope, clientName, ok := a.requireScope(c)
	if !ok {
		return
	}

	asOf := time.Now()
	if q := c.Query("as_of"); q != "" {
		parsed, err := time.Parse("2006-01-02", q)
		if err != nil {
			a.renderAging(c, http.StatusUnprocessableEntity, agingData{Client: clientName, Error: "ngày không hợp lệ"})
			return
		}
		asOf = parsed
	}

	b := books.Open(a.store, scope)

	ar, err := b.Aging(c.Request.Context(), "AR", asOf)
	if err != nil {
		a.renderAging(c, http.StatusInternalServerError, agingData{Client: clientName, Error: err.Error()})
		return
	}
	ap, err := b.Aging(c.Request.Context(), "AP", asOf)
	if err != nil {
		a.renderAging(c, http.StatusInternalServerError, agingData{Client: clientName, Error: err.Error()})
		return
	}

	email, _, _ := UserFrom(c)
	data := agingData{
		Email:  email,
		Client: clientName,
		AsOf:   asOf.Format("2006-01-02"),
	}
	data.Views = []agingView{viewOf(ar), viewOf(ap)}

	c.Status(http.StatusOK)
	if err := templates.ExecuteTemplate(c.Writer, "aging.html", data); err != nil {
		c.String(http.StatusInternalServerError, "không dựng được trang tuổi nợ: %v", err)
	}
}

func viewOf(report books.AgingReport) agingView {
	view := agingView{
		Title:    report.Title,
		Control:  report.Control,
		Current:  format.Money(report.Totals.Current),
		Days30:   format.Money(report.Totals.Days30),
		Days60:   format.Money(report.Totals.Days60),
		Days90:   format.Money(report.Totals.Days90),
		Older:    format.Money(report.Totals.Older),
		Total:    format.Money(report.Totals.Total()),
		Ledger:   format.Money(report.ControlBalance),
		Diff:     format.Money(report.Difference()),
		Balanced: report.Difference().IsZero(),
	}

	for _, row := range report.Rows {
		view.Rows = append(view.Rows, agingRowView{
			Partner:   row.Partner,
			Documents: row.Documents,
			Current:   format.Money(row.Current),
			Days30:    format.Money(row.Days30),
			Days60:    format.Money(row.Days60),
			Days90:    format.Money(row.Days90),
			Older:     format.Money(row.Older),
			Total:     format.Money(row.Total()),
		})
	}

	return view
}

func (a *Auth) renderAging(c *gin.Context, status int, data agingData) {
	c.Status(status)
	if err := templates.ExecuteTemplate(c.Writer, "aging.html", data); err != nil {
		c.String(http.StatusInternalServerError, "không dựng được trang tuổi nợ: %v", err)
	}
}

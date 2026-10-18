// Period close screen.
//
// The engine (internal/close) decides everything: whether the period may close,
// which accounts are carried, what the result is, and that the lock is written.
// This file asks it to run and shows what it said.
package web

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"fingo/internal/books"
	"fingo/internal/close"
	"fingo/internal/format"
)

type closePeriodView struct {
	ID        string
	Name      string
	Start     string
	End       string
	Locked    bool
	NetProfit string
	HasReport bool
}

type closeData struct {
	Email   string
	Client  string
	Periods []closePeriodView
	Closed  string
	Error   string
}

// ShowClose lists the periods and their close state.
func (a *Auth) ShowClose(c *gin.Context) {
	scope, clientName, ok := a.requireScope(c)
	if !ok {
		return
	}

	b := books.Open(a.store, scope)

	if _, err := b.EnsureCurrentYear(c.Request.Context(), time.Now()); err != nil {
		a.renderClose(c, http.StatusInternalServerError, closeData{Client: clientName, Error: err.Error()})
		return
	}

	periods, err := b.List(c.Request.Context())
	if err != nil {
		a.renderClose(c, http.StatusInternalServerError, closeData{Client: clientName, Error: err.Error()})
		return
	}

	views := make([]closePeriodView, 0, len(periods))
	for _, p := range periods {
		v := closePeriodView{
			ID:     p.ID,
			Name:   p.ID,
			Start:  p.StartDate.Format("02/01/2006"),
			End:    p.EndDate.Format("02/01/2006"),
			Locked: p.Locked,
		}

		// A closed period can state what it closed. A period with no report yet
		// simply has nothing to show, which is not an error.
		if report, err := b.ClosingReport(c.Request.Context(), p.ID); err == nil {
			v.HasReport = true
			v.NetProfit = format.Money(report.NetProfit)
		}

		views = append(views, v)
	}

	email, _, _ := UserFrom(c)
	a.renderClose(c, http.StatusOK, closeData{
		Email:   email,
		Client:  clientName,
		Periods: views,
		Closed:  c.Query("closed"),
	})
}

// PostClose runs the close for one period.
func (a *Auth) PostClose(c *gin.Context) {
	scope, clientName, ok := a.requireScope(c)
	if !ok {
		return
	}

	periodID := c.PostForm("period_id")
	if periodID == "" {
		a.renderClose(c, http.StatusBadRequest, closeData{Client: clientName, Error: "thiếu kỳ kế toán"})
		return
	}

	email, _, _ := UserFrom(c)
	b := books.Open(a.store, scope)

	report, err := b.ClosePeriod(c.Request.Context(), close.Command{
		PeriodID:  periodID,
		VoucherID: uuid.NewString(),
		Operator:  email,
		At:        time.Now(),
	})
	if err != nil {
		status := http.StatusUnprocessableEntity
		a.renderClose(c, status, closeData{
			Email:  email,
			Client: clientName,
			Error:  err.Error(),
		})
		return
	}

	c.Redirect(http.StatusSeeOther, "/close?closed="+format.Money(report.NetProfit))
}

func (a *Auth) renderClose(c *gin.Context, status int, data closeData) {
	c.Status(status)
	if err := templates.ExecuteTemplate(c.Writer, "close.html", data); err != nil {
		c.String(http.StatusInternalServerError, "không dựng được trang khoá sổ: %v", err)
	}
}

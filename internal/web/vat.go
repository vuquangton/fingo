// VAT summary screen: the month-end position, read from the ledger.
package web

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"fingo/internal/books"
	"fingo/internal/format"
)

type vatData struct {
	Email  string
	Client string
	From   string
	To     string
	Output string
	Input  string
	Net    string
	Rev    string
	Error  string
}

func (a *Auth) ShowVAT(c *gin.Context) {
	scope, clientName, ok := a.requireScope(c)
	if !ok {
		return
	}

	b := books.Open(a.store, scope)

	period, err := b.EnsureCurrentYear(c.Request.Context(), time.Now())
	if err != nil {
		a.renderVAT(c, http.StatusInternalServerError, vatData{Client: clientName, Error: err.Error()})
		return
	}

	summary, err := b.VATSummary(c.Request.Context(), period.ID)
	if err != nil {
		a.renderVAT(c, http.StatusInternalServerError, vatData{Client: clientName, Error: err.Error()})
		return
	}

	email, _, _ := UserFrom(c)
	a.renderVAT(c, http.StatusOK, vatData{
		Email:  email,
		Client: clientName,
		From:   summary.StartDate.Format("02/01/2006"),
		To:     summary.EndDate.Format("02/01/2006"),
		Output: format.Money(summary.OutputVAT),
		Input:  format.Money(summary.InputVAT),
		Net:    format.Money(summary.NetPayable),
		Rev:    format.Money(summary.Revenue),
	})
}

func (a *Auth) renderVAT(c *gin.Context, status int, data vatData) {
	c.Status(status)
	if err := templates.ExecuteTemplate(c.Writer, "vat.html", data); err != nil {
		c.String(http.StatusInternalServerError, "không dựng được trang thuế GTGT: %v", err)
	}
}

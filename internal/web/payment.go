// Supplier payments: money out, applied to the bill it settles.
package web

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"fingo/internal/books"
	"fingo/internal/format"
	"fingo/internal/money"
)

type payableRow struct {
	ID          string
	Supplier    string
	Date        string
	Total       string
	Allocated   string
	Outstanding string
}

type paymentData struct {
	Email  string
	Client string
	Rows   []payableRow
	Date   string
	Posted string
	Error  string
}

// ShowPayments lists what is still owed to suppliers.
func (a *Auth) ShowPayments(c *gin.Context) {
	scope, clientName, ok := a.requireScope(c)
	if !ok {
		return
	}

	b := books.Open(a.store, scope)

	open, err := b.OpenPayables(c.Request.Context())
	if err != nil {
		a.renderPayments(c, http.StatusInternalServerError, paymentData{Client: clientName, Error: err.Error()})
		return
	}

	partners, err := b.ListPartners(c.Request.Context())
	if err != nil {
		a.renderPayments(c, http.StatusInternalServerError, paymentData{Client: clientName, Error: err.Error()})
		return
	}
	names := make(map[string]string, len(partners))
	for _, p := range partners {
		names[p.ID] = p.Name
	}

	rows := make([]payableRow, 0, len(open))
	for _, d := range open {
		outstanding := d.Total.Amount() - d.Allocated.Amount()
		rows = append(rows, payableRow{
			ID:          d.ID,
			Supplier:    names[d.PartnerID],
			Date:        d.Date.Format("02/01/2006"),
			Total:       format.Money(money.FromInt(d.Total.Amount())),
			Allocated:   format.Money(money.FromInt(d.Allocated.Amount())),
			Outstanding: format.Money(money.FromInt(outstanding)),
		})
	}

	email, _, _ := UserFrom(c)
	a.renderPayments(c, http.StatusOK, paymentData{
		Email:  email,
		Client: clientName,
		Rows:   rows,
		Date:   time.Now().Format("2006-01-02"),
		Posted: c.Query("posted"),
	})
}

// PostPayment records one supplier payment.
func (a *Auth) PostPayment(c *gin.Context) {
	scope, clientName, ok := a.requireScope(c)
	if !ok {
		return
	}

	date, err := time.Parse("2006-01-02", c.PostForm("date"))
	if err != nil {
		a.renderPayments(c, http.StatusUnprocessableEntity, paymentData{Client: clientName, Error: "ngày chi không hợp lệ"})
		return
	}

	amount, err := format.Amount(c.PostForm("amount"))
	if err != nil {
		a.renderPayments(c, http.StatusUnprocessableEntity, paymentData{Client: clientName, Error: err.Error()})
		return
	}

	email, _, _ := UserFrom(c)

	posted, err := books.Open(a.store, scope).Payment(c.Request.Context(), books.PaymentCommand{
		DocumentID: c.PostForm("document_id"),
		Amount:     amount,
		Date:       date,
		Actor:      email,
	})
	if err != nil {
		a.renderPayments(c, http.StatusUnprocessableEntity, paymentData{Client: clientName, Error: err.Error()})
		return
	}

	c.Redirect(http.StatusSeeOther, "/payment?posted="+posted.VoucherNo)
}

func (a *Auth) renderPayments(c *gin.Context, status int, data paymentData) {
	c.Status(status)
	if err := templates.ExecuteTemplate(c.Writer, "payment.html", data); err != nil {
		c.String(http.StatusInternalServerError, "không dựng được trang trả tiền: %v", err)
	}
}

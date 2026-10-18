// Receipts: money from a customer, applied to the invoice it settles.
//
// The engine does the accounting and the allocation: this file shows what is
// open and passes the amount on.
package web

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"fingo/internal/books"
	"fingo/internal/format"
	"fingo/internal/money"
)

type receivableRow struct {
	ID          string
	Partner     string
	Date        string
	Total       string
	Allocated   string
	Outstanding string
}

type receiptData struct {
	Email  string
	Client string
	Rows   []receivableRow
	Date   string
	Posted string
	Error  string
}

// ShowReceipts lists what is still owed and offers a settle form per invoice.
func (a *Auth) ShowReceipts(c *gin.Context) {
	scope, clientName, ok := a.requireScope(c)
	if !ok {
		return
	}

	b := books.Open(a.store, scope)

	open, err := b.OpenReceivables(c.Request.Context())
	if err != nil {
		a.renderReceipts(c, http.StatusInternalServerError, receiptData{Client: clientName, Error: err.Error()})
		return
	}

	partners, err := b.ListPartners(c.Request.Context())
	if err != nil {
		a.renderReceipts(c, http.StatusInternalServerError, receiptData{Client: clientName, Error: err.Error()})
		return
	}
	names := make(map[string]string, len(partners))
	for _, p := range partners {
		names[p.ID] = p.Name
	}

	rows := make([]receivableRow, 0, len(open))
	for _, d := range open {
		outstanding := d.Total.Amount() - d.Allocated.Amount()
		rows = append(rows, receivableRow{
			ID:          d.ID,
			Partner:     names[d.PartnerID],
			Date:        d.Date.Format("02/01/2006"),
			Total:       format.Money(money.FromInt(d.Total.Amount())),
			Allocated:   format.Money(money.FromInt(d.Allocated.Amount())),
			Outstanding: format.Money(money.FromInt(outstanding)),
		})
	}

	email, _, _ := UserFrom(c)
	a.renderReceipts(c, http.StatusOK, receiptData{
		Email:  email,
		Client: clientName,
		Rows:   rows,
		Date:   time.Now().Format("2006-01-02"),
		Posted: c.Query("posted"),
	})
}

// PostReceipt records one receipt and settles the invoice.
func (a *Auth) PostReceipt(c *gin.Context) {
	scope, clientName, ok := a.requireScope(c)
	if !ok {
		return
	}

	documentID := c.PostForm("document_id")
	date, err := time.Parse("2006-01-02", c.PostForm("date"))
	if err != nil {
		a.renderReceipts(c, http.StatusUnprocessableEntity, receiptData{Client: clientName, Error: "ngày thu không hợp lệ"})
		return
	}

	amount, err := format.Amount(c.PostForm("amount"))
	if err != nil {
		a.renderReceipts(c, http.StatusUnprocessableEntity, receiptData{Client: clientName, Error: err.Error()})
		return
	}

	email, _, _ := UserFrom(c)

	posted, err := books.Open(a.store, scope).Receipt(c.Request.Context(), books.ReceiptCommand{
		DocumentID: documentID,
		Amount:     amount,
		Date:       date,
		Actor:      email,
	})
	if err != nil {
		a.renderReceipts(c, http.StatusUnprocessableEntity, receiptData{Client: clientName, Error: err.Error()})
		return
	}

	c.Redirect(http.StatusSeeOther, "/receipt?posted="+posted.VoucherNo)
}

func (a *Auth) renderReceipts(c *gin.Context, status int, data receiptData) {
	c.Status(status)
	if err := templates.ExecuteTemplate(c.Writer, "receipt.html", data); err != nil {
		c.String(http.StatusInternalServerError, "không dựng được trang thu tiền: %v", err)
	}
}

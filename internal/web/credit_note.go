// Sales returns: a credit note against an invoice, full or partial.
package web

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"fingo/internal/books"
	"fingo/internal/format"
)

type creditNoteLine struct {
	LineNo   int
	ItemID   string
	Sold     int
	Returned int
	Price    string
	Rate     string
}

type creditNoteData struct {
	Email     string
	Client    string
	Date      string
	InvoiceID string
	Lines     []creditNoteLine
	Error     string
	Posted    string
}

func (a *Auth) ShowCreditNote(c *gin.Context) {
	scope, clientName, ok := a.requireScope(c)
	if !ok {
		return
	}

	b := books.Open(a.store, scope)
	invoiceID := c.Query("invoice")

	data := creditNoteData{Client: clientName, InvoiceID: invoiceID, Date: time.Now().Format("2006-01-02"), Posted: c.Query("posted")}
	if invoiceID != "" {
		lines, err := b.SaleLines(c.Request.Context(), invoiceID)
		if err != nil {
			data.Error = err.Error()
		}
		for _, l := range lines {
			data.Lines = append(data.Lines, creditNoteLine{
				LineNo: l.LineNo, ItemID: l.ItemID, Sold: l.Quantity, Returned: l.Returned,
				Price: format.Money(l.UnitPrice), Rate: l.RateID,
			})
		}
	}

	email, _, _ := UserFrom(c)
	data.Email = email
	a.renderCreditNote(c, http.StatusOK, data)
}

func (a *Auth) PostCreditNote(c *gin.Context) {
	scope, clientName, ok := a.requireScope(c)
	if !ok {
		return
	}

	date, err := time.Parse("2006-01-02", c.PostForm("date"))
	if err != nil {
		a.renderCreditNote(c, http.StatusUnprocessableEntity, creditNoteData{Client: clientName, Error: "ngày trả lại không hợp lệ"})
		return
	}

	quantities := map[int]int{}
	for key, values := range c.Request.PostForm {
		if !strings.HasPrefix(key, "qty_") || len(values) == 0 || strings.TrimSpace(values[0]) == "" {
			continue
		}
		lineNo, err := strconv.Atoi(strings.TrimPrefix(key, "qty_"))
		if err != nil {
			continue
		}
		qty, err := strconv.Atoi(strings.TrimSpace(values[0]))
		if err != nil || qty <= 0 {
			continue
		}
		quantities[lineNo] = qty
	}

	email, _, _ := UserFrom(c)

	posted, err := books.Open(a.store, scope).CreditNote(c.Request.Context(), books.CreditNoteCommand{
		InvoiceID: c.PostForm("invoice_id"), Date: date,
		Reason: c.PostForm("reason"), Actor: email, Quantities: quantities,
	})
	if err != nil {
		a.renderCreditNote(c, http.StatusUnprocessableEntity, creditNoteData{
			Client: clientName, InvoiceID: c.PostForm("invoice_id"),
			Date: c.PostForm("date"), Error: err.Error(),
		})
		return
	}

	c.Redirect(http.StatusSeeOther, "/credit-note?posted="+posted.VoucherNo)
}

func (a *Auth) renderCreditNote(c *gin.Context, status int, data creditNoteData) {
	if data.InvoiceID != "" && data.Lines == nil {
		if scope, _, ok := a.requireScope(c); ok {
			if lines, err := books.Open(a.store, scope).SaleLines(c.Request.Context(), data.InvoiceID); err == nil {
				for _, l := range lines {
					data.Lines = append(data.Lines, creditNoteLine{
						LineNo: l.LineNo, ItemID: l.ItemID, Sold: l.Quantity, Returned: l.Returned,
						Price: format.Money(l.UnitPrice), Rate: l.RateID,
					})
				}
			}
		}
	}

	c.Status(status)
	if err := templates.ExecuteTemplate(c.Writer, "credit_note.html", data); err != nil {
		c.String(http.StatusInternalServerError, "không dựng được trang trả lại: %v", err)
	}
}

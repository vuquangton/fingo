// Purchase bills: a supplier invoice that is a cost, not stock.
package web

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"fingo/internal/books"
	"fingo/internal/format"
	"fingo/internal/money"
)

type purchaseData struct {
	Email     string
	Client    string
	Suppliers []books.Partner
	Expenses  []AccountOption
	Date      string
	Posted    string
	Error     string
}

// ShowPurchase renders the bill form.
func (a *Auth) ShowPurchase(c *gin.Context) {
	scope, clientName, ok := a.requireScope(c)
	if !ok {
		return
	}

	b := books.Open(a.store, scope)

	suppliers, err := b.ListPartners(c.Request.Context())
	if err != nil {
		a.renderPurchase(c, http.StatusInternalServerError, purchaseData{Client: clientName, Error: err.Error()})
		return
	}

	expenses, err := a.accountsOfType(c, "EXPENSE")
	if err != nil {
		a.renderPurchase(c, http.StatusInternalServerError, purchaseData{Client: clientName, Error: err.Error()})
		return
	}

	email, _, _ := UserFrom(c)
	a.renderPurchase(c, http.StatusOK, purchaseData{
		Email:     email,
		Client:    clientName,
		Suppliers: suppliers,
		Expenses:  expenses,
		Date:      time.Now().Format("2006-01-02"),
		Posted:    c.Query("posted"),
	})
}

// PostPurchase records one supplier bill.
func (a *Auth) PostPurchase(c *gin.Context) {
	scope, clientName, ok := a.requireScope(c)
	if !ok {
		return
	}

	date, err := time.Parse("2006-01-02", c.PostForm("date"))
	if err != nil {
		a.renderPurchase(c, http.StatusUnprocessableEntity, purchaseData{Client: clientName, Error: "ngày hóa đơn không hợp lệ"})
		return
	}

	net, err := format.Amount(c.PostForm("net"))
	if err != nil {
		a.renderPurchase(c, http.StatusUnprocessableEntity, purchaseData{Client: clientName, Error: err.Error()})
		return
	}
	if net <= money.Zero {
		a.renderPurchase(c, http.StatusUnprocessableEntity, purchaseData{Client: clientName, Error: "số tiền phải lớn hơn 0"})
		return
	}

	email, _, _ := UserFrom(c)

	posted, err := books.Open(a.store, scope).PurchaseBill(c.Request.Context(), books.PurchaseBillCommand{
		InvoiceID:      uuid.NewString(),
		PartnerID:      c.PostForm("partner_id"),
		ExpenseAccount: c.PostForm("expense_account"),
		VATRateID:      purchaseRateID(c.PostForm("vat_rate")),
		Net:            net,
		Date:           date,
		Actor:          email,
		Description:    c.PostForm("description"),
	})
	if err != nil {
		a.renderPurchase(c, http.StatusUnprocessableEntity, purchaseData{
			Client: clientName,
			Error:  err.Error(),
		})
		return
	}

	c.Redirect(http.StatusSeeOther, "/purchase?posted="+posted.VoucherNo)
}

// purchaseRateID keeps the form honest: an unknown rate is the standard rate, and
// the three statutory categories are the only ones accepted.
func purchaseRateID(raw string) string {
	switch strings.TrimSpace(raw) {
	case "vat0", "vat5", salesVATRateID:
		return strings.TrimSpace(raw)
	default:
		return salesVATRateID
	}
}

func (a *Auth) renderPurchase(c *gin.Context, status int, data purchaseData) {
	if data.Client != "" && (data.Suppliers == nil || data.Expenses == nil) {
		if scope, _, ok := a.requireScope(c); ok {
			b := books.Open(a.store, scope)
			if data.Suppliers == nil {
				data.Suppliers, _ = b.ListPartners(c.Request.Context())
			}
			if data.Expenses == nil {
				data.Expenses, _ = a.accountsOfType(c, "EXPENSE")
			}
		}
	}

	c.Status(status)
	if err := templates.ExecuteTemplate(c.Writer, "purchase.html", data); err != nil {
		c.String(http.StatusInternalServerError, "không dựng được trang mua hàng: %v", err)
	}
}

// accountsOfType returns the postable accounts of one class, for a picker.
func (a *Auth) accountsOfType(c *gin.Context, kind string) ([]AccountOption, error) {
	scope, _, ok := a.requireScope(c)
	if !ok {
		return nil, errors.New("no client scope")
	}

	var out []AccountOption

	err := a.store.Read(c.Request.Context(), scope, func(tx pgx.Tx) error {
		rows, err := tx.Query(c.Request.Context(), `
			SELECT a.code, a.name
			  FROM accounts a
			 WHERE a.active
			   AND a.account_type = $1
			   AND NOT EXISTS (SELECT 1 FROM accounts child WHERE child.parent_code = a.code)
			 ORDER BY a.code`, kind)
		if err != nil {
			return err
		}
		defer rows.Close()

		for rows.Next() {
			var o AccountOption
			if err := rows.Scan(&o.Code, &o.Name); err != nil {
				return err
			}
			out = append(out, o)
		}
		return rows.Err()
	})

	return out, err
}

var _ = context.Background

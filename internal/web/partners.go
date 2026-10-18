// Partners: customers and suppliers.
//
// This is the first master-data screen, and it exists because a sale cannot post
// without naming who owes it. The list and the form are one page: an operator
// adding a customer is already looking at the list they want it to appear in.
package web

import (
	"net/http"
	"net/url"

	"github.com/gin-gonic/gin"

	"fingo/internal/books"
)

type partnerRow struct {
	Name  string
	TaxID string
	Kind  string
}

type partnersData struct {
	Email   string
	Client  string
	Rows    []partnerRow
	Error   string
	Created string
}

// ShowPartners lists partners and offers the add form.
func (a *Auth) ShowPartners(c *gin.Context) {
	scope, clientName, ok := a.requireScope(c)
	if !ok {
		return
	}

	partners, err := books.Open(a.store, scope).ListPartners(c.Request.Context())
	if err != nil {
		a.renderPartners(c, http.StatusInternalServerError, partnersData{Client: clientName, Error: err.Error()})
		return
	}

	rows := make([]partnerRow, 0, len(partners))
	for _, p := range partners {
		kind := "Khách hàng"
		switch p.Kind {
		case books.PartnerSupplier:
			kind = "Nhà cung cấp"
		case books.PartnerBoth:
			kind = "Cả hai"
		}
		rows = append(rows, partnerRow{Name: p.Name, TaxID: p.TaxID, Kind: kind})
	}

	email, _, _ := UserFrom(c)
	a.renderPartners(c, http.StatusOK, partnersData{
		Email:   email,
		Client:  clientName,
		Rows:    rows,
		Created: c.Query("created"),
	})
}

// PostPartner adds one partner.
func (a *Auth) PostPartner(c *gin.Context) {
	scope, clientName, ok := a.requireScope(c)
	if !ok {
		return
	}

	p, err := books.Open(a.store, scope).CreatePartner(c.Request.Context(), books.Partner{
		Name:  c.PostForm("name"),
		TaxID: c.PostForm("tax_id"),
		Kind:  books.PartnerKind(c.PostForm("kind")),
	})
	if err != nil {
		a.renderPartners(c, http.StatusUnprocessableEntity, partnersData{
			Client: clientName,
			Error:  err.Error(),
		})
		return
	}

	// The name is Vietnamese text in a query string: it must be escaped, or the
	// redirect target is not a valid URL and the browser sees a 400.
	c.Redirect(http.StatusSeeOther, "/partners?created="+url.QueryEscape(p.Name))
}

func (a *Auth) renderPartners(c *gin.Context, status int, data partnersData) {
	c.Status(status)
	if err := templates.ExecuteTemplate(c.Writer, "partners.html", data); err != nil {
		c.String(http.StatusInternalServerError, "không dựng được trang danh mục: %v", err)
	}
}

// Reversal screen: correct a posted voucher with its mirror.
package web

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"fingo/internal/books"
)

type reverseData struct {
	Email   string
	Client  string
	Date    string
	Voucher string
	Posted  string
	Error   string
}

// ShowReverse renders the reversal form.
func (a *Auth) ShowReverse(c *gin.Context) {
	_, clientName, ok := a.requireScope(c)
	if !ok {
		return
	}

	email, _, _ := UserFrom(c)
	a.renderReverse(c, http.StatusOK, reverseData{
		Email:  email,
		Client: clientName,
		Date:   time.Now().Format("2006-01-02"),
		Posted: c.Query("posted"),
	})
}

// PostReverse posts the mirror of a voucher found by its number.
func (a *Auth) PostReverse(c *gin.Context) {
	scope, clientName, ok := a.requireScope(c)
	if !ok {
		return
	}

	date, err := time.Parse("2006-01-02", c.PostForm("date"))
	if err != nil {
		a.renderReverse(c, http.StatusUnprocessableEntity, reverseData{Client: clientName, Error: "ngày đảo không hợp lệ"})
		return
	}

	email, _, _ := UserFrom(c)

	posted, err := books.Open(a.store, scope).ReverseVoucher(c.Request.Context(),
		c.PostForm("voucher_no"), email, date)
	if err != nil {
		a.renderReverse(c, http.StatusUnprocessableEntity, reverseData{
			Client:  clientName,
			Error:   err.Error(),
			Voucher: c.PostForm("voucher_no"),
			Date:    c.PostForm("date"),
		})
		return
	}

	c.Redirect(http.StatusSeeOther, "/reverse?posted="+posted.VoucherNo)
}

func (a *Auth) renderReverse(c *gin.Context, status int, data reverseData) {
	c.Status(status)
	if err := templates.ExecuteTemplate(c.Writer, "reverse.html", data); err != nil {
		c.String(http.StatusInternalServerError, "không dựng được trang đảo bút toán: %v", err)
	}
}

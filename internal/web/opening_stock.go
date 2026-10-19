// Opening stock: quantities and costs, reconciled to the opening 156 balance.
package web

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"fingo/internal/books"
	"fingo/internal/format"
	"fingo/internal/money"
)

type openingStockData struct {
	Email  string
	Client string
	Date   string
	Total  string
	Error  string
}

func (a *Auth) ShowOpeningStock(c *gin.Context) {
	_, clientName, ok := a.requireScope(c)
	if !ok {
		return
	}

	email, _, _ := UserFrom(c)
	a.renderOpeningStock(c, http.StatusOK, openingStockData{
		Email: email, Client: clientName, Date: "2026-01-01",
	})
}

func (a *Auth) PostOpeningStock(c *gin.Context) {
	scope, clientName, ok := a.requireScope(c)
	if !ok {
		return
	}

	date, err := time.Parse("2006-01-02", c.PostForm("date"))
	if err != nil {
		a.renderOpeningStock(c, http.StatusUnprocessableEntity, openingStockData{Client: clientName, Error: "ngày không hợp lệ"})
		return
	}

	skus := c.PostFormArray("sku")
	names := c.PostFormArray("name")
	units := c.PostFormArray("unit")
	qtys := c.PostFormArray("qty")
	costs := c.PostFormArray("unit_cost")

	var lines []books.OpeningStockLine
	for i := range skus {
		if strings.TrimSpace(skus[i]) == "" {
			continue
		}
		qty, err := strconv.Atoi(strings.TrimSpace(pick(qtys, i)))
		if err != nil {
			a.renderOpeningStock(c, http.StatusUnprocessableEntity, openingStockData{Client: clientName, Error: "số lượng không hợp lệ"})
			return
		}
		cost, err := format.Amount(pick(costs, i))
		if err != nil {
			a.renderOpeningStock(c, http.StatusUnprocessableEntity, openingStockData{Client: clientName, Error: err.Error()})
			return
		}
		lines = append(lines, books.OpeningStockLine{
			SKU: pick(skus, i), Name: pick(names, i), Unit: pick(units, i), Qty: qty, UnitCost: cost,
		})
	}

	total, err := books.Open(a.store, scope).ImportOpeningStock(c.Request.Context(), date, lines)
	if err != nil {
		a.renderOpeningStock(c, http.StatusUnprocessableEntity, openingStockData{
			Client: clientName, Date: c.PostForm("date"), Error: err.Error(),
		})
		return
	}

	c.Redirect(http.StatusSeeOther, "/opening/stock?posted="+format.Money(total))
}

func pick(values []string, i int) string {
	if i < len(values) {
		return strings.TrimSpace(values[i])
	}
	return ""
}

func (a *Auth) renderOpeningStock(c *gin.Context, status int, data openingStockData) {
	c.Status(status)
	if err := templates.ExecuteTemplate(c.Writer, "opening_stock.html", data); err != nil {
		c.String(http.StatusInternalServerError, "không dựng được trang số dư kho: %v", err)
	}
}

var _ = money.Zero

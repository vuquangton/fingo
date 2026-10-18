// Stock: items, what is on hand, and goods receipts.
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

type stockData struct {
	Email  string
	Client string
	Items  []books.Item
	Date   string
	Posted string
	Error  string
}

// ShowStock lists items with what is on hand and what it is worth.
func (a *Auth) ShowStock(c *gin.Context) {
	scope, clientName, ok := a.requireScope(c)
	if !ok {
		return
	}

	items, err := books.Open(a.store, scope).ListItems(c.Request.Context())
	if err != nil {
		a.renderStock(c, http.StatusInternalServerError, stockData{Client: clientName, Error: err.Error()})
		return
	}

	email, _, _ := UserFrom(c)
	a.renderStock(c, http.StatusOK, stockData{
		Email:  email,
		Client: clientName,
		Items:  items,
		Date:   time.Now().Format("2006-01-02"),
		Posted: c.Query("posted"),
	})
}

// PostItem stocks a new item.
func (a *Auth) PostItem(c *gin.Context) {
	scope, clientName, ok := a.requireScope(c)
	if !ok {
		return
	}

	reorder, _ := strconv.Atoi(strings.TrimSpace(c.PostForm("reorder_point")))

	item, err := books.Open(a.store, scope).CreateItem(c.Request.Context(), books.Item{
		SKU:          c.PostForm("sku"),
		Name:         c.PostForm("name"),
		Unit:         c.PostForm("unit"),
		ReorderPoint: reorder,
	})
	if err != nil {
		a.renderStock(c, http.StatusUnprocessableEntity, stockData{Client: clientName, Error: err.Error()})
		return
	}

	c.Redirect(http.StatusSeeOther, "/stock?posted="+item.SKU)
}

// PostStocktake brings one item to what a physical count found.
func (a *Auth) PostStocktake(c *gin.Context) {
	scope, clientName, ok := a.requireScope(c)
	if !ok {
		return
	}

	date, err := time.Parse("2006-01-02", c.PostForm("date"))
	if err != nil {
		a.renderStock(c, http.StatusUnprocessableEntity, stockData{Client: clientName, Error: "ngày kiểm kê không hợp lệ"})
		return
	}

	counted, err := strconv.Atoi(strings.TrimSpace(c.PostForm("counted")))
	if err != nil || counted < 0 {
		a.renderStock(c, http.StatusUnprocessableEntity, stockData{Client: clientName, Error: "số lượng kiểm kê không hợp lệ"})
		return
	}

	email, _, _ := UserFrom(c)

	posted, err := books.Open(a.store, scope).AdjustStock(c.Request.Context(), books.StocktakeCommand{
		ItemID:   c.PostForm("item_id"),
		Counted:  counted,
		Date:     date,
		Reason:   c.PostForm("reason"),
		Actor:    email,
		SourceID: strings.TrimSpace(c.PostForm("source_doc")),
	})
	if err != nil {
		a.renderStock(c, http.StatusUnprocessableEntity, stockData{Client: clientName, Error: err.Error()})
		return
	}

	c.Redirect(http.StatusSeeOther, "/stock?posted="+posted.VoucherNo)
}

// PostStockReceipt books goods into stock and posts Dr 156 / Cr 331.
func (a *Auth) PostStockReceipt(c *gin.Context) {
	scope, clientName, ok := a.requireScope(c)
	if !ok {
		return
	}

	date, err := time.Parse("2006-01-02", c.PostForm("date"))
	if err != nil {
		a.renderStock(c, http.StatusUnprocessableEntity, stockData{Client: clientName, Error: "ngày nhập kho không hợp lệ"})
		return
	}

	qty, err := strconv.Atoi(strings.TrimSpace(c.PostForm("qty")))
	if err != nil || qty <= 0 {
		a.renderStock(c, http.StatusUnprocessableEntity, stockData{Client: clientName, Error: "số lượng phải là số nguyên dương"})
		return
	}

	unitCost, err := format.Amount(c.PostForm("unit_cost"))
	if err != nil {
		a.renderStock(c, http.StatusUnprocessableEntity, stockData{Client: clientName, Error: err.Error()})
		return
	}
	if unitCost <= money.Zero {
		a.renderStock(c, http.StatusUnprocessableEntity, stockData{Client: clientName, Error: "đơn giá nhập phải lớn hơn 0"})
		return
	}

	email, _, _ := UserFrom(c)

	posted, err := books.Open(a.store, scope).ReceiveStock(c.Request.Context(), books.StockReceiptCommand{
		ItemID:      c.PostForm("item_id"),
		Qty:         qty,
		UnitCost:    unitCost,
		Date:        date,
		SourceDocID: strings.TrimSpace(c.PostForm("source_doc")),
		Actor:       email,
	})
	if err != nil {
		a.renderStock(c, http.StatusUnprocessableEntity, stockData{Client: clientName, Error: err.Error()})
		return
	}

	c.Redirect(http.StatusSeeOther, "/stock?posted="+posted.VoucherNo)
}

func (a *Auth) renderStock(c *gin.Context, status int, data stockData) {
	if data.Client != "" && data.Items == nil {
		if scope, _, ok := a.requireScope(c); ok {
			data.Items, _ = books.Open(a.store, scope).ListItems(c.Request.Context())
		}
	}

	c.Status(status)
	if err := templates.ExecuteTemplate(c.Writer, "stock.html", data); err != nil {
		c.String(http.StatusInternalServerError, "không dựng được trang kho: %v", err)
	}
}

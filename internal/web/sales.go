// Sales screen.
//
// A sale is priced, taxed and posted by internal/trading. This file collects the
// form, hands it over, and shows the voucher number. There is no VAT arithmetic
// here: the rate is named by id and the engine decides the tax.
package web

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"fingo/internal/books"
	"fingo/internal/format"
	"fingo/internal/money"
	"fingo/internal/trading"
)

// salesVATRateID is the rate in force: 10%. The registry holds one rate per date
// (see books.salesVATRates), so this is a constant until the engine models the
// full rate menu.
const salesVATRateID = "vat10"

type salesLineRow struct {
	ItemID    string
	Quantity  string
	UnitPrice string
}

type salesData struct {
	Email       string
	Client      string
	Partners    []books.Partner
	Rows        []salesLineRow
	Date        string
	Description string
	PartnerID   string
	Posted      string
	MissingVAT8 bool
	NeedsAck    bool
	Error       string
}

// ShowSales renders the invoice form.
func (a *Auth) ShowSales(c *gin.Context) {
	scope, clientName, ok := a.requireScope(c)
	if !ok {
		return
	}

	b := books.Open(a.store, scope)

	partners, err := b.ListPartners(c.Request.Context())
	if err != nil {
		a.renderSales(c, http.StatusInternalServerError, salesData{Client: clientName, Error: err.Error()})
		return
	}

	missingVAT8, _ := b.HasVATRate(c.Request.Context(), "vat8")
	needsAck, _ := b.UnresolvedVATWindow(c.Request.Context(), time.Now())

	email, _, _ := UserFrom(c)
	a.renderSales(c, http.StatusOK, salesData{
		Email:    email,
		Client:   clientName,
		Partners: partners,
		Date:     time.Now().Format("2006-01-02"),
		Rows: []salesLineRow{
			{Quantity: "1"},
			{Quantity: "1"},
			{Quantity: "1"},
		},
		Posted:      c.Query("posted"),
		MissingVAT8: missingVAT8,
		NeedsAck:    needsAck,
	})
}

// PostSales raises and posts one invoice.
func (a *Auth) PostSales(c *gin.Context) {
	scope, clientName, ok := a.requireScope(c)
	if !ok {
		return
	}

	b := books.Open(a.store, scope)

	period, err := b.EnsureCurrentYear(c.Request.Context(), time.Now())
	if err != nil {
		a.renderSales(c, http.StatusInternalServerError, salesData{Client: clientName, Error: err.Error()})
		return
	}

	cmd, rows, err := parseSale(c, period.ID)
	if err != nil {
		a.renderSales(c, http.StatusUnprocessableEntity, salesData{
			Client: clientName,
			Error:  err.Error(),
			Date:   c.PostForm("date"),
			Rows:   rows,
		})
		return
	}

	// 8% is a statutory rate this catalog cannot express yet. The operator must
	// confirm the invoice was reviewed before it is posted at another rate; the
	// confirmation is an audit fact, not a formality.
	missingVAT8, err := b.HasVATRate(c.Request.Context(), "vat8")
	if err != nil {
		a.renderSales(c, http.StatusInternalServerError, salesData{Client: clientName, Error: err.Error()})
		return
	}
	needsAck, err := b.UnresolvedVATWindow(c.Request.Context(), cmd.Date)
	if err != nil {
		a.renderSales(c, http.StatusInternalServerError, salesData{Client: clientName, Error: err.Error()})
		return
	}
	if needsAck && c.PostForm("vat8_ack") != "1" {
		a.renderSales(c, http.StatusUnprocessableEntity, salesData{
			Client:      clientName,
			Error:       "Chưa cấu hình thuế suất 8%: hãy kiểm tra hóa đơn rồi xác nhận trước khi ghi sổ.",
			Date:        c.PostForm("date"),
			Description: c.PostForm("description"),
			PartnerID:   c.PostForm("partner_id"),
			Rows:        rows,
			MissingVAT8: missingVAT8,
			NeedsAck:    true,
		})
		return
	}

	email, _, _ := UserFrom(c)

	invoice, err := b.SaleWithCost(c.Request.Context(), cmd)
	if err != nil {
		a.renderSales(c, http.StatusUnprocessableEntity, salesData{
			Client:      clientName,
			Error:       err.Error(),
			Date:        c.PostForm("date"),
			Description: c.PostForm("description"),
			PartnerID:   c.PostForm("partner_id"),
			Rows:        rows,
		})
		return
	}

	if needsAck {
		if err := b.RecordVATAcknowledgement(c.Request.Context(), invoice.ID, email, cmd.Date); err != nil {
			a.renderSales(c, http.StatusInternalServerError, salesData{Client: clientName, Error: err.Error()})
			return
		}
	}

	c.Redirect(http.StatusSeeOther, "/sales?posted="+invoice.ID)
}

func parseSale(c *gin.Context, periodID string) (trading.InvoiceCommand, []salesLineRow, error) {
	date, err := time.Parse("2006-01-02", c.PostForm("date"))
	if err != nil {
		return trading.InvoiceCommand{}, nil, fmt.Errorf("ngày hóa đơn không hợp lệ: %w", err)
	}

	partnerID := strings.TrimSpace(c.PostForm("partner_id"))
	if partnerID == "" {
		return trading.InvoiceCommand{}, nil, fmt.Errorf("hãy chọn khách hàng")
	}

	items := c.PostFormArray("item")
	quantities := c.PostFormArray("quantity")
	prices := c.PostFormArray("unit_price")
	vatRates := c.PostFormArray("vat_rate")

	if len(items) != len(quantities) || len(items) != len(prices) {
		return trading.InvoiceCommand{}, nil, fmt.Errorf("form không khớp số dòng")
	}

	var (
		lines []trading.InvoiceLine
		rows  []salesLineRow
	)

	for i := range items {
		row := salesLineRow{ItemID: items[i], Quantity: quantities[i], UnitPrice: prices[i]}
		rows = append(rows, row)

		if strings.TrimSpace(items[i]) == "" && strings.TrimSpace(prices[i]) == "" {
			continue
		}

		quantity, err := strconv.Atoi(strings.TrimSpace(quantities[i]))
		if err != nil || quantity <= 0 {
			return trading.InvoiceCommand{}, rows, fmt.Errorf("dòng %d: số lượng phải là số nguyên dương", i+1)
		}

		price, err := format.Amount(prices[i])
		if err != nil {
			return trading.InvoiceCommand{}, rows, fmt.Errorf("dòng %d: %w", i+1, err)
		}
		if price <= money.Zero {
			return trading.InvoiceCommand{}, rows, fmt.Errorf("dòng %d: đơn giá phải lớn hơn 0", i+1)
		}

		rateID := salesVATRateID
		if i < len(vatRates) && strings.TrimSpace(vatRates[i]) != "" {
			rateID = strings.TrimSpace(vatRates[i])
		}
		switch rateID {
		case "vat0", "vat5", "kct", "exempt", salesVATRateID:
		default:
			return trading.InvoiceCommand{}, rows, fmt.Errorf("dòng %d: thuế suất không hợp lệ", i+1)
		}

		lines = append(lines, trading.InvoiceLine{
			ItemID:    strings.TrimSpace(items[i]),
			Quantity:  quantity,
			UnitPrice: money.New(price.Int64(), "VND"),
			VATRateID: rateID,
		})
	}

	if len(lines) == 0 {
		return trading.InvoiceCommand{}, rows, fmt.Errorf("hóa đơn cần ít nhất một dòng hàng")
	}

	email, _, _ := UserFrom(c)

	return trading.InvoiceCommand{
		InvoiceID:   uuid.NewString(),
		PartnerID:   partnerID,
		Date:        date.UTC(),
		PeriodID:    periodID,
		Description: strings.TrimSpace(c.PostForm("description")),
		Lines:       lines,
		At:          time.Now(),
		By:          email,
	}, rows, nil
}

func (a *Auth) renderSales(c *gin.Context, status int, data salesData) {
	// The partner picker always needs the list, including on the error path.
	if data.Client != "" && data.Partners == nil {
		if scope, _, ok := a.requireScope(c); ok {
			if partners, err := books.Open(a.store, scope).ListPartners(c.Request.Context()); err == nil {
				data.Partners = partners
			}
		}
	}

	c.Status(status)
	if err := templates.ExecuteTemplate(c.Writer, "sales.html", data); err != nil {
		c.String(http.StatusInternalServerError, "không dựng được trang bán hàng: %v", err)
	}
}

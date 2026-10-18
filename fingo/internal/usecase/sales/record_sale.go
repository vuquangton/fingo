// Package sales holds the application services for recording sales.
package sales

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/shopspring/decimal"

	"fingo/internal/domain/document"
	"fingo/internal/domain/gl"
	"fingo/internal/domain/money"
	"fingo/internal/domain/tax"
	"fingo/internal/usecase/ledger"
)

// Account codes from the TT133 chart of accounts — Phụ lục 1 of Circular
// 133/2016/TT-BTC. All five are present in gl.TT133Chart().
//
// Circular 46/2025/TT-BTC lets an enterprise modify its own chart, so these are
// the statutory defaults rather than a guarantee. They become per-client
// configuration before this ships to a real practice.
const (
	accountReceivable = "131"   // Phải thu của khách hàng
	accountRevenue    = "5111"  // Doanh thu bán hàng hóa
	accountOutputVAT  = "33311" // Thuế GTGT đầu ra
	accountCOGS       = "632"   // Giá vốn hàng bán
	accountInventory  = "156"   // Hàng hóa
)

// SaleLine is one line of a sale.
//
// No unit cost. What goods cost is a fact about the warehouse, not something an
// operator types: the cost comes from the moving average of what was actually
// paid, resolved by the StockSource below and re-checked inside the posting
// transaction.
type SaleLine struct {
	ItemCode  string
	Quantity  decimal.Decimal
	UnitPrice decimal.Decimal // exclusive of VAT

	// VAT is the treatment the line is sold under, not a percentage. The
	// percentage is resolved from the schedule for the document's date, so a
	// line cannot carry a rate that is not in force.
	VAT tax.Kind
}

// Command describes a sale to record.
//
// No voucher number: the ledger allocates one when the sale is posted, in the
// sales invoice book and the fiscal year the date falls in.
type Command struct {
	VoucherID string

	// IdempotencyKey names the request, not the attempt. The same key posted
	// twice stores one sale; a fresh key means a genuinely new one. A UI that
	// reuses the key until the request succeeds makes a double-clicked button
	// harmless.
	IdempotencyKey string

	// PartnerID names the customer the sale is owed by. Required: a credit sale
	// with no customer cannot be chased, and TK 131 would carry an amount with
	// nobody's name against it.
	PartnerID string

	// WarehouseCode says which warehouse the goods leave. Required: the cost of
	// goods sold comes from that warehouse's moving average, and a sale that
	// named no warehouse could not produce one.
	WarehouseCode string

	// DocumentReference is the number written on the invoice (số hóa đơn). When
	// set, the sale records the invoice as a business document, which is what
	// makes it ageable and settleable. When empty the sale was not billed on
	// paper — a counter sale, an internal transfer.
	DocumentReference string

	// DueDate is when the invoice falls due. Only meaningful alongside a
	// reference.
	DueDate time.Time

	Description string    // diễn giải — explains the entry in the ledger
	Date        time.Time // business date — posted_date
	At          time.Time // posting instant — posted_at, stored in UTC
	Operator    string
	Lines       []SaleLine
}

// Result reports what was recorded.
type Result struct {
	// VoucherNo is the number the ledger allocated. The caller did not choose
	// it, so it must be reported back — it is what the document is referred to
	// by from here on.
	VoucherNo string

	Net   decimal.Decimal
	VAT   decimal.Decimal
	Gross decimal.Decimal
	COGS  decimal.Decimal
}

// Poster admits a posting to the ledger.
//
// Declared here, where it is used: the sale needs one method, not a repository,
// and it should not know how the ledger stores a voucher or which rules it
// applies.
type Poster interface {
	Post(ctx context.Context, p ledger.Posting) error
}

// StockSource says what taking goods out of a warehouse would cost.
//
// It is a question, not an action: the cost is resolved here so the voucher can
// be built, and the posting transaction re-checks it before writing anything. If
// another posting moved the same stock in between, the cost no longer matches
// and the sale is refused rather than posted at a cost that was true a moment
// ago.
type StockSource interface {
	CostOfIssue(ctx context.Context, itemCode, warehouseCode string, quantity decimal.Decimal) (decimal.Decimal, error)
}

// RecordSale records a credit sale together with its cost of goods.
type RecordSale struct {
	poster   Poster
	stock    StockSource
	vatRates *tax.Schedule
}

// NewRecordSale builds the use case.
//
// The VAT schedule is injected because a sale line names a treatment, and the
// percentage that treatment carries depends on when the document is dated. The
// stock source is injected because the cost of goods sold is a fact about the
// warehouse.
func NewRecordSale(poster Poster, stock StockSource, vatRates *tax.Schedule) *RecordSale {
	return &RecordSale{poster: poster, stock: stock, vatRates: vatRates}
}

// Execute builds, balances and hands the sale to the ledger.
//
// One business event produces one posting: the revenue entry (receivable,
// revenue, output VAT), the cost entry (cost of goods, inventory) and the stock
// that actually leaves the warehouse all travel together, in one transaction.
// They are one event, and two of the three written without the third would leave
// books that disagree with themselves.
func (uc *RecordSale) Execute(ctx context.Context, cmd Command) (Result, error) {
	if err := validate(cmd); err != nil {
		return Result{}, err
	}

	var res Result

	issues := make([]ledger.StockIssue, 0, len(cmd.Lines))

	// Every amount is rounded to whole đồng here, before it becomes a ledger
	// line. VAT is rounded per line, so an invoice total is always the sum of
	// its rounded parts and never disagrees with its own lines.
	//
	// The cost of goods is not supplied by the caller: it is what the warehouse
	// says the goods cost, resolved before the voucher is built so the amount
	// can go on it, and re-checked inside the posting transaction.
	for _, l := range cmd.Lines {
		rate, err := uc.vatRates.Percent(l.VAT, cmd.Date)
		if err != nil {
			return Result{}, fmt.Errorf("sale line %s: %w", l.ItemCode, err)
		}

		cost, err := uc.stock.CostOfIssue(ctx, l.ItemCode, cmd.WarehouseCode, l.Quantity)
		if err != nil {
			return Result{}, fmt.Errorf("sale line %s: %w", l.ItemCode, err)
		}

		net := money.LineNet(l.Quantity, l.UnitPrice)
		vat := money.LineVAT(net, rate)

		res.Net = res.Net.Add(net)
		res.VAT = res.VAT.Add(vat)
		res.COGS = res.COGS.Add(cost)

		issues = append(issues, ledger.StockIssue{
			ItemCode:      l.ItemCode,
			WarehouseCode: cmd.WarehouseCode,
			Quantity:      l.Quantity,
			Cost:          cost,
		})
	}
	res.Gross = res.Net.Add(res.VAT)

	if !res.Gross.IsPositive() {
		return Result{}, errors.New("sale total must be greater than zero")
	}

	v := gl.NewVoucher(cmd.VoucherID, cmd.Description, gl.VoucherTypeSales, cmd.Date)

	add := func(code string, side gl.EntrySide, amount decimal.Decimal) error {
		return v.AddLine(gl.VoucherLine{AccountCode: code, Side: side, Amount: amount})
	}

	// The receivable is owed by a named customer. That is what makes Sổ chi tiết
	// TK 131 possible at all: without it the ledger knows the amount but not who
	// owes it.
	if err := v.AddLine(gl.VoucherLine{
		AccountCode: accountReceivable,
		Side:        gl.SideDebit,
		Amount:      res.Gross,
		PartnerID:   cmd.PartnerID,
	}); err != nil {
		return Result{}, fmt.Errorf("sale %s: receivable: %w", cmd.VoucherID, err)
	}

	if err := add(accountRevenue, gl.SideCredit, res.Net); err != nil {
		return Result{}, fmt.Errorf("sale %s: revenue: %w", cmd.VoucherID, err)
	}
	if res.VAT.IsPositive() {
		if err := add(accountOutputVAT, gl.SideCredit, res.VAT); err != nil {
			return Result{}, fmt.Errorf("sale %s: output VAT: %w", cmd.VoucherID, err)
		}
	}
	if res.COGS.IsPositive() {
		if err := add(accountCOGS, gl.SideDebit, res.COGS); err != nil {
			return Result{}, fmt.Errorf("sale %s: cost of goods: %w", cmd.VoucherID, err)
		}
		if err := add(accountInventory, gl.SideCredit, res.COGS); err != nil {
			return Result{}, fmt.Errorf("sale %s: inventory: %w", cmd.VoucherID, err)
		}
	}

	// The ledger applies the client's rules — chart, periods — allocates the
	// number, marks the voucher posted, and writes the voucher, its lines, the
	// stock movements and the invoice as one transaction.
	posting := ledger.Posting{
		Voucher:     v,
		At:          cmd.At,
		By:          cmd.Operator,
		RequestKey:  cmd.IdempotencyKey,
		StockIssues: issues,
	}

	// A sale that names an invoice number records the invoice in the same
	// transaction as the voucher and the stock. Written separately, a failure
	// between the two would leave a sale nobody can settle or age — the amount
	// sits in TK 131 with no document behind it.
	if reference := strings.TrimSpace(cmd.DocumentReference); reference != "" {
		posting.Document = &document.Document{
			// Derived from the voucher rather than drawn at random, so a retried
			// request produces the same document instead of a second one for the
			// same sale.
			ID:           "doc-" + cmd.VoucherID,
			PartnerID:    cmd.PartnerID,
			VoucherID:    cmd.VoucherID,
			Kind:         document.KindSalesInvoice,
			Reference:    reference,
			DocumentDate: cmd.Date,
			DueDate:      cmd.DueDate,
			Amount:       res.Gross,
		}
	}

	if err := uc.poster.Post(ctx, posting); err != nil {
		return Result{}, fmt.Errorf("post sale %s: %w", cmd.VoucherID, err)
	}

	res.VoucherNo = v.VoucherNo()

	return res, nil
}

func validate(cmd Command) error {
	if cmd.VoucherID == "" {
		return errors.New("sale requires a voucher id")
	}
	if strings.TrimSpace(cmd.Description) == "" {
		return errors.New("sale requires a description")
	}
	if cmd.Operator == "" {
		return errors.New("sale requires an operator")
	}
	if strings.TrimSpace(cmd.PartnerID) == "" {
		return errors.New("a credit sale requires the customer it is owed by")
	}
	if strings.TrimSpace(cmd.WarehouseCode) == "" {
		return errors.New("a sale requires the warehouse the goods leave")
	}
	if len(cmd.Lines) == 0 {
		return errors.New("sale requires at least one line")
	}

	for i, l := range cmd.Lines {
		switch {
		case strings.TrimSpace(l.ItemCode) == "":
			return fmt.Errorf("sale line %d: an item code is required", i+1)
		case !l.Quantity.IsPositive():
			return fmt.Errorf("sale line %d (%s): quantity must be greater than zero", i+1, l.ItemCode)
		case l.UnitPrice.IsNegative():
			return fmt.Errorf("sale line %d (%s): unit price must not be negative", i+1, l.ItemCode)
		}
	}

	return nil
}

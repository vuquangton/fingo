package trading

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	arap "fingo/internal/document"
	"fingo/internal/domain/gl"
	"fingo/internal/inventory"
	"fingo/internal/money"
	"fingo/internal/vat"
)

// Errors a caller may need to tell apart.
var (
	// ErrInvalidOrder is a purchase order that could not be raised.
	ErrInvalidOrder = errors.New("trading: invalid purchase order")

	// ErrInvalidReceipt is a goods receipt that could not be taken in.
	ErrInvalidReceipt = errors.New("trading: invalid goods receipt")

	// ErrInvalidSupplierInvoice is a supplier invoice that could not be
	// recorded.
	ErrInvalidSupplierInvoice = errors.New("trading: invalid supplier invoice")

	// ErrInvalidSupplierPayment is a payment to a supplier that could not be
	// made.
	ErrInvalidSupplierPayment = errors.New("trading: invalid supplier payment")

	// ErrUnknownOrder is a receipt or an invoice against an order the book does
	// not hold.
	ErrUnknownOrder = errors.New("trading: no such purchase order")

	// ErrNotOrdered is a receipt or an invoice for an item nobody ordered.
	//
	// Refused rather than warned about, because there is no price for it: the
	// cost of goods received comes from the order, and an item the order does
	// not name has no cost to come from.
	ErrNotOrdered = errors.New("trading: the item is not on the order")
)

// PurchaseLine is one line of a purchase, as the operator writes it.
//
// The same shape serves the order and the invoice, because they describe the
// same thing: what is being bought, how much of it, at what price, taxed at
// which rate, and which account it lands on.
type PurchaseLine struct {
	ItemID    string
	Quantity  int
	UnitPrice money.Money // exclusive of VAT
	VATRateID string

	// Account is what the line is debited to when the supplier is invoiced:
	// inventory (TK 156) for goods, an expense account for a service.
	//
	// Empty falls back to the account the order named for the same item, and
	// then to the configured inventory account. The order is where a buyer says
	// what something is for, so an invoice that does not repeat it is taken to
	// mean the same thing.
	Account string
}

// InvoicedPurchaseLine is a purchase line as it was priced.
type InvoicedPurchaseLine struct {
	PurchaseLine

	Base  money.Money
	VAT   money.Money
	Total money.Money
}

// PurchaseOrder is what a client has asked a supplier for.
type PurchaseOrder struct {
	ID        string
	PartnerID string // the supplier
	Date      time.Time

	// ExpectedDate is when the goods are due. Required: an order that does not
	// say when it is expected cannot be chased.
	ExpectedDate time.Time

	Description string
	Lines       []PurchaseLine

	// Base, VAT and Total are what the order is expected to cost, summed from
	// its lines.
	Base  money.Money
	VAT   money.Money
	Total money.Money
}

// OrderCommand asks for a purchase order to be raised.
type OrderCommand struct {
	OrderID      string
	PartnerID    string
	Date         time.Time
	ExpectedDate time.Time
	Description  string
	Lines        []PurchaseLine
}

// GoodsReceiptLine is what actually arrived.
//
// No price: goods are received at what they were ordered at. A supplier who
// invoices a different price is a price difference between the receipt and the
// invoice, which is a fact about the money and not about the goods, and it
// belongs on the invoice.
type GoodsReceiptLine struct {
	ItemID   string
	Quantity int
}

// GoodsReceipt is what arrived against an order.
type GoodsReceipt struct {
	ID      string
	OrderID string
	Date    time.Time
	Lines   []GoodsReceiptLine

	// Value is what the goods received were worth, at the ordered prices. It is
	// also what went into the warehouse: the stock ledger holds the same
	// figure.
	Value money.Money
}

// ReceiptCommand asks for goods to be taken into the warehouse.
type ReceiptCommand struct {
	ReceiptID string
	OrderID   string
	Date      time.Time
	Lines     []GoodsReceiptLine
}

// SupplierInvoiceCommand asks for a supplier's bill to be recorded.
type SupplierInvoiceCommand struct {
	InvoiceID string
	OrderID   string

	Date     time.Time
	PeriodID string

	Description string
	Lines       []PurchaseLine

	At         time.Time
	By         string
	RequestKey string
}

// SupplierInvoice is a supplier's bill that has been recorded and posted.
type SupplierInvoice struct {
	ID        string
	OrderID   string
	PartnerID string
	Date      time.Time
	PeriodID  string

	Lines []InvoicedPurchaseLine

	Base  money.Money
	VAT   money.Money
	Total money.Money

	// Warnings are the three-way mismatches: places where what was ordered, what
	// arrived and what was invoiced disagree. They do not stop the invoice —
	// suppliers under-deliver, over-deliver and bill for things that came in two
	// shipments, and refusing the bill would not make any of that untrue.
	Warnings []MatchWarning

	Voucher *gl.Voucher
}

// MatchWarning is one item where the order, the delivery and the invoice do not
// agree.
type MatchWarning struct {
	ItemID   string
	Ordered  int
	Received int
	Invoiced int

	Message string
}

// String renders the warning as the sentence an operator reads.
func (w MatchWarning) String() string { return w.Message }

// SupplierPaymentCommand asks for a supplier to be paid.
type SupplierPaymentCommand struct {
	PaymentID string
	InvoiceID string
	Amount    money.Money

	// Account is where the money leaves: 112 for a bank, 111 for cash. Empty
	// falls back to the account the flow was configured with.
	Account string

	Date        time.Time
	Description string

	At         time.Time
	By         string
	RequestKey string
}

// SupplierPayment is a supplier payment that has been posted and applied.
type SupplierPayment struct {
	ID        string
	InvoiceID string

	Applied   money.Money
	Unapplied money.Money
	Status    arap.Status

	Voucher *gl.Voucher
}

// PurchaseAccounts are the accounts a purchase flow posts to.
//
// Configured rather than resolved from posting rules, and that is a real
// difference from the sales cycle. posting.Key is (document kind, side, account
// type), which cannot tell inventory (TK 156) from deductible input VAT (TK
// 1331): both are assets debited by the same document. The posting package says
// as much itself — purchases need a further dimension, the role a line plays,
// before they can be keyed that way — and until it has one, the accounts are
// stated here.
type PurchaseAccounts struct {
	// Payable is what the supplier is owed: TK 331.
	Payable string

	// Inventory is the default debit account for goods received: TK 156.
	Inventory string

	// VATInput is deductible input VAT: TK 1331.
	VATInput string

	// Bank is the default account money is paid from: TK 112, or 111 for cash.
	Bank string
}

// Orders keeps the purchase orders and what has been received against them.
//
// Declared here, where it is used. The flow needs to remember what was ordered
// and what arrived; it does not need to know how either is stored, and a
// purchase order that knew how it was written would be able to rewrite itself.
type Orders interface {
	SaveOrder(ctx context.Context, order PurchaseOrder) error
	Order(ctx context.Context, id string) (PurchaseOrder, error)

	SaveReceipt(ctx context.Context, receipt GoodsReceipt) error
	Receipts(ctx context.Context, orderID string) ([]GoodsReceipt, error)
}

// MemoryOrders is an in-memory order book, for tests and for a screen built
// before the schema is.
type MemoryOrders struct {
	orders   map[string]PurchaseOrder
	receipts map[string][]GoodsReceipt
}

// NewMemoryOrders returns an empty order book.
func NewMemoryOrders() *MemoryOrders { return &MemoryOrders{} }

func (m *MemoryOrders) SaveOrder(_ context.Context, order PurchaseOrder) error {
	if m.orders == nil {
		m.orders = make(map[string]PurchaseOrder)
	}
	if _, exists := m.orders[order.ID]; exists {
		return fmt.Errorf("%w: %s", ErrInvalidOrder, order.ID)
	}
	m.orders[order.ID] = order

	return nil
}

func (m *MemoryOrders) Order(_ context.Context, id string) (PurchaseOrder, error) {
	order, ok := m.orders[id]
	if !ok {
		return PurchaseOrder{}, fmt.Errorf("%w: %s", ErrUnknownOrder, id)
	}

	return order, nil
}

func (m *MemoryOrders) SaveReceipt(_ context.Context, receipt GoodsReceipt) error {
	if m.receipts == nil {
		m.receipts = make(map[string][]GoodsReceipt)
	}
	m.receipts[receipt.OrderID] = append(m.receipts[receipt.OrderID], receipt)

	return nil
}

func (m *MemoryOrders) Receipts(_ context.Context, orderID string) ([]GoodsReceipt, error) {
	return append([]GoodsReceipt(nil), m.receipts[orderID]...), nil
}

// Purchasing runs a purchase, from the order to the payment.
//
// It shares the receivables book and the poster with the sales cycle — one
// client has one ledger and one book of documents — and keeps its own accounts,
// because a client's sales chart and purchase chart are configured separately.
type Purchasing struct {
	payables arap.Repository
	stock    *inventory.Ledger
	orders   Orders
	poster   Poster
	rates    *vat.Registry
	accounts PurchaseAccounts
}

// NewPurchasing builds the purchase flow.
func NewPurchasing(
	payables arap.Repository,
	stock *inventory.Ledger,
	orders Orders,
	poster Poster,
	rates *vat.Registry,
	accounts PurchaseAccounts,
) (*Purchasing, error) {
	switch {
	case payables == nil:
		return nil, errors.New("trading: the purchase flow needs somewhere to keep payables")
	case stock == nil:
		return nil, errors.New("trading: the purchase flow needs the stock ledger")
	case orders == nil:
		return nil, errors.New("trading: the purchase flow needs an order book")
	case poster == nil:
		return nil, errors.New("trading: the purchase flow needs somewhere to post vouchers")
	case rates == nil:
		return nil, errors.New("trading: the purchase flow needs the VAT rates in force")
	case strings.TrimSpace(accounts.Payable) == "":
		return nil, errors.New("trading: the purchase flow needs the account the supplier is owed on")
	case strings.TrimSpace(accounts.Inventory) == "":
		return nil, errors.New("trading: the purchase flow needs an inventory account")
	case strings.TrimSpace(accounts.VATInput) == "":
		return nil, errors.New("trading: the purchase flow needs the deductible VAT account")
	case strings.TrimSpace(accounts.Bank) == "":
		return nil, errors.New("trading: the purchase flow needs an account to pay from")
	}

	return &Purchasing{
		payables: payables,
		stock:    stock,
		orders:   orders,
		poster:   poster,
		rates:    rates,
		accounts: accounts,
	}, nil
}

// Order prices what is being asked for and records it.
//
// An order posts nothing. It is a promise, not an event: no money has moved and
// no goods have arrived, and an entry in the ledger for either would be a claim
// about the future.
func (p *Purchasing) Order(ctx context.Context, cmd OrderCommand) (PurchaseOrder, error) {
	if err := cmd.validate(); err != nil {
		return PurchaseOrder{}, err
	}

	amounts, sum, err := priceLines(p.rates, sourcesOf(cmd.Lines))
	if err != nil {
		return PurchaseOrder{}, fmt.Errorf("%w: %w", ErrInvalidOrder, err)
	}
	_ = amounts

	order := PurchaseOrder{
		ID:           cmd.OrderID,
		PartnerID:    cmd.PartnerID,
		Date:         cmd.Date,
		ExpectedDate: cmd.ExpectedDate,
		Description:  cmd.Description,
		Lines:        append([]PurchaseLine(nil), cmd.Lines...),
		Base:         sum.Base,
		VAT:          sum.VAT,
		Total:        sum.Total,
	}

	if err := p.orders.SaveOrder(ctx, order); err != nil {
		return PurchaseOrder{}, fmt.Errorf("record purchase order %s: %w", cmd.OrderID, err)
	}

	return order, nil
}

// Receive takes goods into the warehouse against an order.
//
// This is the step that moves stock: one inbound movement per line, at the price
// the order named, sourced to the receipt so the quantity in the warehouse can
// be traced back to the delivery it came from.
//
// No voucher is posted. The supplier invoice is what debits inventory — that is
// the entry the client's books wait for — and posting one here as well would put
// the goods on the balance sheet twice. Between the two the warehouse holds
// stock the ledger has not been told about yet, which is what "goods received
// not invoiced" means and is a real position rather than a mistake.
func (p *Purchasing) Receive(ctx context.Context, cmd ReceiptCommand) (GoodsReceipt, error) {
	if err := cmd.validate(); err != nil {
		return GoodsReceipt{}, err
	}

	order, err := p.orders.Order(ctx, cmd.OrderID)
	if err != nil {
		return GoodsReceipt{}, err
	}

	// Every line is checked before anything moves, so a receipt that names one
	// item the order does not have does not half-arrive.
	type arrival struct {
		line  GoodsReceiptLine
		price money.Money
	}

	arrivals := make([]arrival, 0, len(cmd.Lines))

	for i, line := range cmd.Lines {
		price, ordered := orderPrice(order, line.ItemID)
		if !ordered {
			return GoodsReceipt{}, fmt.Errorf("%w: receipt %s line %d names item %s",
				ErrNotOrdered, cmd.ReceiptID, i+1, line.ItemID)
		}

		// The item has to be something the warehouse keeps. Checked here rather
		// than discovered on the second movement, which would leave the first
		// one already in the ledger.
		if _, err := p.stock.Item(line.ItemID); err != nil {
			return GoodsReceipt{}, fmt.Errorf("receipt %s line %d: %w", cmd.ReceiptID, i+1, err)
		}

		arrivals = append(arrivals, arrival{line: line, price: price})
	}

	receipt := GoodsReceipt{
		ID:      cmd.ReceiptID,
		OrderID: cmd.OrderID,
		Date:    cmd.Date,
		Lines:   append([]GoodsReceiptLine(nil), cmd.Lines...),
	}

	for i, arrival := range arrivals {
		value, err := p.stock.Record(inventory.StockMove{
			// Derived from the receipt and the line, so replaying the same
			// receipt writes the same movement rather than a second one.
			ID:          fmt.Sprintf("%s-%d", cmd.ReceiptID, i+1),
			ItemID:      arrival.line.ItemID,
			Direction:   inventory.DirectionIn,
			Qty:         arrival.line.Quantity,
			UnitCost:    arrival.price,
			Date:        cmd.Date,
			SourceDocID: cmd.ReceiptID,
		})
		if err != nil {
			return GoodsReceipt{}, fmt.Errorf("receipt %s line %d: %w", cmd.ReceiptID, i+1, err)
		}

		if receipt.Value, err = add(receipt.Value, value); err != nil {
			return GoodsReceipt{}, err
		}
	}

	if err := p.orders.SaveReceipt(ctx, receipt); err != nil {
		return GoodsReceipt{}, fmt.Errorf("record goods receipt %s: %w", cmd.ReceiptID, err)
	}

	return receipt, nil
}

// Invoice records the supplier's bill: what was bought, the VAT that can be
// reclaimed, and what is now owed.
//
//	Dr  inventory or expense   the base of each line
//	Dr  VAT input              the tax on it, on its own account
//	Cr  payable                the total
//
// The input VAT is a line of its own rather than part of the cost, because it is
// not a cost: it is reclaimed, and a return that cannot find it cannot claim it.
//
// The invoice is matched three ways against the order and the deliveries, and a
// mismatch is reported rather than refused. See MatchWarning.
func (p *Purchasing) Invoice(ctx context.Context, cmd SupplierInvoiceCommand) (SupplierInvoice, error) {
	if err := cmd.validate(); err != nil {
		return SupplierInvoice{}, err
	}

	order, err := p.orders.Order(ctx, cmd.OrderID)
	if err != nil {
		return SupplierInvoice{}, err
	}

	receipts, err := p.orders.Receipts(ctx, cmd.OrderID)
	if err != nil {
		return SupplierInvoice{}, fmt.Errorf("read what arrived against order %s: %w", cmd.OrderID, err)
	}

	for i, line := range cmd.Lines {
		if _, ordered := orderPrice(order, line.ItemID); !ordered {
			return SupplierInvoice{}, fmt.Errorf("%w: invoice %s line %d bills item %s",
				ErrNotOrdered, cmd.InvoiceID, i+1, line.ItemID)
		}
	}

	amounts, sum, err := priceLines(p.rates, sourcesOf(cmd.Lines))
	if err != nil {
		return SupplierInvoice{}, fmt.Errorf("%w: %w", ErrInvalidSupplierInvoice, err)
	}

	warnings := threeWayMatch(order, receipts, cmd.Lines)

	voucher, err := p.supplierInvoiceVoucher(ctx, cmd, order, amounts, sum)
	if err != nil {
		return SupplierInvoice{}, err
	}

	if err := p.poster.Post(ctx, Posting{
		Voucher:    voucher,
		At:         cmd.At,
		By:         cmd.By,
		RequestKey: cmd.RequestKey,
	}); err != nil {
		return SupplierInvoice{}, fmt.Errorf("post the voucher for supplier invoice %s: %w", cmd.InvoiceID, err)
	}

	payable := arap.Document{
		ID:        cmd.InvoiceID,
		Type:      arap.TypeInvoice,
		PartnerID: order.PartnerID,
		Date:      cmd.Date,
		PeriodID:  cmd.PeriodID,
		VATRateID: singlePurchaseRate(cmd.Lines),
		Total:     sum.Total,
		Status:    arap.StatusOpen,
	}

	if err := p.payables.Create(ctx, payable, payableLines(payable, p.accounts, cmd, order, amounts, sum)); err != nil {
		return SupplierInvoice{}, fmt.Errorf("record the payable for invoice %s: %w", cmd.InvoiceID, err)
	}

	invoiced := make([]InvoicedPurchaseLine, len(cmd.Lines))
	for i, line := range cmd.Lines {
		invoiced[i] = InvoicedPurchaseLine{
			PurchaseLine: line,
			Base:         amounts[i].Base,
			VAT:          amounts[i].VAT,
			Total:        amounts[i].Total,
		}
	}

	return SupplierInvoice{
		ID:        cmd.InvoiceID,
		OrderID:   cmd.OrderID,
		PartnerID: order.PartnerID,
		Date:      cmd.Date,
		PeriodID:  cmd.PeriodID,
		Lines:     invoiced,
		Base:      sum.Base,
		VAT:       sum.VAT,
		Total:     sum.Total,
		Warnings:  warnings,
		Voucher:   voucher,
	}, nil
}

// Pay posts money leaving and applies it to the supplier's invoice.
func (p *Purchasing) Pay(ctx context.Context, cmd SupplierPaymentCommand) (SupplierPayment, error) {
	if err := cmd.validate(); err != nil {
		return SupplierPayment{}, err
	}

	// Read before posting, so an invoice that is not there, or is kept in
	// another currency, is found out before a voucher goes to the ledger.
	invoice, err := p.payables.Get(ctx, cmd.InvoiceID)
	if err != nil {
		return SupplierPayment{}, fmt.Errorf("read supplier invoice %s: %w", cmd.InvoiceID, err)
	}
	if cmd.Amount.Currency() != invoice.Total.Currency() {
		return SupplierPayment{}, fmt.Errorf("%w: invoice %s is kept in %s and payment %s is in %s",
			ErrCurrencyMismatch, cmd.InvoiceID, invoice.Total.Currency(), cmd.PaymentID, cmd.Amount.Currency())
	}

	account := strings.TrimSpace(cmd.Account)
	if account == "" {
		account = p.accounts.Bank
	}

	description := cmd.Description
	if strings.TrimSpace(description) == "" {
		description = fmt.Sprintf("Trả tiền nhà cung cấp — payment %s", cmd.PaymentID)
	}

	voucher := gl.NewVoucher(supplierPaymentVoucherID(cmd.PaymentID), description, gl.VoucherTypeCashPayment, cmd.Date)

	for _, line := range []gl.VoucherLine{
		{
			AccountCode: p.accounts.Payable,
			Side:        gl.SideDebit,
			Amount:      decimalOf(cmd.Amount),
			Note:        "Phải trả cho người bán",
		},
		{
			AccountCode: account,
			Side:        gl.SideCredit,
			Amount:      decimalOf(cmd.Amount),
			Note:        "Tiền trả nhà cung cấp",
		},
	} {
		if err := voucher.AddLine(line); err != nil {
			return SupplierPayment{}, fmt.Errorf("build the voucher for payment %s: %w", cmd.PaymentID, err)
		}
	}

	if err := p.poster.Post(ctx, Posting{
		Voucher:    voucher,
		At:         cmd.At,
		By:         cmd.By,
		RequestKey: cmd.RequestKey,
	}); err != nil {
		return SupplierPayment{}, fmt.Errorf("post the voucher for payment %s: %w", cmd.PaymentID, err)
	}

	result, err := p.payables.Apply(ctx, arap.Application{
		ID:         supplierApplicationID(cmd.PaymentID),
		DocumentID: cmd.InvoiceID,
		PaymentID:  cmd.PaymentID,
		Amount:     cmd.Amount,
		At:         cmd.At,
	})
	if err != nil {
		return SupplierPayment{}, fmt.Errorf("apply payment %s to invoice %s: %w", cmd.PaymentID, cmd.InvoiceID, err)
	}

	payment := SupplierPayment{
		ID:        cmd.PaymentID,
		InvoiceID: cmd.InvoiceID,
		Status:    result.Document.Status,
		Voucher:   voucher,
	}
	if result.Allocation != nil {
		payment.Applied = result.Allocation.Amount
	}
	if result.Credit != nil {
		payment.Unapplied = result.Credit.Amount
	}

	return payment, nil
}

// supplierInvoiceVoucher is the bill: what was bought, the reclaimable tax, and
// what is now owed.
func (p *Purchasing) supplierInvoiceVoucher(
	ctx context.Context,
	cmd SupplierInvoiceCommand,
	order PurchaseOrder,
	amounts []lineAmounts,
	sum totals,
) (*gl.Voucher, error) {
	_ = ctx

	description := cmd.Description
	if strings.TrimSpace(description) == "" {
		description = fmt.Sprintf("Mua hàng — supplier invoice %s", cmd.InvoiceID)
	}

	voucher := gl.NewVoucher(supplierInvoiceVoucherID(cmd.InvoiceID), description, gl.VoucherTypePurchase, cmd.Date)

	for i, line := range cmd.Lines {
		if amounts[i].Base.IsZero() {
			continue
		}

		if err := voucher.AddLine(gl.VoucherLine{
			AccountCode: p.debitAccount(line, order),
			Side:        gl.SideDebit,
			Amount:      decimalOf(amounts[i].Base),
			Note:        fmt.Sprintf("Mua %s", line.ItemID),
		}); err != nil {
			return nil, fmt.Errorf("build the voucher for supplier invoice %s: %w", cmd.InvoiceID, err)
		}
	}

	// Deductible input VAT on its own account: it is reclaimed, so it is not
	// part of what the goods cost.
	if !sum.VAT.IsZero() {
		if err := voucher.AddLine(gl.VoucherLine{
			AccountCode: p.accounts.VATInput,
			Side:        gl.SideDebit,
			Amount:      decimalOf(sum.VAT),
			Note:        "Thuế GTGT được khấu trừ",
		}); err != nil {
			return nil, fmt.Errorf("build the voucher for supplier invoice %s: %w", cmd.InvoiceID, err)
		}
	}

	if err := voucher.AddLine(gl.VoucherLine{
		AccountCode: p.accounts.Payable,
		Side:        gl.SideCredit,
		Amount:      decimalOf(sum.Total),
		PartnerID:   order.PartnerID,
		Note:        "Phải trả cho người bán",
	}); err != nil {
		return nil, fmt.Errorf("build the voucher for supplier invoice %s: %w", cmd.InvoiceID, err)
	}

	return voucher, nil
}

// payableLines is the invoice as the payables book sees it.
//
// It mirrors the voucher account for account, so a credit note built from these
// lines would reverse the ledger exactly — and it carries the base and the VAT
// on each line, which is what a VAT return reads to find the deductible input
// tax separately from the cost.
func payableLines(
	payable arap.Document,
	accounts PurchaseAccounts,
	cmd SupplierInvoiceCommand,
	order PurchaseOrder,
	amounts []lineAmounts,
	sum totals,
) []arap.Line {
	lines := make([]arap.Line, 0, len(cmd.Lines)+2)

	for i, line := range cmd.Lines {
		if amounts[i].Base.IsZero() {
			continue
		}

		account := accounts.Inventory
		if named := strings.TrimSpace(line.Account); named != "" {
			account = named
		} else if ordered := orderAccount(order, line.ItemID); ordered != "" {
			account = ordered
		}

		lines = append(lines, arap.Line{
			DocumentID: payable.ID,
			AccountID:  account,
			Debit:      amounts[i].Base,
			BaseAmount: amounts[i].Base,
			VATAmount:  amounts[i].VAT,
		})
	}

	if !sum.VAT.IsZero() {
		lines = append(lines, arap.Line{
			DocumentID: payable.ID,
			AccountID:  accounts.VATInput,
			Debit:      sum.VAT,
			VATAmount:  sum.VAT,
		})
	}

	lines = append(lines, arap.Line{
		DocumentID: payable.ID,
		AccountID:  accounts.Payable,
		Credit:     sum.Total,
	})

	return lines
}

// threeWayMatch compares what was ordered, what arrived, and what is being
// billed, item by item.
//
// The order comes first because a mismatch is named after the item, and the
// items are walked in a fixed order so two runs of the same invoice produce the
// same warnings in the same order.
func threeWayMatch(order PurchaseOrder, receipts []GoodsReceipt, invoiced []PurchaseLine) []MatchWarning {
	ordered := make(map[string]int)
	for _, line := range order.Lines {
		ordered[line.ItemID] += line.Quantity
	}

	received := make(map[string]int)
	for _, receipt := range receipts {
		for _, line := range receipt.Lines {
			received[line.ItemID] += line.Quantity
		}
	}

	billed := make(map[string]int)
	for _, line := range invoiced {
		billed[line.ItemID] += line.Quantity
	}

	items := make(map[string]bool)
	for item := range ordered {
		items[item] = true
	}
	for item := range received {
		items[item] = true
	}
	for item := range billed {
		items[item] = true
	}

	names := make([]string, 0, len(items))
	for item := range items {
		names = append(names, item)
	}
	sort.Strings(names)

	var warnings []MatchWarning

	for _, item := range names {
		o, r, i := ordered[item], received[item], billed[item]
		if o == r && r == i {
			continue
		}

		warnings = append(warnings, MatchWarning{
			ItemID:   item,
			Ordered:  o,
			Received: r,
			Invoiced: i,
			Message: fmt.Sprintf(
				"item %s: %d ordered, %d received, %d invoiced — the three do not agree",
				item, o, r, i),
		})
	}

	return warnings
}

// orderPrice is what the order said one unit of an item costs.
func orderPrice(order PurchaseOrder, itemID string) (money.Money, bool) {
	for _, line := range order.Lines {
		if line.ItemID == itemID {
			return line.UnitPrice, true
		}
	}

	return money.Money{}, false
}

// orderAccount is the account the order said an item belongs on.
func orderAccount(order PurchaseOrder, itemID string) string {
	for _, line := range order.Lines {
		if line.ItemID == itemID {
			return strings.TrimSpace(line.Account)
		}
	}

	return ""
}

// debitAccount is the account a purchased line lands on.
func (p *Purchasing) debitAccount(line PurchaseLine, order PurchaseOrder) string {
	if named := strings.TrimSpace(line.Account); named != "" {
		return named
	}
	if ordered := orderAccount(order, line.ItemID); ordered != "" {
		return ordered
	}

	return p.accounts.Inventory
}

// sourcesOf converts purchase lines to the shape the pricing shares.
func sourcesOf(lines []PurchaseLine) []sourceLine {
	sources := make([]sourceLine, len(lines))
	for i, line := range lines {
		sources[i] = sourceLine{
			ItemID:    line.ItemID,
			Quantity:  line.Quantity,
			UnitPrice: line.UnitPrice,
			VATRateID: line.VATRateID,
		}
	}

	return sources
}

// singlePurchaseRate reports the rate an invoice was taxed at, when every line
// was taxed at the same one.
func singlePurchaseRate(lines []PurchaseLine) string {
	if len(lines) == 0 {
		return ""
	}

	first := lines[0].VATRateID
	for _, line := range lines[1:] {
		if line.VATRateID != first {
			return ""
		}
	}

	return first
}

func (cmd OrderCommand) validate() error {
	switch {
	case strings.TrimSpace(cmd.OrderID) == "":
		return fmt.Errorf("%w: an order needs an id", ErrInvalidOrder)
	case strings.TrimSpace(cmd.PartnerID) == "":
		return fmt.Errorf("%w: order %s is to nobody", ErrInvalidOrder, cmd.OrderID)
	case cmd.Date.IsZero():
		return fmt.Errorf("%w: order %s has no date", ErrInvalidOrder, cmd.OrderID)
	case cmd.ExpectedDate.IsZero():
		return fmt.Errorf("%w: order %s does not say when the goods are expected", ErrInvalidOrder, cmd.OrderID)
	case cmd.ExpectedDate.Before(cmd.Date):
		return fmt.Errorf("%w: order %s is dated %s and expects the goods on %s",
			ErrInvalidOrder, cmd.OrderID, cmd.Date.Format(time.DateOnly), cmd.ExpectedDate.Format(time.DateOnly))
	case len(cmd.Lines) == 0:
		return fmt.Errorf("%w: order %s has no lines", ErrInvalidOrder, cmd.OrderID)
	}

	seen := make(map[string]bool, len(cmd.Lines))

	for i, line := range cmd.Lines {
		if err := checkPurchaseLine(i, line); err != nil {
			return fmt.Errorf("%w: order %s: %w", ErrInvalidOrder, cmd.OrderID, err)
		}
		// An order that names an item twice has no single answer to "what did
		// we order", and both the receipt's cost and the three-way match need
		// one.
		if seen[line.ItemID] {
			return fmt.Errorf("%w: order %s names item %s twice", ErrInvalidOrder, cmd.OrderID, line.ItemID)
		}
		seen[line.ItemID] = true
	}

	return nil
}

func (cmd ReceiptCommand) validate() error {
	switch {
	case strings.TrimSpace(cmd.ReceiptID) == "":
		return fmt.Errorf("%w: a receipt needs an id", ErrInvalidReceipt)
	case strings.TrimSpace(cmd.OrderID) == "":
		return fmt.Errorf("%w: receipt %s is against no order", ErrInvalidReceipt, cmd.ReceiptID)
	case cmd.Date.IsZero():
		return fmt.Errorf("%w: receipt %s has no date", ErrInvalidReceipt, cmd.ReceiptID)
	case len(cmd.Lines) == 0:
		return fmt.Errorf("%w: receipt %s has no lines", ErrInvalidReceipt, cmd.ReceiptID)
	}

	for i, line := range cmd.Lines {
		switch {
		case strings.TrimSpace(line.ItemID) == "":
			return fmt.Errorf("%w: receipt %s line %d names no item", ErrInvalidReceipt, cmd.ReceiptID, i+1)
		case line.Quantity <= 0:
			return fmt.Errorf("%w: receipt %s line %d is for %d units",
				ErrInvalidReceipt, cmd.ReceiptID, i+1, line.Quantity)
		}
	}

	return nil
}

func (cmd SupplierInvoiceCommand) validate() error {
	switch {
	case strings.TrimSpace(cmd.InvoiceID) == "":
		return fmt.Errorf("%w: an invoice needs an id", ErrInvalidSupplierInvoice)
	case strings.TrimSpace(cmd.OrderID) == "":
		return fmt.Errorf("%w: invoice %s bills against no order", ErrInvalidSupplierInvoice, cmd.InvoiceID)
	case cmd.Date.IsZero():
		return fmt.Errorf("%w: invoice %s has no date", ErrInvalidSupplierInvoice, cmd.InvoiceID)
	case strings.TrimSpace(cmd.PeriodID) == "":
		return fmt.Errorf("%w: invoice %s is in no period", ErrInvalidSupplierInvoice, cmd.InvoiceID)
	case len(cmd.Lines) == 0:
		return fmt.Errorf("%w: invoice %s has no lines", ErrInvalidSupplierInvoice, cmd.InvoiceID)
	}

	for i, line := range cmd.Lines {
		if err := checkPurchaseLine(i, line); err != nil {
			return fmt.Errorf("%w: invoice %s: %w", ErrInvalidSupplierInvoice, cmd.InvoiceID, err)
		}
	}

	return nil
}

// checkPurchaseLine reports whether a line could be bought, whichever document
// it is on.
func checkPurchaseLine(i int, line PurchaseLine) error {
	switch {
	case strings.TrimSpace(line.ItemID) == "":
		return fmt.Errorf("line %d names no item", i+1)
	case line.Quantity <= 0:
		return fmt.Errorf("line %d (%s) is for %d units", i+1, line.ItemID, line.Quantity)
	case !line.UnitPrice.Currency().Valid():
		return fmt.Errorf("line %d (%s) is priced at %s, which names no currency", i+1, line.ItemID, line.UnitPrice)
	case line.UnitPrice.IsNegative():
		return fmt.Errorf("line %d (%s) is priced below nothing", i+1, line.ItemID)
	case strings.TrimSpace(line.VATRateID) == "":
		return fmt.Errorf("line %d (%s) names no VAT rate", i+1, line.ItemID)
	}

	return nil
}

func (cmd SupplierPaymentCommand) validate() error {
	switch {
	case strings.TrimSpace(cmd.PaymentID) == "":
		return fmt.Errorf("%w: a payment needs an id", ErrInvalidSupplierPayment)
	case strings.TrimSpace(cmd.InvoiceID) == "":
		return fmt.Errorf("%w: payment %s pays no invoice", ErrInvalidSupplierPayment, cmd.PaymentID)
	case !cmd.Amount.Currency().Valid():
		return fmt.Errorf("%w: payment %s is %s, which names no currency",
			ErrInvalidSupplierPayment, cmd.PaymentID, cmd.Amount)
	case cmd.Amount.Amount() <= 0:
		return fmt.Errorf("%w: payment %s is %s, and a payment is greater than nothing",
			ErrInvalidSupplierPayment, cmd.PaymentID, cmd.Amount)
	}

	return nil
}

// The ids are derived rather than drawn, so a retried request collides with the
// row it already wrote instead of posting twice.
func supplierInvoiceVoucherID(invoiceID string) string { return "purchase-" + invoiceID }

func supplierPaymentVoucherID(paymentID string) string { return "supplier-payment-" + paymentID }

func supplierApplicationID(paymentID string) string { return "supplier-apply-" + paymentID }

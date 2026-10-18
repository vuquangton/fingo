package trading_test

import (
	"context"
	"errors"
	"testing"

	arap "fingo/internal/document"
	"fingo/internal/domain/gl"
	"fingo/internal/inventory"
	"fingo/internal/money"
	"fingo/internal/trading"
)

var (
	orderDate   = day(2026, 3, 1)
	receiptDate = day(2026, 3, 10)
	billDate    = day(2026, 3, 12)
)

// purchaseAccounts is the chart a purchase posts against.
func purchaseAccounts() trading.PurchaseAccounts {
	return trading.PurchaseAccounts{
		Payable:   "331",
		Inventory: "156",
		VATInput:  "1331",
		Bank:      "112",
	}
}

// stockLedger is a warehouse holding the items the tests order.
func stockLedger(t *testing.T) *inventory.Ledger {
	t.Helper()

	ledger, err := inventory.NewLedger(vnd)
	if err != nil {
		t.Fatalf("NewLedger: %v", err)
	}

	for _, item := range []inventory.Item{
		{ID: "item-1", SKU: "SKU-1", Name: "Hàng hoá 1", Unit: "cái"},
		{ID: "item-2", SKU: "SKU-2", Name: "Hàng hoá 2", Unit: "cái"},
		{ID: "svc-1", SKU: "SVC-1", Name: "Dịch vụ", Unit: "lần"},
	} {
		if err := ledger.AddItem(item); err != nil {
			t.Fatalf("AddItem %s: %v", item.ID, err)
		}
	}

	return ledger
}

// purchasing wires the purchase flow over an in-memory payable book, an
// in-memory order book and a warehouse.
func purchasing(t *testing.T) (*trading.Purchasing, *recordingPoster, arap.Repository, *inventory.Ledger) {
	t.Helper()

	payables := arap.NewMemoryRepository()
	stock := stockLedger(t)
	poster := &recordingPoster{}

	flow, err := trading.NewPurchasing(
		payables, stock, trading.NewMemoryOrders(), poster, vatRates(t), purchaseAccounts())
	if err != nil {
		t.Fatalf("NewPurchasing: %v", err)
	}

	return flow, poster, payables, stock
}

// buy is one purchase line.
func buy(item string, qty int, unitPrice int64, rate string) trading.PurchaseLine {
	return trading.PurchaseLine{
		ItemID:    item,
		Quantity:  qty,
		UnitPrice: cents(unitPrice),
		VATRateID: rate,
	}
}

// orderCmd is an order for the given lines.
func orderCmd(id string, lines ...trading.PurchaseLine) trading.OrderCommand {
	return trading.OrderCommand{
		OrderID:      id,
		PartnerID:    "sup-1",
		Date:         orderDate,
		ExpectedDate: receiptDate,
		Description:  "Đặt hàng nhà cung cấp A",
		Lines:        lines,
	}
}

// place raises an order, failing the test if it is refused.
func place(t *testing.T, flow *trading.Purchasing, cmd trading.OrderCommand) trading.PurchaseOrder {
	t.Helper()

	order, err := flow.Order(context.Background(), cmd)
	if err != nil {
		t.Fatalf("Order: %v", err)
	}

	return order
}

// receive takes goods in, failing the test if it is refused.
func receive(t *testing.T, flow *trading.Purchasing, id, orderID string, lines ...trading.GoodsReceiptLine) trading.GoodsReceipt {
	t.Helper()

	receipt, err := flow.Receive(context.Background(), trading.ReceiptCommand{
		ReceiptID: id,
		OrderID:   orderID,
		Date:      receiptDate,
		Lines:     lines,
	})
	if err != nil {
		t.Fatalf("Receive: %v", err)
	}

	return receipt
}

// arrive is one line of a goods receipt.
func arrive(item string, qty int) trading.GoodsReceiptLine {
	return trading.GoodsReceiptLine{ItemID: item, Quantity: qty}
}

// The whole cycle: an order is placed, the goods arrive, the supplier bills, and
// the supplier is paid.
func TestPurchase_FullCycleFromOrderToPayment(t *testing.T) {
	flow, poster, payables, stock := purchasing(t)
	ctx := context.Background()

	// Order ten at 100, taxed at 10%.
	order := place(t, flow, orderCmd("po-1", buy("item-1", 10, 100, "vat-10")))
	if order.PartnerID != "sup-1" {
		t.Errorf("order is to %q, want the supplier", order.PartnerID)
	}
	if order.Base.Amount() != 1000 || order.VAT.Amount() != 100 || order.Total.Amount() != 1100 {
		t.Fatalf("order = %s + %s = %s, want 1000 + 100 = 1100", order.Base, order.VAT, order.Total)
	}

	// The order posts nothing: it is a promise, not an event.
	if len(poster.posted()) != 0 {
		t.Errorf("placing an order posted %v, want nothing", poster.ids())
	}

	// The goods arrive.
	receipt := receive(t, flow, "gr-1", "po-1", arrive("item-1", 10))
	if receipt.OrderID != "po-1" {
		t.Errorf("receipt is against %q, want the order", receipt.OrderID)
	}
	if receipt.Value.Amount() != 1000 {
		t.Errorf("the goods received are worth %s, want the ordered 1000", receipt.Value)
	}
	if len(poster.posted()) != 0 {
		t.Errorf("receiving goods posted %v, want nothing: the bill is what debits inventory", poster.ids())
	}

	// The supplier bills.
	invoice, err := flow.Invoice(ctx, trading.SupplierInvoiceCommand{
		InvoiceID:   "pinv-1",
		OrderID:     "po-1",
		Date:        billDate,
		PeriodID:    "fy2026",
		Description: "Mua hàng của nhà cung cấp A",
		Lines:       []trading.PurchaseLine{buy("item-1", 10, 100, "vat-10")},
		At:          postedAt,
		By:          "operator-1",
		RequestKey:  "request-pinv-1",
	})
	if err != nil {
		t.Fatalf("Invoice: %v", err)
	}

	if len(invoice.Warnings) != 0 {
		t.Errorf("the three agree, and the match reported %v", invoice.Warnings)
	}
	if invoice.Total.Amount() != 1100 {
		t.Errorf("the bill is for %s, want 1100", invoice.Total)
	}

	// And the supplier is paid.
	payment, err := flow.Pay(ctx, trading.SupplierPaymentCommand{
		PaymentID:   "spay-1",
		InvoiceID:   "pinv-1",
		Amount:      cents(1100),
		Date:        day(2026, 4, 1),
		Description: "Trả tiền nhà cung cấp A",
		At:          postedAt,
		By:          "operator-1",
		RequestKey:  "request-spay-1",
	})
	if err != nil {
		t.Fatalf("Pay: %v", err)
	}

	if payment.Status != arap.StatusPaid {
		t.Errorf("status = %s, want paid", payment.Status)
	}
	if payment.Applied.Amount() != 1100 {
		t.Errorf("applied = %s, want the whole 1100", payment.Applied)
	}

	bill, err := payables.Get(ctx, "pinv-1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if bill.Status != arap.StatusPaid {
		t.Errorf("the stored bill is %s, want paid", bill.Status)
	}
	if bill.Type != arap.TypeInvoice {
		t.Errorf("the payable is a %s, want an invoice", bill.Type)
	}
	if bill.PartnerID != "sup-1" {
		t.Errorf("the payable is owed to %q, want the supplier", bill.PartnerID)
	}
	if !bill.Outstanding().IsZero() {
		t.Errorf("outstanding = %s, want nothing", bill.Outstanding())
	}

	// Three entries in all: the bill and the payment. Not one for the order and
	// not one for the receipt.
	posted := poster.ids()
	if len(posted) != 2 || posted[0] != "purchase-pinv-1" || posted[1] != "supplier-payment-spay-1" {
		t.Errorf("posted %v, want the bill and the payment", posted)
	}

	// The warehouse holds the goods.
	if level, err := stock.Level("item-1"); err != nil || level != 10 {
		t.Errorf("stock level = %d (err %v), want 10", level, err)
	}
}

// Goods arriving is what puts stock in the warehouse, at the price they were
// ordered at.
func TestPurchase_GoodsReceiptIncreasesStock(t *testing.T) {
	flow, _, _, stock := purchasing(t)

	place(t, flow, orderCmd("po-1", buy("item-1", 10, 100, "vat-10"), buy("item-2", 5, 200, "vat-8")))

	receive(t, flow, "gr-1", "po-1", arrive("item-1", 10), arrive("item-2", 5))

	for item, want := range map[string]int{"item-1": 10, "item-2": 5} {
		level, err := stock.Level(item)
		if err != nil {
			t.Fatalf("Level %s: %v", item, err)
		}
		if level != want {
			t.Errorf("stock level of %s = %d, want %d", item, level, want)
		}
	}

	// And each is valued at what the order said it costs.
	first, err := stock.Valuation("item-1")
	if err != nil {
		t.Fatalf("Valuation: %v", err)
	}
	if first.AvgCost != 100 {
		t.Errorf("item-1 is carried at %d a unit, want the ordered 100", first.AvgCost)
	}
	if got := first.Value().Amount(); got != 1000 {
		t.Errorf("item-1 on hand is worth %d, want 1000", got)
	}

	// A second delivery of the same item adds to the holding.
	second := receive(t, flow, "gr-2", "po-1", arrive("item-1", 4))
	if second.Value.Amount() != 400 {
		t.Errorf("the second delivery is worth %s, want 400", second.Value)
	}

	level, err := stock.Level("item-1")
	if err != nil {
		t.Fatalf("Level: %v", err)
	}
	if level != 14 {
		t.Errorf("stock level = %d, want 14", level)
	}
}

// The bill posts what was bought, the tax that can be reclaimed, and what is
// owed — with the input VAT on an account of its own.
func TestPurchase_InputVATIsTrackedSeparately(t *testing.T) {
	flow, poster, payables, _ := purchasing(t)
	ctx := context.Background()

	place(t, flow, orderCmd("po-1", buy("item-1", 10, 100, "vat-10")))
	receive(t, flow, "gr-1", "po-1", arrive("item-1", 10))

	invoice, err := flow.Invoice(ctx, trading.SupplierInvoiceCommand{
		InvoiceID: "pinv-1", OrderID: "po-1", Date: billDate, PeriodID: "fy2026",
		Lines: []trading.PurchaseLine{buy("item-1", 10, 100, "vat-10")},
		At:    postedAt, By: "operator-1",
	})
	if err != nil {
		t.Fatalf("Invoice: %v", err)
	}

	if invoice.VAT.Amount() != 100 {
		t.Errorf("input VAT = %s, want 100", invoice.VAT)
	}

	voucher := poster.voucher(t, "purchase-pinv-1")

	if err := voucher.ValidateBalance(); err != nil {
		t.Fatalf("the purchase voucher does not balance: %v", err)
	}
	if voucher.VoucherType() != gl.VoucherTypePurchase {
		t.Errorf("voucher type = %s, want a purchase invoice", voucher.VoucherType())
	}

	// The cost of the goods is 1000 and the tax on it is 100, on two accounts:
	// the tax is reclaimed, so it is not part of what the goods cost.
	stock := lineFor(t, voucher, "156")
	if stock.Side != gl.SideDebit || stock.Amount.String() != "1000" {
		t.Errorf("156 = %s %s, want a debit of 1000", stock.Side, stock.Amount)
	}

	vat := lineFor(t, voucher, "1331")
	if vat.Side != gl.SideDebit || vat.Amount.String() != "100" {
		t.Errorf("1331 = %s %s, want a debit of 100", vat.Side, vat.Amount)
	}

	payable := lineFor(t, voucher, "331")
	if payable.Side != gl.SideCredit || payable.Amount.String() != "1100" {
		t.Errorf("331 = %s %s, want a credit of 1100", payable.Side, payable.Amount)
	}
	if payable.PartnerID != "sup-1" {
		t.Errorf("the payable names %q, want the supplier it is owed to", payable.PartnerID)
	}

	// The payable book carries the same split, so a VAT return can find the
	// deductible tax without re-reading the voucher.
	lines, err := payables.Lines(ctx, "pinv-1")
	if err != nil {
		t.Fatalf("Lines: %v", err)
	}

	var foundVAT, foundBase bool
	for _, line := range lines {
		if line.AccountID == "1331" {
			foundVAT = true
			if line.VATAmount.Amount() != 100 {
				t.Errorf("the 1331 line records %s of input VAT, want 100", line.VATAmount)
			}
		}
		if line.AccountID == "156" {
			foundBase = true
			if line.BaseAmount.Amount() != 1000 {
				t.Errorf("the 156 line records a base of %s, want 1000", line.BaseAmount)
			}
			if line.VATAmount.Amount() != 100 {
				t.Errorf("the 156 line records %s of tax, want 100", line.VATAmount)
			}
		}
	}
	if !foundVAT || !foundBase {
		t.Errorf("the payable is missing its cost or its tax line: %+v", lines)
	}
}

// The order, the delivery and the bill are compared item by item, and a
// disagreement is reported rather than refused.
func TestPurchase_ThreeWayMatchWarnsWhenTheThreeDisagree(t *testing.T) {
	flow, poster, payables, _ := purchasing(t)
	ctx := context.Background()

	place(t, flow, orderCmd("po-1", buy("item-1", 10, 100, "vat-10")))
	receive(t, flow, "gr-1", "po-1", arrive("item-1", 8))

	// The supplier bills for ten and only eight arrived.
	invoice, err := flow.Invoice(ctx, trading.SupplierInvoiceCommand{
		InvoiceID: "pinv-1", OrderID: "po-1", Date: billDate, PeriodID: "fy2026",
		Lines: []trading.PurchaseLine{buy("item-1", 10, 100, "vat-10")},
		At:    postedAt, By: "operator-1",
	})
	if err != nil {
		t.Fatalf("a mismatch must not refuse the bill: %v", err)
	}

	if len(invoice.Warnings) != 1 {
		t.Fatalf("warnings = %+v, want the one item that disagrees", invoice.Warnings)
	}

	warning := invoice.Warnings[0]
	if warning.ItemID != "item-1" {
		t.Errorf("warning names %q, want item-1", warning.ItemID)
	}
	if warning.Ordered != 10 || warning.Received != 8 || warning.Invoiced != 10 {
		t.Errorf("warning = %d ordered, %d received, %d invoiced; want 10, 8, 10",
			warning.Ordered, warning.Received, warning.Invoiced)
	}
	if warning.Message == "" {
		t.Error("a warning an operator cannot read is not a warning")
	}

	// The bill still posted, and is still owed.
	if len(poster.posted()) != 1 {
		t.Errorf("posted %v, want the bill", poster.ids())
	}
	bill, err := payables.Get(ctx, "pinv-1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if bill.Status != arap.StatusOpen || bill.Total.Amount() != 1100 {
		t.Errorf("the bill = %s/%s, want 1100 open", bill.Total, bill.Status)
	}
}

// A bill for something nobody ordered is refused, because there is no ordered
// price for the cost to come from.
func TestPurchase_RefusesABillForSomethingNotOrdered(t *testing.T) {
	flow, _, _, _ := purchasing(t)
	ctx := context.Background()

	place(t, flow, orderCmd("po-1", buy("item-1", 10, 100, "vat-10")))

	_, err := flow.Invoice(ctx, trading.SupplierInvoiceCommand{
		InvoiceID: "pinv-1", OrderID: "po-1", Date: billDate, PeriodID: "fy2026",
		Lines: []trading.PurchaseLine{buy("item-2", 1, 500, "vat-10")},
		At:    postedAt, By: "operator-1",
	})
	if !errors.Is(err, trading.ErrNotOrdered) {
		t.Fatalf("billing for an unordered item = %v, want ErrNotOrdered", err)
	}
}

// A receipt that could not be taken in full takes nothing in: the whole receipt
// is checked before the first movement is written.
func TestPurchase_RefusesAReceiptItCannotTakeWhole(t *testing.T) {
	flow, _, _, stock := purchasing(t)
	ctx := context.Background()

	place(t, flow, orderCmd("po-1", buy("item-1", 10, 100, "vat-10")))

	// The second line is for an item the order does not name.
	_, err := flow.Receive(ctx, trading.ReceiptCommand{
		ReceiptID: "gr-1", OrderID: "po-1", Date: receiptDate,
		Lines: []trading.GoodsReceiptLine{arrive("item-1", 10), arrive("item-2", 1)},
	})
	if !errors.Is(err, trading.ErrNotOrdered) {
		t.Fatalf("receipt of an unordered item = %v, want ErrNotOrdered", err)
	}

	// The first line did not land on the way to finding that out.
	level, err := stock.Level("item-1")
	if err != nil {
		t.Fatalf("Level: %v", err)
	}
	if level != 0 {
		t.Errorf("stock level = %d after a refused receipt, want 0", level)
	}
}

// An item the warehouse does not keep cannot be received, and the refusal leaves
// nothing half-recorded.
func TestPurchase_RefusesToReceiveAnItemTheWarehouseDoesNotKeep(t *testing.T) {
	flow, _, _, stock := purchasing(t)
	ctx := context.Background()

	// item-3 is ordered but was never stocked.
	place(t, flow, orderCmd("po-1", buy("item-1", 4, 100, "vat-10"), buy("item-3", 1, 50, "vat-10")))

	_, err := flow.Receive(ctx, trading.ReceiptCommand{
		ReceiptID: "gr-1", OrderID: "po-1", Date: receiptDate,
		Lines: []trading.GoodsReceiptLine{arrive("item-1", 4), arrive("item-3", 1)},
	})
	if !errors.Is(err, inventory.ErrUnknownItem) {
		t.Fatalf("receiving an item the warehouse does not keep = %v, want ErrUnknownItem", err)
	}

	level, err := stock.Level("item-1")
	if err != nil {
		t.Fatalf("Level: %v", err)
	}
	if level != 0 {
		t.Errorf("stock level = %d, want 0", level)
	}
}

// A service is not stock: it is bought, it is debited to an expense account, and
// no goods arrive.
func TestPurchase_AServiceLineGoesToAnExpenseAccount(t *testing.T) {
	flow, poster, _, _ := purchasing(t)
	ctx := context.Background()

	service := buy("svc-1", 1, 2000, "vat-10")
	service.Account = "642"

	place(t, flow, orderCmd("po-1", service))

	invoice, err := flow.Invoice(ctx, trading.SupplierInvoiceCommand{
		InvoiceID: "pinv-1", OrderID: "po-1", Date: billDate, PeriodID: "fy2026",
		Lines: []trading.PurchaseLine{service},
		At:    postedAt, By: "operator-1",
	})
	if err != nil {
		t.Fatalf("Invoice: %v", err)
	}

	if invoice.Total.Amount() != 2200 {
		t.Errorf("the bill is for %s, want 2000 + 200", invoice.Total)
	}

	voucher := poster.voucher(t, "purchase-pinv-1")
	if err := voucher.ValidateBalance(); err != nil {
		t.Fatalf("the voucher does not balance: %v", err)
	}

	expense := lineFor(t, voucher, "642")
	if expense.Side != gl.SideDebit || expense.Amount.String() != "2000" {
		t.Errorf("642 = %s %s, want a debit of 2000", expense.Side, expense.Amount)
	}
	if lines := voucher.Lines(); len(lines) != 3 {
		t.Errorf("the voucher has %d lines, want the expense, the VAT and the payable", len(lines))
	}
}

// The payment leaves the bank and clears the payable.
func TestPurchase_ThePaymentVoucherBalancesAndClearsThePayable(t *testing.T) {
	flow, poster, _, _ := purchasing(t)
	ctx := context.Background()

	place(t, flow, orderCmd("po-1", buy("item-1", 10, 100, "vat-10")))
	if _, err := flow.Invoice(ctx, trading.SupplierInvoiceCommand{
		InvoiceID: "pinv-1", OrderID: "po-1", Date: billDate, PeriodID: "fy2026",
		Lines: []trading.PurchaseLine{buy("item-1", 10, 100, "vat-10")},
		At:    postedAt, By: "operator-1",
	}); err != nil {
		t.Fatalf("Invoice: %v", err)
	}

	if _, err := flow.Pay(ctx, trading.SupplierPaymentCommand{
		PaymentID: "spay-1", InvoiceID: "pinv-1", Amount: cents(500),
		Date: day(2026, 4, 1), At: postedAt, By: "operator-1",
	}); err != nil {
		t.Fatalf("Pay: %v", err)
	}

	voucher := poster.voucher(t, "supplier-payment-spay-1")

	if err := voucher.ValidateBalance(); err != nil {
		t.Fatalf("the payment voucher does not balance: %v", err)
	}
	if voucher.VoucherType() != gl.VoucherTypeCashPayment {
		t.Errorf("voucher type = %s, want a cash payment", voucher.VoucherType())
	}

	payable := lineFor(t, voucher, "331")
	if payable.Side != gl.SideDebit || payable.Amount.String() != "500" {
		t.Errorf("331 = %s %s, want a debit of 500", payable.Side, payable.Amount)
	}

	bank := lineFor(t, voucher, "112")
	if bank.Side != gl.SideCredit || bank.Amount.String() != "500" {
		t.Errorf("112 = %s %s, want a credit of 500", bank.Side, bank.Amount)
	}
}

// Part of the money leaves the bill partly paid; more than is owed leaves a
// credit the client holds with the supplier.
func TestPurchase_PartialAndOverpayment(t *testing.T) {
	flow, _, payables, _ := purchasing(t)
	ctx := context.Background()

	place(t, flow, orderCmd("po-1", buy("item-1", 10, 100, "vat-10")))
	if _, err := flow.Invoice(ctx, trading.SupplierInvoiceCommand{
		InvoiceID: "pinv-1", OrderID: "po-1", Date: billDate, PeriodID: "fy2026",
		Lines: []trading.PurchaseLine{buy("item-1", 10, 100, "vat-10")},
		At:    postedAt, By: "operator-1",
	}); err != nil {
		t.Fatalf("Invoice: %v", err)
	}

	partial, err := flow.Pay(ctx, trading.SupplierPaymentCommand{
		PaymentID: "spay-1", InvoiceID: "pinv-1", Amount: cents(700),
		Date: day(2026, 4, 1), At: postedAt, By: "operator-1",
	})
	if err != nil {
		t.Fatalf("Pay: %v", err)
	}
	if partial.Status != arap.StatusPartiallyPaid {
		t.Errorf("status = %s, want partially_paid", partial.Status)
	}

	// The rest, and then some.
	over, err := flow.Pay(ctx, trading.SupplierPaymentCommand{
		PaymentID: "spay-2", InvoiceID: "pinv-1", Amount: cents(500),
		Date: day(2026, 4, 2), At: postedAt, By: "operator-1",
	})
	if err != nil {
		t.Fatalf("an overpayment must not be refused: %v", err)
	}
	if over.Status != arap.StatusPaid {
		t.Errorf("status = %s, want paid", over.Status)
	}
	if over.Applied.Amount() != 400 {
		t.Errorf("applied = %s, want the 400 still owed", over.Applied)
	}
	if over.Unapplied.Amount() != 100 {
		t.Errorf("unapplied = %s, want the 100 the supplier now holds", over.Unapplied)
	}

	credits, err := payables.Credits(ctx, "sup-1")
	if err != nil {
		t.Fatalf("Credits: %v", err)
	}
	if len(credits) != 1 || credits[0].Amount.Amount() != 100 {
		t.Fatalf("credits = %+v, want one of 100", credits)
	}
}

// An order that could not be placed is refused, and nothing is recorded.
func TestPurchase_RefusesAnOrderItCannotPrice(t *testing.T) {
	flow, poster, payables, _ := purchasing(t)
	ctx := context.Background()

	late := orderCmd("po-1", buy("item-1", 1, 100, "vat-10"))
	late.ExpectedDate = orderDate.AddDate(0, 0, -1)

	noPartner := orderCmd("po-1", buy("item-1", 1, 100, "vat-10"))
	noPartner.PartnerID = ""

	twice := orderCmd("po-1", buy("item-1", 1, 100, "vat-10"), buy("item-1", 2, 100, "vat-10"))

	cases := map[string]trading.OrderCommand{
		"no id":                {PartnerID: "sup-1", Date: orderDate, ExpectedDate: receiptDate, Lines: []trading.PurchaseLine{buy("item-1", 1, 100, "vat-10")}},
		"no supplier":          noPartner,
		"no date":              {OrderID: "po-1", PartnerID: "sup-1", ExpectedDate: receiptDate, Lines: []trading.PurchaseLine{buy("item-1", 1, 100, "vat-10")}},
		"no expected date":     {OrderID: "po-1", PartnerID: "sup-1", Date: orderDate, Lines: []trading.PurchaseLine{buy("item-1", 1, 100, "vat-10")}},
		"expected in the past": late,
		"no lines":             {OrderID: "po-1", PartnerID: "sup-1", Date: orderDate, ExpectedDate: receiptDate},
		"an item twice":        twice,
		"an unknown VAT rate":  orderCmd("po-1", buy("item-1", 1, 100, "vat-99")),
		"a price of nothing":   orderCmd("po-1", buy("item-1", 1, 0, "vat-10")),
	}

	for name, cmd := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := flow.Order(ctx, cmd); err == nil {
				t.Fatal("the order must be refused")
			}
		})
	}

	if len(poster.posted()) != 0 {
		t.Errorf("a refused order posted %v", poster.ids())
	}
	if _, err := payables.Get(ctx, "po-1"); !errors.Is(err, arap.ErrUnknownDocument) {
		t.Errorf("a refused order reached the payable book: %v", err)
	}
}

// Acting on an order that is not there is reported.
func TestPurchase_ReportsAnUnknownOrder(t *testing.T) {
	flow, _, _, _ := purchasing(t)
	ctx := context.Background()

	if _, err := flow.Receive(ctx, trading.ReceiptCommand{
		ReceiptID: "gr-1", OrderID: "po-nope", Date: receiptDate,
		Lines: []trading.GoodsReceiptLine{arrive("item-1", 1)},
	}); !errors.Is(err, trading.ErrUnknownOrder) {
		t.Errorf("receiving against an order that does not exist = %v, want ErrUnknownOrder", err)
	}

	if _, err := flow.Invoice(ctx, trading.SupplierInvoiceCommand{
		InvoiceID: "pinv-1", OrderID: "po-nope", Date: billDate, PeriodID: "fy2026",
		Lines: []trading.PurchaseLine{buy("item-1", 1, 100, "vat-10")},
		At:    postedAt, By: "operator-1",
	}); !errors.Is(err, trading.ErrUnknownOrder) {
		t.Errorf("billing against an order that does not exist = %v, want ErrUnknownOrder", err)
	}
}

// A payment that could not be made is refused, and a bill that is not there
// cannot be paid.
func TestPurchase_RefusesAPaymentItCannotMake(t *testing.T) {
	flow, poster, _, _ := purchasing(t)
	ctx := context.Background()

	place(t, flow, orderCmd("po-1", buy("item-1", 10, 100, "vat-10")))
	if _, err := flow.Invoice(ctx, trading.SupplierInvoiceCommand{
		InvoiceID: "pinv-1", OrderID: "po-1", Date: billDate, PeriodID: "fy2026",
		Lines: []trading.PurchaseLine{buy("item-1", 10, 100, "vat-10")},
		At:    postedAt, By: "operator-1",
	}); err != nil {
		t.Fatalf("Invoice: %v", err)
	}

	posted := len(poster.posted())

	cases := map[string]trading.SupplierPaymentCommand{
		"no id":       {InvoiceID: "pinv-1", Amount: cents(100)},
		"no invoice":  {PaymentID: "spay-1", Amount: cents(100)},
		"nothing":     {PaymentID: "spay-1", InvoiceID: "pinv-1", Amount: cents(0)},
		"no currency": {PaymentID: "spay-1", InvoiceID: "pinv-1", Amount: money.Money{}},
	}

	for name, cmd := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := flow.Pay(ctx, cmd); err == nil {
				t.Fatal("the payment must be refused")
			}
		})
	}

	if got := len(poster.posted()); got != posted {
		t.Errorf("a refused payment posted a voucher: %d, want %d", got, posted)
	}

	if _, err := flow.Pay(ctx, trading.SupplierPaymentCommand{
		PaymentID: "spay-9", InvoiceID: "pinv-9", Amount: cents(100),
		Date: day(2026, 4, 1), At: postedAt, By: "operator-1",
	}); !errors.Is(err, arap.ErrUnknownDocument) {
		t.Errorf("paying a bill that does not exist = %v, want ErrUnknownDocument", err)
	}
}

func TestNewPurchasing_NeedsEveryPart(t *testing.T) {
	payables := arap.NewMemoryRepository()
	stock := stockLedger(t)
	orders := trading.NewMemoryOrders()
	poster := &recordingPoster{}
	rates := vatRates(t)
	accounts := purchaseAccounts()

	broken := func(mutate func(*trading.PurchaseAccounts)) trading.PurchaseAccounts {
		accounts := purchaseAccounts()
		mutate(&accounts)

		return accounts
	}

	cases := map[string]func() (*trading.Purchasing, error){
		"no payables": func() (*trading.Purchasing, error) {
			return trading.NewPurchasing(nil, stock, orders, poster, rates, accounts)
		},
		"no stock": func() (*trading.Purchasing, error) {
			return trading.NewPurchasing(payables, nil, orders, poster, rates, accounts)
		},
		"no orders": func() (*trading.Purchasing, error) {
			return trading.NewPurchasing(payables, stock, nil, poster, rates, accounts)
		},
		"no poster": func() (*trading.Purchasing, error) {
			return trading.NewPurchasing(payables, stock, orders, nil, rates, accounts)
		},
		"no VAT rates": func() (*trading.Purchasing, error) {
			return trading.NewPurchasing(payables, stock, orders, poster, nil, accounts)
		},
		"no payable account": func() (*trading.Purchasing, error) {
			return trading.NewPurchasing(payables, stock, orders, poster, rates, broken(func(a *trading.PurchaseAccounts) { a.Payable = "" }))
		},
		"no inventory account": func() (*trading.Purchasing, error) {
			return trading.NewPurchasing(payables, stock, orders, poster, rates, broken(func(a *trading.PurchaseAccounts) { a.Inventory = "" }))
		},
		"no VAT account": func() (*trading.Purchasing, error) {
			return trading.NewPurchasing(payables, stock, orders, poster, rates, broken(func(a *trading.PurchaseAccounts) { a.VATInput = "" }))
		},
		"no bank account": func() (*trading.Purchasing, error) {
			return trading.NewPurchasing(payables, stock, orders, poster, rates, broken(func(a *trading.PurchaseAccounts) { a.Bank = "" }))
		},
	}

	for name, build := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := build(); err == nil {
				t.Fatal("the flow must refuse to be built without it")
			}
		})
	}
}

// An order is not posted, and neither is a receipt: between them nothing has
// happened that the ledger records.
func TestPurchase_PlacingAndReceivingPostNothing(t *testing.T) {
	flow, poster, _, _ := purchasing(t)

	place(t, flow, orderCmd("po-1", buy("item-1", 2, 100, "vat-10")))
	receive(t, flow, "gr-1", "po-1", arrive("item-1", 2))

	if posted := poster.ids(); len(posted) != 0 {
		t.Errorf("posted %v, want nothing until the supplier bills", posted)
	}
}

// The clock is not used to order anything: two receipts on the same day still
// both land.
func TestPurchase_TwoReceiptsOnTheSameDay(t *testing.T) {
	flow, _, _, stock := purchasing(t)

	place(t, flow, orderCmd("po-1", buy("item-1", 10, 100, "vat-10")))

	receive(t, flow, "gr-1", "po-1", arrive("item-1", 3))
	receive(t, flow, "gr-2", "po-1", arrive("item-1", 4))

	level, err := stock.Level("item-1")
	if err != nil {
		t.Fatalf("Level: %v", err)
	}
	if level != 7 {
		t.Errorf("stock level = %d, want 7", level)
	}
}

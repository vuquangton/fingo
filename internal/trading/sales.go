// Package trading runs the two cycles a trading client lives by: selling, and
// buying.
//
// It is the flow, not the rules. An invoice is priced with internal/vat, the
// receivable or payable it creates is kept by internal/document, goods are
// valued by internal/inventory, the accounts a sale posts to come from
// internal/posting, and the entries themselves are the ledger's. What lives
// here is the order those things happen in, and the fact that they happen
// together: a document that reached the ledger and not the sub-ledger is one
// nobody can chase, and one that reached the sub-ledger and not the ledger is a
// debt the books do not admit to.
//
// Sales — Service, one call per step:
//
//	Invoice  price the lines, post Dr AR / Cr revenue / Cr VAT out, record the receivable
//	Pay      post Dr cash or bank / Cr AR, then allocate it against the invoice
//	Credit   mirror what the invoice recorded, post it, and allocate it back
//
// Purchases — Purchasing, the same shape:
//
//	Order    price what is being asked for; posts nothing, because a promise is not an event
//	Receive  move the goods into the warehouse at the ordered price; posts nothing
//	Invoice  post Dr inventory or expense / Dr VAT input / Cr AP, and match the three ways
//	Pay      post Dr AP / Cr bank, then allocate it against the bill
//
// Every amount is money.Money — an integer number of the currency's minor units.
// Nothing here is a float; the conversion to the ledger's decimal happens once,
// at the voucher, and is exact.
package trading

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/shopspring/decimal"

	arap "fingo/internal/document"
	"fingo/internal/domain/document"
	"fingo/internal/domain/gl"
	"fingo/internal/money"
	"fingo/internal/posting"
	"fingo/internal/vat"
)

// Errors a caller may need to tell apart.
var (
	// ErrInvalidInvoice is an invoice that could not be raised: no id, no
	// partner, no period, no lines, or a line that could not be priced.
	ErrInvalidInvoice = errors.New("trading: invalid invoice")

	// ErrInvalidPayment is a payment that could not be taken: no id, no
	// invoice, no amount, or no account for the money to arrive in.
	ErrInvalidPayment = errors.New("trading: invalid payment")

	// ErrInvalidCredit is a credit note that could not be raised.
	ErrInvalidCredit = errors.New("trading: invalid credit note")

	// ErrCurrencyMismatch is a flow whose amounts are in more than one
	// currency. A sale is priced in one currency or it is not a sale.
	ErrCurrencyMismatch = errors.New("trading: amounts are in different currencies")
)

// Poster admits a posting to the ledger: numbered, validated against the
// client's chart and periods, and marked posted.
//
// Declared here, where it is used. The flow needs to hand an entry to the ledger
// and does not need to know how one is numbered or stored — and it must not,
// because a sale that knew how vouchers are written would be a sale that could
// write one itself.
type Poster interface {
	Post(ctx context.Context, p Posting) error
}

// Posting is one voucher on its way to the ledger, with the audit trail around
// it.
//
// Its shape follows ledger.Posting, so a caller wiring this to the real ledger
// writes an adapter of a few lines and nothing more.
type Posting struct {
	Voucher *gl.Voucher
	At      time.Time
	By      string

	// RequestKey names the request this posting answers, so that a retried one
	// does not post twice. Empty for a flow content to rely on the derived
	// voucher ids alone.
	RequestKey string
}

// Service raises invoices, takes payments against them, and credits them.
type Service struct {
	receivables arap.Repository
	poster      Poster
	rates       *vat.Registry
	rules       *posting.RuleSet
}

// NewService builds the flow.
//
// Everything it needs is injected: where receivables are kept, where vouchers go,
// which VAT rates are in force, and which accounts the client posts a sale to.
// None of those is a detail this package may decide for itself.
func NewService(
	receivables arap.Repository,
	poster Poster,
	rates *vat.Registry,
	rules *posting.RuleSet,
) (*Service, error) {
	switch {
	case receivables == nil:
		return nil, errors.New("trading: the flow needs somewhere to keep receivables")
	case poster == nil:
		return nil, errors.New("trading: the flow needs somewhere to post vouchers")
	case rates == nil:
		return nil, errors.New("trading: the flow needs the VAT rates in force")
	case rules == nil:
		return nil, fmt.Errorf("%w: the flow needs the client's posting rules", posting.ErrUndefined)
	}

	return &Service{receivables: receivables, poster: poster, rates: rates, rules: rules}, nil
}

// InvoiceLine is one line of a sale, as the operator writes it.
type InvoiceLine struct {
	ItemID    string
	Quantity  int
	UnitPrice money.Money // exclusive of VAT
	VATRateID string      // the rate row this line is taxed under — never a percentage
}

// InvoicedLine is a line of a sale as it was priced.
type InvoicedLine struct {
	InvoiceLine

	Base  money.Money // quantity × unit price
	VAT   money.Money // the tax that base carries
	Total money.Money // base + VAT
}

// InvoiceCommand asks for a sale to be invoiced.
type InvoiceCommand struct {
	InvoiceID string

	// PartnerID is who owes the invoice. Recorded on the receivable line, which
	// is what makes Sổ chi tiết TK 131 possible at all.
	PartnerID string

	Date     time.Time // the business date of the sale
	PeriodID string    // the accounting period it falls in

	Description string
	Lines       []InvoiceLine

	At         time.Time // the instant it is posted
	By         string    // the operator
	RequestKey string
}

// Invoice is a sale that has been invoiced, posted and recorded.
type Invoice struct {
	ID        string
	PartnerID string
	Date      time.Time
	PeriodID  string

	Lines []InvoicedLine

	// Base, VAT and Total are the invoice's own totals, added up from the lines
	// after each was priced. The total is the sum of the lines and never a
	// figure computed separately from them, so a document cannot disagree with
	// its own detail.
	Base  money.Money
	VAT   money.Money
	Total money.Money

	Voucher *gl.Voucher
}

// PaymentCommand asks for money to be taken against an invoice.
type PaymentCommand struct {
	// PaymentID identifies the payment — the receipt, the bank line, the
	// transaction — and is what the allocation is recorded against.
	PaymentID string

	// InvoiceID is the invoice being paid.
	InvoiceID string

	// Amount is what was received. More than is owed is not an error: the
	// difference becomes a credit the partner holds.
	Amount money.Money

	// Account is the account the money arrived in: 111 for cash, 112 for a
	// bank. Required, and not derivable — the sales cycle knows the receivable
	// but not which till or which bank the customer paid into.
	Account string

	Date        time.Time
	Description string

	At         time.Time
	By         string
	RequestKey string
}

// Payment is a payment that has been posted and applied.
type Payment struct {
	ID        string
	InvoiceID string

	// Applied is what the invoice could absorb, and Unapplied is what it could
	// not — the credit the partner now holds.
	Applied   money.Money
	Unapplied money.Money

	// Status is the invoice's status after the allocation, which is the answer
	// to "is it paid?".
	Status arap.Status

	Voucher *gl.Voucher
}

// CreditCommand asks for the whole of an invoice to be credited back.
type CreditCommand struct {
	CreditID  string
	InvoiceID string

	Date        time.Time
	Description string

	At         time.Time
	By         string
	RequestKey string
}

// CreditNote is an invoice reversed.
type CreditNote struct {
	ID        string
	InvoiceID string

	// Amount is what was credited: the invoice's total, read back from what was
	// actually recorded rather than computed again.
	Amount money.Money

	Status arap.Status

	Voucher *gl.Voucher
}

// Invoice prices a sale, posts the revenue entry, and records the receivable.
//
// The voucher is posted first. The ledger is the book of record and the
// receivable is a sub-ledger derived from it, so an invoice that reached the
// ledger and not the receivables book is missing an ageing row — recoverable,
// and visible. The other order would put a debt in the sub-ledger that the
// ledger never admitted to, which nothing downstream can detect.
//
// A production implementation runs both in one transaction; this package cannot,
// because the ledger and the receivables book are separate seams. The order is
// chosen so that the failure it cannot prevent leaves the smaller hole.
func (s *Service) Invoice(ctx context.Context, cmd InvoiceCommand) (Invoice, error) {
	if err := cmd.validate(); err != nil {
		return Invoice{}, err
	}

	accounts, err := resolveSalesAccounts(s.rules)
	if err != nil {
		return Invoice{}, err
	}

	priced, base, tax, total, err := s.price(cmd.Lines)
	if err != nil {
		return Invoice{}, err
	}

	voucher, err := invoiceVoucher(cmd, accounts, base, tax, total)
	if err != nil {
		return Invoice{}, err
	}

	if err := s.poster.Post(ctx, Posting{
		Voucher:    voucher,
		At:         cmd.At,
		By:         cmd.By,
		RequestKey: cmd.RequestKey,
	}); err != nil {
		return Invoice{}, fmt.Errorf("post the voucher for invoice %s: %w", cmd.InvoiceID, err)
	}

	receivable := arap.Document{
		// The invoice's own id. Not derived, so a caller that has an invoice id
		// can ask the receivables book about it without knowing a convention.
		ID:        cmd.InvoiceID,
		Type:      arap.TypeInvoice,
		PartnerID: cmd.PartnerID,
		Date:      cmd.Date,
		PeriodID:  cmd.PeriodID,
		VATRateID: singleRate(cmd.Lines),
		Total:     total,
		Status:    arap.StatusOpen,
	}

	if err := s.receivables.Create(ctx, receivable, receivableLines(receivable, accounts, base, tax, total)); err != nil {
		return Invoice{}, fmt.Errorf("record the receivable for invoice %s: %w", cmd.InvoiceID, err)
	}

	return Invoice{
		ID:        cmd.InvoiceID,
		PartnerID: cmd.PartnerID,
		Date:      cmd.Date,
		PeriodID:  cmd.PeriodID,
		Lines:     priced,
		Base:      base,
		VAT:       tax,
		Total:     total,
		Voucher:   voucher,
	}, nil
}

// Pay posts the money arriving and applies it to the invoice.
//
// The voucher is posted first, then the allocation is made. That order leaves
// cash on the books that has not been matched to an invoice — a real position an
// operator deals with every month — rather than an invoice marked paid that no
// money ever arrived for.
//
// Paying more than is owed is not an error. The invoice absorbs what it can and
// the rest becomes a credit the partner holds, which is what a customer paying a
// round sum against an untidy invoice leaves behind.
func (s *Service) Pay(ctx context.Context, cmd PaymentCommand) (Payment, error) {
	if err := cmd.validate(); err != nil {
		return Payment{}, err
	}

	accounts, err := resolveSalesAccounts(s.rules)
	if err != nil {
		return Payment{}, err
	}

	// The invoice is read before anything is posted. Whether it exists, and what
	// it is kept in, are both things to know before a voucher goes to the ledger
	// for money paid against it — a receipt for an invoice that is not there is
	// cash the ledger holds and the receivables book cannot explain.
	invoice, err := s.receivables.Get(ctx, cmd.InvoiceID)
	if err != nil {
		return Payment{}, fmt.Errorf("read invoice %s: %w", cmd.InvoiceID, err)
	}
	if cmd.Amount.Currency() != invoice.Total.Currency() {
		return Payment{}, fmt.Errorf("%w: invoice %s is kept in %s and payment %s is in %s",
			ErrCurrencyMismatch, cmd.InvoiceID, invoice.Total.Currency(), cmd.PaymentID, cmd.Amount.Currency())
	}

	voucher, err := paymentVoucher(cmd, accounts)
	if err != nil {
		return Payment{}, err
	}

	if err := s.poster.Post(ctx, Posting{
		Voucher:    voucher,
		At:         cmd.At,
		By:         cmd.By,
		RequestKey: cmd.RequestKey,
	}); err != nil {
		return Payment{}, fmt.Errorf("post the voucher for payment %s: %w", cmd.PaymentID, err)
	}

	result, err := s.receivables.Apply(ctx, arap.Application{
		ID:         paymentApplicationID(cmd.PaymentID),
		DocumentID: cmd.InvoiceID,
		PaymentID:  cmd.PaymentID,
		Amount:     cmd.Amount,
		At:         cmd.At,
	})
	if err != nil {
		return Payment{}, fmt.Errorf("apply payment %s to invoice %s: %w", cmd.PaymentID, cmd.InvoiceID, err)
	}

	payment := Payment{
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

// Credit reverses the whole of an invoice.
//
// The reversal is built from what the invoice actually recorded — the receivable
// and the accounts beside it — and not by pricing the sale again. Re-pricing
// would give a different answer the moment a VAT rate moved or a line was
// corrected, and a credit note that does not cancel its invoice to the minor
// unit is not a credit note.
//
// Crediting part of an invoice is a different and harder act: the VAT on a
// partial credit is not a proportion of the whole, and it needs its own
// reasoning about what was returned. It is not attempted here.
func (s *Service) Credit(ctx context.Context, cmd CreditCommand) (CreditNote, error) {
	if err := cmd.validate(); err != nil {
		return CreditNote{}, err
	}

	recorded, err := s.receivables.Lines(ctx, cmd.InvoiceID)
	if err != nil {
		return CreditNote{}, fmt.Errorf("read what invoice %s recorded: %w", cmd.InvoiceID, err)
	}
	if len(recorded) == 0 {
		return CreditNote{}, fmt.Errorf("%w: invoice %s has nothing recorded against it", ErrInvalidCredit, cmd.InvoiceID)
	}

	voucher, amount, err := reversalVoucher(cmd, recorded)
	if err != nil {
		return CreditNote{}, err
	}

	if err := s.poster.Post(ctx, Posting{
		Voucher:    voucher,
		At:         cmd.At,
		By:         cmd.By,
		RequestKey: cmd.RequestKey,
	}); err != nil {
		return CreditNote{}, fmt.Errorf("post the credit note for invoice %s: %w", cmd.InvoiceID, err)
	}

	result, err := s.receivables.Apply(ctx, arap.Application{
		ID:         creditApplicationID(cmd.CreditID),
		DocumentID: cmd.InvoiceID,
		PaymentID:  cmd.CreditID,
		Amount:     amount,
		At:         cmd.At,
	})
	if err != nil {
		return CreditNote{}, fmt.Errorf("apply credit note %s to invoice %s: %w", cmd.CreditID, cmd.InvoiceID, err)
	}

	return CreditNote{
		ID:        cmd.CreditID,
		InvoiceID: cmd.InvoiceID,
		Amount:    amount,
		Status:    result.Document.Status,
		Voucher:   voucher,
	}, nil
}

// price works out what each line is worth and what the invoice comes to.
//
// The arithmetic is priceLines'; this adapts the invoice's own line type to it
// and back, so the sales cycle and the purchase cycle price a line exactly once
// between them.
func (s *Service) price(lines []InvoiceLine) ([]InvoicedLine, money.Money, money.Money, money.Money, error) {
	sources := make([]sourceLine, len(lines))
	for i, line := range lines {
		sources[i] = sourceLine{
			ItemID:    line.ItemID,
			Quantity:  line.Quantity,
			UnitPrice: line.UnitPrice,
			VATRateID: line.VATRateID,
		}
	}

	amounts, sum, err := priceLines(s.rates, sources)
	if err != nil {
		return nil, money.Money{}, money.Money{}, money.Money{}, fmt.Errorf("%w: %w", ErrInvalidInvoice, err)
	}

	priced := make([]InvoicedLine, len(lines))
	for i, line := range lines {
		priced[i] = InvoicedLine{
			InvoiceLine: line,
			Base:        amounts[i].Base,
			VAT:         amounts[i].VAT,
			Total:       amounts[i].Total,
		}
	}

	return priced, sum.Base, sum.VAT, sum.Total, nil
}

// salesAccounts are the three accounts a sale touches.
type salesAccounts struct {
	receivable string // TK 131 — phải thu của khách hàng
	revenue    string // TK 5111 — doanh thu bán hàng hoá
	vatOutput  string // TK 33311 — thuế GTGT đầu ra
}

func resolveSalesAccounts(rules *posting.RuleSet) (salesAccounts, error) {
	lookup := func(side gl.EntrySide, accountType gl.AccountType) (string, error) {
		code, answered := rules.Lookup(document.KindSalesInvoice, side, accountType)
		if !answered {
			return "", fmt.Errorf("%w: %s/%s/%s",
				posting.ErrUndefined, document.KindSalesInvoice, side, accountType)
		}

		return code, nil
	}

	receivable, err := lookup(gl.SideDebit, gl.AccountTypeAsset)
	if err != nil {
		return salesAccounts{}, err
	}
	revenue, err := lookup(gl.SideCredit, gl.AccountTypeRevenue)
	if err != nil {
		return salesAccounts{}, err
	}
	vatOutput, err := lookup(gl.SideCredit, gl.AccountTypeLiability)
	if err != nil {
		return salesAccounts{}, err
	}

	return salesAccounts{receivable: receivable, revenue: revenue, vatOutput: vatOutput}, nil
}

// invoiceVoucher is the revenue entry: what is owed, what was earned, and the
// tax on it.
func invoiceVoucher(
	cmd InvoiceCommand,
	accounts salesAccounts,
	base, tax, total money.Money,
) (*gl.Voucher, error) {
	description := cmd.Description
	if strings.TrimSpace(description) == "" {
		description = fmt.Sprintf("Bán hàng — invoice %s", cmd.InvoiceID)
	}

	voucher := gl.NewVoucher(invoiceVoucherID(cmd.InvoiceID), description, gl.VoucherTypeSales, cmd.Date)

	// The receivable carries the partner, because "TK 131 is 500,000,000" is a
	// number and "TK 131 from this customer is 500,000,000" is a position
	// somebody can act on.
	lines := []gl.VoucherLine{{
		AccountCode: accounts.receivable,
		Side:        gl.SideDebit,
		Amount:      decimalOf(total),
		PartnerID:   cmd.PartnerID,
		Note:        "Phải thu của khách hàng",
	}}

	if !base.IsZero() {
		lines = append(lines, gl.VoucherLine{
			AccountCode: accounts.revenue,
			Side:        gl.SideCredit,
			Amount:      decimalOf(base),
			Note:        "Doanh thu bán hàng",
		})
	}

	// A zero VAT line is not written. gl refuses a line that moves nothing, and
	// a zero-rated sale genuinely has no output VAT to declare.
	if !tax.IsZero() {
		lines = append(lines, gl.VoucherLine{
			AccountCode: accounts.vatOutput,
			Side:        gl.SideCredit,
			Amount:      decimalOf(tax),
			Note:        "Thuế GTGT đầu ra",
		})
	}

	for _, line := range lines {
		if err := voucher.AddLine(line); err != nil {
			return nil, fmt.Errorf("build the voucher for invoice %s: %w", cmd.InvoiceID, err)
		}
	}

	return voucher, nil
}

// paymentVoucher is money arriving: the till or the bank debited, the receivable
// cleared.
func paymentVoucher(cmd PaymentCommand, accounts salesAccounts) (*gl.Voucher, error) {
	description := cmd.Description
	if strings.TrimSpace(description) == "" {
		description = fmt.Sprintf("Thu tiền — payment %s", cmd.PaymentID)
	}

	// A receipt that settles a receivable is numbered in the cash book, not the
	// sales book: it is a phiếu thu, and giving it an invoice number would put
	// two documents under one series.
	voucher := gl.NewVoucher(paymentVoucherID(cmd.PaymentID), description, gl.VoucherTypeCashReceipt, cmd.Date)

	lines := []gl.VoucherLine{
		{
			AccountCode: cmd.Account,
			Side:        gl.SideDebit,
			Amount:      decimalOf(cmd.Amount),
			Note:        "Tiền thu từ khách hàng",
		},
		{
			AccountCode: accounts.receivable,
			Side:        gl.SideCredit,
			Amount:      decimalOf(cmd.Amount),
			Note:        "Phải thu của khách hàng",
		},
	}

	for _, line := range lines {
		if err := voucher.AddLine(line); err != nil {
			return nil, fmt.Errorf("build the voucher for payment %s: %w", cmd.PaymentID, err)
		}
	}

	return voucher, nil
}

// reversalVoucher mirrors what an invoice recorded, and reports what the
// reversal is worth.
//
// Every side is swapped and every amount is carried across unchanged, so the
// reversal cancels the original to the minor unit whatever the original was.
func reversalVoucher(cmd CreditCommand, recorded []arap.Line) (*gl.Voucher, money.Money, error) {
	description := cmd.Description
	if strings.TrimSpace(description) == "" {
		description = fmt.Sprintf("Giảm giá hàng bán — credit note for invoice %s", cmd.InvoiceID)
	}

	voucher := gl.NewVoucher(creditVoucherID(cmd.CreditID), description, gl.VoucherTypeGeneral, cmd.Date)

	var total money.Money

	for _, line := range recorded {
		amount := line.Debit
		mirrored := gl.SideCredit

		if line.Debit.IsZero() {
			amount = line.Credit
			mirrored = gl.SideDebit
		}

		if amount.IsZero() {
			continue
		}

		if err := voucher.AddLine(gl.VoucherLine{
			AccountCode: line.AccountID,
			Side:        mirrored,
			Amount:      decimalOf(amount),
			Note:        "Đảo bút toán",
		}); err != nil {
			return nil, money.Money{}, fmt.Errorf("build the credit note for invoice %s: %w", cmd.InvoiceID, err)
		}

		// What was debited is what the invoice was worth: the receivable side.
		if line.Debit.IsZero() {
			continue
		}

		sum, err := add(total, amount)
		if err != nil {
			return nil, money.Money{}, err
		}
		total = sum
	}

	if total.IsZero() {
		return nil, money.Money{}, fmt.Errorf("%w: invoice %s recorded nothing to reverse", ErrInvalidCredit, cmd.InvoiceID)
	}

	return voucher, total, nil
}

// receivableLines is the invoice as the receivables book sees it: the same three
// accounts as the voucher, so a credit note built from these lines mirrors the
// ledger exactly.
func receivableLines(
	receivable arap.Document,
	accounts salesAccounts,
	base, tax, total money.Money,
) []arap.Line {
	lines := []arap.Line{
		{DocumentID: receivable.ID, AccountID: accounts.receivable, Debit: total},
		{DocumentID: receivable.ID, AccountID: accounts.revenue, Credit: base},
	}

	if !tax.IsZero() {
		lines = append(lines, arap.Line{DocumentID: receivable.ID, AccountID: accounts.vatOutput, Credit: tax})
	}

	return lines
}

// singleRate reports the rate an invoice was taxed at, when every line was taxed
// at the same one.
//
// Empty otherwise: an invoice with three rates on it has no single rate, and
// writing the first line's would be a claim about the other lines that is not
// true.
func singleRate(lines []InvoiceLine) string {
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

// decimalOf renders a whole number of minor units as the ledger's decimal.
//
// The ledger carries amounts as decimal đồng; this package carries them as
// integers. The conversion is exact in both directions and happens only here,
// which is what keeps the integer discipline from being a claim and makes it a
// boundary.
func decimalOf(amount money.Money) decimal.Decimal {
	return decimal.NewFromInt(amount.Amount())
}

func (cmd InvoiceCommand) validate() error {
	if strings.TrimSpace(cmd.InvoiceID) == "" {
		return fmt.Errorf("%w: an invoice needs an id", ErrInvalidInvoice)
	}
	if strings.TrimSpace(cmd.PartnerID) == "" {
		return fmt.Errorf("%w: invoice %s is owed by nobody", ErrInvalidInvoice, cmd.InvoiceID)
	}
	if cmd.Date.IsZero() {
		return fmt.Errorf("%w: invoice %s has no date", ErrInvalidInvoice, cmd.InvoiceID)
	}
	if strings.TrimSpace(cmd.PeriodID) == "" {
		return fmt.Errorf("%w: invoice %s is in no period", ErrInvalidInvoice, cmd.InvoiceID)
	}
	if len(cmd.Lines) == 0 {
		return fmt.Errorf("%w: invoice %s has no lines", ErrInvalidInvoice, cmd.InvoiceID)
	}

	for i, line := range cmd.Lines {
		switch {
		case strings.TrimSpace(line.ItemID) == "":
			return fmt.Errorf("%w: invoice %s line %d names no item", ErrInvalidInvoice, cmd.InvoiceID, i+1)
		case line.Quantity <= 0:
			return fmt.Errorf("%w: invoice %s line %d is for %d units", ErrInvalidInvoice, cmd.InvoiceID, i+1, line.Quantity)
		case !line.UnitPrice.Currency().Valid():
			return fmt.Errorf("%w: invoice %s line %d is priced at %s, which names no currency",
				ErrInvalidInvoice, cmd.InvoiceID, i+1, line.UnitPrice)
		case line.UnitPrice.IsNegative():
			return fmt.Errorf("%w: invoice %s line %d is priced below nothing", ErrInvalidInvoice, cmd.InvoiceID, i+1)
		case strings.TrimSpace(line.VATRateID) == "":
			return fmt.Errorf("%w: invoice %s line %d names no VAT rate", ErrInvalidInvoice, cmd.InvoiceID, i+1)
		}
	}

	return nil
}

func (cmd PaymentCommand) validate() error {
	switch {
	case strings.TrimSpace(cmd.PaymentID) == "":
		return fmt.Errorf("%w: a payment needs an id", ErrInvalidPayment)
	case strings.TrimSpace(cmd.InvoiceID) == "":
		return fmt.Errorf("%w: payment %s pays no invoice", ErrInvalidPayment, cmd.PaymentID)
	case !cmd.Amount.Currency().Valid():
		return fmt.Errorf("%w: payment %s is %s, which names no currency", ErrInvalidPayment, cmd.PaymentID, cmd.Amount)
	case cmd.Amount.Amount() <= 0:
		return fmt.Errorf("%w: payment %s is %s, and a payment is greater than nothing",
			ErrInvalidPayment, cmd.PaymentID, cmd.Amount)
	case strings.TrimSpace(cmd.Account) == "":
		return fmt.Errorf("%w: payment %s does not say which account the money arrived in",
			ErrInvalidPayment, cmd.PaymentID)
	}

	return nil
}

func (cmd CreditCommand) validate() error {
	switch {
	case strings.TrimSpace(cmd.CreditID) == "":
		return fmt.Errorf("%w: a credit note needs an id", ErrInvalidCredit)
	case strings.TrimSpace(cmd.InvoiceID) == "":
		return fmt.Errorf("%w: credit note %s credits no invoice", ErrInvalidCredit, cmd.CreditID)
	case cmd.Date.IsZero():
		return fmt.Errorf("%w: credit note %s has no date", ErrInvalidCredit, cmd.CreditID)
	}

	return nil
}

// The ids are derived rather than drawn, so a retried request collides with the
// row it already wrote instead of posting twice.
func invoiceVoucherID(invoiceID string) string { return "invoice-" + invoiceID }

func paymentVoucherID(paymentID string) string { return "payment-" + paymentID }

func creditVoucherID(creditID string) string { return "credit-" + creditID }

func paymentApplicationID(paymentID string) string { return "apply-" + paymentID }

func creditApplicationID(creditID string) string { return "credit-apply-" + creditID }

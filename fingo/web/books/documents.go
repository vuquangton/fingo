package books

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/shopspring/decimal"

	"fingo/internal/document"
	domaindoc "fingo/internal/domain/document"
	"fingo/internal/domain/gl"
	"fingo/internal/money"
	"fingo/internal/vat"
	"fingo/web"
)

// Documents lists documents, newest first.
func (b *Books) Documents(ctx context.Context, filter web.DocumentFilter) ([]web.DocumentRow, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	return b.rowsLocked(ctx, filter)
}

func (b *Books) rowsLocked(ctx context.Context, filter web.DocumentFilter) ([]web.DocumentRow, error) {
	want := strings.TrimSpace(filter.Type)

	var out []web.DocumentRow

	if want == "" || want == string(document.TypeInvoice) || want == string(document.TypeReceipt) {
		stored, err := b.docs.List(ctx)
		if err != nil {
			return nil, err
		}

		for _, d := range stored {
			if want != "" && string(d.Type) != want {
				continue
			}

			out = append(out, b.rowOf(d))
		}
	}

	if want == "" || want == string(document.TypeJournal) {
		for _, j := range b.journals {
			out = append(out, web.DocumentRow{
				ID:      j.ID,
				Type:    string(document.TypeJournal),
				Partner: "â€”",
				Date:    j.Date.Format(time.DateOnly),
				Total:   totalOf(j.Lines),
				Status:  "posted",
			})
		}
	}

	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Date != out[j].Date {
			return out[i].Date > out[j].Date
		}

		return out[i].ID > out[j].ID
	})

	return out, nil
}

// rowOf renders one document as a list row.
func (b *Books) rowOf(d document.Document) web.DocumentRow {
	outstanding := d.Total.Sub(d.Allocated)

	return web.DocumentRow{
		ID:          d.ID,
		Type:        string(d.Type),
		Partner:     b.partners[d.ID],
		Date:        d.Date.Format(time.DateOnly),
		Total:       web.NewAmount(d.Total.Amount()),
		Allocated:   web.NewAmount(d.Allocated.Amount()),
		Outstanding: web.NewAmount(outstanding.Amount()),
		Status:      string(d.Status),
	}
}

// totalOf sums a set of lines on their debit side.
//
// A journal's total is what it moves, not what it is worth: both sides are the
// same number and picking either gives it.
func totalOf(lines []entry) web.Amount {
	total := decimal.Zero

	for _, l := range lines {
		if l.Side == gl.SideDebit {
			total = total.Add(l.Amount)
		}
	}

	return web.NewAmount(total.IntPart())
}

// Document returns one document in full.
func (b *Books) Document(ctx context.Context, id string) (web.DocumentPage, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if j, ok := b.journalByID(id); ok {
		return b.journalPage(j), nil
	}

	d, err := b.docs.Get(ctx, id)
	if err != nil {
		return web.DocumentPage{}, err
	}

	lines, err := b.docs.Lines(ctx, id)
	if err != nil {
		return web.DocumentPage{}, err
	}

	allocations, err := b.docs.Allocations(ctx, id)
	if err != nil {
		return web.DocumentPage{}, err
	}

	page := web.DocumentPage{
		DocumentRow: b.rowOf(d),
		Description: b.notes[id],
		Payable: d.Status == document.StatusOpen ||
			d.Status == document.StatusPartiallyPaid,
	}

	if rate, err := b.vat.Get(d.VATRateID); err == nil {
		page.VATRate = percent(rate.Rate)
	}

	for _, l := range lines {
		page.Lines = append(page.Lines, web.DocumentLineRow{
			Account: l.AccountID,
			Debit:   web.NewAmount(l.Debit.Amount()),
			Credit:  web.NewAmount(l.Credit.Amount()),
		})
	}

	for _, a := range allocations {
		page.Allocations = append(page.Allocations, web.AllocationRow{
			Payment: a.PaymentID,
			Date:    a.At.Format(time.DateOnly),
			Amount:  web.NewAmount(a.Amount.Amount()),
		})
	}

	return page, nil
}

// journalByID finds a manual journal entry.
func (b *Books) journalByID(id string) (journal, bool) {
	for _, j := range b.journals {
		if j.ID == id {
			return j, true
		}
	}

	return journal{}, false
}

// journalPage renders a manual journal entry.
func (b *Books) journalPage(j journal) web.DocumentPage {
	page := web.DocumentPage{
		DocumentRow: web.DocumentRow{
			ID:      j.ID,
			Type:    string(document.TypeJournal),
			Partner: "â€”",
			Date:    j.Date.Format(time.DateOnly),
			Total:   totalOf(j.Lines),
			Status:  "posted",
		},
		Description: j.Description,
	}

	for _, l := range j.Lines {
		row := web.DocumentLineRow{Account: l.AccountCode}
		if l.Side == gl.SideDebit {
			row.Debit = web.NewAmount(l.Amount.IntPart())
		} else {
			row.Credit = web.NewAmount(l.Amount.IntPart())
		}

		page.Lines = append(page.Lines, row)
	}

	return page
}

// CreateInvoice records a sales invoice and posts it.
func (b *Books) CreateInvoice(ctx context.Context, in web.InvoiceInput) (string, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	customer := strings.TrimSpace(in.Customer)
	if customer == "" {
		return "", fmt.Errorf("an invoice needs a customer")
	}
	if in.NetAmount <= 0 {
		return "", fmt.Errorf("an invoice needs a value greater than zero")
	}

	p, err := b.periodFor(in.Date)
	if err != nil {
		return "", err
	}

	// The tax is whatever the registry says the named rate charges on this
	// net. It is not computed here: internal/vat owns what tax an amount
	// carries, and a second implementation is a second answer.
	tax := int64(0)

	if in.VATRateID != "" {
		charge, err := b.vat.Charge(vat.Line{NetAmount: in.NetAmount, VATRateID: in.VATRateID})
		if err != nil {
			return "", err
		}

		tax = charge.Tax
	}

	total := in.NetAmount + tax

	rules, err := tt133()
	if err != nil {
		return "", err
	}

	receivable, err := b.resolve(rules, domaindoc.KindSalesInvoice, gl.SideDebit, gl.AccountTypeAsset)
	if err != nil {
		return "", err
	}
	revenue, err := b.resolve(rules, domaindoc.KindSalesInvoice, gl.SideCredit, gl.AccountTypeRevenue)
	if err != nil {
		return "", err
	}

	id := b.nextID("inv")

	lines := []document.Line{{
		DocumentID: id,
		AccountID:  receivable,
		Debit:      money.New(total, currency),
		BaseAmount: money.New(in.NetAmount, currency),
		VATAmount:  money.New(tax, currency),
	}, {
		DocumentID: id,
		AccountID:  revenue,
		Credit:     money.New(in.NetAmount, currency),
	}}

	if tax > 0 {
		outputVAT, err := b.resolve(rules, domaindoc.KindSalesInvoice, gl.SideCredit, gl.AccountTypeLiability)
		if err != nil {
			return "", err
		}

		lines = append(lines, document.Line{
			DocumentID: id,
			AccountID:  outputVAT,
			Credit:     money.New(tax, currency),
		})
	}

	doc := document.Document{
		ID:        id,
		Type:      document.TypeInvoice,
		PartnerID: customer,
		Date:      in.Date,
		PeriodID:  p.ID,
		VATRateID: in.VATRateID,
		Total:     money.New(total, currency),
		Status:    document.StatusOpen,
	}

	if err := b.docs.Create(ctx, doc, lines); err != nil {
		return "", err
	}

	posted := make([]entry, 0, len(lines))

	for _, l := range lines {
		side := gl.SideDebit
		amount := l.Debit

		if l.Credit.Amount() > 0 {
			side, amount = gl.SideCredit, l.Credit
		}

		posted = append(posted, entry{
			AccountCode: l.AccountID,
			Side:        side,
			Amount:      decimal.NewFromInt(amount.Amount()),
		})
	}

	if err := b.post(id, in.Date, in.Description, posted); err != nil {
		return "", err
	}

	b.notes[id] = in.Description
	b.partners[id] = customer

	return id, nil
}

// CreatePayment records a receipt and, when it names one, settles a document.
//
// The money and the settlement are one operation from the operator's point of
// view â€” they clicked "pay" â€” and the target is therefore checked before
// anything is written. An application that failed after the receipt had been
// posted would leave money in the books with nothing saying what it was for.
func (b *Books) CreatePayment(ctx context.Context, in web.PaymentInput) (string, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	customer := strings.TrimSpace(in.Customer)
	if customer == "" {
		return "", fmt.Errorf("a payment needs a customer")
	}
	if in.Amount <= 0 {
		return "", fmt.Errorf("a payment needs an amount greater than zero")
	}
	if strings.TrimSpace(in.AccountCode) == "" {
		return "", fmt.Errorf("a payment needs the cash or bank account the money moved through")
	}
	if _, known := b.chart.ByCode(in.AccountCode); !known {
		return "", fmt.Errorf("account %s is not in the chart of accounts", in.AccountCode)
	}

	p, err := b.periodFor(in.Date)
	if err != nil {
		return "", err
	}

	rules, err := tt133()
	if err != nil {
		return "", err
	}

	receivable, err := b.resolve(rules, domaindoc.KindSalesInvoice, gl.SideCredit, gl.AccountTypeAsset)
	if err != nil {
		return "", err
	}

	against := strings.TrimSpace(in.Against)
	if against != "" {
		target, err := b.docs.Get(ctx, against)
		if err != nil {
			return "", err
		}
		if target.Status == document.StatusPaid {
			return "", fmt.Errorf("document %s is already paid", against)
		}
		if target.Status == document.StatusVoid {
			return "", fmt.Errorf("document %s has been voided and cannot be settled", against)
		}
	}

	id := b.nextID("rcp")

	doc := document.Document{
		ID:        id,
		Type:      document.TypeReceipt,
		PartnerID: customer,
		Date:      in.Date,
		PeriodID:  p.ID,
		Total:     money.New(in.Amount, currency),
		Status:    document.StatusOpen,
	}

	lines := []document.Line{{
		DocumentID: id,
		AccountID:  in.AccountCode,
		Debit:      money.New(in.Amount, currency),
	}, {
		DocumentID: id,
		AccountID:  receivable,
		Credit:     money.New(in.Amount, currency),
	}}

	if err := b.docs.Create(ctx, doc, lines); err != nil {
		return "", err
	}

	if err := b.post(id, in.Date, "payment from "+customer, []entry{
		{AccountCode: in.AccountCode, Side: gl.SideDebit, Amount: decimal.NewFromInt(in.Amount)},
		{AccountCode: receivable, Side: gl.SideCredit, Amount: decimal.NewFromInt(in.Amount)},
	}); err != nil {
		return "", err
	}

	b.partners[id] = customer

	if against == "" {
		return id, nil
	}

	// Settle is what moves the status: the document absorbs what it can and
	// anything over becomes a credit, which internal/document records rather
	// than refusing the payment.
	if _, err := b.docs.Apply(ctx, document.Application{
		ID:         b.nextID("app"),
		DocumentID: against,
		PaymentID:  id,
		Amount:     money.New(in.Amount, currency),
		At:         in.Date,
	}); err != nil {
		return "", err
	}

	return id, nil
}

// CreateJournal records a manual journal entry and posts it.
func (b *Books) CreateJournal(_ context.Context, in web.JournalInput) (string, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if _, err := b.periodFor(in.Date); err != nil {
		return "", err
	}

	var lines []entry

	for i, l := range in.Lines {
		code := strings.TrimSpace(l.AccountCode)

		// The form renders more rows than most entries need. A row that names
		// nothing at all is one the operator left alone; a row that names an
		// account but no amount is a half-filled row and is refused.
		if code == "" && l.Amount == 0 {
			continue
		}
		if code == "" {
			return "", fmt.Errorf("line %d has an amount but no account", i+1)
		}
		if _, known := b.chart.ByCode(code); !known {
			return "", fmt.Errorf("account %s is not in the chart of accounts", code)
		}
		if l.Amount <= 0 {
			return "", fmt.Errorf("line %d on account %s needs an amount greater than zero", i+1, code)
		}

		var side gl.EntrySide
		switch strings.ToLower(strings.TrimSpace(l.Side)) {
		case "debit":
			side = gl.SideDebit
		case "credit":
			side = gl.SideCredit
		default:
			return "", fmt.Errorf("line %d on account %s is neither a debit nor a credit", i+1, code)
		}

		lines = append(lines, entry{AccountCode: code, Side: side, Amount: decimal.NewFromInt(l.Amount)})
	}

	if len(lines) < 2 {
		return "", fmt.Errorf("a journal entry needs at least two lines")
	}

	id := b.nextID("jnl")

	if err := b.post(id, in.Date, in.Description, lines); err != nil {
		return "", err
	}

	b.journals = append(b.journals, journal{
		ID:          id,
		Date:        in.Date,
		Description: in.Description,
		Lines:       lines,
	})

	return id, nil
}

// covers reports whether a rate's window includes a date.
func covers(rate vat.VATRate, at time.Time) bool {
	if at.Before(rate.EffectiveFrom) {
		return false
	}

	return rate.EffectiveTo == nil || !at.After(*rate.EffectiveTo)
}

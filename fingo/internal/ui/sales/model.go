// Package voucher is the sales-invoice entry form.
//
// It is the one screen in the application that writes. Everything else reads
// books that something else produced, and this is the something else.
//
// It is a model and it contains no widgets. What it decides — how the totals are
// computed, when a form may be posted, what a refusal says — is asserted in
// tests. Drawing the fields is the shell's job.
//
// Two rules run through it, and both are about what happens when something goes
// slightly wrong at the wrong moment:
//
//   - The arithmetic is the ledger's, not a second copy of it. `money.LineNet`
//     and `money.LineVAT` are the same functions the posting uses, so the total
//     the operator reads before pressing Post is the total that gets posted.
//
//   - The idempotency key is fixed when the form opens and is reused until
//     something succeeds. An operator who does not see a response clicks again,
//     and that click must not be a second sale.
package sales

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"fingo/internal/domain/money"
	"fingo/internal/domain/tax"
	"fingo/internal/usecase/sales"
)

// Rates resolves a VAT treatment to a percentage for a given date.
//
// Declared here, where it is used: the form needs one question answered, and it
// should not know that a schedule exists or how it is stored. *tax.Schedule
// satisfies it as it stands.
type Rates interface {
	Percent(kind tax.Kind, date time.Time) (decimal.Decimal, error)
}

// Recorder records a sale. *practice.Client satisfies it.
type Recorder interface {
	RecordSale(ctx context.Context, cmd sales.Command) (sales.Result, error)
}

// Line is one row of the form, as the operator filled it in.
type Line struct {
	ItemCode  string
	Quantity  decimal.Decimal
	UnitPrice decimal.Decimal

	// VAT is the treatment, not a percentage. The percentage depends on when
	// the document is dated, and a line that carried a rate could carry one that
	// is not in force.
	VAT tax.Kind
}

// Totals is what the invoice comes to.
type Totals struct {
	Net   decimal.Decimal
	VAT   decimal.Decimal
	Gross decimal.Decimal
}

// Model is the form.
type Model struct {
	rates    Rates
	recorder Recorder

	// The form's fields. Exported because they are what the operator types and
	// there is nothing to decide by setting them; Validate is where they are
	// judged.
	Party       string
	Warehouse   string
	Reference   string
	Description string
	Operator    string
	Date        time.Time

	lines []Line

	// key names this document, not this attempt. See Command.
	key string
}

// New opens a blank sales invoice dated now.
func New(rates Rates, recorder Recorder, now time.Time) *Model {
	return &Model{
		rates:    rates,
		recorder: recorder,
		Date:     now,
		key:      uuid.NewString(),
	}
}

// AddLine appends a row.
func (m *Model) AddLine(l Line) { m.lines = append(m.lines, l) }

// RemoveLine drops a row by position, ignoring a position that is not there.
//
// Out of range is ignored rather than a panic: a grid fires a remove for the row
// the operator selected, and a selection can go stale between the click and the
// handler.
func (m *Model) RemoveLine(i int) {
	if i < 0 || i >= len(m.lines) {
		return
	}

	m.lines = append(m.lines[:i], m.lines[i+1:]...)
}

// Lines is the form's rows, in the order they are shown.
func (m *Model) Lines() []Line { return m.lines }

// Totals adds the form up.
//
// It calls the ledger's own arithmetic rather than repeating it. VAT is rounded
// per line, which is what makes an invoice total the sum of its rounded parts —
// and a preview that rounded differently would show one number and post another.
func (m *Model) Totals() (Totals, error) {
	var net, vat decimal.Decimal

	for i, l := range m.lines {
		rate, err := m.rates.Percent(l.VAT, m.Date)
		if err != nil {
			return Totals{}, fmt.Errorf("dòng %d (%s): %w", i+1, l.ItemCode, err)
		}

		lineNet := money.LineNet(l.Quantity, l.UnitPrice)

		net = net.Add(lineNet)
		vat = vat.Add(money.LineVAT(lineNet, rate))
	}

	return Totals{Net: money.Dong(net), VAT: vat, Gross: net.Add(vat)}, nil
}

// Command is the sale to record.
//
// The idempotency key is the same on every call for the life of this form. An
// operator who does not see a response clicks Post again; with a stable key the
// ledger stores one sale, and with a fresh one it stores two — two receivables,
// twice the revenue, and a customer who owes double.
func (m *Model) Command() (sales.Command, error) {
	if err := m.Validate(); err != nil {
		return sales.Command{}, err
	}

	return sales.Command{
		VoucherID:      m.key,
		IdempotencyKey: m.key,

		PartnerID:         m.Party,
		WarehouseCode:     m.Warehouse,
		DocumentReference: m.Reference,
		Description:       m.Description,
		Operator:          m.Operator,

		Date: m.Date,
		At:   time.Now().UTC(),

		Lines: append([]sales.SaleLine(nil), m.toSaleLines()...),
	}, nil
}

// Validate reports the first thing wrong with the form.
//
// The wording is the operator's. "a credit sale requires the customer it is owed
// by" is true and is addressed to whoever wrote the ledger; the person at the
// keyboard needs to be told which box is empty.
func (m *Model) Validate() error {
	switch {
	case m.Party == "":
		return errors.New("chưa chọn khách hàng — hóa đơn bán hàng phải ghi cho ai")
	case m.Warehouse == "":
		return errors.New("chưa chọn kho xuất hàng — giá vốn lấy từ kho nào")
	case m.Description == "":
		return errors.New("chưa ghi diễn giải — sổ cần biết bút toán này là việc gì")
	case m.Operator == "":
		return errors.New("chưa có người lập — sổ phải biết ai nhập chứng từ này")
	case len(m.lines) == 0:
		return errors.New("hóa đơn chưa có dòng hàng nào")
	}

	for i, l := range m.lines {
		if l.ItemCode == "" {
			return fmt.Errorf("dòng %d chưa chọn mặt hàng", i+1)
		}
		if !l.Quantity.IsPositive() {
			return fmt.Errorf("dòng %d: số lượng phải lớn hơn 0", i+1)
		}
		if !l.UnitPrice.IsPositive() {
			return fmt.Errorf("dòng %d: đơn giá phải lớn hơn 0", i+1)
		}
	}

	totals, err := m.Totals()
	if err != nil {
		return err
	}
	if !totals.Gross.IsPositive() {
		return errors.New("hóa đơn không có giá trị — tổng tiền phải lớn hơn 0")
	}

	return nil
}

// Post records the sale.
//
// On success the form is finished with: it is blank, with a new key, ready for
// the next document. On failure nothing is cleared and the key is kept, so the
// operator can correct and retry without typing the invoice again — and the
// retry is still recognised as the same request.
func (m *Model) Post(ctx context.Context) (sales.Result, error) {
	cmd, err := m.Command()
	if err != nil {
		return sales.Result{}, err
	}

	result, err := m.recorder.RecordSale(ctx, cmd)
	if err != nil {
		return sales.Result{}, fmt.Errorf("ghi sổ hóa đơn: %w", err)
	}

	m.lines = nil
	m.Party = ""
	m.Reference = ""
	m.Description = ""
	m.key = uuid.NewString()

	return result, nil
}

func (m *Model) toSaleLines() []sales.SaleLine {
	out := make([]sales.SaleLine, 0, len(m.lines))

	for _, l := range m.lines {
		out = append(out, sales.SaleLine{
			ItemCode:  l.ItemCode,
			Quantity:  l.Quantity,
			UnitPrice: l.UnitPrice,
			VAT:       l.VAT,
		})
	}

	return out
}

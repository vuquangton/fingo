package gl

import (
	"context"
	"errors"
	"fingo/internal/money"
	"fmt"
	"strings"
	"time"
)

type VoucherType string

const (
	VoucherTypeSales       VoucherType = "SALES"
	VoucherTypePurchase    VoucherType = "PURCHASE"
	VoucherTypeCashReceipt VoucherType = "CASH_RECEIPT"
	VoucherTypeCashPayment VoucherType = "CASH_PAYMENT"
	VoucherTypeGeneral     VoucherType = "GENERAL"
)

// EntrySide is the side of the ledger a line posts to.
type EntrySide string

const (
	SideDebit  EntrySide = "DEBIT"
	SideCredit EntrySide = "CREDIT"
)

// VoucherLine is a single account movement: one account, one side, one amount.
// A voucher is a set of these. Do not reintroduce a debit/credit pair here —
// see docs/adr/0001-journal-line-model.md for why.
type VoucherLine struct {
	ID          string
	VoucherID   string
	LineOrder   int
	AccountCode string

	// PartnerID names the counterparty this line is owed by or to. Empty on the
	// many lines that belong to nobody in particular — cash, inventory, VAT.
	//
	// It is a dimension of the line rather than a note beside it because TT133
	// keeps Sổ chi tiết TK 131 by customer: without it the ledger can say 131 is
	// 500,000,000 but not who owes it.
	PartnerID string

	Side   EntrySide
	Amount money.Dong
	Note   string
}

// Voucher is an accounting voucher: a balanced set of account movements.
//
// State is unexported so that a voucher can only reach a valid state through
// its own methods. Callers cannot flip a posted voucher back to draft.
type Voucher struct {
	id          string
	book        Book
	fiscalYear  int
	seq         int
	voucherDate time.Time
	voucherType VoucherType
	description string
	isPosted    bool
	postedAt    time.Time
	postedBy    string
	lines       []VoucherLine
}

// NewVoucher creates a draft voucher.
//
// No number is supplied: a voucher is numbered when it is posted, by the ledger
// allocating the next place in the right book. A draft, therefore, has no
// number — see Number.
//
// The description explains the entry and is required before posting: a ledger
// line with no explanation cannot be defended to an auditor.
func NewVoucher(id, description string, vType VoucherType, vDate time.Time) *Voucher {
	return &Voucher{
		id:          id,
		description: description,
		voucherType: vType,
		voucherDate: vDate,
		lines:       make([]VoucherLine, 0),
	}
}

// SetDescription corrects the description of a draft. A posted voucher's
// description is part of the record and cannot be rewritten.
func (v *Voucher) SetDescription(description string) error {
	if v.isPosted {
		return errors.New("cannot change the description of a posted voucher")
	}

	v.description = description

	return nil
}

func (v *Voucher) ID() string               { return v.id }
func (v *Voucher) VoucherDate() time.Time   { return v.voucherDate }
func (v *Voucher) VoucherType() VoucherType { return v.voucherType }
func (v *Voucher) Description() string      { return v.description }
func (v *Voucher) IsPosted() bool           { return v.isPosted }
func (v *Voucher) PostedAt() time.Time      { return v.postedAt }
func (v *Voucher) PostedBy() string         { return v.postedBy }

// Number is the voucher's place in its book, or the zero Number while the
// voucher is still a draft.
func (v *Voucher) Number() Number {
	return Number{Book: v.book, Year: v.fiscalYear, Seq: v.seq}
}

// VoucherNo renders the number as it is written on the document. It is empty
// until the voucher is posted.
func (v *Voucher) VoucherNo() string {
	if v.seq == 0 {
		return ""
	}

	return v.Number().String()
}

// Lines returns a copy, so callers cannot mutate the voucher's internals.
func (v *Voucher) Lines() []VoucherLine {
	out := make([]VoucherLine, len(v.lines))
	copy(out, v.lines)
	return out
}

// AddLine appends one account movement. The line is validated on the way in,
// so an illegal amount or side can never enter the ledger.
func (v *Voucher) AddLine(line VoucherLine) error {
	if v.isPosted {
		return errors.New("cannot add a line to a posted voucher")
	}
	if line.AccountCode == "" {
		return errors.New("voucher line account is required")
	}
	if line.Amount.LessThanOrEqual(money.Zero) {
		return errors.New("voucher line amount must be greater than zero")
	}
	if !line.Amount.Equal(line.Amount.Truncate(0)) {
		return fmt.Errorf("voucher line amount %s carries a fractional đồng part; VND has no subunit", line.Amount)
	}
	switch line.Side {
	case SideDebit, SideCredit:
	default:
		return fmt.Errorf("voucher line has an unknown side %q", line.Side)
	}

	line.VoucherID = v.id
	line.LineOrder = len(v.lines) + 1
	v.lines = append(v.lines, line)
	return nil
}

// ValidateBalance reports whether the voucher is a legal set of books: at
// least one line, and total debits equal to total credits.
func (v *Voucher) ValidateBalance() error {
	if len(v.lines) == 0 {
		return errors.New("voucher must contain at least one line")
	}

	totalDebit := money.Zero
	totalCredit := money.Zero

	for _, l := range v.lines {
		switch l.Side {
		case SideDebit:
			totalDebit = totalDebit.Add(l.Amount)
		case SideCredit:
			totalCredit = totalCredit.Add(l.Amount)
		}
	}

	if totalDebit.Cmp(totalCredit) != 0 {
		return fmt.Errorf("voucher does not balance: debit %s != credit %s", totalDebit, totalCredit)
	}

	return nil
}

// PostingRules are the ledger's rules at the moment of posting.
//
// Rules are a required argument rather than ambient state. A caller cannot post
// without supplying the chart the voucher is posted against, so the check
// cannot be forgotten — and cannot be bypassed by calling Post behind the
// ledger's back.
type PostingRules struct {
	// Chart is the client's chart of accounts. Required.
	Chart *ChartOfAccounts

	// Periods is the client's period calendar. Required.
	Periods *Periods
}

// ValidatePostable reports whether the voucher could be posted under these
// rules, leaving the number aside.
//
// It exists so the ledger can refuse a posting before it consumes a place in a
// book's run: an entry the rules reject must not burn a number.
func (v *Voucher) ValidatePostable(rules PostingRules) error {
	if v.isPosted {
		return errors.New("voucher is already posted")
	}
	if err := v.ValidateBalance(); err != nil {
		return err
	}
	if strings.TrimSpace(v.description) == "" {
		return errors.New("a voucher must carry a description before it can be posted")
	}
	if rules.Chart == nil {
		return errors.New("posting requires the chart of accounts the voucher is posted against")
	}
	if rules.Periods == nil {
		return errors.New("posting requires the period calendar")
	}
	if !rules.Periods.IsOpen(v.voucherDate) {
		return fmt.Errorf("period %s is closed, so an entry dated %s cannot be posted",
			MonthOf(v.voucherDate), v.voucherDate.UTC().Format("2006-01-02"))
	}
	if _, err := BookFor(v.voucherType); err != nil {
		return err
	}

	for _, l := range v.lines {
		if !rules.Chart.IsPostable(l.AccountCode) {
			return fmt.Errorf(
				"account %s cannot be posted to: it is unknown, inactive, or a heading with sub-accounts",
				l.AccountCode)
		}
	}

	return nil
}

// Post admits the voucher to the ledger under an allocated number.
//
// This is the last point at which the ledger's rules can be enforced, so every
// one of them is applied here, and the number must belong to this voucher's own
// book and fiscal year.
func (v *Voucher) Post(rules PostingRules, number Number, at time.Time, by string) error {
	if err := v.ValidatePostable(rules); err != nil {
		return err
	}

	book, err := BookFor(v.voucherType)
	if err != nil {
		return err
	}
	if err := number.Validate(); err != nil {
		return err
	}
	if number.Book != book {
		return fmt.Errorf("a %s voucher is numbered in the %s book, not the %s book",
			v.voucherType, book, number.Book)
	}
	if year := v.voucherDate.UTC().Year(); number.Year != year {
		return fmt.Errorf("a voucher dated %s belongs to fiscal year %d, not %d",
			v.voucherDate.UTC().Format("2006-01-02"), year, number.Year)
	}

	v.book = number.Book
	v.fiscalYear = number.Year
	v.seq = number.Seq
	v.isPosted = true
	v.postedAt = at.UTC()
	v.postedBy = by

	return nil
}

// restorePosted marks a voucher posted from stored state.
//
// Only Rehydrate may use it. A stored voucher was validated by Post when it was
// first written, so only the balance is re-checked here. The chart, period and
// numbering rules are deliberately not re-applied: a chart can be modified after
// the fact (TT 46/2025/TT-BTC permits it), and refusing a stored voucher because
// the chart moved would make the client's own books unreadable.
func (v *Voucher) restorePosted(at time.Time, by string) error {
	if v.isPosted {
		return errors.New("voucher is already posted")
	}
	if err := v.ValidateBalance(); err != nil {
		return err
	}

	v.isPosted = true
	v.postedAt = at.UTC()
	v.postedBy = by

	return nil
}

// Reverse builds an unposted counter-entry that cancels this voucher.
//
// The caller posts it through the same door every other posting uses, so the
// counter-entry faces the same rules — in particular its target date must fall
// in an open period. Posted vouchers are never edited or deleted: the ledger is
// corrected by mirroring, so the original entry and its correction both remain
// visible.
func (v *Voucher) Reverse(newID string, at time.Time) (*Voucher, error) {
	if !v.isPosted {
		return nil, errors.New("only a posted voucher can be reversed")
	}

	// No number: the caller posts the reversal, and it takes the next place in
	// its own book on the date it is posted.
	reversal := NewVoucher(newID, "Reversal of "+v.VoucherNo(), v.voucherType, at)

	for _, l := range v.lines {
		mirrored := l
		mirrored.ID = ""
		mirrored.VoucherID = ""
		mirrored.LineOrder = 0

		if l.Side == SideDebit {
			mirrored.Side = SideCredit
		} else {
			mirrored.Side = SideDebit
		}

		if err := reversal.AddLine(mirrored); err != nil {
			return nil, err
		}
	}

	return reversal, nil
}

// PersistedVoucher is the complete stored state of a voucher.
//
// It exists so a persistence adapter can rebuild a Voucher without reopening
// the invariants Voucher protects. See Rehydrate.
type PersistedVoucher struct {
	ID          string
	Book        Book
	FiscalYear  int
	Seq         int
	VoucherDate time.Time
	VoucherType VoucherType
	Description string
	IsPosted    bool
	PostedAt    time.Time
	PostedBy    string
	Lines       []VoucherLine
}

// Rehydrate rebuilds a voucher from stored state.
//
// This is the only way to obtain a posted Voucher without going through Post,
// and it is intended solely for persistence adapters.
//
// Stored state is not trusted. Rehydrate re-applies every domain rule to what
// it loads — each line is re-validated and a posted voucher is put through Post
// again — so a database holding a malformed or unbalanced voucher fails here
// rather than leaking a state into the domain that AddLine or Post would have
// refused.
func Rehydrate(p PersistedVoucher) (*Voucher, error) {
	v := NewVoucher(p.ID, p.Description, p.VoucherType, p.VoucherDate)
	v.book = p.Book
	v.fiscalYear = p.FiscalYear
	v.seq = p.Seq

	for _, l := range p.Lines {
		if err := v.AddLine(l); err != nil {
			return nil, fmt.Errorf("rehydrate voucher %s: line %s: %w", p.ID, l.ID, err)
		}
	}

	if p.IsPosted {
		if err := v.restorePosted(p.PostedAt, p.PostedBy); err != nil {
			return nil, fmt.Errorf("rehydrate posted voucher %s: %w", p.ID, err)
		}
	}

	return v, nil
}

// ErrDuplicateRequest reports that a request has already been posted.
//
// The caller receives it instead of a second voucher. The original is still
// there, findable by its idempotency key.
var ErrDuplicateRequest = errors.New("this request has already been posted")

// VoucherRepository stores and retrieves vouchers. The adapter that implements
// it is responsible for writing a voucher and all of its lines atomically, and
// for resolving account codes to whatever identity the storage uses.
type VoucherRepository interface {
	// SaveVoucher stores a voucher and its lines atomically.
	//
	// An idempotencyKey, when non-empty, is recorded in the same transaction. A
	// repeat with the same key stores nothing and returns ErrDuplicateRequest,
	// so a resubmitted request cannot post a second voucher.
	SaveVoucher(ctx context.Context, v *Voucher, idempotencyKey string) error

	GetVoucher(ctx context.Context, id string) (*Voucher, error)
}

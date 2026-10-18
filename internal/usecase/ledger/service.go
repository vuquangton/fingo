// Package ledger is the single path by which a voucher reaches a client's books.
//
// Every document use case — sales, purchases, cash, closing — posts through this
// service, so the ledger's rules are assembled and applied in one place instead
// of being repeated, and forgotten, at each call site.
// See docs/adr/0002-posting-lifecycle.md.
package ledger

import (
	"context"
	"fmt"
	"time"

	"github.com/shopspring/decimal"

	"fingo/internal/domain/document"
	"fingo/internal/domain/gl"
)

// StockIssue is stock leaving the warehouse because of a posting.
type StockIssue struct {
	ItemCode      string
	WarehouseCode string
	Quantity      decimal.Decimal

	// Cost is what the issue was resolved at, before the transaction, and is
	// re-checked inside it. If another posting moved the same stock first then
	// the average has changed, the cost no longer matches, and this posting is
	// refused rather than written at a cost that was true a moment ago.
	//
	// That check is what makes it safe to resolve a cost outside a transaction
	// at all.
	Cost decimal.Decimal
}

// StockReceipt is stock arriving because of a posting.
type StockReceipt struct {
	ItemCode      string
	WarehouseCode string
	Quantity      decimal.Decimal
	Cost          decimal.Decimal
}

// Posting is one atomic write to the books.
//
// Money, stock and the document being invoiced describe one event. If two of
// them were written and the third were not, the books would disagree with
// themselves in a way no later reader could untangle — so they carry one
// transaction, and the transaction belongs to the repository.
type Posting struct {
	Voucher    *gl.Voucher
	At         time.Time
	By         string
	RequestKey string

	// Cost is the voucher that records what the goods this posting moved were
	// worth — the inventory side of the same event.
	//
	// It travels in the same Posting, and therefore in the same transaction,
	// because a sale and the cost of what it sold are one event: written apart,
	// a failure between them leaves a sale that overstated profit, or a
	// warehouse short of goods the ledger still carries. Nil for a posting that
	// moves no stock — a service sale, a cash transfer, an adjustment.
	//
	// It is numbered in its own book, because it is not an invoice: see
	// gl.BookFor.
	Cost *gl.Voucher

	// Document is the business document this voucher records, when it records
	// one. Nil for a voucher that records no invoice — a journal adjustment, a
	// cash transfer.
	Document *document.Document

	StockIssues   []StockIssue
	StockReceipts []StockReceipt
}

// Repository writes postings atomically.
type Repository interface {
	Save(ctx context.Context, p Posting) error
}

// ChartSource supplies the client's chart of accounts.
type ChartSource interface {
	Chart(ctx context.Context) (*gl.ChartOfAccounts, error)
}

// PeriodSource supplies the client's closed months.
//
// This is the older, month-based model. It is kept while the range-based lock in
// internal/period is adopted alongside it; one of the two has to go eventually,
// and the ranges express strictly more.
type PeriodSource interface {
	ClosedPeriods(ctx context.Context) (*gl.Periods, error)
}

// LockGuard refuses a posting date that falls in a locked accounting period.
//
// Declared here, where it is used: the ledger needs one question answered, and
// internal/period.Guard answers it without the ledger knowing how a period is
// stored or what makes one locked.
type LockGuard interface {
	CheckLocked(ctx context.Context, transactionDate time.Time) error
}

// NumberSource allocates the next place in a book's run for a fiscal year.
type NumberSource interface {
	NextNumber(ctx context.Context, book gl.Book, fiscalYear int) (gl.Number, error)
}

// Service posts vouchers. It is the only production route to a posted voucher.
type Service struct {
	postings Repository
	numbers  NumberSource
	chart    ChartSource
	periods  PeriodSource
	locks    LockGuard
}

func NewService(
	postings Repository,
	numbers NumberSource,
	chart ChartSource,
	periods PeriodSource,
	locks LockGuard,
) *Service {
	return &Service{
		postings: postings,
		numbers:  numbers,
		chart:    chart,
		periods:  periods,
		locks:    locks,
	}
}

// Post applies the client's rules, allocates a number, admits the voucher to the
// ledger, and writes the whole posting in one transaction.
//
// The rules are checked before a number is taken. An entry the ledger would
// refuse must not burn a place in a book's run — a gap should mean something
// happened to a document, not that someone mistyped a date.
//
// The voucher is marked posted before it is stored, because the stored row
// carries the posting state. If the write then fails, the in-memory voucher is
// posted but nothing was written — the error propagates and the caller discards
// the object, so no partially-posted state can escape into the books.
func (s *Service) Post(ctx context.Context, p Posting) error {
	if p.Voucher == nil {
		return fmt.Errorf("a posting needs a voucher")
	}

	// The lock is checked first. An entry into a period somebody has shut is
	// refused before anything else is considered, and before a number is taken
	// from the book's run — a refused entry must not burn one. The cost voucher
	// is checked here too, so a sale whose cost cannot be posted is refused
	// before the sale has taken a number either.
	if err := s.locks.CheckLocked(ctx, p.Voucher.VoucherDate()); err != nil {
		return err
	}
	if p.Cost != nil {
		if err := s.locks.CheckLocked(ctx, p.Cost.VoucherDate()); err != nil {
			return err
		}
	}

	rules, err := s.rules(ctx)
	if err != nil {
		return err
	}

	if err := s.admit(ctx, p.Voucher, rules, p.At, p.By); err != nil {
		return err
	}

	// Both vouchers are admitted before either is written. A cost that the rules
	// reject must not leave a posted sale behind it, nor a sale that the rules
	// reject leave a posted cost.
	if p.Cost != nil {
		if err := s.admit(ctx, p.Cost, rules, p.At, p.By); err != nil {
			return err
		}
	}

	if err := s.postings.Save(ctx, p); err != nil {
		return fmt.Errorf("store voucher %s: %w", p.Voucher.VoucherNo(), err)
	}

	return nil
}

// admit applies the client's rules to one voucher, allocates its number, and
// marks it posted.
//
// The rules are checked before a number is taken. An entry the ledger would
// refuse must not burn a place in a book's run — a gap should mean something
// happened to a document, not that someone mistyped a date.
//
// The voucher is marked posted before it is stored, because the stored row
// carries the posting state. If the write then fails, the in-memory voucher is
// posted but nothing was written — the error propagates and the caller discards
// the object, so no partially-posted state can escape into the books.
func (s *Service) admit(ctx context.Context, voucher *gl.Voucher, rules gl.PostingRules, at time.Time, by string) error {
	if err := voucher.ValidatePostable(rules); err != nil {
		return err
	}

	book, err := gl.BookFor(voucher.VoucherType())
	if err != nil {
		return err
	}

	number, err := s.numbers.NextNumber(ctx, book, voucher.VoucherDate().UTC().Year())
	if err != nil {
		return fmt.Errorf("number the %s voucher: %w", book, err)
	}

	return voucher.Post(rules, number, at, by)
}

// rules assembles the client's posting rules.
//
// Loaded per posting rather than cached: TT 46/2025/TT-BTC lets a client modify
// their chart and close periods at will, and posting against stale rules would
// either refuse entries the client has legitimately made possible or, worse,
// accept one into a period they have closed.
func (s *Service) rules(ctx context.Context) (gl.PostingRules, error) {
	chart, err := s.chart.Chart(ctx)
	if err != nil {
		return gl.PostingRules{}, fmt.Errorf("load the chart of accounts: %w", err)
	}

	periods, err := s.periods.ClosedPeriods(ctx)
	if err != nil {
		return gl.PostingRules{}, fmt.Errorf("load the period calendar: %w", err)
	}

	return gl.PostingRules{Chart: chart, Periods: periods}, nil
}

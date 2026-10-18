package books

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"fingo/internal/domain/gl"
	"fingo/internal/ledger"
	"fingo/internal/money"
)

// ErrAlreadyReversed is returned when a voucher has already been reversed.
var ErrAlreadyReversed = errors.New("books: that voucher has already been reversed")

// ErrNotPosted is returned for a voucher that cannot be corrected.
var ErrNotPosted = errors.New("books: only a posted voucher can be reversed")

// reversalPrefix marks the mirror entries this package writes.
const reversalPrefix = "Đảo bút toán "

// ReverseVoucher posts the mirror of a posted voucher.
//
// A posted voucher is immutable (ADR 0002): a mistake is corrected by an opposite
// entry, dated when it was noticed, never by editing the original. The mirror
// carries the same accounts, opposite sides and the same amounts, so every
// account it touches returns to where it stood.
//
// It is refused if the voucher has already been reversed, because a second mirror
// would silently undo the correction and leave the books looking untouched.
func (b *Books) ReverseVoucher(ctx context.Context, voucherNo, actor string, date time.Time) (ledger.Posted, error) {
	voucherNo = strings.TrimSpace(voucherNo)
	if voucherNo == "" {
		return ledger.Posted{}, errors.New("books: a reversal needs the voucher number")
	}
	if actor == "" {
		return ledger.Posted{}, errors.New("books: a reversal needs an operator")
	}

	var (
		originalID string
		posted     bool
	)

	type line struct {
		account string
		side    gl.EntrySide
		amount  money.Dong
		note    string
	}
	var lines []line

	err := b.store.Read(ctx, b.scope, func(tx pgx.Tx) error {
		err := tx.QueryRow(ctx, `
			SELECT id::text, posted_at IS NOT NULL
			  FROM vouchers WHERE voucher_no = $1`, voucherNo).Scan(&originalID, &posted)
		if errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("%w: no voucher %s", ledger.ErrInvalidRequest, voucherNo)
		}
		if err != nil {
			return err
		}

		rows, err := tx.Query(ctx, `
			SELECT account_code, side, amount, note
			  FROM voucher_lines WHERE voucher_id = $1 ORDER BY line_order`, originalID)
		if err != nil {
			return err
		}
		defer rows.Close()

		for rows.Next() {
			var l line
			var side string
			if err := rows.Scan(&l.account, &side, &l.amount, &l.note); err != nil {
				return err
			}
			l.side = gl.EntrySide(side)
			lines = append(lines, l)
		}
		return rows.Err()
	})
	if err != nil {
		return ledger.Posted{}, err
	}

	if !posted {
		return ledger.Posted{}, fmt.Errorf("%w: %s is still a draft", ErrNotPosted, voucherNo)
	}

	var already int
	err = b.store.Read(ctx, b.scope, func(tx pgx.Tx) error {
		return tx.QueryRow(ctx,
			`SELECT count(*) FROM vouchers WHERE description = $1`,
			reversalPrefix+voucherNo).Scan(&already)
	})
	if err != nil {
		return ledger.Posted{}, err
	}
	if already > 0 {
		return ledger.Posted{}, fmt.Errorf("%w: %s", ErrAlreadyReversed, voucherNo)
	}

	mirror := make([]ledger.Line, 0, len(lines))
	for _, l := range lines {
		side := gl.SideDebit
		if l.side == gl.SideDebit {
			side = gl.SideCredit
		}
		mirror = append(mirror, ledger.Line{
			Account: l.account,
			Side:    side,
			Amount:  l.amount,
			Note:    "Đảo: " + l.note,
		})
	}

	day := time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, time.UTC)

	return ledger.Post(ctx, b.store, b.scope, ledger.Request{
		ID:          newUUID(),
		Description: reversalPrefix + voucherNo,
		Actor:       actor,
		Date:        day,
		Type:        gl.VoucherTypeGeneral,
		Lines:       mirror,
	})
}

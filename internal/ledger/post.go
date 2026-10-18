// Package ledger posts a voucher: domain rules, then storage, then audit, in one
// transaction.
//
// This is the only path from a user's entry to a posted voucher. It exists
// because the domain can validate a voucher and storage can satisfy the
// database's constraints, but neither can do both, and a posting that half
// happens is the failure the whole design is built to prevent.
package ledger

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"fingo/internal/audit"
	"fingo/internal/domain/gl"
	"fingo/internal/money"
	"fingo/internal/store"
)

// ErrInvalidRequest is returned for a request that cannot describe a voucher.
var ErrInvalidRequest = errors.New("ledger: invalid posting request")

// Line is one account movement on the way in.
type Line struct {
	Account string
	Side    gl.EntrySide
	Amount  money.Dong
	Note    string
}

// Request is a voucher to post.
type Request struct {
	ID          string
	Description string
	Actor       string
	Date        time.Time
	Type        gl.VoucherType
	Lines       []Line
}

// Posted identifies what the ledger admitted.
type Posted struct {
	ID        string
	VoucherNo string
	Seq       int
}

// Post validates a voucher against the client's own chart and period calendar,
// allocates its number, and writes the voucher, its lines and the audit record in
// the caller's transaction scope.
//
// The number is allocated under a transaction-scoped advisory lock keyed on
// (client, book, year): two concurrent posts into one book cannot pick the same
// sequence. PostgreSQL's unique (client_id, book, fiscal_year, seq) is the second
// line of defence, not the first.
func Post(ctx context.Context, st *store.Store, scope store.Scope, req Request) (Posted, error) {
	if err := validate(req); err != nil {
		return Posted{}, err
	}

	var out Posted

	err := st.InClient(ctx, scope, func(tx pgx.Tx) error {
		v := gl.NewVoucher(req.ID, req.Description, req.Type, req.Date)
		for i, line := range req.Lines {
			if err := v.AddLine(gl.VoucherLine{
				ID:          newUUID(),
				LineOrder:   i + 1,
				AccountCode: line.Account,
				Side:        line.Side,
				Amount:      line.Amount,
				Note:        line.Note,
			}); err != nil {
				return fmt.Errorf("line %d: %w", i+1, err)
			}
		}

		var postErr error
		out, postErr = postInTx(ctx, tx, scope, v, req.Actor)
		return postErr
	})
	if err != nil {
		return Posted{}, err
	}
	return out, nil
}

func validate(req Request) error {
	switch {
	case req.ID == "":
		return fmt.Errorf("%w: a voucher needs an id", ErrInvalidRequest)
	case req.Description == "":
		return fmt.Errorf("%w: a voucher needs a description", ErrInvalidRequest)
	case req.Actor == "":
		return fmt.Errorf("%w: a posting needs an actor", ErrInvalidRequest)
	case req.Date.IsZero():
		return fmt.Errorf("%w: a voucher needs a date", ErrInvalidRequest)
	case len(req.Lines) < 2:
		return fmt.Errorf("%w: a voucher needs at least two lines", ErrInvalidRequest)
	}
	return nil
}

// PostVoucher posts a voucher the caller built itself.
//
// The period close builds one — revenue and expenses carried through 911 into
// retained earnings — and it must travel the same rules and the same transaction
// as a hand-entered voucher. A second path to the ledger is a second set of
// rules, and the two would disagree.
func PostVoucher(ctx context.Context, st *store.Store, scope store.Scope, v *gl.Voucher, actor string) (Posted, error) {
	var out Posted

	err := st.InClient(ctx, scope, func(tx pgx.Tx) error {
		var postErr error
		out, postErr = postInTx(ctx, tx, scope, v, actor)
		return postErr
	})
	if err != nil {
		return Posted{}, err
	}
	return out, nil
}

// postInTx is the whole posting path, inside a scoped transaction: the rules, the
// number, the rows, and the audit record.
func postInTx(ctx context.Context, tx pgx.Tx, scope store.Scope, v *gl.Voucher, actor string) (Posted, error) {
	chart, err := loadChart(ctx, tx)
	if err != nil {
		return Posted{}, err
	}

	periods, err := loadPeriods(ctx, tx)
	if err != nil {
		return Posted{}, err
	}

	book, err := gl.BookFor(v.VoucherType())
	if err != nil {
		return Posted{}, err
	}
	year := v.VoucherDate().UTC().Year()

	seq, err := nextSeq(ctx, tx, scope, book, year)
	if err != nil {
		return Posted{}, err
	}

	number := gl.Number{Book: book, Year: year, Seq: seq}
	if err := v.Post(gl.PostingRules{Chart: chart, Periods: periods}, number, time.Now(), actor); err != nil {
		return Posted{}, err
	}

	voucherID := v.ID()
	// trading derives a readable id ("invoice-<uuid>"), which is a fine domain
	// identifier but not a uuid. The database key is generated here; the domain
	// id stays what the domain said.
	if !isUUID(voucherID) {
		voucherID = newUUID()
	}

	if err := insert(ctx, tx, scope, v, voucherID); err != nil {
		return Posted{}, err
	}

	if err := audit.Record(ctx, tx, audit.Event{
		ClientID: scope.ID(),
		Actor:    actor,
		Action:   audit.ActionPost,
		Entity:   "voucher",
		EntityID: v.ID(),
		After:    []byte(fmt.Sprintf(`{"voucher_no":%q,"book":%q,"seq":%d}`, v.VoucherNo(), book, seq)),
	}); err != nil {
		return Posted{}, err
	}

	return Posted{ID: voucherID, VoucherNo: v.VoucherNo(), Seq: seq}, nil
}

// loadChart reads the client's chart. Row-level security already limits the rows
// to this client; the filter is repeated so the query states its own intent.
func loadChart(ctx context.Context, tx pgx.Tx) (*gl.ChartOfAccounts, error) {
	rows, err := tx.Query(ctx, `
		SELECT code, name, COALESCE(parent_code, ''), account_type, nature, active
		  FROM accounts
		 WHERE client_id = app_require_client()
		 ORDER BY code`)
	if err != nil {
		return nil, fmt.Errorf("read the chart: %w", err)
	}
	defer rows.Close()

	chart := gl.NewChartOfAccounts()
	for rows.Next() {
		var a gl.Account
		var accountType, nature string
		if err := rows.Scan(&a.Code, &a.Name, &a.ParentCode, &accountType, &nature, &a.IsActive); err != nil {
			return nil, fmt.Errorf("scan an account: %w", err)
		}
		a.Type = gl.AccountType(accountType)
		a.Nature = gl.AccountNature(nature)
		if err := chart.Load(a); err != nil {
			return nil, fmt.Errorf("load account %s: %w", a.Code, err)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read the chart: %w", err)
	}
	return chart, nil
}

// loadPeriods marks every month a lock covers as closed.
func loadPeriods(ctx context.Context, tx pgx.Tx) (*gl.Periods, error) {
	rows, err := tx.Query(ctx, `
		SELECT p.start_date, p.end_date
		  FROM accounting_period_locks l
		  JOIN accounting_periods p ON p.id = l.period_id
		 WHERE l.client_id = app_require_client()`)
	if err != nil {
		return nil, fmt.Errorf("read the period locks: %w", err)
	}
	defer rows.Close()

	periods := gl.NewPeriods()
	for rows.Next() {
		var start, end time.Time
		if err := rows.Scan(&start, &end); err != nil {
			return nil, fmt.Errorf("scan a period lock: %w", err)
		}
		// Every month the lock covers is closed. Iterating first-of-month avoids
		// the 31st-of-February trap a day-by-day step would hit.
		first := time.Date(start.Year(), start.Month(), 1, 0, 0, 0, 0, time.UTC)
		last := time.Date(end.Year(), end.Month(), 1, 0, 0, 0, 0, time.UTC)
		for m := first; !m.After(last); m = m.AddDate(0, 1, 0) {
			ym := gl.MonthOf(m)
			if periods.IsClosed(ym) {
				continue
			}
			if err := periods.Close(ym); err != nil {
				return nil, err
			}
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read the period locks: %w", err)
	}
	return periods, nil
}

func nextSeq(ctx context.Context, tx pgx.Tx, scope store.Scope, book gl.Book, year int) (int, error) {
	key := fmt.Sprintf("%s/%s/%d", scope.ID(), book, year)
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext($1))`, key); err != nil {
		return 0, fmt.Errorf("lock the number series: %w", err)
	}

	var seq int
	err := tx.QueryRow(ctx, `
		SELECT COALESCE(MAX(seq), 0) + 1
		  FROM vouchers
		 WHERE client_id = $1 AND book = $2 AND fiscal_year = $3`,
		scope.ID(), string(book), year).Scan(&seq)
	if err != nil {
		return 0, fmt.Errorf("allocate a voucher number: %w", err)
	}
	return seq, nil
}

func insert(ctx context.Context, tx pgx.Tx, scope store.Scope, v *gl.Voucher, voucherID string) error {
	// The header is written as a DRAFT, then its lines, then it is marked posted.
	//
	// That order is not cosmetic: the immutability triggers refuse to attach lines
	// to a voucher that is already posted, which is exactly the rule that stops a
	// posted voucher being edited later. Writing the header posted and then
	// inserting lines would trip that guard on our own write. The whole sequence is
	// still one transaction, so no reader ever sees the draft.
	_, err := tx.Exec(ctx, `
		INSERT INTO vouchers
			(id, client_id, voucher_no, book, fiscal_year, seq, voucher_date,
			 description, created_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
		voucherID, scope.ID(), v.VoucherNo(), string(v.Number().Book), v.Number().Year, v.Number().Seq,
		v.VoucherDate(), v.Description(), v.PostedBy())
	if err != nil {
		return fmt.Errorf("write the voucher: %w", err)
	}

	for _, line := range v.Lines() {
		// A voucher built by the domain may carry no line ids: the closing entries
		// are constructed as account, side, amount. The id is the ledger's to issue.
		lineID := line.ID
		if lineID == "" {
			lineID = newUUID()
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO voucher_lines
				(id, client_id, voucher_id, line_order, account_code, side, amount, note)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
			lineID, scope.ID(), voucherID, line.LineOrder, line.AccountCode,
			string(line.Side), line.Amount.Int64(), line.Note); err != nil {
			return fmt.Errorf("write line %d: %w", line.LineOrder, err)
		}
	}

	if _, err := tx.Exec(ctx, `
		UPDATE vouchers SET posted_at = $2, posted_by = $3 WHERE id = $1`,
		voucherID, v.PostedAt(), v.PostedBy()); err != nil {
		return fmt.Errorf("post the voucher: %w", err)
	}

	return nil
}

// newUUID is the RFC 4122 v4 layout, from crypto/rand.
func newUUID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic("ledger: crypto/rand failed: " + err.Error())
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

// isUUID reports whether an id is already in the form the database key needs.
func isUUID(s string) bool {
	if len(s) != 36 {
		return false
	}
	for i, r := range s {
		switch i {
		case 8, 13, 18, 23:
			if r != '-' {
				return false
			}
		default:
			if !((r >= '0' && r <= '9') || (r >= 'a' && r <= 'f') || (r >= 'A' && r <= 'F')) {
				return false
			}
		}
	}
	return true
}

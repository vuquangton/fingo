package books

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"fingo/internal/audit"
	"fingo/internal/close"
	"fingo/internal/document"
	"fingo/internal/domain/gl"
	postledger "fingo/internal/ledger"
	"fingo/internal/money"
	"fingo/internal/period"
	"fingo/internal/usecase/ledger"
)

// CloseService builds the period close over these books.
//
// One value implements all five of the close's ports because every one of them is
// a question about the same client's rows, asked in the same transaction scope.
func (b *Books) CloseService() (*close.Service, error) {
	ports := closePorts{b: b}
	return close.NewService(ports, ports, ports, ports, ports, close.TT133ClosingAccounts())
}

// ClosePeriod closes one period and records its report.
//
// The lock row itself records the operation, not a person: the engine's Lock port
// does not carry the operator. The audit row is written here, where the command's
// operator is known, so "every post and lock is audited" is true rather than
// nearly true.
func (b *Books) ClosePeriod(ctx context.Context, cmd close.Command) (close.Report, error) {
	service, err := b.CloseService()
	if err != nil {
		return close.Report{}, err
	}

	report, err := service.ClosePeriod(ctx, cmd)
	if err != nil {
		return close.Report{}, err
	}

	after, err := json.Marshal(map[string]string{
		"period_id": cmd.PeriodID,
		"locked_at": time.Now().UTC().Format(time.RFC3339),
	})
	if err != nil {
		return report, fmt.Errorf("books: encode the lock audit: %w", err)
	}

	if err := b.store.InClient(ctx, b.scope, func(tx pgx.Tx) error {
		return audit.Record(ctx, tx, audit.Event{
			ClientID: b.scope.ID(),
			Actor:    cmd.Operator,
			Action:   audit.ActionLock,
			Entity:   "accounting_period",
			EntityID: cmd.PeriodID,
			After:    after,
		})
	}); err != nil {
		return report, fmt.Errorf("books: the period is closed but the lock was not audited: %w", err)
	}

	return report, nil
}

// ClosingReport reads back what a close recorded.
func (b *Books) ClosingReport(ctx context.Context, periodID string) (close.Report, error) {
	service, err := b.CloseService()
	if err != nil {
		return close.Report{}, err
	}
	return service.Report(ctx, periodID)
}

type closePorts struct {
	b *Books
}

// List implements close.Periods.
func (p closePorts) List(ctx context.Context) ([]period.Period, error) {
	return p.b.List(ctx)
}

// Lock implements close.Periods. The close's Lock does not carry the operator, so
// the row records the operation rather than a person; the audit trail has the
// person, written by the posting path.
func (p closePorts) Lock(ctx context.Context, periodID string, at time.Time) error {
	return p.b.store.InClient(ctx, p.b.scope, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `
			INSERT INTO accounting_period_locks (client_id, period_id, locked_by, locked_at)
			VALUES ($1, $2, 'period-close', $3)
			ON CONFLICT (period_id) DO NOTHING`,
			p.b.scope.ID(), periodID, at)
		return err
	})
}

// Balances implements close.Ledger: the revenue and expense accounts, in each
// account's own sense, plus the profit-determination account even when it is zero.
func (p closePorts) Balances(ctx context.Context, per period.Period) ([]close.AccountBalance, error) {
	var out []close.AccountBalance

	err := p.b.store.Read(ctx, p.b.scope, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			SELECT a.code, a.account_type,
			       COALESCE((
			           SELECT SUM(CASE WHEN l.side = 'CREDIT' THEN l.amount ELSE -l.amount END)
			             FROM voucher_lines l
			             JOIN vouchers v ON v.id = l.voucher_id
			            WHERE v.posted_at IS NOT NULL
			              AND v.voucher_date BETWEEN $1 AND $2
			              AND l.account_code = a.code
			       ), 0)
			  FROM accounts a
			 WHERE a.account_type IN ('REVENUE', 'EXPENSE') OR a.code = '911'
			 ORDER BY a.code`,
			per.StartDate, per.EndDate)
		if err != nil {
			return err
		}
		defer rows.Close()

		for rows.Next() {
			var code, accountType string
			var creditNet int64
			if err := rows.Scan(&code, &accountType, &creditNet); err != nil {
				return err
			}

			amount := creditNet
			if accountType == "EXPENSE" {
				// An expense is positive when spent, which is the debit side.
				amount = -creditNet
			}

			out = append(out, close.AccountBalance{
				AccountCode: code,
				Type:        accountTypeOf(accountType),
				Amount:      money.FromInt(amount),
			})
		}
		return rows.Err()
	})
	if err != nil {
		return nil, err
	}

	return out, nil
}

// Post implements close.Poster: the closing voucher travels the ordinary posting
// path, so it cannot skip the chart, the period calendar, or the audit record.
func (p closePorts) Post(ctx context.Context, posting ledger.Posting) error {
	if posting.Voucher == nil {
		return errors.New("books: a closing posting has no voucher")
	}

	_, err := postledger.PostVoucher(ctx, p.b.store, p.b.scope, posting.Voucher, posting.By)
	return err
}

// OpenInPeriod implements close.Documents.
func (p closePorts) OpenInPeriod(ctx context.Context, periodID string) ([]document.Document, error) {
	var out []document.Document

	err := p.b.store.Read(ctx, p.b.scope, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			SELECT d.id, COALESCE(d.partner_id::text, ''), d.document_date, d.total, d.allocated, d.status
			  FROM documents d
			  JOIN accounting_periods p ON p.id = $1
			 WHERE d.document_date BETWEEN p.start_date AND p.end_date
			   AND d.status IN ('OPEN', 'PARTIAL')
			 ORDER BY d.document_date, d.id`, periodID)
		if err != nil {
			return err
		}
		defer rows.Close()

		for rows.Next() {
			var (
				doc       document.Document
				total     int64
				allocated int64
				status    string
			)
			if err := rows.Scan(&doc.ID, &doc.PartnerID, &doc.Date, &total, &allocated, &status); err != nil {
				return err
			}
			doc.PeriodID = periodID
			doc.Total = money.New(total, "VND")
			doc.Allocated = money.New(allocated, "VND")
			doc.Status = document.Status(status)
			out = append(out, doc)
		}
		return rows.Err()
	})

	return out, err
}

// Save implements close.Reports.
func (p closePorts) Save(ctx context.Context, r close.Report) error {
	payload, err := json.Marshal(r)
	if err != nil {
		return fmt.Errorf("encode the closing report: %w", err)
	}

	return p.b.store.InClient(ctx, p.b.scope, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `
			INSERT INTO period_close_reports (period_id, client_id, report)
			VALUES ($1, $2, $3)
			ON CONFLICT (period_id) DO UPDATE SET report = EXCLUDED.report, closed_at = now()`,
			r.PeriodID, p.b.scope.ID(), payload)
		return err
	})
}

// Get implements close.Reports.
func (p closePorts) Get(ctx context.Context, periodID string) (close.Report, error) {
	var payload []byte

	err := p.b.store.Read(ctx, p.b.scope, func(tx pgx.Tx) error {
		return tx.QueryRow(ctx,
			`SELECT report FROM period_close_reports WHERE period_id = $1`, periodID).Scan(&payload)
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return close.Report{}, close.ErrNoReport
	}
	if err != nil {
		return close.Report{}, err
	}

	var r close.Report
	if err := json.Unmarshal(payload, &r); err != nil {
		return close.Report{}, fmt.Errorf("decode the closing report: %w", err)
	}
	return r, nil
}

func accountTypeOf(s string) gl.AccountType { return gl.AccountType(s) }

// ClosePreview is what a close would write, without writing it.
type ClosePreview struct {
	PeriodID      string
	PeriodLabel   string
	StartDate     time.Time
	EndDate       time.Time
	Lines         []gl.VoucherLine
	Result        money.Dong
	OpenDocuments int
	DocumentTotal string
}

// PreviewClose computes the closing journal for a period.
//
// It posts nothing and locks nothing. The figures come from the same engine call
// the close itself uses, so a confirmation screen cannot show a different set of
// numbers from the ones that will be posted.
func (b *Books) PreviewClose(ctx context.Context, periodID string) (ClosePreview, error) {
	periods, err := b.List(ctx)
	if err != nil {
		return ClosePreview{}, err
	}

	var chosen *period.Period
	for i := range periods {
		if periods[i].ID == periodID {
			chosen = &periods[i]
			break
		}
	}
	if chosen == nil {
		return ClosePreview{}, fmt.Errorf("books: period %s is not in the calendar", periodID)
	}

	ports := closePorts{b: b}

	balances, err := ports.Balances(ctx, *chosen)
	if err != nil {
		return ClosePreview{}, err
	}

	lines, result, err := close.ClosingEntries(balances, close.TT133ClosingAccounts())
	if err != nil {
		return ClosePreview{}, err
	}

	preview := ClosePreview{
		PeriodID:    chosen.ID,
		PeriodLabel: chosen.StartDate.Format("01/2006"),
		StartDate:   chosen.StartDate,
		EndDate:     chosen.EndDate,
		Lines:       lines,
		Result:      result,
	}
	return preview, nil
}

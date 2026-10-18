package books

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/jackc/pgx/v5"

	"fingo/internal/money"
)

// Aging buckets, by how long a document has been overdue.
type AgingBuckets struct {
	Current money.Dong
	Days30  money.Dong
	Days60  money.Dong
	Days90  money.Dong
	Older   money.Dong
}

// Total is everything still owed, however old.
func (b AgingBuckets) Total() money.Dong {
	return b.Current.Add(b.Days30).Add(b.Days60).Add(b.Days90).Add(b.Older)
}

// AgingRow is one partner's outstanding documents, bucketed by age.
type AgingRow struct {
	PartnerID string
	Partner   string
	AgingBuckets
	Documents int
}

// AgingReport is what is owed, by age, and what the ledger says is owed.
//
// The control figures are read from the voucher lines, not from the documents:
// the subledger and the control account are two independent records of the same
// debt, and an aging report is only worth reading if it says whether they agree.
type AgingReport struct {
	AsOf    time.Time
	Kind    string // "AR" or "AP"
	Title   string
	Control string

	Rows   []AgingRow
	Totals AgingBuckets

	ControlBalance money.Dong
}

// Difference is what the subledger says outstanding beyond the control account.
// Zero is the only acceptable answer in a working set of books.
func (r AgingReport) Difference() money.Dong {
	return r.Totals.Total().Sub(r.ControlBalance)
}

// Aging tallies open documents by how long they have been overdue.
//
// A document with no due date is aged from its document date plus thirty days:
// the invoice date is a fact, the payment term is a policy, and a report that
// quietly treated a missing term as "due immediately" would overstate the
// overdue column for every client who never fills it in.
func (b *Books) Aging(ctx context.Context, kind string, asOf time.Time) (AgingReport, error) {
	var (
		docKind     string
		title       string
		control     string
		controlSide string
	)

	switch kind {
	case "AR":
		docKind, title, control, controlSide = "SALES_INVOICE", "Phải thu khách hàng", "131", "debit"
	case "AP":
		docKind, title, control, controlSide = "PURCHASE_BILL", "Phải trả nhà cung cấp", "331", "credit"
	default:
		return AgingReport{}, fmt.Errorf("books: aging kind %q is neither AR nor AP", kind)
	}

	asOf = time.Date(asOf.Year(), asOf.Month(), asOf.Day(), 0, 0, 0, 0, time.UTC)
	report := AgingReport{AsOf: asOf, Kind: kind, Title: title, Control: control}

	rows := map[string]*AgingRow{}

	err := b.store.Read(ctx, b.scope, func(tx pgx.Tx) error {
		docs, err := tx.Query(ctx, `
			SELECT COALESCE(d.partner_id::text, ''), COALESCE(p.name, ''),
			       COALESCE(d.due_date, d.document_date + 30), d.total - d.allocated
			  FROM documents d
			  LEFT JOIN partners p ON p.id = d.partner_id
			 WHERE d.kind = $1
			   AND d.status IN ('OPEN', 'PARTIAL')
			   AND d.total > d.allocated`, docKind)
		if err != nil {
			return err
		}
		defer docs.Close()

		for docs.Next() {
			var (
				partnerID, partnerName string
				due                    time.Time
				outstanding            int64
			)
			if err := docs.Scan(&partnerID, &partnerName, &due, &outstanding); err != nil {
				return err
			}

			row, seen := rows[partnerID]
			if !seen {
				row = &AgingRow{PartnerID: partnerID, Partner: partnerName}
				if row.Partner == "" {
					row.Partner = "(không ghi đối tượng)"
				}
				rows[partnerID] = row
			}
			row.Documents++

			amount := money.FromInt(outstanding)
			overdue := int(asOf.Sub(due).Hours() / 24)

			switch {
			case overdue <= 0:
				row.Current = row.Current.Add(amount)
			case overdue <= 30:
				row.Days30 = row.Days30.Add(amount)
			case overdue <= 60:
				row.Days60 = row.Days60.Add(amount)
			case overdue <= 90:
				row.Days90 = row.Days90.Add(amount)
			default:
				row.Older = row.Older.Add(amount)
			}
		}
		if err := docs.Err(); err != nil {
			return err
		}

		return tx.QueryRow(ctx, `
			SELECT COALESCE(SUM(CASE WHEN side = $2 THEN amount ELSE -amount END), 0)
			  FROM voucher_lines
			 WHERE account_code = $1
			   AND voucher_id IN (SELECT id FROM vouchers WHERE voucher_date <= $3)`,
			control, controlSide, asOf).Scan(&report.ControlBalance)
	})
	if err != nil {
		return AgingReport{}, err
	}

	report.Rows = make([]AgingRow, 0, len(rows))
	for _, row := range rows {
		report.Rows = append(report.Rows, *row)
		report.Totals.Current = report.Totals.Current.Add(row.Current)
		report.Totals.Days30 = report.Totals.Days30.Add(row.Days30)
		report.Totals.Days60 = report.Totals.Days60.Add(row.Days60)
		report.Totals.Days90 = report.Totals.Days90.Add(row.Days90)
		report.Totals.Older = report.Totals.Older.Add(row.Older)
	}

	// Oldest money first: a reader is looking for what has been outstanding
	// longest, not for alphabetical order.
	sort.Slice(report.Rows, func(i, j int) bool {
		if report.Rows[i].Older != report.Rows[j].Older {
			return report.Rows[i].Older > report.Rows[j].Older
		}
		return report.Rows[i].Total() > report.Rows[j].Total()
	})

	return report, nil
}

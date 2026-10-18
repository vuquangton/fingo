package books

import (
	"archive/zip"
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

// handoverTables is every table that holds a client's data, in an order that can
// be read back without resolving a foreign key first.
//
// The `clients` registry is deliberately absent: it is the practice's list, not
// the client's books, and handing one client another client's row would be the
// leak this whole design exists to prevent.
var handoverTables = []string{
	"accounts",
	"partners",
	"accounting_periods",
	"accounting_period_locks",
	"vouchers",
	"voucher_lines",
	"documents",
	"document_lines",
	"allocations",
	"document_credits",
	"inventory_policies",
	"stock_movements",
	"period_close_reports",
	"audit_log",
}

// Export writes one client's books as a zip of CSV files plus a manifest.
//
// It uses PostgreSQL's COPY, inside the scoped transaction, so row-level security
// is the filter: a table cannot accidentally include another client's rows,
// because a row without this client's id is not visible to the query that copies
// it. Writing a SELECT per table in Go would be a second, weaker guarantee.
func (b *Books) Export(ctx context.Context, w interface{ Write([]byte) (int, error) }) error {
	archive := zip.NewWriter(w)

	manifest := struct {
		ClientID   string           `json:"client_id"`
		ExportedAt string           `json:"exported_at"`
		Tables     map[string]int64 `json:"tables"`
		Note       string           `json:"note"`
	}{
		ClientID:   b.scope.ID(),
		ExportedAt: time.Now().UTC().Format(time.RFC3339),
		Tables:     map[string]int64{},
		Note: "CSV per table, UTF-8, header row. Amounts are whole đồng as integers. " +
			"Restore by loading these in the listed order into a database with the same schema.",
	}

	for _, table := range handoverTables {
		entry, err := archive.Create(table + ".csv")
		if err != nil {
			_ = archive.Close()
			return fmt.Errorf("books: create %s.csv: %w", table, err)
		}

		var rows int64

		err = b.store.Read(ctx, b.scope, func(tx pgx.Tx) error {
			sql := fmt.Sprintf("COPY (SELECT * FROM %s) TO STDOUT WITH (FORMAT csv, HEADER true)", quoteIdent(table))
			tag, err := tx.Conn().PgConn().CopyTo(ctx, entry, sql)
			if err != nil {
				return err
			}
			rows = tag.RowsAffected()
			return nil
		})
		if err != nil {
			_ = archive.Close()
			return fmt.Errorf("books: copy %s: %w", table, err)
		}

		manifest.Tables[table] = rows
	}

	entry, err := archive.Create("MANIFEST.json")
	if err != nil {
		_ = archive.Close()
		return fmt.Errorf("books: create MANIFEST.json: %w", err)
	}

	encoder := json.NewEncoder(entry)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(manifest); err != nil {
		_ = archive.Close()
		return fmt.Errorf("books: write MANIFEST.json: %w", err)
	}

	if err := archive.Close(); err != nil {
		return fmt.Errorf("books: close the archive: %w", err)
	}
	return nil
}

// quoteIdent quotes a table name for the COPY statement. The names are a constant
// list, but quoting them keeps the statement honest if one ever needs a capital.
func quoteIdent(name string) string {
	out := make([]byte, 0, len(name)+2)
	out = append(out, '"')
	for i := 0; i < len(name); i++ {
		if name[i] == '"' {
			out = append(out, '"')
		}
		out = append(out, name[i])
	}
	out = append(out, '"')
	return string(out)
}

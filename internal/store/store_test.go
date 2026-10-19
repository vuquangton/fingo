package store_test

import (
	"crypto/rand"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"fingo/internal/store"
	"fingo/internal/testdb"
)

// dsn is the app-role connection. One harness per test keeps the failures
// readable and lets each test own the ids it creates.
type harness struct {
	store  *store.Store
	scope  store.Scope
	client string
}

func newHarness(t *testing.T) *harness {
	t.Helper()

	cfg := testdb.Load(t)
	pool := testdb.Open(t, cfg)

	st, err := store.Open(t.Context(), store.Config{DSN: cfg.DSN(), MaxOpenConns: 4})
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(st.Close)
	_ = pool // the pool proves connectivity and schema; the store owns its own

	clientID := newUUID(t)
	scope, err := store.NewScope(clientID)
	if err != nil {
		t.Fatalf("scope: %v", err)
	}

	if _, err := st.Pool().Exec(t.Context(),
		`INSERT INTO clients (id, name, tax_id) VALUES ($1, $2, $3)`,
		clientID, "Test client "+clientID[:8], "TEST-"+clientID); err != nil {
		t.Fatalf("insert client: %v", err)
	}

	if err := st.InClient(t.Context(), scope, func(tx pgx.Tx) error {
		_, err := tx.Exec(t.Context(), `SELECT provision_client_chart($1)`, clientID)
		return err
	}); err != nil {
		t.Fatalf("provision chart: %v", err)
	}

	return &harness{store: st, scope: scope, client: clientID}
}

// post writes a voucher and its lines in one scoped transaction.
func (h *harness) post(t *testing.T, no string, lines ...[3]any) string {
	t.Helper()

	id := newUUID(t)
	date := time.Date(2026, 3, 15, 0, 0, 0, 0, time.UTC)

	err := h.store.InClient(t.Context(), h.scope, func(tx pgx.Tx) error {
		if _, err := tx.Exec(t.Context(),
			`INSERT INTO vouchers (id, client_id, voucher_no, fiscal_year, seq, voucher_date, created_by)
			 VALUES ($1, $2, $3, 2026, $4, $5, 'test')`,
			id, h.client, no, nextSeq(no), date); err != nil {
			return err
		}

		for i, line := range lines {
			if _, err := tx.Exec(t.Context(),
				`INSERT INTO voucher_lines (client_id, voucher_id, line_order, account_code, side, amount)
				 VALUES ($1, $2, $3, $4, $5, $6)`,
				h.client, id, i+1, line[0], line[1], line[2]); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("post %s: %v", no, err)
	}
	return id
}

func nextSeq(no string) int {
	// Distinct seq per voucher number, so tests never collide on the
	// (client, book, year, seq) constraint.
	var n int
	for _, r := range no {
		n = n*31 + int(r)
	}
	if n < 0 {
		n = -n
	}
	return n%1_000_000 + 1
}

func newUUID(t *testing.T) string {
	t.Helper()

	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		t.Fatalf("rand: %v", err)
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

func TestScopeIsRequired(t *testing.T) {
	cfg := testdb.Load(t)
	st, err := store.Open(t.Context(), store.Config{DSN: cfg.DSN()})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer st.Close()

	err = st.InClient(t.Context(), store.Scope{}, func(pgx.Tx) error {
		t.Fatal("fn ran without a scope")
		return nil
	})
	if err == nil {
		t.Fatal("InClient accepted a zero scope")
	}
	if !strings.Contains(err.Error(), "no client scope") {
		t.Fatalf("error = %v, want a no-scope error", err)
	}
}

func TestUnscopedQuerySeesNoRows(t *testing.T) {
	cfg := testdb.Load(t)
	pool := testdb.Open(t, cfg)

	var accounts, vouchers int
	if err := pool.QueryRow(t.Context(), `SELECT count(*) FROM accounts`).Scan(&accounts); err != nil {
		t.Fatalf("count accounts: %v", err)
	}
	if err := pool.QueryRow(t.Context(), `SELECT count(*) FROM vouchers`).Scan(&vouchers); err != nil {
		t.Fatalf("count vouchers: %v", err)
	}

	if accounts != 0 || vouchers != 0 {
		t.Fatalf("unscoped query saw %d accounts and %d vouchers, want 0 and 0: "+
			"row-level security is not applied to this role", accounts, vouchers)
	}
}

func TestAppRoleCannotBypassRLS(t *testing.T) {
	cfg := testdb.Load(t)
	pool := testdb.Open(t, cfg)

	var super, bypass bool
	if err := pool.QueryRow(t.Context(),
		`SELECT rolsuper, rolbypassrls FROM pg_roles WHERE rolname = current_user`).Scan(&super, &bypass); err != nil {
		t.Fatalf("read role flags: %v", err)
	}

	if super || bypass {
		t.Fatalf("the suite runs as a role with rolsuper=%v rolbypassrls=%v; "+
			"row-level security would not prove anything", super, bypass)
	}
}

func TestClientsCannotSeeEachOther(t *testing.T) {
	a := newHarness(t)
	b := newHarness(t)

	a.post(t, "A-1", [3]any{"111", "DEBIT", int64(1000)}, [3]any{"5111", "CREDIT", int64(1000)})
	b.post(t, "B-1", [3]any{"111", "DEBIT", int64(2000)}, [3]any{"5111", "CREDIT", int64(2000)})

	var seen int
	if err := a.store.InClient(t.Context(), a.scope, func(tx pgx.Tx) error {
		return tx.QueryRow(t.Context(), `SELECT count(*) FROM vouchers`).Scan(&seen)
	}); err != nil {
		t.Fatalf("count as A: %v", err)
	}
	if seen != 1 {
		t.Fatalf("client A sees %d vouchers, want 1", seen)
	}

	// A read of B's row by id is indistinguishable from a row that does not
	// exist: no enumeration oracle.
	var found int
	if err := a.store.InClient(t.Context(), a.scope, func(tx pgx.Tx) error {
		return tx.QueryRow(t.Context(),
			`SELECT count(*) FROM vouchers WHERE client_id = $1`, b.client).Scan(&found)
	}); err != nil {
		t.Fatalf("count B as A: %v", err)
	}
	if found != 0 {
		t.Fatalf("client A can address client B's vouchers (%d rows)", found)
	}
}

func TestWriteForAnotherClientIsRefused(t *testing.T) {
	a := newHarness(t)
	b := newHarness(t)

	err := a.store.InClient(t.Context(), a.scope, func(tx pgx.Tx) error {
		_, err := tx.Exec(t.Context(),
			`INSERT INTO vouchers (id, client_id, voucher_no, fiscal_year, seq, voucher_date, created_by)
			 VALUES ($1, $2, 'FORGED', 2026, 999, '2026-03-15', 'test')`,
			newUUID(t), b.client)
		return err
	})
	if err == nil {
		t.Fatal("a write naming another client was accepted")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "row-level security") &&
		!strings.Contains(strings.ToLower(err.Error()), "policy") {
		t.Fatalf("error = %v, want a row-level security refusal", err)
	}
}

func TestUnbalancedVoucherIsRefusedAtCommit(t *testing.T) {
	h := newHarness(t)

	err := h.store.InClient(t.Context(), h.scope, func(tx pgx.Tx) error {
		id := newUUID(t)
		if _, err := tx.Exec(t.Context(),
			`INSERT INTO vouchers (id, client_id, voucher_no, fiscal_year, seq, voucher_date, created_by)
			 VALUES ($1, $2, 'UNBALANCED', 2026, 7, '2026-03-15', 'test')`, id, h.client); err != nil {
			return err
		}
		if _, err := tx.Exec(t.Context(),
			`INSERT INTO voucher_lines (client_id, voucher_id, line_order, account_code, side, amount)
			 VALUES ($1, $2, 1, '111', 'DEBIT', 1000), ($1, $2, 2, '5111', 'CREDIT', 900)`,
			h.client, id); err != nil {
			return err
		}
		return nil
	})
	if err == nil {
		t.Fatal("an unbalanced voucher committed")
	}
	if !strings.Contains(err.Error(), "unbalanced") {
		t.Fatalf("error = %v, want an unbalanced refusal", err)
	}
}

func TestPostedVoucherIsImmutable(t *testing.T) {
	h := newHarness(t)
	id := h.post(t, "IMM-1", [3]any{"111", "DEBIT", int64(500)}, [3]any{"5111", "CREDIT", int64(500)})

	if err := h.store.InClient(t.Context(), h.scope, func(tx pgx.Tx) error {
		_, err := tx.Exec(t.Context(),
			`UPDATE vouchers SET posted_at = now(), posted_by = 'test' WHERE id = $1`, id)
		return err
	}); err != nil {
		t.Fatalf("post: %v", err)
	}

	err := h.store.InClient(t.Context(), h.scope, func(tx pgx.Tx) error {
		_, err := tx.Exec(t.Context(),
			`UPDATE vouchers SET description = 'edited after posting' WHERE id = $1`, id)
		return err
	})
	if err == nil {
		t.Fatal("a posted voucher was edited")
	}
	if !strings.Contains(err.Error(), "immutable") {
		t.Fatalf("error = %v, want an immutability refusal", err)
	}

	err = h.store.InClient(t.Context(), h.scope, func(tx pgx.Tx) error {
		_, err := tx.Exec(t.Context(), `DELETE FROM vouchers WHERE id = $1`, id)
		return err
	})
	if err == nil {
		t.Fatal("a posted voucher was deleted")
	}
}

func TestAllocationCannotExceedItsDocument(t *testing.T) {
	h := newHarness(t)
	invoice := h.post(t, "AR-1", [3]any{"131", "DEBIT", int64(1000)}, [3]any{"5111", "CREDIT", int64(1000)})
	receipt := h.post(t, "PT-1", [3]any{"111", "DEBIT", int64(1000)}, [3]any{"131", "CREDIT", int64(1000)})

	var docID string
	if err := h.store.InClient(t.Context(), h.scope, func(tx pgx.Tx) error {
		return tx.QueryRow(t.Context(),
			`INSERT INTO documents (client_id, kind, document_date, total)
			 VALUES ($1, 'SALES_INVOICE', '2026-03-15', 1000) RETURNING id`, h.client).Scan(&docID)
	}); err != nil {
		t.Fatalf("insert document: %v", err)
	}

	if err := h.store.InClient(t.Context(), h.scope, func(tx pgx.Tx) error {
		_, err := tx.Exec(t.Context(),
			`INSERT INTO allocations (client_id, document_id, voucher_id, amount) VALUES ($1, $2, $3, 600)`,
			h.client, docID, invoice)
		return err
	}); err != nil {
		t.Fatalf("first allocation: %v", err)
	}

	err := h.store.InClient(t.Context(), h.scope, func(tx pgx.Tx) error {
		_, err := tx.Exec(t.Context(),
			`INSERT INTO allocations (client_id, document_id, voucher_id, amount) VALUES ($1, $2, $3, 600)`,
			h.client, docID, receipt)
		return err
	})
	if err == nil {
		t.Fatal("allocations exceeded the document total")
	}
	if !strings.Contains(err.Error(), "exceeds document") {
		t.Fatalf("error = %v, want an over-allocation refusal", err)
	}
}

func TestPeriodLockIsIrreversible(t *testing.T) {
	h := newHarness(t)

	var periodID, lockID string
	if err := h.store.InClient(t.Context(), h.scope, func(tx pgx.Tx) error {
		if err := tx.QueryRow(t.Context(),
			`INSERT INTO accounting_periods (client_id, name, start_date, end_date)
			 VALUES ($1, '2026-03', '2026-03-01', '2026-03-31') RETURNING id`, h.client).Scan(&periodID); err != nil {
			return err
		}
		return tx.QueryRow(t.Context(),
			`INSERT INTO accounting_period_locks (client_id, period_id, locked_by)
			 VALUES ($1, $2, 'test') RETURNING id`, h.client, periodID).Scan(&lockID)
	}); err != nil {
		t.Fatalf("lock period: %v", err)
	}

	err := h.store.InClient(t.Context(), h.scope, func(tx pgx.Tx) error {
		_, err := tx.Exec(t.Context(), `DELETE FROM accounting_period_locks WHERE id = $1`, lockID)
		return err
	})
	if err == nil {
		t.Fatal("a period lock was removed")
	}
	if !strings.Contains(err.Error(), "append-only") {
		t.Fatalf("error = %v, want an append-only refusal", err)
	}
}

func TestEveryBusinessTableIsClientScoped(t *testing.T) {
	cfg := testdb.Load(t)
	pool := testdb.Open(t, cfg)

	rows, err := pool.Query(t.Context(), `
		SELECT c.relname
		  FROM pg_class c
		  JOIN pg_namespace n ON n.oid = c.relnamespace
		 WHERE n.nspname = 'public'
		   AND c.relkind = 'r'
		   AND c.relname NOT IN ('clients', 'chart_template', 'users', 'sessions', 'failed_logins', 'goose_db_version', 'vat_rates', 'vat_unresolved_windows')
		   AND NOT EXISTS (
		       SELECT 1 FROM information_schema.columns col
		        WHERE col.table_schema = 'public'
		          AND col.table_name = c.relname
		          AND col.column_name = 'client_id')
		 ORDER BY c.relname`)
	if err != nil {
		t.Fatalf("list tables: %v", err)
	}
	defer rows.Close()

	var missing []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatalf("scan: %v", err)
		}
		missing = append(missing, name)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate: %v", err)
	}

	if len(missing) > 0 {
		t.Fatalf("business tables without client_id: %v", missing)
	}
}

func TestChartIsSeededForEveryClient(t *testing.T) {
	h := newHarness(t)

	var count int
	if err := h.store.InClient(t.Context(), h.scope, func(tx pgx.Tx) error {
		return tx.QueryRow(t.Context(), `SELECT count(*) FROM accounts`).Scan(&count)
	}); err != nil {
		t.Fatalf("count accounts: %v", err)
	}
	if count != 127 {
		t.Fatalf("client %s has %d accounts, want 127", h.client[:8], count)
	}
}

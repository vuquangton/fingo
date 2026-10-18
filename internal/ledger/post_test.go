package ledger_test

import (
	"crypto/rand"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"fingo/internal/domain/gl"
	"fingo/internal/ledger"
	"fingo/internal/money"
	"fingo/internal/store"
	"fingo/internal/testdb"
)

type fixture struct {
	store *store.Store
	scope store.Scope
}

func setup(t *testing.T) *fixture {
	t.Helper()

	cfg := testdb.Load(t)
	testdb.Open(t, cfg)

	st, err := store.Open(t.Context(), store.Config{DSN: cfg.DSN()})
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(st.Close)

	id := newUUID(t)
	if _, err := st.Pool().Exec(t.Context(),
		`INSERT INTO clients (id, name, tax_id) VALUES ($1, $2, $3)`,
		id, "Ledger client", "LED-"+id); err != nil {
		t.Fatalf("insert client: %v", err)
	}

	scope, err := store.NewScope(id)
	if err != nil {
		t.Fatalf("scope: %v", err)
	}

	if err := st.InClient(t.Context(), scope, func(tx pgx.Tx) error {
		_, err := tx.Exec(t.Context(), `SELECT provision_client_chart($1)`, id)
		return err
	}); err != nil {
		t.Fatalf("provision chart: %v", err)
	}

	return &fixture{store: st, scope: scope}
}

func (f *fixture) request(t *testing.T) ledger.Request {
	t.Helper()

	return ledger.Request{
		ID:          newUUID(t),
		Description: "Bán hàng cho khách",
		Actor:       "ketoan",
		Date:        time.Date(2026, 3, 15, 0, 0, 0, 0, time.UTC),
		Type:        gl.VoucherTypeGeneral,
		Lines: []ledger.Line{
			{Account: "131", Side: gl.SideDebit, Amount: money.FromInt(1_100_000)},
			{Account: "5111", Side: gl.SideCredit, Amount: money.FromInt(1_000_000)},
			{Account: "33311", Side: gl.SideCredit, Amount: money.FromInt(100_000)},
		},
	}
}

func (f *fixture) counts(t *testing.T) (vouchers, lines, audits int) {
	t.Helper()

	if err := f.store.InClient(t.Context(), f.scope, func(tx pgx.Tx) error {
		if err := tx.QueryRow(t.Context(), `SELECT count(*) FROM vouchers`).Scan(&vouchers); err != nil {
			return err
		}
		if err := tx.QueryRow(t.Context(), `SELECT count(*) FROM voucher_lines`).Scan(&lines); err != nil {
			return err
		}
		return tx.QueryRow(t.Context(), `SELECT count(*) FROM audit_log`).Scan(&audits)
	}); err != nil {
		t.Fatalf("count: %v", err)
	}
	return vouchers, lines, audits
}

func TestPostWritesVoucherLinesAndAuditInOneTransaction(t *testing.T) {
	f := setup(t)

	posted, err := ledger.Post(t.Context(), f.store, f.scope, f.request(t))
	if err != nil {
		t.Fatalf("post: %v", err)
	}

	if posted.Seq != 1 {
		t.Fatalf("seq = %d, want 1", posted.Seq)
	}
	if posted.VoucherNo == "" {
		t.Fatal("the posted voucher has no number")
	}

	vouchers, lines, audits := f.counts(t)
	if vouchers != 1 || lines != 3 || audits != 1 {
		t.Fatalf("vouchers=%d lines=%d audits=%d, want 1/3/1", vouchers, lines, audits)
	}
}

func TestPostAllocatesTheNextNumberInTheBook(t *testing.T) {
	f := setup(t)

	first, err := ledger.Post(t.Context(), f.store, f.scope, f.request(t))
	if err != nil {
		t.Fatalf("post 1: %v", err)
	}
	second, err := ledger.Post(t.Context(), f.store, f.scope, f.request(t))
	if err != nil {
		t.Fatalf("post 2: %v", err)
	}

	if first.Seq != 1 || second.Seq != 2 {
		t.Fatalf("sequences = %d, %d; want 1, 2", first.Seq, second.Seq)
	}
	if first.VoucherNo == second.VoucherNo {
		t.Fatalf("two vouchers share the number %q", first.VoucherNo)
	}
}

func TestUnbalancedVoucherIsRefusedAndLeavesNothing(t *testing.T) {
	f := setup(t)

	req := f.request(t)
	req.Lines = req.Lines[:2] // drop the VAT line: debit 1,100,000 against credit 1,000,000

	_, err := ledger.Post(t.Context(), f.store, f.scope, req)
	if err == nil {
		t.Fatal("an unbalanced voucher was posted")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "balance") {
		t.Fatalf("error = %v, want an unbalanced refusal", err)
	}

	vouchers, lines, audits := f.counts(t)
	if vouchers != 0 || lines != 0 || audits != 0 {
		t.Fatalf("a refused post wrote vouchers=%d lines=%d audits=%d", vouchers, lines, audits)
	}
}

func TestUnknownAccountIsRefused(t *testing.T) {
	f := setup(t)

	req := f.request(t)
	req.Lines[0].Account = "9999"

	_, err := ledger.Post(t.Context(), f.store, f.scope, req)
	if err == nil {
		t.Fatal("a voucher to an account outside the chart was posted")
	}

	vouchers, _, _ := f.counts(t)
	if vouchers != 0 {
		t.Fatalf("a refused post wrote %d vouchers", vouchers)
	}
}

func TestClosedPeriodIsRefused(t *testing.T) {
	f := setup(t)

	if err := f.store.InClient(t.Context(), f.scope, func(tx pgx.Tx) error {
		var periodID string
		if err := tx.QueryRow(t.Context(),
			`INSERT INTO accounting_periods (client_id, name, start_date, end_date)
			 VALUES ($1, '2026-03', '2026-03-01', '2026-03-31') RETURNING id`,
			f.scope.ID()).Scan(&periodID); err != nil {
			return err
		}
		_, err := tx.Exec(t.Context(),
			`INSERT INTO accounting_period_locks (client_id, period_id, locked_by) VALUES ($1, $2, 'test')`,
			f.scope.ID(), periodID)
		return err
	}); err != nil {
		t.Fatalf("lock the period: %v", err)
	}

	_, err := ledger.Post(t.Context(), f.store, f.scope, f.request(t))
	if err == nil {
		t.Fatal("a voucher was posted into a closed period")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "closed") {
		t.Fatalf("error = %v, want a closed-period refusal", err)
	}
}

func TestRequestValidation(t *testing.T) {
	f := setup(t)

	tests := []struct {
		name string
		spoil func(*ledger.Request)
	}{
		{"no id", func(r *ledger.Request) { r.ID = "" }},
		{"no description", func(r *ledger.Request) { r.Description = "" }},
		{"no actor", func(r *ledger.Request) { r.Actor = "" }},
		{"no date", func(r *ledger.Request) { r.Date = time.Time{} }},
		{"one line", func(r *ledger.Request) { r.Lines = r.Lines[:1] }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := f.request(t)
			tt.spoil(&req)

			if _, err := ledger.Post(t.Context(), f.store, f.scope, req); err == nil {
				t.Fatalf("%s: the request was accepted", tt.name)
			}
		})
	}
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

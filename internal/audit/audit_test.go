package audit_test

import (
	"crypto/rand"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"

	"fingo/internal/audit"
	"fingo/internal/store"
	"fingo/internal/testdb"
)

func setup(t *testing.T) (*store.Store, store.Scope) {
	t.Helper()

	cfg := testdb.Load(t)
	testdb.Open(t, cfg) // proves connectivity and schema

	st, err := store.Open(t.Context(), store.Config{DSN: cfg.DSN()})
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(st.Close)

	id := newUUID(t)
	if _, err := st.Pool().Exec(t.Context(),
		`INSERT INTO clients (id, name, tax_id) VALUES ($1, $2, $3)`,
		id, "Audit client", "AUD-"+id); err != nil {
		t.Fatalf("insert client: %v", err)
	}

	scope, err := store.NewScope(id)
	if err != nil {
		t.Fatalf("scope: %v", err)
	}
	return st, scope
}

func TestRecordIsWrittenWithTheChange(t *testing.T) {
	st, scope := setup(t)

	err := st.InClient(t.Context(), scope, func(tx pgx.Tx) error {
		if _, err := tx.Exec(t.Context(),
			`INSERT INTO accounting_periods (client_id, name, start_date, end_date)
			 VALUES ($1, '2026-03', '2026-03-01', '2026-03-31')`, scope.ID()); err != nil {
			return err
		}
		return audit.Record(t.Context(), tx, audit.Event{
			ClientID: scope.ID(),
			Actor:    "ketoan",
			Action:   audit.ActionLock,
			Entity:   "accounting_period",
			EntityID: "2026-03",
		})
	})
	if err != nil {
		t.Fatalf("record: %v", err)
	}

	var count int
	if err := st.InClient(t.Context(), scope, func(tx pgx.Tx) error {
		return tx.QueryRow(t.Context(), `SELECT count(*) FROM audit_log`).Scan(&count)
	}); err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 1 {
		t.Fatalf("audit rows = %d, want 1", count)
	}
}

func TestRollbackLeavesNoAuditRow(t *testing.T) {
	st, scope := setup(t)

	want := errors.New("something failed after the audit was written")
	err := st.InClient(t.Context(), scope, func(tx pgx.Tx) error {
		if err := audit.Record(t.Context(), tx, audit.Event{
			ClientID: scope.ID(),
			Actor:    "ketoan",
			Action:   audit.ActionPost,
			Entity:   "voucher",
			EntityID: "v-1",
		}); err != nil {
			return err
		}
		return want
	})
	if !errors.Is(err, want) {
		t.Fatalf("error = %v, want the injected failure", err)
	}

	var count int
	if err := st.InClient(t.Context(), scope, func(tx pgx.Tx) error {
		return tx.QueryRow(t.Context(), `SELECT count(*) FROM audit_log`).Scan(&count)
	}); err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 0 {
		t.Fatalf("a rolled-back transaction left %d audit rows", count)
	}
}

func TestAuditLogIsAppendOnly(t *testing.T) {
	st, scope := setup(t)

	if err := st.InClient(t.Context(), scope, func(tx pgx.Tx) error {
		return audit.Record(t.Context(), tx, audit.Event{
			ClientID: scope.ID(), Actor: "ketoan", Action: audit.ActionPost,
			Entity: "voucher", EntityID: "v-1",
		})
	}); err != nil {
		t.Fatalf("record: %v", err)
	}

	err := st.InClient(t.Context(), scope, func(tx pgx.Tx) error {
		_, err := tx.Exec(t.Context(), `UPDATE audit_log SET actor = 'someone else'`)
		return err
	})
	if err == nil {
		t.Fatal("an audit row was updated")
	}
	if !strings.Contains(err.Error(), "append-only") {
		t.Fatalf("error = %v, want an append-only refusal", err)
	}
}

func TestAuditRecordRequiresAnActor(t *testing.T) {
	st, scope := setup(t)

	err := st.InClient(t.Context(), scope, func(tx pgx.Tx) error {
		return audit.Record(t.Context(), tx, audit.Event{
			ClientID: scope.ID(), Entity: "voucher", EntityID: "v-1",
		})
	})
	if err == nil {
		t.Fatal("an audit row was written with no actor")
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

package ledger_test

import (
	"context"
	"crypto/rand"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"fingo/internal/domain/gl"
	"fingo/internal/ledger"
	"fingo/internal/money"
	"fingo/internal/store"
	"fingo/internal/testdb"
)

// TestParallelPostsGetDistinctNumbers posts twenty vouchers at once into one book.
//
// Numbering is MAX(seq)+1 under a transaction-scoped advisory lock. If that lock
// is wrong, two vouchers share a number and nobody notices until an auditor asks
// why there are two GH2026-0007. This is the test that would fail.
func TestParallelPostsGetDistinctNumbers(t *testing.T) {
	cfg := testdb.Load(t)
	testdb.Open(t, cfg)

	st, err := store.Open(t.Context(), store.Config{DSN: cfg.DSN(), MaxOpenConns: 20})
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(st.Close)

	clientID := uuidString()
	if _, err := st.Pool().Exec(t.Context(),
		`INSERT INTO clients (id, name, tax_id) VALUES ($1, $2, $3)`,
		clientID, "Parallel client", "PAR-"+clientID); err != nil {
		t.Fatalf("insert client: %v", err)
	}

	scope, err := store.NewScope(clientID)
	if err != nil {
		t.Fatalf("scope: %v", err)
	}

	if err := st.InClient(t.Context(), scope, func(tx pgx.Tx) error {
		if _, err := tx.Exec(t.Context(), `SELECT provision_client_chart($1)`, clientID); err != nil {
			return err
		}
		_, err := tx.Exec(t.Context(), `
			INSERT INTO accounting_periods (client_id, name, start_date, end_date)
			VALUES ($1, 'Năm 2026', '2026-01-01', '2026-12-31')`, clientID)
		return err
	}); err != nil {
		t.Fatalf("prepare: %v", err)
	}

	const posts = 20

	var (
		wg       sync.WaitGroup
		mu       sync.Mutex
		failures []error
	)

	for i := 0; i < posts; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()

			_, err := ledger.Post(context.Background(), st, scope, ledger.Request{
				ID:          uuidString(),
				Description: fmt.Sprintf("Song song %d", i),
				Actor:       "bench",
				Date:        time.Date(2026, 3, 15, 0, 0, 0, 0, time.UTC),
				Type:        gl.VoucherTypeGeneral,
				Lines: []ledger.Line{
					{Account: "1111", Side: gl.SideDebit, Amount: money.FromInt(int64(1_000_000 + i))},
					{Account: "4111", Side: gl.SideCredit, Amount: money.FromInt(int64(1_000_000 + i))},
				},
			})
			if err != nil {
				mu.Lock()
				failures = append(failures, err)
				mu.Unlock()
			}
		}(i)
	}
	wg.Wait()

	if len(failures) > 0 {
		t.Fatalf("%d of %d posts failed; first: %v", len(failures), posts, failures[0])
	}

	var vouchers, distinctNumbers, distinctSeqs int
	var debit, credit int64
	if err := st.InClient(t.Context(), scope, func(tx pgx.Tx) error {
		if err := tx.QueryRow(t.Context(), `
			SELECT count(*), count(DISTINCT voucher_no), count(DISTINCT seq) FROM vouchers`).
			Scan(&vouchers, &distinctNumbers, &distinctSeqs); err != nil {
			return err
		}
		return tx.QueryRow(t.Context(), `
			SELECT COALESCE(SUM(CASE WHEN side = 'DEBIT' THEN amount ELSE 0 END), 0),
			       COALESCE(SUM(CASE WHEN side = 'CREDIT' THEN amount ELSE 0 END), 0)
			  FROM voucher_lines`).Scan(&debit, &credit)
	}); err != nil {
		t.Fatalf("read: %v", err)
	}

	if vouchers != posts {
		t.Fatalf("vouchers = %d, want %d", vouchers, posts)
	}
	if distinctNumbers != posts {
		t.Fatalf("%d distinct voucher numbers for %d vouchers: numbers collided", distinctNumbers, posts)
	}
	if distinctSeqs != posts {
		t.Fatalf("%d distinct sequence numbers for %d vouchers: the counter collided", distinctSeqs, posts)
	}
	if debit != credit {
		t.Fatalf("debit %d / credit %d", debit, credit)
	}
}

func uuidString() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(err)
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

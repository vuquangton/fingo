package books_test

import (
	"crypto/rand"
	"fmt"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"fingo/internal/books"
	"fingo/internal/domain/gl"
	"fingo/internal/money"
	"fingo/internal/store"
	"fingo/internal/testdb"
	"fingo/internal/trading"
)

func TestSalesInvoicePostsRevenueVATAndReceivable(t *testing.T) {
	cfg := testdb.Load(t)
	testdb.Open(t, cfg)

	st, err := store.Open(t.Context(), store.Config{DSN: cfg.DSN()})
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(st.Close)

	clientID := newUUID(t)
	if _, err := st.Pool().Exec(t.Context(),
		`INSERT INTO clients (id, name, tax_id) VALUES ($1, $2, $3)`,
		clientID, "Sales client", "SAL-"+clientID); err != nil {
		t.Fatalf("insert client: %v", err)
	}

	scope, err := store.NewScope(clientID)
	if err != nil {
		t.Fatalf("scope: %v", err)
	}

	if err := st.InClient(t.Context(), scope, func(tx pgx.Tx) error {
		_, err := tx.Exec(t.Context(), `SELECT provision_client_chart($1)`, clientID)
		return err
	}); err != nil {
		t.Fatalf("provision chart: %v", err)
	}

	b := books.Open(st, scope)

	period, err := b.EnsureCurrentYear(t.Context(), time.Date(2026, 3, 15, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("ensure period: %v", err)
	}

	customer, err := b.CreatePartner(t.Context(), books.Partner{
		Name: "Công ty Đông Á", TaxID: "0101234567", Kind: books.PartnerCustomer,
	})
	if err != nil {
		t.Fatalf("create partner: %v", err)
	}

	sales, err := b.Sales(t.Context())
	if err != nil {
		t.Fatalf("build the sales flow: %v", err)
	}

	invoice, err := sales.Invoice(t.Context(), trading.InvoiceCommand{
		InvoiceID:   newUUID(t),
		PartnerID:   customer.ID,
		Date:        time.Date(2026, 3, 15, 0, 0, 0, 0, time.UTC),
		PeriodID:    period.ID,
		Description: "Bán hàng cho Đông Á",
		Lines: []trading.InvoiceLine{
			{ItemID: "HH-01", Quantity: 2, UnitPrice: money.New(500_000, "VND"), VATRateID: "vat10"},
		},
		At: time.Now(),
		By: "ketoan",
	})
	if err != nil {
		t.Fatalf("invoice: %v", err)
	}

	if got := invoice.Total.Amount(); got != 1_100_000 {
		t.Fatalf("invoice total = %d, want 1,100,000", got)
	}
	if got := invoice.VAT.Amount(); got != 100_000 {
		t.Fatalf("VAT = %d, want 100,000", got)
	}

	// The ledger holds the three postings the business expects.
	var debitAR, creditRevenue, creditVAT int64
	if err := st.InClient(t.Context(), scope, func(tx pgx.Tx) error {
		return tx.QueryRow(t.Context(), `
			SELECT
			  COALESCE(SUM(CASE WHEN account_code = '131'   AND side = 'DEBIT'  THEN amount END), 0),
			  COALESCE(SUM(CASE WHEN account_code = '5111'  AND side = 'CREDIT' THEN amount END), 0),
			  COALESCE(SUM(CASE WHEN account_code = '33311' AND side = 'CREDIT' THEN amount END), 0)
			  FROM voucher_lines`).Scan(&debitAR, &creditRevenue, &creditVAT)
	}); err != nil {
		t.Fatalf("read the ledger: %v", err)
	}

	if debitAR != 1_100_000 {
		t.Fatalf("TK 131 debit = %d, want 1,100,000", debitAR)
	}
	if creditRevenue != 1_000_000 {
		t.Fatalf("TK 5111 credit = %d, want 1,000,000", creditRevenue)
	}
	if creditVAT != 100_000 {
		t.Fatalf("TK 33311 credit = %d, want 100,000", creditVAT)
	}

	// The receivable exists, open, for the same amount.
	doc, err := b.Documents().Get(t.Context(), invoice.ID)
	if err != nil {
		t.Fatalf("read the receivable: %v", err)
	}
	if doc.Status != "open" {
		t.Fatalf("receivable status = %q, want open", doc.Status)
	}
	if got := doc.Total.Amount(); got != 1_100_000 {
		t.Fatalf("receivable total = %d, want 1,100,000", got)
	}

	lines, err := b.Documents().Lines(t.Context(), invoice.ID)
	if err != nil {
		t.Fatalf("read the document lines: %v", err)
	}
	// The document records the postings it created: AR debit, revenue credit and
	// VAT credit. It is the same shape as the voucher, by design.
	if len(lines) != 3 {
		t.Fatalf("document lines = %d, want 3", len(lines))
	}

	var documentDebit int64
	for _, l := range lines {
		documentDebit += l.Debit.Amount()
	}
	if documentDebit != 1_100_000 {
		t.Fatalf("document debit lines total %d, want 1,100,000", documentDebit)
	}
}

var _ = gl.VoucherTypeGeneral

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

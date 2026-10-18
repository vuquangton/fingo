package practice_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/shopspring/decimal"

	"fingo/internal/adapter/mariadb/glrepo"
	"fingo/internal/adapter/mariadb/testsupport"
	"fingo/internal/domain/document"
	"fingo/internal/domain/gl"
	"fingo/internal/domain/inventory"
	"fingo/internal/domain/partner"
	"fingo/internal/domain/tax"
	"fingo/internal/period"
	"fingo/internal/usecase/ledger"
	"fingo/internal/usecase/practice"
	"fingo/internal/usecase/sales"
)

var (
	saleDate = time.Date(2026, 3, 14, 0, 0, 0, 0, time.UTC)
	postedAt = time.Date(2026, 3, 14, 10, 30, 0, 0, time.UTC)
)

// The edge in one test: open a brand-new client database, onboard it, record a
// sale, and read back books that balance.
//
// The database is created WITHOUT migrations on purpose — opening the client is
// what must bring the schema up to date.
func TestClient_OpenMigratesOnboardsAndRecordsASale(t *testing.T) {
	cfg := testsupport.Config(t)
	admin := testsupport.Admin(t, cfg)
	_, name := testsupport.EmptyDatabase(t, cfg, admin)

	ctx := context.Background()

	client, err := practice.Open(ctx, cfg, name)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() {
		if err := client.Close(); err != nil {
			t.Errorf("close: %v", err)
		}
	})

	// Open migrated the database, so the chart can now be seeded.
	if err := client.EnsureChart(ctx); err != nil {
		t.Fatalf("EnsureChart: %v", err)
	}

	// Onboarding is idempotent: opening the same client again must not duplicate
	// their chart.
	if err := client.EnsureChart(ctx); err != nil {
		t.Fatalf("EnsureChart (second call): %v", err)
	}

	seedStock(t, client, ctx)

	accounts, err := client.Accounts(ctx)
	if err != nil {
		t.Fatalf("Accounts: %v", err)
	}
	// 49 cấp-1 plus 74 prescribed cấp-2 accounts in Phụ lục 1.
	if len(accounts) != 123 {
		t.Fatalf("accounts = %d, want 123", len(accounts))
	}
	assertAccount(t, accounts, "131", "Phải thu của khách hàng", "")
	assertAccount(t, accounts, "1111", "Tiền Việt Nam", "111")
	assertAccount(t, accounts, "5111", "Doanh thu bán hàng hóa", "511")

	// Onboarding a customer goes through the domain, so the code and tax code
	// are normalised on the way in.
	customer, err := client.CreatePartner(
		ctx, "p-1", " KH001 ", "Công ty Bình Minh", "0123 456 789", partner.KindCustomer)
	if err != nil {
		t.Fatalf("CreatePartner: %v", err)
	}
	if customer.Code != "KH001" || customer.TaxCode != "0123456789" {
		t.Fatalf("partner = code %q tax %q, want them normalised",
			customer.Code, customer.TaxCode)
	}

	res, err := client.RecordSale(ctx, sales.Command{
		VoucherID:     "v-1",
		PartnerID:     customer.ID,
		WarehouseCode: "KHO1",
		Description:   "Bán hàng cho khách A",
		Date:          saleDate,
		At:            postedAt,
		Operator:      "operator-1",
		Lines: []sales.SaleLine{{
			ItemCode:  "VT01",
			Quantity:  decimal.NewFromInt(1),
			UnitPrice: decimal.NewFromInt(100),
			VAT:       tax.KindStandard,
		}},
	})
	if err != nil {
		t.Fatalf("RecordSale: %v", err)
	}
	if !res.Gross.Equal(decimal.NewFromInt(110)) || !res.COGS.Equal(decimal.NewFromInt(60)) {
		t.Fatalf("sale totals = gross %s cogs %s, want 110 / 60", res.Gross, res.COGS)
	}

	// One sale: debit 110 (receivable) + 60 (cost) = 170, and the same on the
	// credit side. A trial balance that does not balance is not a set of books.
	tb, err := client.TrialBalance(ctx)
	if err != nil {
		t.Fatalf("TrialBalance: %v", err)
	}
	if !tb.Debit.Equal(decimal.NewFromInt(170)) || !tb.Credit.Equal(decimal.NewFromInt(170)) {
		t.Fatalf("trial balance = debit %s credit %s, want 170 / 170", tb.Debit, tb.Credit)
	}

	byCode := map[string]practice.Balance{}
	for _, r := range tb.Rows {
		byCode[r.AccountCode] = r
	}
	for code, want := range map[string]int64{"131": 110, "5111": 100, "33311": 10, "632": 60, "156": 60} {
		got, ok := byCode[code]
		if !ok {
			t.Errorf("trial balance is missing account %s", code)
			continue
		}
		amount := got.Debit
		if amount.IsZero() {
			amount = got.Credit
		}
		if !amount.Equal(decimal.NewFromInt(want)) {
			t.Errorf("account %s = %s, want %d", code, amount, want)
		}
	}
}

// A draft is not part of the books, so it must not appear in a trial balance.
func TestClient_TrialBalanceIgnoresUnpostedDrafts(t *testing.T) {
	cfg := testsupport.Config(t)
	admin := testsupport.Admin(t, cfg)
	db, name := testsupport.MigratedDatabase(t, cfg, admin)
	testsupport.SeedChart(t, db)
	testsupport.SeedPartner(t, db)

	ctx := context.Background()

	// Store a balanced but UNPOSTED voucher straight through the repository.
	draft := gl.NewVoucher("v-draft", "Bút toán nháp", gl.VoucherTypeGeneral, saleDate)
	for _, l := range []gl.VoucherLine{
		{AccountCode: "111", Side: gl.SideDebit, Amount: decimal.NewFromInt(500)},
		{AccountCode: "5111", Side: gl.SideCredit, Amount: decimal.NewFromInt(500)},
	} {
		if err := draft.AddLine(l); err != nil {
			t.Fatalf("build draft: %v", err)
		}
	}
	if err := glrepo.NewVoucherStore(db).SaveVoucher(ctx, draft, ""); err != nil {
		t.Fatalf("save draft: %v", err)
	}

	client, err := practice.Open(ctx, cfg, name)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { client.Close() })

	seedStock(t, client, ctx)

	tb, err := client.TrialBalance(ctx)
	if err != nil {
		t.Fatalf("TrialBalance: %v", err)
	}
	if !tb.Debit.IsZero() || !tb.Credit.IsZero() {
		t.Fatalf("a draft must not reach the books: got debit %s credit %s, want 0 / 0",
			tb.Debit, tb.Credit)
	}
	if len(tb.Rows) != 0 {
		t.Fatalf("trial balance rows = %d, want 0", len(tb.Rows))
	}
}

// saleAt builds a sale dated at an instant, so a test can aim one at a specific
// accounting period.
func saleAt(id string, at time.Time) sales.Command {
	return sales.Command{
		VoucherID:     id,
		PartnerID:     testsupport.TestPartnerID,
		WarehouseCode: "KHO1",
		Description:   "Bán hàng cho khách A",
		Date:          at,
		At:            at,
		Operator:      "operator-1",
		Lines: []sales.SaleLine{{
			ItemCode:  "VT01",
			Quantity:  decimal.NewFromInt(1),
			UnitPrice: decimal.NewFromInt(100),
			VAT:       tax.KindStandard,
		}},
	}
}

// Period locks, end to end: a month closed in the database stops entries being
// posted into it, and reopening lets them through again.
//
// This is the control that stops a reported figure changing underneath, so it is
// asserted through the database rather than against the in-memory calendar.
func TestClient_ClosedPeriodRefusesPosting(t *testing.T) {
	cfg := testsupport.Config(t)
	admin := testsupport.Admin(t, cfg)
	db, name := testsupport.MigratedDatabase(t, cfg, admin)
	testsupport.SeedChart(t, db)
	testsupport.SeedPartner(t, db)

	ctx := context.Background()

	client, err := practice.Open(ctx, cfg, name)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { client.Close() })

	seedStock(t, client, ctx)

	february := gl.YearMonth{Year: 2026, Month: time.February}
	inFebruary := time.Date(2026, 2, 10, 9, 0, 0, 0, time.UTC)
	inMarch := time.Date(2026, 3, 10, 9, 0, 0, 0, time.UTC)

	if err := client.ClosePeriod(ctx, february, inMarch, "operator-1"); err != nil {
		t.Fatalf("ClosePeriod: %v", err)
	}

	// Closing twice is a mistake, not a no-op.
	if err := client.ClosePeriod(ctx, february, inMarch, "operator-1"); err == nil {
		t.Fatal("closing an already-closed period must be refused, got nil")
	}

	// The closure survives a round trip through the database.
	calendar, err := client.ClosedPeriods(ctx)
	if err != nil {
		t.Fatalf("ClosedPeriods: %v", err)
	}
	if !calendar.IsClosed(february) {
		t.Fatal("the closed period was not persisted")
	}

	// A February sale is refused.
	if _, err := client.RecordSale(ctx, saleAt("v-feb", inFebruary)); err == nil {
		t.Fatal("posting into a closed period must be refused, got nil")
	}

	// The same sale in March goes through.
	if _, err := client.RecordSale(ctx, saleAt("v-mar", inMarch)); err != nil {
		t.Fatalf("posting into an open period failed: %v", err)
	}

	// Reopening February lets the February sale through.
	if err := client.ReopenPeriod(ctx, february); err != nil {
		t.Fatalf("ReopenPeriod: %v", err)
	}
	if _, err := client.RecordSale(ctx, saleAt("v-feb2", inFebruary)); err != nil {
		t.Fatalf("posting after reopening failed: %v", err)
	}

	// Reopening something already open is equally a mistake.
	if err := client.ReopenPeriod(ctx, february); err == nil {
		t.Fatal("reopening a period that is not closed must be refused, got nil")
	}
}

// Numbers are allocated by the ledger: in the book that follows the voucher
// type, in sequence, and restarting each fiscal year.
//
// This is the sequencing that a global voucher_no made impossible, so it is
// asserted through the database rather than against an in-memory counter.
func TestClient_AllocatesVoucherNumbersInSequence(t *testing.T) {
	cfg := testsupport.Config(t)
	admin := testsupport.Admin(t, cfg)
	db, name := testsupport.MigratedDatabase(t, cfg, admin)
	testsupport.SeedChart(t, db)
	testsupport.SeedPartner(t, db)

	ctx := context.Background()

	client, err := practice.Open(ctx, cfg, name)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { client.Close() })

	seedStock(t, client, ctx)

	sell := func(id string, when time.Time) string {
		t.Helper()

		res, err := client.RecordSale(ctx, saleAt(id, when))
		if err != nil {
			t.Fatalf("RecordSale(%s): %v", id, err)
		}

		return res.VoucherNo
	}

	// A fresh client's first three sales take the first three places in the
	// sales invoice book for 2026.
	want := []string{"HD2026-0001", "HD2026-0002", "HD2026-0003"}
	for i, when := range []time.Time{
		time.Date(2026, 1, 15, 9, 0, 0, 0, time.UTC),
		time.Date(2026, 2, 20, 9, 0, 0, 0, time.UTC),
		time.Date(2026, 3, 10, 9, 0, 0, 0, time.UTC),
	} {
		if got := sell(fmt.Sprintf("v-2026-%d", i+1), when); got != want[i] {
			t.Errorf("sale %d numbered %q, want %q", i+1, got, want[i])
		}
	}

	// A new fiscal year starts its own run at 1.
	if got := sell("v-2027-1", time.Date(2027, 1, 5, 9, 0, 0, 0, time.UTC)); got != "HD2027-0001" {
		t.Errorf("first sale of 2027 numbered %q, want %q", got, "HD2027-0001")
	}
}

// A number that was reserved but never reached a document is reported, so a hole
// in a book is visible rather than silent.
func TestClient_ReportsNumberGaps(t *testing.T) {
	cfg := testsupport.Config(t)
	admin := testsupport.Admin(t, cfg)
	db, name := testsupport.MigratedDatabase(t, cfg, admin)
	testsupport.SeedChart(t, db)
	testsupport.SeedPartner(t, db)

	ctx := context.Background()

	client, err := practice.Open(ctx, cfg, name)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { client.Close() })

	seedStock(t, client, ctx)

	// Two sales take 0001 and 0002.
	when := time.Date(2026, 3, 10, 9, 0, 0, 0, time.UTC)
	for i := 1; i <= 2; i++ {
		if _, err := client.RecordSale(ctx, saleAt(fmt.Sprintf("v-%d", i), when)); err != nil {
			t.Fatalf("RecordSale %d: %v", i, err)
		}
	}

	// A reserved number that no document claims — the crash-between-allocating-
	// and-saving case, provoked deliberately.
	if _, err := client.NextNumber(ctx, gl.BookSalesInvoice, 2026); err != nil {
		t.Fatalf("NextNumber: %v", err)
	}

	gaps, err := client.NumberGaps(ctx)
	if err != nil {
		t.Fatalf("NumberGaps: %v", err)
	}

	if len(gaps) != 1 {
		t.Fatalf("gaps = %v, want exactly one (HD2026-0003)", gaps)
	}
	if got, want := gaps[0].String(), "HD2026-0003"; got != want {
		t.Fatalf("gap = %q, want %q", got, want)
	}
}

// The 8% reduction is available inside its window and not outside it, decided by
// the document's own date rather than by when the entry is keyed in.
//
// This is the discipline that stops a client invoicing at 8% after the reduction
// has lapsed — because the treatment, not a percentage, is what a sale carries.
func TestClient_VATReductionAppliesByDocumentDate(t *testing.T) {
	cfg := testsupport.Config(t)
	admin := testsupport.Admin(t, cfg)
	db, name := testsupport.MigratedDatabase(t, cfg, admin)
	testsupport.SeedChart(t, db)
	testsupport.SeedPartner(t, db)

	ctx := context.Background()

	client, err := practice.Open(ctx, cfg, name)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { client.Close() })

	seedStock(t, client, ctx)

	reduced := func(id string, when time.Time) (sales.Result, error) {
		return client.RecordSale(ctx, sales.Command{
			VoucherID:     id,
			PartnerID:     testsupport.TestPartnerID,
			WarehouseCode: "KHO1",
			Description:   "Bán hàng giảm thuế",
			Date:          when,
			At:            when,
			Operator:      "operator-1",
			Lines: []sales.SaleLine{{
				ItemCode:  "VT01",
				Quantity:  decimal.NewFromInt(1),
				UnitPrice: decimal.NewFromInt(1000),
				VAT:       tax.KindTemporarilyReduced,
			}},
		})
	}

	// The last day of the window is inside it.
	res, err := reduced("v-inside", time.Date(2026, 12, 31, 9, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("8%% sale on the last day of the window: %v", err)
	}
	if got, want := res.VAT.String(), "80"; got != want {
		t.Errorf("VAT = %s, want %s — 8%% of 1000", got, want)
	}

	// The day after is not. Nothing about keying it in later rescues it.
	if _, err := reduced("v-after", time.Date(2027, 1, 1, 9, 0, 0, 0, time.UTC)); err == nil {
		t.Fatal("an 8% sale dated after the window must be refused, got nil")
	}
}

// A resubmitted request posts once.
//
// This is what stops a double-clicked button — or a retried call — becoming a
// second invoice, a second VAT liability and a second receivable. Nothing
// downstream could tell that second sale apart from a real one.
func TestClient_ResubmittedRequestPostsOnce(t *testing.T) {
	cfg := testsupport.Config(t)
	admin := testsupport.Admin(t, cfg)
	db, name := testsupport.MigratedDatabase(t, cfg, admin)
	testsupport.SeedChart(t, db)
	testsupport.SeedPartner(t, db)

	ctx := context.Background()

	client, err := practice.Open(ctx, cfg, name)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { client.Close() })

	seedStock(t, client, ctx)

	first := saleAt("v-once", time.Date(2026, 3, 10, 9, 0, 0, 0, time.UTC))
	first.IdempotencyKey = "req-0001"

	if _, err := client.RecordSale(ctx, first); err != nil {
		t.Fatalf("first submit: %v", err)
	}

	// The same request again. The voucher id differs, as it would if the form
	// had been re-sent — the key is what names the request, not the attempt.
	retry := first
	retry.VoucherID = "v-once-retry"

	_, err = client.RecordSale(ctx, retry)
	if err == nil {
		t.Fatal("a resubmitted request must be refused as a duplicate, got nil")
	}
	if !errors.Is(err, gl.ErrDuplicateRequest) {
		t.Fatalf("error = %v, want it recognised as a duplicate request", err)
	}

	var posted int
	if err := db.QueryRow("SELECT COUNT(*) FROM vouchers WHERE is_posted = TRUE").Scan(&posted); err != nil {
		t.Fatalf("count posted vouchers: %v", err)
	}
	if posted != 1 {
		t.Errorf("posted vouchers = %d, want exactly 1", posted)
	}
}

// A different key is a different request, so a genuine second sale still posts.
// Idempotency must not turn into a refusal to record repeat business.
func TestClient_ADifferentKeyPostsASecondSale(t *testing.T) {
	cfg := testsupport.Config(t)
	admin := testsupport.Admin(t, cfg)
	db, name := testsupport.MigratedDatabase(t, cfg, admin)
	testsupport.SeedChart(t, db)
	testsupport.SeedPartner(t, db)

	ctx := context.Background()

	client, err := practice.Open(ctx, cfg, name)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { client.Close() })

	seedStock(t, client, ctx)

	when := time.Date(2026, 3, 10, 9, 0, 0, 0, time.UTC)

	for i, key := range []string{"req-0001", "req-0002"} {
		cmd := saleAt(fmt.Sprintf("v-%d", i+1), when)
		cmd.IdempotencyKey = key

		if _, err := client.RecordSale(ctx, cmd); err != nil {
			t.Fatalf("sale under key %s: %v", key, err)
		}
	}

	var posted int
	if err := db.QueryRow("SELECT COUNT(*) FROM vouchers WHERE is_posted = TRUE").Scan(&posted); err != nil {
		t.Fatalf("count posted vouchers: %v", err)
	}
	if posted != 2 {
		t.Errorf("posted vouchers = %d, want 2 — two different requests", posted)
	}
}

// The customer lands on the receivable line — and only there. Revenue and VAT
// belong to nobody in particular, so a sub-ledger built from every labelled line
// would overstate what the customer owes.
func TestClient_ReceivableCarriesTheCustomerAndTotals(t *testing.T) {
	cfg := testsupport.Config(t)
	admin := testsupport.Admin(t, cfg)
	db, name := testsupport.MigratedDatabase(t, cfg, admin)
	testsupport.SeedChart(t, db)

	ctx := context.Background()

	client, err := practice.Open(ctx, cfg, name)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { client.Close() })

	seedStock(t, client, ctx)

	customer, err := client.CreatePartner(
		ctx, "p-1", "KH001", "Công ty Bình Minh", "0123456789", partner.KindCustomer)
	if err != nil {
		t.Fatalf("CreatePartner: %v", err)
	}

	when := time.Date(2026, 3, 10, 9, 0, 0, 0, time.UTC)

	// Two credit sales of 100 net plus 10% VAT.
	for i := 1; i <= 2; i++ {
		cmd := saleAt(fmt.Sprintf("v-%d", i), when)
		cmd.PartnerID = customer.ID

		if _, err := client.RecordSale(ctx, cmd); err != nil {
			t.Fatalf("RecordSale %d: %v", i, err)
		}
	}

	// The label sits on 131 and nowhere else.
	labelled := map[string]string{}
	rows, err := db.Query(`
		SELECT a.code, COALESCE(vl.partner_id, '')
		FROM voucher_lines vl
		JOIN accounts a ON a.id = vl.account_id`)
	if err != nil {
		t.Fatalf("read lines: %v", err)
	}
	for rows.Next() {
		var code, partnerID string
		if err := rows.Scan(&code, &partnerID); err != nil {
			t.Fatalf("scan line: %v", err)
		}
		labelled[code] = partnerID
	}
	rows.Close()

	if got := labelled["131"]; got != customer.ID {
		t.Errorf("the receivable line carries partner %q, want %q", got, customer.ID)
	}
	for _, code := range []string{"5111", "33311"} {
		if got := labelled[code]; got != "" {
			t.Errorf("%s carries partner %q, want nobody — it belongs to no customer", code, got)
		}
	}

	// The sub-ledger totals 131 for that customer: 110 twice.
	receivables, err := client.Receivables(ctx)
	if err != nil {
		t.Fatalf("Receivables: %v", err)
	}
	if len(receivables) != 1 {
		t.Fatalf("receivables = %d rows, want 1", len(receivables))
	}
	if got, want := receivables[0].Amount.String(), "220"; got != want {
		t.Errorf("receivable = %s, want %s", got, want)
	}
	if receivables[0].Partner.Code != "KH001" {
		t.Errorf("partner code = %q, want KH001", receivables[0].Partner.Code)
	}
}

// A payment settles a partner's invoices oldest first, and the books can say
// afterwards which invoice is still owed and by how much.
//
// This is what turns "TK 131 is 300" into an aging position somebody can act on.
func TestClient_SettlesDocumentsOldestFirst(t *testing.T) {
	cfg := testsupport.Config(t)
	admin := testsupport.Admin(t, cfg)
	db, name := testsupport.MigratedDatabase(t, cfg, admin)
	testsupport.SeedChart(t, db)

	ctx := context.Background()

	client, err := practice.Open(ctx, cfg, name)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { client.Close() })

	seedStock(t, client, ctx)

	customer, err := client.CreatePartner(
		ctx, "p-1", "KH001", "Công ty Bình Minh", "", partner.KindCustomer)
	if err != nil {
		t.Fatalf("CreatePartner: %v", err)
	}

	// Two credit sales. Each is one unit at 100 plus 10% VAT, so each invoice is
	// worth 110 — and the invoice is recorded by the sale itself, in the sale's
	// own transaction.
	invoices := []struct {
		voucherID string
		reference string
		day       time.Time
	}{
		{"v-jan", "HD001", time.Date(2026, 1, 10, 9, 0, 0, 0, time.UTC)},
		{"v-feb", "HD002", time.Date(2026, 2, 10, 9, 0, 0, 0, time.UTC)},
	}

	for _, inv := range invoices {
		cmd := saleAt(inv.voucherID, inv.day)
		cmd.PartnerID = customer.ID
		cmd.DocumentReference = inv.reference
		cmd.DueDate = inv.day.AddDate(0, 1, 0)

		if _, err := client.RecordSale(ctx, cmd); err != nil {
			t.Fatalf("RecordSale %s: %v", inv.voucherID, err)
		}
	}

	// Both invoices are open, oldest first.
	open, err := client.Outstanding(ctx, customer.ID)
	if err != nil {
		t.Fatalf("Outstanding: %v", err)
	}
	if len(open) != 2 {
		t.Fatalf("open documents = %d, want 2", len(open))
	}
	if open[0].Document.Reference != "HD001" || !open[0].Outstanding.Equal(decimal.NewFromInt(110)) {
		t.Errorf("first open = %s/%s, want HD001/110", open[0].Document.Reference, open[0].Outstanding)
	}

	// A cash receipt of 150, dated after both invoices.
	paymentAt := time.Date(2026, 3, 1, 9, 0, 0, 0, time.UTC)
	payment := gl.NewVoucher("v-pay", "Thu tiền khách Bình Minh", gl.VoucherTypeCashReceipt, paymentAt)

	for _, line := range []gl.VoucherLine{
		{AccountCode: "1111", Side: gl.SideDebit, Amount: decimal.NewFromInt(150)},
		{AccountCode: "131", Side: gl.SideCredit, Amount: decimal.NewFromInt(150), PartnerID: customer.ID},
	} {
		if err := payment.AddLine(line); err != nil {
			t.Fatalf("AddLine %s: %v", line.AccountCode, err)
		}
	}

	if err := client.Ledger().Post(ctx, ledger.Posting{
		Voucher:    payment,
		At:         paymentAt,
		By:         "operator-1",
		RequestKey: "req-pay",
	}); err != nil {
		t.Fatalf("post payment: %v", err)
	}

	plan, err := client.Settle(ctx, customer.ID, "v-pay", decimal.NewFromInt(150), paymentAt)
	if err != nil {
		t.Fatalf("Settle: %v", err)
	}

	if len(plan.Allocations) != 2 {
		t.Fatalf("allocations = %v, want HD001 in full and HD002 in part", plan.Allocations)
	}
	if plan.Allocations[0].DocumentID != "doc-v-jan" ||
		!plan.Allocations[0].Amount.Equal(decimal.NewFromInt(110)) {
		t.Errorf("first allocation = %v, want all of the January invoice", plan.Allocations[0])
	}
	if plan.Allocations[1].DocumentID != "doc-v-feb" ||
		!plan.Allocations[1].Amount.Equal(decimal.NewFromInt(40)) {
		t.Errorf("second allocation = %v, want 40 of the February invoice", plan.Allocations[1])
	}
	if !plan.Unapplied.IsZero() {
		t.Errorf("Unapplied = %s, want 0", plan.Unapplied)
	}

	// January is clear; February still owes 70.
	open, err = client.Outstanding(ctx, customer.ID)
	if err != nil {
		t.Fatalf("Outstanding: %v", err)
	}
	if !open[0].Outstanding.IsZero() {
		t.Errorf("HD001 outstanding = %s, want 0", open[0].Outstanding)
	}
	if got, want := open[1].Outstanding.String(), "70"; got != want {
		t.Errorf("HD002 outstanding = %s, want %s", got, want)
	}
}

// The database refuses an allocation beyond what a document owes, whatever
// reaches it. The domain already refuses it; this is the guarantee that survives
// a repair script or a future bug.
func TestClient_DatabaseRefusesOverAllocation(t *testing.T) {
	cfg := testsupport.Config(t)
	admin := testsupport.Admin(t, cfg)
	db, name := testsupport.MigratedDatabase(t, cfg, admin)
	testsupport.SeedChart(t, db)

	ctx := context.Background()

	client, err := practice.Open(ctx, cfg, name)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { client.Close() })

	seedStock(t, client, ctx)

	customer, err := client.CreatePartner(
		ctx, "p-1", "KH001", "Công ty Bình Minh", "", partner.KindCustomer)
	if err != nil {
		t.Fatalf("CreatePartner: %v", err)
	}

	when := time.Date(2026, 1, 10, 9, 0, 0, 0, time.UTC)
	cmd := saleAt("v-jan", when)
	cmd.PartnerID = customer.ID
	if _, err := client.RecordSale(ctx, cmd); err != nil {
		t.Fatalf("RecordSale: %v", err)
	}

	if err := client.IssueDocument(ctx, document.Document{
		ID:           "doc-1",
		PartnerID:    customer.ID,
		VoucherID:    "v-jan",
		Kind:         document.KindSalesInvoice,
		Reference:    "HD001",
		DocumentDate: when,
		Amount:       decimal.NewFromInt(100),
	}); err != nil {
		t.Fatalf("IssueDocument: %v", err)
	}

	if _, err := db.Exec(`
		INSERT INTO allocations (id, document_id, voucher_id, amount, allocated_at)
		VALUES ('a-over', 'doc-1', 'v-jan', 150, NOW(6))`); err == nil {
		t.Fatal("allocating 150 against a 100 invoice must be refused, got nil")
	}
}

// Stock survives a restart, and reopening the books replays the same balance
// from the same history.
//
// Nothing stores a running total, so this is really a test that the log is
// complete and ordered: if a movement were lost or reordered, the reopened
// balance would differ.
func TestClient_StockSurvivesReopening(t *testing.T) {
	cfg := testsupport.Config(t)
	admin := testsupport.Admin(t, cfg)
	db, name := testsupport.MigratedDatabase(t, cfg, admin)
	testsupport.SeedChart(t, db)

	ctx := context.Background()
	at := time.Date(2026, 3, 10, 9, 0, 0, 0, time.UTC)

	client, err := practice.Open(ctx, cfg, name)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}

	// 10 units for 100,000 then 10 more for 140,000: a moving average of 12,000.
	if _, err := client.ReceiveStock(ctx, "VT01", "KHO1", decimal.NewFromInt(10), decimal.NewFromInt(100_000), "", at); err != nil {
		t.Fatalf("first receipt: %v", err)
	}
	if _, err := client.ReceiveStock(ctx, "VT01", "KHO1", decimal.NewFromInt(10), decimal.NewFromInt(140_000), "", at.Add(time.Hour)); err != nil {
		t.Fatalf("second receipt: %v", err)
	}

	cost, _, err := client.IssueStock(ctx, "VT01", "KHO1", decimal.NewFromInt(5), "", at.Add(2*time.Hour))
	if err != nil {
		t.Fatalf("issue: %v", err)
	}
	if got, want := cost.String(), "60000"; got != want {
		t.Errorf("cost of 5 at the moving average = %s, want %s", got, want)
	}

	before, err := client.StockPosition(ctx, "VT01", "KHO1")
	if err != nil {
		t.Fatalf("StockPosition: %v", err)
	}
	if !before.Quantity.Equal(decimal.NewFromInt(15)) || !before.Value.Equal(decimal.NewFromInt(180_000)) {
		t.Fatalf("before reopening: %s units worth %s, want 15 / 180000", before.Quantity, before.Value)
	}

	if err := client.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	// A fresh connection, reading the same history back from MariaDB.
	reopened, err := practice.Open(ctx, cfg, name)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	t.Cleanup(func() { reopened.Close() })

	after, err := reopened.StockPosition(ctx, "VT01", "KHO1")
	if err != nil {
		t.Fatalf("StockPosition after reopening: %v", err)
	}
	if !after.Quantity.Equal(before.Quantity) || !after.Value.Equal(before.Value) {
		t.Errorf("after reopening: %s units worth %s, want %s / %s",
			after.Quantity, after.Value, before.Quantity, before.Value)
	}

	// Emptying the item leaves nothing behind, exactly.
	_, empty, err := reopened.IssueStock(ctx, "VT01", "KHO1", decimal.NewFromInt(15), "", at.Add(3*time.Hour))
	if err != nil {
		t.Fatalf("issue the rest: %v", err)
	}
	if !empty.Quantity.IsZero() || !empty.Value.IsZero() {
		t.Errorf("emptied position = %s units worth %s, want nothing", empty.Quantity, empty.Value)
	}
}

// More cannot be issued than is on hand: negative stock is a stocktake problem,
// not a posting the ledger should accept.
func TestClient_RefusesToIssueStockItDoesNotHave(t *testing.T) {
	cfg := testsupport.Config(t)
	admin := testsupport.Admin(t, cfg)
	db, name := testsupport.MigratedDatabase(t, cfg, admin)
	testsupport.SeedChart(t, db)

	ctx := context.Background()
	at := time.Date(2026, 3, 10, 9, 0, 0, 0, time.UTC)

	client, err := practice.Open(ctx, cfg, name)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { client.Close() })

	if _, err := client.ReceiveStock(ctx, "VT01", "KHO1", decimal.NewFromInt(10), decimal.NewFromInt(100_000), "", at); err != nil {
		t.Fatalf("receipt: %v", err)
	}

	if _, _, err := client.IssueStock(ctx, "VT01", "KHO1", decimal.NewFromInt(11), "", at); err == nil {
		t.Fatal("issuing 11 of 10 must be refused, got nil")
	}
}

// TT133 requires one valuation method for a whole fiscal year, and the database
// holds that choice.
func TestClient_ValuationMethodIsFixedPerFiscalYear(t *testing.T) {
	cfg := testsupport.Config(t)
	admin := testsupport.Admin(t, cfg)
	db, name := testsupport.MigratedDatabase(t, cfg, admin)
	testsupport.SeedChart(t, db)

	ctx := context.Background()
	at := time.Date(2026, 1, 1, 9, 0, 0, 0, time.UTC)

	client, err := practice.Open(ctx, cfg, name)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { client.Close() })

	seedStock(t, client, ctx)

	if err := client.ChooseValuationMethod(ctx, 2026, inventory.MethodMovingAverage, at); err != nil {
		t.Fatalf("ChooseValuationMethod: %v", err)
	}

	// The choice survives a read back from the database.
	policy, err := client.InventoryPolicy(ctx)
	if err != nil {
		t.Fatalf("InventoryPolicy: %v", err)
	}
	if got, _ := policy.MethodFor(2026); got != inventory.MethodMovingAverage {
		t.Fatalf("2026 method = %s, want %s", got, inventory.MethodMovingAverage)
	}

	// Changing it within the year is refused — that is the TT133 rule.
	if err := client.ChooseValuationMethod(ctx, 2026, inventory.MethodFIFO, at); err == nil {
		t.Fatal("changing a fiscal year's method must be refused, got nil")
	}

	// A different year may use a different method.
	if err := client.ChooseValuationMethod(ctx, 2027, inventory.MethodFIFO, at); err != nil {
		t.Fatalf("choosing the method for 2027: %v", err)
	}
}

// A movement is a record: the log cannot be edited to make the balance come out
// differently.
func TestClient_StockMovementsAreAppendOnly(t *testing.T) {
	cfg := testsupport.Config(t)
	admin := testsupport.Admin(t, cfg)
	db, name := testsupport.MigratedDatabase(t, cfg, admin)
	testsupport.SeedChart(t, db)

	ctx := context.Background()
	at := time.Date(2026, 3, 10, 9, 0, 0, 0, time.UTC)

	client, err := practice.Open(ctx, cfg, name)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { client.Close() })

	seedStock(t, client, ctx)

	if _, err := client.ReceiveStock(ctx, "VT01", "KHO1", decimal.NewFromInt(10), decimal.NewFromInt(100_000), "", at); err != nil {
		t.Fatalf("receipt: %v", err)
	}

	if _, err := db.Exec("UPDATE stock_movements SET cost = 1"); err == nil {
		t.Error("editing a stock movement must be refused, got nil")
	}
	if _, err := db.Exec("DELETE FROM stock_movements"); err == nil {
		t.Error("deleting a stock movement must be refused, got nil")
	}
}

// seedStock puts goods in the warehouse so a test can sell them.
//
// What they cost is no longer the test's to choose: the cost of goods sold comes
// from the moving average of what was actually paid.
func seedStock(t *testing.T, client *practice.Client, ctx context.Context) {
	t.Helper()

	at := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	if _, err := client.ReceiveStock(ctx, "VT01", "KHO1",
		decimal.NewFromInt(1000), decimal.NewFromInt(60_000), "", at); err != nil {
		t.Fatalf("seed stock: %v", err)
	}
}

// A posting that cannot take its stock leaves nothing behind.
//
// The voucher, its lines and the stock movement are one write. A voucher without
// its movement would be a sale of goods that never left the warehouse: cost of
// goods on the books and the inventory still there, which is wrong in two places
// at once and reconcilable from neither.
func TestClient_APostingThatCannotTakeStockLeavesNothingBehind(t *testing.T) {
	cfg := testsupport.Config(t)
	admin := testsupport.Admin(t, cfg)
	db, name := testsupport.MigratedDatabase(t, cfg, admin)
	testsupport.SeedChart(t, db)
	testsupport.SeedPartner(t, db)

	ctx := context.Background()

	client, err := practice.Open(ctx, cfg, name)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { client.Close() })

	// Deliberately no stock: the warehouse has never held VT01.
	_, err = client.RecordSale(ctx, saleAt("v-1", time.Date(2026, 3, 10, 9, 0, 0, 0, time.UTC)))
	if err == nil {
		t.Fatal("selling goods the warehouse has never held must be refused, got nil")
	}

	for _, table := range []string{"vouchers", "voucher_lines", "stock_movements"} {
		var rows int
		if err := db.QueryRow("SELECT COUNT(*) FROM " + table).Scan(&rows); err != nil {
			t.Fatalf("count %s: %v", table, err)
		}
		if rows != 0 {
			t.Errorf("%s has %d rows after a refused posting, want none — the whole write must roll back",
				table, rows)
		}
	}
}

// And a posting that can take its stock writes all of it: the voucher, its
// lines, and the movement that relieved the warehouse.
func TestClient_ASaleWritesVoucherAndStockTogether(t *testing.T) {
	cfg := testsupport.Config(t)
	admin := testsupport.Admin(t, cfg)
	db, name := testsupport.MigratedDatabase(t, cfg, admin)
	testsupport.SeedChart(t, db)
	testsupport.SeedPartner(t, db)

	ctx := context.Background()

	client, err := practice.Open(ctx, cfg, name)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { client.Close() })

	seedStock(t, client, ctx)
	at := time.Date(2026, 3, 10, 9, 0, 0, 0, time.UTC)

	res, err := client.RecordSale(ctx, saleAt("v-1", at))
	if err != nil {
		t.Fatalf("RecordSale: %v", err)
	}

	// The cost of goods sold is the seeded moving average, not a number the test
	// chose: 1 unit out of a holding costed at 60 each.
	if got, want := res.COGS.String(), "60"; got != want {
		t.Errorf("COGS = %s, want %s — the warehouse's average, not the caller's guess", got, want)
	}

	var movements int
	if err := db.QueryRow("SELECT COUNT(*) FROM stock_movements WHERE kind = 'ISSUE'").Scan(&movements); err != nil {
		t.Fatalf("count movements: %v", err)
	}
	if movements != 1 {
		t.Errorf("issue movements = %d, want 1", movements)
	}

	// The movement points back at the voucher that caused it.
	var linked int
	if err := db.QueryRow(`
		SELECT COUNT(*) FROM stock_movements sm
		JOIN vouchers v ON v.id = sm.voucher_id
		WHERE v.voucher_no = ?`, res.VoucherNo).Scan(&linked); err != nil {
		t.Fatalf("count linked movements: %v", err)
	}
	if linked != 1 {
		t.Errorf("stock movements linked to %s = %d, want 1", res.VoucherNo, linked)
	}

	// And the warehouse is one unit lighter.
	position, err := client.StockPosition(ctx, "VT01", "KHO1")
	if err != nil {
		t.Fatalf("StockPosition: %v", err)
	}
	if got, want := position.Quantity.String(), "999"; got != want {
		t.Errorf("quantity on hand = %s, want %s", got, want)
	}
}

// A sale that names an invoice number records the invoice, in the sale's own
// transaction.
//
// A voucher without its invoice is an amount sitting in TK 131 with no document
// behind it: nothing to settle against, nothing to age, nothing to chase.
func TestClient_ASaleRecordsItsInvoice(t *testing.T) {
	cfg := testsupport.Config(t)
	admin := testsupport.Admin(t, cfg)
	db, name := testsupport.MigratedDatabase(t, cfg, admin)
	testsupport.SeedChart(t, db)

	ctx := context.Background()

	client, err := practice.Open(ctx, cfg, name)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { client.Close() })

	seedStock(t, client, ctx)

	customer, err := client.CreatePartner(
		ctx, "p-1", "KH001", "Công ty Bình Minh", "", partner.KindCustomer)
	if err != nil {
		t.Fatalf("CreatePartner: %v", err)
	}

	when := time.Date(2026, 3, 10, 9, 0, 0, 0, time.UTC)

	first := saleAt("v-1", when)
	first.PartnerID = customer.ID
	first.DocumentReference = "HD001"
	first.DueDate = when.AddDate(0, 1, 0)

	if _, err := client.RecordSale(ctx, first); err != nil {
		t.Fatalf("RecordSale: %v", err)
	}

	// The invoice is there, owed by the customer, for the sale's gross.
	var (
		reference string
		amount    string
		partnerID string
	)
	if err := db.QueryRow(`
		SELECT d.reference, d.amount, d.partner_id
		FROM documents d
		JOIN vouchers v ON v.id = d.voucher_id
		WHERE v.voucher_no = 'HD2026-0001'`).Scan(&reference, &amount, &partnerID); err != nil {
		t.Fatalf("read the invoice: %v", err)
	}
	if reference != "HD001" {
		t.Errorf("reference = %q, want HD001", reference)
	}
	if amount != "110" {
		t.Errorf("amount = %s, want 110 — the sale's gross, VAT included", amount)
	}
	if partnerID != customer.ID {
		t.Errorf("partner = %q, want %q", partnerID, customer.ID)
	}

	// The same invoice number from the same customer is a double entry, and the
	// whole posting is refused — voucher, lines, stock and all.
	second := saleAt("v-2", when)
	second.PartnerID = customer.ID
	second.DocumentReference = "HD001"

	if _, err := client.RecordSale(ctx, second); err == nil {
		t.Fatal("a duplicate invoice reference must be refused, got nil")
	}

	// Counted by kind, not in total: seedStock left a receipt behind, so the
	// warehouse holds more movements than the sales account for.
	for what, query := range map[string]string{
		"posted vouchers": "SELECT COUNT(*) FROM vouchers WHERE is_posted = TRUE",
		"stock issues":    "SELECT COUNT(*) FROM stock_movements WHERE kind = 'ISSUE'",
		"documents":       "SELECT COUNT(*) FROM documents",
	} {
		var rows int
		if err := db.QueryRow(query).Scan(&rows); err != nil {
			t.Fatalf("count %s: %v", what, err)
		}
		if rows != 1 {
			t.Errorf("%s = %d after the refused sale, want 1 — the whole posting must roll back",
				what, rows)
		}
	}
}

// A locked period refuses a real posting.
//
// This is the point of the lock: the guard is consulted by the ledger on the way
// to writing a voucher, so an entry into a shut period is refused before
// anything reaches the books. The guard's own tests call it directly; this one
// proves the engine calls it.
func TestClient_LockedPeriodRefusesARealPosting(t *testing.T) {
	cfg := testsupport.Config(t)
	admin := testsupport.Admin(t, cfg)
	db, name := testsupport.MigratedDatabase(t, cfg, admin)
	testsupport.SeedChart(t, db)

	ctx := context.Background()

	client, err := practice.Open(ctx, cfg, name)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { client.Close() })

	seedStock(t, client, ctx)

	customer, err := client.CreatePartner(
		ctx, "p-1", "KH001", "Công ty Bình Minh", "", partner.KindCustomer)
	if err != nil {
		t.Fatalf("CreatePartner: %v", err)
	}

	if err := client.CreatePeriod(ctx, period.Period{
		ID:        "fy2026",
		StartDate: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		EndDate:   time.Date(2026, 12, 31, 0, 0, 0, 0, time.UTC),
	}); err != nil {
		t.Fatalf("CreatePeriod: %v", err)
	}

	// While the year is open, a sale goes through — with an invoice, so the
	// check below has a document to lose as well as a voucher.
	open := saleAt("v-open", time.Date(2026, 3, 10, 9, 0, 0, 0, time.UTC))
	open.PartnerID = customer.ID
	open.DocumentReference = "HD001"

	if _, err := client.RecordSale(ctx, open); err != nil {
		t.Fatalf("a sale into an open period must be allowed: %v", err)
	}

	if err := client.LockPeriod(ctx, "fy2026", time.Date(2026, 4, 1, 9, 0, 0, 0, time.UTC)); err != nil {
		t.Fatalf("LockPeriod: %v", err)
	}

	// Now the same kind of sale is refused, and the error is the lock's own.
	shut := saleAt("v-shut", time.Date(2026, 5, 10, 9, 0, 0, 0, time.UTC))
	shut.PartnerID = customer.ID

	_, err = client.RecordSale(ctx, shut)
	if !errors.Is(err, period.ErrPeriodLocked) {
		t.Fatalf("posting into a locked period = %v, want ErrPeriodLocked", err)
	}

	// The refused sale left nothing behind: not a voucher, not a document, not a
	// stock movement. It was stopped before the ledger did any of it.
	for what, query := range map[string]string{
		"posted vouchers": "SELECT COUNT(*) FROM vouchers WHERE is_posted = TRUE",
		"documents":       "SELECT COUNT(*) FROM documents",
		"stock issues":    "SELECT COUNT(*) FROM stock_movements WHERE kind = 'ISSUE'",
	} {
		var rows int
		if err := db.QueryRow(query).Scan(&rows); err != nil {
			t.Fatalf("count %s: %v", what, err)
		}
		if rows != 1 {
			t.Errorf("%s = %d, want 1 — the refused sale must have written nothing", what, rows)
		}
	}

	// And a date outside every period is still allowed: the lock stops entries
	// into a period somebody shut, not entries into one nobody described.
	uncovered := saleAt("v-2027", time.Date(2027, 3, 10, 9, 0, 0, 0, time.UTC))
	uncovered.PartnerID = customer.ID

	if _, err := client.RecordSale(ctx, uncovered); err != nil {
		t.Fatalf("a date outside every period must still be postable: %v", err)
	}
}

func assertAccount(t *testing.T, accounts []gl.Account, code, name, parent string) {
	t.Helper()
	for _, a := range accounts {
		if a.Code != code {
			continue
		}
		if a.Name != name {
			t.Errorf("account %s name = %q, want %q", code, a.Name, name)
		}
		if a.ParentCode != parent {
			t.Errorf("account %s parent = %q, want %q", code, a.ParentCode, parent)
		}
		return
	}
	t.Errorf("account %s is missing from the chart", code)
}

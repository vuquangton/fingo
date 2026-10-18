package boot_test

import (
	"context"
	"testing"
	"time"

	"fingo/internal/books"
	"fingo/internal/boot"
	"fingo/internal/period"
	"fingo/internal/reports"
	"fingo/internal/ui/voucher"
	"fingo/internal/usecase/ledger"
)

// TestBooks_ProducesABalanceSheetOverRealPostedVouchers — the first time the
// reports layer has been wired to anything.
//
// internal/reports has had no caller outside its own tests since it was written:
// it can produce a balance sheet, and nothing has ever asked it for one. What
// this exercises is the assembly — a ledger repository asked to be three
// different ports at once — over a real provisioned database, with a real
// voucher posted and a real period to place it in.
//
// Two things are being established. That the composition works at all. And that
// a figure on the report can name the vouchers it came from, which is what
// separates a report from a printout.
func TestBooks_ProducesABalanceSheetOverRealPostedVouchers(t *testing.T) {
	bin := mariadbBin(t)
	root := t.TempDir()

	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Minute)
	defer cancel()

	live, err := boot.NewLive(boot.LiveConfig{Root: root, BinDir: bin, Port: freePort(t)})
	if err != nil {
		t.Fatalf("NewLive: %v", err)
	}
	t.Cleanup(func() { _ = live.Close(context.Background()) })

	if err := boot.Ensure(ctx, live); err != nil {
		t.Fatalf("provisioning: %v", err)
	}

	// The books, composed with the reports over them.
	held, err := books.Open(ctx, live.Connection().Config(), "fingo")
	if err != nil {
		t.Fatalf("books.Open: %v", err)
	}
	t.Cleanup(func() { _ = held.Close() })

	// A period to place the voucher in. Nothing seeds one, and every report
	// needs one.
	year := time.Now().Year()
	fy := period.Period{
		ID:        "FY" + time.Now().Format("2006"),
		StartDate: time.Date(year, 1, 1, 0, 0, 0, 0, time.UTC),
		EndDate:   time.Date(year, 12, 31, 0, 0, 0, 0, time.UTC),
	}

	if err := held.CreatePeriod(ctx, fy); err != nil {
		t.Fatalf("CreatePeriod: %v", err)
	}

	chart, err := held.Chart(ctx)
	if err != nil {
		t.Fatalf("Chart: %v", err)
	}

	accounts, err := held.Accounts(ctx)
	if err != nil {
		t.Fatalf("Accounts: %v", err)
	}

	var postable []string

	for _, a := range accounts {
		if chart.IsPostable(a.Code) {
			postable = append(postable, a.Code)
		}
	}

	if len(postable) < 2 {
		t.Fatalf("the seeded chart has %d postable accounts", len(postable))
	}

	debitAccount, creditAccount := postable[0], postable[1]

	// A voucher, typed the way an operator types one.
	grid := voucher.NewEntryGrid(chart, 2)

	for _, r := range []struct {
		row     int
		account string
		side    string
	}{
		{0, debitAccount, voucher.SideDebit},
		{1, creditAccount, voucher.SideCredit},
	} {
		for col, text := range []string{r.account, r.side, "2.500.000", "Kiểm tra báo cáo"} {
			if err := grid.SetCell(r.row, col, text); err != nil {
				t.Fatalf("SetCell: %v", err)
			}
		}
	}

	date := time.Date(year, time.Now().Month(), 15, 0, 0, 0, 0, time.UTC)

	v, err := grid.Voucher("vch-books-1", date, "Kiểm tra báo cáo")
	if err != nil {
		t.Fatalf("Voucher: %v", err)
	}

	if err := held.Ledger().Post(ctx, ledger.Posting{
		Voucher:    v,
		At:         time.Now().UTC(),
		By:         "integration-test",
		RequestKey: "vch-books-1",
	}); err != nil {
		t.Fatalf("Post: %v", err)
	}

	// And the report over it.
	sheet, err := held.BalanceSheet(ctx, fy.ID)
	if err != nil {
		t.Fatalf("BalanceSheet: %v", err)
	}

	if sheet.PeriodID != fy.ID {
		t.Errorf("the report is for %q, want %q", sheet.PeriodID, fy.ID)
	}

	// The two accounts the voucher touched must be on it, at the amount posted,
	// and each must name the voucher it came from.
	wanted := map[string]string{debitAccount: "2500000", creditAccount: "2500000"}
	found := map[string]bool{}

	rows := make([]reports.BalanceSheetRow, 0, len(sheet.Assets)+len(sheet.Liabilities))
	rows = append(rows, sheet.Assets...)
	rows = append(rows, sheet.Liabilities...)

	for _, row := range rows {
		want, ok := wanted[row.AccountCode]
		if !ok {
			continue
		}

		found[row.AccountCode] = true

		// The magnitude, not the sign. The test picks its two accounts by
		// position in the chart rather than by nature, so it may well credit an
		// asset — and a credit balance on an asset is a real thing the report
		// states as a negative rather than an error. What is under test here is
		// the assembly and the traceability, not which side of the ledger 1112
		// belongs on.
		if row.Amount.Abs().String() != want {
			t.Errorf("%s is reported at %s, want %s", row.AccountCode, row.Amount, want)
		}
		if row.SourceVoucherCount == 0 {
			t.Errorf("%s names no source voucher, so its figure cannot be traced", row.AccountCode)
		}
	}

	for code := range wanted {
		if !found[code] {
			t.Errorf("account %s is not on the balance sheet after posting to it", code)
		}
	}

	// The statement is a statement: what is owned equals what is owed.
	if !sheet.Balanced() {
		t.Errorf("the balance sheet does not balance: assets %s, liabilities+equity %s",
			sheet.TotalAssets, sheet.TotalLiabilities.Add(sheet.TotalEquity))
	}
}

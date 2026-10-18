package boot_test

import (
	"context"
	"testing"
	"time"

	"fingo/internal/boot"
	"fingo/internal/ui/voucher"
	"fingo/internal/usecase/ledger"
)

// TestPosting_AVoucherReachesTheTrialBalance — the objective's verification, at
// the level a test can reach it.
//
// A journal entry typed into the grid, converted to a ledger voucher, posted
// through the real service against a real provisioned database, and then read
// back off the trial balance. Every layer between the operator's keystroke and
// the report is in the path, and if any of them disagreed with the next, the
// figure would not be there.
//
// It lives here rather than beside the grid because provisioning is what this
// package already knows how to do, and because a UI package should not have to
// stand up a database server to test its own rules.
func TestPosting_AVoucherReachesTheTrialBalance(t *testing.T) {
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

	client, err := live.Books(ctx)
	if err != nil {
		t.Fatalf("Books: %v", err)
	}
	t.Cleanup(func() { _ = client.Close() })

	chart, err := client.Chart(ctx)
	if err != nil {
		t.Fatalf("Chart: %v", err)
	}

	// Two accounts a movement can actually be posted to. The chart has 123 of
	// them and the first two are as good as any — what matters is that they are
	// postable, because the grid refuses anything else.
	accounts, err := client.Accounts(ctx)
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
		t.Fatalf("the seeded chart has %d postable accounts, want at least 2", len(postable))
	}

	debitAccount, creditAccount := postable[0], postable[1]

	// The operator types an entry.
	grid := voucher.NewEntryGrid(chart, 2)

	rows := []struct {
		row     int
		account string
		side    string
		amount  string
	}{
		{0, debitAccount, voucher.SideDebit, "1.500.000"},
		{1, creditAccount, voucher.SideCredit, "1.500.000"},
	}

	for _, r := range rows {
		for col, text := range []string{r.account, r.side, r.amount, "Bút toán kiểm tra"} {
			if err := grid.SetCell(r.row, col, text); err != nil {
				t.Fatalf("SetCell(row %d, col %d, %q): %v", r.row, col, text, err)
			}
		}
	}

	if err := grid.Validate(); err != nil {
		t.Fatalf("the entry the operator typed is not postable: %v", err)
	}

	date := time.Date(time.Now().Year(), time.Now().Month(), 15, 0, 0, 0, 0, time.UTC)

	v, err := grid.Voucher("vch-integration-1", date, "Bút toán kiểm tra")
	if err != nil {
		t.Fatalf("Voucher: %v", err)
	}

	// It goes to the ledger.
	err = client.Ledger().Post(ctx, ledger.Posting{
		Voucher:    v,
		At:         time.Now().UTC(),
		By:         "integration-test",
		RequestKey: "vch-integration-1",
	})
	if err != nil {
		t.Fatalf("Post: %v", err)
	}

	// And it comes back off the report.
	balances, err := client.TrialBalance(ctx)
	if err != nil {
		t.Fatalf("TrialBalance: %v", err)
	}

	found := map[string]string{}

	for _, b := range balances.Rows {
		switch b.AccountCode {
		case debitAccount:
			found[debitAccount] = b.Debit.String()
		case creditAccount:
			found[creditAccount] = b.Credit.String()
		}
	}

	if got, ok := found[debitAccount]; !ok {
		t.Errorf("account %s is not on the trial balance after posting to it", debitAccount)
	} else if got != "1500000" {
		t.Errorf("%s debit = %s, want 1500000", debitAccount, got)
	}

	if got, ok := found[creditAccount]; !ok {
		t.Errorf("account %s is not on the trial balance after posting to it", creditAccount)
	} else if got != "1500000" {
		t.Errorf("%s credit = %s, want 1500000", creditAccount, got)
	}

	// The book the operator just changed still balances, which is the one thing
	// the grid was there to guarantee.
	if !balances.Debit.Equal(balances.Credit) {
		t.Errorf("the trial balance does not balance after posting: debit %s, credit %s",
			balances.Debit, balances.Credit)
	}

	// And the voucher can be read back, which is what makes the figure on the
	// report traceable to something an operator can look at.
	read, err := client.Voucher(ctx, "vch-integration-1")
	if err != nil {
		t.Fatalf("Voucher: %v", err)
	}
	if len(read.Lines()) != 2 {
		t.Errorf("the stored voucher has %d lines, want 2", len(read.Lines()))
	}
	if read.VoucherNo() == "" {
		t.Error("the stored voucher has no number; the ledger did not allocate one")
	}
}

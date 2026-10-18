package sales_test

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/shopspring/decimal"

	"fingo/internal/adapter/mariadb/glrepo"
	"fingo/internal/adapter/mariadb/postingrepo"
	"fingo/internal/adapter/mariadb/stockrepo"
	"fingo/internal/adapter/mariadb/testsupport"
	"fingo/internal/domain/gl"
	"fingo/internal/domain/inventory"
	"fingo/internal/domain/tax"
	"fingo/internal/usecase/ledger"
	"fingo/internal/usecase/sales"
)

// staticChart supplies the same TT133 chart that testsupport.SeedChart writes
// into the scratch database, so the accounts a sale names are exactly the
// accounts the database holds.
type staticChart struct{ t *testing.T }

func (s staticChart) Chart(context.Context) (*gl.ChartOfAccounts, error) {
	return testRules(s.t).Chart, nil
}

// noClosedPeriods is a calendar with nothing shut.
type noClosedPeriods struct{}

func (noClosedPeriods) ClosedPeriods(context.Context) (*gl.Periods, error) {
	return gl.NewPeriods(), nil
}

// noLocks is a client with no accounting period locked, which is the state a
// client is in before anybody shuts one.
type noLocks struct{}

func (noLocks) CheckLocked(context.Context, time.Time) error { return nil }

// stockSource answers what goods cost from the same database the posting writes
// to, so the cost of goods sold here is the real moving average rather than a
// number the test chose.
type stockSource struct {
	store *stockrepo.Store
}

func (s stockSource) CostOfIssue(
	ctx context.Context,
	itemCode, warehouseCode string,
	quantity decimal.Decimal,
) (decimal.Decimal, error) {
	position, err := s.store.Position(ctx, itemCode, warehouseCode)
	if err != nil {
		return decimal.Zero, err
	}

	_, cost, err := position.Issue(quantity)

	return cost, err
}

// seedStock puts stock in the warehouse at a known cost.
func seedStock(t *testing.T, store *stockrepo.Store, itemCode string, quantity, cost int64) {
	t.Helper()

	if err := store.Record(context.Background(), inventory.Movement{
		ItemCode:      itemCode,
		WarehouseCode: "KHO1",
		Kind:          inventory.KindReceipt,
		Quantity:      decimal.NewFromInt(quantity),
		Cost:          decimal.NewFromInt(cost),
		MovedAt:       postedAt,
	}); err != nil {
		t.Fatalf("seed stock %s: %v", itemCode, err)
	}
}

func saleCommand(id, no string, lines ...sales.SaleLine) sales.Command {
	return sales.Command{
		VoucherID:     id,
		PartnerID:     testsupport.TestPartnerID,
		WarehouseCode: "KHO1",
		Description:   "Bán hàng cho khách A",
		Date:          saleDate,
		At:            postedAt,
		Operator:      "operator-1",
		Lines:         lines,
	}
}

// The whole spine, end to end.
//
// A sale goes in as a business event, becomes a balanced posted voucher, is
// written to MariaDB, is read back, and the ledger it leaves behind balances.
// The balances are aggregated with SQL rather than through the domain, so this
// asserts what is actually in the database and not what the read path says.
func TestRecordSale_EndToEndAgainstMariaDB(t *testing.T) {
	cfg := testsupport.Config(t)
	admin := testsupport.Admin(t, cfg)
	db, _ := testsupport.MigratedDatabase(t, cfg, admin)
	testsupport.SeedChart(t, db)
	testsupport.SeedPartner(t, db)

	store := glrepo.NewVoucherStore(db)
	stock := stockrepo.NewStore(db)

	// Stock at a known cost, so the cost of goods sold is a real moving average.
	seedStock(t, stock, "VT01", 10, 600)  // 60 each
	seedStock(t, stock, "VT02", 10, 1200) // 120 each

	uc := sales.NewRecordSale(
		ledger.NewService(postingrepo.NewStore(db), store, staticChart{t}, noClosedPeriods{}, noLocks{}),
		stockSource{store: stock},
		tax.DefaultSchedule(),
	)
	ctx := context.Background()

	// Two sales, so the trial balance has something to aggregate.
	// Sale 1: net 100, VAT 10, gross 110, cost 60.
	first, err := uc.Execute(ctx, saleCommand("v-1", "HD0001", sales.SaleLine{
		ItemCode:  "VT01",
		Quantity:  decimal.NewFromInt(1),
		UnitPrice: decimal.NewFromInt(100),
		VAT:       tax.KindStandard,
	}))
	if err != nil {
		t.Fatalf("first sale: %v", err)
	}

	// Sale 2: net 400, VAT 40, gross 440, cost 240.
	if _, err := uc.Execute(ctx, saleCommand("v-2", "HD0002", sales.SaleLine{
		ItemCode:  "VT02",
		Quantity:  decimal.NewFromInt(2),
		UnitPrice: decimal.NewFromInt(200),
		VAT:       tax.KindStandard,
	})); err != nil {
		t.Fatalf("second sale: %v", err)
	}

	if !first.Gross.Equal(decimal.NewFromInt(110)) || !first.COGS.Equal(decimal.NewFromInt(60)) {
		t.Fatalf("first sale totals = gross %s cogs %s, want 110 / 60", first.Gross, first.COGS)
	}

	// What is actually in the database. Expected values are derived from the two
	// sales above, not recomputed the way the use case computes them.
	balances, debit, credit := aggregateLedger(t, db)

	want := map[string]int64{
		"131/DEBIT":    550, // 110 + 440 receivable
		"5111/CREDIT":  500, // 100 + 400 revenue, net of VAT
		"33311/CREDIT": 50,  // 10 + 40 output VAT
		"632/DEBIT":    300, // 60 + 240 cost of goods
		"156/CREDIT":   300, // inventory relieved
	}
	for key, amount := range want {
		got, ok := balances[key]
		if !ok {
			t.Errorf("no balance for %s", key)
			continue
		}
		if !got.Equal(decimal.NewFromInt(amount)) {
			t.Errorf("%s = %s, want %d", key, got, amount)
		}
	}

	if debit.Cmp(credit) != 0 {
		t.Errorf("the ledger does not balance: debit %s vs credit %s", debit, credit)
	}

	// The read path agrees with the stored rows.
	got, err := store.GetVoucher(ctx, "v-1")
	if err != nil {
		t.Fatalf("read back v-1: %v", err)
	}
	if !got.IsPosted() {
		t.Fatal("voucher came back unposted")
	}
	if !got.PostedAt().Equal(postedAt) || got.PostedBy() != "operator-1" {
		t.Fatalf("posting attribution lost: at=%v by=%q", got.PostedAt(), got.PostedBy())
	}
	if err := got.ValidateBalance(); err != nil {
		t.Fatalf("voucher read back does not balance: %v", err)
	}
}

// aggregateLedger sums posted lines per account and side, straight from SQL.
func aggregateLedger(t *testing.T, db *sql.DB) (map[string]decimal.Decimal, decimal.Decimal, decimal.Decimal) {
	t.Helper()

	rows, err := db.Query(`
		SELECT a.code, vl.side, SUM(vl.amount)
		FROM voucher_lines vl
		JOIN accounts a ON a.id = vl.account_id
		JOIN vouchers v ON v.id = vl.voucher_id
		WHERE v.is_posted = TRUE
		GROUP BY a.code, vl.side`)
	if err != nil {
		t.Fatalf("aggregate ledger: %v", err)
	}
	defer rows.Close()

	balances := make(map[string]decimal.Decimal)
	debit := decimal.Zero
	credit := decimal.Zero

	for rows.Next() {
		var code, side, amount string
		if err := rows.Scan(&code, &side, &amount); err != nil {
			t.Fatalf("scan ledger row: %v", err)
		}

		value, err := decimal.NewFromString(amount)
		if err != nil {
			t.Fatalf("balance %q for %s/%s is not a number: %v", amount, code, side, err)
		}

		balances[code+"/"+side] = value

		if side == "DEBIT" {
			debit = debit.Add(value)
		} else {
			credit = credit.Add(value)
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate ledger: %v", err)
	}

	return balances, debit, credit
}

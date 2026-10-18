package inventory_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/shopspring/decimal"

	"fingo/internal/domain/gl"
	"fingo/internal/inventory"
	"fingo/internal/money"
	"fingo/internal/posting"
)

// dec renders a whole number of minor units the way the ledger takes it.
func dec(amount int64) decimal.Decimal { return decimal.NewFromInt(amount) }

// salesRules is the chart the sales cycle posts against, checked the way the
// ledger checks it at boot.
func salesRules(t *testing.T) *posting.RuleSet {
	t.Helper()

	rules, err := posting.Load(posting.TT133Defaults())
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	return rules
}

// lineOf finds the one line on a side.
func lineOf(t *testing.T, voucher *gl.Voucher, side gl.EntrySide) gl.VoucherLine {
	t.Helper()

	for _, line := range voucher.Lines() {
		if line.Side == side {
			return line
		}
	}

	t.Fatalf("the voucher has no %s line: %+v", side, voucher.Lines())

	return gl.VoucherLine{}
}

// Selling ten units books what they cost: the cost of sales debited, inventory
// credited, for the same money.
func TestRecordIssue_SellingTenUnitsBooksWhatTheyCost(t *testing.T) {
	book, one := stocked(t)
	rules := salesRules(t)

	// Twenty at 100 and twenty at 200 — 150 each.
	if _, err := book.Record(receipt("m-1", one.ID, 20, 100)); err != nil {
		t.Fatalf("Record: %v", err)
	}
	if _, err := book.Record(receipt("m-2", one.ID, 20, 200)); err != nil {
		t.Fatalf("Record: %v", err)
	}

	entry, err := book.RecordIssue(rules, issue("m-3", one.ID, 10))
	if err != nil {
		t.Fatalf("RecordIssue: %v", err)
	}

	if got := entry.Amount; got.Amount() != 1500 || got.Currency() != vnd {
		t.Errorf("the entry moves %s, want 10 × 150 = 1500 VND", got)
	}
	if entry.UnitCost != 150 {
		t.Errorf("unit cost = %d, want 150", entry.UnitCost)
	}
	if entry.Qty != 10 {
		t.Errorf("quantity = %d, want 10", entry.Qty)
	}

	debit := lineOf(t, entry.Voucher, gl.SideDebit)
	if debit.AccountCode != "632" {
		t.Errorf("the cost of sales went to %s, want 632 from the posting rules", debit.AccountCode)
	}
	if debit.Amount.String() != "1500" {
		t.Errorf("632 was debited %s, want 1500", debit.Amount)
	}

	credit := lineOf(t, entry.Voucher, gl.SideCredit)
	if credit.AccountCode != "156" {
		t.Errorf("inventory was credited to %s, want 156 from the posting rules", credit.AccountCode)
	}
	if credit.Amount.String() != "1500" {
		t.Errorf("156 was credited %s, want 1500", credit.Amount)
	}

	if err := entry.Voucher.ValidateBalance(); err != nil {
		t.Errorf("the cost voucher does not balance: %v", err)
	}

	// And the stock says the same thing: ten units gone, at the same figure.
	level, err := book.Level(one.ID)
	if err != nil {
		t.Fatalf("Level: %v", err)
	}
	if level != 30 {
		t.Errorf("level = %d, want 30", level)
	}
}

// The entry is traceable: it names the movement, and the voucher id is derived
// from it, so the same movement cannot be booked twice.
func TestRecordIssue_IsTraceableToTheMovementThatCausedIt(t *testing.T) {
	book, one := stocked(t)
	rules := salesRules(t)

	if _, err := book.Record(receipt("m-1", one.ID, 10, 100)); err != nil {
		t.Fatalf("Record: %v", err)
	}

	first, err := book.RecordIssue(rules, issue("m-2", one.ID, 4))
	if err != nil {
		t.Fatalf("RecordIssue: %v", err)
	}

	if first.MoveID != "m-2" {
		t.Errorf("the entry names movement %q, want m-2", first.MoveID)
	}
	if first.Voucher.ID() != "cogs-m-2" {
		t.Errorf("voucher id = %q, want it derived from the movement", first.Voucher.ID())
	}
	if first.Reversal() {
		t.Error("an issue is not a reversal")
	}

	for _, line := range first.Voucher.Lines() {
		if !strings.Contains(line.Note, "m-2") {
			t.Errorf("line note %q does not name the movement it came from", line.Note)
		}
	}

	// The same movement booked again produces the same voucher id, so a retry
	// collides with the row it already wrote rather than booking the cost twice.
	again := issue("m-2", one.ID, 4)
	second, err := inventory.IssueCost(rules, again, cents(400))
	if err != nil {
		t.Fatalf("IssueCost: %v", err)
	}
	if second.Voucher.ID() != first.Voucher.ID() {
		t.Errorf("the same movement produced two vouchers: %q and %q", first.Voucher.ID(), second.Voucher.ID())
	}
}

// Three units coming back reverse what they left at, and put them back into
// stock at that same cost.
func TestRecordReturn_ReturningThreeUnitsReversesWhatTheyLeftAt(t *testing.T) {
	book, one := stocked(t)
	rules := salesRules(t)

	if _, err := book.Record(receipt("m-1", one.ID, 20, 150)); err != nil {
		t.Fatalf("Record: %v", err)
	}

	issued, err := book.RecordIssue(rules, issue("m-2", one.ID, 10))
	if err != nil {
		t.Fatalf("RecordIssue: %v", err)
	}
	if issued.Amount.Amount() != 1500 {
		t.Fatalf("the issue was worth %s, want 1500", issued.Amount)
	}

	returned := inventory.StockMove{
		ID:          "m-3",
		ItemID:      one.ID,
		Direction:   inventory.DirectionIn,
		Qty:         3,
		Date:        day(2026, 3, 20),
		SourceDocID: "credit-note-1",
	}

	reversal, err := book.RecordReturn(issued, returned)
	if err != nil {
		t.Fatalf("RecordReturn: %v", err)
	}

	if got := reversal.Amount; got.Amount() != 450 || got.Currency() != vnd {
		t.Errorf("the reversal moves %s, want 3 × 150 = 450 VND", got)
	}
	if reversal.UnitCost != 150 {
		t.Errorf("unit cost = %d, want the 150 they left at", reversal.UnitCost)
	}
	if !reversal.Reversal() {
		t.Error("the entry does not say it is a reversal")
	}
	if reversal.Reverses != "m-2" {
		t.Errorf("the reversal names %q, want the issue it reverses (m-2)", reversal.Reverses)
	}
	if reversal.Voucher.ID() != "cogs-return-m-3" {
		t.Errorf("voucher id = %q, want it derived from the returning movement", reversal.Voucher.ID())
	}

	debit := lineOf(t, reversal.Voucher, gl.SideDebit)
	if debit.AccountCode != "156" || debit.Amount.String() != "450" {
		t.Errorf("the reversal debited %s %s, want inventory 156 with 450", debit.AccountCode, debit.Amount)
	}

	credit := lineOf(t, reversal.Voucher, gl.SideCredit)
	if credit.AccountCode != "632" || credit.Amount.String() != "450" {
		t.Errorf("the reversal credited %s %s, want cost of sales 632 with 450", credit.AccountCode, credit.Amount)
	}

	if err := reversal.Voucher.ValidateBalance(); err != nil {
		t.Errorf("the reversal does not balance: %v", err)
	}

	// The goods are back in the warehouse, at what they left at.
	level, err := book.Level(one.ID)
	if err != nil {
		t.Fatalf("Level: %v", err)
	}
	if level != 13 {
		t.Errorf("level = %d, want 20 − 10 + 3", level)
	}

	valuation, err := book.Valuation(one.ID)
	if err != nil {
		t.Fatalf("Valuation: %v", err)
	}
	if valuation.AvgCost != 150 {
		t.Errorf("average = %d, want it unmoved at 150: they came back at what they left at", valuation.AvgCost)
	}

	// The issue and its reversal net to nothing.
	if issued.Amount.Amount()-reversal.Amount.Amount() != 1050 {
		t.Errorf("the issue and the reversal do not leave 1050 of cost booked")
	}
}

// Returns are counted against the issue they reverse: a run of them cannot
// together take back more than went out.
func TestRecordReturn_RefusesReturningMoreThanWentOut(t *testing.T) {
	book, one := stocked(t)
	rules := salesRules(t)

	if _, err := book.Record(receipt("m-1", one.ID, 20, 150)); err != nil {
		t.Fatalf("Record: %v", err)
	}
	issued, err := book.RecordIssue(rules, issue("m-2", one.ID, 10))
	if err != nil {
		t.Fatalf("RecordIssue: %v", err)
	}

	if _, err := book.RecordReturn(issued, returnMove("m-3", one.ID, 6)); err != nil {
		t.Fatalf("RecordReturn of 6 of 10: %v", err)
	}

	// Six have come back, so five more would be one too many.
	_, err = book.RecordReturn(issued, returnMove("m-4", one.ID, 5))
	if !errors.Is(err, inventory.ErrOverReturn) {
		t.Fatalf("RecordReturn of 5 after 6 of 10 = %v, want ErrOverReturn", err)
	}

	level, err := book.Level(one.ID)
	if err != nil {
		t.Fatalf("Level: %v", err)
	}
	if level != 16 {
		t.Errorf("level = %d, want 20 − 10 + 6: the refused return was recorded anyway", level)
	}

	// The four that have not come back still may.
	if _, err := book.RecordReturn(issued, returnMove("m-5", one.ID, 4)); err != nil {
		t.Fatalf("RecordReturn of the last 4: %v", err)
	}
}

// returnMove is goods coming back, with no cost stated: the ledger supplies what
// they left at.
func returnMove(id, itemID string, qty int) inventory.StockMove {
	return inventory.StockMove{
		ID:          id,
		ItemID:      itemID,
		Direction:   inventory.DirectionIn,
		Qty:         qty,
		Date:        day(2026, 3, 20),
		SourceDocID: "credit-note-" + id,
	}
}

// Goods do not come back at today's average. A caller that stated a different
// cost is refused rather than quietly overruled.
func TestRecordReturn_RefusesGoodsComingBackAtAnotherCost(t *testing.T) {
	book, one := stocked(t)
	rules := salesRules(t)

	if _, err := book.Record(receipt("m-1", one.ID, 20, 150)); err != nil {
		t.Fatalf("Record: %v", err)
	}
	issued, err := book.RecordIssue(rules, issue("m-2", one.ID, 10))
	if err != nil {
		t.Fatalf("RecordIssue: %v", err)
	}

	returned := returnMove("m-3", one.ID, 3)
	returned.UnitCost = cents(400)

	if _, err := book.RecordReturn(issued, returned); !errors.Is(err, inventory.ErrInvalidMove) {
		t.Fatalf("RecordReturn at 400 = %v, want ErrInvalidMove", err)
	}

	level, err := book.Level(one.ID)
	if err != nil {
		t.Fatalf("Level: %v", err)
	}
	if level != 10 {
		t.Errorf("level = %d, want the refused return left alone at 10", level)
	}
}

// A sale and its cost reach the ledger in one write, or neither does.
func TestPostSale_WritesTheRevenueAndItsCostTogether(t *testing.T) {
	book, one := stocked(t)
	rules := salesRules(t)

	if _, err := book.Record(receipt("m-1", one.ID, 20, 150)); err != nil {
		t.Fatalf("Record: %v", err)
	}
	cost, err := book.RecordIssue(rules, issue("m-2", one.ID, 10))
	if err != nil {
		t.Fatalf("RecordIssue: %v", err)
	}

	revenue := gl.NewVoucher("sale-1", "Bán hàng", gl.VoucherTypeSales, day(2026, 3, 10))
	if err := revenue.AddLine(gl.VoucherLine{AccountCode: "131", Side: gl.SideDebit, Amount: dec(2000)}); err != nil {
		t.Fatalf("AddLine: %v", err)
	}
	if err := revenue.AddLine(gl.VoucherLine{AccountCode: "5111", Side: gl.SideCredit, Amount: dec(2000)}); err != nil {
		t.Fatalf("AddLine: %v", err)
	}

	poster := &recordingPoster{}
	sale := inventory.Sale{Revenue: revenue, Cost: cost}

	if err := inventory.PostSale(context.Background(), poster, sale); err != nil {
		t.Fatalf("PostSale: %v", err)
	}

	if calls := poster.callCount(); calls != 1 {
		t.Fatalf("the ledger was called %d times, want once: one sale is one transaction", calls)
	}

	seen := poster.saleAt(0)
	if seen.Revenue == nil || seen.Cost == nil {
		t.Fatalf("the ledger was handed %+v, want the revenue and its cost in the same call", seen)
	}
	if seen.Revenue.ID() != "sale-1" || seen.Cost.Voucher.ID() != "cogs-m-2" {
		t.Errorf("the ledger was handed %q and %q", seen.Revenue.ID(), seen.Cost.Voucher.ID())
	}
	if got := seen.Cost.Amount.Amount(); got != 1500 {
		t.Errorf("the cost in the sale is %d, want 1500", got)
	}

	written := poster.written()
	if len(written) != 2 || written[0] != "sale-1" || written[1] != "cogs-m-2" {
		t.Errorf("written = %v, want both vouchers", written)
	}
}

// When the write fails, neither voucher lands. That is the whole point of the
// two travelling together.
func TestPostSale_WritesNeitherWhenTheTransactionFails(t *testing.T) {
	book, one := stocked(t)
	rules := salesRules(t)

	if _, err := book.Record(receipt("m-1", one.ID, 20, 150)); err != nil {
		t.Fatalf("Record: %v", err)
	}
	cost, err := book.RecordIssue(rules, issue("m-2", one.ID, 10))
	if err != nil {
		t.Fatalf("RecordIssue: %v", err)
	}

	revenue := gl.NewVoucher("sale-1", "Bán hàng", gl.VoucherTypeSales, day(2026, 3, 10))
	if err := revenue.AddLine(gl.VoucherLine{AccountCode: "131", Side: gl.SideDebit, Amount: dec(2000)}); err != nil {
		t.Fatalf("AddLine: %v", err)
	}
	if err := revenue.AddLine(gl.VoucherLine{AccountCode: "5111", Side: gl.SideCredit, Amount: dec(2000)}); err != nil {
		t.Fatalf("AddLine: %v", err)
	}

	poster := &recordingPoster{fail: errors.New("the disk went away")}

	err = inventory.PostSale(context.Background(), poster, inventory.Sale{Revenue: revenue, Cost: cost})
	if err == nil {
		t.Fatal("a failed write must be reported")
	}

	// One attempt, and nothing committed: the revenue and its cost roll back
	// together, so the books never hold a sale with no cost.
	if calls := poster.callCount(); calls != 1 {
		t.Errorf("the ledger was called %d times, want once", calls)
	}
	if written := poster.written(); len(written) != 0 {
		t.Errorf("written = %v after a failed transaction, want nothing", written)
	}
}

// A cost with no revenue is not a sale, and neither is a cost nobody can trace
// to a movement.
func TestPostSale_RefusesASaleThatCouldNotBePostedWhole(t *testing.T) {
	book, one := stocked(t)
	rules := salesRules(t)

	if _, err := book.Record(receipt("m-1", one.ID, 20, 150)); err != nil {
		t.Fatalf("Record: %v", err)
	}
	cost, err := book.RecordIssue(rules, issue("m-2", one.ID, 10))
	if err != nil {
		t.Fatalf("RecordIssue: %v", err)
	}

	revenue := gl.NewVoucher("sale-1", "Bán hàng", gl.VoucherTypeSales, day(2026, 3, 10))

	untraceable := *cost
	untraceable.MoveID = ""

	cases := map[string]inventory.Sale{
		"no revenue voucher":        {Cost: cost},
		"a cost with no voucher":    {Revenue: revenue, Cost: &inventory.Entry{MoveID: "m-2"}},
		"a cost naming no movement": {Revenue: revenue, Cost: &untraceable},
	}

	for name, sale := range cases {
		t.Run(name, func(t *testing.T) {
			poster := &recordingPoster{}

			if err := inventory.PostSale(context.Background(), poster, sale); !errors.Is(err, inventory.ErrInvalidMove) {
				t.Fatalf("PostSale = %v, want ErrInvalidMove", err)
			}
			if poster.callCount() != 0 {
				t.Error("a sale that could not be posted whole reached the ledger anyway")
			}
		})
	}

	// A sale of something that was never in a warehouse has no cost, and that is
	// a sale.
	if err := inventory.PostSale(context.Background(), &recordingPoster{}, inventory.Sale{Revenue: revenue}); err != nil {
		t.Errorf("a sale with no cost of goods = %v, want it accepted", err)
	}
}

// A cost entry cannot be built without knowing which accounts it touches, and
// that is a configuration fault the caller hears about rather than a panic.
func TestIssueCost_RefusesAChartThatDoesNotAnswer(t *testing.T) {
	_, one := stocked(t)

	empty, err := posting.LoadFor(nil, nil)
	if err != nil {
		t.Fatalf("LoadFor: %v", err)
	}

	if _, err := inventory.IssueCost(empty, issue("m-2", one.ID, 1), cents(100)); !errors.Is(err, posting.ErrUndefined) {
		t.Errorf("IssueCost with no rules = %v, want posting.ErrUndefined", err)
	}
	if _, err := inventory.IssueCost(nil, issue("m-2", one.ID, 1), cents(100)); !errors.Is(err, posting.ErrUndefined) {
		t.Errorf("IssueCost with nil rules = %v, want posting.ErrUndefined", err)
	}
}

// Taking goods out and booking their cost are one act. A chart that cannot say
// where the cost goes refuses the movement as well, rather than leaving the
// warehouse short with the inventory account still carrying it.
func TestRecordIssue_RecordsNothingWhenTheCostCannotBeBooked(t *testing.T) {
	book, one := stocked(t)

	if _, err := book.Record(receipt("m-1", one.ID, 10, 100)); err != nil {
		t.Fatalf("Record: %v", err)
	}

	empty, err := posting.LoadFor(nil, nil)
	if err != nil {
		t.Fatalf("LoadFor: %v", err)
	}

	if _, err := book.RecordIssue(empty, issue("m-2", one.ID, 4)); !errors.Is(err, posting.ErrUndefined) {
		t.Fatalf("RecordIssue with no rules = %v, want posting.ErrUndefined", err)
	}

	level, err := book.Level(one.ID)
	if err != nil {
		t.Fatalf("Level: %v", err)
	}
	if level != 10 {
		t.Errorf("level = %d, want 10: the refused issue must not have moved stock", level)
	}

	moves, err := book.Moves(one.ID)
	if err != nil {
		t.Fatalf("Moves: %v", err)
	}
	if len(moves) != 1 {
		t.Errorf("movements = %d, want only the receipt", len(moves))
	}
}

// Only goods leaving carry a cost of sales.
func TestIssueCost_RefusesAMovementThatIsNotGoingOut(t *testing.T) {
	_, one := stocked(t)
	rules := salesRules(t)

	if _, err := inventory.IssueCost(rules, receipt("m-1", one.ID, 1, 100), cents(100)); !errors.Is(err, inventory.ErrInvalidMove) {
		t.Errorf("IssueCost on a receipt = %v, want ErrInvalidMove", err)
	}
	if _, err := ledger(t).RecordIssue(rules, receipt("m-1", one.ID, 1, 100)); !errors.Is(err, inventory.ErrInvalidMove) {
		t.Errorf("RecordIssue on a receipt = %v, want ErrInvalidMove", err)
	}
}

// Only goods arriving reverse one.
func TestReverseCost_RefusesAMovementThatIsNotArriving(t *testing.T) {
	book, one := stocked(t)
	rules := salesRules(t)

	if _, err := book.Record(receipt("m-1", one.ID, 10, 100)); err != nil {
		t.Fatalf("Record: %v", err)
	}
	issued, err := book.RecordIssue(rules, issue("m-2", one.ID, 4))
	if err != nil {
		t.Fatalf("RecordIssue: %v", err)
	}

	if _, err := inventory.ReverseCost(issued, issue("m-3", one.ID, 2)); !errors.Is(err, inventory.ErrInvalidMove) {
		t.Errorf("ReverseCost on an issue = %v, want ErrInvalidMove", err)
	}
	if _, err := inventory.ReverseCost(nil, returnMove("m-3", one.ID, 2)); !errors.Is(err, inventory.ErrInvalidMove) {
		t.Errorf("ReverseCost with no original = %v, want ErrInvalidMove", err)
	}
}

// A cost entry that would move nothing is not worth writing.
func TestIssueCost_RefusesAnIssueThatCostNothing(t *testing.T) {
	_, one := stocked(t)
	rules := salesRules(t)

	if _, err := inventory.IssueCost(rules, issue("m-1", one.ID, 1), cents(0)); !errors.Is(err, inventory.ErrInvalidMove) {
		t.Errorf("IssueCost of nothing = %v, want ErrInvalidMove", err)
	}
	if _, err := inventory.IssueCost(rules, issue("m-1", one.ID, 1), money.Money{}); !errors.Is(err, inventory.ErrInvalidMove) {
		t.Errorf("IssueCost with no currency = %v, want ErrInvalidMove", err)
	}
}

// Two issues booked at once each get their own entry, and the stock is not
// oversold.
func TestRecordIssue_ConcurrentIssuesEachGetAnEntry(t *testing.T) {
	book, one := stocked(t)
	rules := salesRules(t)

	if _, err := book.Record(receipt("m-0", one.ID, 5, 100)); err != nil {
		t.Fatalf("Record: %v", err)
	}

	const attempts = 8

	entries := make([]*inventory.Entry, attempts)
	errs := make([]error, attempts)

	start := make(chan struct{})
	var wg sync.WaitGroup

	for i := 0; i < attempts; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			entries[i], errs[i] = book.RecordIssue(rules, issue(moveID(i), one.ID, 1))
		}(i)
	}

	close(start)
	wg.Wait()

	booked := 0
	for i, err := range errs {
		switch {
		case err == nil:
			booked++
			if entries[i] == nil || entries[i].Amount.Amount() != 100 {
				t.Fatalf("issue %d produced %+v, want an entry of 100", i, entries[i])
			}
		case errors.Is(err, inventory.ErrNegativeStock):
			if entries[i] != nil {
				t.Errorf("issue %d was refused and still produced an entry", i)
			}
		default:
			t.Fatalf("issue %d: %v", i, err)
		}
	}

	if booked != 5 {
		t.Errorf("%d issues booked out of 8 against 5 units, want exactly 5", booked)
	}

	level, err := book.Level(one.ID)
	if err != nil {
		t.Fatalf("Level: %v", err)
	}
	if level != 0 {
		t.Errorf("level = %d, want 0", level)
	}
}

func moveID(i int) string {
	return fmt.Sprintf("m-%d", i)
}

// recordingPoster stands in for the ledger. It takes one sale and writes both
// its vouchers, or neither — which is what one transaction means.
type recordingPoster struct {
	mu    sync.Mutex
	calls []inventory.Sale
	fail  error
	wrote []string
}

func (p *recordingPoster) Post(_ context.Context, sale inventory.Sale) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.calls = append(p.calls, sale)

	if p.fail != nil {
		// Nothing is committed, and nothing is recorded as committed.
		return p.fail
	}

	p.wrote = append(p.wrote, sale.Revenue.ID())
	if sale.Cost != nil {
		p.wrote = append(p.wrote, sale.Cost.Voucher.ID())
	}

	return nil
}

func (p *recordingPoster) callCount() int {
	p.mu.Lock()
	defer p.mu.Unlock()

	return len(p.calls)
}

func (p *recordingPoster) saleAt(i int) inventory.Sale {
	p.mu.Lock()
	defer p.mu.Unlock()

	return p.calls[i]
}

func (p *recordingPoster) written() []string {
	p.mu.Lock()
	defer p.mu.Unlock()

	return append([]string(nil), p.wrote...)
}

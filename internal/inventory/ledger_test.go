package inventory_test

import (
	"errors"
	"fmt"
	"sync"
	"testing"

	"fingo/internal/inventory"
	"fingo/internal/money"
)

// stocked returns a ledger with one item already in it.
func stocked(t *testing.T) (*inventory.Ledger, inventory.Item) {
	t.Helper()

	book := ledger(t)
	one := item("i-1")
	if err := book.AddItem(one); err != nil {
		t.Fatalf("AddItem: %v", err)
	}

	return book, one
}

// Receipts and issues move the level, and the level is what the movements say it
// is.
func TestLedger_LevelIsTheSumOfTheMovements(t *testing.T) {
	book, one := stocked(t)

	for _, move := range []inventory.StockMove{
		receipt("m-1", one.ID, 10, 100),
		receipt("m-2", one.ID, 5, 200),
		issue("m-3", one.ID, 4),
		issue("m-4", one.ID, 1),
	} {
		if _, err := book.Record(move); err != nil {
			t.Fatalf("Record %s: %v", move.ID, err)
		}
	}

	level, err := book.Level(one.ID)
	if err != nil {
		t.Fatalf("Level: %v", err)
	}
	if level != 10 {
		t.Errorf("level = %d, want 10 + 5 − 4 − 1", level)
	}

	// And the valuation says the same thing: the level is summed, not read from
	// the valuation, so the two only agree if both are right.
	valuation, err := book.Valuation(one.ID)
	if err != nil {
		t.Fatalf("Valuation: %v", err)
	}
	if valuation.Quantity != level {
		t.Errorf("the valuation says %d units and the movements say %d", valuation.Quantity, level)
	}

	// 10 at 100 and 5 at 200 is 2000 over 15 units, and 5 have left at that
	// average: 2000 − (5 × 133) = 1335 over the 10 that remain.
	if valuation.AvgCost != 133 {
		t.Errorf("average = %d, want 2000 / 15 truncated to 133", valuation.AvgCost)
	}
	if got := valuation.Value().Amount(); got != 1330 {
		t.Errorf("value on hand = %d, want 10 × 133", got)
	}
}

// An issue leaves at the average standing, and the movement log says so.
func TestLedger_RecordsAnIssueAtTheAverageItWasCostedAt(t *testing.T) {
	book, one := stocked(t)

	if _, err := book.Record(receipt("m-1", one.ID, 10, 100)); err != nil {
		t.Fatalf("Record: %v", err)
	}
	if _, err := book.Record(receipt("m-2", one.ID, 10, 200)); err != nil {
		t.Fatalf("Record: %v", err)
	}

	if _, err := book.Record(issue("m-3", one.ID, 5)); err != nil {
		t.Fatalf("Record: %v", err)
	}

	moves, err := book.Moves(one.ID)
	if err != nil {
		t.Fatalf("Moves: %v", err)
	}
	if len(moves) != 3 {
		t.Fatalf("movements = %d, want 3", len(moves))
	}
	if got := moves[2].UnitCost.Amount(); got != 150 {
		t.Errorf("the issue was recorded at %d a unit, want the average 150", got)
	}
	if moves[2].UnitCost.Currency() != vnd {
		t.Errorf("the issue was recorded in %s, want VND", moves[2].UnitCost.Currency())
	}
}

// Stock cannot go below nothing, and a refused movement is not recorded.
func TestLedger_RefusesAnIssueBeyondTheStockOnHand(t *testing.T) {
	book, one := stocked(t)

	if _, err := book.Record(receipt("m-1", one.ID, 3, 100)); err != nil {
		t.Fatalf("Record: %v", err)
	}

	_, err := book.Record(issue("m-2", one.ID, 4))
	if !errors.Is(err, inventory.ErrNegativeStock) {
		t.Fatalf("Record of an issue of 4 from 3 = %v, want ErrNegativeStock", err)
	}

	level, err := book.Level(one.ID)
	if err != nil {
		t.Fatalf("Level: %v", err)
	}
	if level != 3 {
		t.Errorf("level = %d after a refused issue, want 3", level)
	}

	moves, err := book.Moves(one.ID)
	if err != nil {
		t.Fatalf("Moves: %v", err)
	}
	if len(moves) != 1 {
		t.Errorf("movements = %d, want only the receipt that was accepted", len(moves))
	}
}

// Two issues at once cannot both find the same stock on hand and both take it.
func TestLedger_ConcurrentIssuesDoNotOversell(t *testing.T) {
	book, one := stocked(t)

	if _, err := book.Record(receipt("m-0", one.ID, 5, 100)); err != nil {
		t.Fatalf("Record: %v", err)
	}

	const attempts = 10

	errs := make([]error, attempts)

	start := make(chan struct{})
	var wg sync.WaitGroup

	for i := 0; i < attempts; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			_, errs[i] = book.Record(issue(fmt.Sprintf("m-%d", i+1), one.ID, 1))
		}(i)
	}

	close(start)
	wg.Wait()

	issued := 0
	for i, err := range errs {
		switch {
		case err == nil:
			issued++
		case errors.Is(err, inventory.ErrNegativeStock):
			// The stock had already gone.
		default:
			t.Fatalf("issue %d: %v", i, err)
		}
	}

	if issued != 5 {
		t.Errorf("%d issues succeeded out of 10 against 5 units, want exactly 5", issued)
	}

	level, err := book.Level(one.ID)
	if err != nil {
		t.Fatalf("Level: %v", err)
	}
	if level != 0 {
		t.Errorf("level = %d, want 0: the stock was oversold", level)
	}
}

// A movement in another currency is a rate nobody applied.
func TestLedger_RefusesAMovementInAnotherCurrency(t *testing.T) {
	book, one := stocked(t)

	move := receipt("m-1", one.ID, 1, 100)
	move.UnitCost = money.New(100, money.Currency("USD"))

	if _, err := book.Record(move); !errors.Is(err, inventory.ErrCurrencyMismatch) {
		t.Fatalf("Record in USD = %v, want ErrCurrencyMismatch", err)
	}
}

// A receipt has to say what it cost. Recording it at nothing and leaving the
// average to absorb the difference is how inventory quietly becomes free.
func TestLedger_RefusesAReceiptThatDoesNotSayWhatItCost(t *testing.T) {
	book, one := stocked(t)

	move := receipt("m-1", one.ID, 1, 100)
	move.UnitCost = money.Money{}

	if _, err := book.Record(move); !errors.Is(err, inventory.ErrInvalidMove) {
		t.Fatalf("Record of a receipt with no cost = %v, want ErrInvalidMove", err)
	}
}

// Movements are refused for the reasons the model gives, and nothing is recorded
// when they are.
func TestLedger_RefusesAMovementThatCouldNotBeRecorded(t *testing.T) {
	book, one := stocked(t)

	cases := map[string]inventory.StockMove{
		"no id":                 {ItemID: one.ID, Direction: inventory.DirectionIn, Qty: 1, UnitCost: cents(100), Date: day(2026, 3, 1), SourceDocID: "doc-1"},
		"no item":               {ID: "m-1", Direction: inventory.DirectionIn, Qty: 1, UnitCost: cents(100), Date: day(2026, 3, 1), SourceDocID: "doc-1"},
		"unknown direction":     {ID: "m-1", ItemID: one.ID, Direction: "sideways", Qty: 1, UnitCost: cents(100), Date: day(2026, 3, 1), SourceDocID: "doc-1"},
		"nothing moving":        {ID: "m-1", ItemID: one.ID, Direction: inventory.DirectionIn, Qty: 0, UnitCost: cents(100), Date: day(2026, 3, 1), SourceDocID: "doc-1"},
		"a negative amount":     {ID: "m-1", ItemID: one.ID, Direction: inventory.DirectionIn, Qty: 1, UnitCost: cents(-100), Date: day(2026, 3, 1), SourceDocID: "doc-1"},
		"no date":               {ID: "m-1", ItemID: one.ID, Direction: inventory.DirectionIn, Qty: 1, UnitCost: cents(100), SourceDocID: "doc-1"},
		"no document behind it": {ID: "m-1", ItemID: one.ID, Direction: inventory.DirectionIn, Qty: 1, UnitCost: cents(100), Date: day(2026, 3, 1)},
	}

	for name, move := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := book.Record(move); !errors.Is(err, inventory.ErrInvalidMove) {
				t.Fatalf("Record = %v, want ErrInvalidMove", err)
			}
		})
	}

	moves, err := book.Moves(one.ID)
	if err != nil {
		t.Fatalf("Moves: %v", err)
	}
	if len(moves) != 0 {
		t.Errorf("movements = %d after every one was refused, want 0", len(moves))
	}
}

func TestLedger_KnowsWhichItemsItHolds(t *testing.T) {
	book, one := stocked(t)

	if _, err := book.Record(receipt("m-1", "i-nothing", 1, 100)); !errors.Is(err, inventory.ErrUnknownItem) {
		t.Errorf("Record against an unknown item = %v, want ErrUnknownItem", err)
	}
	if err := book.AddItem(item("i-1")); !errors.Is(err, inventory.ErrItemExists) {
		t.Errorf("AddItem of a taken id = %v, want ErrItemExists", err)
	}
	if _, err := book.Item("i-nothing"); !errors.Is(err, inventory.ErrUnknownItem) {
		t.Errorf("Item = %v, want ErrUnknownItem", err)
	}
	if _, err := book.Level("i-nothing"); !errors.Is(err, inventory.ErrUnknownItem) {
		t.Errorf("Level = %v, want ErrUnknownItem", err)
	}
	if _, err := book.Valuation("i-nothing"); !errors.Is(err, inventory.ErrUnknownItem) {
		t.Errorf("Valuation = %v, want ErrUnknownItem", err)
	}
	if _, err := book.Moves("i-nothing"); !errors.Is(err, inventory.ErrUnknownItem) {
		t.Errorf("Moves = %v, want ErrUnknownItem", err)
	}

	held, err := book.Item(one.ID)
	if err != nil {
		t.Fatalf("Item: %v", err)
	}
	if held.SKU != "SKU-i-1" {
		t.Errorf("item = %+v", held)
	}
}

func TestLedger_RefusesAnItemThatCouldNotBeStocked(t *testing.T) {
	book := ledger(t)

	cases := map[string]inventory.Item{
		"no id":          {SKU: "S", Name: "N", Unit: "cái"},
		"no SKU":         {ID: "i-1", Name: "N", Unit: "cái"},
		"no name":        {ID: "i-1", SKU: "S", Unit: "cái"},
		"no unit":        {ID: "i-1", SKU: "S", Name: "N"},
		"negative point": {ID: "i-1", SKU: "S", Name: "N", Unit: "cái", ReorderPoint: -1},
	}

	for name, one := range cases {
		t.Run(name, func(t *testing.T) {
			if err := book.AddItem(one); !errors.Is(err, inventory.ErrInvalidItem) {
				t.Fatalf("AddItem = %v, want ErrInvalidItem", err)
			}
		})
	}

	if got := len(book.Items()); got != 0 {
		t.Errorf("items = %d after every one was refused, want 0", got)
	}
}

// The reorder point is what the item is watched at. Nothing is reported for an
// item nobody set a level for — a level of zero and no level are different
// facts.
func TestLedger_ReportsWhatIsAtOrBelowItsReorderPoint(t *testing.T) {
	book := ledger(t)

	low := item("i-low")
	low.SKU = "A"
	low.ReorderPoint = 5

	fine := item("i-fine")
	fine.SKU = "B"
	fine.ReorderPoint = 2

	unset := item("i-unset")
	unset.SKU = "C"
	unset.ReorderPoint = 0

	for _, one := range []inventory.Item{low, fine, unset} {
		if err := book.AddItem(one); err != nil {
			t.Fatalf("AddItem %s: %v", one.ID, err)
		}
	}

	if _, err := book.Record(receipt("m-1", low.ID, 5, 100)); err != nil {
		t.Fatalf("Record: %v", err)
	}
	if _, err := book.Record(receipt("m-2", fine.ID, 10, 100)); err != nil {
		t.Fatalf("Record: %v", err)
	}

	reported := book.BelowReorderPoint()
	if len(reported) != 1 {
		t.Fatalf("reported %d items, want only the one at its reorder point: %+v", len(reported), reported)
	}
	if reported[0].ID != low.ID {
		t.Errorf("reported %s, want %s", reported[0].ID, low.ID)
	}

	// Taking it further down still reports it.
	if _, err := book.Record(issue("m-3", low.ID, 1)); err != nil {
		t.Fatalf("Record: %v", err)
	}
	if len(book.BelowReorderPoint()) != 1 {
		t.Error("an item below its reorder point stopped being reported")
	}
}

// Items come back by SKU, and the movements come back in the order they were
// recorded — the only order a moving average can be replayed in.
func TestLedger_KeepsItsOrders(t *testing.T) {
	book := ledger(t)

	for _, one := range []inventory.Item{
		{ID: "i-b", SKU: "B", Name: "B", Unit: "cái"},
		{ID: "i-a", SKU: "A", Name: "A", Unit: "cái"},
		{ID: "i-c", SKU: "C", Name: "C", Unit: "cái"},
	} {
		if err := book.AddItem(one); err != nil {
			t.Fatalf("AddItem: %v", err)
		}
	}

	items := book.Items()
	if len(items) != 3 || items[0].SKU != "A" || items[1].SKU != "B" || items[2].SKU != "C" {
		t.Errorf("items = %+v, want them by SKU", items)
	}

	// Two movements on the same day still replay in the order they arrived.
	sameDay := receipt("m-1", "i-a", 1, 100)
	if _, err := book.Record(sameDay); err != nil {
		t.Fatalf("Record: %v", err)
	}
	sameDay = receipt("m-2", "i-a", 1, 300)
	if _, err := book.Record(sameDay); err != nil {
		t.Fatalf("Record: %v", err)
	}

	moves, err := book.Moves("i-a")
	if err != nil {
		t.Fatalf("Moves: %v", err)
	}
	if len(moves) != 2 || moves[0].ID != "m-1" || moves[1].ID != "m-2" {
		t.Errorf("movements = %+v, want them in the order they were recorded", moves)
	}

	valuation, err := book.Valuation("i-a")
	if err != nil {
		t.Fatalf("Valuation: %v", err)
	}
	if valuation.AvgCost != 200 {
		t.Errorf("average = %d, want (100 + 300) / 2", valuation.AvgCost)
	}
}

// The ledger hands out copies: a caller that edited the movements it was given
// would otherwise be editing the history the valuation is derived from.
func TestLedger_HandsOutCopies(t *testing.T) {
	book, one := stocked(t)

	if _, err := book.Record(receipt("m-1", one.ID, 2, 100)); err != nil {
		t.Fatalf("Record: %v", err)
	}

	moves, err := book.Moves(one.ID)
	if err != nil {
		t.Fatalf("Moves: %v", err)
	}
	moves[0].Qty = 99

	again, err := book.Moves(one.ID)
	if err != nil {
		t.Fatalf("Moves: %v", err)
	}
	if again[0].Qty != 2 {
		t.Errorf("the stored movement was edited through a copy: %+v", again[0])
	}

	level, err := book.Level(one.ID)
	if err != nil {
		t.Fatalf("Level: %v", err)
	}
	if level != 2 {
		t.Errorf("level = %d, want 2", level)
	}
}

func TestNewLedger_RefusesACurrencyThatIsNotOne(t *testing.T) {
	for _, code := range []money.Currency{"", "VNDD", "vnd", "12"} {
		if _, err := inventory.NewLedger(code); err == nil {
			t.Errorf("NewLedger(%q) must be refused", code)
		}
	}

	book, err := inventory.NewLedger(vnd)
	if err != nil {
		t.Fatalf("NewLedger: %v", err)
	}
	if book.Currency() != vnd {
		t.Errorf("currency = %s, want VND", book.Currency())
	}
}

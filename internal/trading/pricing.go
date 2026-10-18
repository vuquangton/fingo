package trading

import (
	"fmt"
	"math"

	"fingo/internal/money"
	"fingo/internal/vat"
)

// sourceLine is a line of a document before it is priced.
//
// Both cycles price their lines the same way — quantity × price, taxed at the
// rate the line names — so both convert to this and neither keeps its own copy
// of the arithmetic. A second implementation of "what does this line come to"
// is a second answer, and the two would eventually disagree on an invoice that
// has to add up.
type sourceLine struct {
	ItemID    string
	Quantity  int
	UnitPrice money.Money
	VATRateID string
}

// lineAmounts is what one line came to.
type lineAmounts struct {
	Base  money.Money // quantity × unit price
	VAT   money.Money // the tax that base carries
	Total money.Money // base + VAT
}

// totals is what a document came to.
type totals struct {
	Base  money.Money
	VAT   money.Money
	Total money.Money
}

// priceLines prices each line at the rate it names and adds the document up.
//
// Each line is priced on its own and the totals are the sum of the priced lines,
// so a document's total is always the sum of its own detail rather than a figure
// computed beside it — the two can never disagree.
//
// The errors say what went wrong; a caller wraps them in the sentinel of the
// document it was building, so a purchase failure reads as a purchase failure.
func priceLines(rates *vat.Registry, lines []sourceLine) ([]lineAmounts, totals, error) {
	amounts := make([]lineAmounts, 0, len(lines))

	var sum totals

	for i, line := range lines {
		// quantity × price, checked. A wrapped product would put an amount on a
		// document that nobody wrote and nothing would flag: the document would
		// balance, the VAT would be computed from it, and the counterparty would
		// be billed it.
		price := line.UnitPrice.Amount()
		if price > 0 && int64(line.Quantity) > math.MaxInt64/price {
			return nil, totals{}, fmt.Errorf("line %d (%s) is %d × %s, which is more than can be held",
				i+1, line.ItemID, line.Quantity, line.UnitPrice)
		}

		net := money.New(price*int64(line.Quantity), line.UnitPrice.Currency())

		// The rate is named by id and read from the registry, so a line cannot
		// carry a percentage that is not in force. See internal/vat.
		charged, err := rates.Charge(vat.Line{NetAmount: net.Amount(), VATRateID: line.VATRateID})
		if err != nil {
			return nil, totals{}, fmt.Errorf("line %d (%s): %w", i+1, line.ItemID, err)
		}

		lineVAT := money.New(charged.Tax, net.Currency())
		lineTotal := net.Add(lineVAT)

		amounts = append(amounts, lineAmounts{Base: net, VAT: lineVAT, Total: lineTotal})

		if sum.Base, err = add(sum.Base, net); err != nil {
			return nil, totals{}, err
		}
		if sum.VAT, err = add(sum.VAT, lineVAT); err != nil {
			return nil, totals{}, err
		}
		if sum.Total, err = add(sum.Total, lineTotal); err != nil {
			return nil, totals{}, err
		}
	}

	// Checked before anything is posted. A document worth nothing is refused by
	// the document book, and finding that out after the voucher was posted would
	// leave a ledger entry for a document no sub-ledger admits to.
	if sum.Total.Amount() <= 0 {
		return nil, totals{}, fmt.Errorf("the document comes to %s, and a document is worth more than nothing", sum.Total)
	}

	return amounts, sum, nil
}

// add sums two amounts, reporting a mismatched currency rather than panicking
// the way money.Money.Add does.
//
// The panic is right for amounts the program built itself; it is not right for
// amounts a caller supplied, and every amount here came from a caller.
func add(a, b money.Money) (money.Money, error) {
	if a.Currency() != "" && b.Currency() != "" && a.Currency() != b.Currency() {
		return money.Money{}, fmt.Errorf("%w: %s and %s", ErrCurrencyMismatch, a.Currency(), b.Currency())
	}

	return a.Add(b), nil
}

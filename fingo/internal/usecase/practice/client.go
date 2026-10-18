// Package practice owns one client's books: their database, their chart of
// accounts, and the services that operate on them.
//
// FinGo serves an accounting practice — one operator keeping the books of many
// clients, with one database per client (docs/requirements-v1.md §1.1). Nothing
// in this package reaches outside a single client's database, which is what
// makes that isolation structural rather than a convention.
package practice

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"fingo/db"
	"fingo/internal/adapter/mariadb/conn"
	"fingo/internal/adapter/mariadb/documentrepo"
	"fingo/internal/adapter/mariadb/glrepo"
	"fingo/internal/adapter/mariadb/periodrepo"
	"fingo/internal/adapter/mariadb/periodstore"
	"fingo/internal/adapter/mariadb/postingrepo"
	mariadb "fingo/internal/adapter/mariadb/sqlc"
	"fingo/internal/adapter/mariadb/stockrepo"
	"fingo/internal/domain/document"
	"fingo/internal/domain/gl"
	"fingo/internal/domain/inventory"
	"fingo/internal/domain/partner"
	"fingo/internal/domain/tax"
	"fingo/internal/period"
	"fingo/internal/usecase/ledger"
	"fingo/internal/usecase/sales"
)

// Client is one client's books, opened.
type Client struct {
	db        *sql.DB
	queries   *mariadb.Queries
	vouchers  *glrepo.VoucherStore
	postings  *postingrepo.Store
	periods   *periodrepo.PeriodStore
	locks     *periodstore.Store
	documents *documentrepo.Store
	stock     *stockrepo.Store
	vatRates  *tax.Schedule
}

// Open connects to one client's database and brings its schema up to date.
//
// Migrating on open is deliberate. A practice takes on clients over time, and
// every client database must end up on the same schema. goose tracks the
// applied version per database, so opening an up-to-date client is a no-op —
// and a client whose migration fails surfaces the failure here rather than
// half-way through recording a sale.
func Open(ctx context.Context, cfg conn.Config, database string) (*Client, error) {
	if database == "" {
		return nil, errors.New("a client must name its database")
	}

	sqlDB, err := sql.Open("mysql", cfg.DSN(database))
	if err != nil {
		return nil, fmt.Errorf("open client database %s: %w", database, err)
	}

	if err := sqlDB.PingContext(ctx); err != nil {
		sqlDB.Close()
		return nil, fmt.Errorf("reach client database %s: %w", database, err)
	}

	if err := db.Up(sqlDB); err != nil {
		sqlDB.Close()
		return nil, fmt.Errorf("migrate client database %s: %w", database, err)
	}

	vouchers := glrepo.NewVoucherStore(sqlDB)

	return &Client{
		db:        sqlDB,
		queries:   mariadb.New(sqlDB),
		vouchers:  vouchers,
		postings:  postingrepo.NewStore(sqlDB),
		periods:   periodrepo.NewPeriodStore(sqlDB),
		locks:     periodstore.NewStore(sqlDB),
		documents: documentrepo.NewStore(sqlDB),
		stock:     stockrepo.NewStore(sqlDB),
		vatRates:  tax.DefaultSchedule(),
	}, nil
}

func (c *Client) Close() error {
	return c.db.Close()
}

// DB returns the connection pool this client's books are on.
//
// Exposed so an adapter can read the same books without opening a second pool to
// the same database. Two pools to one database is two sets of connections, two
// idle timeouts and two chances to exhaust the server's connection limit, for no
// benefit at all. The pool belongs to the client and is closed with it, so a
// caller must not close it.
func (c *Client) DB() *sql.DB { return c.db }

// RecordSale records a credit sale and reports what was recorded.
func (c *Client) RecordSale(ctx context.Context, cmd sales.Command) (sales.Result, error) {
	return sales.NewRecordSale(c.Ledger(), c, c.vatRates).Execute(ctx, cmd)
}

// CostOfIssue reports what taking goods out of a warehouse would cost, at the
// current moving average.
//
// It changes nothing. The posting is what writes the movement, and it re-checks
// this figure inside its own transaction — so a cost resolved here and overtaken
// by another posting is refused rather than written.
func (c *Client) CostOfIssue(
	ctx context.Context,
	itemCode, warehouseCode string,
	quantity decimal.Decimal,
) (decimal.Decimal, error) {
	position, err := c.stock.Position(ctx, itemCode, warehouseCode)
	if err != nil {
		return decimal.Zero, err
	}

	_, cost, err := position.Issue(quantity)
	if err != nil {
		return decimal.Zero, err
	}

	return cost, nil
}

// VATRates returns the schedule a sale's treatment is resolved against.
func (c *Client) VATRates() *tax.Schedule {
	return c.vatRates
}

// Ledger is the single path by which vouchers reach this client's books.
//
// It carries no state of its own, so it is built on demand rather than held; the
// rules it applies are read fresh on every posting, and the transaction that
// writes a posting spans the voucher, its lines, the stock it moves and the
// document it records.
func (c *Client) Ledger() *ledger.Service {
	return ledger.NewService(c.postings, c, c, c, period.NewGuard(c.locks))
}

// NextNumber allocates the next place in a book's run for a fiscal year. It
// implements ledger.NumberSource.
func (c *Client) NextNumber(ctx context.Context, book gl.Book, fiscalYear int) (gl.Number, error) {
	return c.vouchers.NextNumber(ctx, book, fiscalYear)
}

// NumberGaps reports places in a book's run that were reserved but never issued.
//
// A gap is not automatically wrong — a number is taken before a document is
// stored, and a crash between the two leaves one behind. But it must be visible,
// because an unexplained hole in a statutory book is exactly what an inspector
// asks about, and a numbering system that hides its own holes is worse than none.
func (c *Client) NumberGaps(ctx context.Context) ([]gl.Number, error) {
	sequences, err := c.queries.ListVoucherSequences(ctx)
	if err != nil {
		return nil, fmt.Errorf("list number sequences: %w", err)
	}

	issuedRows, err := c.queries.ListIssuedVoucherNumbers(ctx)
	if err != nil {
		return nil, fmt.Errorf("list issued numbers: %w", err)
	}

	type place struct {
		book gl.Book
		year int
		seq  int
	}

	issued := make(map[place]struct{}, len(issuedRows))
	for _, r := range issuedRows {
		issued[place{gl.Book(r.Book.String), int(r.FiscalYear.Int16), int(r.Seq.Int32)}] = struct{}{}
	}

	var gaps []gl.Number

	for _, s := range sequences {
		book := gl.Book(s.Book)
		year := int(s.FiscalYear)

		// next_seq is the next place to hand out, so 1..next_seq-1 have been
		// reserved and each should have reached a document.
		for seq := 1; seq < int(s.NextSeq); seq++ {
			if _, found := issued[place{book, year, seq}]; !found {
				gaps = append(gaps, gl.Number{Book: book, Year: year, Seq: seq})
			}
		}
	}

	return gaps, nil
}

// Receivable is what one customer owes, from posted entries only.
type Receivable struct {
	Partner partner.Partner
	Amount  decimal.Decimal // positive means the customer owes the client
}

// Receivables totals TK 131 by customer — the detail behind Sổ chi tiết TK 131.
//
// The lines are summed here, in exact decimal arithmetic, rather than in SQL, so
// the sub-ledger cannot total differently from the ledger it came from.
func (c *Client) Receivables(ctx context.Context) ([]Receivable, error) {
	lines, err := c.queries.ListPartnerLedgerLines(ctx)
	if err != nil {
		return nil, fmt.Errorf("list receivable lines: %w", err)
	}

	order := make([]string, 0, len(lines))
	totals := make(map[string]*Receivable, len(lines))

	for _, l := range lines {
		amount, err := decimal.NewFromString(l.Amount)
		if err != nil {
			return nil, fmt.Errorf("receivable line for %s: %w", l.PartnerCode, err)
		}

		running, seen := totals[l.PartnerID]
		if !seen {
			running = &Receivable{Partner: partner.Partner{
				ID:   l.PartnerID,
				Code: l.PartnerCode,
				Name: l.PartnerName,
			}}
			totals[l.PartnerID] = running
			order = append(order, l.PartnerID)
		}

		// A debit grows what the customer owes; a credit — a payment, a return,
		// a credit note — reduces it. A customer in credit therefore comes out
		// negative rather than being clamped to zero, because that is a real
		// position the client has to deal with.
		if gl.EntrySide(l.Side) == gl.SideDebit {
			running.Amount = running.Amount.Add(amount)
		} else {
			running.Amount = running.Amount.Sub(amount)
		}
	}

	out := make([]Receivable, 0, len(order))
	for _, id := range order {
		out = append(out, *totals[id])
	}

	return out, nil
}

// IssueDocument records a business document against a voucher that has already
// posted.
//
// Recording it is a separate act from posting the voucher, which is deliberate:
// the two would need one transaction to be atomic, and the ledger's save path
// owns its own. Wiring them together belongs with the trading-cycle documents
// that know an invoice is being issued, not with a generic posting call.
func (c *Client) IssueDocument(ctx context.Context, d document.Document) error {
	return c.documents.Record(ctx, d)
}

// Outstanding returns a partner's documents with what is still owed on each,
// oldest first.
func (c *Client) Outstanding(ctx context.Context, partnerID string) ([]document.Open, error) {
	return c.documents.OpenDocuments(ctx, partnerID)
}

// Settle applies a payment to a partner's open documents, oldest first.
//
// The amount is what the payment puts towards this partner's receivables; a
// payment that exceeds them leaves a remainder, which comes back in the plan
// rather than being absorbed into an invoice that does not owe it.
func (c *Client) Settle(
	ctx context.Context,
	partnerID, paymentVoucherID string,
	amount decimal.Decimal,
	at time.Time,
) (document.Plan, error) {
	open, err := c.documents.OpenDocuments(ctx, partnerID)
	if err != nil {
		return document.Plan{}, err
	}

	plan, err := document.PlanAllocation(amount, open)
	if err != nil {
		return document.Plan{}, err
	}

	if err := c.documents.Allocate(ctx, paymentVoucherID, plan, at); err != nil {
		return document.Plan{}, err
	}

	return plan, nil
}

// StockPosition returns what is on hand for one item in one warehouse.
func (c *Client) StockPosition(ctx context.Context, itemCode, warehouseCode string) (inventory.Stock, error) {
	return c.stock.Position(ctx, itemCode, warehouseCode)
}

// StockPositions returns every position that has moved.
func (c *Client) StockPositions(ctx context.Context) ([]inventory.Stock, error) {
	return c.stock.Positions(ctx)
}

// ReceiveStock records stock coming in, at what it cost in total.
//
// A total rather than a unit cost because giá gốc includes transport, handling
// and insurance: forcing those through a per-unit figure on the way in would
// round a fraction of a đồng per unit straight into the inventory value.
func (c *Client) ReceiveStock(
	ctx context.Context,
	itemCode, warehouseCode string,
	quantity, totalCost decimal.Decimal,
	voucherID string,
	at time.Time,
) (inventory.Stock, error) {
	current, err := c.stock.Position(ctx, itemCode, warehouseCode)
	if err != nil {
		return inventory.Stock{}, err
	}

	next, err := current.Receive(quantity, totalCost)
	if err != nil {
		return inventory.Stock{}, err
	}

	if err := c.stock.Record(ctx, inventory.Movement{
		ItemCode:      itemCode,
		WarehouseCode: warehouseCode,
		Kind:          inventory.KindReceipt,
		Quantity:      quantity,
		Cost:          totalCost,
		VoucherID:     voucherID,
		MovedAt:       at,
	}); err != nil {
		return inventory.Stock{}, err
	}

	return next, nil
}

// IssueStock records stock going out and reports what it cost.
//
// This is the real cost of goods sold: the moving average of what the item
// actually cost, rather than whatever an operator remembered. The quantity is
// checked against the current position before the movement is written, so more
// cannot be issued than is on hand.
func (c *Client) IssueStock(
	ctx context.Context,
	itemCode, warehouseCode string,
	quantity decimal.Decimal,
	voucherID string,
	at time.Time,
) (decimal.Decimal, inventory.Stock, error) {
	current, err := c.stock.Position(ctx, itemCode, warehouseCode)
	if err != nil {
		return decimal.Zero, inventory.Stock{}, err
	}

	next, cost, err := current.Issue(quantity)
	if err != nil {
		return decimal.Zero, inventory.Stock{}, err
	}

	if err := c.stock.Record(ctx, inventory.Movement{
		ItemCode:      itemCode,
		WarehouseCode: warehouseCode,
		Kind:          inventory.KindIssue,
		Quantity:      quantity,
		Cost:          cost,
		VoucherID:     voucherID,
		MovedAt:       at,
	}); err != nil {
		return decimal.Zero, inventory.Stock{}, err
	}

	return cost, next, nil
}

// InventoryPolicy returns the valuation method chosen for each fiscal year.
func (c *Client) InventoryPolicy(ctx context.Context) (*inventory.Policy, error) {
	return c.stock.LoadPolicy(ctx)
}

// ChooseValuationMethod sets the method a fiscal year is valued by.
//
// Refused when the year already carries a different method: TT133 requires one
// method for a whole fiscal year, and a mid-year change would restate a cost of
// sales that has already been reported.
func (c *Client) ChooseValuationMethod(
	ctx context.Context,
	year int,
	method inventory.Method,
	at time.Time,
) error {
	policy, err := c.stock.LoadPolicy(ctx)
	if err != nil {
		return err
	}

	if err := policy.Set(year, method); err != nil {
		return err
	}

	return c.stock.SavePolicy(ctx, year, method, at)
}

// CreatePartner onboards a customer or supplier.
//
// The partner is built through the domain, so the code and tax code are
// normalised here and no other path into the books can skip that: an import, a
// script or a future screen all get the same treatment, and one business does
// not become two records.
func (c *Client) CreatePartner(
	ctx context.Context,
	id, code, name, taxCode string,
	kind partner.Kind,
) (*partner.Partner, error) {
	p, err := partner.NewPartner(id, code, name, taxCode, kind)
	if err != nil {
		return nil, err
	}

	if err := c.queries.CreatePartner(ctx, mariadb.CreatePartnerParams{
		ID:       p.ID,
		Code:     p.Code,
		Name:     p.Name,
		TaxCode:  sql.NullString{String: p.TaxCode, Valid: p.TaxCode != ""},
		Address:  sql.NullString{String: p.Address, Valid: p.Address != ""},
		Kind:     mariadb.PartnersKind(p.Kind),
		IsActive: p.IsActive,
	}); err != nil {
		return nil, fmt.Errorf("create partner %s: %w", p.Code, err)
	}

	return p, nil
}

// Partners returns the client's customers and suppliers.
func (c *Client) Partners(ctx context.Context) ([]partner.Partner, error) {
	rows, err := c.queries.ListPartners(ctx)
	if err != nil {
		return nil, fmt.Errorf("list partners: %w", err)
	}

	out := make([]partner.Partner, 0, len(rows))
	for _, r := range rows {
		out = append(out, partner.Partner{
			ID:       r.ID,
			Code:     r.Code,
			Name:     r.Name,
			TaxCode:  r.TaxCode.String,
			Address:  r.Address.String,
			Kind:     partner.Kind(r.Kind),
			IsActive: r.IsActive,
		})
	}

	return out, nil
}

// Chart loads the client's chart of accounts. It implements ledger.ChartSource.
func (c *Client) Chart(ctx context.Context) (*gl.ChartOfAccounts, error) {
	accounts, err := c.Accounts(ctx)
	if err != nil {
		return nil, err
	}

	chart := gl.NewChartOfAccounts()
	for _, a := range accounts {
		// Load, not Add: an account the client deactivated must stay
		// deactivated, and Load preserves that.
		if err := chart.Load(a); err != nil {
			return nil, fmt.Errorf("build the client's chart: %w", err)
		}
	}

	return chart, nil
}

// ClosedPeriods loads the client's closed months. It implements
// ledger.PeriodSource.
//
// Named for what it is: this is the older, month-based model, and the client also
// has range-based periods below. Two models of one idea is one too many, and the
// names should not pretend otherwise.
func (c *Client) ClosedPeriods(ctx context.Context) (*gl.Periods, error) {
	return c.periods.Load(ctx)
}

// Periods returns the client's accounting periods, oldest first.
func (c *Client) Periods(ctx context.Context) ([]period.Period, error) {
	return c.locks.List(ctx)
}

// CreatePeriod adds a period to the client's calendar.
//
// Refused if it would overlap an existing one or leave a gap: every date must
// belong to exactly one period, or a transaction's date could fall in two and
// nothing could say which lock applies.
func (c *Client) CreatePeriod(ctx context.Context, p period.Period) error {
	return c.locks.Create(ctx, p)
}

// LockPeriod shuts a period. It records when, and there is no counterpart.
//
// Locking is irreversible in v1, which is why it is called out here rather than
// left to the reader to discover: a period that could be reopened is not a lock,
// and a reported figure could then change underneath the report that relied on
// it.
func (c *Client) LockPeriod(ctx context.Context, id string, at time.Time) error {
	return c.locks.Lock(ctx, id, at)
}

// ClosePeriod shuts an accounting period, recording when and by whom.
//
// Refused when the period is already closed: the caller should have read the
// calendar first, and a second close means they did not.
func (c *Client) ClosePeriod(ctx context.Context, period gl.YearMonth, at time.Time, by string) error {
	current, err := c.periods.Load(ctx)
	if err != nil {
		return err
	}
	if current.IsClosed(period) {
		return fmt.Errorf("period %s is already closed", period)
	}

	return c.periods.Close(ctx, period, at, by)
}

// ReopenPeriod opens a closed period again.
func (c *Client) ReopenPeriod(ctx context.Context, period gl.YearMonth) error {
	current, err := c.periods.Load(ctx)
	if err != nil {
		return err
	}
	if !current.IsClosed(period) {
		return fmt.Errorf("period %s is not closed", period)
	}

	return c.periods.Reopen(ctx, period)
}

// EnsureChart seeds the TT133 chart of accounts when the client has none.
//
// Idempotent, on purpose: reopening a client must never duplicate their chart.
// Circular 46/2025/TT-BTC lets an enterprise modify its own chart, so this only
// ever acts on a client that has no accounts — it never overwrites accounts
// that already exist.
func (c *Client) EnsureChart(ctx context.Context) error {
	existing, err := c.queries.ListActiveAccounts(ctx)
	if err != nil {
		return fmt.Errorf("read the client's chart: %w", err)
	}
	if len(existing) > 0 {
		return nil
	}

	ids := make(map[string]string)

	for _, a := range gl.TT133Chart() {
		id := uuid.NewString()

		parent := sql.NullString{}
		if a.ParentCode != "" {
			parentID, ok := ids[a.ParentCode]
			if !ok {
				return fmt.Errorf("chart lists %s before its parent %s", a.Code, a.ParentCode)
			}
			parent = sql.NullString{String: parentID, Valid: true}
		}

		if err := c.queries.CreateAccount(ctx, mariadb.CreateAccountParams{
			ID:          id,
			Code:        a.Code,
			Name:        a.Name,
			ParentID:    parent,
			AccountType: mariadb.AccountsAccountType(a.Type),
			Nature:      mariadb.AccountsNature(a.Nature),
			IsActive:    true,
		}); err != nil {
			return fmt.Errorf("seed account %s: %w", a.Code, err)
		}

		ids[a.Code] = id
	}

	return nil
}

// Accounts returns the client's chart of accounts, including any the client has
// deactivated — postability is decided by the active flag, and the caller can
// see it.
func (c *Client) Accounts(ctx context.Context) ([]gl.Account, error) {
	rows, err := c.queries.ListAllAccounts(ctx)
	if err != nil {
		return nil, fmt.Errorf("list accounts: %w", err)
	}

	// Resolve parent identity back into the parent's code: the domain speaks
	// codes, storage speaks ids.
	codeByID := make(map[string]string, len(rows))
	for _, r := range rows {
		codeByID[r.ID] = r.Code
	}

	out := make([]gl.Account, 0, len(rows))
	for _, r := range rows {
		parent := ""
		if r.ParentID.Valid {
			parent = codeByID[r.ParentID.String]
		}

		out = append(out, gl.Account{
			Code:       r.Code,
			Name:       r.Name,
			ParentCode: parent,
			Type:       gl.AccountType(r.AccountType),
			Nature:     gl.AccountNature(r.Nature),
			IsActive:   r.IsActive,
		})
	}

	return out, nil
}

// Balance is one account's position in a trial balance. An account appears on
// one side only; the other is zero.
type Balance struct {
	AccountCode string
	AccountName string
	Debit       decimal.Decimal
	Credit      decimal.Decimal
}

// TrialBalance is the client's books summarised by account.
type TrialBalance struct {
	Rows   []Balance
	Debit  decimal.Decimal
	Credit decimal.Decimal
}

// TrialBalance sums posted lines by account.
//
// It reads only what has been posted: an unposted draft is not part of the
// books, and including one would report figures that no one has signed off.
func (c *Client) TrialBalance(ctx context.Context) (TrialBalance, error) {
	rows, err := c.db.QueryContext(ctx, `
		SELECT a.code, a.name, vl.side, SUM(vl.amount)
		FROM voucher_lines vl
		JOIN accounts a ON a.id = vl.account_id
		JOIN vouchers v ON v.id = vl.voucher_id
		WHERE v.is_posted = TRUE
		GROUP BY a.code, a.name, vl.side
		ORDER BY a.code, vl.side`)
	if err != nil {
		return TrialBalance{}, fmt.Errorf("sum posted lines: %w", err)
	}
	defer rows.Close()

	type key struct{ code, side string }

	var (
		order  []string
		names  = make(map[string]string)
		amount = make(map[key]decimal.Decimal)
		tb     TrialBalance
	)

	for rows.Next() {
		var code, name, side, raw string
		if err := rows.Scan(&code, &name, &side, &raw); err != nil {
			return TrialBalance{}, fmt.Errorf("scan balance row: %w", err)
		}

		value, err := decimal.NewFromString(raw)
		if err != nil {
			return TrialBalance{}, fmt.Errorf("balance %q for %s is not a number: %w", raw, code, err)
		}

		if _, seen := names[code]; !seen {
			order = append(order, code)
			names[code] = name
		}
		amount[key{code, side}] = value
	}
	if err := rows.Err(); err != nil {
		return TrialBalance{}, fmt.Errorf("iterate balances: %w", err)
	}

	for _, code := range order {
		debit := amount[key{code, "DEBIT"}]
		credit := amount[key{code, "CREDIT"}]

		tb.Rows = append(tb.Rows, Balance{
			AccountCode: code,
			AccountName: names[code],
			Debit:       debit,
			Credit:      credit,
		})
		tb.Debit = tb.Debit.Add(debit)
		tb.Credit = tb.Credit.Add(credit)
	}

	return tb, nil
}

func (c *Client) Voucher(ctx context.Context, id string) (*gl.Voucher, error) {
	return c.vouchers.GetVoucher(ctx, id)
}

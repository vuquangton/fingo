package web

import (
	"context"
	"time"
)

// Books is everything the pages read and write.
//
// One interface rather than eight, because there is one implementation and the
// screens share it: the dashboard sums the same ledger the trial balance reads,
// and a document's status is what the payment screen changes. Splitting it into
// per-page ports would be eight names for one collaborator, and the caller would
// spend its time assembling them.
//
// The interface is declared here, where it is used, so that the pages depend on
// what they ask for and not on how the books are stored. Two implementations
// satisfy it today: the in-memory books this repository runs the tests against,
// and — the next slice — an adapter over the MariaDB ledger.
//
// Every method returns an error whose text is shown to the operator unchanged.
// Handlers do not translate errors, so an implementation is expected to write
// messages a person can act on.
type Books interface {
	// Dashboard summarises where the books stand: the current period, what is
	// still owed to and by the business, and the cash position.
	Dashboard(ctx context.Context) (Dashboard, error)

	// Documents lists documents, newest first. filter.Type narrows the list to
	// one kind when it is not empty.
	Documents(ctx context.Context, filter DocumentFilter) ([]DocumentRow, error)

	// Document returns one document in full, including its lines and what has
	// settled it.
	Document(ctx context.Context, id string) (DocumentPage, error)

	// CreateInvoice records a sales invoice and posts it. It returns the new
	// document's id.
	CreateInvoice(ctx context.Context, in InvoiceInput) (string, error)

	// CreatePayment records a receipt or payment and posts it. When Against is
	// set the money is applied to that document, which is what moves its
	// status.
	CreatePayment(ctx context.Context, in PaymentInput) (string, error)

	// CreateJournal records a manual journal entry and posts it. The lines must
	// balance; the error says by how much they do not.
	CreateJournal(ctx context.Context, in JournalInput) (string, error)

	// Accounts returns the chart of accounts, by code.
	Accounts(ctx context.Context) ([]AccountRow, error)

	// CreateAccount adds an account to the chart.
	CreateAccount(ctx context.Context, in AccountInput) error

	// TrialBalance reads every account's position for the current period.
	TrialBalance(ctx context.Context) (TrialBalancePage, error)

	// ProfitAndLoss reads the current period's revenue and expenses.
	ProfitAndLoss(ctx context.Context) (ProfitAndLossPage, error)

	// BalanceSheet reads the position at the end of the current period.
	BalanceSheet(ctx context.Context) (BalanceSheetPage, error)

	// Settings returns the periods, the VAT rates and the posting rules.
	Settings(ctx context.Context) (SettingsPage, error)

	// CreatePeriod adds an accounting period.
	CreatePeriod(ctx context.Context, in PeriodInput) error

	// CreateVATRate adds a VAT rate.
	CreateVATRate(ctx context.Context, in VATRateInput) error
}

// DocumentFilter narrows a document list.
type DocumentFilter struct {
	// Type is a document kind — "invoice", "payment", "journal" — or empty for
	// every kind.
	Type string
}

// InvoiceInput is a sales invoice as the form collected it.
//
// Amounts are whole đồng and already parsed: a handler that hands a string
// through would push the same parsing into every implementation.
type InvoiceInput struct {
	// Customer is the counterparty's name as the form collected it. There is no
	// partner directory wired yet, so this is free text and is stored as the
	// document's partner identifier.
	Customer    string
	Date        time.Time
	Description string
	// NetAmount is the invoice value before VAT.
	NetAmount int64
	// VATRateID names a rate in the VAT registry. Empty means no VAT, which is
	// not the same as a zero rate: one is outside the tax, the other is taxed
	// at nothing.
	VATRateID string
}

// PaymentInput is a receipt or payment as the form collected it.
type PaymentInput struct {
	Customer string
	Date     time.Time
	Amount   int64
	// AccountCode is the cash or bank account the money moved through.
	AccountCode string
	// Against is the document being settled, when the money is settling one.
	Against string
}

// JournalLineInput is one side of a manual journal entry.
type JournalLineInput struct {
	AccountCode string
	// Side is "debit" or "credit", as the form's radio buttons named it.
	Side   string
	Amount int64
}

// JournalInput is a manual journal entry as the form collected it.
type JournalInput struct {
	Date        time.Time
	Description string
	Lines       []JournalLineInput
}

// AccountInput is a new chart account as the form collected it.
type AccountInput struct {
	Code       string
	Name       string
	ParentCode string
	Type       string
	Nature     string
}

// PeriodInput is a new accounting period as the form collected it.
type PeriodInput struct {
	ID   string
	From time.Time
	To   time.Time
}

// VATRateInput is a new VAT rate as the form collected it.
type VATRateInput struct {
	ID string
	// RateBasisPoints is the rate in hundredths of a percent: 10% is 1000. The
	// registry stores it that way and a float must never carry a tax rate.
	RateBasisPoints int
	From            time.Time
	To              time.Time
}

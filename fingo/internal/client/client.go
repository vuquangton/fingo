// Package client holds the practice's registry of client companies, and the
// scope that every financial repository is addressed by.
//
// There are two ideas in here and they are deliberately separate.
//
// A Client is an administrative fact: a company the practice keeps books for,
// with a name and a tax code. The registry holds no financial data of its own,
// so it can be listed, renamed and closed without touching a ledger.
//
// A Scope is the boundary around one client's books. It is not a filter a caller
// may forget to apply: it is the first argument of every repository method that
// touches financial data, and a zero Scope is refused before any query runs. A
// repository written against a Scope has no way to express "all clients",
// because there is no method that does not take one.
//
// The middleware in this package builds the Scope from the authenticated
// request, so a handler never chooses the client it reads — it is handed the one
// the request was authenticated for.
//
// This reverses one clause of ADR 0001, which held that one database per client
// left no need for a client column. See docs/adr/0003-client-data-boundary.md,
// and migration db/migrations/00015_client_scope.sql for the schema.
package client

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

// Errors a caller may need to tell apart.
var (
	// ErrInvalidClient is a client that could not be registered: no id, no
	// name, or a name or tax code longer than the column it is stored in.
	ErrInvalidClient = errors.New("client: invalid client")

	// ErrClientExists is a registration of an id already in the registry.
	ErrClientExists = errors.New("client: a client with this id already exists")

	// ErrUnknownClient is a lookup, update or status change for an id the
	// registry does not hold.
	ErrUnknownClient = errors.New("client: no such client")

	// ErrClientInactive is a request for a client that has been closed. The
	// client is known; it is simply not being served any more.
	ErrClientInactive = errors.New("client: client is not active")
)

// Column widths. These are the widths of the columns the client is stored in,
// and they are checked here so an over-long name is refused at the point it is
// entered rather than silently truncated by the database.
const (
	// MaxNameLen bounds the company name, in bytes of the stored column.
	MaxNameLen = 255

	// MaxTaxIDLen bounds the tax code. A Vietnamese mã số thuế is ten digits
	// plus a three-digit branch suffix; the column allows room beyond that.
	MaxTaxIDLen = 32
)

// Client is one company the practice keeps books for.
//
// It is the unit of the books, not a user of the system: one operator serves
// many clients, and each client's ledger is separate from every other's.
type Client struct {
	// ID identifies the client for the life of the practice's relationship
	// with them. Stable — usually a UUID — because it is what every financial
	// row is stamped with and what a scope is built from.
	ID string

	// Name is the company name (tên doanh nghiệp).
	Name string

	// TaxID is Mã số thuế, normalised. Optional at registration, because a
	// client may engage before its paperwork is to hand, but unique once set:
	// two clients with one tax code would be one company registered twice.
	TaxID string

	// CreatedAt is when the client was engaged, in UTC.
	CreatedAt time.Time

	// Active reports whether the practice is still serving the client. A
	// client that has left is deactivated rather than deleted, because its
	// ledger and its history outlive the engagement.
	Active bool
}

// NewClient builds a client, trimming and normalising what was entered.
//
// It is the only supported way to make one, and it stamps CreatedAt — now, in
// UTC, if no instant was given — so that a client can never exist without a date
// attached. Normalising here rather than at the screen means an import, a script
// or a future screen all get the same treatment.
func NewClient(id, name, taxID string, at time.Time) (Client, error) {
	c := Client{
		ID:        id,
		Name:      name,
		TaxID:     taxID,
		CreatedAt: at,
		Active:    true,
	}.Normalized()

	if c.CreatedAt.IsZero() {
		c.CreatedAt = time.Now().UTC()
	}

	if err := c.Validate(); err != nil {
		return Client{}, err
	}

	return c, nil
}

// Validate reports whether the client could be registered and served.
func (c Client) Validate() error {
	if strings.TrimSpace(c.ID) == "" {
		return fmt.Errorf("%w: a client needs an id", ErrInvalidClient)
	}
	if strings.TrimSpace(c.Name) == "" {
		return fmt.Errorf("%w: a client needs a name", ErrInvalidClient)
	}
	if len(c.Name) > MaxNameLen {
		return fmt.Errorf("%w: the name is %d bytes, over the %d-byte column",
			ErrInvalidClient, len(c.Name), MaxNameLen)
	}
	if len(c.TaxID) > MaxTaxIDLen {
		return fmt.Errorf("%w: the tax code is %d bytes, over the %d-byte column",
			ErrInvalidClient, len(c.TaxID), MaxTaxIDLen)
	}

	return nil
}

// Normalized returns the client as it should be stored: trimmed, with its tax
// code in the one form the registry compares on, and dated in UTC.
//
// It runs on the way into every registry write as well as in NewClient, so a
// Client assembled as a struct literal — by a migration, an import, or a test —
// cannot skip it. A tax code stored with its punctuation still in it is a
// duplicate that no uniqueness check can find.
func (c Client) Normalized() Client {
	c.ID = strings.TrimSpace(c.ID)
	c.Name = strings.TrimSpace(c.Name)
	c.TaxID = NormalizeTaxID(c.TaxID)

	if !c.CreatedAt.IsZero() {
		c.CreatedAt = c.CreatedAt.UTC()
	}

	return c
}

// NormalizeTaxID strips the separators people type into a tax code.
//
// The same company is written "0101234567", "0101 234 567", "0101.234.567",
// and "0101234567-001" for a branch. Collapsing those to one form is what makes
// "is this company already registered?" a question the registry can answer, and
// what stops one client being onboarded twice.
//
// Digits survive, and so does the hyphen before a branch number. Everything else
// goes. A string with no digits at all is empty, not punctuation.
//
// This mirrors internal/partner.NormalizeTaxID. The two are deliberately not
// shared: the registry is the control layer and must be usable — to list and
// close clients — when no client's books are reachable at all.
func NormalizeTaxID(raw string) string {
	var kept strings.Builder

	for _, r := range raw {
		if (r >= '0' && r <= '9') || r == '-' {
			kept.WriteRune(r)
		}
	}

	out := strings.Trim(kept.String(), "-")
	if !strings.ContainsAny(out, "0123456789") {
		return ""
	}

	return out
}

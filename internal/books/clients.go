package books

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"

	"fingo/internal/store"
)

// Client is one set of books the practice keeps.
type Client struct {
	ID     string
	Name   string
	TaxID  string
	Active bool
}

// ErrClientExists is returned for a tax code the registry already holds.
var ErrClientExists = errors.New("books: a client with that tax code already exists")

// ListClients returns the registry.
//
// The registry is practice-level, so this deliberately does not take a client
// scope: it is the list you choose a scope *from*, and row-level security does
// not apply to it.
func ListClients(ctx context.Context, st *store.Store) ([]Client, error) {
	rows, err := st.Pool().Query(ctx, `
		SELECT id::text, name, tax_id, active FROM clients ORDER BY name`)
	if err != nil {
		return nil, fmt.Errorf("books: list clients: %w", err)
	}
	defer rows.Close()

	var out []Client
	for rows.Next() {
		var c Client
		if err := rows.Scan(&c.ID, &c.Name, &c.TaxID, &c.Active); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// CreateClient registers a client and gives it the statutory chart.
//
// The chart is copied by `provision_client_chart` inside a transaction scoped to
// the new client: row-level security then permits exactly the rows the function
// writes, and refuses any row belonging to somebody else. Doing it without the
// scope would be refused — which is the boundary working.
func CreateClient(ctx context.Context, st *store.Store, name, taxID string) (Client, error) {
	name = strings.TrimSpace(name)
	taxID = strings.TrimSpace(taxID)

	switch {
	case name == "":
		return Client{}, errors.New("books: a client needs a name")
	case taxID == "":
		return Client{}, errors.New("books: a client needs a tax code")
	}

	var id string
	err := st.Pool().QueryRow(ctx, `
		INSERT INTO clients (name, tax_id) VALUES ($1, $2) RETURNING id::text`,
		name, taxID).Scan(&id)
	if err != nil {
		if isUniqueViolation(err) {
			return Client{}, ErrClientExists
		}
		return Client{}, fmt.Errorf("books: create client: %w", err)
	}

	scope, err := store.NewScope(id)
	if err != nil {
		return Client{}, err
	}

	if err := st.InClient(ctx, scope, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `SELECT provision_client_chart($1)`, id)
		return err
	}); err != nil {
		return Client{}, fmt.Errorf("books: provision the chart for %s: %w", name, err)
	}

	return Client{ID: id, Name: name, TaxID: taxID, Active: true}, nil
}

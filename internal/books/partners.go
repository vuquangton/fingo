package books

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"

	"fingo/internal/store"
)

// PartnerKind is who a partner is to the practice.
type PartnerKind string

const (
	PartnerCustomer PartnerKind = "CUSTOMER"
	PartnerSupplier PartnerKind = "SUPPLIER"
	PartnerBoth     PartnerKind = "BOTH"
)

// Partner is a customer or a supplier.
type Partner struct {
	ID     string
	Name   string
	TaxID  string
	Kind   PartnerKind
	Active bool
}

// ErrPartnerExists is returned when the name or tax code is already taken.
var ErrPartnerExists = errors.New("books: a partner with that name or tax code already exists")

// WalkInName is the customer an anonymous sale is booked to.
//
// A sale still has to name who owes it, so a retail sale is booked to one
// standing customer rather than to nobody: TK 131 then reconciles, and the
// operator can see the retail balance instead of an unexplained lump.
const WalkInName = "Khách lẻ"

// ListPartners returns every partner, by name.
func (b *Books) ListPartners(ctx context.Context) ([]Partner, error) {
	var out []Partner

	err := b.store.Read(ctx, b.scope, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			SELECT id, name, tax_id, kind, active FROM partners ORDER BY name`)
		if err != nil {
			return err
		}
		defer rows.Close()

		for rows.Next() {
			var p Partner
			var kind string
			if err := rows.Scan(&p.ID, &p.Name, &p.TaxID, &kind, &p.Active); err != nil {
				return err
			}
			p.Kind = PartnerKind(kind)
			out = append(out, p)
		}
		return rows.Err()
	})

	return out, err
}

// CreatePartner adds a customer or supplier.
func (b *Books) CreatePartner(ctx context.Context, p Partner) (Partner, error) {
	p.Name = strings.TrimSpace(p.Name)
	p.TaxID = strings.TrimSpace(p.TaxID)
	p.Kind = PartnerKind(strings.ToUpper(strings.TrimSpace(string(p.Kind))))

	switch {
	case p.Name == "":
		return Partner{}, errors.New("books: a partner needs a name")
	case p.Kind != PartnerCustomer && p.Kind != PartnerSupplier && p.Kind != PartnerBoth:
		return Partner{}, fmt.Errorf("books: %q is not a partner kind", p.Kind)
	}

	p.Active = true

	err := b.store.InClient(ctx, b.scope, func(tx pgx.Tx) error {
		return tx.QueryRow(ctx, `
			INSERT INTO partners (client_id, name, tax_id, kind)
			VALUES ($1, $2, $3, $4)
			RETURNING id`,
			b.scope.ID(), p.Name, p.TaxID, string(p.Kind)).Scan(&p.ID)
	})
	if err != nil {
		if isUniqueViolation(err) {
			return Partner{}, ErrPartnerExists
		}
		return Partner{}, err
	}

	return p, nil
}

// WalkIn returns the standing retail customer, creating it on first use.
func (b *Books) WalkIn(ctx context.Context) (Partner, error) {
	var p Partner

	err := b.store.InClient(ctx, b.scope, func(tx pgx.Tx) error {
		return tx.QueryRow(ctx, `
			INSERT INTO partners (client_id, name, tax_id, kind)
			VALUES ($1, $2, '', 'CUSTOMER')
			ON CONFLICT (client_id, name) DO UPDATE SET name = EXCLUDED.name
			RETURNING id, name, tax_id, kind, active`,
			b.scope.ID(), WalkInName).Scan(&p.ID, &p.Name, &p.TaxID, &p.Kind, &p.Active)
	})
	if err != nil {
		return Partner{}, fmt.Errorf("books: open the walk-in customer: %w", err)
	}
	return p, nil
}

// isUniqueViolation reports whether PostgreSQL refused a duplicate.
func isUniqueViolation(err error) bool {
	var pgErr interface{ SQLState() string }
	return errors.As(err, &pgErr) && pgErr.SQLState() == "23505"
}

var _ = store.Scope{}

// Package partnerstore stores the partner directory in MariaDB.
//
// It is the adapter for partner.Repository, and it writes the `trade_partners`
// table — the directory model, not the older `partners` table that
// `voucher_lines.partner_id` points at. The two coexist; see the note at the top
// of migration 00013.
package partnerstore

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	mariadb "fingo/internal/adapter/mariadb/sqlc"
	"fingo/internal/partner"
)

var _ partner.Repository = (*Store)(nil)

// Store is the MariaDB adapter for partner.Repository.
type Store struct {
	db      *sql.DB
	queries *mariadb.Queries
}

// NewStore builds the adapter over an open connection.
func NewStore(db *sql.DB) *Store {
	return &Store{db: db, queries: mariadb.New(db)}
}

// Create stores a partner, warning when another row already holds its tax id.
//
// The duplicate check is a read followed by the insert, with no transaction
// around it. That is deliberate: a duplicate is permitted, so there is no
// invariant for a transaction to protect. Two writers racing on the same tax id
// can at worst both miss the warning — and the second row is one an operator can
// see and merge, which is exactly what the warning is for.
func (s *Store) Create(ctx context.Context, p partner.Partner) (*partner.Warning, error) {
	p = p.Normalized()
	if err := p.Validate(); err != nil {
		return nil, err
	}

	warning, err := s.duplicateWarning(ctx, p)
	if err != nil {
		return nil, err
	}

	kind, err := toDBType(p.Type)
	if err != nil {
		return nil, err
	}

	if err := s.queries.CreateTradePartner(ctx, mariadb.CreateTradePartnerParams{
		ID:               p.ID,
		Name:             p.Name,
		PartnerType:      kind,
		TaxID:            nullString(p.TaxID),
		PaymentTermsDays: int32(p.PaymentTerms),
		IsActive:         p.Active,
	}); err != nil {
		return nil, fmt.Errorf("create partner %s: %w", p.ID, err)
	}

	return warning, nil
}

// Get returns one partner.
func (s *Store) Get(ctx context.Context, id string) (partner.Partner, error) {
	row, err := s.queries.GetTradePartner(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return partner.Partner{}, fmt.Errorf("%w: %s", partner.ErrUnknownPartner, id)
	}
	if err != nil {
		return partner.Partner{}, fmt.Errorf("get partner %s: %w", id, err)
	}

	return toPartner(row.ID, row.Name, row.PartnerType, row.TaxID, row.PaymentTermsDays, row.IsActive)
}

// List returns every partner, by name.
func (s *Store) List(ctx context.Context) ([]partner.Partner, error) {
	rows, err := s.queries.ListTradePartners(ctx)
	if err != nil {
		return nil, fmt.Errorf("list partners: %w", err)
	}

	out := make([]partner.Partner, 0, len(rows))
	for _, r := range rows {
		p, err := toPartner(r.ID, r.Name, r.PartnerType, r.TaxID, r.PaymentTermsDays, r.IsActive)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}

	return out, nil
}

// Update replaces a stored partner, warning when another row already holds its
// tax id.
//
// The row is locked and re-read inside the transaction rather than inferred from
// the UPDATE's row count. MySQL reports the rows an UPDATE CHANGED, not the rows
// it matched, so re-saving a partner under the values it already has would
// report zero — and a zero read as "no such partner" would refuse an update that
// did exactly what it was asked to.
func (s *Store) Update(ctx context.Context, p partner.Partner) (*partner.Warning, error) {
	p = p.Normalized()
	if err := p.Validate(); err != nil {
		return nil, err
	}

	kind, err := toDBType(p.Type)
	if err != nil {
		return nil, err
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("update partner %s: begin: %w", p.ID, err)
	}
	defer func() { _ = tx.Rollback() }()

	q := s.queries.WithTx(tx)

	if _, err := q.GetTradePartnerForUpdate(ctx, p.ID); errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("%w: %s", partner.ErrUnknownPartner, p.ID)
	} else if err != nil {
		return nil, fmt.Errorf("update partner %s: lock: %w", p.ID, err)
	}

	warning, err := s.duplicateWarning(ctx, p)
	if err != nil {
		return nil, err
	}

	if _, err := q.UpdateTradePartner(ctx, mariadb.UpdateTradePartnerParams{
		Name:             p.Name,
		PartnerType:      kind,
		TaxID:            nullString(p.TaxID),
		PaymentTermsDays: int32(p.PaymentTerms),
		IsActive:         p.Active,
		ID:               p.ID,
	}); err != nil {
		return nil, fmt.Errorf("update partner %s: %w", p.ID, err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("update partner %s: commit: %w", p.ID, err)
	}

	return warning, nil
}

// Delete removes a partner.
func (s *Store) Delete(ctx context.Context, id string) error {
	affected, err := s.queries.DeleteTradePartner(ctx, id)
	if err != nil {
		return fmt.Errorf("delete partner %s: %w", id, err)
	}
	if affected == 0 {
		return fmt.Errorf("%w: %s", partner.ErrUnknownPartner, id)
	}

	return nil
}

// duplicateWarning asks the directory who else carries this tax id.
//
// A partner with no tax id is not asked about: nothing is being duplicated, and
// the query would match nothing anyway.
func (s *Store) duplicateWarning(ctx context.Context, p partner.Partner) (*partner.Warning, error) {
	if p.TaxID == "" {
		return nil, nil
	}

	rows, err := s.queries.ListTradePartnersByTaxID(ctx, nullString(p.TaxID))
	if err != nil {
		return nil, fmt.Errorf("check tax id %s: %w", p.TaxID, err)
	}

	others := make([]partner.Partner, 0, len(rows))
	for _, r := range rows {
		other, err := toPartner(r.ID, r.Name, r.PartnerType, r.TaxID, r.PaymentTermsDays, r.IsActive)
		if err != nil {
			return nil, err
		}
		others = append(others, other)
	}

	return partner.CheckDuplicateTaxID(p, others), nil
}

// toPartner assembles a domain partner from a stored row.
func toPartner(
	id, name string,
	kind mariadb.TradePartnersPartnerType,
	taxID sql.NullString,
	paymentTermsDays int32,
	active bool,
) (partner.Partner, error) {
	partnerType, err := fromDBType(kind)
	if err != nil {
		return partner.Partner{}, fmt.Errorf("partner %s: %w", id, err)
	}

	return partner.Partner{
		ID:           id,
		Name:         name,
		Type:         partnerType,
		TaxID:        taxID.String,
		PaymentTerms: int(paymentTermsDays),
		Active:       active,
	}, nil
}

// toDBType maps the domain's type onto the stored enum.
//
// The two are spelled differently on purpose: the enum is upper case because
// every other enum in the schema is, and the domain says the words an operator
// says. An unknown type is refused rather than written as anything.
func toDBType(t partner.Type) (mariadb.TradePartnersPartnerType, error) {
	switch t {
	case partner.TypeCustomer:
		return mariadb.TradePartnersPartnerTypeCUSTOMER, nil
	case partner.TypeSupplier:
		return mariadb.TradePartnersPartnerTypeSUPPLIER, nil
	case partner.TypeBoth:
		return mariadb.TradePartnersPartnerTypeBOTH, nil
	}

	return "", fmt.Errorf("%w: partner type %q has no stored form", partner.ErrInvalidPartner, t)
}

// fromDBType maps a stored enum back onto the domain's type.
func fromDBType(t mariadb.TradePartnersPartnerType) (partner.Type, error) {
	switch t {
	case mariadb.TradePartnersPartnerTypeCUSTOMER:
		return partner.TypeCustomer, nil
	case mariadb.TradePartnersPartnerTypeSUPPLIER:
		return partner.TypeSupplier, nil
	case mariadb.TradePartnersPartnerTypeBOTH:
		return partner.TypeBoth, nil
	}

	return "", fmt.Errorf("%w: stored partner type %q is not one the domain knows", partner.ErrInvalidPartner, t)
}

// nullString stores an absent tax id as NULL rather than as an empty string, so
// that "no tax id" is one value in the column and not two.
func nullString(value string) sql.NullString {
	return sql.NullString{String: value, Valid: value != ""}
}

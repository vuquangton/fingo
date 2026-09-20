package repository

import (
	"context"
	"database/sql"

	sqlc "fingo/internal/adapter/mariadb/sqlc"
	"fingo/internal/domain/system"
)

type BranchOrgUnitRepo struct {
	db      *sql.DB
	queries *sqlc.Queries
}

func NewBranchOrgUnitRepo(db *sql.DB) *BranchOrgUnitRepo {
	return &BranchOrgUnitRepo{
		db:      db,
		queries: sqlc.New(db),
	}
}

func (r *BranchOrgUnitRepo) CreateOrgUnit(ctx context.Context, u *system.BranchOrgUnit) error {
	var parentID sql.NullString
	if u.ParentID != nil && *u.ParentID != "" {
		parentID = sql.NullString{String: *u.ParentID, Valid: true}
	}

	arg := sqlc.CreateBranchOrgUnitParams{
		ID:                        u.ID,
		ParentID:                  parentID,
		CompanyProfileID:          u.CompanyProfileID,
		Code:                      u.Code,
		Name:                      u.Name,
		UnitType:                  sqlc.BranchOrgUnitsUnitType(u.UnitType),
		AccountingGovernance:      sqlc.BranchOrgUnitsAccountingGovernance(u.AccountingGovernance),
		TaxFilingMechanism:        sqlc.BranchOrgUnitsTaxFilingMechanism(u.TaxFilingMechanism),
		TaxCode:                   toNullString(u.TaxCode),
		TaxAuthorityCode:          toNullString(u.TaxAuthorityCode),
		TaxAuthorityName:          toNullString(u.TaxAuthorityName),
		ProvinceCityCode:          u.ProvinceCityCode,
		Address:                   u.Address,
		ManagerName:               toNullString(u.ManagerName),
		ChiefAccountant:           toNullString(u.ChiefAccountant),
		InternalReceivableAccount: u.InternalReceivableAccount,
		InternalPayableAccount:    u.InternalPayableAccount,
		HasOwnEinvoice:            u.HasOwnEInvoice,
		IsActive:                  u.IsActive,
	}

	return r.queries.CreateBranchOrgUnit(ctx, arg)
}

func (r *BranchOrgUnitRepo) GetOrgUnitByID(ctx context.Context, id string) (*system.BranchOrgUnit, error) {
	row, err := r.queries.GetBranchOrgUnitByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return mapRowToBranchOrgUnit(row), nil
}

func (r *BranchOrgUnitRepo) GetOrgUnitByCode(ctx context.Context, companyID, code string) (*system.BranchOrgUnit, error) {
	row, err := r.queries.GetBranchOrgUnitByCode(ctx, sqlc.GetBranchOrgUnitByCodeParams{
		CompanyProfileID: companyID,
		Code:             code,
	})
	if err != nil {
		return nil, err
	}
	return mapRowToBranchOrgUnit(row), nil
}

func (r *BranchOrgUnitRepo) ListOrgUnitsByCompany(ctx context.Context, companyID string) ([]system.BranchOrgUnit, error) {
	rows, err := r.queries.ListBranchOrgUnitsByCompany(ctx, companyID)
	if err != nil {
		return nil, err
	}

	result := make([]system.BranchOrgUnit, 0, len(rows))
	for _, row := range rows {
		result = append(result, *mapRowToBranchOrgUnit(row))
	}
	return result, nil
}

func (r *BranchOrgUnitRepo) UpdateOrgUnit(ctx context.Context, u *system.BranchOrgUnit) error {
	arg := sqlc.UpdateBranchOrgUnitParams{
		ID:                        u.ID,
		Name:                      u.Name,
		AccountingGovernance:      sqlc.BranchOrgUnitsAccountingGovernance(u.AccountingGovernance),
		TaxFilingMechanism:        sqlc.BranchOrgUnitsTaxFilingMechanism(u.TaxFilingMechanism),
		TaxCode:                   toNullString(u.TaxCode),
		TaxAuthorityCode:          toNullString(u.TaxAuthorityCode),
		TaxAuthorityName:          toNullString(u.TaxAuthorityName),
		ProvinceCityCode:          u.ProvinceCityCode,
		Address:                   u.Address,
		ManagerName:               toNullString(u.ManagerName),
		ChiefAccountant:           toNullString(u.ChiefAccountant),
		InternalReceivableAccount: u.InternalReceivableAccount,
		InternalPayableAccount:    u.InternalPayableAccount,
		HasOwnEinvoice:            u.HasOwnEInvoice,
		IsActive:                  u.IsActive,
	}
	return r.queries.UpdateBranchOrgUnit(ctx, arg)
}

func mapRowToBranchOrgUnit(row sqlc.BranchOrgUnit) *system.BranchOrgUnit {
	var parentID *string
	if row.ParentID.Valid {
		p := row.ParentID.String
		parentID = &p
	}

	return &system.BranchOrgUnit{
		ID:                        row.ID,
		ParentID:                  parentID,
		CompanyProfileID:          row.CompanyProfileID,
		Code:                      row.Code,
		Name:                      row.Name,
		UnitType:                  system.OrgUnitType(row.UnitType),
		AccountingGovernance:      system.AccountingGovernance(row.AccountingGovernance),
		TaxFilingMechanism:        system.TaxFilingMechanism(row.TaxFilingMechanism),
		TaxCode:                   fromNullString(row.TaxCode),
		TaxAuthorityCode:          fromNullString(row.TaxAuthorityCode),
		TaxAuthorityName:          fromNullString(row.TaxAuthorityName),
		ProvinceCityCode:          row.ProvinceCityCode,
		Address:                   row.Address,
		ManagerName:               fromNullString(row.ManagerName),
		ChiefAccountant:           fromNullString(row.ChiefAccountant),
		InternalReceivableAccount: row.InternalReceivableAccount,
		InternalPayableAccount:    row.InternalPayableAccount,
		HasOwnEInvoice:            row.HasOwnEinvoice,
		IsActive:                  row.IsActive,
		CreatedAt:                 row.CreatedAt,
		UpdatedAt:                 row.UpdatedAt,
	}
}

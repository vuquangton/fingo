package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	sqlc "fingo/internal/adapter/mariadb/sqlc"
	"fingo/internal/domain/system"
)

type CompanyProfileRepo struct {
	db      *sql.DB
	queries *sqlc.Queries
}

func NewCompanyProfileRepo(db *sql.DB) *CompanyProfileRepo {
	return &CompanyProfileRepo{
		db:      db,
		queries: sqlc.New(db),
	}
}

func toNullString(s string) sql.NullString {
	if s == "" {
		return sql.NullString{Valid: false}
	}
	return sql.NullString{String: s, Valid: true}
}

func fromNullString(ns sql.NullString) string {
	if ns.Valid {
		return ns.String
	}
	return ""
}

func (r *CompanyProfileRepo) GetProfile(ctx context.Context) (*system.ProductionCompanyProfile, error) {
	row, err := r.queries.GetActiveCompanyProfile(ctx)
	if err != nil {
		return nil, err
	}

	var banks []system.BankAccountRegistration
	if len(row.RegisteredBanks) > 0 {
		_ = json.Unmarshal(row.RegisteredBanks, &banks)
	}

	var einv system.EInvoiceConfig
	if len(row.EinvoiceConfig) > 0 {
		_ = json.Unmarshal(row.EinvoiceConfig, &einv)
	}

	profile := &system.ProductionCompanyProfile{
		ID:                     row.ID,
		TaxCode:                row.TaxCode,
		LegalName:              row.LegalName,
		TradeName:              fromNullString(row.TradeName),
		EnglishName:            fromNullString(row.EnglishName),
		Address:                row.Address,
		ProvinceCity:           fromNullString(row.ProvinceCity),
		DistrictWard:           fromNullString(row.DistrictWard),
		Phone:                  fromNullString(row.Phone),
		Email:                  fromNullString(row.Email),
		Website:                fromNullString(row.Website),
		LegalRepresentative:    row.LegalRepresentative,
		RepresentativePosition: fromNullString(row.RepresentativePosition),
		ChiefAccountant:        row.ChiefAccountant,
		TaxAuthority: system.TaxAuthority{
			Code: row.TaxAuthorityCode,
			Name: row.TaxAuthorityName,
		},
		StateBudgetChapter:   fromNullString(row.StateBudgetChapter),
		Regime:               system.AccountingRegime(row.Regime),
		BaseCurrency:         row.BaseCurrency,
		FiscalYearStartMonth: time.Month(row.FiscalYearStartMonth),
		VATMethod:            system.VATMethod(row.VatMethod),
		CostingMethod:        system.CostingMethod(row.CostingMethod),
		BusinessType:         system.BusinessType(row.BusinessType),
		RegisteredBanks:      banks,
		EInvoice:             einv,
		IsActive:             row.IsActive,
		CreatedAt:            row.CreatedAt,
		UpdatedAt:            row.UpdatedAt,
	}

	if row.LockDate.Valid {
		profile.LockDate = row.LockDate.Time
	}

	return profile, nil
}

func (r *CompanyProfileRepo) SaveProfile(ctx context.Context, p *system.ProductionCompanyProfile) error {
	banks := p.RegisteredBanks
	if banks == nil {
		banks = []system.BankAccountRegistration{}
	}
	banksJSON, _ := json.Marshal(banks)
	einvJSON, _ := json.Marshal(p.EInvoice)

	var lockDate sql.NullTime
	if !p.LockDate.IsZero() {
		lockDate = sql.NullTime{Time: p.LockDate, Valid: true}
	}

	arg := sqlc.UpsertCompanyProfileParams{
		ID:                     p.ID,
		TaxCode:                p.TaxCode,
		LegalName:              p.LegalName,
		TradeName:              toNullString(p.TradeName),
		EnglishName:            toNullString(p.EnglishName),
		Address:                p.Address,
		ProvinceCity:           toNullString(p.ProvinceCity),
		DistrictWard:           toNullString(p.DistrictWard),
		Phone:                  toNullString(p.Phone),
		Email:                  toNullString(p.Email),
		Website:                toNullString(p.Website),
		LegalRepresentative:    p.LegalRepresentative,
		RepresentativePosition: toNullString(p.RepresentativePosition),
		ChiefAccountant:        p.ChiefAccountant,
		TaxAuthorityCode:       p.TaxAuthority.Code,
		TaxAuthorityName:       p.TaxAuthority.Name,
		StateBudgetChapter:     toNullString(p.StateBudgetChapter),
		Regime:                 sqlc.CompanyProfileRegime(p.Regime),
		BaseCurrency:           p.BaseCurrency,
		FiscalYearStartMonth:   int8(p.FiscalYearStartMonth),
		VatMethod:              sqlc.CompanyProfileVatMethod(p.VATMethod),
		CostingMethod:          sqlc.CompanyProfileCostingMethod(p.CostingMethod),
		BusinessType:           sqlc.CompanyProfileBusinessType(p.BusinessType),
		RegisteredBanks:        banksJSON,
		EinvoiceConfig:         einvJSON,
		IsActive:               p.IsActive,
		LockDate:               lockDate,
	}

	return r.queries.UpsertCompanyProfile(ctx, arg)
}

func (r *CompanyProfileRepo) SetLockDate(ctx context.Context, id string, lockDate time.Time) error {
	var lt sql.NullTime
	if !lockDate.IsZero() {
		lt = sql.NullTime{Time: lockDate, Valid: true}
	}
	return r.queries.UpdateLockDate(ctx, sqlc.UpdateLockDateParams{
		ID:       id,
		LockDate: lt,
	})
}

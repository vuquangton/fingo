package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	sqlc "fingo/internal/adapter/mariadb/sqlc"
	"fingo/internal/domain/system"
)

func ptrToNullString(s *string) sql.NullString {
	if s == nil || *s == "" {
		return sql.NullString{Valid: false}
	}
	return sql.NullString{String: *s, Valid: true}
}

func nullStringToPtr(ns sql.NullString) *string {
	if ns.Valid {
		str := ns.String
		return &str
	}
	return nil
}

func toSystemOption(
	id, companyProfileID string,
	branchID sql.NullString,
	category, optionKey, optionValue, dataType, defaultValue, scopeLevel string,
	description sql.NullString,
	isReadonly, isEncrypted bool,
	updatedAt time.Time,
	updatedBy sql.NullString,
) system.SystemOption {
	return system.SystemOption{
		ID:               id,
		CompanyProfileID: companyProfileID,
		BranchID:         nullStringToPtr(branchID),
		Category:         system.OptionCategory(category),
		OptionKey:        optionKey,
		OptionValue:      optionValue,
		DataType:         system.OptionDataType(dataType),
		DefaultValue:     defaultValue,
		ScopeLevel:       system.ScopeLevel(scopeLevel),
		Description:      fromNullString(description),
		IsReadonly:       isReadonly,
		IsEncrypted:      isEncrypted,
		UpdatedAt:        updatedAt,
		UpdatedBy:        fromNullString(updatedBy),
	}
}

func toVoucherConfig(
	id, companyProfileID string,
	branchID sql.NullString,
	voucherType, prefix, pattern, resetFreq string,
	currentSeq int64,
	lastResetDate, updatedAt time.Time,
) system.VoucherNumberingConfig {
	return system.VoucherNumberingConfig{
		ID:               id,
		CompanyProfileID: companyProfileID,
		BranchID:         nullStringToPtr(branchID),
		VoucherType:      voucherType,
		Prefix:           prefix,
		Pattern:          pattern,
		ResetFrequency:   system.ResetFrequency(resetFreq),
		CurrentSequence:  currentSeq,
		LastResetDate:    lastResetDate,
		UpdatedAt:        updatedAt,
	}
}

// SystemOptionRepo implements system.SystemOptionRepository using MariaDB via sqlc
type SystemOptionRepo struct {
	db      *sql.DB
	queries *sqlc.Queries
}

func NewSystemOptionRepo(db *sql.DB) *SystemOptionRepo {
	return &SystemOptionRepo{
		db:      db,
		queries: sqlc.New(db),
	}
}

func (r *SystemOptionRepo) UpsertOption(ctx context.Context, opt *system.SystemOption) error {
	arg := sqlc.UpsertSystemOptionParams{
		ID:               opt.ID,
		CompanyProfileID: opt.CompanyProfileID,
		BranchID:         ptrToNullString(opt.BranchID),
		Category:         string(opt.Category),
		OptionKey:        opt.OptionKey,
		OptionValue:      opt.OptionValue,
		DataType:         string(opt.DataType),
		DefaultValue:     opt.DefaultValue,
		ScopeLevel:       string(opt.ScopeLevel),
		Description:      toNullString(opt.Description),
		IsReadonly:       opt.IsReadonly,
		IsEncrypted:      opt.IsEncrypted,
		UpdatedBy:        toNullString(opt.UpdatedBy),
	}
	if err := r.queries.UpsertSystemOption(ctx, arg); err != nil {
		return fmt.Errorf("failed to upsert system option (%s): %w", opt.OptionKey, err)
	}
	return nil
}

func (r *SystemOptionRepo) GetOption(ctx context.Context, companyID string, branchID *string, key string) (*system.SystemOption, error) {
	row, err := r.queries.GetSystemOption(ctx, sqlc.GetSystemOptionParams{
		CompanyProfileID: companyID,
		BranchID:         ptrToNullString(branchID),
		OptionKey:        key,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get system option (%s): %w", key, err)
	}

	opt := toSystemOption(
		row.ID, row.CompanyProfileID, row.BranchID,
		row.Category, row.OptionKey, row.OptionValue, row.DataType, row.DefaultValue, row.ScopeLevel,
		row.Description, row.IsReadonly, row.IsEncrypted, row.UpdatedAt, row.UpdatedBy,
	)
	return &opt, nil
}

func (r *SystemOptionRepo) GetEffectiveOption(ctx context.Context, companyID string, branchID *string, key string) (*system.SystemOption, error) {
	row, err := r.queries.GetEffectiveSystemOption(ctx, sqlc.GetEffectiveSystemOptionParams{
		CompanyProfileID: companyID,
		OptionKey:        key,
		BranchID:         ptrToNullString(branchID),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get effective system option (%s): %w", key, err)
	}

	opt := toSystemOption(
		row.ID, row.CompanyProfileID, row.BranchID,
		row.Category, row.OptionKey, row.OptionValue, row.DataType, row.DefaultValue, row.ScopeLevel,
		row.Description, row.IsReadonly, row.IsEncrypted, row.UpdatedAt, row.UpdatedBy,
	)
	return &opt, nil
}

func (r *SystemOptionRepo) ListOptionsByCompany(ctx context.Context, companyID string) ([]system.SystemOption, error) {
	rows, err := r.queries.ListSystemOptionsByCompany(ctx, companyID)
	if err != nil {
		return nil, fmt.Errorf("failed to list options for company %s: %w", companyID, err)
	}

	opts := make([]system.SystemOption, 0, len(rows))
	for _, row := range rows {
		opts = append(opts, toSystemOption(
			row.ID, row.CompanyProfileID, row.BranchID,
			row.Category, row.OptionKey, row.OptionValue, row.DataType, row.DefaultValue, row.ScopeLevel,
			row.Description, row.IsReadonly, row.IsEncrypted, row.UpdatedAt, row.UpdatedBy,
		))
	}
	return opts, nil
}

func (r *SystemOptionRepo) ListOptionsByCategory(ctx context.Context, companyID string, category system.OptionCategory) ([]system.SystemOption, error) {
	rows, err := r.queries.ListSystemOptionsByCategory(ctx, sqlc.ListSystemOptionsByCategoryParams{
		CompanyProfileID: companyID,
		Category:         string(category),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list options by category %s: %w", category, err)
	}

	opts := make([]system.SystemOption, 0, len(rows))
	for _, row := range rows {
		opts = append(opts, toSystemOption(
			row.ID, row.CompanyProfileID, row.BranchID,
			row.Category, row.OptionKey, row.OptionValue, row.DataType, row.DefaultValue, row.ScopeLevel,
			row.Description, row.IsReadonly, row.IsEncrypted, row.UpdatedAt, row.UpdatedBy,
		))
	}
	return opts, nil
}

func (r *SystemOptionRepo) RecordHistory(ctx context.Context, h *system.SystemConfigHistory) error {
	if err := r.queries.RecordConfigHistory(ctx, sqlc.RecordConfigHistoryParams{
		ID:               h.ID,
		CompanyProfileID: h.CompanyProfileID,
		OptionKey:        h.OptionKey,
		OldValue:         ptrToNullString(h.OldValue),
		NewValue:         h.NewValue,
		ChangedBy:        h.ChangedBy,
		Reason:           ptrToNullString(h.Reason),
		ClientIp:         ptrToNullString(h.ClientIP),
	}); err != nil {
		return fmt.Errorf("failed to record config history (%s): %w", h.OptionKey, err)
	}
	return nil
}

func (r *SystemOptionRepo) ListHistory(ctx context.Context, companyID, key string, limit int32) ([]system.SystemConfigHistory, error) {
	rows, err := r.queries.ListConfigHistory(ctx, sqlc.ListConfigHistoryParams{
		CompanyProfileID: companyID,
		OptionKey:        key,
		Limit:            limit,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list config history (%s): %w", key, err)
	}

	entries := make([]system.SystemConfigHistory, 0, len(rows))
	for _, row := range rows {
		entries = append(entries, system.SystemConfigHistory{
			ID:               row.ID,
			CompanyProfileID: row.CompanyProfileID,
			OptionKey:        row.OptionKey,
			OldValue:         nullStringToPtr(row.OldValue),
			NewValue:         row.NewValue,
			ChangedBy:        row.ChangedBy,
			ChangedAt:        row.ChangedAt,
			Reason:           nullStringToPtr(row.Reason),
			ClientIP:         nullStringToPtr(row.ClientIp),
		})
	}
	return entries, nil
}

// VoucherNumberingRepo implements system.VoucherNumberingRepository
type VoucherNumberingRepo struct {
	db      *sql.DB
	queries *sqlc.Queries
}

func NewVoucherNumberingRepo(db *sql.DB) *VoucherNumberingRepo {
	return &VoucherNumberingRepo{
		db:      db,
		queries: sqlc.New(db),
	}
}

func (r *VoucherNumberingRepo) UpsertConfig(ctx context.Context, cfg *system.VoucherNumberingConfig) error {
	arg := sqlc.UpsertVoucherNumberingConfigParams{
		ID:               cfg.ID,
		CompanyProfileID: cfg.CompanyProfileID,
		BranchID:         ptrToNullString(cfg.BranchID),
		VoucherType:      cfg.VoucherType,
		Prefix:           cfg.Prefix,
		Pattern:          cfg.Pattern,
		ResetFrequency:   sqlc.VoucherNumberingConfigsResetFrequency(cfg.ResetFrequency),
		CurrentSequence:  cfg.CurrentSequence,
		LastResetDate:    cfg.LastResetDate,
	}
	if err := r.queries.UpsertVoucherNumberingConfig(ctx, arg); err != nil {
		return fmt.Errorf("failed to upsert voucher numbering config (%s): %w", cfg.VoucherType, err)
	}
	return nil
}

func (r *VoucherNumberingRepo) GetConfigByID(ctx context.Context, id string) (*system.VoucherNumberingConfig, error) {
	row, err := r.queries.GetVoucherNumberingConfigByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get voucher numbering config (%s): %w", id, err)
	}
	cfg := toVoucherConfig(
		row.ID, row.CompanyProfileID, row.BranchID,
		row.VoucherType, row.Prefix, row.Pattern, string(row.ResetFrequency),
		row.CurrentSequence, row.LastResetDate, row.UpdatedAt,
	)
	return &cfg, nil
}

func (r *VoucherNumberingRepo) GetEffectiveConfig(ctx context.Context, companyID string, branchID *string, voucherType string) (*system.VoucherNumberingConfig, error) {
	row, err := r.queries.GetEffectiveVoucherNumberingConfig(ctx, sqlc.GetEffectiveVoucherNumberingConfigParams{
		CompanyProfileID: companyID,
		VoucherType:      voucherType,
		BranchID:         ptrToNullString(branchID),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get effective voucher numbering config (%s): %w", voucherType, err)
	}
	cfg := toVoucherConfig(
		row.ID, row.CompanyProfileID, row.BranchID,
		row.VoucherType, row.Prefix, row.Pattern, string(row.ResetFrequency),
		row.CurrentSequence, row.LastResetDate, row.UpdatedAt,
	)
	return &cfg, nil
}

// NextSequence acquires pessimistic row lock (SELECT FOR UPDATE) in a transaction,
// advances the sequence counter, updates MariaDB, and returns the formatted voucher number
func (r *VoucherNumberingRepo) NextSequence(
	ctx context.Context,
	companyID string,
	branchID *string,
	voucherType string,
	voucherDate time.Time,
) (string, *system.VoucherNumberingConfig, error) {
	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return "", nil, fmt.Errorf("failed to begin tx: %w", err)
	}
	defer tx.Rollback()

	qtx := r.queries.WithTx(tx)

	row, err := qtx.GetEffectiveVoucherNumberingConfigForUpdate(ctx, sqlc.GetEffectiveVoucherNumberingConfigForUpdateParams{
		CompanyProfileID: companyID,
		VoucherType:      voucherType,
		BranchID:         ptrToNullString(branchID),
	})
	if err != nil {
		return "", nil, fmt.Errorf("failed to fetch voucher config for update: %w", err)
	}

	cfg := toVoucherConfig(
		row.ID, row.CompanyProfileID, row.BranchID,
		row.VoucherType, row.Prefix, row.Pattern, string(row.ResetFrequency),
		row.CurrentSequence, row.LastResetDate, row.UpdatedAt,
	)

	voucherNumber := cfg.NextNumber(voucherDate)

	err = qtx.UpdateVoucherSequence(ctx, sqlc.UpdateVoucherSequenceParams{
		CurrentSequence: cfg.CurrentSequence,
		LastResetDate:   cfg.LastResetDate,
		ID:              cfg.ID,
	})
	if err != nil {
		return "", nil, fmt.Errorf("failed to update voucher sequence: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return "", nil, fmt.Errorf("failed to commit tx: %w", err)
	}

	return voucherNumber, &cfg, nil
}

func (r *VoucherNumberingRepo) ListConfigs(ctx context.Context, companyID string) ([]system.VoucherNumberingConfig, error) {
	rows, err := r.queries.ListVoucherNumberingConfigs(ctx, companyID)
	if err != nil {
		return nil, fmt.Errorf("failed to list voucher configs for company %s: %w", companyID, err)
	}

	configs := make([]system.VoucherNumberingConfig, 0, len(rows))
	for _, row := range rows {
		configs = append(configs, toVoucherConfig(
			row.ID, row.CompanyProfileID, row.BranchID,
			row.VoucherType, row.Prefix, row.Pattern, string(row.ResetFrequency),
			row.CurrentSequence, row.LastResetDate, row.UpdatedAt,
		))
	}
	return configs, nil
}

package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/shopspring/decimal"

	sqlc "fingo/internal/adapter/mariadb/sqlc"
	"fingo/internal/domain/cash"
)

type CashRepo struct {
	db      *sql.DB
	queries *sqlc.Queries
}

func NewCashRepo(db *sql.DB) *CashRepo {
	return &CashRepo{
		db:      db,
		queries: sqlc.New(db),
	}
}

func (r *CashRepo) SaveCashReceipt(ctx context.Context, cr *cash.CashReceipt) error {
	if cr == nil {
		return errors.New("cash receipt cannot be nil")
	}

	payerAddr := sql.NullString{Valid: false}
	if cr.PayerAddress != "" {
		payerAddr = sql.NullString{String: cr.PayerAddress, Valid: true}
	}

	accompanyingDocs := sql.NullString{Valid: false}
	if cr.AccompanyingDocuments != "" {
		accompanyingDocs = sql.NullString{String: cr.AccompanyingDocuments, Valid: true}
	}

	err := r.queries.CreateCashReceipt(ctx, sqlc.CreateCashReceiptParams{
		ID:                    cr.ID,
		VoucherID:             cr.VoucherID,
		CompanyProfileID:      cr.CompanyProfileID,
		PayerName:             cr.PayerName,
		PayerAddress:          payerAddr,
		Reason:                cr.Reason,
		CashAccountID:         cr.CashAccountID,
		TotalAmountVnd:        cr.TotalAmountVND.String(),
		AccompanyingDocuments: accompanyingDocs,
	})
	if err != nil {
		return fmt.Errorf("failed to save cash receipt: %w", err)
	}

	return nil
}

func (r *CashRepo) GetCashReceiptByID(ctx context.Context, id string) (*cash.CashReceipt, error) {
	row, err := r.queries.GetCashReceiptByID(ctx, id)
	if err != nil {
		return nil, err
	}

	amount, err := decimal.NewFromString(row.TotalAmountVnd)
	if err != nil {
		return nil, fmt.Errorf("invalid cash receipt amount '%s': %w", row.TotalAmountVnd, err)
	}

	return &cash.CashReceipt{
		ID:                    row.ID,
		VoucherID:             row.VoucherID,
		CompanyProfileID:      row.CompanyProfileID,
		PayerName:             row.PayerName,
		PayerAddress:          row.PayerAddress.String,
		Reason:                row.Reason,
		CashAccountID:         row.CashAccountID,
		TotalAmountVND:        amount,
		AccompanyingDocuments: row.AccompanyingDocuments.String,
	}, nil
}

func (r *CashRepo) GetCashReceiptByVoucherID(ctx context.Context, voucherID string) (*cash.CashReceipt, error) {
	row, err := r.queries.GetCashReceiptByVoucherID(ctx, voucherID)
	if err != nil {
		return nil, err
	}

	amount, err := decimal.NewFromString(row.TotalAmountVnd)
	if err != nil {
		return nil, fmt.Errorf("invalid cash receipt amount '%s': %w", row.TotalAmountVnd, err)
	}

	return &cash.CashReceipt{
		ID:                    row.ID,
		VoucherID:             row.VoucherID,
		CompanyProfileID:      row.CompanyProfileID,
		PayerName:             row.PayerName,
		PayerAddress:          row.PayerAddress.String,
		Reason:                row.Reason,
		CashAccountID:         row.CashAccountID,
		TotalAmountVND:        amount,
		AccompanyingDocuments: row.AccompanyingDocuments.String,
	}, nil
}

func (r *CashRepo) SaveCashPayment(ctx context.Context, cp *cash.CashPayment) error {
	if cp == nil {
		return errors.New("cash payment cannot be nil")
	}

	receiverAddr := sql.NullString{Valid: false}
	if cp.ReceiverAddress != "" {
		receiverAddr = sql.NullString{String: cp.ReceiverAddress, Valid: true}
	}

	overrideReason := sql.NullString{Valid: false}
	if cp.OverrideReason != "" {
		overrideReason = sql.NullString{String: cp.OverrideReason, Valid: true}
	}

	accompanyingDocs := sql.NullString{Valid: false}
	if cp.AccompanyingDocuments != "" {
		accompanyingDocs = sql.NullString{String: cp.AccompanyingDocuments, Valid: true}
	}

	err := r.queries.CreateCashPayment(ctx, sqlc.CreateCashPaymentParams{
		ID:                    cp.ID,
		VoucherID:             cp.VoucherID,
		CompanyProfileID:      cp.CompanyProfileID,
		ReceiverName:          cp.ReceiverName,
		ReceiverAddress:       receiverAddr,
		Reason:                cp.Reason,
		CashAccountID:         cp.CashAccountID,
		TotalAmountVnd:        cp.TotalAmountVND.String(),
		IsNonCashOverride:     cp.IsNonCashOverride,
		OverrideReason:        overrideReason,
		AccompanyingDocuments: accompanyingDocs,
	})
	if err != nil {
		return fmt.Errorf("failed to save cash payment: %w", err)
	}

	return nil
}

func (r *CashRepo) GetCashPaymentByID(ctx context.Context, id string) (*cash.CashPayment, error) {
	row, err := r.queries.GetCashPaymentByID(ctx, id)
	if err != nil {
		return nil, err
	}

	amount, err := decimal.NewFromString(row.TotalAmountVnd)
	if err != nil {
		return nil, fmt.Errorf("invalid cash payment amount '%s': %w", row.TotalAmountVnd, err)
	}

	return &cash.CashPayment{
		ID:                    row.ID,
		VoucherID:             row.VoucherID,
		CompanyProfileID:      row.CompanyProfileID,
		ReceiverName:          row.ReceiverName,
		ReceiverAddress:       row.ReceiverAddress.String,
		Reason:                row.Reason,
		CashAccountID:         row.CashAccountID,
		TotalAmountVND:        amount,
		IsNonCashOverride:     row.IsNonCashOverride,
		OverrideReason:        row.OverrideReason.String,
		AccompanyingDocuments: row.AccompanyingDocuments.String,
	}, nil
}

func (r *CashRepo) GetCashPaymentByVoucherID(ctx context.Context, voucherID string) (*cash.CashPayment, error) {
	row, err := r.queries.GetCashPaymentByVoucherID(ctx, voucherID)
	if err != nil {
		return nil, err
	}

	amount, err := decimal.NewFromString(row.TotalAmountVnd)
	if err != nil {
		return nil, fmt.Errorf("invalid cash payment amount '%s': %w", row.TotalAmountVnd, err)
	}

	return &cash.CashPayment{
		ID:                    row.ID,
		VoucherID:             row.VoucherID,
		CompanyProfileID:      row.CompanyProfileID,
		ReceiverName:          row.ReceiverName,
		ReceiverAddress:       row.ReceiverAddress.String,
		Reason:                row.Reason,
		CashAccountID:         row.CashAccountID,
		TotalAmountVND:        amount,
		IsNonCashOverride:     row.IsNonCashOverride,
		OverrideReason:        row.OverrideReason.String,
		AccompanyingDocuments: row.AccompanyingDocuments.String,
	}, nil
}

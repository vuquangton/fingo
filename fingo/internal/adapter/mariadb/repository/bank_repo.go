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

type BankRepo struct {
	db      *sql.DB
	queries *sqlc.Queries
}

func NewBankRepo(db *sql.DB) *BankRepo {
	return &BankRepo{
		db:      db,
		queries: sqlc.New(db),
	}
}

func (r *BankRepo) SaveBankTransaction(ctx context.Context, bt *cash.BankTransaction) error {
	if bt == nil {
		return errors.New("bank transaction cannot be nil")
	}

	accNo := sql.NullString{Valid: false}
	if bt.Counterparty.AccountNo != "" {
		accNo = sql.NullString{String: bt.Counterparty.AccountNo, Valid: true}
	}

	bankName := sql.NullString{Valid: false}
	if bt.Counterparty.BankName != "" {
		bankName = sql.NullString{String: bt.Counterparty.BankName, Valid: true}
	}

	cpName := sql.NullString{Valid: false}
	if bt.Counterparty.Name != "" {
		cpName = sql.NullString{String: bt.Counterparty.Name, Valid: true}
	}

	refNo := sql.NullString{Valid: false}
	if bt.Counterparty.Reference != "" {
		refNo = sql.NullString{String: bt.Counterparty.Reference, Valid: true}
	}

	err := r.queries.CreateBankTransaction(ctx, sqlc.CreateBankTransactionParams{
		ID:                    bt.ID,
		VoucherID:             bt.VoucherID,
		CompanyProfileID:      bt.CompanyProfileID,
		BankAccountID:         bt.BankAccountID,
		TransactionType:       sqlc.BankTransactionsTransactionType(bt.TransactionType),
		CounterpartyAccountNo: accNo,
		CounterpartyBankName:  bankName,
		CounterpartyName:      cpName,
		BankReferenceNo:       refNo,
		FeeAmountVnd:          bt.FeeAmountVND.String(),
		VatFeeAmountVnd:       bt.VatFeeAmountVND.String(),
		TotalAmountVnd:        bt.TotalAmountVND.String(),
	})
	if err != nil {
		return fmt.Errorf("failed to save bank transaction: %w", err)
	}

	return nil
}

func (r *BankRepo) GetBankTransactionByID(ctx context.Context, id string) (*cash.BankTransaction, error) {
	row, err := r.queries.GetBankTransactionByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return mapRowToBankTransaction(
		row.ID,
		row.VoucherID,
		row.CompanyProfileID,
		row.BankAccountID,
		string(row.TransactionType),
		row.CounterpartyAccountNo.String,
		row.CounterpartyBankName.String,
		row.CounterpartyName.String,
		row.BankReferenceNo.String,
		row.FeeAmountVnd,
		row.VatFeeAmountVnd,
		row.TotalAmountVnd,
	)
}

func (r *BankRepo) GetBankTransactionByVoucherID(ctx context.Context, voucherID string) (*cash.BankTransaction, error) {
	row, err := r.queries.GetBankTransactionByVoucherID(ctx, voucherID)
	if err != nil {
		return nil, err
	}
	return mapRowToBankTransaction(
		row.ID,
		row.VoucherID,
		row.CompanyProfileID,
		row.BankAccountID,
		string(row.TransactionType),
		row.CounterpartyAccountNo.String,
		row.CounterpartyBankName.String,
		row.CounterpartyName.String,
		row.BankReferenceNo.String,
		row.FeeAmountVnd,
		row.VatFeeAmountVnd,
		row.TotalAmountVnd,
	)
}

func mapRowToBankTransaction(
	id, voucherID, companyProfileID, bankAccountID, txType string,
	accNo, bankName, cpName, refNo string,
	feeStr, vatFeeStr, totalStr string,
) (*cash.BankTransaction, error) {
	fee, err := decimal.NewFromString(feeStr)
	if err != nil {
		return nil, fmt.Errorf("invalid fee amount '%s': %w", feeStr, err)
	}

	vatFee, err := decimal.NewFromString(vatFeeStr)
	if err != nil {
		return nil, fmt.Errorf("invalid vat fee amount '%s': %w", vatFeeStr, err)
	}

	total, err := decimal.NewFromString(totalStr)
	if err != nil {
		return nil, fmt.Errorf("invalid total amount '%s': %w", totalStr, err)
	}

	return &cash.BankTransaction{
		ID:               id,
		VoucherID:        voucherID,
		CompanyProfileID: companyProfileID,
		BankAccountID:    bankAccountID,
		TransactionType:  cash.BankTransactionType(txType),
		Counterparty: cash.BankCounterpartyInfo{
			AccountNo: accNo,
			BankName:  bankName,
			Name:      cpName,
			Reference: refNo,
		},
		FeeAmountVND:    fee,
		VatFeeAmountVND: vatFee,
		TotalAmountVND:  total,
	}, nil
}

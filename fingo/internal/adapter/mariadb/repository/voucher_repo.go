package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	mariadb "fingo/internal/adapter/mariadb/sqlc"
	"fingo/internal/domain/gl"
)

type VoucherRepo struct {
	db *sql.DB
	q  *mariadb.Queries
}

func NewVoucherRepo(db *sql.DB) *VoucherRepo {
	return &VoucherRepo{
		db: db,
		q:  mariadb.New(db),
	}
}

func (r *VoucherRepo) SaveVoucher(ctx context.Context, v *gl.Voucher) error {
	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	qtx := r.q.WithTx(tx)

	if v.ID == "" {
		v.ID = uuid.New().String()
	}

	rate := v.ExchangeRate
	if rate.IsZero() {
		rate = decimal.NewFromInt(1)
	}

	var branchID, srcID, srcType, idempKey sql.NullString
	if v.BranchID != nil {
		branchID = sql.NullString{String: *v.BranchID, Valid: true}
	}
	if v.SourceDocumentID != nil {
		srcID = sql.NullString{String: *v.SourceDocumentID, Valid: true}
	}
	if v.SourceDocumentType != nil {
		srcType = sql.NullString{String: *v.SourceDocumentType, Valid: true}
	}
	if v.IdempotencyKey != nil {
		idempKey = sql.NullString{String: *v.IdempotencyKey, Valid: true}
	}

	now := time.Now()

	// Check if exists
	existing, err := qtx.GetVoucherByID(ctx, v.ID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			// Insert new voucher
			params := mariadb.CreateVoucherParams{
				ID:                 v.ID,
				CompanyProfileID:   v.CompanyProfileID,
				BranchID:           branchID,
				VoucherNo:          v.VoucherNo,
				VoucherDate:        v.VoucherDate,
				PostedDate:         v.PostedDate,
				VoucherType:        mariadb.VouchersVoucherType(v.VoucherType),
				Description:        v.Description,
				Status:             mariadb.VouchersStatus(v.Status),
				TotalDebit:         v.TotalDebit.StringFixed(0),
				TotalCredit:        v.TotalCredit.StringFixed(0),
				CurrencyCode:       v.CurrencyCode,
				ExchangeRate:       rate.StringFixed(6),
				SourceDocumentID:   srcID,
				SourceDocumentType: srcType,
				IdempotencyKey:     idempKey,
				CreatedBy:          v.CreatedBy,
				CreatedAt:          now,
				UpdatedAt:          now,
			}
			if err := qtx.CreateVoucher(ctx, params); err != nil {
				return fmt.Errorf("failed to create voucher: %w", err)
			}
		} else {
			return fmt.Errorf("failed to check existing voucher: %w", err)
		}
	} else {
		// Immutability Guard: Cannot overwrite posted or cancelled vouchers
		if existing.Status == mariadb.VouchersStatusPOSTED || existing.Status == mariadb.VouchersStatusCANCELLED {
			return gl.ErrVoucherAlreadyPosted
		}

		// Existing draft voucher: update totals & status, delete previous lines
		if err := qtx.UpdateVoucherTotals(ctx, mariadb.UpdateVoucherTotalsParams{
			ID:          v.ID,
			TotalDebit:  v.TotalDebit.StringFixed(0),
			TotalCredit: v.TotalCredit.StringFixed(0),
			UpdatedAt:   now,
		}); err != nil {
			return fmt.Errorf("failed to update voucher totals: %w", err)
		}
		if err := qtx.UpdateVoucherStatus(ctx, mariadb.UpdateVoucherStatusParams{
			ID:        v.ID,
			Status:    mariadb.VouchersStatus(v.Status),
			UpdatedAt: now,
		}); err != nil {
			return fmt.Errorf("failed to update voucher status: %w", err)
		}
		if err := qtx.DeleteVoucherLinesByVoucherID(ctx, v.ID); err != nil {
			return fmt.Errorf("failed to delete previous voucher lines: %w", err)
		}
	}

	// Insert child lines
	for i, line := range v.Lines {
		lineID := line.ID
		if lineID == "" {
			lineID = uuid.New().String()
		}

		var custID, vendID, empID, itemID, whID, ccID, expID, invNo sql.NullString
		var invDate sql.NullTime

		if line.CustomerID != nil {
			custID = sql.NullString{String: *line.CustomerID, Valid: true}
		}
		if line.VendorID != nil {
			vendID = sql.NullString{String: *line.VendorID, Valid: true}
		}
		if line.EmployeeID != nil {
			empID = sql.NullString{String: *line.EmployeeID, Valid: true}
		}
		if line.ItemID != nil {
			itemID = sql.NullString{String: *line.ItemID, Valid: true}
		}
		if line.WarehouseID != nil {
			whID = sql.NullString{String: *line.WarehouseID, Valid: true}
		}
		if line.CostCenterID != nil {
			ccID = sql.NullString{String: *line.CostCenterID, Valid: true}
		}
		if line.ExpenseItemID != nil {
			expID = sql.NullString{String: *line.ExpenseItemID, Valid: true}
		}
		if line.InvoiceNo != nil {
			invNo = sql.NullString{String: *line.InvoiceNo, Valid: true}
		}
		if line.InvoiceDate != nil {
			invDate = sql.NullTime{Time: *line.InvoiceDate, Valid: true}
		}

		lineParams := mariadb.InsertVoucherLineParams{
			ID:                lineID,
			VoucherID:         v.ID,
			LineOrder:         int32(i + 1),
			DebitAccountID:    line.DebitAccountID,
			CreditAccountID:   line.CreditAccountID,
			DebitAccountCode:  line.DebitAccountCode,
			CreditAccountCode: line.CreditAccountCode,
			AmountFc:          line.AmountFC.StringFixed(2),
			AmountVnd:         line.AmountVND.StringFixed(0),
			Note:              sql.NullString{String: line.Note, Valid: line.Note != ""},
			CustomerID:        custID,
			VendorID:          vendID,
			EmployeeID:        empID,
			ItemID:            itemID,
			WarehouseID:       whID,
			CostCenterID:      ccID,
			ExpenseItemID:     expID,
			InvoiceNo:         invNo,
			InvoiceDate:       invDate,
			CreatedAt:         now,
		}

		if err := qtx.InsertVoucherLine(ctx, lineParams); err != nil {
			return fmt.Errorf("failed to insert voucher line %d: %w", i+1, err)
		}
	}

	return tx.Commit()
}

func (r *VoucherRepo) GetVoucherByID(ctx context.Context, id string) (*gl.Voucher, error) {
	row, err := r.q.GetVoucherByID(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, sql.ErrNoRows
		}
		return nil, fmt.Errorf("failed to query voucher: %w", err)
	}

	return r.assembleVoucher(ctx, row)
}

func (r *VoucherRepo) GetVoucherByNo(ctx context.Context, companyID string, vType gl.VoucherType, voucherNo string) (*gl.Voucher, error) {
	row, err := r.q.GetVoucherByNo(ctx, mariadb.GetVoucherByNoParams{
		CompanyProfileID: companyID,
		VoucherType:      mariadb.VouchersVoucherType(vType),
		VoucherNo:        voucherNo,
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, sql.ErrNoRows
		}
		return nil, fmt.Errorf("failed to query voucher by number: %w", err)
	}

	return r.assembleVoucher(ctx, row)
}

func (r *VoucherRepo) UpdateVoucherStatus(ctx context.Context, id string, status gl.VoucherStatus) error {
	return r.q.UpdateVoucherStatus(ctx, mariadb.UpdateVoucherStatusParams{
		ID:        id,
		Status:    mariadb.VouchersStatus(status),
		UpdatedAt: time.Now(),
	})
}

func (r *VoucherRepo) ListVouchersByPeriod(ctx context.Context, companyID string, fromDate, toDate time.Time) ([]gl.Voucher, error) {
	rows, err := r.q.ListVouchersByPeriod(ctx, mariadb.ListVouchersByPeriodParams{
		CompanyProfileID: companyID,
		VoucherDate:      fromDate,
		VoucherDate_2:    toDate,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list vouchers by period: %w", err)
	}

	result := make([]gl.Voucher, len(rows))
	for i, row := range rows {
		v, err := r.assembleVoucher(ctx, row)
		if err != nil {
			return nil, err
		}
		result[i] = *v
	}
	return result, nil
}

func (r *VoucherRepo) assembleVoucher(ctx context.Context, row mariadb.Voucher) (*gl.Voucher, error) {
	lines, err := r.q.ListVoucherLinesByVoucherID(ctx, row.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to query voucher lines: %w", err)
	}

	totalDr, _ := decimal.NewFromString(row.TotalDebit)
	totalCr, _ := decimal.NewFromString(row.TotalCredit)
	rate, _ := decimal.NewFromString(row.ExchangeRate)

	var branchID, srcID, srcType, idempKey *string
	if row.BranchID.Valid {
		branchID = &row.BranchID.String
	}
	if row.SourceDocumentID.Valid {
		srcID = &row.SourceDocumentID.String
	}
	if row.SourceDocumentType.Valid {
		srcType = &row.SourceDocumentType.String
	}
	if row.IdempotencyKey.Valid {
		idempKey = &row.IdempotencyKey.String
	}

	domainLines := make([]gl.VoucherLine, len(lines))
	for i, l := range lines {
		amtFC, _ := decimal.NewFromString(l.AmountFc)
		amtVND, _ := decimal.NewFromString(l.AmountVnd)

		var custID, vendID, empID, itemID, whID, ccID, expID, invNo *string
		var invDate *time.Time

		if l.CustomerID.Valid {
			custID = &l.CustomerID.String
		}
		if l.VendorID.Valid {
			vendID = &l.VendorID.String
		}
		if l.EmployeeID.Valid {
			empID = &l.EmployeeID.String
		}
		if l.ItemID.Valid {
			itemID = &l.ItemID.String
		}
		if l.WarehouseID.Valid {
			whID = &l.WarehouseID.String
		}
		if l.CostCenterID.Valid {
			ccID = &l.CostCenterID.String
		}
		if l.ExpenseItemID.Valid {
			expID = &l.ExpenseItemID.String
		}
		if l.InvoiceNo.Valid {
			invNo = &l.InvoiceNo.String
		}
		if l.InvoiceDate.Valid {
			invDate = &l.InvoiceDate.Time
		}

		domainLines[i] = gl.VoucherLine{
			ID:                l.ID,
			VoucherID:         l.VoucherID,
			LineOrder:         int(l.LineOrder),
			DebitAccountID:    l.DebitAccountID,
			CreditAccountID:   l.CreditAccountID,
			DebitAccountCode:  l.DebitAccountCode,
			CreditAccountCode: l.CreditAccountCode,
			AmountFC:          amtFC,
			AmountVND:         amtVND,
			Note:              l.Note.String,
			CustomerID:        custID,
			VendorID:          vendID,
			EmployeeID:        empID,
			ItemID:            itemID,
			WarehouseID:       whID,
			CostCenterID:      ccID,
			ExpenseItemID:     expID,
			InvoiceNo:         invNo,
			InvoiceDate:       invDate,
			CreatedAt:         l.CreatedAt,
		}
	}

	return &gl.Voucher{
		ID:                 row.ID,
		CompanyProfileID:   row.CompanyProfileID,
		BranchID:           branchID,
		VoucherNo:          row.VoucherNo,
		VoucherDate:        row.VoucherDate,
		PostedDate:         row.PostedDate,
		VoucherType:        gl.VoucherType(row.VoucherType),
		Description:        row.Description,
		Status:             gl.VoucherStatus(row.Status),
		TotalDebit:         totalDr,
		TotalCredit:        totalCr,
		CurrencyCode:       row.CurrencyCode,
		ExchangeRate:       rate,
		SourceDocumentID:   srcID,
		SourceDocumentType: srcType,
		IdempotencyKey:     idempKey,
		CreatedBy:          row.CreatedBy,
		Lines:              domainLines,
		CreatedAt:          row.CreatedAt,
		UpdatedAt:          row.UpdatedAt,
	}, nil
}

func (r *VoucherRepo) GetVoucherByIdempotencyKey(ctx context.Context, companyID, idempotencyKey string) (*gl.Voucher, error) {
	row, err := r.q.GetVoucherByIdempotencyKey(ctx, mariadb.GetVoucherByIdempotencyKeyParams{
		CompanyProfileID: companyID,
		IdempotencyKey:   sql.NullString{String: idempotencyKey, Valid: true},
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, sql.ErrNoRows
		}
		return nil, fmt.Errorf("failed to query voucher by idempotency key: %w", err)
	}

	return r.assembleVoucher(ctx, row)
}

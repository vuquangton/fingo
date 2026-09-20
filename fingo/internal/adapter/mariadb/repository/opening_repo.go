package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"fingo/internal/adapter/mariadb/sqlc"
	"fingo/internal/domain/opening"
)

type OpeningRepo struct {
	db *sql.DB
	q  *mariadb.Queries
}

func NewOpeningRepo(db *sql.DB) *OpeningRepo {
	return &OpeningRepo{
		db: db,
		q:  mariadb.New(db),
	}
}

func (r *OpeningRepo) SaveOpeningBatch(ctx context.Context, b *opening.OpeningBatch) error {
	now := time.Now()
	if b.ID == "" {
		b.ID = uuid.New().String()
	}

	params := mariadb.CreateOpeningBatchParams{
		ID:               b.ID,
		CompanyProfileID: b.CompanyProfileID,
		AsOfDate:         b.AsOfDate,
		Status:           mariadb.OpeningBatchesStatus(b.Status),
		TotalDebit:       b.TotalDebit.StringFixed(2),
		TotalCredit:      b.TotalCredit.StringFixed(2),
		Notes:            sql.NullString{String: b.Notes, Valid: b.Notes != ""},
		CreatedAt:        now,
		UpdatedAt:        now,
	}

	if err := r.q.CreateOpeningBatch(ctx, params); err != nil {
		return fmt.Errorf("failed to insert opening batch: %w", err)
	}
	return nil
}

func (r *OpeningRepo) GetOpeningBatchByID(ctx context.Context, id string) (*opening.OpeningBatch, error) {
	row, err := r.q.GetOpeningBatchByID(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, sql.ErrNoRows
		}
		return nil, fmt.Errorf("failed to get opening batch by id: %w", err)
	}

	batch := toDomainOpeningBatch(row)
	if err := r.populateBatchChildren(ctx, batch); err != nil {
		return nil, err
	}
	return batch, nil
}

func (r *OpeningRepo) GetOpeningBatchByDate(ctx context.Context, companyID string, asOfDate time.Time) (*opening.OpeningBatch, error) {
	row, err := r.q.GetOpeningBatchByDate(ctx, mariadb.GetOpeningBatchByDateParams{
		CompanyProfileID: companyID,
		AsOfDate:         asOfDate,
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, sql.ErrNoRows
		}
		return nil, fmt.Errorf("failed to get opening batch by date: %w", err)
	}

	batch := toDomainOpeningBatch(row)
	if err := r.populateBatchChildren(ctx, batch); err != nil {
		return nil, err
	}
	return batch, nil
}

func (r *OpeningRepo) populateBatchChildren(ctx context.Context, b *opening.OpeningBatch) error {
	// 1. Accounts
	accRows, err := r.q.ListAccountBalancesByBatch(ctx, b.ID)
	if err != nil {
		return fmt.Errorf("failed to list account opening balances: %w", err)
	}
	b.Accounts = make([]opening.AccountOpeningBalance, len(accRows))
	for i, row := range accRows {
		debitVND, _ := decimal.NewFromString(row.DebitAmountVnd)
		creditVND, _ := decimal.NewFromString(row.CreditAmountVnd)
		debitFC, _ := decimal.NewFromString(row.DebitAmountFc)
		creditFC, _ := decimal.NewFromString(row.CreditAmountFc)
		rate, _ := decimal.NewFromString(row.ExchangeRate)

		b.Accounts[i] = opening.AccountOpeningBalance{
			ID:              row.ID,
			BatchID:         row.BatchID,
			AccountID:       row.AccountID,
			AccountCode:     row.AccountCode,
			CurrencyCode:    row.CurrencyCode,
			DebitAmountFC:   debitFC,
			CreditAmountFC:  creditFC,
			ExchangeRate:    rate,
			DebitAmountVND:  debitVND,
			CreditAmountVND: creditVND,
		}
	}

	// 2. Customers
	custRows, err := r.q.ListCustomerBalancesByBatch(ctx, b.ID)
	if err != nil {
		return fmt.Errorf("failed to list customer opening balances: %w", err)
	}
	b.Customers = make([]opening.CustomerOpeningBalance, len(custRows))
	for i, row := range custRows {
		debitVND, _ := decimal.NewFromString(row.DebitAmountVnd)
		creditVND, _ := decimal.NewFromString(row.CreditAmountVnd)
		debitFC, _ := decimal.NewFromString(row.DebitAmountFc)
		creditFC, _ := decimal.NewFromString(row.CreditAmountFc)
		rate, _ := decimal.NewFromString(row.ExchangeRate)

		var invDate, dueDate *time.Time
		if row.InvoiceDate.Valid {
			invDate = &row.InvoiceDate.Time
		}
		if row.DueDate.Valid {
			dueDate = &row.DueDate.Time
		}

		b.Customers[i] = opening.CustomerOpeningBalance{
			ID:              row.ID,
			BatchID:         row.BatchID,
			CustomerID:      row.CustomerID,
			CustomerCode:    row.CustomerCode,
			InvoiceNo:       row.InvoiceNo.String,
			InvoiceDate:     invDate,
			DueDate:         dueDate,
			CurrencyCode:    row.CurrencyCode,
			DebitAmountFC:   debitFC,
			CreditAmountFC:  creditFC,
			ExchangeRate:    rate,
			DebitAmountVND:  debitVND,
			CreditAmountVND: creditVND,
			Notes:           row.Notes.String,
		}
	}

	// 3. Vendors
	vendRows, err := r.q.ListVendorBalancesByBatch(ctx, b.ID)
	if err != nil {
		return fmt.Errorf("failed to list vendor opening balances: %w", err)
	}
	b.Vendors = make([]opening.VendorOpeningBalance, len(vendRows))
	for i, row := range vendRows {
		debitVND, _ := decimal.NewFromString(row.DebitAmountVnd)
		creditVND, _ := decimal.NewFromString(row.CreditAmountVnd)
		debitFC, _ := decimal.NewFromString(row.DebitAmountFc)
		creditFC, _ := decimal.NewFromString(row.CreditAmountFc)
		rate, _ := decimal.NewFromString(row.ExchangeRate)

		var billDate, dueDate *time.Time
		if row.BillDate.Valid {
			billDate = &row.BillDate.Time
		}
		if row.DueDate.Valid {
			dueDate = &row.DueDate.Time
		}

		b.Vendors[i] = opening.VendorOpeningBalance{
			ID:              row.ID,
			BatchID:         row.BatchID,
			VendorID:        row.VendorID,
			VendorCode:      row.VendorCode,
			BillNo:          row.BillNo.String,
			BillDate:        billDate,
			DueDate:         dueDate,
			CurrencyCode:    row.CurrencyCode,
			DebitAmountFC:   debitFC,
			CreditAmountFC:  creditFC,
			ExchangeRate:    rate,
			DebitAmountVND:  debitVND,
			CreditAmountVND: creditVND,
			Notes:           row.Notes.String,
		}
	}

	// 4. Inventory
	invRows, err := r.q.ListInventoryBalancesByBatch(ctx, b.ID)
	if err != nil {
		return fmt.Errorf("failed to list inventory opening balances: %w", err)
	}
	b.Inventory = make([]opening.InventoryOpeningBalance, len(invRows))
	for i, row := range invRows {
		qty, _ := decimal.NewFromString(row.Quantity)
		cost, _ := decimal.NewFromString(row.UnitCost)
		totalVND, _ := decimal.NewFromString(row.TotalAmountVnd)

		var expDate *time.Time
		if row.ExpiryDate.Valid {
			expDate = &row.ExpiryDate.Time
		}

		b.Inventory[i] = opening.InventoryOpeningBalance{
			ID:             row.ID,
			BatchID:        row.BatchID,
			WarehouseID:    row.WarehouseID,
			WarehouseCode:  row.WarehouseCode,
			ItemID:         row.ItemID,
			ItemCode:       row.ItemCode,
			UOMID:          row.UomID,
			Quantity:       qty,
			UnitCost:       cost,
			TotalAmountVND: totalVND,
			BatchNumber:    row.BatchNumber.String,
			ExpiryDate:     expDate,
		}
	}

	// 5. Assets
	assetRows, err := r.q.ListAssetBalancesByBatch(ctx, b.ID)
	if err != nil {
		return fmt.Errorf("failed to list asset opening balances: %w", err)
	}
	b.Assets = make([]opening.AssetOpeningBalance, len(assetRows))
	for i, row := range assetRows {
		origCost, _ := decimal.NewFromString(row.OriginalCost)
		accDepr, _ := decimal.NewFromString(row.AccumulatedDepreciation)
		nbv, _ := decimal.NewFromString(row.NetBookValue)
		mDepr, _ := decimal.NewFromString(row.MonthlyDepreciation)

		var deptID *string
		if row.DepartmentID.Valid {
			deptID = &row.DepartmentID.String
		}

		b.Assets[i] = opening.AssetOpeningBalance{
			ID:                      row.ID,
			BatchID:                 row.BatchID,
			AssetCode:               row.AssetCode,
			AssetName:               row.AssetName,
			AssetAccountID:          row.AssetAccountID,
			AssetAccountCode:        row.AssetAccountCode,
			DepreciationAccountID:   row.DepreciationAccountID,
			DepreciationAccountCode: row.DepreciationAccountCode,
			CostAccountID:           row.CostAccountID,
			CostAccountCode:         row.CostAccountCode,
			DepartmentID:            deptID,
			AcquisitionDate:         row.AcquisitionDate,
			StartDepreciationDate:   row.StartDepreciationDate,
			OriginalCost:            origCost,
			AccumulatedDepreciation: accDepr,
			NetBookValue:            nbv,
			UsefulLifeMonths:        int(row.UsefulLifeMonths),
			RemainingLifeMonths:     int(row.RemainingLifeMonths),
			MonthlyDepreciation:     mDepr,
		}
	}

	return nil
}

func (r *OpeningRepo) SaveAccountBalances(ctx context.Context, batchID string, balances []opening.AccountOpeningBalance) error {
	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	qtx := r.q.WithTx(tx)
	if err := qtx.DeleteAccountBalancesByBatch(ctx, batchID); err != nil {
		return fmt.Errorf("failed to delete previous account balances: %w", err)
	}

	now := time.Now()
	for _, b := range balances {
		id := b.ID
		if id == "" {
			id = uuid.New().String()
		}
		rate := b.ExchangeRate
		if rate.IsZero() {
			rate = decimal.RequireFromString("1")
		}

		err := qtx.InsertAccountBalance(ctx, mariadb.InsertAccountBalanceParams{
			ID:              id,
			BatchID:         batchID,
			AccountID:       b.AccountID,
			CurrencyCode:    b.CurrencyCode,
			DebitAmountFc:   b.DebitAmountFC.StringFixed(2),
			CreditAmountFc:  b.CreditAmountFC.StringFixed(2),
			ExchangeRate:    rate.StringFixed(6),
			DebitAmountVnd:  b.DebitAmountVND.StringFixed(2),
			CreditAmountVnd: b.CreditAmountVND.StringFixed(2),
			CreatedAt:       now,
			UpdatedAt:       now,
		})
		if err != nil {
			return fmt.Errorf("failed to insert account balance for %s: %w", b.AccountCode, err)
		}
	}

	totalDebit := decimal.Zero
	totalCredit := decimal.Zero
	for _, b := range balances {
		totalDebit = totalDebit.Add(b.DebitAmountVND)
		totalCredit = totalCredit.Add(b.CreditAmountVND)
	}

	if err := qtx.UpdateOpeningBatchTotals(ctx, mariadb.UpdateOpeningBatchTotalsParams{
		ID:          batchID,
		TotalDebit:  totalDebit.StringFixed(2),
		TotalCredit: totalCredit.StringFixed(2),
	}); err != nil {
		return fmt.Errorf("failed to update opening batch totals: %w", err)
	}

	return tx.Commit()
}

func (r *OpeningRepo) SaveCustomerBalances(ctx context.Context, batchID string, balances []opening.CustomerOpeningBalance) error {
	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	qtx := r.q.WithTx(tx)
	if err := qtx.DeleteCustomerBalancesByBatch(ctx, batchID); err != nil {
		return fmt.Errorf("failed to delete previous customer balances: %w", err)
	}

	now := time.Now()
	for _, b := range balances {
		id := b.ID
		if id == "" {
			id = uuid.New().String()
		}
		rate := b.ExchangeRate
		if rate.IsZero() {
			rate = decimal.RequireFromString("1")
		}

		var invDate, dueDate sql.NullTime
		if b.InvoiceDate != nil {
			invDate = sql.NullTime{Time: *b.InvoiceDate, Valid: true}
		}
		if b.DueDate != nil {
			dueDate = sql.NullTime{Time: *b.DueDate, Valid: true}
		}

		err := qtx.InsertCustomerBalance(ctx, mariadb.InsertCustomerBalanceParams{
			ID:              id,
			BatchID:         batchID,
			CustomerID:      b.CustomerID,
			InvoiceNo:       sql.NullString{String: b.InvoiceNo, Valid: b.InvoiceNo != ""},
			InvoiceDate:     invDate,
			DueDate:         dueDate,
			CurrencyCode:    b.CurrencyCode,
			DebitAmountFc:   b.DebitAmountFC.StringFixed(2),
			CreditAmountFc:  b.CreditAmountFC.StringFixed(2),
			ExchangeRate:    rate.StringFixed(6),
			DebitAmountVnd:  b.DebitAmountVND.StringFixed(2),
			CreditAmountVnd: b.CreditAmountVND.StringFixed(2),
			Notes:           sql.NullString{String: b.Notes, Valid: b.Notes != ""},
			CreatedAt:       now,
			UpdatedAt:       now,
		})
		if err != nil {
			return fmt.Errorf("failed to insert customer balance: %w", err)
		}
	}

	return tx.Commit()
}

func (r *OpeningRepo) SaveVendorBalances(ctx context.Context, batchID string, balances []opening.VendorOpeningBalance) error {
	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	qtx := r.q.WithTx(tx)
	if err := qtx.DeleteVendorBalancesByBatch(ctx, batchID); err != nil {
		return fmt.Errorf("failed to delete previous vendor balances: %w", err)
	}

	now := time.Now()
	for _, b := range balances {
		id := b.ID
		if id == "" {
			id = uuid.New().String()
		}
		rate := b.ExchangeRate
		if rate.IsZero() {
			rate = decimal.RequireFromString("1")
		}

		var billDate, dueDate sql.NullTime
		if b.BillDate != nil {
			billDate = sql.NullTime{Time: *b.BillDate, Valid: true}
		}
		if b.DueDate != nil {
			dueDate = sql.NullTime{Time: *b.DueDate, Valid: true}
		}

		err := qtx.InsertVendorBalance(ctx, mariadb.InsertVendorBalanceParams{
			ID:              id,
			BatchID:         batchID,
			VendorID:        b.VendorID,
			BillNo:          sql.NullString{String: b.BillNo, Valid: b.BillNo != ""},
			BillDate:        billDate,
			DueDate:         dueDate,
			CurrencyCode:    b.CurrencyCode,
			DebitAmountFc:   b.DebitAmountFC.StringFixed(2),
			CreditAmountFc:  b.CreditAmountFC.StringFixed(2),
			ExchangeRate:    rate.StringFixed(6),
			DebitAmountVnd:  b.DebitAmountVND.StringFixed(2),
			CreditAmountVnd: b.CreditAmountVND.StringFixed(2),
			Notes:           sql.NullString{String: b.Notes, Valid: b.Notes != ""},
			CreatedAt:       now,
			UpdatedAt:       now,
		})
		if err != nil {
			return fmt.Errorf("failed to insert vendor balance: %w", err)
		}
	}

	return tx.Commit()
}

func (r *OpeningRepo) SaveInventoryBalances(ctx context.Context, batchID string, balances []opening.InventoryOpeningBalance) error {
	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	qtx := r.q.WithTx(tx)
	if err := qtx.DeleteInventoryBalancesByBatch(ctx, batchID); err != nil {
		return fmt.Errorf("failed to delete previous inventory balances: %w", err)
	}

	now := time.Now()
	for _, b := range balances {
		id := b.ID
		if id == "" {
			id = uuid.New().String()
		}
		var expDate sql.NullTime
		if b.ExpiryDate != nil {
			expDate = sql.NullTime{Time: *b.ExpiryDate, Valid: true}
		}

		err := qtx.InsertInventoryBalance(ctx, mariadb.InsertInventoryBalanceParams{
			ID:             id,
			BatchID:        batchID,
			WarehouseID:    b.WarehouseID,
			ItemID:         b.ItemID,
			UomID:          b.UOMID,
			Quantity:       b.Quantity.StringFixed(4),
			UnitCost:       b.UnitCost.StringFixed(4),
			TotalAmountVnd: b.TotalAmountVND.StringFixed(2),
			BatchNumber:    sql.NullString{String: b.BatchNumber, Valid: b.BatchNumber != ""},
			ExpiryDate:     expDate,
			CreatedAt:      now,
			UpdatedAt:      now,
		})
		if err != nil {
			return fmt.Errorf("failed to insert inventory balance for item %s: %w", b.ItemCode, err)
		}
	}

	return tx.Commit()
}

func (r *OpeningRepo) SaveAssetBalances(ctx context.Context, batchID string, balances []opening.AssetOpeningBalance) error {
	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	qtx := r.q.WithTx(tx)
	if err := qtx.DeleteAssetBalancesByBatch(ctx, batchID); err != nil {
		return fmt.Errorf("failed to delete previous asset balances: %w", err)
	}

	now := time.Now()
	for _, b := range balances {
		id := b.ID
		if id == "" {
			id = uuid.New().String()
		}
		var deptID sql.NullString
		if b.DepartmentID != nil {
			deptID = sql.NullString{String: *b.DepartmentID, Valid: true}
		}

		err := qtx.InsertAssetBalance(ctx, mariadb.InsertAssetBalanceParams{
			ID:                      id,
			BatchID:                 batchID,
			AssetCode:               b.AssetCode,
			AssetName:               b.AssetName,
			AssetAccountID:          b.AssetAccountID,
			DepreciationAccountID:   b.DepreciationAccountID,
			CostAccountID:           b.CostAccountID,
			DepartmentID:            deptID,
			AcquisitionDate:         b.AcquisitionDate,
			StartDepreciationDate:   b.StartDepreciationDate,
			OriginalCost:            b.OriginalCost.StringFixed(2),
			AccumulatedDepreciation: b.AccumulatedDepreciation.StringFixed(2),
			NetBookValue:            b.NetBookValue.StringFixed(2),
			UsefulLifeMonths:        int32(b.UsefulLifeMonths),
			RemainingLifeMonths:     int32(b.RemainingLifeMonths),
			MonthlyDepreciation:     b.MonthlyDepreciation.StringFixed(2),
			CreatedAt:               now,
			UpdatedAt:               now,
		})
		if err != nil {
			return fmt.Errorf("failed to insert asset balance for %s: %w", b.AssetCode, err)
		}
	}

	return tx.Commit()
}

func (r *OpeningRepo) UpdateBatchStatus(ctx context.Context, batchID string, status opening.BatchStatus, committedBy *string, committedAt *time.Time) error {
	var cBy sql.NullString
	if committedBy != nil {
		cBy = sql.NullString{String: *committedBy, Valid: true}
	}
	var cAt sql.NullTime
	if committedAt != nil {
		cAt = sql.NullTime{Time: *committedAt, Valid: true}
	}

	now := time.Now()
	err := r.q.UpdateOpeningBatchStatus(ctx, mariadb.UpdateOpeningBatchStatusParams{
		Status:      mariadb.OpeningBatchesStatus(status),
		CommittedAt: cAt,
		CommittedBy: cBy,
		UpdatedAt:   now,
		ID:          batchID,
	})
	if err != nil {
		return fmt.Errorf("failed to update batch status: %w", err)
	}
	return nil
}

func toDomainOpeningBatch(row mariadb.OpeningBatch) *opening.OpeningBatch {
	totalDr, _ := decimal.NewFromString(row.TotalDebit)
	totalCr, _ := decimal.NewFromString(row.TotalCredit)

	var cBy *string
	if row.CommittedBy.Valid {
		cBy = &row.CommittedBy.String
	}
	var cAt *time.Time
	if row.CommittedAt.Valid {
		cAt = &row.CommittedAt.Time
	}

	return &opening.OpeningBatch{
		ID:               row.ID,
		CompanyProfileID: row.CompanyProfileID,
		AsOfDate:         row.AsOfDate,
		Status:           opening.BatchStatus(row.Status),
		TotalDebit:       totalDr,
		TotalCredit:      totalCr,
		CommittedAt:      cAt,
		CommittedBy:      cBy,
		Notes:            row.Notes.String,
		CreatedAt:        row.CreatedAt,
		UpdatedAt:        row.UpdatedAt,
		Accounts:         make([]opening.AccountOpeningBalance, 0),
		Customers:        make([]opening.CustomerOpeningBalance, 0),
		Vendors:          make([]opening.VendorOpeningBalance, 0),
		Inventory:        make([]opening.InventoryOpeningBalance, 0),
		Assets:           make([]opening.AssetOpeningBalance, 0),
	}
}

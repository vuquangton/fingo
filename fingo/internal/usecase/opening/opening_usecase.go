package opening

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	catalogDomain "fingo/internal/domain/catalog"
	domain "fingo/internal/domain/opening"
	spineDomain "fingo/internal/domain/spine"
	"fingo/pkg/logger"
)

var (
	ErrBatchNotFound         = errors.New("opening batch not found")
	ErrBatchAlreadyExists    = errors.New("opening batch already exists for specified company and date")
	ErrAccountNotFound       = errors.New("account not found in chart of accounts")
	ErrCustomerNotFound      = errors.New("customer not found in catalog")
	ErrVendorNotFound        = errors.New("vendor not found in catalog")
	ErrWarehouseNotFound     = errors.New("warehouse not found in catalog")
	ErrItemNotFound          = errors.New("item not found in catalog")
	ErrNonLeafPostingAccount = errors.New("opening balance can only be posted to active leaf accounts")
)

type CreateOpeningBatchCommand struct {
	IdempotencyKey   string    `json:"idempotency_key,omitempty"`
	CompanyProfileID string    `json:"company_profile_id"`
	AsOfDate         time.Time `json:"as_of_date"`
	Notes            string    `json:"notes,omitempty"`
}

type AccountBalanceItem struct {
	AccountID      string          `json:"account_id"`
	CurrencyCode   string          `json:"currency_code"`
	DebitAmountFC  decimal.Decimal `json:"debit_amount_fc"`
	CreditAmountFC decimal.Decimal `json:"credit_amount_fc"`
	ExchangeRate   decimal.Decimal `json:"exchange_rate"`
	DebitAmountVND decimal.Decimal `json:"debit_amount_vnd"`
	CreditAmountVND decimal.Decimal `json:"credit_amount_vnd"`
}

type SaveAccountBalancesCommand struct {
	BatchID  string               `json:"batch_id"`
	Balances []AccountBalanceItem `json:"balances"`
}

type CustomerBalanceItem struct {
	CustomerID      string          `json:"customer_id"`
	InvoiceNo       string          `json:"invoice_no,omitempty"`
	InvoiceDate     *time.Time      `json:"invoice_date,omitempty"`
	DueDate         *time.Time      `json:"due_date,omitempty"`
	CurrencyCode    string          `json:"currency_code"`
	DebitAmountFC   decimal.Decimal `json:"debit_amount_fc"`
	CreditAmountFC  decimal.Decimal `json:"credit_amount_fc"`
	ExchangeRate    decimal.Decimal `json:"exchange_rate"`
	DebitAmountVND  decimal.Decimal `json:"debit_amount_vnd"`
	CreditAmountVND decimal.Decimal `json:"credit_amount_vnd"`
	Notes           string          `json:"notes,omitempty"`
}

type SaveCustomerBalancesCommand struct {
	BatchID  string                `json:"batch_id"`
	Balances []CustomerBalanceItem `json:"balances"`
}

type VendorBalanceItem struct {
	VendorID        string          `json:"vendor_id"`
	BillNo          string          `json:"bill_no,omitempty"`
	BillDate        *time.Time      `json:"bill_date,omitempty"`
	DueDate         *time.Time      `json:"due_date,omitempty"`
	CurrencyCode    string          `json:"currency_code"`
	DebitAmountFC   decimal.Decimal `json:"debit_amount_fc"`
	CreditAmountFC  decimal.Decimal `json:"credit_amount_fc"`
	ExchangeRate    decimal.Decimal `json:"exchange_rate"`
	DebitAmountVND  decimal.Decimal `json:"debit_amount_vnd"`
	CreditAmountVND decimal.Decimal `json:"credit_amount_vnd"`
	Notes           string          `json:"notes,omitempty"`
}

type SaveVendorBalancesCommand struct {
	BatchID  string              `json:"batch_id"`
	Balances []VendorBalanceItem `json:"balances"`
}

type InventoryBalanceItem struct {
	WarehouseID string          `json:"warehouse_id"`
	ItemID      string          `json:"item_id"`
	UOMID       string          `json:"uom_id"`
	Quantity    decimal.Decimal `json:"quantity"`
	UnitCost    decimal.Decimal `json:"unit_cost"`
	BatchNumber string          `json:"batch_number,omitempty"`
	ExpiryDate  *time.Time      `json:"expiry_date,omitempty"`
}

type SaveInventoryBalancesCommand struct {
	BatchID  string                 `json:"batch_id"`
	Balances []InventoryBalanceItem `json:"balances"`
}

type AssetBalanceItem struct {
	AssetCode               string          `json:"asset_code"`
	AssetName               string          `json:"asset_name"`
	AssetAccountID          string          `json:"asset_account_id"`
	DepreciationAccountID   string          `json:"depreciation_account_id"`
	CostAccountID           string          `json:"cost_account_id"`
	DepartmentID            *string         `json:"department_id,omitempty"`
	AcquisitionDate         time.Time       `json:"acquisition_date"`
	StartDepreciationDate   time.Time       `json:"start_depreciation_date"`
	OriginalCost            decimal.Decimal `json:"original_cost"`
	AccumulatedDepreciation decimal.Decimal `json:"accumulated_depreciation"`
	UsefulLifeMonths        int             `json:"useful_life_months"`
	RemainingLifeMonths     int             `json:"remaining_life_months"`
	MonthlyDepreciation     decimal.Decimal `json:"monthly_depreciation"`
}

type SaveAssetBalancesCommand struct {
	BatchID  string             `json:"batch_id"`
	Balances []AssetBalanceItem `json:"balances"`
}

type CommitOpeningBatchCommand struct {
	BatchID     string `json:"batch_id"`
	CommittedBy string `json:"committed_by"`
}

type OpeningUseCase struct {
	openingRepo domain.OpeningRepository
	accRepo     spineDomain.AccountRepository
	custRepo    catalogDomain.CustomerRepository
	vendorRepo  catalogDomain.VendorRepository
	whRepo      catalogDomain.WarehouseRepository
	itemRepo    catalogDomain.ItemRepository
	logger      *logger.Logger
}

func NewOpeningUseCase(
	openingRepo domain.OpeningRepository,
	accRepo spineDomain.AccountRepository,
	custRepo catalogDomain.CustomerRepository,
	vendorRepo catalogDomain.VendorRepository,
	whRepo catalogDomain.WarehouseRepository,
	itemRepo catalogDomain.ItemRepository,
	appLogger *logger.Logger,
) *OpeningUseCase {
	if appLogger == nil {
		appLogger = logger.New(logger.Config{Level: slog.LevelInfo, Format: logger.FormatJSON})
	}
	return &OpeningUseCase{
		openingRepo: openingRepo,
		accRepo:     accRepo,
		custRepo:    custRepo,
		vendorRepo:  vendorRepo,
		whRepo:      whRepo,
		itemRepo:    itemRepo,
		logger:      appLogger,
	}
}

// CreateOpeningBatch creates a new draft opening balance cutover batch
func (u *OpeningUseCase) CreateOpeningBatch(ctx context.Context, cmd CreateOpeningBatchCommand) (*domain.OpeningBatch, error) {
	if existing, err := u.openingRepo.GetOpeningBatchByDate(ctx, cmd.CompanyProfileID, cmd.AsOfDate); err == nil && existing != nil {
		u.logger.Info(ctx, "opening batch already exists (idempotent submission)",
			slog.String("company_id", cmd.CompanyProfileID),
			slog.String("batch_id", existing.ID),
		)
		return existing, nil
	}

	id := uuid.New().String()
	batch := domain.NewOpeningBatch(id, cmd.CompanyProfileID, cmd.AsOfDate, cmd.Notes)

	if err := u.openingRepo.SaveOpeningBatch(ctx, batch); err != nil {
		return nil, fmt.Errorf("failed to save opening batch: %w", err)
	}

	u.logger.Info(ctx, "opening batch created successfully",
		slog.String("company_id", cmd.CompanyProfileID),
		slog.String("batch_id", id),
	)
	return batch, nil
}

func (u *OpeningUseCase) assertEditableBatch(ctx context.Context, batchID string) (*domain.OpeningBatch, error) {
	batch, err := u.openingRepo.GetOpeningBatchByID(ctx, batchID)
	if err != nil {
		return nil, fmt.Errorf("%w: %s", ErrBatchNotFound, batchID)
	}
	if batch.Status == domain.BatchStatusCommitted || batch.Status == domain.BatchStatusLocked {
		return nil, domain.ErrBatchAlreadyCommitted
	}
	return batch, nil
}

// SaveAccountBalances validates leaf COA status and saves general ledger balances
func (u *OpeningUseCase) SaveAccountBalances(ctx context.Context, cmd SaveAccountBalancesCommand) error {
	if _, err := u.assertEditableBatch(ctx, cmd.BatchID); err != nil {
		return err
	}

	domainBalances := make([]domain.AccountOpeningBalance, len(cmd.Balances))
	for i, item := range cmd.Balances {
		accCode := ""
		if u.accRepo != nil {
			acc, err := u.accRepo.GetAccountByID(ctx, item.AccountID)
			if err != nil {
				return fmt.Errorf("%w: account ID %s", ErrAccountNotFound, item.AccountID)
			}
			if err := spineDomain.ValidatePostingAccount(acc); err != nil {
				return fmt.Errorf("%w: %w", ErrNonLeafPostingAccount, err)
			}
			accCode = acc.Code
		}

		domainBalances[i] = domain.AccountOpeningBalance{
			ID:              uuid.New().String(),
			BatchID:         cmd.BatchID,
			AccountID:       item.AccountID,
			AccountCode:     accCode,
			CurrencyCode:    item.CurrencyCode,
			DebitAmountFC:   item.DebitAmountFC,
			CreditAmountFC:  item.CreditAmountFC,
			ExchangeRate:    item.ExchangeRate,
			DebitAmountVND:  item.DebitAmountVND,
			CreditAmountVND: item.CreditAmountVND,
		}
	}

	tempBatch := &domain.OpeningBatch{Accounts: domainBalances}
	if err := tempBatch.ValidateEquilibrium(); err != nil {
		return err
	}

	return u.openingRepo.SaveAccountBalances(ctx, cmd.BatchID, domainBalances)
}

// SaveCustomerBalances validates customer existence and saves AR balances
func (u *OpeningUseCase) SaveCustomerBalances(ctx context.Context, cmd SaveCustomerBalancesCommand) error {
	if _, err := u.assertEditableBatch(ctx, cmd.BatchID); err != nil {
		return err
	}

	domainBalances := make([]domain.CustomerOpeningBalance, len(cmd.Balances))
	for i, item := range cmd.Balances {
		custCode := ""
		if u.custRepo != nil {
			cust, err := u.custRepo.GetCustomerByID(ctx, item.CustomerID)
			if err != nil {
				return fmt.Errorf("%w: customer ID %s", ErrCustomerNotFound, item.CustomerID)
			}
			custCode = cust.Code
		}

		domainBalances[i] = domain.CustomerOpeningBalance{
			ID:              uuid.New().String(),
			BatchID:         cmd.BatchID,
			CustomerID:      item.CustomerID,
			CustomerCode:    custCode,
			InvoiceNo:       item.InvoiceNo,
			InvoiceDate:     item.InvoiceDate,
			DueDate:         item.DueDate,
			CurrencyCode:    item.CurrencyCode,
			DebitAmountFC:   item.DebitAmountFC,
			CreditAmountFC:  item.CreditAmountFC,
			ExchangeRate:    item.ExchangeRate,
			DebitAmountVND:  item.DebitAmountVND,
			CreditAmountVND: item.CreditAmountVND,
			Notes:           item.Notes,
		}
	}

	return u.openingRepo.SaveCustomerBalances(ctx, cmd.BatchID, domainBalances)
}

// SaveVendorBalances validates vendor existence and saves AP balances
func (u *OpeningUseCase) SaveVendorBalances(ctx context.Context, cmd SaveVendorBalancesCommand) error {
	if _, err := u.assertEditableBatch(ctx, cmd.BatchID); err != nil {
		return err
	}

	domainBalances := make([]domain.VendorOpeningBalance, len(cmd.Balances))
	for i, item := range cmd.Balances {
		vendCode := ""
		if u.vendorRepo != nil {
			v, err := u.vendorRepo.GetVendorByID(ctx, item.VendorID)
			if err != nil {
				return fmt.Errorf("%w: vendor ID %s", ErrVendorNotFound, item.VendorID)
			}
			vendCode = v.Code
		}

		domainBalances[i] = domain.VendorOpeningBalance{
			ID:              uuid.New().String(),
			BatchID:         cmd.BatchID,
			VendorID:        item.VendorID,
			VendorCode:      vendCode,
			BillNo:          item.BillNo,
			BillDate:        item.BillDate,
			DueDate:         item.DueDate,
			CurrencyCode:    item.CurrencyCode,
			DebitAmountFC:   item.DebitAmountFC,
			CreditAmountFC:  item.CreditAmountFC,
			ExchangeRate:    item.ExchangeRate,
			DebitAmountVND:  item.DebitAmountVND,
			CreditAmountVND: item.CreditAmountVND,
			Notes:           item.Notes,
		}
	}

	return u.openingRepo.SaveVendorBalances(ctx, cmd.BatchID, domainBalances)
}

// SaveInventoryBalances validates warehouse/item existence and positive amounts
func (u *OpeningUseCase) SaveInventoryBalances(ctx context.Context, cmd SaveInventoryBalancesCommand) error {
	if _, err := u.assertEditableBatch(ctx, cmd.BatchID); err != nil {
		return err
	}

	domainBalances := make([]domain.InventoryOpeningBalance, len(cmd.Balances))
	for i, item := range cmd.Balances {
		if item.Quantity.LessThanOrEqual(decimal.Zero) || item.UnitCost.LessThanOrEqual(decimal.Zero) {
			return domain.ErrInvalidInventoryBalance
		}

		whCode := ""
		if u.whRepo != nil {
			wh, err := u.whRepo.GetWarehouseByID(ctx, item.WarehouseID)
			if err != nil {
				return fmt.Errorf("%w: warehouse ID %s", ErrWarehouseNotFound, item.WarehouseID)
			}
			whCode = wh.Code
		}

		itemCode := ""
		if u.itemRepo != nil {
			it, err := u.itemRepo.GetItemByID(ctx, item.ItemID)
			if err != nil {
				return fmt.Errorf("%w: item ID %s", ErrItemNotFound, item.ItemID)
			}
			itemCode = it.Code
		}

		totalAmount := item.Quantity.Mul(item.UnitCost).RoundBank(0)

		domainBalances[i] = domain.InventoryOpeningBalance{
			ID:             uuid.New().String(),
			BatchID:        cmd.BatchID,
			WarehouseID:    item.WarehouseID,
			WarehouseCode:  whCode,
			ItemID:         item.ItemID,
			ItemCode:       itemCode,
			UOMID:          item.UOMID,
			Quantity:       item.Quantity,
			UnitCost:       item.UnitCost,
			TotalAmountVND: totalAmount,
			BatchNumber:    item.BatchNumber,
			ExpiryDate:     item.ExpiryDate,
		}
	}

	return u.openingRepo.SaveInventoryBalances(ctx, cmd.BatchID, domainBalances)
}

// SaveAssetBalances validates accounts 211, 214, cost account, and asset amounts
func (u *OpeningUseCase) SaveAssetBalances(ctx context.Context, cmd SaveAssetBalancesCommand) error {
	if _, err := u.assertEditableBatch(ctx, cmd.BatchID); err != nil {
		return err
	}

	domainBalances := make([]domain.AssetOpeningBalance, len(cmd.Balances))
	for i, item := range cmd.Balances {
		if item.OriginalCost.LessThanOrEqual(decimal.Zero) || item.UsefulLifeMonths <= 0 {
			return domain.ErrInvalidAssetBalance
		}

		assetAccCode := ""
		deprAccCode := ""
		costAccCode := ""

		if u.accRepo != nil {
			assetAcc, err := u.accRepo.GetAccountByID(ctx, item.AssetAccountID)
			if err != nil || spineDomain.ValidatePostingAccount(assetAcc) != nil {
				return fmt.Errorf("%w: asset account ID %s", ErrNonLeafPostingAccount, item.AssetAccountID)
			}
			assetAccCode = assetAcc.Code

			deprAcc, err := u.accRepo.GetAccountByID(ctx, item.DepreciationAccountID)
			if err != nil || spineDomain.ValidatePostingAccount(deprAcc) != nil {
				return fmt.Errorf("%w: depreciation account ID %s", ErrNonLeafPostingAccount, item.DepreciationAccountID)
			}
			deprAccCode = deprAcc.Code

			costAcc, err := u.accRepo.GetAccountByID(ctx, item.CostAccountID)
			if err != nil || spineDomain.ValidatePostingAccount(costAcc) != nil {
				return fmt.Errorf("%w: cost account ID %s", ErrNonLeafPostingAccount, item.CostAccountID)
			}
			costAccCode = costAcc.Code
		}

		nbv := item.OriginalCost.Sub(item.AccumulatedDepreciation)
		if nbv.IsNegative() {
			return fmt.Errorf("%w: accumulated depreciation exceeds original cost for asset %s", domain.ErrInvalidAssetBalance, item.AssetCode)
		}

		remaining := item.RemainingLifeMonths
		if remaining == 0 && item.UsefulLifeMonths > 0 {
			remaining = item.UsefulLifeMonths
		}
		if remaining > item.UsefulLifeMonths || remaining < 0 {
			return domain.ErrInvalidAssetBalance
		}

		domainBalances[i] = domain.AssetOpeningBalance{
			ID:                      uuid.New().String(),
			BatchID:                 cmd.BatchID,
			AssetCode:               item.AssetCode,
			AssetName:               item.AssetName,
			AssetAccountID:          item.AssetAccountID,
			AssetAccountCode:        assetAccCode,
			DepreciationAccountID:   item.DepreciationAccountID,
			DepreciationAccountCode: deprAccCode,
			CostAccountID:           item.CostAccountID,
			CostAccountCode:         costAccCode,
			DepartmentID:            item.DepartmentID,
			AcquisitionDate:         item.AcquisitionDate,
			StartDepreciationDate:   item.StartDepreciationDate,
			OriginalCost:            item.OriginalCost,
			AccumulatedDepreciation: item.AccumulatedDepreciation,
			NetBookValue:            nbv,
			UsefulLifeMonths:        item.UsefulLifeMonths,
			RemainingLifeMonths:     remaining,
			MonthlyDepreciation:     item.MonthlyDepreciation,
		}
	}

	return u.openingRepo.SaveAssetBalances(ctx, cmd.BatchID, domainBalances)
}

// ReconcileBatch executes cross-layer verification producing reconciliation report
func (u *OpeningUseCase) ReconcileBatch(ctx context.Context, batchID string) (*domain.ReconciliationReport, error) {
	batch, err := u.openingRepo.GetOpeningBatchByID(ctx, batchID)
	if err != nil {
		return nil, fmt.Errorf("%w: %s", ErrBatchNotFound, batchID)
	}

	return batch.Reconcile()
}

// CommitOpeningBatch verifies all reconciliation rules and places immutable lock
func (u *OpeningUseCase) CommitOpeningBatch(ctx context.Context, cmd CommitOpeningBatchCommand) error {
	batch, err := u.openingRepo.GetOpeningBatchByID(ctx, cmd.BatchID)
	if err != nil {
		return fmt.Errorf("%w: %s", ErrBatchNotFound, cmd.BatchID)
	}

	if err := batch.Commit(cmd.CommittedBy); err != nil {
		return err
	}

	now := time.Now()
	err = u.openingRepo.UpdateBatchStatus(ctx, batch.ID, domain.BatchStatusCommitted, &cmd.CommittedBy, &now)
	if err != nil {
		return fmt.Errorf("failed to commit opening batch: %w", err)
	}

	u.logger.Info(ctx, "opening batch successfully committed and locked",
		slog.String("batch_id", batch.ID),
		slog.String("committed_by", cmd.CommittedBy),
	)
	return nil
}

// Helper query function
func (u *OpeningUseCase) GetOpeningBatch(ctx context.Context, batchID string) (*domain.OpeningBatch, error) {
	batch, err := u.openingRepo.GetOpeningBatchByID(ctx, batchID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrBatchNotFound
		}
		return nil, err
	}
	return batch, nil
}

package opening

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/shopspring/decimal"
)

var (
	ErrInvalidOpeningAccount         = errors.New("invalid opening account: P&L nominal accounts (Class 5-9) cannot carry opening balances")
	ErrTrialBalanceUnbalanced        = errors.New("trial balance double-entry equilibrium violated: total debit does not equal total credit")
	ErrSubledgerReconciliationFailed = errors.New("subledger reconciliation failed: subledger total does not match general ledger opening balance")
	ErrInvalidInventoryBalance       = errors.New("invalid inventory balance: quantity or unit cost cannot be negative or zero")
	ErrInvalidAssetBalance           = errors.New("invalid asset balance: original cost, depreciation, or useful life invalid")
	ErrBatchAlreadyCommitted         = errors.New("opening batch is already committed: modifications are strictly prohibited")
	ErrBatchLocked                   = errors.New("opening batch is locked by Chief Accountant")
	ErrBatchNotValidated             = errors.New("opening batch must pass all reconciliation checks before commit")
)

type BatchStatus string

const (
	BatchStatusDraft     BatchStatus = "DRAFT"
	BatchStatusValidated BatchStatus = "VALIDATED"
	BatchStatusCommitted BatchStatus = "COMMITTED"
	BatchStatusLocked    BatchStatus = "LOCKED"
)

// AccountOpeningBalance represents a General Ledger opening balance for a specific leaf account
type AccountOpeningBalance struct {
	ID              string          `json:"id"`
	BatchID         string          `json:"batch_id"`
	AccountID       string          `json:"account_id"`
	AccountCode     string          `json:"account_code"`
	CurrencyCode    string          `json:"currency_code"`
	DebitAmountFC   decimal.Decimal `json:"debit_amount_fc"`
	CreditAmountFC  decimal.Decimal `json:"credit_amount_fc"`
	ExchangeRate    decimal.Decimal `json:"exchange_rate"`
	DebitAmountVND  decimal.Decimal `json:"debit_amount_vnd"`
	CreditAmountVND decimal.Decimal `json:"credit_amount_vnd"`
}

// CustomerOpeningBalance represents an open unpaid customer invoice or prepayment (TK 131)
type CustomerOpeningBalance struct {
	ID              string          `json:"id"`
	BatchID         string          `json:"batch_id"`
	CustomerID      string          `json:"customer_id"`
	CustomerCode    string          `json:"customer_code"`
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

// VendorOpeningBalance represents an open unpaid vendor bill or prepayment (TK 331)
type VendorOpeningBalance struct {
	ID              string          `json:"id"`
	BatchID         string          `json:"batch_id"`
	VendorID        string          `json:"vendor_id"`
	VendorCode      string          `json:"vendor_code"`
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

// InventoryOpeningBalance represents opening warehouse stock for an item (TK 151-158)
type InventoryOpeningBalance struct {
	ID             string          `json:"id"`
	BatchID        string          `json:"batch_id"`
	WarehouseID    string          `json:"warehouse_id"`
	WarehouseCode  string          `json:"warehouse_code"`
	ItemID         string          `json:"item_id"`
	ItemCode       string          `json:"item_code"`
	UOMID          string          `json:"uom_id"`
	Quantity       decimal.Decimal `json:"quantity"`
	UnitCost       decimal.Decimal `json:"unit_cost"`
	TotalAmountVND decimal.Decimal `json:"total_amount_vnd"`
	BatchNumber    string          `json:"batch_number,omitempty"`
	ExpiryDate     *time.Time      `json:"expiry_date,omitempty"`
}

// AssetOpeningBalance represents an opening Fixed Asset card (TK 211 / 214) per Circular 45/2013
type AssetOpeningBalance struct {
	ID                      string          `json:"id"`
	BatchID                 string          `json:"batch_id"`
	AssetCode               string          `json:"asset_code"`
	AssetName               string          `json:"asset_name"`
	AssetAccountID          string          `json:"asset_account_id"`          // TK 211x
	AssetAccountCode        string          `json:"asset_account_code"`
	DepreciationAccountID   string          `json:"depreciation_account_id"`   // TK 214x
	DepreciationAccountCode string          `json:"depreciation_account_code"`
	CostAccountID           string          `json:"cost_account_id"`           // TK 627/641/642
	CostAccountCode         string          `json:"cost_account_code"`
	DepartmentID            *string         `json:"department_id,omitempty"`
	AcquisitionDate         time.Time       `json:"acquisition_date"`
	StartDepreciationDate   time.Time       `json:"start_depreciation_date"`
	OriginalCost            decimal.Decimal `json:"original_cost"`
	AccumulatedDepreciation decimal.Decimal `json:"accumulated_depreciation"`
	NetBookValue            decimal.Decimal `json:"net_book_value"`
	UsefulLifeMonths        int             `json:"useful_life_months"`
	RemainingLifeMonths     int             `json:"remaining_life_months"`
	MonthlyDepreciation     decimal.Decimal `json:"monthly_depreciation"`
}

// ReconciliationReport contains detailed cross-layer variance results
type ReconciliationReport struct {
	TrialBalanceBalanced bool            `json:"trial_balance_balanced"`
	TotalGLDebit         decimal.Decimal `json:"total_gl_debit"`
	TotalGLCredit        decimal.Decimal `json:"total_gl_credit"`
	TrialBalanceDiff     decimal.Decimal `json:"trial_balance_diff"`

	CustomerARReconciled bool            `json:"customer_ar_reconciled"`
	GL131Debit           decimal.Decimal `json:"gl_131_debit"`
	GL131Credit          decimal.Decimal `json:"gl_131_credit"`
	CustSubledgerDebit   decimal.Decimal `json:"cust_subledger_debit"`
	CustSubledgerCredit  decimal.Decimal `json:"cust_subledger_credit"`

	VendorAPReconciled  bool            `json:"vendor_ap_reconciled"`
	GL331Debit          decimal.Decimal `json:"gl_331_debit"`
	GL331Credit         decimal.Decimal `json:"gl_331_credit"`
	VendSubledgerDebit  decimal.Decimal `json:"vend_subledger_debit"`
	VendSubledgerCredit decimal.Decimal `json:"vend_subledger_credit"`

	InventoryReconciled bool            `json:"inventory_reconciled"`
	GLInventoryDebit    decimal.Decimal `json:"gl_inventory_debit"`
	InventorySubledger  decimal.Decimal `json:"inventory_subledger"`

	FixedAssetsReconciled bool            `json:"fixed_assets_reconciled"`
	GL211Debit            decimal.Decimal `json:"gl_211_debit"`
	AssetTotalCost        decimal.Decimal `json:"asset_total_cost"`
	GL214Credit           decimal.Decimal `json:"gl_214_credit"`
	AssetTotalDepr        decimal.Decimal `json:"asset_total_depr"`

	AllReconciled bool     `json:"all_reconciled"`
	Discrepancies []string `json:"discrepancies,omitempty"`
}

// OpeningBatch groups and manages all opening balances for a fiscal year cutover
type OpeningBatch struct {
	ID               string                    `json:"id"`
	CompanyProfileID string                    `json:"company_profile_id"`
	AsOfDate         time.Time                 `json:"as_of_date"`
	Status           BatchStatus               `json:"status"`
	TotalDebit       decimal.Decimal           `json:"total_debit"`
	TotalCredit      decimal.Decimal           `json:"total_credit"`
	CommittedAt      *time.Time                `json:"committed_at,omitempty"`
	CommittedBy      *string                   `json:"committed_by,omitempty"`
	Notes            string                    `json:"notes,omitempty"`
	CreatedAt        time.Time                 `json:"created_at"`
	UpdatedAt        time.Time                 `json:"updated_at"`
	Accounts         []AccountOpeningBalance   `json:"accounts"`
	Customers        []CustomerOpeningBalance  `json:"customers"`
	Vendors          []VendorOpeningBalance    `json:"vendors"`
	Inventory        []InventoryOpeningBalance `json:"inventory"`
	Assets           []AssetOpeningBalance     `json:"assets"`
}

func NewOpeningBatch(id, companyID string, asOfDate time.Time, notes string) *OpeningBatch {
	return &OpeningBatch{
		ID:               id,
		CompanyProfileID: companyID,
		AsOfDate:         asOfDate,
		Status:           BatchStatusDraft,
		TotalDebit:       decimal.Zero,
		TotalCredit:      decimal.Zero,
		Notes:            notes,
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
		Accounts:         make([]AccountOpeningBalance, 0),
		Customers:        make([]CustomerOpeningBalance, 0),
		Vendors:          make([]VendorOpeningBalance, 0),
		Inventory:        make([]InventoryOpeningBalance, 0),
		Assets:           make([]AssetOpeningBalance, 0),
	}
}

// ValidateEquilibrium checks INV-OPEN-01: Double-entry trial balance equilibrium & P&L zero check
func (b *OpeningBatch) ValidateEquilibrium() error {
	totalDebit := decimal.Zero
	totalCredit := decimal.Zero

	for _, acc := range b.Accounts {
		if len(acc.AccountCode) > 0 {
			firstChar := acc.AccountCode[0]
			if firstChar == '0' || (firstChar >= '5' && firstChar <= '9') {
				if !acc.DebitAmountVND.IsZero() || !acc.CreditAmountVND.IsZero() {
					return fmt.Errorf("%w: account %s", ErrInvalidOpeningAccount, acc.AccountCode)
				}
			}
		}
		totalDebit = totalDebit.Add(acc.DebitAmountVND)
		totalCredit = totalCredit.Add(acc.CreditAmountVND)
	}

	b.TotalDebit = totalDebit
	b.TotalCredit = totalCredit

	if !totalDebit.Equal(totalCredit) {
		diff := totalDebit.Sub(totalCredit).Abs()
		return fmt.Errorf("%w: total debit %s != total credit %s (variance: %s VND)",
			ErrTrialBalanceUnbalanced, totalDebit.String(), totalCredit.String(), diff.String())
	}

	return nil
}

// Reconcile performs exhaustive cross-layer verification (INV-OPEN-01 through INV-OPEN-06)
func (b *OpeningBatch) Reconcile() (*ReconciliationReport, error) {
	report := &ReconciliationReport{
		Discrepancies: make([]string, 0),
	}

	// 1. Trial Balance Equilibrium
	errEquil := b.ValidateEquilibrium()
	report.TotalGLDebit = b.TotalDebit
	report.TotalGLCredit = b.TotalCredit
	report.TrialBalanceDiff = b.TotalDebit.Sub(b.TotalCredit).Abs()
	report.TrialBalanceBalanced = errEquil == nil
	if errEquil != nil {
		report.Discrepancies = append(report.Discrepancies, errEquil.Error())
	}

	// 2. Customer AR Reconciliation (TK 131)
	custDr := decimal.Zero
	custCr := decimal.Zero
	for _, c := range b.Customers {
		custDr = custDr.Add(c.DebitAmountVND)
		custCr = custCr.Add(c.CreditAmountVND)
	}
	report.CustSubledgerDebit = custDr
	report.CustSubledgerCredit = custCr
	gl131Dr, gl131Cr := b.GetGLBalanceByPrefix("131")
	report.GL131Debit = gl131Dr
	report.GL131Credit = gl131Cr
	if custDr.Equal(gl131Dr) && custCr.Equal(gl131Cr) {
		report.CustomerARReconciled = true
	} else {
		report.CustomerARReconciled = false
		report.Discrepancies = append(report.Discrepancies, fmt.Sprintf(
			"Customer subledger (Dr: %s, Cr: %s) does not match TK 131 (Dr: %s, Cr: %s)",
			custDr, custCr, gl131Dr, gl131Cr,
		))
	}

	// 3. Vendor AP Reconciliation (TK 331)
	vendDr := decimal.Zero
	vendCr := decimal.Zero
	for _, v := range b.Vendors {
		vendDr = vendDr.Add(v.DebitAmountVND)
		vendCr = vendCr.Add(v.CreditAmountVND)
	}
	report.VendSubledgerDebit = vendDr
	report.VendSubledgerCredit = vendCr
	gl331Dr, gl331Cr := b.GetGLBalanceByPrefix("331")
	report.GL331Debit = gl331Dr
	report.GL331Credit = gl331Cr
	if vendDr.Equal(gl331Dr) && vendCr.Equal(gl331Cr) {
		report.VendorAPReconciled = true
	} else {
		report.VendorAPReconciled = false
		report.Discrepancies = append(report.Discrepancies, fmt.Sprintf(
			"Vendor subledger (Dr: %s, Cr: %s) does not match TK 331 (Dr: %s, Cr: %s)",
			vendDr, vendCr, gl331Dr, gl331Cr,
		))
	}

	// 4. Inventory Valuation Reconciliation (Group 15: 151-158)
	invTotal := decimal.Zero
	for _, item := range b.Inventory {
		if item.Quantity.LessThanOrEqual(decimal.Zero) || item.UnitCost.LessThanOrEqual(decimal.Zero) {
			report.Discrepancies = append(report.Discrepancies, fmt.Sprintf(
				"Inventory item %s in warehouse %s has invalid quantity (%s) or cost (%s)",
				item.ItemCode, item.WarehouseCode, item.Quantity, item.UnitCost,
			))
		}
		invTotal = invTotal.Add(item.TotalAmountVND)
	}
	report.InventorySubledger = invTotal
	glInvDebit := b.GetGLInventoryTotal()
	report.GLInventoryDebit = glInvDebit
	if invTotal.Equal(glInvDebit) {
		report.InventoryReconciled = true
	} else {
		report.InventoryReconciled = false
		report.Discrepancies = append(report.Discrepancies, fmt.Sprintf(
			"Inventory subledger (%s VND) does not match TK 15x (%s VND)",
			invTotal, glInvDebit,
		))
	}

	// 5. Fixed Asset Reconciliation (TK 211 & TK 214)
	assetCost := decimal.Zero
	assetDepr := decimal.Zero
	for _, a := range b.Assets {
		if a.OriginalCost.LessThanOrEqual(decimal.Zero) || a.UsefulLifeMonths <= 0 {
			report.Discrepancies = append(report.Discrepancies, fmt.Sprintf(
				"Fixed asset %s has invalid original cost (%s) or useful life (%d)",
				a.AssetCode, a.OriginalCost, a.UsefulLifeMonths,
			))
		}
		assetCost = assetCost.Add(a.OriginalCost)
		assetDepr = assetDepr.Add(a.AccumulatedDepreciation)
	}
	report.FixedAssetsReconciled = true
	for _, a := range b.Assets {
		if a.OriginalCost.LessThan(decimal.NewFromInt(30000000)) {
			report.FixedAssetsReconciled = false
			report.Discrepancies = append(report.Discrepancies, fmt.Sprintf(
				"Asset %s original cost %s is below statutory 30,000,000 VND threshold (Circular 45/2013/TT-BTC)",
				a.AssetCode, a.OriginalCost,
			))
		}
		if a.NetBookValue.IsNegative() {
			report.FixedAssetsReconciled = false
			report.Discrepancies = append(report.Discrepancies, fmt.Sprintf(
				"Asset %s net book value %s is negative (INV-OPEN-07)",
				a.AssetCode, a.NetBookValue,
			))
		}
	}
	report.AssetTotalCost = assetCost
	report.AssetTotalDepr = assetDepr
	gl211Dr, _ := b.GetGLBalanceByPrefix("211")
	_, gl214Cr := b.GetGLBalanceByPrefix("214")
	report.GL211Debit = gl211Dr
	report.GL214Credit = gl214Cr
	if !(assetCost.Equal(gl211Dr) && assetDepr.Equal(gl214Cr)) {
		report.FixedAssetsReconciled = false
		report.Discrepancies = append(report.Discrepancies, fmt.Sprintf(
			"Fixed asset schedule (Cost: %s, Depr: %s) does not match TK 211/214 (Dr 211: %s, Cr 214: %s)",
			assetCost, assetDepr, gl211Dr, gl214Cr,
		))
	}

	// 6. Multi-Currency Valuation Verification (INV-OPEN-06)
	for _, c := range b.Customers {
		if c.CurrencyCode != "" && c.CurrencyCode != "VND" && !c.ExchangeRate.IsZero() {
			fc := c.DebitAmountFC.Add(c.CreditAmountFC)
			expectedVND := fc.Mul(c.ExchangeRate).RoundBank(0)
			actualVND := c.DebitAmountVND.Add(c.CreditAmountVND)
			if !expectedVND.Sub(actualVND).Abs().LessThanOrEqual(decimal.NewFromInt(1)) {
				report.Discrepancies = append(report.Discrepancies, fmt.Sprintf(
					"Customer invoice %s FC valuation mismatch: %s %s @ %s != %s VND",
					c.InvoiceNo, fc, c.CurrencyCode, c.ExchangeRate, actualVND,
				))
			}
		}
	}
	for _, v := range b.Vendors {
		if v.CurrencyCode != "" && v.CurrencyCode != "VND" && !v.ExchangeRate.IsZero() {
			fc := v.DebitAmountFC.Add(v.CreditAmountFC)
			expectedVND := fc.Mul(v.ExchangeRate).RoundBank(0)
			actualVND := v.DebitAmountVND.Add(v.CreditAmountVND)
			if !expectedVND.Sub(actualVND).Abs().LessThanOrEqual(decimal.NewFromInt(1)) {
				report.Discrepancies = append(report.Discrepancies, fmt.Sprintf(
					"Vendor bill %s FC valuation mismatch: %s %s @ %s != %s VND",
					v.BillNo, fc, v.CurrencyCode, v.ExchangeRate, actualVND,
				))
			}
		}
	}

	report.AllReconciled = report.TrialBalanceBalanced &&
		report.CustomerARReconciled &&
		report.VendorAPReconciled &&
		report.InventoryReconciled &&
		report.FixedAssetsReconciled &&
		len(report.Discrepancies) == 0

	if !report.AllReconciled {
		return report, fmt.Errorf("%w: %s", ErrSubledgerReconciliationFailed, strings.Join(report.Discrepancies, "; "))
	}

	b.Status = BatchStatusValidated
	return report, nil
}

// Commit finalizes and locks the opening batch (INV-OPEN-07)
func (b *OpeningBatch) Commit(committedBy string) error {
	if b.Status == BatchStatusCommitted || b.Status == BatchStatusLocked {
		return ErrBatchAlreadyCommitted
	}

	report, err := b.Reconcile()
	if err != nil || !report.AllReconciled {
		return fmt.Errorf("%w: reconciliation failed", ErrBatchNotValidated)
	}

	now := time.Now()
	b.Status = BatchStatusCommitted
	b.CommittedAt = &now
	b.CommittedBy = &committedBy
	b.UpdatedAt = now
	return nil
}

// Lock places hard freeze on opening batch
func (b *OpeningBatch) Lock() error {
	if b.Status != BatchStatusCommitted {
		return fmt.Errorf("cannot lock batch with status '%s': batch must be committed first", b.Status)
	}
	b.Status = BatchStatusLocked
	b.UpdatedAt = time.Now()
	return nil
}

// Helper methods for GL balance aggregation
func (b *OpeningBatch) GetGLBalanceByPrefix(prefix string) (debit, credit decimal.Decimal) {
	debit = decimal.Zero
	credit = decimal.Zero
	for _, acc := range b.Accounts {
		if strings.HasPrefix(acc.AccountCode, prefix) {
			debit = debit.Add(acc.DebitAmountVND)
			credit = credit.Add(acc.CreditAmountVND)
		}
	}
	return debit, credit
}

func (b *OpeningBatch) GetGLInventoryTotal() decimal.Decimal {
	total := decimal.Zero
	prefixes := []string{"151", "152", "153", "155", "156", "157", "158"}
	for _, acc := range b.Accounts {
		for _, p := range prefixes {
			if strings.HasPrefix(acc.AccountCode, p) {
				total = total.Add(acc.DebitAmountVND)
				break
			}
		}
	}
	return total
}

// OpeningRepository defines domain persistence methods
type OpeningRepository interface {
	SaveOpeningBatch(ctx context.Context, batch *OpeningBatch) error
	GetOpeningBatchByID(ctx context.Context, id string) (*OpeningBatch, error)
	GetOpeningBatchByDate(ctx context.Context, companyID string, asOfDate time.Time) (*OpeningBatch, error)
	SaveAccountBalances(ctx context.Context, batchID string, balances []AccountOpeningBalance) error
	SaveCustomerBalances(ctx context.Context, batchID string, balances []CustomerOpeningBalance) error
	SaveVendorBalances(ctx context.Context, batchID string, balances []VendorOpeningBalance) error
	SaveInventoryBalances(ctx context.Context, batchID string, balances []InventoryOpeningBalance) error
	SaveAssetBalances(ctx context.Context, batchID string, balances []AssetOpeningBalance) error
	UpdateBatchStatus(ctx context.Context, batchID string, status BatchStatus, committedBy *string, committedAt *time.Time) error
}

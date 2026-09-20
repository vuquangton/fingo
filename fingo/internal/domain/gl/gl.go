package gl

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/shopspring/decimal"
)

// Standard domain errors
var (
	ErrVoucherEmptyLines        = errors.New("voucher must contain at least one line")
	ErrVoucherUnbalanced        = errors.New("voucher debit and credit amounts must be equal")
	ErrSameDebitCreditAccount   = errors.New("debit and credit accounts cannot be identical")
	ErrAccountRequired          = errors.New("both debit and credit accounts are required")
	ErrCostCenterRequired       = errors.New("cost center is required for cost and expense accounts (INV-OPS-03)")
	ErrInvalidVoucherStatus     = errors.New("invalid voucher status transition")
	ErrVoucherAlreadyPosted     = errors.New("cannot modify or delete already posted voucher")
	ErrVoucherAlreadyCancelled  = errors.New("voucher is already cancelled")
	ErrZeroVoucherAmount        = errors.New("voucher line amount must not be zero")
	ErrPeriodLocked             = errors.New("voucher date falls within locked accounting period (INV-OPS-04)")
)

type VoucherStatus string

const (
	VoucherStatusDraft     VoucherStatus = "DRAFT"
	VoucherStatusPosted    VoucherStatus = "POSTED"
	VoucherStatusCancelled VoucherStatus = "CANCELLED"
)

type VoucherType string

const (
	VoucherTypeGeneral        VoucherType = "GENERAL"
	VoucherTypeCashReceipt    VoucherType = "CASH_RECEIPT"
	VoucherTypeCashPayment    VoucherType = "CASH_PAYMENT"
	VoucherTypeBankReceipt    VoucherType = "BANK_RECEIPT"
	VoucherTypeBankPayment    VoucherType = "BANK_PAYMENT"
	VoucherTypeSales          VoucherType = "SALES"
	VoucherTypeSalesReturn    VoucherType = "SALES_RETURN"
	VoucherTypePurchase       VoucherType = "PURCHASE"
	VoucherTypePurchaseReturn VoucherType = "PURCHASE_RETURN"
	VoucherTypeStockInward    VoucherType = "STOCK_INWARD"
	VoucherTypeStockOutward   VoucherType = "STOCK_OUTWARD"
	VoucherTypeAsset          VoucherType = "ASSET"
	VoucherTypePayroll        VoucherType = "PAYROLL"
)

type VoucherLine struct {
	ID                string          `json:"id"`
	VoucherID         string          `json:"voucher_id"`
	LineOrder         int             `json:"line_order"`
	DebitAccountID    string          `json:"debit_account_id"`
	CreditAccountID   string          `json:"credit_account_id"`
	DebitAccountCode  string          `json:"debit_account_code"`
	CreditAccountCode string          `json:"credit_account_code"`
	AmountFC          decimal.Decimal `json:"amount_fc"`
	AmountVND         decimal.Decimal `json:"amount_vnd"`
	Note              string          `json:"note"`
	CustomerID        *string         `json:"customer_id,omitempty"`
	VendorID          *string         `json:"vendor_id,omitempty"`
	EmployeeID        *string         `json:"employee_id,omitempty"`
	ItemID            *string         `json:"item_id,omitempty"`
	WarehouseID       *string         `json:"warehouse_id,omitempty"`
	CostCenterID      *string         `json:"cost_center_id,omitempty"`
	ExpenseItemID     *string         `json:"expense_item_id,omitempty"`
	InvoiceNo         *string         `json:"invoice_no,omitempty"`
	InvoiceDate       *time.Time      `json:"invoice_date,omitempty"`
	CreatedAt         time.Time       `json:"created_at"`
}

type Voucher struct {
	ID                 string          `json:"id"`
	CompanyProfileID   string          `json:"company_profile_id"`
	BranchID           *string         `json:"branch_id,omitempty"`
	VoucherNo          string          `json:"voucher_no"`
	VoucherDate        time.Time       `json:"voucher_date"`
	PostedDate         time.Time       `json:"posted_date"`
	VoucherType        VoucherType     `json:"voucher_type"`
	Description        string          `json:"description"`
	Status             VoucherStatus   `json:"status"`
	TotalDebit         decimal.Decimal `json:"total_debit"`
	TotalCredit        decimal.Decimal `json:"total_credit"`
	CurrencyCode       string          `json:"currency_code"`
	ExchangeRate       decimal.Decimal `json:"exchange_rate"`
	SourceDocumentID   *string         `json:"source_document_id,omitempty"`
	SourceDocumentType *string         `json:"source_document_type,omitempty"`
	IdempotencyKey     *string         `json:"idempotency_key,omitempty"`
	CreatedBy          string          `json:"created_by"`
	Lines              []VoucherLine   `json:"lines"`
	CreatedAt          time.Time       `json:"created_at"`
	UpdatedAt          time.Time       `json:"updated_at"`
}

// CreateVoucherParams domain parameters
type CreateVoucherParams struct {
	ID                 string
	CompanyProfileID   string
	BranchID           *string
	VoucherNo          string
	VoucherDate        time.Time
	PostedDate         time.Time
	VoucherType        VoucherType
	Description        string
	CurrencyCode       string
	ExchangeRate       decimal.Decimal
	SourceDocumentID   *string
	SourceDocumentType *string
	IdempotencyKey     *string
	CreatedBy          string
	Lines              []VoucherLine
}

func NewVoucher(p CreateVoucherParams) (*Voucher, error) {
	if strings.TrimSpace(p.CompanyProfileID) == "" {
		return nil, errors.New("company profile id is required")
	}
	if strings.TrimSpace(p.VoucherNo) == "" {
		return nil, errors.New("voucher number is required")
	}
	if p.VoucherDate.IsZero() {
		return nil, errors.New("voucher date is required")
	}
	postedDate := p.PostedDate
	if postedDate.IsZero() {
		postedDate = p.VoucherDate
	}
	vType := p.VoucherType
	if vType == "" {
		vType = VoucherTypeGeneral
	}
	curr := p.CurrencyCode
	if curr == "" {
		curr = "VND"
	}
	rate := p.ExchangeRate
	if rate.IsZero() {
		rate = decimal.NewFromInt(1)
	}

	now := time.Now()
	v := &Voucher{
		ID:                 p.ID,
		CompanyProfileID:   p.CompanyProfileID,
		BranchID:           p.BranchID,
		VoucherNo:          p.VoucherNo,
		VoucherDate:        p.VoucherDate,
		PostedDate:         postedDate,
		VoucherType:        vType,
		Description:        p.Description,
		Status:             VoucherStatusDraft,
		TotalDebit:         decimal.Zero,
		TotalCredit:        decimal.Zero,
		CurrencyCode:       curr,
		ExchangeRate:       rate,
		SourceDocumentID:   p.SourceDocumentID,
		SourceDocumentType: p.SourceDocumentType,
		IdempotencyKey:     p.IdempotencyKey,
		CreatedBy:          p.CreatedBy,
		Lines:              make([]VoucherLine, 0, len(p.Lines)),
		CreatedAt:          now,
		UpdatedAt:          now,
	}

	for _, line := range p.Lines {
		if err := v.AddLine(line); err != nil {
			return nil, err
		}
	}

	return v, nil
}

func (v *Voucher) AddLine(line VoucherLine) error {
	if line.AmountVND.IsZero() {
		return ErrZeroVoucherAmount
	}
	if strings.TrimSpace(line.DebitAccountID) == "" || strings.TrimSpace(line.CreditAccountID) == "" {
		return ErrAccountRequired
	}
	if line.DebitAccountID == line.CreditAccountID {
		return ErrSameDebitCreditAccount
	}

	// Cost Center mandatory check (INV-OPS-03)
	// Check if debit or credit account is an expense/production cost account (641, 642, 621, 622, 627, 154)
	if isCostOrExpenseAccount(line.DebitAccountCode) || isCostOrExpenseAccount(line.CreditAccountCode) {
		if line.CostCenterID == nil || strings.TrimSpace(*line.CostCenterID) == "" {
			return fmt.Errorf("%w: account %s / %s", ErrCostCenterRequired, line.DebitAccountCode, line.CreditAccountCode)
		}
	}

	// Multi-Currency Valuation check (INV-OPS-06)
	if !line.AmountFC.IsZero() && !v.ExchangeRate.IsZero() && v.CurrencyCode != "VND" {
		expectedVND := line.AmountFC.Mul(v.ExchangeRate).RoundBank(0)
		if !expectedVND.Sub(line.AmountVND).Abs().LessThanOrEqual(decimal.NewFromInt(1)) {
			return fmt.Errorf("amount VND %s does not match foreign currency %s @ rate %s (%s VND)",
				line.AmountVND, line.AmountFC, v.ExchangeRate, expectedVND)
		}
	}

	line.LineOrder = len(v.Lines) + 1
	v.Lines = append(v.Lines, line)
	v.recalculateTotals()
	return nil
}

func (v *Voucher) recalculateTotals() {
	total := decimal.Zero
	for _, l := range v.Lines {
		total = total.Add(l.AmountVND)
	}
	v.TotalDebit = total
	v.TotalCredit = total
}

func (v *Voucher) ValidateBalance() error {
	if len(v.Lines) == 0 {
		return ErrVoucherEmptyLines
	}
	if !v.TotalDebit.Equal(v.TotalCredit) {
		return ErrVoucherUnbalanced
	}
	return nil
}

// Post transitions voucher from DRAFT to POSTED (INV-OPS-15)
func (v *Voucher) Post() error {
	if v.Status == VoucherStatusPosted {
		return ErrVoucherAlreadyPosted
	}
	if v.Status == VoucherStatusCancelled {
		return ErrVoucherAlreadyCancelled
	}
	if err := v.ValidateBalance(); err != nil {
		return err
	}
	v.Status = VoucherStatusPosted
	v.UpdatedAt = time.Now()
	return nil
}

// Cancel transitions voucher to CANCELLED
func (v *Voucher) Cancel() error {
	if v.Status == VoucherStatusCancelled {
		return ErrVoucherAlreadyCancelled
	}
	v.Status = VoucherStatusCancelled
	v.UpdatedAt = time.Now()
	return nil
}

// CreateReversingVoucher creates a Storno reversing entry voucher (ghi số âm / đảo nợ có)
func (v *Voucher) CreateReversingVoucher(newID, newVoucherNo string, reverseDate time.Time, createdBy string) (*Voucher, error) {
	if v.Status != VoucherStatusPosted {
		return nil, errors.New("only posted vouchers can be reversed")
	}

	revLines := make([]VoucherLine, len(v.Lines))
	for i, l := range v.Lines {
		revLines[i] = VoucherLine{
			LineOrder:         i + 1,
			DebitAccountID:    l.DebitAccountID,
			CreditAccountID:   l.CreditAccountID,
			DebitAccountCode:  l.DebitAccountCode,
			CreditAccountCode: l.CreditAccountCode,
			AmountFC:          l.AmountFC.Neg(),
			AmountVND:         l.AmountVND.Neg(),
			Note:              fmt.Sprintf("Reversal of %s: %s", v.VoucherNo, l.Note),
			CustomerID:        l.CustomerID,
			VendorID:          l.VendorID,
			EmployeeID:        l.EmployeeID,
			ItemID:            l.ItemID,
			WarehouseID:       l.WarehouseID,
			CostCenterID:      l.CostCenterID,
			ExpenseItemID:     l.ExpenseItemID,
			InvoiceNo:         l.InvoiceNo,
			InvoiceDate:       l.InvoiceDate,
		}
	}

	desc := fmt.Sprintf("Reversing voucher for %s (Storno)", v.VoucherNo)
	sourceDocID := v.ID
	sourceDocType := "REVERSAL"

	return NewVoucher(CreateVoucherParams{
		ID:                 newID,
		CompanyProfileID:   v.CompanyProfileID,
		BranchID:           v.BranchID,
		VoucherNo:          newVoucherNo,
		VoucherDate:        reverseDate,
		PostedDate:         reverseDate,
		VoucherType:        v.VoucherType,
		Description:        desc,
		CurrencyCode:       v.CurrencyCode,
		ExchangeRate:       v.ExchangeRate,
		SourceDocumentID:   &sourceDocID,
		SourceDocumentType: &sourceDocType,
		CreatedBy:          createdBy,
		Lines:              revLines,
	})
}

func isCostOrExpenseAccount(code string) bool {
	if len(code) < 3 {
		return false
	}
	prefix := code[:3]
	switch prefix {
	case "641", "642", "621", "622", "627", "154":
		return true
	}
	return false
}

// Pure domain repository interface
type GLRepository interface {
	SaveVoucher(ctx context.Context, v *Voucher) error
	GetVoucherByID(ctx context.Context, id string) (*Voucher, error)
	GetVoucherByNo(ctx context.Context, companyID string, vType VoucherType, voucherNo string) (*Voucher, error)
	GetVoucherByIdempotencyKey(ctx context.Context, companyID, idempotencyKey string) (*Voucher, error)
	UpdateVoucherStatus(ctx context.Context, id string, status VoucherStatus) error
	ListVouchersByPeriod(ctx context.Context, companyID string, fromDate, toDate time.Time) ([]Voucher, error)
}

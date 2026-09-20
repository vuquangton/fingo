package cash

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/shopspring/decimal"
)

const (
	NonCashConfirmationCode = "NON_CASH_VIOLATION_CONFIRMED"
)

var (
	NonCashThresholdVND = decimal.NewFromInt(5_000_000)

	ErrInvalidCashAmount                   = errors.New("cash amount must be greater than zero")
	ErrMissingPayerOrReceiver              = errors.New("payer or receiver name is required")
	ErrMissingReason                       = errors.New("business reason for cash transaction is required")
	ErrMissingCashAccount                  = errors.New("cash account ID is required")
	ErrNonCashThresholdExceeded            = errors.New("commercial transactions of 5,000,000 VND or more require non-cash payment (Decree 181/2025/ND-CP)")
	ErrChiefAccountantConfirmationRequired = errors.New("cash payment >= 5,000,000 VND requires Chief Accountant confirmation code (NON_CASH_VIOLATION_CONFIRMED)")
	ErrOverrideReasonRequired              = errors.New("override reason is required when non-cash threshold is bypassed")
)

// CashReceipt represents a Vietnamese Phiếu Thu (Form 01-TT, Circular 133/200).
type CashReceipt struct {
	ID                    string
	VoucherID             string
	CompanyProfileID      string
	PayerName             string
	PayerAddress          string
	Reason                string
	CashAccountID         string
	TotalAmountVND        decimal.Decimal
	AccompanyingDocuments string
	CreatedAt             time.Time
}

type CreateCashReceiptParams struct {
	ID                    string
	VoucherID             string
	CompanyProfileID      string
	PayerName             string
	PayerAddress          string
	Reason                string
	CashAccountID         string
	TotalAmountVND        decimal.Decimal
	AccompanyingDocuments string
}

func NewCashReceipt(params CreateCashReceiptParams) (*CashReceipt, error) {
	if params.TotalAmountVND.LessThanOrEqual(decimal.Zero) {
		return nil, ErrInvalidCashAmount
	}
	if strings.TrimSpace(params.PayerName) == "" {
		return nil, ErrMissingPayerOrReceiver
	}
	if strings.TrimSpace(params.Reason) == "" {
		return nil, ErrMissingReason
	}
	if strings.TrimSpace(params.CashAccountID) == "" {
		return nil, ErrMissingCashAccount
	}

	return &CashReceipt{
		ID:                    params.ID,
		VoucherID:             params.VoucherID,
		CompanyProfileID:      params.CompanyProfileID,
		PayerName:             params.PayerName,
		PayerAddress:          params.PayerAddress,
		Reason:                params.Reason,
		CashAccountID:         params.CashAccountID,
		TotalAmountVND:        params.TotalAmountVND.RoundBank(0),
		AccompanyingDocuments: params.AccompanyingDocuments,
		CreatedAt:             time.Now(),
	}, nil
}

// CashPayment represents a Vietnamese Phiếu Chi (Form 02-TT, Circular 133/200).
type CashPayment struct {
	ID                    string
	VoucherID             string
	CompanyProfileID      string
	ReceiverName          string
	ReceiverAddress       string
	Reason                string
	CashAccountID         string
	TotalAmountVND        decimal.Decimal
	IsNonCashOverride     bool
	OverrideReason        string
	AccompanyingDocuments string
	CreatedAt             time.Time
}

type CreateCashPaymentParams struct {
	ID                         string
	VoucherID                  string
	CompanyProfileID           string
	ReceiverName               string
	ReceiverAddress            string
	Reason                     string
	CashAccountID              string
	TotalAmountVND             decimal.Decimal
	RequiresNonCashCompliance  bool
	IsNonCashOverride          bool
	OverrideReason             string
	ConfirmationCode           string
	AccompanyingDocuments      string
}

func NewCashPayment(params CreateCashPaymentParams) (*CashPayment, error) {
	if params.TotalAmountVND.LessThanOrEqual(decimal.Zero) {
		return nil, ErrInvalidCashAmount
	}
	if strings.TrimSpace(params.ReceiverName) == "" {
		return nil, ErrMissingPayerOrReceiver
	}
	if strings.TrimSpace(params.Reason) == "" {
		return nil, ErrMissingReason
	}
	if strings.TrimSpace(params.CashAccountID) == "" {
		return nil, ErrMissingCashAccount
	}

	// INV-OPS-05: Decree 181/2025/NĐ-CP Non-Cash Rule on commercial transactions >= 5,000,000 VND
	if params.RequiresNonCashCompliance && params.TotalAmountVND.GreaterThanOrEqual(NonCashThresholdVND) {
		if !params.IsNonCashOverride {
			return nil, ErrNonCashThresholdExceeded
		}
		if params.ConfirmationCode != NonCashConfirmationCode {
			return nil, ErrChiefAccountantConfirmationRequired
		}
		if strings.TrimSpace(params.OverrideReason) == "" {
			return nil, ErrOverrideReasonRequired
		}
	}

	return &CashPayment{
		ID:                    params.ID,
		VoucherID:             params.VoucherID,
		CompanyProfileID:      params.CompanyProfileID,
		ReceiverName:          params.ReceiverName,
		ReceiverAddress:       params.ReceiverAddress,
		Reason:                params.Reason,
		CashAccountID:         params.CashAccountID,
		TotalAmountVND:        params.TotalAmountVND.RoundBank(0),
		IsNonCashOverride:     params.IsNonCashOverride,
		OverrideReason:        params.OverrideReason,
		AccompanyingDocuments: params.AccompanyingDocuments,
		CreatedAt:             time.Now(),
	}, nil
}

// CashRepository defines database operations for the cash subledger.
type CashRepository interface {
	SaveCashReceipt(ctx context.Context, cr *CashReceipt) error
	GetCashReceiptByID(ctx context.Context, id string) (*CashReceipt, error)
	GetCashReceiptByVoucherID(ctx context.Context, voucherID string) (*CashReceipt, error)

	SaveCashPayment(ctx context.Context, cp *CashPayment) error
	GetCashPaymentByID(ctx context.Context, id string) (*CashPayment, error)
	GetCashPaymentByVoucherID(ctx context.Context, voucherID string) (*CashPayment, error)
}

// CashRepositoryStub provides an in-memory thread-safe implementation of CashRepository for testing.
type CashRepositoryStub struct {
	mu                sync.RWMutex
	receipts          map[string]*CashReceipt
	receiptsByVoucher map[string]*CashReceipt
	payments          map[string]*CashPayment
	paymentsByVoucher map[string]*CashPayment
}

func NewCashRepositoryStub() *CashRepositoryStub {
	return &CashRepositoryStub{
		receipts:          make(map[string]*CashReceipt),
		receiptsByVoucher: make(map[string]*CashReceipt),
		payments:          make(map[string]*CashPayment),
		paymentsByVoucher: make(map[string]*CashPayment),
	}
}

func (s *CashRepositoryStub) SaveCashReceipt(ctx context.Context, cr *CashReceipt) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.receipts[cr.ID] = cr
	s.receiptsByVoucher[cr.VoucherID] = cr
	return nil
}

func (s *CashRepositoryStub) GetCashReceiptByID(ctx context.Context, id string) (*CashReceipt, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	cr, ok := s.receipts[id]
	if !ok {
		return nil, sql.ErrNoRows
	}
	return cr, nil
}

func (s *CashRepositoryStub) GetCashReceiptByVoucherID(ctx context.Context, voucherID string) (*CashReceipt, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	cr, ok := s.receiptsByVoucher[voucherID]
	if !ok {
		return nil, sql.ErrNoRows
	}
	return cr, nil
}

func (s *CashRepositoryStub) SaveCashPayment(ctx context.Context, cp *CashPayment) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.payments[cp.ID] = cp
	s.paymentsByVoucher[cp.VoucherID] = cp
	return nil
}

func (s *CashRepositoryStub) GetCashPaymentByID(ctx context.Context, id string) (*CashPayment, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	cp, ok := s.payments[id]
	if !ok {
		return nil, sql.ErrNoRows
	}
	return cp, nil
}

func (s *CashRepositoryStub) GetCashPaymentByVoucherID(ctx context.Context, voucherID string) (*CashPayment, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	cp, ok := s.paymentsByVoucher[voucherID]
	if !ok {
		return nil, sql.ErrNoRows
	}
	return cp, nil
}

package cash

import (
	"context"
	"errors"
	"strings"
	"sync"

	"github.com/shopspring/decimal"
)

type BankTransactionType string

const (
	BankTransactionTypeCreditAdvice  BankTransactionType = "CREDIT_ADVICE"  // Báo Có (112)
	BankTransactionTypeDebitAdvice   BankTransactionType = "DEBIT_ADVICE"   // Báo Nợ (112)
	BankTransactionTypeTransferOrder BankTransactionType = "TRANSFER_ORDER" // Ủy nhiệm chi (UNC)
)

var (
	ErrInvalidBankAmount         = errors.New("total bank transaction amount must be greater than zero")
	ErrNegativeBankFee           = errors.New("bank fee and VAT fee amounts cannot be negative")
	ErrMissingBankAccount       = errors.New("bank account ID is required")
	ErrInvalidTransactionType    = errors.New("invalid bank transaction type")
	ErrBankTransactionNotFound   = errors.New("bank transaction not found")
	ErrFeeAccountRequired        = errors.New("fee expense account ID is required when bank fee is specified")
	ErrVatFeeAccountRequired     = errors.New("VAT fee account ID is required when VAT fee is specified")
	ErrPrincipalMismatch         = errors.New("sum of payment lines does not equal principal amount")
	ErrForeignCurrencyMismatch   = errors.New("foreign currency bank transaction requires currency code other than VND and positive exchange rate (INV-OPS-06)")
	ErrBaseCurrencyMismatch      = errors.New("VND bank account (1121) requires currency VND and exchange rate 1.0 (INV-OPS-06)")
)

// BankCounterpartyInfo bundles counterparty remittance details to prevent data clumps.
type BankCounterpartyInfo struct {
	AccountNo string
	BankName  string
	Name      string
	Reference string
}

// BankTransaction represents a transaction against a corporate bank account (TK 112).
type BankTransaction struct {
	ID               string
	VoucherID        string
	CompanyProfileID string
	BankAccountID    string
	TransactionType  BankTransactionType
	Counterparty     BankCounterpartyInfo
	FeeAmountVND     decimal.Decimal
	VatFeeAmountVND  decimal.Decimal
	TotalAmountVND   decimal.Decimal
}

type CreateBankTransactionParams struct {
	ID              string
	VoucherID       string
	CompanyProfileID string
	BankAccountID   string
	TransactionType BankTransactionType
	Counterparty    BankCounterpartyInfo
	FeeAmountVND    decimal.Decimal
	VatFeeAmountVND decimal.Decimal
	TotalAmountVND  decimal.Decimal
}

func NewBankTransaction(params CreateBankTransactionParams) (*BankTransaction, error) {
	if params.TotalAmountVND.LessThanOrEqual(decimal.Zero) {
		return nil, ErrInvalidBankAmount
	}
	if params.FeeAmountVND.LessThan(decimal.Zero) || params.VatFeeAmountVND.LessThan(decimal.Zero) {
		return nil, ErrNegativeBankFee
	}
	if strings.TrimSpace(params.BankAccountID) == "" {
		return nil, ErrMissingBankAccount
	}

	switch params.TransactionType {
	case BankTransactionTypeCreditAdvice, BankTransactionTypeDebitAdvice, BankTransactionTypeTransferOrder:
		// Valid
	default:
		return nil, ErrInvalidTransactionType
	}

	return &BankTransaction{
		ID:               params.ID,
		VoucherID:        params.VoucherID,
		CompanyProfileID: params.CompanyProfileID,
		BankAccountID:    params.BankAccountID,
		TransactionType:  params.TransactionType,
		Counterparty:     params.Counterparty,
		FeeAmountVND:     params.FeeAmountVND.RoundBank(0),
		VatFeeAmountVND:  params.VatFeeAmountVND.RoundBank(0),
		TotalAmountVND:   params.TotalAmountVND.RoundBank(0),
	}, nil
}

// BankRepository defines database operations for the bank subledger.
type BankRepository interface {
	SaveBankTransaction(ctx context.Context, bt *BankTransaction) error
	GetBankTransactionByID(ctx context.Context, id string) (*BankTransaction, error)
	GetBankTransactionByVoucherID(ctx context.Context, voucherID string) (*BankTransaction, error)
}

// BankRepositoryStub provides an in-memory thread-safe pure Go implementation of BankRepository.
type BankRepositoryStub struct {
	mu                   sync.RWMutex
	transactions         map[string]*BankTransaction
	transactionByVoucher map[string]*BankTransaction
}

func NewBankRepositoryStub() *BankRepositoryStub {
	return &BankRepositoryStub{
		transactions:         make(map[string]*BankTransaction),
		transactionByVoucher: make(map[string]*BankTransaction),
	}
}

func (s *BankRepositoryStub) SaveBankTransaction(ctx context.Context, bt *BankTransaction) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.transactions[bt.ID] = bt
	s.transactionByVoucher[bt.VoucherID] = bt
	return nil
}

func (s *BankRepositoryStub) GetBankTransactionByID(ctx context.Context, id string) (*BankTransaction, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	bt, ok := s.transactions[id]
	if !ok {
		return nil, ErrBankTransactionNotFound
	}
	return bt, nil
}

func (s *BankRepositoryStub) GetBankTransactionByVoucherID(ctx context.Context, voucherID string) (*BankTransaction, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	bt, ok := s.transactionByVoucher[voucherID]
	if !ok {
		return nil, ErrBankTransactionNotFound
	}
	return bt, nil
}

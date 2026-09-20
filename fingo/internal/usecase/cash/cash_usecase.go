package cash

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"fingo/internal/domain/cash"
	"fingo/internal/domain/gl"
	spineDomain "fingo/internal/domain/spine"
	glUseCase "fingo/internal/usecase/gl"
	"fingo/pkg/logger"
)

var (
	ErrCashReceiptNotFound = errors.New("cash receipt not found")
	ErrCashPaymentNotFound = errors.New("cash payment not found")
	ErrInvalidCashAccount  = errors.New("specified account is not a valid cash account (class 111)")
)

type AccountRepo interface {
	GetAccountByID(ctx context.Context, id string) (*spineDomain.Account, error)
}

type CashUseCase struct {
	cashRepo  cash.CashRepository
	glUseCase *glUseCase.GLUseCase
	accRepo   AccountRepo
	logger    *logger.Logger
}

func NewCashUseCase(
	cashRepo cash.CashRepository,
	glUC *glUseCase.GLUseCase,
	accRepo AccountRepo,
	log *logger.Logger,
) *CashUseCase {
	return &CashUseCase{
		cashRepo:  cashRepo,
		glUseCase: glUC,
		accRepo:   accRepo,
		logger:    log,
	}
}

type CreateCashReceiptLineCommand struct {
	CreditAccountID string
	AmountVND       decimal.Decimal
	CustomerID      *string
	Note            string
}

type CreateCashReceiptCommand struct {
	CompanyProfileID      string
	BranchID              *string
	VoucherNo             string
	VoucherDate           time.Time
	PostedDate            time.Time
	PayerName             string
	PayerAddress          string
	Reason                string
	CashAccountID         string
	CreditAccountID       string // For simple 1-line receipt
	TotalAmountVND        decimal.Decimal
	CustomerID            *string
	Lines                 []CreateCashReceiptLineCommand // For compound multi-credit lines (e.g. 511 + 33311)
	AccompanyingDocuments string
	IdempotencyKey        *string
	CreatedBy             string
}

type CreateCashPaymentLineCommand struct {
	DebitAccountID string
	AmountVND      decimal.Decimal
	VendorID       *string
	EmployeeID     *string
	CostCenterID   *string
	ExpenseItemID  *string
	Note           string
}

type CreateCashPaymentCommand struct {
	CompanyProfileID          string
	BranchID                  *string
	VoucherNo                 string
	VoucherDate               time.Time
	PostedDate                time.Time
	ReceiverName              string
	ReceiverAddress           string
	Reason                    string
	CashAccountID             string
	DebitAccountID            string // For simple 1-line payment
	TotalAmountVND            decimal.Decimal
	VendorID                  *string
	EmployeeID                *string
	CostCenterID              *string
	ExpenseItemID             *string
	Lines                     []CreateCashPaymentLineCommand // For compound lines
	RequiresNonCashCompliance bool
	IsNonCashOverride         bool
	ConfirmationCode          string // NON_CASH_VIOLATION_CONFIRMED
	OverrideReason            string
	AccompanyingDocuments     string
	IdempotencyKey            *string
	CreatedBy                 string
}

func (u *CashUseCase) CreateCashReceipt(ctx context.Context, cmd CreateCashReceiptCommand) (*cash.CashReceipt, *gl.Voucher, error) {
	// 1. Validate Cash Account (Must belong to Class 111)
	if u.accRepo != nil {
		cashAcc, err := u.accRepo.GetAccountByID(ctx, cmd.CashAccountID)
		if err != nil {
			return nil, nil, fmt.Errorf("%w: cash account %s", glUseCase.ErrAccountNotFound, cmd.CashAccountID)
		}
		if !strings.HasPrefix(cashAcc.Code, "111") {
			return nil, nil, fmt.Errorf("%w: account %s is not a cash account (111)", ErrInvalidCashAccount, cashAcc.Code)
		}
	}

	// 2. Build Voucher Lines (Single line or Compound lines)
	var vLines []glUseCase.CreateVoucherLineCommand
	if len(cmd.Lines) > 0 {
		vLines = make([]glUseCase.CreateVoucherLineCommand, len(cmd.Lines))
		for i, l := range cmd.Lines {
			vLines[i] = glUseCase.CreateVoucherLineCommand{
				DebitAccountID:  cmd.CashAccountID,
				CreditAccountID: l.CreditAccountID,
				AmountVND:       l.AmountVND,
				CustomerID:      l.CustomerID,
				Note:            l.Note,
			}
		}
	} else {
		vLines = []glUseCase.CreateVoucherLineCommand{
			{
				DebitAccountID:  cmd.CashAccountID,
				CreditAccountID: cmd.CreditAccountID,
				AmountVND:       cmd.TotalAmountVND,
				CustomerID:      cmd.CustomerID,
				Note:            cmd.Reason,
			},
		}
	}

	// 3. Delegate Master GL Voucher Creation
	vCmd := glUseCase.CreateVoucherCommand{
		CompanyProfileID: cmd.CompanyProfileID,
		BranchID:         cmd.BranchID,
		VoucherNo:        cmd.VoucherNo,
		VoucherDate:      cmd.VoucherDate,
		PostedDate:       cmd.PostedDate,
		VoucherType:      gl.VoucherTypeCashReceipt,
		Description:      cmd.Reason,
		IdempotencyKey:   cmd.IdempotencyKey,
		CreatedBy:        cmd.CreatedBy,
		Lines:            vLines,
	}

	voucher, err := u.glUseCase.CreateVoucher(ctx, vCmd)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create master cash receipt voucher: %w", err)
	}

	// 4. Idempotent check for existing subledger record
	existing, err := u.cashRepo.GetCashReceiptByVoucherID(ctx, voucher.ID)
	if err == nil && existing != nil {
		return existing, voucher, nil
	}

	// 5. Construct and validate domain CashReceipt
	cr, err := cash.NewCashReceipt(cash.CreateCashReceiptParams{
		ID:                    uuid.New().String(),
		VoucherID:             voucher.ID,
		CompanyProfileID:      cmd.CompanyProfileID,
		PayerName:             cmd.PayerName,
		PayerAddress:          cmd.PayerAddress,
		Reason:                cmd.Reason,
		CashAccountID:         cmd.CashAccountID,
		TotalAmountVND:        cmd.TotalAmountVND,
		AccompanyingDocuments: cmd.AccompanyingDocuments,
	})
	if err != nil {
		return nil, nil, err
	}

	// 6. Persist CashReceipt
	if err := u.cashRepo.SaveCashReceipt(ctx, cr); err != nil {
		return nil, nil, fmt.Errorf("failed to persist cash receipt: %w", err)
	}

	u.logger.Info(ctx, "cash receipt created successfully",
		slog.String("entity_id", cr.ID),
		slog.String("voucher_id", voucher.ID),
		slog.String("voucher_no", voucher.VoucherNo),
		slog.String("amount", cr.TotalAmountVND.String()),
	)

	return cr, voucher, nil
}

func (u *CashUseCase) CreateCashPayment(ctx context.Context, cmd CreateCashPaymentCommand) (*cash.CashPayment, *gl.Voucher, error) {
	// 1. Validate Cash Account (Must belong to Class 111)
	if u.accRepo != nil {
		cashAcc, err := u.accRepo.GetAccountByID(ctx, cmd.CashAccountID)
		if err != nil {
			return nil, nil, fmt.Errorf("%w: cash account %s", glUseCase.ErrAccountNotFound, cmd.CashAccountID)
		}
		if !strings.HasPrefix(cashAcc.Code, "111") {
			return nil, nil, fmt.Errorf("%w: account %s is not a cash account (111)", ErrInvalidCashAccount, cashAcc.Code)
		}
	}

	// Determine if non-cash compliance is triggered (e.g. paying vendor or commercial purchase)
	requiresCompliance := cmd.RequiresNonCashCompliance || cmd.VendorID != nil

	// 2. Validate Decree 181/2025 non-cash gate via domain entity pre-validation
	cpID := uuid.New().String()
	cp, err := cash.NewCashPayment(cash.CreateCashPaymentParams{
		ID:                        cpID,
		VoucherID:                 "pre-validation",
		CompanyProfileID:          cmd.CompanyProfileID,
		ReceiverName:              cmd.ReceiverName,
		ReceiverAddress:           cmd.ReceiverAddress,
		Reason:                    cmd.Reason,
		CashAccountID:             cmd.CashAccountID,
		TotalAmountVND:            cmd.TotalAmountVND,
		RequiresNonCashCompliance: requiresCompliance,
		IsNonCashOverride:         cmd.IsNonCashOverride,
		ConfirmationCode:          cmd.ConfirmationCode,
		OverrideReason:            cmd.OverrideReason,
		AccompanyingDocuments:     cmd.AccompanyingDocuments,
	})
	if err != nil {
		return nil, nil, err
	}

	// 3. Build Voucher Lines (Single line or Compound lines)
	var vLines []glUseCase.CreateVoucherLineCommand
	if len(cmd.Lines) > 0 {
		vLines = make([]glUseCase.CreateVoucherLineCommand, len(cmd.Lines))
		for i, l := range cmd.Lines {
			vLines[i] = glUseCase.CreateVoucherLineCommand{
				DebitAccountID:  l.DebitAccountID,
				CreditAccountID: cmd.CashAccountID,
				AmountVND:       l.AmountVND,
				VendorID:        l.VendorID,
				EmployeeID:      l.EmployeeID,
				CostCenterID:    l.CostCenterID,
				ExpenseItemID:   l.ExpenseItemID,
				Note:            l.Note,
			}
		}
	} else {
		vLines = []glUseCase.CreateVoucherLineCommand{
			{
				DebitAccountID:  cmd.DebitAccountID,
				CreditAccountID: cmd.CashAccountID,
				AmountVND:       cmd.TotalAmountVND,
				VendorID:        cmd.VendorID,
				EmployeeID:      cmd.EmployeeID,
				CostCenterID:    cmd.CostCenterID,
				ExpenseItemID:   cmd.ExpenseItemID,
				Note:            cmd.Reason,
			},
		}
	}

	// 4. Delegate Master GL Voucher Creation
	vCmd := glUseCase.CreateVoucherCommand{
		CompanyProfileID: cmd.CompanyProfileID,
		BranchID:         cmd.BranchID,
		VoucherNo:        cmd.VoucherNo,
		VoucherDate:      cmd.VoucherDate,
		PostedDate:       cmd.PostedDate,
		VoucherType:      gl.VoucherTypeCashPayment,
		Description:      cmd.Reason,
		IdempotencyKey:   cmd.IdempotencyKey,
		CreatedBy:        cmd.CreatedBy,
		Lines:            vLines,
	}

	voucher, err := u.glUseCase.CreateVoucher(ctx, vCmd)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create master cash payment voucher: %w", err)
	}

	// 5. Idempotent check for existing subledger record
	existing, err := u.cashRepo.GetCashPaymentByVoucherID(ctx, voucher.ID)
	if err == nil && existing != nil {
		return existing, voucher, nil
	}

	// 6. Link voucher ID to domain payment and persist
	cp.VoucherID = voucher.ID
	if err := u.cashRepo.SaveCashPayment(ctx, cp); err != nil {
		return nil, nil, fmt.Errorf("failed to persist cash payment: %w", err)
	}

	u.logger.Info(ctx, "cash payment created successfully",
		slog.String("entity_id", cp.ID),
		slog.String("voucher_id", voucher.ID),
		slog.String("voucher_no", voucher.VoucherNo),
		slog.String("amount", cp.TotalAmountVND.String()),
	)

	return cp, voucher, nil
}

func (u *CashUseCase) GetCashReceipt(ctx context.Context, id string) (*cash.CashReceipt, error) {
	cr, err := u.cashRepo.GetCashReceiptByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("%w: %s", ErrCashReceiptNotFound, id)
	}
	return cr, nil
}

func (u *CashUseCase) GetCashPayment(ctx context.Context, id string) (*cash.CashPayment, error) {
	cp, err := u.cashRepo.GetCashPaymentByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("%w: %s", ErrCashPaymentNotFound, id)
	}
	return cp, nil
}

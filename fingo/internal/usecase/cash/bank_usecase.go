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
	"fingo/internal/domain/catalog"
	"fingo/internal/domain/gl"
	spineDomain "fingo/internal/domain/spine"
	glUseCase "fingo/internal/usecase/gl"
	"fingo/pkg/logger"
)

var (
	ErrBankTransactionNotFound = errors.New("bank transaction not found")
	ErrBankAccountNotFound     = errors.New("bank account not found in company catalog")
	ErrAccountNotFound         = errors.New("account not found in chart of accounts")
)

type BankAccountRepo interface {
	GetBankAccountByID(ctx context.Context, id string) (*catalog.BankAccount, error)
}

type AccountCatalogRepo interface {
	GetAccountByID(ctx context.Context, id string) (*spineDomain.Account, error)
}

type BankUseCase struct {
	bankRepo    cash.BankRepository
	bankAccRepo BankAccountRepo
	accRepo     AccountCatalogRepo
	glUseCase   *glUseCase.GLUseCase
	logger      *logger.Logger
}

func NewBankUseCase(
	bankRepo cash.BankRepository,
	bankAccRepo BankAccountRepo,
	accRepo AccountCatalogRepo,
	glUC *glUseCase.GLUseCase,
	log *logger.Logger,
) *BankUseCase {
	return &BankUseCase{
		bankRepo:    bankRepo,
		bankAccRepo: bankAccRepo,
		accRepo:     accRepo,
		glUseCase:   glUC,
		logger:      log,
	}
}

type CreateBankCreditAdviceLineCommand struct {
	CreditAccountID string
	AmountVND       decimal.Decimal
	CustomerID      *string
	Note            string
}

type CreateBankCreditAdviceCommand struct {
	CompanyProfileID string
	BranchID         *string
	VoucherNo        string
	VoucherDate      time.Time
	PostedDate       time.Time
	BankAccountID    string // ID from bank_accounts catalog
	Counterparty     cash.BankCounterpartyInfo
	Description      string
	TotalAmountVND   decimal.Decimal
	CreditAccountID  string                              // Simple 1-line credit
	CustomerID       *string                             // Optional counterparty
	Lines            []CreateBankCreditAdviceLineCommand // Compound lines
	CurrencyCode     string
	ExchangeRate     decimal.Decimal
	AmountFC         decimal.Decimal
	IdempotencyKey   *string
	CreatedBy        string
}

type CreateBankPaymentLineCommand struct {
	DebitAccountID string
	AmountVND      decimal.Decimal
	VendorID       *string
	EmployeeID     *string
	CostCenterID   *string
	ExpenseItemID  *string
	Note           string
}

type CreateBankPaymentCommand struct {
	CompanyProfileID   string
	BranchID           *string
	VoucherNo          string
	VoucherDate        time.Time
	PostedDate         time.Time
	BankAccountID      string                   // ID from bank_accounts catalog
	TransactionType    cash.BankTransactionType // DEBIT_ADVICE or TRANSFER_ORDER
	Counterparty       cash.BankCounterpartyInfo
	Description        string
	PrincipalAmountVND decimal.Decimal
	FeeAmountVND       decimal.Decimal
	FeeAccountID       string // Debit TK 642
	VatFeeAmountVND    decimal.Decimal
	VatFeeAccountID    string                         // Debit TK 13311
	DebitAccountID     string                         // Simple 1-line debit
	VendorID           *string                        // Optional vendor
	EmployeeID         *string                        // Optional employee
	CostCenterID       *string                        // Optional cost center
	ExpenseItemID      *string                        // Optional expense item
	Lines              []CreateBankPaymentLineCommand // Compound lines
	CurrencyCode       string
	ExchangeRate       decimal.Decimal
	AmountFC           decimal.Decimal
	IdempotencyKey     *string
	CreatedBy          string
}

func (u *BankUseCase) CreateCreditAdvice(ctx context.Context, cmd CreateBankCreditAdviceCommand) (*cash.BankTransaction, *gl.Voucher, error) {
	// 1. Resolve Bank GL Account from catalog and check currency seam (INV-OPS-06)
	bankGLAccountID, err := u.resolveBankGLAccount(ctx, cmd.BankAccountID, cmd.CurrencyCode, cmd.ExchangeRate)
	if err != nil {
		return nil, nil, err
	}

	// 2. Reconcile compound lines against total amount
	if len(cmd.Lines) > 0 {
		sumLines := decimal.Zero
		for _, l := range cmd.Lines {
			sumLines = sumLines.Add(l.AmountVND)
		}
		if !sumLines.Equal(cmd.TotalAmountVND) {
			return nil, nil, fmt.Errorf("%w: lines sum %s != total %s", cash.ErrPrincipalMismatch, sumLines.String(), cmd.TotalAmountVND.String())
		}
	}

	// 3. Build Voucher Lines
	var vLines []glUseCase.CreateVoucherLineCommand
	if len(cmd.Lines) > 0 {
		vLines = make([]glUseCase.CreateVoucherLineCommand, len(cmd.Lines))
		for i, l := range cmd.Lines {
			vLines[i] = glUseCase.CreateVoucherLineCommand{
				DebitAccountID:  bankGLAccountID,
				CreditAccountID: l.CreditAccountID,
				AmountVND:       l.AmountVND,
				CustomerID:      l.CustomerID,
				Note:            l.Note,
			}
		}
	} else {
		vLines = []glUseCase.CreateVoucherLineCommand{
			{
				DebitAccountID:  bankGLAccountID,
				CreditAccountID: cmd.CreditAccountID,
				AmountVND:       cmd.TotalAmountVND,
				CustomerID:      cmd.CustomerID,
				Note:            cmd.Description,
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
		VoucherType:      gl.VoucherTypeBankReceipt,
		Description:      cmd.Description,
		CurrencyCode:     cmd.CurrencyCode,
		ExchangeRate:     cmd.ExchangeRate,
		IdempotencyKey:   cmd.IdempotencyKey,
		CreatedBy:        cmd.CreatedBy,
		Lines:            vLines,
	}

	voucher, err := u.glUseCase.CreateVoucher(ctx, vCmd)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create master credit advice voucher: %w", err)
	}

	// 5. Idempotent check
	existing, err := u.bankRepo.GetBankTransactionByVoucherID(ctx, voucher.ID)
	if err == nil && existing != nil {
		return existing, voucher, nil
	}

	// 6. Construct domain BankTransaction
	bt, err := cash.NewBankTransaction(cash.CreateBankTransactionParams{
		ID:               uuid.New().String(),
		VoucherID:        voucher.ID,
		CompanyProfileID: cmd.CompanyProfileID,
		BankAccountID:    cmd.BankAccountID,
		TransactionType:  cash.BankTransactionTypeCreditAdvice,
		Counterparty:     cmd.Counterparty,
		FeeAmountVND:     decimal.Zero,
		VatFeeAmountVND:  decimal.Zero,
		TotalAmountVND:   cmd.TotalAmountVND,
	})
	if err != nil {
		return nil, nil, err
	}

	// 7. Persist BankTransaction
	if err := u.bankRepo.SaveBankTransaction(ctx, bt); err != nil {
		return nil, nil, fmt.Errorf("failed to persist credit advice transaction: %w", err)
	}

	u.logger.Info(ctx, "credit advice created successfully",
		slog.String("entity_id", bt.ID),
		slog.String("voucher_id", voucher.ID),
		slog.String("voucher_no", voucher.VoucherNo),
		slog.String("amount", bt.TotalAmountVND.String()),
	)

	return bt, voucher, nil
}

func (u *BankUseCase) CreateBankPayment(ctx context.Context, cmd CreateBankPaymentCommand) (*cash.BankTransaction, *gl.Voucher, error) {
	// Guard: Only accept DEBIT_ADVICE or TRANSFER_ORDER
	if cmd.TransactionType != cash.BankTransactionTypeDebitAdvice && cmd.TransactionType != cash.BankTransactionTypeTransferOrder {
		return nil, nil, fmt.Errorf("%w: bank payment must be DEBIT_ADVICE or TRANSFER_ORDER (got %s)", cash.ErrInvalidTransactionType, cmd.TransactionType)
	}

	// Guard: Banking Fee Account validation
	if cmd.FeeAmountVND.GreaterThan(decimal.Zero) && strings.TrimSpace(cmd.FeeAccountID) == "" {
		return nil, nil, cash.ErrFeeAccountRequired
	}
	if cmd.VatFeeAmountVND.GreaterThan(decimal.Zero) && strings.TrimSpace(cmd.VatFeeAccountID) == "" {
		return nil, nil, cash.ErrVatFeeAccountRequired
	}

	// 1. Resolve Bank GL Account from catalog and check currency seam (INV-OPS-06)
	bankGLAccountID, err := u.resolveBankGLAccount(ctx, cmd.BankAccountID, cmd.CurrencyCode, cmd.ExchangeRate)
	if err != nil {
		return nil, nil, err
	}

	// 2. Reconcile compound lines against principal amount
	if len(cmd.Lines) > 0 {
		sumLines := decimal.Zero
		for _, l := range cmd.Lines {
			sumLines = sumLines.Add(l.AmountVND)
		}
		if !sumLines.Equal(cmd.PrincipalAmountVND) {
			return nil, nil, fmt.Errorf("%w: lines sum %s != principal %s", cash.ErrPrincipalMismatch, sumLines.String(), cmd.PrincipalAmountVND.String())
		}
	}

	// Calculate total amount = Principal + Fee + VAT Fee
	totalAmount := cmd.PrincipalAmountVND.Add(cmd.FeeAmountVND).Add(cmd.VatFeeAmountVND)

	// 3. Build Voucher Lines (Principal lines + Automatic banking fee split lines)
	var vLines []glUseCase.CreateVoucherLineCommand
	if len(cmd.Lines) > 0 {
		for _, l := range cmd.Lines {
			vLines = append(vLines, glUseCase.CreateVoucherLineCommand{
				DebitAccountID:  l.DebitAccountID,
				CreditAccountID: bankGLAccountID,
				AmountVND:       l.AmountVND,
				VendorID:        l.VendorID,
				EmployeeID:      l.EmployeeID,
				CostCenterID:    l.CostCenterID,
				ExpenseItemID:   l.ExpenseItemID,
				Note:            l.Note,
			})
		}
	} else {
		vLines = append(vLines, glUseCase.CreateVoucherLineCommand{
			DebitAccountID:  cmd.DebitAccountID,
			CreditAccountID: bankGLAccountID,
			AmountVND:       cmd.PrincipalAmountVND,
			VendorID:        cmd.VendorID,
			EmployeeID:      cmd.EmployeeID,
			CostCenterID:    cmd.CostCenterID,
			ExpenseItemID:   cmd.ExpenseItemID,
			Note:            cmd.Description,
		})
	}

	// Automatic Fee line: Nợ 642 / Có 112
	if cmd.FeeAmountVND.GreaterThan(decimal.Zero) {
		feeNote := "Phí chuyển tiền ngân hàng"
		vLines = append(vLines, glUseCase.CreateVoucherLineCommand{
			DebitAccountID:  cmd.FeeAccountID,
			CreditAccountID: bankGLAccountID,
			AmountVND:       cmd.FeeAmountVND,
			CostCenterID:    cmd.CostCenterID,
			Note:            feeNote,
		})
	}

	// Automatic VAT Fee line: Nợ 13311 / Có 112
	if cmd.VatFeeAmountVND.GreaterThan(decimal.Zero) {
		vatNote := "Thuế GTGT phí chuyển tiền"
		vLines = append(vLines, glUseCase.CreateVoucherLineCommand{
			DebitAccountID:  cmd.VatFeeAccountID,
			CreditAccountID: bankGLAccountID,
			AmountVND:       cmd.VatFeeAmountVND,
			Note:            vatNote,
		})
	}

	// 4. Delegate Master GL Voucher Creation
	vCmd := glUseCase.CreateVoucherCommand{
		CompanyProfileID: cmd.CompanyProfileID,
		BranchID:         cmd.BranchID,
		VoucherNo:        cmd.VoucherNo,
		VoucherDate:      cmd.VoucherDate,
		PostedDate:       cmd.PostedDate,
		VoucherType:      gl.VoucherTypeBankPayment,
		Description:      cmd.Description,
		CurrencyCode:     cmd.CurrencyCode,
		ExchangeRate:     cmd.ExchangeRate,
		IdempotencyKey:   cmd.IdempotencyKey,
		CreatedBy:        cmd.CreatedBy,
		Lines:            vLines,
	}

	voucher, err := u.glUseCase.CreateVoucher(ctx, vCmd)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create master bank payment voucher: %w", err)
	}

	// 5. Idempotent check
	existing, err := u.bankRepo.GetBankTransactionByVoucherID(ctx, voucher.ID)
	if err == nil && existing != nil {
		return existing, voucher, nil
	}

	// 6. Construct domain BankTransaction
	bt, err := cash.NewBankTransaction(cash.CreateBankTransactionParams{
		ID:               uuid.New().String(),
		VoucherID:        voucher.ID,
		CompanyProfileID: cmd.CompanyProfileID,
		BankAccountID:    cmd.BankAccountID,
		TransactionType:  cmd.TransactionType,
		Counterparty:     cmd.Counterparty,
		FeeAmountVND:     cmd.FeeAmountVND,
		VatFeeAmountVND:  cmd.VatFeeAmountVND,
		TotalAmountVND:   totalAmount,
	})
	if err != nil {
		return nil, nil, err
	}

	// 7. Persist BankTransaction
	if err := u.bankRepo.SaveBankTransaction(ctx, bt); err != nil {
		return nil, nil, fmt.Errorf("failed to persist bank payment transaction: %w", err)
	}

	u.logger.Info(ctx, "bank payment created successfully",
		slog.String("entity_id", bt.ID),
		slog.String("voucher_id", voucher.ID),
		slog.String("voucher_no", voucher.VoucherNo),
		slog.String("amount", bt.TotalAmountVND.String()),
	)

	return bt, voucher, nil
}

func (u *BankUseCase) GetBankTransaction(ctx context.Context, id string) (*cash.BankTransaction, error) {
	bt, err := u.bankRepo.GetBankTransactionByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("%w: %s", ErrBankTransactionNotFound, id)
	}
	return bt, nil
}

func (u *BankUseCase) resolveBankGLAccount(ctx context.Context, bankAccountID, currencyCode string, rate decimal.Decimal) (string, error) {
	if u.bankAccRepo == nil {
		return "", nil
	}
	ba, err := u.bankAccRepo.GetBankAccountByID(ctx, bankAccountID)
	if err != nil {
		return "", fmt.Errorf("%w: %s", ErrBankAccountNotFound, bankAccountID)
	}

	// INV-OPS-06: Check linked GL account and currency alignment
	if u.accRepo != nil {
		glAcc, err := u.accRepo.GetAccountByID(ctx, ba.GLAccountID)
		if err != nil {
			return "", fmt.Errorf("%w: bank GL account %s", ErrAccountNotFound, ba.GLAccountID)
		}
		if glAcc.Code == "1121" {
			if currencyCode != "" && currencyCode != "VND" {
				return "", cash.ErrBaseCurrencyMismatch
			}
		} else if glAcc.Code == "1122" {
			if currencyCode == "VND" || (currencyCode != "" && rate.LessThanOrEqual(decimal.Zero)) {
				return "", cash.ErrForeignCurrencyMismatch
			}
		}
	}

	return ba.GLAccountID, nil
}

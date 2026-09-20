package gl

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"fingo/internal/domain/gl"
	spineDomain "fingo/internal/domain/spine"
	"fingo/internal/domain/system"
	"fingo/pkg/logger"
)

var (
	ErrVoucherNotFound        = errors.New("voucher not found")
	ErrAccountNotFound        = errors.New("posting account not found in chart of accounts")
	ErrNonLeafPostingAccount  = errors.New("cannot post to non-leaf parent account (INV-OPS-02)")
	ErrLockDateVerification   = errors.New("failed to verify accounting period lock date")
)

type CompanyRepo interface {
	GetProfile(ctx context.Context) (*system.ProductionCompanyProfile, error)
}

type AccountRepo interface {
	GetAccountByID(ctx context.Context, id string) (*spineDomain.Account, error)
}

type CreateVoucherLineCommand struct {
	DebitAccountID  string          `json:"debit_account_id"`
	CreditAccountID string          `json:"credit_account_id"`
	AmountFC        decimal.Decimal `json:"amount_fc"`
	AmountVND       decimal.Decimal `json:"amount_vnd"`
	Note            string          `json:"note"`
	CustomerID      *string         `json:"customer_id,omitempty"`
	VendorID        *string         `json:"vendor_id,omitempty"`
	EmployeeID      *string         `json:"employee_id,omitempty"`
	ItemID          *string         `json:"item_id,omitempty"`
	WarehouseID     *string         `json:"warehouse_id,omitempty"`
	CostCenterID    *string         `json:"cost_center_id,omitempty"`
	ExpenseItemID   *string         `json:"expense_item_id,omitempty"`
	InvoiceNo       *string         `json:"invoice_no,omitempty"`
	InvoiceDate     *time.Time      `json:"invoice_date,omitempty"`
}

type CreateVoucherCommand struct {
	CompanyProfileID   string                     `json:"company_profile_id"`
	BranchID           *string                    `json:"branch_id,omitempty"`
	VoucherNo          string                     `json:"voucher_no"`
	VoucherDate        time.Time                  `json:"voucher_date"`
	PostedDate         time.Time                  `json:"posted_date"`
	VoucherType        gl.VoucherType             `json:"voucher_type"`
	Description        string                     `json:"description"`
	CurrencyCode       string                     `json:"currency_code"`
	ExchangeRate       decimal.Decimal            `json:"exchange_rate"`
	SourceDocumentID   *string                    `json:"source_document_id,omitempty"`
	SourceDocumentType *string                    `json:"source_document_type,omitempty"`
	IdempotencyKey     *string                    `json:"idempotency_key,omitempty"`
	CreatedBy          string                     `json:"created_by"`
	Lines              []CreateVoucherLineCommand `json:"lines"`
}

type GLUseCase struct {
	glRepo      gl.GLRepository
	companyRepo CompanyRepo
	accRepo     AccountRepo
	logger      *logger.Logger
}

func NewGLUseCase(
	glRepo gl.GLRepository,
	companyRepo CompanyRepo,
	accRepo AccountRepo,
	log *logger.Logger,
) *GLUseCase {
	return &GLUseCase{
		glRepo:      glRepo,
		companyRepo: companyRepo,
		accRepo:     accRepo,
		logger:      log,
	}
}

func (u *GLUseCase) CreateVoucher(ctx context.Context, cmd CreateVoucherCommand) (*gl.Voucher, error) {
	// 0. Idempotency Check (INV-OPS-14)
	if cmd.IdempotencyKey != nil && *cmd.IdempotencyKey != "" {
		existing, err := u.glRepo.GetVoucherByIdempotencyKey(ctx, cmd.CompanyProfileID, *cmd.IdempotencyKey)
		if err == nil && existing != nil {
			u.logger.Info(ctx, "voucher already exists (idempotent submission)",
				slog.String("voucher_id", existing.ID),
				slog.String("idempotency_key", *cmd.IdempotencyKey),
			)
			return existing, nil
		}
	}

	// 1. Period Lock Date Check (INV-OPS-04)
	if err := u.assertPeriodNotLocked(ctx, cmd.VoucherDate); err != nil {
		return nil, err
	}

	// 2. Validate Posting Accounts (INV-OPS-02)
	domainLines := make([]gl.VoucherLine, len(cmd.Lines))
	for i, l := range cmd.Lines {
		debitAccCode := ""
		creditAccCode := ""

		if u.accRepo != nil {
			drAcc, err := u.accRepo.GetAccountByID(ctx, l.DebitAccountID)
			if err != nil {
				return nil, fmt.Errorf("%w: debit account ID %s", ErrAccountNotFound, l.DebitAccountID)
			}
			if err := spineDomain.ValidatePostingAccount(drAcc); err != nil {
				return nil, fmt.Errorf("%w: debit account %s (%s)", ErrNonLeafPostingAccount, drAcc.Code, err)
			}
			debitAccCode = drAcc.Code

			crAcc, err := u.accRepo.GetAccountByID(ctx, l.CreditAccountID)
			if err != nil {
				return nil, fmt.Errorf("%w: credit account ID %s", ErrAccountNotFound, l.CreditAccountID)
			}
			if err := spineDomain.ValidatePostingAccount(crAcc); err != nil {
				return nil, fmt.Errorf("%w: credit account %s (%s)", ErrNonLeafPostingAccount, crAcc.Code, err)
			}
			creditAccCode = crAcc.Code
		}

		domainLines[i] = gl.VoucherLine{
			ID:                uuid.New().String(),
			LineOrder:         i + 1,
			DebitAccountID:    l.DebitAccountID,
			CreditAccountID:   l.CreditAccountID,
			DebitAccountCode:  debitAccCode,
			CreditAccountCode: creditAccCode,
			AmountFC:          l.AmountFC,
			AmountVND:         l.AmountVND,
			Note:              l.Note,
			CustomerID:        l.CustomerID,
			VendorID:          l.VendorID,
			EmployeeID:        l.EmployeeID,
			ItemID:            l.ItemID,
			WarehouseID:       l.WarehouseID,
			CostCenterID:      l.CostCenterID,
			ExpenseItemID:     l.ExpenseItemID,
			InvoiceNo:         l.InvoiceNo,
			InvoiceDate:       l.InvoiceDate,
			CreatedAt:         time.Now(),
		}
	}

	voucherID := uuid.New().String()
	voucherNo := cmd.VoucherNo
	if voucherNo == "" {
		voucherNo = fmt.Sprintf("PKT-%s-%s", cmd.VoucherDate.Format("200601"), voucherID[:8])
	}

	v, err := gl.NewVoucher(gl.CreateVoucherParams{
		ID:                 voucherID,
		CompanyProfileID:   cmd.CompanyProfileID,
		BranchID:           cmd.BranchID,
		VoucherNo:          voucherNo,
		VoucherDate:        cmd.VoucherDate,
		PostedDate:         cmd.PostedDate,
		VoucherType:        cmd.VoucherType,
		Description:        cmd.Description,
		CurrencyCode:       cmd.CurrencyCode,
		ExchangeRate:       cmd.ExchangeRate,
		SourceDocumentID:   cmd.SourceDocumentID,
		SourceDocumentType: cmd.SourceDocumentType,
		IdempotencyKey:     cmd.IdempotencyKey,
		CreatedBy:          cmd.CreatedBy,
		Lines:              domainLines,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to construct domain voucher: %w", err)
	}

	if err := v.ValidateBalance(); err != nil {
		return nil, err
	}

	if err := u.glRepo.SaveVoucher(ctx, v); err != nil {
		return nil, fmt.Errorf("failed to persist voucher: %w", err)
	}

	u.logger.Info(ctx, "voucher created successfully",
		slog.String("voucher_id", v.ID),
		slog.String("voucher_no", v.VoucherNo),
		slog.String("total_vnd", v.TotalDebit.String()),
	)

	return v, nil
}

func (u *GLUseCase) PostVoucher(ctx context.Context, id string) error {
	v, err := u.glRepo.GetVoucherByID(ctx, id)
	if err != nil {
		return fmt.Errorf("%w: %s", ErrVoucherNotFound, id)
	}

	if err := u.assertPeriodNotLocked(ctx, v.VoucherDate); err != nil {
		return err
	}

	if err := v.Post(); err != nil {
		return err
	}

	if err := u.glRepo.UpdateVoucherStatus(ctx, v.ID, gl.VoucherStatusPosted); err != nil {
		return fmt.Errorf("failed to update voucher status to POSTED: %w", err)
	}

	u.logger.Info(ctx, "voucher posted successfully", slog.String("voucher_id", v.ID), slog.String("voucher_no", v.VoucherNo))
	return nil
}

func (u *GLUseCase) CancelVoucher(ctx context.Context, id string) error {
	v, err := u.glRepo.GetVoucherByID(ctx, id)
	if err != nil {
		return fmt.Errorf("%w: %s", ErrVoucherNotFound, id)
	}

	if err := u.assertPeriodNotLocked(ctx, v.VoucherDate); err != nil {
		return err
	}

	if err := v.Cancel(); err != nil {
		return err
	}

	if err := u.glRepo.UpdateVoucherStatus(ctx, v.ID, gl.VoucherStatusCancelled); err != nil {
		return fmt.Errorf("failed to update voucher status to CANCELLED: %w", err)
	}

	u.logger.Info(ctx, "voucher cancelled successfully", slog.String("voucher_id", v.ID), slog.String("voucher_no", v.VoucherNo))
	return nil
}

func (u *GLUseCase) ReverseVoucher(ctx context.Context, id, newVoucherNo string, reverseDate time.Time, createdBy string) (*gl.Voucher, error) {
	v, err := u.glRepo.GetVoucherByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("%w: %s", ErrVoucherNotFound, id)
	}

	if err := u.assertPeriodNotLocked(ctx, reverseDate); err != nil {
		return nil, err
	}

	revVoucher, err := v.CreateReversingVoucher(uuid.New().String(), newVoucherNo, reverseDate, createdBy)
	if err != nil {
		return nil, fmt.Errorf("failed to create reversing voucher: %w", err)
	}

	if err := u.glRepo.SaveVoucher(ctx, revVoucher); err != nil {
		return nil, fmt.Errorf("failed to save reversing voucher: %w", err)
	}

	u.logger.Info(ctx, "reversing voucher created (storno)",
		slog.String("original_voucher_id", v.ID),
		slog.String("reversing_voucher_id", revVoucher.ID),
		slog.String("reversing_voucher_no", revVoucher.VoucherNo),
	)

	return revVoucher, nil
}

func (u *GLUseCase) GetVoucherByID(ctx context.Context, id string) (*gl.Voucher, error) {
	v, err := u.glRepo.GetVoucherByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("%w: %s", ErrVoucherNotFound, id)
	}
	return v, nil
}

func (u *GLUseCase) GetVoucherByNo(ctx context.Context, companyID string, vType gl.VoucherType, voucherNo string) (*gl.Voucher, error) {
	v, err := u.glRepo.GetVoucherByNo(ctx, companyID, vType, voucherNo)
	if err != nil {
		return nil, fmt.Errorf("%w: %s (type: %s)", ErrVoucherNotFound, voucherNo, vType)
	}
	return v, nil
}

func (u *GLUseCase) ListVouchersByPeriod(ctx context.Context, companyID string, fromDate, toDate time.Time) ([]gl.Voucher, error) {
	return u.glRepo.ListVouchersByPeriod(ctx, companyID, fromDate, toDate)
}

func (u *GLUseCase) assertPeriodNotLocked(ctx context.Context, vDate time.Time) error {
	if u.companyRepo == nil {
		return nil
	}
	profile, err := u.companyRepo.GetProfile(ctx)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		return fmt.Errorf("%w: %v", ErrLockDateVerification, err)
	}
	if profile != nil && !profile.LockDate.IsZero() {
		if !vDate.After(profile.LockDate) {
			return fmt.Errorf("%w: transaction date %s is on or before lock date %s",
				gl.ErrPeriodLocked,
				vDate.Format("2006-01-02"),
				profile.LockDate.Format("2006-01-02"),
			)
		}
	}
	return nil
}

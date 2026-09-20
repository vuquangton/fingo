package catalog

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	domain "fingo/internal/domain/catalog"
	spineDomain "fingo/internal/domain/spine"
	"fingo/pkg/logger"
)

var (
	ErrNonCashBankDetailsRequired = errors.New("non-cash payment statutory violation: transactions >= 5,000,000 VND strictly mandate vendor bank account details (Decree 181/2025/NĐ-CP & Law 48/2024/QH15)")
	ErrCustomerNotFound           = errors.New("customer not found")
	ErrVendorNotFound             = errors.New("vendor not found")
	ErrItemNotFound               = errors.New("item not found")
	ErrWarehouseNotFound          = errors.New("warehouse not found")
	ErrBankAccountNotFound        = errors.New("bank account not found")
	ErrEmployeeNotFound           = errors.New("employee not found")
	ErrUOMNotFound                = errors.New("unit of measure not found")
	ErrConversionNotFound         = errors.New("unit of measure conversion multiplier not found")
	ErrInvalidPostingAccount      = errors.New("default account must be an active leaf account")
)

type RegisterCustomerCommand struct {
	IdempotencyKey     string          `json:"idempotency_key,omitempty"`
	CompanyProfileID   string          `json:"company_profile_id"`
	Code               string          `json:"code"`
	Name               string          `json:"name"`
	TaxCode            string          `json:"tax_code,omitempty"`
	Address            string          `json:"address,omitempty"`
	Phone              string          `json:"phone,omitempty"`
	Email              string          `json:"email,omitempty"`
	ContactPerson      string          `json:"contact_person,omitempty"`
	PaymentTermDays    int             `json:"payment_term_days"`
	CreditLimit        decimal.Decimal `json:"credit_limit"`
	EnforceCreditLimit bool            `json:"enforce_credit_limit"`
	DefaultARAccountID string          `json:"default_ar_account_id"`
}

type RegisterVendorCommand struct {
	IdempotencyKey     string `json:"idempotency_key,omitempty"`
	CompanyProfileID   string `json:"company_profile_id"`
	Code               string `json:"code"`
	Name               string `json:"name"`
	TaxCode            string `json:"tax_code,omitempty"`
	Address            string `json:"address,omitempty"`
	Phone              string `json:"phone,omitempty"`
	Email              string `json:"email,omitempty"`
	ContactPerson      string `json:"contact_person,omitempty"`
	BankAccountNumber  string `json:"bank_account_number,omitempty"`
	BankName           string `json:"bank_name,omitempty"`
	BankBranch         string `json:"bank_branch,omitempty"`
	PaymentTermDays    int    `json:"payment_term_days"`
	DefaultAPAccountID string `json:"default_ap_account_id"`
}

type ConversionParam struct {
	FromUOMID      string                 `json:"from_uom_id"`
	ToUOMID        string                 `json:"to_uom_id"`
	Multiplier     decimal.Decimal        `json:"multiplier"`
	ConversionType domain.ConversionType  `json:"conversion_type"`
}

type RegisterItemCommand struct {
	IdempotencyKey     string            `json:"idempotency_key,omitempty"`
	CompanyProfileID   string            `json:"company_profile_id"`
	Code               string            `json:"code"`
	Name               string            `json:"name"`
	Barcode            string            `json:"barcode,omitempty"`
	ItemType           domain.ItemType   `json:"item_type"`
	BaseUOMID          string            `json:"base_uom_id"`
	DefaultWarehouseID *string           `json:"default_warehouse_id,omitempty"`
	InventoryAccountID *string           `json:"inventory_account_id,omitempty"`
	COGSAccountID      string            `json:"cogs_account_id"`
	RevenueAccountID   string            `json:"revenue_account_id"`
	DefaultVATRate     decimal.Decimal   `json:"default_vat_rate"`
	StandardCostPrice  decimal.Decimal   `json:"standard_cost_price"`
	StandardSalePrice  decimal.Decimal   `json:"standard_sale_price"`
	Conversions        []ConversionParam `json:"conversions,omitempty"`
}

type RegisterWarehouseCommand struct {
	IdempotencyKey   string  `json:"idempotency_key,omitempty"`
	CompanyProfileID string  `json:"company_profile_id"`
	BranchID         *string `json:"branch_id,omitempty"`
	Code             string  `json:"code"`
	Name             string  `json:"name"`
	Address          string  `json:"address,omitempty"`
	DefaultAccountID string  `json:"default_account_id"`
}

type RegisterBankAccountCommand struct {
	IdempotencyKey   string  `json:"idempotency_key,omitempty"`
	CompanyProfileID string  `json:"company_profile_id"`
	BranchID         *string `json:"branch_id,omitempty"`
	AccountNumber    string  `json:"account_number"`
	BankName         string  `json:"bank_name"`
	BankCode         string  `json:"bank_code"`
	BranchName       string  `json:"branch_name,omitempty"`
	CurrencyCode     string  `json:"currency_code"`
	GLAccountID      string  `json:"gl_account_id"`
}

type RegisterEmployeeCommand struct {
	IdempotencyKey      string          `json:"idempotency_key,omitempty"`
	CompanyProfileID    string          `json:"company_profile_id"`
	BranchID            *string         `json:"branch_id,omitempty"`
	Code                string          `json:"code"`
	FullName            string          `json:"full_name"`
	Department          string          `json:"department"`
	Position            string          `json:"position"`
	CitizenID           string          `json:"citizen_id"`
	TaxCode             string          `json:"tax_code,omitempty"`
	SocialInsuranceNo   string          `json:"social_insurance_no"`
	BaseSalary          decimal.Decimal `json:"base_salary"`
	SalaryCoefficient   decimal.Decimal `json:"salary_coefficient"`
	BankAccountNumber   string          `json:"bank_account_number,omitempty"`
	BankName            string          `json:"bank_name,omitempty"`
	DefaultAdvanceAccID string          `json:"default_advance_acc_id"`
	DefaultPayrollAccID string          `json:"default_payroll_acc_id"`
}

type CatalogUseCase struct {
	uomRepo     domain.UOMRepository
	whRepo      domain.WarehouseRepository
	bankRepo    domain.BankAccountRepository
	custRepo    domain.CustomerRepository
	vendorRepo  domain.VendorRepository
	itemRepo    domain.ItemRepository
	empRepo     domain.EmployeeRepository
	accRepo     spineDomain.AccountRepository
	logger      *logger.Logger
}

func NewCatalogUseCase(
	uomRepo domain.UOMRepository,
	whRepo domain.WarehouseRepository,
	bankRepo domain.BankAccountRepository,
	custRepo domain.CustomerRepository,
	vendorRepo domain.VendorRepository,
	itemRepo domain.ItemRepository,
	empRepo domain.EmployeeRepository,
	accRepo spineDomain.AccountRepository,
	appLogger *logger.Logger,
) *CatalogUseCase {
	if appLogger == nil {
		appLogger = logger.New(logger.Config{Level: slog.LevelInfo, Format: logger.FormatJSON})
	}
	return &CatalogUseCase{
		uomRepo:    uomRepo,
		whRepo:     whRepo,
		bankRepo:   bankRepo,
		custRepo:   custRepo,
		vendorRepo: vendorRepo,
		itemRepo:   itemRepo,
		empRepo:    empRepo,
		accRepo:    accRepo,
		logger:     appLogger,
	}
}

// RegisterCustomer handles customer master data creation with validation
func (u *CatalogUseCase) RegisterCustomer(ctx context.Context, cmd RegisterCustomerCommand) (*domain.Customer, error) {
	// 1. Idempotency check: if code already registered, return existing
	if existing, err := u.custRepo.GetCustomerByCode(ctx, cmd.CompanyProfileID, cmd.Code); err == nil && existing != nil {
		u.logger.Info(ctx, "customer already exists (idempotent submission)",
			slog.String("company_id", cmd.CompanyProfileID),
			slog.String("code", cmd.Code),
		)
		return existing, nil
	}

	// 2. Validate DefaultARAccount is active leaf account in COA (INV-CAT-01)
	if u.accRepo != nil && cmd.DefaultARAccountID != "" {
		acc, err := u.accRepo.GetAccountByID(ctx, cmd.DefaultARAccountID)
		if err != nil {
			return nil, fmt.Errorf("%w: account not found for AR account ID %s", ErrInvalidPostingAccount, cmd.DefaultARAccountID)
		}
		if err := spineDomain.ValidatePostingAccount(acc); err != nil {
			return nil, fmt.Errorf("%w: AR account %s: %v", ErrInvalidPostingAccount, acc.Code, err)
		}
	}

	id := uuid.New().String()
	cust, err := domain.NewCustomer(
		id, cmd.CompanyProfileID, cmd.Code, cmd.Name, cmd.TaxCode,
		cmd.Address, cmd.Phone, cmd.Email, cmd.ContactPerson,
		cmd.PaymentTermDays, cmd.CreditLimit, cmd.EnforceCreditLimit,
		cmd.DefaultARAccountID, "",
	)
	if err != nil {
		return nil, err
	}

	if err := u.custRepo.SaveCustomer(ctx, cust); err != nil {
		return nil, fmt.Errorf("failed to persist customer: %w", err)
	}

	u.logger.Info(ctx, "customer successfully registered",
		slog.String("company_id", cmd.CompanyProfileID),
		slog.String("code", cmd.Code),
		slog.String("name", cmd.Name),
	)

	return cust, nil
}

// RegisterVendor handles vendor master data creation with validation
func (u *CatalogUseCase) RegisterVendor(ctx context.Context, cmd RegisterVendorCommand) (*domain.Vendor, error) {
	// 1. Idempotency check
	if existing, err := u.vendorRepo.GetVendorByCode(ctx, cmd.CompanyProfileID, cmd.Code); err == nil && existing != nil {
		u.logger.Info(ctx, "vendor already exists (idempotent submission)",
			slog.String("company_id", cmd.CompanyProfileID),
			slog.String("code", cmd.Code),
		)
		return existing, nil
	}

	// 2. Validate DefaultAPAccount is active leaf account in COA (INV-CAT-01)
	if u.accRepo != nil && cmd.DefaultAPAccountID != "" {
		acc, err := u.accRepo.GetAccountByID(ctx, cmd.DefaultAPAccountID)
		if err != nil {
			return nil, fmt.Errorf("%w: account not found for AP account ID %s", ErrInvalidPostingAccount, cmd.DefaultAPAccountID)
		}
		if err := spineDomain.ValidatePostingAccount(acc); err != nil {
			return nil, fmt.Errorf("%w: AP account %s: %v", ErrInvalidPostingAccount, acc.Code, err)
		}
	}

	id := uuid.New().String()
	v, err := domain.NewVendor(
		id, cmd.CompanyProfileID, cmd.Code, cmd.Name, cmd.TaxCode,
		cmd.Address, cmd.Phone, cmd.Email, cmd.ContactPerson,
		cmd.BankAccountNumber, cmd.BankName, cmd.BankBranch,
		cmd.PaymentTermDays, cmd.DefaultAPAccountID, "",
	)
	if err != nil {
		return nil, err
	}

	if err := u.vendorRepo.SaveVendor(ctx, v); err != nil {
		return nil, fmt.Errorf("failed to persist vendor: %w", err)
	}

	u.logger.Info(ctx, "vendor successfully registered",
		slog.String("company_id", cmd.CompanyProfileID),
		slog.String("code", cmd.Code),
		slog.String("name", cmd.Name),
	)

	return v, nil
}

// RegisterItem handles item creation and multi-UOM conversion table setup
func (u *CatalogUseCase) RegisterItem(ctx context.Context, cmd RegisterItemCommand) (*domain.Item, error) {
	// 1. Idempotency check
	if existing, err := u.itemRepo.GetItemByCode(ctx, cmd.CompanyProfileID, cmd.Code); err == nil && existing != nil {
		return existing, nil
	}

	// 2. Validate Base UOM
	if _, err := u.uomRepo.GetUOMByID(ctx, cmd.BaseUOMID); err != nil {
		return nil, fmt.Errorf("%w: base UOM ID %s", ErrUOMNotFound, cmd.BaseUOMID)
	}

	// 3. Validate Accounts if AccountRepository available
	if u.accRepo != nil {
		if cmd.InventoryAccountID != nil && *cmd.InventoryAccountID != "" {
			acc, err := u.accRepo.GetAccountByID(ctx, *cmd.InventoryAccountID)
			if err != nil || spineDomain.ValidatePostingAccount(acc) != nil {
				return nil, fmt.Errorf("%w: invalid inventory account", ErrInvalidPostingAccount)
			}
		}
		cogsAcc, err := u.accRepo.GetAccountByID(ctx, cmd.COGSAccountID)
		if err != nil || spineDomain.ValidatePostingAccount(cogsAcc) != nil {
			return nil, fmt.Errorf("%w: invalid COGS account", ErrInvalidPostingAccount)
		}
		revAcc, err := u.accRepo.GetAccountByID(ctx, cmd.RevenueAccountID)
		if err != nil || spineDomain.ValidatePostingAccount(revAcc) != nil {
			return nil, fmt.Errorf("%w: invalid revenue account", ErrInvalidPostingAccount)
		}
	}

	id := uuid.New().String()
	item, err := domain.NewItem(
		id, cmd.CompanyProfileID, cmd.Code, cmd.Name, cmd.Barcode,
		cmd.ItemType, cmd.BaseUOMID, cmd.DefaultWarehouseID,
		cmd.InventoryAccountID, cmd.COGSAccountID, cmd.RevenueAccountID,
		cmd.DefaultVATRate, cmd.StandardCostPrice, cmd.StandardSalePrice,
	)
	if err != nil {
		return nil, err
	}

	if err := u.itemRepo.SaveItem(ctx, item); err != nil {
		return nil, fmt.Errorf("failed to persist item: %w", err)
	}

	// 4. Save optional conversion definitions
	for _, cp := range cmd.Conversions {
		convID := uuid.New().String()
		conv, err := domain.NewUOMConversion(
			convID, cmd.CompanyProfileID, &id,
			cp.FromUOMID, cp.ToUOMID, cp.Multiplier, cp.ConversionType,
		)
		if err != nil {
			return nil, fmt.Errorf("invalid conversion configuration: %w", err)
		}
		if err := u.uomRepo.SaveConversion(ctx, conv); err != nil {
			return nil, fmt.Errorf("failed to save uom conversion: %w", err)
		}
	}

	u.logger.Info(ctx, "item successfully registered",
		slog.String("company_id", cmd.CompanyProfileID),
		slog.String("code", cmd.Code),
		slog.String("type", string(cmd.ItemType)),
	)

	return item, nil
}

// RegisterWarehouse registers a new warehouse with inventory account COA validation (INV-CAT-01, INV-CAT-07)
func (u *CatalogUseCase) RegisterWarehouse(ctx context.Context, cmd RegisterWarehouseCommand) (*domain.Warehouse, error) {
	if existing, err := u.whRepo.GetWarehouseByCode(ctx, cmd.CompanyProfileID, cmd.Code); err == nil && existing != nil {
		return existing, nil
	}

	var accountCode string
	if u.accRepo != nil {
		acc, err := u.accRepo.GetAccountByID(ctx, cmd.DefaultAccountID)
		if err != nil {
			return nil, fmt.Errorf("%w: inventory account %s", ErrInvalidPostingAccount, cmd.DefaultAccountID)
		}
		if err := spineDomain.ValidatePostingAccount(acc); err != nil {
			return nil, fmt.Errorf("%w: warehouse account %s: %v", ErrInvalidPostingAccount, acc.Code, err)
		}
		accountCode = acc.Code
	}

	id := uuid.New().String()
	wh, err := domain.NewWarehouse(
		id, cmd.CompanyProfileID, cmd.BranchID,
		cmd.Code, cmd.Name, cmd.Address,
		cmd.DefaultAccountID, accountCode,
	)
	if err != nil {
		return nil, err
	}

	if err := u.whRepo.SaveWarehouse(ctx, wh); err != nil {
		return nil, fmt.Errorf("failed to persist warehouse: %w", err)
	}

	u.logger.Info(ctx, "warehouse successfully registered",
		slog.String("company_id", cmd.CompanyProfileID),
		slog.String("code", cmd.Code),
	)

	return wh, nil
}

// RegisterBankAccount registers a bank account with GL alignment validation (INV-CAT-01, INV-CAT-06)
func (u *CatalogUseCase) RegisterBankAccount(ctx context.Context, cmd RegisterBankAccountCommand) (*domain.BankAccount, error) {
	if existing, err := u.bankRepo.GetBankAccountByNumber(ctx, cmd.CompanyProfileID, cmd.AccountNumber); err == nil && existing != nil {
		return existing, nil
	}

	var accountCode string
	if u.accRepo != nil {
		acc, err := u.accRepo.GetAccountByID(ctx, cmd.GLAccountID)
		if err != nil {
			return nil, fmt.Errorf("%w: gl account %s", ErrInvalidPostingAccount, cmd.GLAccountID)
		}
		if err := spineDomain.ValidatePostingAccount(acc); err != nil {
			return nil, fmt.Errorf("%w: bank gl account %s: %v", ErrInvalidPostingAccount, acc.Code, err)
		}
		accountCode = acc.Code
	}

	id := uuid.New().String()
	bankAcc, err := domain.NewBankAccount(
		id, cmd.CompanyProfileID, cmd.BranchID,
		cmd.AccountNumber, cmd.BankName, cmd.BankCode, cmd.BranchName,
		cmd.CurrencyCode, cmd.GLAccountID, accountCode,
	)
	if err != nil {
		return nil, err
	}

	if err := u.bankRepo.SaveBankAccount(ctx, bankAcc); err != nil {
		return nil, fmt.Errorf("failed to persist bank account: %w", err)
	}

	u.logger.Info(ctx, "bank account successfully registered",
		slog.String("company_id", cmd.CompanyProfileID),
		slog.String("account_number", cmd.AccountNumber),
	)

	return bankAcc, nil
}

// RegisterEmployee registers an employee with statutory CCCD, BHXH and leaf account validation (INV-CAT-01, INV-CAT-08)
func (u *CatalogUseCase) RegisterEmployee(ctx context.Context, cmd RegisterEmployeeCommand) (*domain.Employee, error) {
	if existing, err := u.empRepo.GetEmployeeByCode(ctx, cmd.CompanyProfileID, cmd.Code); err == nil && existing != nil {
		return existing, nil
	}

	if u.accRepo != nil {
		advAcc, err := u.accRepo.GetAccountByID(ctx, cmd.DefaultAdvanceAccID)
		if err != nil || spineDomain.ValidatePostingAccount(advAcc) != nil {
			return nil, fmt.Errorf("%w: invalid advance account", ErrInvalidPostingAccount)
		}
		payAcc, err := u.accRepo.GetAccountByID(ctx, cmd.DefaultPayrollAccID)
		if err != nil || spineDomain.ValidatePostingAccount(payAcc) != nil {
			return nil, fmt.Errorf("%w: invalid payroll account", ErrInvalidPostingAccount)
		}
	}

	id := uuid.New().String()
	emp, err := domain.NewEmployee(
		id, cmd.CompanyProfileID, cmd.BranchID,
		cmd.Code, cmd.FullName, cmd.Department, cmd.Position,
		cmd.CitizenID, cmd.TaxCode, cmd.SocialInsuranceNo,
		cmd.BaseSalary, cmd.SalaryCoefficient,
		cmd.BankAccountNumber, cmd.BankName,
		cmd.DefaultAdvanceAccID, cmd.DefaultPayrollAccID,
	)
	if err != nil {
		return nil, err
	}

	if err := u.empRepo.SaveEmployee(ctx, emp); err != nil {
		return nil, fmt.Errorf("failed to persist employee: %w", err)
	}

	u.logger.Info(ctx, "employee successfully registered",
		slog.String("company_id", cmd.CompanyProfileID),
		slog.String("code", cmd.Code),
	)

	return emp, nil
}

// ConvertItemQuantity converts item quantity using defined conversion multipliers
func (u *CatalogUseCase) ConvertItemQuantity(
	ctx context.Context,
	companyID, itemID string,
	fromUOMID, toUOMID string,
	qty decimal.Decimal,
	decimals int32,
) (decimal.Decimal, error) {
	if fromUOMID == toUOMID {
		return qty.RoundBank(decimals), nil
	}

	// 1. Try forward conversion
	conv, err := u.uomRepo.GetConversion(ctx, companyID, &itemID, fromUOMID, toUOMID)
	if err == nil && conv != nil {
		return conv.ConvertQuantity(qty, decimals)
	}

	// 2. Try reverse conversion (using domain Invert method)
	revConv, err := u.uomRepo.GetConversion(ctx, companyID, &itemID, toUOMID, fromUOMID)
	if err == nil && revConv != nil {
		return revConv.Invert().ConvertQuantity(qty, decimals)
	}

	return decimal.Zero, fmt.Errorf("%w: from %s to %s for item %s", ErrConversionNotFound, fromUOMID, toUOMID, itemID)
}

// ValidateCustomerCreditLimit checks if an incoming transaction breaches authorized credit ceiling (INV-CAT-05)
func (u *CatalogUseCase) ValidateCustomerCreditLimit(
	ctx context.Context,
	companyID, customerID string,
	currentOutstandingAR, newVoucherAmount decimal.Decimal,
) error {
	cust, err := u.custRepo.GetCustomerByID(ctx, customerID)
	if err != nil {
		return fmt.Errorf("%w: %s", ErrCustomerNotFound, customerID)
	}
	return cust.CheckCreditLimit(currentOutstandingAR, newVoucherAmount)
}

// ValidateVendorNonCashPayment enforces Decree 181/2025/NĐ-CP & Law 48/2024/QH15 for transactions >= 5,000,000 VND
func (u *CatalogUseCase) ValidateVendorNonCashPayment(
	ctx context.Context,
	companyID, vendorID string,
	amount decimal.Decimal,
) error {
	threshold := decimal.RequireFromString("5000000")
	if amount.LessThan(threshold) {
		return nil
	}

	v, err := u.vendorRepo.GetVendorByID(ctx, vendorID)
	if err != nil {
		return fmt.Errorf("%w: %s", ErrVendorNotFound, vendorID)
	}

	if !v.HasValidBankAccountForNonCash() {
		return fmt.Errorf("%w: vendor %s (%s) has no registered bank account for settlement of %s VND",
			ErrNonCashBankDetailsRequired, v.Code, v.Name, amount.String())
	}

	return nil
}

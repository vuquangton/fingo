package system

import (
	"errors"
	"strings"
	"time"
)

type AccountingRegime string

const (
	RegimeCircular133 AccountingRegime = "TT133_2016" // Chế độ kế toán Doanh nghiệp nhỏ và vừa
	RegimeCircular99  AccountingRegime = "TT99_2025"  // Chế độ kế toán Doanh nghiệp mới (thay thế TT200)
)

type VATMethod string

const (
	VATMethodDeduction VATMethod = "DEDUCTION" // Khấu trừ thuế
	VATMethodDirect    VATMethod = "DIRECT"    // Trực tiếp trên doanh thu
)

type CostingMethod string

const (
	CostingFIFO           CostingMethod = "FIFO"
	CostingMovingWeighted CostingMethod = "MOVING_WEIGHTED_AVG"
	CostingPeriodic       CostingMethod = "PERIODIC_AVG"
)

type BusinessType string

const (
	BusinessTrading    BusinessType = "TRADING"    // Thương mại
	BusinessService    BusinessType = "SERVICE"    // Dịch vụ
	BusinessProduction BusinessType = "PRODUCTION" // Sản xuất
	BusinessMix        BusinessType = "MIXED"      // Hỗn hợp
)

type TaxAuthority struct {
	Code     string `json:"code"`
	Name     string `json:"name"`
	CityCode string `json:"city_code,omitempty"`
}

type BankAccountRegistration struct {
	AccountNumber string `json:"account_number"`
	BankName      string `json:"bank_name"`
	BankBranch    string `json:"bank_branch"`
	BankSwiftCode string `json:"bank_swift_code,omitempty"`
	IsTaxPayment  bool   `json:"is_tax_payment"`
}

type EInvoiceConfig struct {
	ProviderCode  string    `json:"provider_code"`
	TaxServiceURL string    `json:"tax_service_url"`
	CertSerial    string    `json:"cert_serial"`
	CertSubjectDN string    `json:"cert_subject_dn"`
	CertValidTo   time.Time `json:"cert_valid_to"`
	AutoSendToTCT bool      `json:"auto_send_to_tct"`
}

type CreateCompanyProfileParams struct {
	ID                     string
	TaxCode                string
	LegalName              string
	TradeName              string
	EnglishName            string
	Address                string
	ProvinceCity           string
	DistrictWard           string
	Phone                  string
	Email                  string
	Website                string
	LegalRepresentative    string
	RepresentativePosition string
	ChiefAccountant        string
	TaxAuthorityCode       string
	TaxAuthorityName       string
	StateBudgetChapter     string
	Regime                 AccountingRegime
	VATMethod              VATMethod
	CostingMethod          CostingMethod
	BusinessType           BusinessType
}

type ProductionCompanyProfile struct {
	ID                     string
	TaxCode                string
	LegalName              string
	TradeName              string
	EnglishName            string
	Address                string
	ProvinceCity           string
	DistrictWard           string
	Phone                  string
	Email                  string
	Website                string
	LegalRepresentative    string
	RepresentativePosition string
	ChiefAccountant        string
	TaxAuthority           TaxAuthority
	StateBudgetChapter     string
	Regime                 AccountingRegime
	BaseCurrency           string
	FiscalYearStartMonth   time.Month
	VATMethod              VATMethod
	CostingMethod          CostingMethod
	BusinessType           BusinessType
	RegisteredBanks        []BankAccountRegistration
	EInvoice               EInvoiceConfig
	IsActive               bool
	LockDate               time.Time
	CreatedAt              time.Time
	UpdatedAt              time.Time
}

var (
	ErrEmptyLegalName       = errors.New("legal name cannot be empty")
	ErrEmptyTaxAuthority    = errors.New("tax authority code and name are required")
	ErrEmptyRepresentative  = errors.New("legal representative is required")
	ErrEmptyChiefAccountant = errors.New("chief accountant is required")
	ErrVoucherLocked        = errors.New("voucher date is on or before accounting lock date")
)

// NewProductionCompanyProfile constructs and validates a production company profile entity
func NewProductionCompanyProfile(p CreateCompanyProfileParams) (*ProductionCompanyProfile, error) {
	if err := ValidateTaxCode(p.TaxCode); err != nil {
		return nil, err
	}

	cleanLegalName := strings.TrimSpace(p.LegalName)
	if cleanLegalName == "" {
		return nil, ErrEmptyLegalName
	}

	cleanTaxAuthCode := strings.TrimSpace(p.TaxAuthorityCode)
	cleanTaxAuthName := strings.TrimSpace(p.TaxAuthorityName)
	if cleanTaxAuthCode == "" || cleanTaxAuthName == "" {
		return nil, ErrEmptyTaxAuthority
	}

	cleanRep := strings.TrimSpace(p.LegalRepresentative)
	if cleanRep == "" {
		return nil, ErrEmptyRepresentative
	}

	cleanChief := strings.TrimSpace(p.ChiefAccountant)
	if cleanChief == "" {
		return nil, ErrEmptyChiefAccountant
	}

	now := time.Now()

	return &ProductionCompanyProfile{
		ID:                     p.ID,
		TaxCode:                strings.TrimSpace(p.TaxCode),
		LegalName:              cleanLegalName,
		TradeName:              strings.TrimSpace(p.TradeName),
		EnglishName:            strings.TrimSpace(p.EnglishName),
		Address:                strings.TrimSpace(p.Address),
		ProvinceCity:           strings.TrimSpace(p.ProvinceCity),
		DistrictWard:           strings.TrimSpace(p.DistrictWard),
		Phone:                  strings.TrimSpace(p.Phone),
		Email:                  strings.TrimSpace(p.Email),
		Website:                strings.TrimSpace(p.Website),
		LegalRepresentative:    cleanRep,
		RepresentativePosition: strings.TrimSpace(p.RepresentativePosition),
		ChiefAccountant:        cleanChief,
		TaxAuthority: TaxAuthority{
			Code: cleanTaxAuthCode,
			Name: cleanTaxAuthName,
		},
		StateBudgetChapter:   strings.TrimSpace(p.StateBudgetChapter),
		Regime:               p.Regime,
		BaseCurrency:         "VND",
		FiscalYearStartMonth: time.January,
		VATMethod:            p.VATMethod,
		CostingMethod:        p.CostingMethod,
		BusinessType:         p.BusinessType,
		RegisteredBanks:      make([]BankAccountRegistration, 0),
		IsActive:             true,
		CreatedAt:            now,
		UpdatedAt:            now,
	}, nil
}

// SetLockDate sets the accounting lock date
func (p *ProductionCompanyProfile) SetLockDate(t time.Time) {
	p.LockDate = t
	p.UpdatedAt = time.Now()
}

// CheckVoucherDate enforces accounting lock date rules
func (p *ProductionCompanyProfile) CheckVoucherDate(voucherDate time.Time) error {
	if !p.LockDate.IsZero() && !voucherDate.After(p.LockDate) {
		return ErrVoucherLocked
	}
	return nil
}

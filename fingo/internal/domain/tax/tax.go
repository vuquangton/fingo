package tax

import (
	"context"
	"time"

	"github.com/shopspring/decimal"
)

type TaxPeriodType string

const (
	PeriodMonth   TaxPeriodType = "MONTH"
	PeriodQuarter TaxPeriodType = "QUARTER"
)

type EInvoiceStatus string

const (
	EInvoiceStatusDraft     EInvoiceStatus = "DRAFT"
	EInvoiceStatusSigned    EInvoiceStatus = "SIGNED"
	EInvoiceStatusSubmitted EInvoiceStatus = "SUBMITTED"
	EInvoiceStatusApproved  EInvoiceStatus = "APPROVED"
	EInvoiceStatusRejected  EInvoiceStatus = "REJECTED"
	EInvoiceStatusCanceled  EInvoiceStatus = "CANCELED"
)

type EInvoice struct {
	ID                 string
	VoucherID          string
	TemplateCode       string // e.g. 1/001
	Series             string // e.g. C26TAV
	InvoiceNumber      string // e.g. 00000123
	IssueDate          time.Time
	BuyerTaxCode       string
	BuyerLegalName     string
	TotalAmountWithoutVAT decimal.Decimal
	VATAmount          decimal.Decimal
	TotalAmountWithVAT decimal.Decimal
	Status             EInvoiceStatus
	TaxAuthorityCode   string // Mã CQT cấp
}

func NewEInvoiceStub(id, template, series, invNo string, issueDate time.Time) *EInvoice {
	return &EInvoice{
		ID:            id,
		TemplateCode:  template,
		Series:        series,
		InvoiceNumber: invNo,
		IssueDate:     issueDate,
		Status:        EInvoiceStatusDraft,
	}
}

type VATDeclaration struct {
	ID                 string
	Year               int
	Period             int
	PeriodType         TaxPeriodType
	DeductibleInputVAT decimal.Decimal // Nợ 1331 khấu trừ
	OutputVAT          decimal.Decimal // Có 33311 phát sinh
	CarriedForwardPrev decimal.Decimal // Thuế GTGT còn được khấu trừ kỳ trước chuyển sang
	IsSubmitted        bool
}

func NewVATDeclarationStub(id string, year, period int, pType TaxPeriodType) *VATDeclaration {
	return &VATDeclaration{
		ID:                 id,
		Year:               year,
		Period:             period,
		PeriodType:         pType,
		DeductibleInputVAT: decimal.Zero,
		OutputVAT:          decimal.Zero,
		CarriedForwardPrev: decimal.Zero,
		IsSubmitted:        false,
	}
}

// CalculateTaxPosition computes VAT payable (Chỉ tiêu 40) or VAT carried forward (Chỉ tiêu 43)
func (v *VATDeclaration) CalculateTaxPosition() (taxPayable, carryForward decimal.Decimal) {
	totalInput := v.DeductibleInputVAT.Add(v.CarriedForwardPrev)
	diff := v.OutputVAT.Sub(totalInput)

	if diff.IsPositive() {
		return diff, decimal.Zero
	}
	return decimal.Zero, diff.Abs()
}

type TaxRepositoryStub interface {
	SaveEInvoice(ctx context.Context, einv *EInvoice) error
	GetEInvoice(ctx context.Context, id string) (*EInvoice, error)
	SaveVATDeclaration(ctx context.Context, decl *VATDeclaration) error
}

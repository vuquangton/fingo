package payroll

import (
	"context"

	"github.com/shopspring/decimal"
)

type Employee struct {
	ID                 string
	Code               string
	FullName           string
	DepartmentID       string
	TaxCode            string
	SocialSecurityNo   string
	BaseSalary         decimal.Decimal
	NumberOfDependents int
	IsActive           bool
}

type PayrollItem struct {
	EmployeeID  string
	GrossSalary decimal.Decimal
	PIT         decimal.Decimal // Personal income tax (Thuế TNCN)
}

func NewPayrollItemStub(empID string, gross decimal.Decimal) *PayrollItem {
	return &PayrollItem{
		EmployeeID:  empID,
		GrossSalary: gross,
		PIT:         decimal.Zero,
	}
}

// CalculateEmployeeInsurance returns statutory 10.5% (BHXH 8%, BHYT 1.5%, BHTN 1%)
func (p *PayrollItem) CalculateEmployeeInsurance() decimal.Decimal {
	rate := decimal.NewFromFloat(0.105)
	return p.GrossSalary.Mul(rate)
}

// CalculateEmployerInsurance returns statutory 21.5% (BHXH 17%, BHYT 3%, BHTN 1%, KPCĐ 2%)
func (p *PayrollItem) CalculateEmployerInsurance() decimal.Decimal {
	rate := decimal.NewFromFloat(0.215)
	return p.GrossSalary.Mul(rate)
}

func (p *PayrollItem) CalculateNetSalary() decimal.Decimal {
	ins := p.CalculateEmployeeInsurance()
	return p.GrossSalary.Sub(ins).Sub(p.PIT)
}

type PayrollTable struct {
	ID    string
	Year  int
	Month int
	Items []PayrollItem
}

type PayrollRepositoryStub interface {
	SavePayrollTable(ctx context.Context, table *PayrollTable) error
	GetEmployee(ctx context.Context, id string) (*Employee, error)
}

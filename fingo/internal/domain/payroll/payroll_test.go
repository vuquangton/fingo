package payroll_test

import (
	"testing"

	"github.com/shopspring/decimal"

	"fingo/internal/domain/payroll"
)

func TestPayrollItem_NetSalaryCalculation(t *testing.T) {
	item := payroll.NewPayrollItemStub("emp-1", decimal.NewFromInt(20000000))

	// Base 20,000,000 VND
	// Statutory mandatory employee deductions: BHXH 8%, BHYT 1.5%, BHTN 1% = 10.5% (2,100,000)
	// Net before tax = 17,900,000 VND
	employeeIns := item.CalculateEmployeeInsurance()
	expectedIns := decimal.NewFromInt(2100000)

	if employeeIns.Cmp(expectedIns) != 0 {
		t.Fatalf("expected employee insurance %s, got %s", expectedIns, employeeIns)
	}

	net := item.CalculateNetSalary()
	expectedNet := decimal.NewFromInt(17900000)
	if net.Cmp(expectedNet) != 0 {
		t.Fatalf("expected net salary %s, got %s", expectedNet, net)
	}
}

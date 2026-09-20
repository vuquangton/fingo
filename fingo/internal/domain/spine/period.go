package spine

import (
	"errors"
	"fmt"
	"time"
)

var (
	ErrPeriodLocked          = errors.New("cannot post or modify transaction in locked accounting period")
	ErrInvalidPeriodDates    = errors.New("start date must be before or equal to end date")
	ErrPeriodAlreadyClosed   = errors.New("period is already closed")
	ErrInvalidPeriodNumber   = errors.New("period number must be between 1 and 13")
	ErrInvalidFiscalYearDate = errors.New("fiscal year start date must precede end date")
)

type FiscalYearStatus string

const (
	FiscalYearStatusOpen       FiscalYearStatus = "OPEN"
	FiscalYearStatusSoftLocked FiscalYearStatus = "SOFT_LOCKED"
	FiscalYearStatusHardLocked FiscalYearStatus = "HARD_LOCKED"
	FiscalYearStatusClosed     FiscalYearStatus = "CLOSED"
)

// FiscalYear represents a 12-month accounting fiscal year under Law 88/2015
type FiscalYear struct {
	ID               string           `json:"id"`
	CompanyProfileID string           `json:"company_profile_id"`
	Year             int              `json:"year"`
	StartDate        time.Time        `json:"start_date"`
	EndDate          time.Time        `json:"end_date"`
	Status           FiscalYearStatus `json:"status"`
	CreatedAt        time.Time        `json:"created_at"`
	UpdatedAt        time.Time        `json:"updated_at"`
}

func NewFiscalYear(id, companyID string, year int, startDate, endDate time.Time) (*FiscalYear, error) {
	if startDate.After(endDate) {
		return nil, ErrInvalidFiscalYearDate
	}

	return &FiscalYear{
		ID:               id,
		CompanyProfileID: companyID,
		Year:             year,
		StartDate:        startDate,
		EndDate:          endDate,
		Status:           FiscalYearStatusOpen,
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
	}, nil
}

type PeriodStatus string

const (
	PeriodStatusOpen       PeriodStatus = "OPEN"
	PeriodStatusSoftLocked PeriodStatus = "SOFT_LOCKED"
	PeriodStatusHardLocked PeriodStatus = "HARD_LOCKED"
	PeriodStatusAudited    PeriodStatus = "AUDITED"
)

// AccountingPeriod represents an individual monthly accounting period (or Period 13 audit adjustment)
type AccountingPeriod struct {
	ID               string       `json:"id"`
	FiscalYearID     string       `json:"fiscal_year_id"`
	CompanyProfileID string       `json:"company_profile_id"`
	PeriodNumber     int          `json:"period_number"` // 1-12, 13 (Adjustment)
	Name             string       `json:"name"`
	StartDate        time.Time    `json:"start_date"`
	EndDate          time.Time    `json:"end_date"`
	LockDate         time.Time    `json:"lock_date"` // Transactions with VoucherDate <= LockDate are strictly rejected
	Status           PeriodStatus `json:"status"`
	CreatedAt        time.Time    `json:"created_at"`
	UpdatedAt        time.Time    `json:"updated_at"`
}

func NewAccountingPeriod(
	id, fyID, companyID string,
	pNum int,
	name string,
	startDate, endDate, lockDate time.Time,
) (*AccountingPeriod, error) {
	if pNum < 1 || pNum > 13 {
		return nil, fmt.Errorf("%w: %d", ErrInvalidPeriodNumber, pNum)
	}
	if startDate.After(endDate) {
		return nil, ErrInvalidPeriodDates
	}

	return &AccountingPeriod{
		ID:               id,
		FiscalYearID:     fyID,
		CompanyProfileID: companyID,
		PeriodNumber:     pNum,
		Name:             name,
		StartDate:        startDate,
		EndDate:          endDate,
		LockDate:         lockDate,
		Status:           PeriodStatusOpen,
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
	}, nil
}

// CheckPeriodLock enforces INV-SPINE-04: rejects transaction if VoucherDate <= LockDate
func CheckPeriodLock(voucherDate, lockDate time.Time) error {
	if lockDate.IsZero() {
		return nil
	}

	// Truncate both to start-of-day for calendar date comparison
	vDay := time.Date(voucherDate.Year(), voucherDate.Month(), voucherDate.Day(), 0, 0, 0, 0, time.UTC)
	lDay := time.Date(lockDate.Year(), lockDate.Month(), lockDate.Day(), 0, 0, 0, 0, time.UTC)

	if !vDay.After(lDay) { // vDay <= lDay
		return fmt.Errorf("%w: voucher date (%s) is on or before lock date (%s)",
			ErrPeriodLocked, vDay.Format("2006-01-02"), lDay.Format("2006-01-02"))
	}
	return nil
}

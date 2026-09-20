package system

import (
	"context"
	"time"
)

type CompanyProfile struct {
	ID                   string
	CompanyName          string
	TaxCode              string
	Address              string
	LegalRepresentative  string
	ChiefAccountant      string
	CurrencyCode         string
	FiscalYearStartMonth time.Month
	AccountingStandard   string // TT133 or TT200
}

type AuditLog struct {
	ID        string
	UserID    string
	Action    string
	Entity    string
	EntityID  string
	OldValue  string
	NewValue  string
	CreatedAt time.Time
}

func NewUserStub(id, username, email string) *User {
	return &User{
		ID:        id,
		Username:  username,
		Email:     email,
		Status:    UserStatusActive,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
}

func NewCompanyProfileStub(id, name, taxCode string) *CompanyProfile {
	return &CompanyProfile{
		ID:                   id,
		CompanyName:          name,
		TaxCode:              taxCode,
		CurrencyCode:         "VND",
		FiscalYearStartMonth: time.January,
		AccountingStandard:   "TT133",
	}
}

type SystemRepositoryStub interface {
	GetUser(ctx context.Context, id string) (*User, error)
	GetCompanyProfile(ctx context.Context) (*CompanyProfile, error)
	LogAudit(ctx context.Context, log *AuditLog) error
}

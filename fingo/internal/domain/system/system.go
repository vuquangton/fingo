package system

import (
	"context"
	"time"
)

type RoleType string

const (
	RoleAdmin           RoleType = "ADMIN"
	RoleChiefAccountant RoleType = "CHIEF_ACCOUNTANT"
	RoleGeneralLedger   RoleType = "GENERAL_LEDGER"
	RoleCashier         RoleType = "CASHIER"
	RoleSales           RoleType = "SALES"
	RoleWarehouse       RoleType = "WAREHOUSE"
)

type User struct {
	ID           string
	Username     string
	Email        string
	PasswordHash string
	FullName     string
	IsActive     bool
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type Role struct {
	ID          string
	Code        RoleType
	Name        string
	Description string
}

type Permission struct {
	ID       string
	Module   string
	Action   string // VIEW, ADD, EDIT, DELETE, POST, UNPOST, EXPORT
	Resource string
}

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
		IsActive:  true,
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

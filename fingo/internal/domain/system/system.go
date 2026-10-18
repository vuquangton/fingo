package system

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

// Regime identifies the accounting regime a client's books are kept under.
//
// Each client database carries exactly one regime. It drives the chart of
// accounts, the statutory forms and the books, so it is a closed set of known
// instruments rather than a free-form string. See docs/legal-basis.md.
type Regime string

const (
	// RegimeTT133 is Circular 133/2016/TT-BTC as amended by Circular
	// 46/2025/TT-BTC — the SME regime. This is what FinGo v1 implements.
	RegimeTT133 Regime = "TT133"

	// RegimeTT99 is Circular 99/2025/TT-BTC, the general regime, which SMEs
	// may elect. Modelled so the seam exists; not yet implemented.
	RegimeTT99 Regime = "TT99"

	// RegimeTT58_2026 is Circular 58/2026/TT-BTC, the micro-enterprise regime,
	// which prescribes books but no chart of accounts. Modelled; not yet
	// implemented.
	RegimeTT58_2026 Regime = "TT58_2026"
)

func (r Regime) Valid() bool {
	switch r {
	case RegimeTT133, RegimeTT99, RegimeTT58_2026:
		return true
	}
	return false
}

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
	Regime               Regime
}

// Validate checks the profile carries enough to keep books and file returns.
func (c CompanyProfile) Validate() error {
	if strings.TrimSpace(c.TaxCode) == "" {
		return errors.New("company profile requires a tax code")
	}
	if !c.Regime.Valid() {
		return fmt.Errorf("company profile has an unsupported accounting regime %q", c.Regime)
	}
	return nil
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
		Regime:               RegimeTT133,
	}
}

type SystemRepositoryStub interface {
	GetUser(ctx context.Context, id string) (*User, error)
	GetCompanyProfile(ctx context.Context) (*CompanyProfile, error)
	LogAudit(ctx context.Context, log *AuditLog) error
}

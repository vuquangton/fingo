package system

import (
	"context"
	"time"
)

// CompanyProfileRepository defines the deep persistence seam for company profile
type CompanyProfileRepository interface {
	GetProfile(ctx context.Context) (*ProductionCompanyProfile, error)
	SaveProfile(ctx context.Context, profile *ProductionCompanyProfile) error
	SetLockDate(ctx context.Context, id string, lockDate time.Time) error
}

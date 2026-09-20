package system

import (
	"context"
)

// BranchOrgUnitRepository defines the deep persistence seam for organizational units
type BranchOrgUnitRepository interface {
	CreateOrgUnit(ctx context.Context, unit *BranchOrgUnit) error
	GetOrgUnitByID(ctx context.Context, id string) (*BranchOrgUnit, error)
	GetOrgUnitByCode(ctx context.Context, companyID, code string) (*BranchOrgUnit, error)
	ListOrgUnitsByCompany(ctx context.Context, companyID string) ([]BranchOrgUnit, error)
	UpdateOrgUnit(ctx context.Context, unit *BranchOrgUnit) error
}

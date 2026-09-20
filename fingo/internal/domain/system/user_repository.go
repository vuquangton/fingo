package system

import (
	"context"
	"time"
)

// UserRepository defines the persistence seam for User identity, roles, and scoping
type UserRepository interface {
	CreateUser(ctx context.Context, user *User) error
	GetUserByID(ctx context.Context, id string) (*User, error)
	GetUserByUsername(ctx context.Context, companyID, username string) (*User, error)
	GetUserByEmail(ctx context.Context, companyID, email string) (*User, error)
	ListUsersByCompany(ctx context.Context, companyID string) ([]User, error)
	UpdateUserStatus(ctx context.Context, id string, status UserStatus, lockoutUntil *time.Time, failedAttempts int) error
	UpdateUserPassword(ctx context.Context, id, passwordHash string, changedAt time.Time, mustChange bool) error
	RecordFailedLogin(ctx context.Context, id string, failedAttempts int, status UserStatus, lockoutUntil *time.Time) error
	RecordSuccessfulLogin(ctx context.Context, id, ip string, loginAt time.Time) error
	AssignRole(ctx context.Context, userID, roleID, assignedBy string) error
	RemoveRole(ctx context.Context, userID, roleID string) error
	GetUserRoles(ctx context.Context, userID string) ([]Role, error)
	AssignOrgUnitScope(ctx context.Context, scope *UserOrgUnitScope) error
	GetUserOrgUnitScopes(ctx context.Context, userID string) ([]UserOrgUnitScope, error)
	RemoveOrgUnitScope(ctx context.Context, userID, orgUnitID string) error
}

// RoleRepository defines the persistence seam for Role catalog lookups
type RoleRepository interface {
	ListRoles(ctx context.Context) ([]Role, error)
	GetRoleByCode(ctx context.Context, code SystemRoleCode) (*Role, error)
}

package repository

import (
	"context"
	"database/sql"
	"time"

	sqlc "fingo/internal/adapter/mariadb/sqlc"
	"fingo/internal/domain/system"
)

type UserRepo struct {
	db      *sql.DB
	queries *sqlc.Queries
}

func NewUserRepo(db *sql.DB) *UserRepo {
	return &UserRepo{
		db:      db,
		queries: sqlc.New(db),
	}
}

func (r *UserRepo) CreateUser(ctx context.Context, u *system.User) error {
	var lockout sql.NullTime
	if u.LockoutUntil != nil {
		lockout = sql.NullTime{Time: *u.LockoutUntil, Valid: true}
	}

	arg := sqlc.CreateUserParams{
		ID:                     u.ID,
		CompanyProfileID:       u.CompanyProfileID,
		Username:               u.Username,
		Email:                  u.Email,
		PasswordHash:           u.PasswordHash,
		FullName:               u.FullName,
		Title:                  toNullString(u.Title),
		EmployeeCode:           toNullString(u.EmployeeCode),
		Status:                 sqlc.UsersStatus(u.Status),
		FailedLoginAttempts:    int32(u.FailedLoginAttempts),
		LockoutUntil:           lockout,
		PasswordChangedAt:      u.PasswordChangedAt,
		MustChangePasswordNext: u.MustChangePasswordNext,
		MfaSecret:              toNullString(u.MFASecret),
		IsMfaEnabled:           u.IsMFAEnabled,
		DigitalCertSubject:     toNullString(u.DigitalCertSubject),
		DigitalCertSerial:      toNullString(u.DigitalCertSerial),
	}

	return r.queries.CreateUser(ctx, arg)
}

func (r *UserRepo) GetUserByID(ctx context.Context, id string) (*system.User, error) {
	row, err := r.queries.GetUserByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return mapRowToUser(row), nil
}

func (r *UserRepo) GetUserByUsername(ctx context.Context, companyID, username string) (*system.User, error) {
	row, err := r.queries.GetUserByUsername(ctx, sqlc.GetUserByUsernameParams{
		CompanyProfileID: companyID,
		Username:         username,
	})
	if err != nil {
		return nil, err
	}
	return mapRowToUser(row), nil
}

func (r *UserRepo) GetUserByEmail(ctx context.Context, companyID, email string) (*system.User, error) {
	row, err := r.queries.GetUserByEmail(ctx, sqlc.GetUserByEmailParams{
		CompanyProfileID: companyID,
		Email:            email,
	})
	if err != nil {
		return nil, err
	}
	return mapRowToUser(row), nil
}

func (r *UserRepo) ListUsersByCompany(ctx context.Context, companyID string) ([]system.User, error) {
	rows, err := r.queries.ListUsersByCompany(ctx, companyID)
	if err != nil {
		return nil, err
	}

	users := make([]system.User, 0, len(rows))
	for _, row := range rows {
		users = append(users, *mapRowToUser(row))
	}
	return users, nil
}

func (r *UserRepo) UpdateUserStatus(ctx context.Context, id string, status system.UserStatus, lockoutUntil *time.Time, failedAttempts int) error {
	var lockout sql.NullTime
	if lockoutUntil != nil {
		lockout = sql.NullTime{Time: *lockoutUntil, Valid: true}
	}

	return r.queries.UpdateUserStatus(ctx, sqlc.UpdateUserStatusParams{
		ID:                  id,
		Status:              sqlc.UsersStatus(status),
		LockoutUntil:        lockout,
		FailedLoginAttempts: int32(failedAttempts),
	})
}

func (r *UserRepo) UpdateUserPassword(ctx context.Context, id, passwordHash string, changedAt time.Time, mustChange bool) error {
	return r.queries.UpdateUserPassword(ctx, sqlc.UpdateUserPasswordParams{
		ID:                     id,
		PasswordHash:           passwordHash,
		PasswordChangedAt:      changedAt,
		MustChangePasswordNext: mustChange,
	})
}

func (r *UserRepo) RecordFailedLogin(ctx context.Context, id string, failedAttempts int, status system.UserStatus, lockoutUntil *time.Time) error {
	var lockout sql.NullTime
	if lockoutUntil != nil {
		lockout = sql.NullTime{Time: *lockoutUntil, Valid: true}
	}

	return r.queries.UpdateUserFailedLogin(ctx, sqlc.UpdateUserFailedLoginParams{
		ID:                  id,
		FailedLoginAttempts: int32(failedAttempts),
		Status:              sqlc.UsersStatus(status),
		LockoutUntil:        lockout,
	})
}

func (r *UserRepo) RecordSuccessfulLogin(ctx context.Context, id, ip string, loginAt time.Time) error {
	return r.queries.UpdateUserSuccessfulLogin(ctx, sqlc.UpdateUserSuccessfulLoginParams{
		ID:          id,
		LastLoginAt: sql.NullTime{Time: loginAt, Valid: true},
		LastLoginIp: sql.NullString{String: ip, Valid: true},
	})
}

func (r *UserRepo) AssignRole(ctx context.Context, userID, roleID, assignedBy string) error {
	return r.queries.AssignUserRole(ctx, sqlc.AssignUserRoleParams{
		UserID:     userID,
		RoleID:     roleID,
		AssignedBy: toNullString(assignedBy),
	})
}

func (r *UserRepo) RemoveRole(ctx context.Context, userID, roleID string) error {
	return r.queries.RemoveUserRole(ctx, sqlc.RemoveUserRoleParams{
		UserID: userID,
		RoleID: roleID,
	})
}

func (r *UserRepo) GetUserRoles(ctx context.Context, userID string) ([]system.Role, error) {
	rows, err := r.queries.GetUserRoles(ctx, userID)
	if err != nil {
		return nil, err
	}

	roles := make([]system.Role, 0, len(rows))
	for _, row := range rows {
		roles = append(roles, mapRowToRole(row.ID, row.Code, row.Name, row.Description, row.IsSystem, row.CreatedAt, row.UpdatedAt))
	}
	return roles, nil
}

func (r *UserRepo) AssignOrgUnitScope(ctx context.Context, scope *system.UserOrgUnitScope) error {
	return r.queries.AssignUserOrgUnitScope(ctx, sqlc.AssignUserOrgUnitScopeParams{
		ID:              scope.ID,
		UserID:          scope.UserID,
		OrgUnitID:       scope.OrgUnitID,
		IsDefault:       scope.IsDefault,
		IncludeChildren: scope.IncludeChildren,
	})
}

func (r *UserRepo) GetUserOrgUnitScopes(ctx context.Context, userID string) ([]system.UserOrgUnitScope, error) {
	rows, err := r.queries.GetUserOrgUnitScopes(ctx, userID)
	if err != nil {
		return nil, err
	}

	scopes := make([]system.UserOrgUnitScope, 0, len(rows))
	for _, row := range rows {
		scopes = append(scopes, system.UserOrgUnitScope{
			ID:              row.ID,
			UserID:          row.UserID,
			OrgUnitID:       row.OrgUnitID,
			IsDefault:       row.IsDefault,
			IncludeChildren: row.IncludeChildren,
			CreatedAt:       row.CreatedAt,
		})
	}
	return scopes, nil
}

func (r *UserRepo) RemoveOrgUnitScope(ctx context.Context, userID, orgUnitID string) error {
	return r.queries.RemoveUserOrgUnitScope(ctx, sqlc.RemoveUserOrgUnitScopeParams{
		UserID:    userID,
		OrgUnitID: orgUnitID,
	})
}

func (r *UserRepo) ListRoles(ctx context.Context) ([]system.Role, error) {
	rows, err := r.queries.ListRoles(ctx)
	if err != nil {
		return nil, err
	}

	roles := make([]system.Role, 0, len(rows))
	for _, row := range rows {
		roles = append(roles, mapRowToRole(row.ID, row.Code, row.Name, row.Description, row.IsSystem, row.CreatedAt, row.UpdatedAt))
	}
	return roles, nil
}

func (r *UserRepo) GetRoleByCode(ctx context.Context, code system.SystemRoleCode) (*system.Role, error) {
	row, err := r.queries.GetRoleByCode(ctx, string(code))
	if err != nil {
		return nil, err
	}

	role := mapRowToRole(row.ID, row.Code, row.Name, row.Description, row.IsSystem, row.CreatedAt, row.UpdatedAt)
	return &role, nil
}

func mapRowToRole(id, code, name string, desc sql.NullString, isSystem bool, createdAt, updatedAt time.Time) system.Role {
	return system.Role{
		ID:          id,
		Code:        system.SystemRoleCode(code),
		Name:        name,
		Description: fromNullString(desc),
		IsSystem:    isSystem,
		CreatedAt:   createdAt,
		UpdatedAt:   updatedAt,
	}
}

func mapRowToUser(row sqlc.User) *system.User {
	var lockout *time.Time
	if row.LockoutUntil.Valid {
		t := row.LockoutUntil.Time
		lockout = &t
	}

	var lastLogin *time.Time
	if row.LastLoginAt.Valid {
		t := row.LastLoginAt.Time
		lastLogin = &t
	}

	return &system.User{
		ID:                     row.ID,
		CompanyProfileID:       row.CompanyProfileID,
		Username:               row.Username,
		Email:                  row.Email,
		PasswordHash:           row.PasswordHash,
		FullName:               row.FullName,
		Title:                  fromNullString(row.Title),
		EmployeeCode:           fromNullString(row.EmployeeCode),
		Status:                 system.UserStatus(row.Status),
		FailedLoginAttempts:    int(row.FailedLoginAttempts),
		LockoutUntil:           lockout,
		PasswordChangedAt:      row.PasswordChangedAt,
		MustChangePasswordNext: row.MustChangePasswordNext,
		MFASecret:              fromNullString(row.MfaSecret),
		IsMFAEnabled:           row.IsMfaEnabled,
		DigitalCertSubject:     fromNullString(row.DigitalCertSubject),
		DigitalCertSerial:      fromNullString(row.DigitalCertSerial),
		LastLoginAt:            lastLogin,
		LastLoginIP:            fromNullString(row.LastLoginIp),
		CreatedAt:              row.CreatedAt,
		UpdatedAt:              row.UpdatedAt,
	}
}

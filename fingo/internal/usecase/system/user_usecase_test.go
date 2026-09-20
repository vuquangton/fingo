package system_test

import (
	"context"
	"testing"
	"time"

	domain "fingo/internal/domain/system"
	usecase "fingo/internal/usecase/system"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
)

// In-Memory Repository Fakes for Usecase Testing
type mockUserRepo struct {
	users      map[string]*domain.User
	userRoles  map[string][]domain.Role
	userScopes map[string][]domain.UserOrgUnitScope
	roleRepo   *mockRoleRepo
}

func newMockUserRepo(roleRepo *mockRoleRepo) *mockUserRepo {
	return &mockUserRepo{
		users:      make(map[string]*domain.User),
		userRoles:  make(map[string][]domain.Role),
		userScopes: make(map[string][]domain.UserOrgUnitScope),
		roleRepo:   roleRepo,
	}
}

func (m *mockUserRepo) CreateUser(ctx context.Context, u *domain.User) error {
	m.users[u.ID] = u
	return nil
}

func (m *mockUserRepo) GetUserByID(ctx context.Context, id string) (*domain.User, error) {
	u, ok := m.users[id]
	if !ok {
		return nil, usecase.ErrUserNotFound
	}
	return u, nil
}

func (m *mockUserRepo) GetUserByUsername(ctx context.Context, companyID, username string) (*domain.User, error) {
	for _, u := range m.users {
		if u.CompanyProfileID == companyID && u.Username == username {
			return u, nil
		}
	}
	return nil, usecase.ErrUserNotFound
}

func (m *mockUserRepo) GetUserByEmail(ctx context.Context, companyID, email string) (*domain.User, error) {
	for _, u := range m.users {
		if u.CompanyProfileID == companyID && u.Email == email {
			return u, nil
		}
	}
	return nil, usecase.ErrUserNotFound
}

func (m *mockUserRepo) ListUsersByCompany(ctx context.Context, companyID string) ([]domain.User, error) {
	list := make([]domain.User, 0)
	for _, u := range m.users {
		if u.CompanyProfileID == companyID {
			list = append(list, *u)
		}
	}
	return list, nil
}

func (m *mockUserRepo) UpdateUserStatus(ctx context.Context, id string, status domain.UserStatus, lockoutUntil *time.Time, failedAttempts int) error {
	u, ok := m.users[id]
	if !ok {
		return usecase.ErrUserNotFound
	}
	u.Status = status
	u.LockoutUntil = lockoutUntil
	u.FailedLoginAttempts = failedAttempts
	return nil
}

func (m *mockUserRepo) UpdateUserPassword(ctx context.Context, id, passwordHash string, changedAt time.Time, mustChange bool) error {
	u, ok := m.users[id]
	if !ok {
		return usecase.ErrUserNotFound
	}
	u.PasswordHash = passwordHash
	u.PasswordChangedAt = changedAt
	u.MustChangePasswordNext = mustChange
	return nil
}

func (m *mockUserRepo) RecordFailedLogin(ctx context.Context, id string, failedAttempts int, status domain.UserStatus, lockoutUntil *time.Time) error {
	u, ok := m.users[id]
	if !ok {
		return usecase.ErrUserNotFound
	}
	u.FailedLoginAttempts = failedAttempts
	u.Status = status
	u.LockoutUntil = lockoutUntil
	return nil
}

func (m *mockUserRepo) RecordSuccessfulLogin(ctx context.Context, id, ip string, loginAt time.Time) error {
	u, ok := m.users[id]
	if !ok {
		return usecase.ErrUserNotFound
	}
	u.FailedLoginAttempts = 0
	u.LockoutUntil = nil
	if u.Status == domain.UserStatusLocked {
		u.Status = domain.UserStatusActive
	}
	u.LastLoginIP = ip
	u.LastLoginAt = &loginAt
	return nil
}

func (m *mockUserRepo) AssignRole(ctx context.Context, userID, roleID, assignedBy string) error {
	for _, r := range m.roleRepo.roles {
		if r.ID == roleID {
			m.userRoles[userID] = append(m.userRoles[userID], *r)
			return nil
		}
	}
	return nil
}

func (m *mockUserRepo) RemoveRole(ctx context.Context, userID, roleID string) error {
	roles := m.userRoles[userID]
	filtered := make([]domain.Role, 0)
	for _, r := range roles {
		if r.ID != roleID {
			filtered = append(filtered, r)
		}
	}
	m.userRoles[userID] = filtered
	return nil
}

func (m *mockUserRepo) GetUserRoles(ctx context.Context, userID string) ([]domain.Role, error) {
	return m.userRoles[userID], nil
}

func (m *mockUserRepo) AssignOrgUnitScope(ctx context.Context, scope *domain.UserOrgUnitScope) error {
	m.userScopes[scope.UserID] = append(m.userScopes[scope.UserID], *scope)
	return nil
}

func (m *mockUserRepo) GetUserOrgUnitScopes(ctx context.Context, userID string) ([]domain.UserOrgUnitScope, error) {
	return m.userScopes[userID], nil
}

func (m *mockUserRepo) RemoveOrgUnitScope(ctx context.Context, userID, orgUnitID string) error {
	scopes := m.userScopes[userID]
	filtered := make([]domain.UserOrgUnitScope, 0)
	for _, s := range scopes {
		if s.OrgUnitID != orgUnitID {
			filtered = append(filtered, s)
		}
	}
	m.userScopes[userID] = filtered
	return nil
}

type mockRoleRepo struct {
	roles map[domain.SystemRoleCode]*domain.Role
}

func newMockRoleRepo() *mockRoleRepo {
	m := &mockRoleRepo{roles: make(map[domain.SystemRoleCode]*domain.Role)}
	m.roles[domain.RoleChiefAccountant] = &domain.Role{ID: "role-ktt", Code: domain.RoleChiefAccountant, Name: "Kế toán trưởng"}
	m.roles[domain.RoleGeneralAccountant] = &domain.Role{ID: "role-ktth", Code: domain.RoleGeneralAccountant, Name: "Kế toán tổng hợp"}
	m.roles[domain.RoleCashier] = &domain.Role{ID: "role-tq", Code: domain.RoleCashier, Name: "Thủ quỹ"}
	return m
}

func (m *mockRoleRepo) ListRoles(ctx context.Context) ([]domain.Role, error) {
	list := make([]domain.Role, 0)
	for _, r := range m.roles {
		list = append(list, *r)
	}
	return list, nil
}

func (m *mockRoleRepo) GetRoleByCode(ctx context.Context, code domain.SystemRoleCode) (*domain.Role, error) {
	r, ok := m.roles[code]
	if !ok {
		return nil, usecase.ErrRoleNotFound
	}
	return r, nil
}

func TestUserUseCase_CreateUser_HappyPath(t *testing.T) {
	ctx := context.Background()
	roleRepo := newMockRoleRepo()
	userRepo := newMockUserRepo(roleRepo)
	uc := usecase.NewUserUseCase(userRepo, roleRepo)

	cmd := usecase.CreateUserCommand{
		CompanyProfileID: "comp-01",
		Username:         "ketoan_moi",
		Email:            "kt_moi@fingo.vn",
		Password:         "Password@123",
		FullName:         "Lê Thị Mới",
		Title:            "Kế toán viên",
		EmployeeCode:     "NV099",
	}

	created, err := uc.CreateUser(ctx, cmd)
	require.NoError(t, err)
	require.NotNil(t, created)
	assert.Equal(t, "ketoan_moi", created.Username)
	assert.Equal(t, domain.UserStatusPendingActivation, created.Status)

	// Verify bcrypt hash
	err = bcrypt.CompareHashAndPassword([]byte(created.PasswordHash), []byte("Password@123"))
	assert.NoError(t, err)

	// Duplicate username should fail
	_, err = uc.CreateUser(ctx, cmd)
	require.ErrorIs(t, err, usecase.ErrDuplicateUsername)
}

func TestUserUseCase_AssignRole_WithSoDEnforcement(t *testing.T) {
	ctx := context.Background()
	roleRepo := newMockRoleRepo()
	userRepo := newMockUserRepo(roleRepo)
	uc := usecase.NewUserUseCase(userRepo, roleRepo)

	// 1. Create User
	u, err := uc.CreateUser(ctx, usecase.CreateUserCommand{
		CompanyProfileID: "comp-01",
		Username:         "ketoan_sod",
		Email:            "sod@fingo.vn",
		Password:         "SecurePass123!",
		FullName:         "Phạm Văn SoD",
	})
	require.NoError(t, err)

	// 2. Assign General Accountant (Allowed)
	err = uc.AssignRole(ctx, u.ID, domain.RoleGeneralAccountant, "admin")
	require.NoError(t, err)

	// 3. Assign Cashier to same user -> MUST FAIL per Article 52 Law 88/2015
	err = uc.AssignRole(ctx, u.ID, domain.RoleCashier, "admin")
	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrSoDConflictViolation)
}

func TestUserUseCase_Authenticate_And_Lockout(t *testing.T) {
	ctx := context.Background()
	roleRepo := newMockRoleRepo()
	userRepo := newMockUserRepo(roleRepo)
	uc := usecase.NewUserUseCase(userRepo, roleRepo)

	// Create user
	u, err := uc.CreateUser(ctx, usecase.CreateUserCommand{
		CompanyProfileID: "comp-01",
		Username:         "auth_user",
		Email:            "auth@fingo.vn",
		Password:         "CorrectPassword123!",
		FullName:         "Hoàng Auth",
	})
	require.NoError(t, err)
	u.Activate()

	now := time.Date(2026, 3, 20, 10, 0, 0, 0, time.UTC)

	// Wrong password attempts (1 to 4)
	for i := 1; i <= 4; i++ {
		_, err := uc.Authenticate(ctx, "comp-01", "auth_user", "WrongPassword", "127.0.0.1", now)
		require.ErrorIs(t, err, usecase.ErrInvalidCredentials)
	}

	// 5th failed attempt -> locks account
	_, err = uc.Authenticate(ctx, "comp-01", "auth_user", "WrongPassword", "127.0.0.1", now)
	require.ErrorIs(t, err, usecase.ErrInvalidCredentials)

	// Now try correct password while locked -> returns ErrAccountLocked
	_, err = uc.Authenticate(ctx, "comp-01", "auth_user", "CorrectPassword123!", "127.0.0.1", now.Add(5*time.Minute))
	require.ErrorIs(t, err, domain.ErrUserLocked)

	// After 30 minutes, correct password succeeds
	authed, err := uc.Authenticate(ctx, "comp-01", "auth_user", "CorrectPassword123!", "127.0.0.1", now.Add(31*time.Minute))
	require.NoError(t, err)
	assert.Equal(t, domain.UserStatusActive, authed.Status)
	assert.Equal(t, 0, authed.FailedLoginAttempts)
}

func TestUserUseCase_ValidationAndScopes(t *testing.T) {
	ctx := context.Background()
	roleRepo := newMockRoleRepo()
	userRepo := newMockUserRepo(roleRepo)
	uc := usecase.NewUserUseCase(userRepo, roleRepo)

	// 1. Weak password
	_, err := uc.CreateUser(ctx, usecase.CreateUserCommand{
		CompanyProfileID: "comp-01",
		Username:         "user_weak",
		Email:            "weak@fingo.vn",
		Password:         "short",
		FullName:         "User Weak",
	})
	require.ErrorIs(t, err, usecase.ErrWeakPassword)

	// 2. Valid create
	u, err := uc.CreateUser(ctx, usecase.CreateUserCommand{
		CompanyProfileID: "comp-01",
		Username:         "user_scope",
		Email:            "scope@fingo.vn",
		Password:         "ValidPass123!",
		FullName:         "User Scope",
	})
	require.NoError(t, err)

	// 3. Duplicate email
	_, err = uc.CreateUser(ctx, usecase.CreateUserCommand{
		CompanyProfileID: "comp-01",
		Username:         "user_scope_2",
		Email:            "scope@fingo.vn",
		Password:         "ValidPass123!",
		FullName:         "User Scope 2",
	})
	require.ErrorIs(t, err, usecase.ErrDuplicateEmail)

	// 4. Assign OrgUnit Scope
	err = uc.AssignOrgUnitScope(ctx, usecase.AssignScopeCommand{
		UserID:          u.ID,
		OrgUnitID:       "branch-hcm-01",
		IsDefault:       true,
		IncludeChildren: true,
	})
	require.NoError(t, err)

	scopes, err := userRepo.GetUserOrgUnitScopes(ctx, u.ID)
	require.NoError(t, err)
	require.Len(t, scopes, 1)
	assert.Equal(t, "branch-hcm-01", scopes[0].OrgUnitID)
	assert.True(t, scopes[0].IsDefault)
	assert.True(t, scopes[0].IncludeChildren)
}

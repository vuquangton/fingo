package repository_test

import (
	"context"
	"testing"
	"time"

	"fingo/internal/adapter/mariadb/repository"
	"fingo/internal/domain/system"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUserRepo_CRUD_And_Roles(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	companyRepo := repository.NewCompanyProfileRepo(db)
	userRepo := repository.NewUserRepo(db)
	ctx := context.Background()

	// 1. Ensure company exists
	activeComp, err := companyRepo.GetProfile(ctx)
	var companyID string
	if err == nil && activeComp != nil {
		companyID = activeComp.ID
	} else {
		companyID = "test-cp-user-01"
		comp, err := system.NewProductionCompanyProfile(system.CreateCompanyProfileParams{
			ID:                  companyID,
			TaxCode:             "0101243150",
			LegalName:           "CÔNG TY TEST USER REPO",
			Address:             "Hà Nội",
			LegalRepresentative: "Giám Đốc",
			ChiefAccountant:     "Kế Toán Trưởng",
			TaxAuthorityCode:    "10500",
			TaxAuthorityName:    "Cục Thuế Cầu Giấy",
			Regime:              system.RegimeCircular133,
			VATMethod:           system.VATMethodDeduction,
			CostingMethod:       system.CostingMovingWeighted,
			BusinessType:        system.BusinessTrading,
		})
		require.NoError(t, err)
		require.NoError(t, companyRepo.SaveProfile(ctx, comp))
	}

	// 2. Create User
	userID := "test-user-db-01"
	username := "test_ktt_01"
	email := "test_ktt@fingo.vn"
	u, err := system.NewUser(
		userID,
		companyID,
		username,
		email,
		"$2a$12$somevalidhash",
		"Trần Văn Kế Toán",
		"Kế toán trưởng",
		"NV_TEST_01",
	)
	require.NoError(t, err)

	// Clean up if already exists from prior run
	_, _ = db.ExecContext(ctx, "DELETE FROM user_roles WHERE user_id = ?", userID)
	_, _ = db.ExecContext(ctx, "DELETE FROM users WHERE id = ? OR username = ?", userID, username)

	err = userRepo.CreateUser(ctx, u)
	require.NoError(t, err)

	// 3. Get User By ID
	fetched, err := userRepo.GetUserByID(ctx, userID)
	require.NoError(t, err)
	assert.Equal(t, userID, fetched.ID)
	assert.Equal(t, username, fetched.Username)
	assert.Equal(t, email, fetched.Email)

	// 4. Get User By Username
	byUsername, err := userRepo.GetUserByUsername(ctx, companyID, username)
	require.NoError(t, err)
	assert.Equal(t, userID, byUsername.ID)

	// 5. Get User By Email
	byEmail, err := userRepo.GetUserByEmail(ctx, companyID, email)
	require.NoError(t, err)
	assert.Equal(t, userID, byEmail.ID)

	// 6. Test Status & Lockout Update
	lockTime := time.Now().Add(30 * time.Minute).Truncate(time.Second)
	err = userRepo.RecordFailedLogin(ctx, userID, 5, system.UserStatusLocked, &lockTime)
	require.NoError(t, err)

	lockedUser, err := userRepo.GetUserByID(ctx, userID)
	require.NoError(t, err)
	assert.Equal(t, system.UserStatusLocked, lockedUser.Status)
	assert.Equal(t, 5, lockedUser.FailedLoginAttempts)
	require.NotNil(t, lockedUser.LockoutUntil)

	// 7. Test Successful Login Reset
	loginTime := time.Now().Truncate(time.Second)
	err = userRepo.RecordSuccessfulLogin(ctx, userID, "192.168.1.50", loginTime)
	require.NoError(t, err)

	activeUser, err := userRepo.GetUserByID(ctx, userID)
	require.NoError(t, err)
	assert.Equal(t, system.UserStatusActive, activeUser.Status)
	assert.Equal(t, 0, activeUser.FailedLoginAttempts)
	assert.Nil(t, activeUser.LockoutUntil)
	assert.Equal(t, "192.168.1.50", activeUser.LastLoginIP)

	// 8. Test Role Assignment
	roles, err := userRepo.ListRoles(ctx)
	require.NoError(t, err)
	assert.NotEmpty(t, roles)

	roleKTT, err := userRepo.GetRoleByCode(ctx, system.RoleChiefAccountant)
	require.NoError(t, err)
	assert.Equal(t, system.RoleChiefAccountant, roleKTT.Code)

	err = userRepo.AssignRole(ctx, userID, roleKTT.ID, "admin-01")
	require.NoError(t, err)

	userRoles, err := userRepo.GetUserRoles(ctx, userID)
	require.NoError(t, err)
	assert.NotEmpty(t, userRoles)
	assert.Equal(t, system.RoleChiefAccountant, userRoles[0].Code)

	// Remove Role
	err = userRepo.RemoveRole(ctx, userID, roleKTT.ID)
	require.NoError(t, err)

	userRolesAfter, err := userRepo.GetUserRoles(ctx, userID)
	require.NoError(t, err)
	assert.Empty(t, userRolesAfter)
}

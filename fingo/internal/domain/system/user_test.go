package system_test

import (
	"testing"
	"time"

	"fingo/internal/domain/system"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewUser_Validation(t *testing.T) {
	t.Parallel()

	validCompanyID := "comp-001"
	validUsername := "ketoan_vien"
	validEmail := "ketoan@fingo.vn"
	validPasswordHash := "$2a$12$e8YkZ...validhash"
	validFullName := "Nguyễn Văn Kế Toán"

	tests := []struct {
		name        string
		companyID   string
		username    string
		email       string
		pwdHash     string
		fullName    string
		wantErr     bool
		errExpected error
	}{
		{
			name:      "Valid user creation",
			companyID: validCompanyID,
			username:  validUsername,
			email:     validEmail,
			pwdHash:   validPasswordHash,
			fullName:  validFullName,
			wantErr:   false,
		},
		{
			name:        "Empty company profile ID",
			companyID:   "",
			username:    validUsername,
			email:       validEmail,
			pwdHash:     validPasswordHash,
			fullName:    validFullName,
			wantErr:     true,
			errExpected: system.ErrInvalidUserData,
		},
		{
			name:        "Empty username",
			companyID:   validCompanyID,
			username:    "",
			email:       validEmail,
			pwdHash:     validPasswordHash,
			fullName:    validFullName,
			wantErr:     true,
			errExpected: system.ErrInvalidUserData,
		},
		{
			name:        "Invalid email format",
			companyID:   validCompanyID,
			username:    validUsername,
			email:       "invalid-email",
			pwdHash:     validPasswordHash,
			fullName:    validFullName,
			wantErr:     true,
			errExpected: system.ErrInvalidEmail,
		},
		{
			name:        "Empty full name",
			companyID:   validCompanyID,
			username:    validUsername,
			email:       validEmail,
			pwdHash:     validPasswordHash,
			fullName:    "",
			wantErr:     true,
			errExpected: system.ErrInvalidUserData,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			u, err := system.NewUser(
				"user-001",
				tt.companyID,
				tt.username,
				tt.email,
				tt.pwdHash,
				tt.fullName,
				"Kế toán viên",
				"NV001",
			)
			if tt.wantErr {
				require.Error(t, err)
				if tt.errExpected != nil {
					require.ErrorIs(t, err, tt.errExpected)
				}
				return
			}
			require.NoError(t, err)
			require.NotNil(t, u)
			assert.Equal(t, system.UserStatusPendingActivation, u.Status)
			assert.True(t, u.MustChangePasswordNext)
			assert.Equal(t, 0, u.FailedLoginAttempts)
			assert.Nil(t, u.LockoutUntil)
		})
	}
}

func TestUser_LoginAndLockout(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 3, 20, 10, 0, 0, 0, time.UTC)
	u, err := system.NewUser(
		"user-002",
		"comp-001",
		"chief_acc",
		"ktt@fingo.vn",
		"$2a$12$somehash",
		"Trần Thị Kế Toán Trưởng",
		"Kế toán trưởng",
		"NV002",
	)
	require.NoError(t, err)
	u.Activate()

	// Initial active status
	require.NoError(t, u.CanLogin(now))

	// Simulate 4 failed attempts
	for i := 1; i <= 4; i++ {
		isLocked := u.RecordFailedLogin(now)
		assert.False(t, isLocked)
		assert.Equal(t, i, u.FailedLoginAttempts)
		assert.Equal(t, system.UserStatusActive, u.Status)
	}

	// 5th failed attempt triggers 30-minute lockout
	isLocked := u.RecordFailedLogin(now)
	assert.True(t, isLocked)
	assert.Equal(t, 5, u.FailedLoginAttempts)
	assert.Equal(t, system.UserStatusLocked, u.Status)
	require.NotNil(t, u.LockoutUntil)
	assert.Equal(t, now.Add(30*time.Minute), *u.LockoutUntil)

	// CanLogin should fail when locked
	err = u.CanLogin(now.Add(10 * time.Minute))
	require.ErrorIs(t, err, system.ErrUserLocked)

	// After 30 minutes, CanLogin allows, status returns to ACTIVE on auto-unlock
	require.NoError(t, u.CanLogin(now.Add(31*time.Minute)))

	// Successful login resets counter
	u.RecordSuccessfulLogin("192.168.1.100", now.Add(32*time.Minute))
	assert.Equal(t, 0, u.FailedLoginAttempts)
	assert.Equal(t, system.UserStatusActive, u.Status)
	assert.Nil(t, u.LockoutUntil)
	assert.Equal(t, "192.168.1.100", u.LastLoginIP)
}

func TestUser_LifecycleTransitions(t *testing.T) {
	t.Parallel()

	u, err := system.NewUser(
		"user-003",
		"comp-001",
		"operator",
		"op@fingo.vn",
		"$2a$12$hash",
		"Lê Văn Vận Hành",
		"Nhân viên",
		"NV003",
	)
	require.NoError(t, err)
	assert.Equal(t, system.UserStatusPendingActivation, u.Status)

	// Activate
	u.Activate()
	assert.Equal(t, system.UserStatusActive, u.Status)

	// Suspend
	u.Suspend()
	assert.Equal(t, system.UserStatusSuspended, u.Status)
	err = u.CanLogin(time.Now())
	require.ErrorIs(t, err, system.ErrUserSuspended)

	// Reactivate
	u.Reactivate()
	assert.Equal(t, system.UserStatusActive, u.Status)

	// Terminate
	u.Terminate()
	assert.Equal(t, system.UserStatusTerminated, u.Status)
	err = u.CanLogin(time.Now())
	require.ErrorIs(t, err, system.ErrUserTerminated)

	// Change Password
	changeTime := time.Date(2026, 3, 20, 12, 0, 0, 0, time.UTC)
	u.ChangePassword("$2a$12$newhash", changeTime)
	assert.False(t, u.MustChangePasswordNext)
	assert.Equal(t, changeTime, u.PasswordChangedAt)

	// Set Digital Cert
	u.SetDigitalCert("CN=NGUYEN VAN A, O=FINGO SME, C=VN", "540123987456")
	assert.Equal(t, "CN=NGUYEN VAN A, O=FINGO SME, C=VN", u.DigitalCertSubject)
	assert.Equal(t, "540123987456", u.DigitalCertSerial)
}

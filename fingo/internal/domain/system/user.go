package system

import (
	"errors"
	"fmt"
	"net/mail"
	"strings"
	"time"
)

var (
	ErrInvalidUserData = errors.New("invalid user data")
	ErrInvalidEmail    = errors.New("invalid email address format")
	ErrUserLocked      = errors.New("user account is locked due to consecutive failed login attempts")
	ErrUserSuspended   = errors.New("user account is suspended")
	ErrUserTerminated  = errors.New("user account is terminated")
	ErrUserNotActive   = errors.New("user account is not active")
)

// UserStatus represents the lifecycle state of a user account
type UserStatus string

const (
	UserStatusPendingActivation UserStatus = "PENDING_ACTIVATION"
	UserStatusActive            UserStatus = "ACTIVE"
	UserStatusSuspended         UserStatus = "SUSPENDED"
	UserStatusLocked            UserStatus = "LOCKED"
	UserStatusTerminated        UserStatus = "TERMINATED"
)

// User represents an enterprise operator within the accounting system
type User struct {
	ID                     string     `json:"id"`
	CompanyProfileID       string     `json:"company_profile_id"`
	Username               string     `json:"username"`
	Email                  string     `json:"email"`
	PasswordHash           string     `json:"-"`
	FullName               string     `json:"full_name"`
	Title                  string     `json:"title"`
	EmployeeCode           string     `json:"employee_code,omitempty"`
	Status                 UserStatus `json:"status"`
	FailedLoginAttempts    int        `json:"failed_login_attempts"`
	LockoutUntil           *time.Time `json:"lockout_until,omitempty"`
	PasswordChangedAt      time.Time  `json:"password_changed_at"`
	MustChangePasswordNext bool       `json:"must_change_password_next"`
	MFASecret              string     `json:"-"`
	IsMFAEnabled           bool       `json:"is_mfa_enabled"`
	DigitalCertSubject     string     `json:"digital_cert_subject,omitempty"`
	DigitalCertSerial      string     `json:"digital_cert_serial,omitempty"`
	LastLoginAt            *time.Time `json:"last_login_at,omitempty"`
	LastLoginIP            string     `json:"last_login_ip,omitempty"`
	CreatedAt              time.Time  `json:"created_at"`
	UpdatedAt              time.Time  `json:"updated_at"`
}

// NewUser constructs and validates a new User entity
func NewUser(
	id string,
	companyProfileID string,
	username string,
	email string,
	passwordHash string,
	fullName string,
	title string,
	employeeCode string,
) (*User, error) {
	if strings.TrimSpace(id) == "" {
		return nil, fmt.Errorf("%w: user ID is required", ErrInvalidUserData)
	}
	if strings.TrimSpace(companyProfileID) == "" {
		return nil, fmt.Errorf("%w: company profile ID is required", ErrInvalidUserData)
	}
	if strings.TrimSpace(username) == "" {
		return nil, fmt.Errorf("%w: username is required", ErrInvalidUserData)
	}
	if strings.TrimSpace(passwordHash) == "" {
		return nil, fmt.Errorf("%w: password hash is required", ErrInvalidUserData)
	}
	if strings.TrimSpace(fullName) == "" {
		return nil, fmt.Errorf("%w: full name is required", ErrInvalidUserData)
	}

	trimmedEmail := strings.TrimSpace(email)
	if trimmedEmail == "" {
		return nil, fmt.Errorf("%w: email is required", ErrInvalidUserData)
	}
	if _, err := mail.ParseAddress(trimmedEmail); err != nil {
		return nil, fmt.Errorf("%w: %s", ErrInvalidEmail, trimmedEmail)
	}

	now := time.Now().UTC()
	return &User{
		ID:                     id,
		CompanyProfileID:       companyProfileID,
		Username:               username,
		Email:                  trimmedEmail,
		PasswordHash:           passwordHash,
		FullName:               fullName,
		Title:                  title,
		EmployeeCode:           employeeCode,
		Status:                 UserStatusPendingActivation,
		FailedLoginAttempts:    0,
		LockoutUntil:           nil,
		PasswordChangedAt:      now,
		MustChangePasswordNext: true,
		IsMFAEnabled:           false,
		CreatedAt:              now,
		UpdatedAt:              now,
	}, nil
}

// Activate transitions a user from PENDING_ACTIVATION to ACTIVE
func (u *User) Activate() {
	u.Status = UserStatusActive
	u.UpdatedAt = time.Now().UTC()
}

// Suspend transitions a user to SUSPENDED
func (u *User) Suspend() {
	u.Status = UserStatusSuspended
	u.UpdatedAt = time.Now().UTC()
}

// Reactivate transitions a user from SUSPENDED back to ACTIVE
func (u *User) Reactivate() {
	u.Status = UserStatusActive
	u.UpdatedAt = time.Now().UTC()
}

// Terminate permanently deactivates a user upon resignation or dismissal
func (u *User) Terminate() {
	u.Status = UserStatusTerminated
	u.UpdatedAt = time.Now().UTC()
}

// CanLogin checks whether the user is in an allowable state to log in at the specified time
func (u *User) CanLogin(now time.Time) error {
	switch u.Status {
	case UserStatusActive:
		return nil
	case UserStatusPendingActivation:
		return nil // Allowed to log in to set password
	case UserStatusSuspended:
		return ErrUserSuspended
	case UserStatusTerminated:
		return ErrUserTerminated
	case UserStatusLocked:
		if u.LockoutUntil != nil && now.After(*u.LockoutUntil) {
			// Auto-unlock after lockout window elapses
			u.Status = UserStatusActive
			u.FailedLoginAttempts = 0
			u.LockoutUntil = nil
			u.UpdatedAt = now
			return nil
		}
		return ErrUserLocked
	default:
		return ErrUserNotActive
	}
}

// RecordFailedLogin registers a failed authentication attempt and locks the account on 5th consecutive failure
func (u *User) RecordFailedLogin(now time.Time) (isLocked bool) {
	u.FailedLoginAttempts++
	u.UpdatedAt = now

	if u.FailedLoginAttempts >= 5 {
		u.Status = UserStatusLocked
		lockoutTime := now.Add(30 * time.Minute)
		u.LockoutUntil = &lockoutTime
		return true
	}
	return false
}

// RecordSuccessfulLogin clears failed attempts and updates login telemetry
func (u *User) RecordSuccessfulLogin(ip string, now time.Time) {
	u.FailedLoginAttempts = 0
	u.LockoutUntil = nil
	if u.Status == UserStatusLocked {
		u.Status = UserStatusActive
	}
	u.LastLoginAt = &now
	u.LastLoginIP = ip
	u.UpdatedAt = now
}

// ChangePassword updates password hash and updates audit timestamp
func (u *User) ChangePassword(newHash string, now time.Time) {
	u.PasswordHash = newHash
	u.PasswordChangedAt = now
	u.MustChangePasswordNext = false
	u.UpdatedAt = now
}

// SetDigitalCert binds a Decree 123 e-invoicing cryptographic certificate
func (u *User) SetDigitalCert(subject, serial string) {
	u.DigitalCertSubject = subject
	u.DigitalCertSerial = serial
	u.UpdatedAt = time.Now().UTC()
}

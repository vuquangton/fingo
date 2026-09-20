package system

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"
	"unicode"

	domain "fingo/internal/domain/system"
	"fingo/pkg/logger"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

var (
	ErrUserNotFound       = errors.New("user not found")
	ErrRoleNotFound       = errors.New("role not found")
	ErrDuplicateUsername  = errors.New("username already exists for this company")
	ErrDuplicateEmail     = errors.New("email already exists for this company")
	ErrInvalidCredentials = errors.New("invalid username or password")
	ErrWeakPassword       = errors.New("password does not meet security complexity requirements")
)

type CreateUserCommand struct {
	CompanyProfileID string `json:"company_profile_id"`
	Username         string `json:"username"`
	Email            string `json:"email"`
	Password         string `json:"password"`
	FullName         string `json:"full_name"`
	Title            string `json:"title"`
	EmployeeCode     string `json:"employee_code"`
}

type AssignScopeCommand struct {
	UserID          string `json:"user_id"`
	OrgUnitID       string `json:"org_unit_id"`
	IsDefault       bool   `json:"is_default"`
	IncludeChildren bool   `json:"include_children"`
}

type UserUseCase struct {
	userRepo domain.UserRepository
	roleRepo domain.RoleRepository
	logger   *logger.Logger
}

func NewUserUseCase(userRepo domain.UserRepository, roleRepo domain.RoleRepository) *UserUseCase {
	return &UserUseCase{
		userRepo: userRepo,
		roleRepo: roleRepo,
		logger:   logger.New(logger.Config{Level: slog.LevelInfo, Format: logger.FormatJSON}),
	}
}

// WithLogger allows injecting a custom logger
func (u *UserUseCase) WithLogger(l *logger.Logger) *UserUseCase {
	if l != nil {
		u.logger = l
	}
	return u
}

func (u *UserUseCase) CreateUser(ctx context.Context, cmd CreateUserCommand) (*domain.User, error) {
	if err := validatePasswordComplexity(cmd.Password); err != nil {
		return nil, err
	}

	// 1. Check duplicate username
	existing, err := u.userRepo.GetUserByUsername(ctx, cmd.CompanyProfileID, cmd.Username)
	if err == nil && existing != nil {
		return nil, fmt.Errorf("%w: %s", ErrDuplicateUsername, cmd.Username)
	}

	// 2. Check duplicate email
	existingEmail, err := u.userRepo.GetUserByEmail(ctx, cmd.CompanyProfileID, cmd.Email)
	if err == nil && existingEmail != nil {
		return nil, fmt.Errorf("%w: %s", ErrDuplicateEmail, cmd.Email)
	}

	// 3. Bcrypt password hashing (work factor 12)
	hashedBytes, err := bcrypt.GenerateFromPassword([]byte(cmd.Password), 12)
	if err != nil {
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}

	// 4. Instantiate domain User entity
	user, err := domain.NewUser(
		uuid.NewString(),
		cmd.CompanyProfileID,
		cmd.Username,
		cmd.Email,
		string(hashedBytes),
		cmd.FullName,
		cmd.Title,
		cmd.EmployeeCode,
	)
	if err != nil {
		return nil, fmt.Errorf("invalid user entity: %w", err)
	}

	// 5. Persist
	if err := u.userRepo.CreateUser(ctx, user); err != nil {
		u.logger.Error(ctx, "failed to persist user",
			slog.String("username", cmd.Username),
			slog.String("error", err.Error()),
		)
		return nil, fmt.Errorf("failed to create user in repository: %w", err)
	}

	u.logger.Info(ctx, "user created successfully",
		slog.String("user_id", user.ID),
		slog.String("username", user.Username),
		slog.String("company_id", user.CompanyProfileID),
	)

	return user, nil
}

func (u *UserUseCase) AssignRole(ctx context.Context, userID string, roleCode domain.SystemRoleCode, assignedBy string) error {
	// 1. Fetch user to ensure exists
	user, err := u.userRepo.GetUserByID(ctx, userID)
	if err != nil {
		return fmt.Errorf("%w: user %s (%v)", ErrUserNotFound, userID, err)
	}

	// 2. Fetch role to ensure exists
	role, err := u.roleRepo.GetRoleByCode(ctx, roleCode)
	if err != nil {
		return fmt.Errorf("%w: role %s (%v)", ErrRoleNotFound, roleCode, err)
	}

	// 3. Fetch existing roles of the user
	existingRoles, err := u.userRepo.GetUserRoles(ctx, userID)
	if err != nil {
		return fmt.Errorf("failed to retrieve user roles: %w", err)
	}

	existingCodes := make([]domain.SystemRoleCode, 0, len(existingRoles))
	for _, r := range existingRoles {
		existingCodes = append(existingCodes, r.Code)
	}

	// 4. Statutory Segregation of Duties (SoD) Invariant Check (Article 52 Law 88/2015)
	if err := domain.ValidateRoleAssignment(existingCodes, roleCode); err != nil {
		u.logger.Warn(ctx, "SoD conflict prevented role assignment",
			slog.String("user_id", userID),
			slog.String("attempted_role", string(roleCode)),
			slog.String("violation", err.Error()),
		)
		return err
	}

	// 5. Persist role assignment
	if err := u.userRepo.AssignRole(ctx, userID, role.ID, assignedBy); err != nil {
		return fmt.Errorf("failed to assign role: %w", err)
	}

	u.logger.Info(ctx, "role successfully assigned to user",
		slog.String("user_id", userID),
		slog.String("username", user.Username),
		slog.String("role_code", string(roleCode)),
		slog.String("assigned_by", assignedBy),
	)

	return nil
}

func (u *UserUseCase) AssignOrgUnitScope(ctx context.Context, cmd AssignScopeCommand) error {
	_, err := u.userRepo.GetUserByID(ctx, cmd.UserID)
	if err != nil {
		return fmt.Errorf("%w: user %s (%v)", ErrUserNotFound, cmd.UserID, err)
	}

	scope := &domain.UserOrgUnitScope{
		ID:              uuid.NewString(),
		UserID:          cmd.UserID,
		OrgUnitID:       cmd.OrgUnitID,
		IsDefault:       cmd.IsDefault,
		IncludeChildren: cmd.IncludeChildren,
		CreatedAt:       time.Now().UTC(),
	}

	if err := u.userRepo.AssignOrgUnitScope(ctx, scope); err != nil {
		return fmt.Errorf("failed to assign org unit scope: %w", err)
	}

	u.logger.Info(ctx, "org unit scope assigned to user",
		slog.String("user_id", cmd.UserID),
		slog.String("org_unit_id", cmd.OrgUnitID),
		slog.Bool("is_default", cmd.IsDefault),
	)

	return nil
}

func (u *UserUseCase) Authenticate(
	ctx context.Context,
	companyID string,
	username string,
	password string,
	clientIP string,
	now time.Time,
) (*domain.User, error) {
	// 1. Lookup user by username
	user, err := u.userRepo.GetUserByUsername(ctx, companyID, username)
	if err != nil {
		u.logger.Warn(ctx, "login attempt failed: user not found",
			slog.String("username", username),
			slog.String("ip", clientIP),
		)
		return nil, ErrInvalidCredentials
	}

	// 2. Check if account is in allowable login state
	if err := user.CanLogin(now); err != nil {
		u.logger.Warn(ctx, "login attempt rejected: account locked or suspended",
			slog.String("user_id", user.ID),
			slog.String("username", user.Username),
			slog.String("status", string(user.Status)),
			slog.String("error", err.Error()),
		)
		return nil, err
	}

	// 3. Verify password hash
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		// Wrong password: record failed attempt & check lockout
		isLocked := user.RecordFailedLogin(now)
		if err := u.userRepo.RecordFailedLogin(ctx, user.ID, user.FailedLoginAttempts, user.Status, user.LockoutUntil); err != nil {
			u.logger.Error(ctx, "failed to persist failed login state",
				slog.String("user_id", user.ID),
				slog.String("error", err.Error()),
			)
		}

		if isLocked {
			lockoutStr := ""
			if user.LockoutUntil != nil {
				lockoutStr = user.LockoutUntil.Format(time.RFC3339)
			}
			u.logger.Warn(ctx, "user account locked due to 5 consecutive failures",
				slog.String("user_id", user.ID),
				slog.String("username", user.Username),
				slog.String("lockout_until", lockoutStr),
			)
		} else {
			u.logger.Warn(ctx, "incorrect password provided",
				slog.String("user_id", user.ID),
				slog.Int("attempts", user.FailedLoginAttempts),
			)
		}
		return nil, ErrInvalidCredentials
	}

	// 4. Successful login: reset failed attempts & record telemetry
	user.RecordSuccessfulLogin(clientIP, now)
	if err := u.userRepo.RecordSuccessfulLogin(ctx, user.ID, clientIP, now); err != nil {
		u.logger.Error(ctx, "failed to record successful login telemetry",
			slog.String("user_id", user.ID),
			slog.String("error", err.Error()),
		)
	}

	u.logger.Info(ctx, "user authenticated successfully",
		slog.String("user_id", user.ID),
		slog.String("username", user.Username),
		slog.String("ip", clientIP),
	)

	return user, nil
}

func validatePasswordComplexity(password string) error {
	if len(password) < 8 {
		return fmt.Errorf("%w: minimum 8 characters required", ErrWeakPassword)
	}
	var hasUpper, hasLower, hasDigit, hasSpecial bool
	for _, ch := range password {
		switch {
		case unicode.IsUpper(ch):
			hasUpper = true
		case unicode.IsLower(ch):
			hasLower = true
		case unicode.IsDigit(ch):
			hasDigit = true
		case unicode.IsPunct(ch) || unicode.IsSymbol(ch):
			hasSpecial = true
		}
	}
	if !hasUpper || !hasLower || !hasDigit || !hasSpecial {
		return fmt.Errorf("%w: must include uppercase, lowercase, digit, and special character", ErrWeakPassword)
	}
	return nil
}

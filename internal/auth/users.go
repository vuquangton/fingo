package auth

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// User is a person who may sign in.
type User struct {
	ID          string
	Email       string
	DisplayName string
	Role        string
	Active      bool
}

// ErrUserExists is returned for an email already in use.
var ErrUserExists = errors.New("auth: that email is already registered")

// ErrBadPassword is returned when a change is offered with the wrong current
// password, or a new one that is too short.
var ErrBadPassword = errors.New("auth: the password is not acceptable")

// ListUsers returns every account, by email. It is practice-level data, so it is
// read straight from the pool rather than through a client scope.
func ListUsers(ctx context.Context, pool *pgxpool.Pool) ([]User, error) {
	rows, err := pool.Query(ctx, `
		SELECT id::text, email, display_name, role, active FROM users ORDER BY email`)
	if err != nil {
		return nil, fmt.Errorf("auth: list users: %w", err)
	}
	defer rows.Close()

	var out []User
	for rows.Next() {
		var u User
		if err := rows.Scan(&u.ID, &u.Email, &u.DisplayName, &u.Role, &u.Active); err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, rows.Err()
}

// CreateUser registers an account with an initial password.
func CreateUser(ctx context.Context, pool *pgxpool.Pool, email, displayName, role, password string) (User, error) {
	email = strings.TrimSpace(strings.ToLower(email))
	displayName = strings.TrimSpace(displayName)
	role = strings.ToLower(strings.TrimSpace(role))

	switch {
	case email == "":
		return User{}, errors.New("auth: a user needs an email")
	case displayName == "":
		return User{}, errors.New("auth: a user needs a display name")
	case role != "admin" && role != "accountant" && role != "viewer":
		return User{}, fmt.Errorf("auth: %q is not a role", role)
	case len(password) < 12:
		return User{}, fmt.Errorf("%w: at least 12 characters", ErrBadPassword)
	}

	hash, err := Hash(password)
	if err != nil {
		return User{}, err
	}

	var u User
	err = pool.QueryRow(ctx, `
		INSERT INTO users (email, display_name, password_hash, role)
		VALUES ($1, $2, $3, $4)
		RETURNING id::text, email, display_name, role, active`,
		email, displayName, hash, role).Scan(&u.ID, &u.Email, &u.DisplayName, &u.Role, &u.Active)
	if err != nil {
		if isUniqueViolation(err) {
			return User{}, ErrUserExists
		}
		return User{}, fmt.Errorf("auth: create user: %w", err)
	}

	return u, nil
}

// ChangePassword replaces a password after verifying the current one.
//
// The current password is required even for an administrator: a session left open
// on an unlocked machine must not be enough to take the account over.
func ChangePassword(ctx context.Context, pool *pgxpool.Pool, email, current, next string) error {
	if len(next) < 12 {
		return fmt.Errorf("%w: at least 12 characters", ErrBadPassword)
	}

	var hash string
	err := pool.QueryRow(ctx, `SELECT password_hash FROM users WHERE email = $1 AND active`, email).Scan(&hash)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrMismatch
	}
	if err != nil {
		return fmt.Errorf("auth: read the password: %w", err)
	}

	if err := Verify(hash, current); err != nil {
		return ErrMismatch
	}

	next, err = Hash(next)
	if err != nil {
		return err
	}

	if _, err := pool.Exec(ctx, `UPDATE users SET password_hash = $2 WHERE email = $1`, email, next); err != nil {
		return fmt.Errorf("auth: change the password: %w", err)
	}
	return nil
}

func isUniqueViolation(err error) bool {
	var pgErr interface{ SQLState() string }
	return errors.As(err, &pgErr) && pgErr.SQLState() == "23505"
}

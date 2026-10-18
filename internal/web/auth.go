// Package web is the HTTP surface: Gin for routing, scs for sessions, the
// standard library for CSRF, and nothing hand-rolled where a vetted library
// exists.
//
// Security posture: deny by default. Every route is registered inside a group
// that requires an authenticated user; the login and health routes are the only
// ones registered outside it, and both are explicitly listed here so that
// "which routes are public" is one readable fact rather than a grep.
package web

import (
	"errors"
	"net/http"
	"time"

	"github.com/alexedwards/scs/pgxstore"
	"github.com/alexedwards/scs/v2"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"fingo/internal/auth"
)

// Session keys. Constants, not literals, so a typo is a compile error.
const (
	keyUserID = "user_id"
	keyEmail  = "email"
	keyRole   = "role"
	keyClient = "client_id"
)

// Role is what a user may do.
type Role string

const (
	RoleAdmin      Role = "admin"
	RoleAccountant Role = "accountant"
	RoleViewer     Role = "viewer"
)

// Auth is the login surface and its middleware.
type Auth struct {
	sessions *scs.SessionManager
	pool     *pgxpool.Pool
}

// NewAuth builds the session manager. Sessions live in PostgreSQL, so a restart
// does not sign everybody out and the operations team has one store to back up.
func NewAuth(pool *pgxpool.Pool) *Auth {
	sessions := scs.New()
	sessions.Store = pgxstore.New(pool)
	sessions.Lifetime = 8 * time.Hour
	sessions.Cookie.Name = "fingo_session"
	sessions.Cookie.HttpOnly = true
	sessions.Cookie.Secure = true // the server refuses to start over plain HTTP
	sessions.Cookie.SameSite = http.SameSiteLaxMode
	sessions.Cookie.Path = "/"

	return &Auth{sessions: sessions, pool: pool}
}

// Sessions exposes the manager for Gin middleware and tests.
func (a *Auth) Sessions() *scs.SessionManager { return a.sessions }

// Handler wraps the whole engine with the session loader and the standard
// library's cross-origin protection.
//
// scs must wrap the outermost handler, not sit inside a Gin middleware: it sets
// the session cookie on the ResponseWriter it is given, and only a wrapper around
// the entire engine guarantees that writer is the one the client sees.
//
// net/http.CrossOriginProtection (Go 1.25+) rejects unsafe cross-origin browser
// requests using Sec-Fetch-Site/Origin. It is the whole CSRF story: no token
// table, no third-party middleware, no per-form field.
func (a *Auth) Handler(engine http.Handler) http.Handler {
	return http.NewCrossOriginProtection().Handler(a.sessions.LoadAndSave(engine))
}
// UserFrom returns the signed-in user's identity.
func UserFrom(c *gin.Context) (email string, role Role, ok bool) {
	email = ""
	if v, exists := c.Get(keyEmail); exists {
		email, _ = v.(string)
	}
	if v, exists := c.Get(keyRole); exists {
		r, _ := v.(string)
		role = Role(r)
	}
	return email, role, email != ""
}

// RequireUser refuses a request with no session.
//
// It answers 401 with no body rather than redirecting: this is an API-shaped
// surface, and a redirect to a login page that does not exist yet would be a
// confusing 200 in a test.
func (a *Auth) RequireUser() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !a.sessions.Exists(c.Request.Context(), keyUserID) {
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}

		c.Set(keyEmail, a.sessions.GetString(c.Request.Context(), keyEmail))
		c.Set(keyRole, a.sessions.GetString(c.Request.Context(), keyRole))
		c.Next()
	}
}

// RequireRole admits only the named roles.
func (a *Auth) RequireRole(roles ...Role) gin.HandlerFunc {
	allowed := make(map[Role]bool, len(roles))
	for _, r := range roles {
		allowed[r] = true
	}

	return func(c *gin.Context) {
		_, role, ok := UserFrom(c)
		if !ok || !allowed[role] {
			c.AbortWithStatus(http.StatusForbidden)
			return
		}
		c.Next()
	}
}

// LoginRequest is the form a login posts.
type LoginRequest struct {
	Email    string `form:"email"    json:"email"    binding:"required"`
	Password string `form:"password" json:"password" binding:"required"`
}

// Login verifies a password and starts a session.
//
// The error for an unknown email and the error for a wrong password are the same
// status and the same body: distinguishing them tells an attacker which addresses
// exist. The failure is still recorded, with the address, so brute force is
// visible in the log even though the response says nothing.
func (a *Auth) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBind(&req); err != nil {
		c.Status(http.StatusBadRequest)
		return
	}

	var (
		userID string
		hash   string
		role   string
		active bool
	)

	err := a.pool.QueryRow(c.Request.Context(),
		`SELECT id, password_hash, role, active FROM users WHERE email = $1`,
		req.Email).Scan(&userID, &hash, &role, &active)

	if errors.Is(err, pgx.ErrNoRows) {
		a.recordFailure(c, req.Email)
		c.Status(http.StatusUnauthorized)
		return
	}
	if err != nil {
		c.Status(http.StatusInternalServerError)
		return
	}

	if !active || auth.Verify(hash, req.Password) != nil {
		a.recordFailure(c, req.Email)
		c.Status(http.StatusUnauthorized)
		return
	}

	// Fixation defence: a fresh session on privilege change, never the one the
	// request arrived with.
	if err := a.sessions.RenewToken(c.Request.Context()); err != nil {
		c.Status(http.StatusInternalServerError)
		return
	}

	a.sessions.Put(c.Request.Context(), keyUserID, userID)
	a.sessions.Put(c.Request.Context(), keyEmail, req.Email)
	a.sessions.Put(c.Request.Context(), keyRole, role)

	c.Status(http.StatusNoContent)
}

// Logout destroys the session.
func (a *Auth) Logout(c *gin.Context) {
	if err := a.sessions.Destroy(c.Request.Context()); err != nil {
		c.Status(http.StatusInternalServerError)
		return
	}
	c.Status(http.StatusNoContent)
}

// recordFailure is best effort: a login that failed must still fail if the log
// write does, so the error is deliberately not surfaced.
func (a *Auth) recordFailure(c *gin.Context, email string) {
	_, _ = a.pool.Exec(c.Request.Context(),
		`INSERT INTO failed_logins (email, remote_ip) VALUES ($1, $2)`,
		email, c.ClientIP())
}

// RequireClient refuses a request that has not selected a client.
//
// A financial handler needs the tenant scope that becomes app.client_id; a
// session with no client is a session that must pick one first.
func (a *Auth) RequireClient() gin.HandlerFunc {
	return func(c *gin.Context) {
		if a.sessions.GetString(c.Request.Context(), keyClient) == "" {
			c.AbortWithStatus(http.StatusConflict)
			return
		}
		c.Next()
	}
}

// SelectClient pins the session to one client, after checking it exists.
func (a *Auth) SelectClient(c *gin.Context) {
	clientID := c.Param("id")

	var exists bool
	if err := a.pool.QueryRow(c.Request.Context(),
		`SELECT EXISTS (SELECT 1 FROM clients WHERE id = $1 AND active)`, clientID).Scan(&exists); err != nil || !exists {
		c.Status(http.StatusNotFound)
		return
	}

	a.sessions.Put(c.Request.Context(), keyClient, clientID)
	c.Status(http.StatusNoContent)
}

// ClientFrom returns the client the session is scoped to.
func (a *Auth) ClientFrom(c *gin.Context) string {
	return a.sessions.GetString(c.Request.Context(), keyClient)
}

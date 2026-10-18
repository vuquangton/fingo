package client

import (
	"context"
	"fmt"
)

// Principal is who a request is acting as.
//
// It is produced by the authentication layer, which runs before the client
// middleware and has already established that the caller is the operator it
// claims to be. This package does not authenticate anybody: it takes the client
// that authentication named and turns it into a Scope. Keeping the two apart
// means the client boundary can be tested without a working login, and a change
// to how operators sign in cannot weaken how books are scoped.
type Principal struct {
	// UserID is the operator. Informational here; the audit trail is what
	// needs it.
	UserID string

	// ClientID is the client company the operator has switched into. It is
	// the only field this package acts on.
	ClientID string
}

// contextKey is an unexported type so that no other package can collide with
// these values, deliberately or by accident.
type contextKey int

const (
	principalKey contextKey = iota
	scopeKey
)

// WithPrincipal installs the authenticated principal on a context.
//
// The authentication layer calls this. Nothing in this package does, because
// nothing in this package is allowed to decide who the caller is.
func WithPrincipal(ctx context.Context, p Principal) context.Context {
	return context.WithValue(ctx, principalKey, p)
}

// PrincipalFrom returns the principal installed on a context, if any.
func PrincipalFrom(ctx context.Context) (Principal, bool) {
	p, ok := ctx.Value(principalKey).(Principal)

	return p, ok
}

// WithScope installs the client scope on a context.
//
// The middleware calls this once per request. It is exported so that a
// non-HTTP entry point — a scheduled job, a migration, a test — can establish
// the same boundary the middleware does, without pretending to be a request.
func WithScope(ctx context.Context, s Scope) context.Context {
	return context.WithValue(ctx, scopeKey, s)
}

// ScopeFrom returns the client scope carried by a context.
//
// It fails rather than returning the zero Scope when there is none, so a handler
// that reads books without the middleware in front of it gets an error instead
// of an unscoped query.
func ScopeFrom(ctx context.Context) (Scope, error) {
	s, ok := ctx.Value(scopeKey).(Scope)
	if !ok || !s.Valid() {
		return Scope{}, fmt.Errorf(
			"%w: this context carries no client scope; the request did not pass through the client middleware",
			ErrNoScope)
	}

	return s, nil
}

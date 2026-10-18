package client

import (
	"errors"
	"net/http"
	"strings"
)

// Extractor reads the client a request claims from the authenticated request.
//
// It is a function rather than a fixed header read because how a session names
// its client is the authentication layer's business, not this package's. The
// default reads the principal the authentication layer installed on the request
// context; a deployment that carries the client on the token instead supplies
// its own, and the rest of this middleware is unchanged.
type Extractor func(r *http.Request) (clientID string, ok bool)

// Option configures a Middleware.
type Option func(*Middleware)

// WithExtractor replaces how the requested client is read.
func WithExtractor(e Extractor) Option {
	return func(m *Middleware) {
		if e != nil {
			m.extract = e
		}
	}
}

// WithErrorHandler is told about registry failures that are not the caller's
// fault, so they can be logged rather than only turned into a 500.
func WithErrorHandler(report func(error)) Option {
	return func(m *Middleware) { m.onError = report }
}

// Middleware turns the client a request was authenticated for into a Scope, and
// puts it on the request context.
//
// It is the one place where a request learns which client's books it may touch.
// The handler underneath never chooses: it is handed a context in which the
// scope is already fixed, and every repository call it makes needs that scope.
// A handler that ignores it cannot fall back to "all clients" — there is no
// such scope to fall back to — it simply fails.
//
// Three things are refused, and they are refused differently on purpose:
//
//   - a request with no client the extractor can find is 401: nothing has
//     established who is asking.
//   - a request naming a client the registry does not hold is 403.
//   - a request naming a client that has been closed is 403.
//
// The last two are one answer to the caller and two to the log, because whether
// a company is a client of this practice is itself information the practice does
// not owe to an unauthenticated caller.
type Middleware struct {
	registry Registry
	extract  Extractor
	onError  func(error)
}

// ScopedHandler is a handler that is handed the client it is serving.
//
// Taking the scope as an argument rather than reading it out of the context is
// the point: the repository call and the scope are written together, and a
// handler cannot reach a financial repository without having the scope in hand
// to pass it.
type ScopedHandler func(w http.ResponseWriter, r *http.Request, scope Scope)

// NewMiddleware builds the middleware over a registry.
func NewMiddleware(registry Registry, opts ...Option) *Middleware {
	m := &Middleware{registry: registry, extract: PrincipalClient}

	for _, opt := range opts {
		opt(m)
	}

	return m
}

// PrincipalClient reads the client id off the authenticated principal.
//
// It is the default Extractor: authentication runs first and says who the caller
// is and which client they have switched into, and this is the only part of that
// answer the books are scoped by.
func PrincipalClient(r *http.Request) (string, bool) {
	p, ok := PrincipalFrom(r.Context())
	if !ok || strings.TrimSpace(p.ClientID) == "" {
		return "", false
	}

	return strings.TrimSpace(p.ClientID), true
}

// Wrap returns the handler with the client scope established.
func (m *Middleware) Wrap(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id, ok := m.extract(r)
		if !ok {
			http.Error(w, "the request is not authenticated for a client", http.StatusUnauthorized)
			return
		}

		scope, err := ScopeFor(id)
		if err != nil {
			http.Error(w, "the request is not authenticated for a client", http.StatusUnauthorized)
			return
		}

		c, err := m.registry.Get(r.Context(), scope.ClientID())
		if err != nil {
			if !errors.Is(err, ErrUnknownClient) {
				m.report(err)
				http.Error(w, "the client could not be looked up", http.StatusInternalServerError)

				return
			}

			http.Error(w, "this client is not served here", http.StatusForbidden)

			return
		}
		if !c.Active {
			m.report(errClientClosed(scope.ClientID()))
			http.Error(w, "this client is not served here", http.StatusForbidden)

			return
		}

		next.ServeHTTP(w, r.WithContext(WithScope(r.Context(), scope)))
	})
}

// Handle adapts a scoped handler, so the client is an argument rather than
// something the handler has to remember to look up.
func (m *Middleware) Handle(h ScopedHandler) http.Handler {
	return m.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		scope, err := ScopeFrom(r.Context())
		if err != nil {
			// Unreachable through Handle, which wraps this in Wrap. It is
			// checked rather than assumed so that the invariant fails loudly
			// if the two are ever separated.
			m.report(err)
			http.Error(w, "the request carries no client scope", http.StatusInternalServerError)

			return
		}

		h(w, r, scope)
	}))
}

// report hands a failure to the error handler, if one was set.
func (m *Middleware) report(err error) {
	if m.onError != nil && err != nil {
		m.onError(err)
	}
}

// errClientClosed describes a refusal for the log.
func errClientClosed(id string) error {
	return errors.New("client: client " + id + " is not active")
}

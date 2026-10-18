package client

import (
	"errors"
	"fmt"
	"strings"
)

// Errors the scoping layer can produce.
var (
	// ErrNoScope is a financial query that was not told which client it is
	// for. It is refused before the repository is touched, so no query runs
	// without a client.
	ErrNoScope = errors.New("client: no client scope")

	// ErrForeignClient is a write that carries another client's identity. It
	// is refused rather than allowed to overrule the scope: the scope is what
	// the request was authenticated for, and a record that disagrees with it
	// is a caller bug or an attack, never a request to be honoured.
	ErrForeignClient = errors.New("client: record belongs to another client")
)

// Scope names the one client a financial query may touch.
//
// The zero Scope is not a wildcard. It is invalid, and every repository in the
// system refuses it — which is the point. If "no client" meant "all clients",
// then the most common bug in a multi-tenant system, forgetting to scope a
// query, would silently return everybody's data. Here it returns an error, and
// the mistake is loud.
//
// The field is unexported on purpose: the only ways to hold a Scope are to build
// one from a client id that was checked, or to read one the middleware put in
// the request context. A caller cannot reassign it to another client halfway
// through a request.
type Scope struct {
	clientID string
}

// ScopeFor builds the scope of one client. An empty id is refused: a scope that
// names nobody is not a scope.
func ScopeFor(clientID string) (Scope, error) {
	id := strings.TrimSpace(clientID)
	if id == "" {
		return Scope{}, fmt.Errorf("%w: a scope must name a client", ErrNoScope)
	}

	return Scope{clientID: id}, nil
}

// ClientID returns the client this scope is for, or "" for the zero Scope.
func (s Scope) ClientID() string { return s.clientID }

// Valid reports whether the scope names a client.
func (s Scope) Valid() bool { return strings.TrimSpace(s.clientID) != "" }

// String renders the scope for a log line. It is the client id, never "all".
func (s Scope) String() string {
	if !s.Valid() {
		return "client(none)"
	}

	return "client(" + s.clientID + ")"
}

// guard is the check every repository method starts with.
//
// It is a method rather than an exported function so that a repository built on
// this package cannot forget it by accident: the guard is one line, and the
// alternative — reading rows before checking — is not expressible through the
// helpers here.
func (s Scope) guard() error {
	if !s.Valid() {
		return fmt.Errorf("%w: this query was not told which client it is for,"+
			" and an unscoped query would return every client's data", ErrNoScope)
	}

	return nil
}

// owns reports whether a stored row's client column agrees with the scope.
//
// A row that states no owner is adopted by the scope — a record written before
// the column existed (NULL in the table, empty here: migration 00015 leaves
// those rows NULL rather than filing them under a client called nothing), or a
// caller that had no reason to repeat what the request already said. A row that
// states a different owner is refused.
func (s Scope) owns(rowClientID string) error {
	other := strings.TrimSpace(rowClientID)
	if other == "" || other == s.clientID {
		return nil
	}

	return fmt.Errorf("%w: the row is client %s, the request is for client %s",
		ErrForeignClient, other, s.clientID)
}

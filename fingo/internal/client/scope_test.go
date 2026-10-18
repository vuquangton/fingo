package client_test

import (
	"context"
	"errors"
	"testing"

	"fingo/internal/client"
)

// A scope that names nobody is not a scope. It is refused at the point it is
// built, so the mistake cannot travel as far as a repository.
func TestScopeFor_RefusesAScopeThatNamesNobody(t *testing.T) {
	for _, id := range []string{"", "   ", "\t\n"} {
		if _, err := client.ScopeFor(id); !errors.Is(err, client.ErrNoScope) {
			t.Errorf("ScopeFor(%q) = %v, want ErrNoScope", id, err)
		}
	}
}

func TestScopeFor_NamesOneClient(t *testing.T) {
	s, err := client.ScopeFor("  c-1  ")
	if err != nil {
		t.Fatalf("ScopeFor: %v", err)
	}
	if s.ClientID() != "c-1" {
		t.Errorf("client id = %q, want %q", s.ClientID(), "c-1")
	}
	if !s.Valid() {
		t.Error("a scope built from an id is valid")
	}
	if s.String() != "client(c-1)" {
		t.Errorf("String() = %q", s.String())
	}
}

// The zero Scope is what a caller gets by doing nothing, and it is the whole
// safety property: it means "nobody", never "everybody".
func TestScope_ZeroValueNamesNobody(t *testing.T) {
	var s client.Scope

	if s.Valid() {
		t.Fatal("the zero Scope is valid")
	}
	if s.ClientID() != "" {
		t.Errorf("client id = %q, want empty", s.ClientID())
	}
	if s.String() != "client(none)" {
		t.Errorf("String() = %q, want client(none)", s.String())
	}
}

// Without a scope on the context there is no client to query for, and a handler
// that forgot the middleware gets an error rather than everybody's books.
func TestScopeFrom_RefusesAContextWithNoScope(t *testing.T) {
	if _, err := client.ScopeFrom(context.Background()); !errors.Is(err, client.ErrNoScope) {
		t.Fatalf("ScopeFrom(empty) = %v, want ErrNoScope", err)
	}
}

func TestWithScope_RoundTripsThroughAContext(t *testing.T) {
	want, err := client.ScopeFor("c-1")
	if err != nil {
		t.Fatalf("ScopeFor: %v", err)
	}

	got, err := client.ScopeFrom(client.WithScope(context.Background(), want))
	if err != nil {
		t.Fatalf("ScopeFrom: %v", err)
	}
	if got != want {
		t.Errorf("scope = %v, want %v", got, want)
	}
}

// A scope installed on one context does not appear on another, so two requests
// in flight at once cannot share a client.
func TestWithScope_DoesNotLeakIntoTheParentContext(t *testing.T) {
	parent := context.Background()
	scope, err := client.ScopeFor("c-1")
	if err != nil {
		t.Fatalf("ScopeFor: %v", err)
	}

	child := client.WithScope(parent, scope)

	if got, err := client.ScopeFrom(child); err != nil || got != scope {
		t.Fatalf("child scope = %v, %v; want %v", got, err, scope)
	}
	if _, err := client.ScopeFrom(parent); !errors.Is(err, client.ErrNoScope) {
		t.Fatalf("the parent context gained a scope: %v", err)
	}
}

func TestPrincipal_RoundTripsThroughAContext(t *testing.T) {
	want := client.Principal{UserID: "u-1", ClientID: "c-1"}

	got, ok := client.PrincipalFrom(client.WithPrincipal(context.Background(), want))
	if !ok {
		t.Fatal("PrincipalFrom found no principal on a context that has one")
	}
	if got != want {
		t.Errorf("principal = %+v, want %+v", got, want)
	}

	if _, ok := client.PrincipalFrom(context.Background()); ok {
		t.Error("PrincipalFrom found a principal on an empty context")
	}
}

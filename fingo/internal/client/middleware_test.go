package client_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"fingo/internal/client"
)

// authStub stands in for the authentication layer.
//
// It runs before the client middleware and says who the caller is — here, from a
// header, which is what a test can set. The client middleware never authenticates
// anybody; it takes the client this layer named.
func authStub(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if id := r.Header.Get("X-Test-Client"); id != "" {
			r = r.WithContext(client.WithPrincipal(r.Context(),
				client.Principal{UserID: "u-1", ClientID: id}))
		}

		next.ServeHTTP(w, r)
	})
}

// documentNumbers answers a request with the numbers of the documents the scoped
// client holds. It reads the scope it was handed and passes that same scope to
// the repository, which is the only way it can call it.
func documentNumbers(repo client.DocumentRepository) client.ScopedHandler {
	return func(w http.ResponseWriter, r *http.Request, scope client.Scope) {
		fromContext, err := client.ScopeFrom(r.Context())
		if err != nil || fromContext != scope {
			http.Error(w, "the handler was handed a scope the context does not carry", http.StatusInternalServerError)

			return
		}

		docs, err := repo.List(r.Context(), scope)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)

			return
		}

		numbers := make([]string, 0, len(docs))
		for _, d := range docs {
			numbers = append(numbers, d.Number)
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(numbers)
	}
}

// get drives one request through a stack as the given client.
func get(t *testing.T, h http.Handler, clientID string) (status int, numbers []string) {
	t.Helper()

	req := httptest.NewRequest(http.MethodGet, "/documents", nil)
	if clientID != "" {
		req.Header.Set("X-Test-Client", clientID)
	}

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code == http.StatusOK {
		if err := json.Unmarshal(rec.Body.Bytes(), &numbers); err != nil {
			t.Fatalf("decode response %q: %v", rec.Body.String(), err)
		}
	}

	return rec.Code, numbers
}

// seededStack builds the whole stack — authentication, middleware, handler —
// over two registered clients that each hold one document.
func seededStack(t *testing.T) (http.Handler, *client.MemoryRegistry) {
	t.Helper()

	ctx := context.Background()
	reg, _, _, _ := register(t)

	repo := client.NewMemoryDocumentRepository()
	for _, seed := range []struct {
		clientID string
		number   string
	}{
		{"c-a", "HD-A1"},
		{"c-b", "HD-B1"},
	} {
		d := doc("d-"+seed.clientID, seed.number, day1, 100)
		if err := repo.Put(ctx, mustScope(t, seed.clientID), d); err != nil {
			t.Fatalf("seed %s: %v", seed.clientID, err)
		}
	}

	return authStub(client.NewMiddleware(reg).Handle(documentNumbers(repo))), reg
}

// The end-to-end statement of the objective: two clients go through one stack,
// and each is answered with its own documents and nothing else.
func TestMiddleware_AnswersEachRequestWithItsOwnClientsDocuments(t *testing.T) {
	stack, _ := seededStack(t)

	status, numbers := get(t, stack, "c-a")
	if status != http.StatusOK {
		t.Fatalf("client A: status %d, want 200", status)
	}
	if len(numbers) != 1 || numbers[0] != "HD-A1" {
		t.Errorf("client A got %v, want [HD-A1]", numbers)
	}

	status, numbers = get(t, stack, "c-b")
	if status != http.StatusOK {
		t.Fatalf("client B: status %d, want 200", status)
	}
	if len(numbers) != 1 || numbers[0] != "HD-B1" {
		t.Errorf("client B got %v, want [HD-B1]", numbers)
	}
}

// Repeating A's request does not begin to answer with B's documents, and the
// order the two are asked in makes no difference to either.
func TestMiddleware_DoesNotCarryAScopeBetweenRequests(t *testing.T) {
	stack, _ := seededStack(t)

	for i := 0; i < 3; i++ {
		if _, numbers := get(t, stack, "c-a"); len(numbers) != 1 || numbers[0] != "HD-A1" {
			t.Fatalf("request %d for client A got %v", i, numbers)
		}
		if _, numbers := get(t, stack, "c-b"); len(numbers) != 1 || numbers[0] != "HD-B1" {
			t.Fatalf("request %d for client B got %v", i, numbers)
		}
	}
}

// Nothing has established which client is asking, so nothing is answered.
func TestMiddleware_RefusesARequestWithNoClient(t *testing.T) {
	stack, _ := seededStack(t)

	if status, _ := get(t, stack, ""); status != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", status)
	}
}

// A client the practice does not hold is refused, and refused the same way as a
// client that has been closed: whether a company is a client here is not
// information the answer owes the caller.
func TestMiddleware_RefusesAnUnknownOrClosedClient(t *testing.T) {
	stack, reg := seededStack(t)

	if status, _ := get(t, stack, "c-nobody"); status != http.StatusForbidden {
		t.Fatalf("unknown client: status = %d, want 403", status)
	}

	if err := reg.SetActive(context.Background(), "c-b", false); err != nil {
		t.Fatalf("SetActive: %v", err)
	}
	if status, _ := get(t, stack, "c-b"); status != http.StatusForbidden {
		t.Fatalf("closed client: status = %d, want 403", status)
	}

	// The other client is unaffected: one closed client is not the practice
	// going down.
	if status, numbers := get(t, stack, "c-a"); status != http.StatusOK || len(numbers) != 1 {
		t.Fatalf("client A after closing client B: status %d, numbers %v", status, numbers)
	}
}

// How a session names its client is the authentication layer's business. A
// deployment that carries it elsewhere supplies an extractor and nothing else
// changes.
func TestMiddleware_AcceptsAnotherExtractor(t *testing.T) {
	reg, _, _, _ := register(t)

	// The client travels on the request itself; no principal is installed at
	// all, which is what a token-carried client looks like here.
	withHeader := client.NewMiddleware(reg,
		client.WithExtractor(func(r *http.Request) (string, bool) {
			id := r.Header.Get("X-Client")

			return id, id != ""
		}),
	)

	req := httptest.NewRequest(http.MethodGet, "/documents", nil)
	req.Header.Set("X-Client", "c-a")

	rec := httptest.NewRecorder()
	withHeader.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		scope, err := client.ScopeFrom(r.Context())
		if err != nil {
			t.Errorf("ScopeFrom: %v", err)

			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(scope.ClientID()))
	})).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK || rec.Body.String() != "c-a" {
		t.Fatalf("status %d, body %q; want 200 and c-a", rec.Code, rec.Body.String())
	}
}

// A registry that cannot be read is a 500, not a silent pass-through: running
// the handler without knowing whose books it is for is the one thing this
// middleware must never do.
func TestMiddleware_RefusesTheRequestWhenTheRegistryFails(t *testing.T) {
	failure := errors.New("the registry is unreachable")

	var reported []error

	handler := client.NewMiddleware(brokenRegistry{err: failure},
		client.WithErrorHandler(func(err error) { reported = append(reported, err) }),
	).Handle(func(http.ResponseWriter, *http.Request, client.Scope) {
		t.Error("the handler ran without a client being established")
	})

	req := httptest.NewRequest(http.MethodGet, "/documents", nil)
	req = req.WithContext(client.WithPrincipal(req.Context(),
		client.Principal{ClientID: "c-a"}))

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
	if len(reported) != 1 || !errors.Is(reported[0], failure) {
		t.Fatalf("reported = %v, want the registry failure once", reported)
	}
}

// brokenRegistry is a registry that cannot answer.
type brokenRegistry struct{ err error }

func (b brokenRegistry) Create(context.Context, client.Client) error { return b.err }

func (b brokenRegistry) Get(context.Context, string) (client.Client, error) {
	return client.Client{}, b.err
}

func (b brokenRegistry) List(context.Context) ([]client.Client, error) { return nil, b.err }

func (b brokenRegistry) Update(context.Context, client.Client) error { return b.err }

func (b brokenRegistry) SetActive(context.Context, string, bool) error { return b.err }

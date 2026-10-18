package web

import (
	"bytes"
	"io"
	"io/fs"
	"net/http"
	"net/url"
	"strings"

	"github.com/google/uuid"

	"fingo/internal/idempotency"
)

// keyField is the form field the idempotency key travels in.
//
// A hidden input rather than a header, because an HTML form cannot set one. The
// server renders a usable key into it, so a browser with no JavaScript still
// sends a good key; the script in the layout replaces it with a freshly minted
// one where it can, which is what makes two tabs of the same form two requests.
const keyField = "idempotency_key"

// Server is the admin UI's HTTP surface.
type Server struct {
	books  Books
	tmpl   *renderer
	static fs.FS

	// newKey mints the key a form is rendered with.
	newKey func() string

	// idempotency answers a repeated write with the answer it already gave.
	idempotency *idempotency.Middleware
}

// ServerOption configures a Server.
type ServerOption func(*Server)

// WithKeyFunc replaces the generator behind the idempotency key.
//
// Injected so that a test can know the key a form carries without scraping it
// back out of the page, and so that a deployment could seed keys from somewhere
// other than crypto/rand.
func WithKeyFunc(newKey func() string) ServerOption {
	return func(s *Server) {
		if newKey != nil {
			s.newKey = newKey
		}
	}
}

// NewServer builds the admin UI over a set of books.
func NewServer(books Books, opts ...ServerOption) (*Server, error) {
	s := &Server{
		books:  books,
		newKey: func() string { return uuid.NewString() },
	}

	for _, opt := range opts {
		opt(s)
	}

	tmpl, err := newRenderer(s.newKey)
	if err != nil {
		return nil, err
	}
	s.tmpl = tmpl

	static, err := staticFS()
	if err != nil {
		return nil, err
	}
	s.static = static

	s.idempotency = idempotency.New(idempotency.NewMemoryStore())

	return s, nil
}

// Handler returns the routes.
//
// The idempotency middleware wraps the whole mux rather than each write. It only
// acts on mutating methods, so a read passes through untouched, and a route
// added later cannot be forgotten — which is the failure mode of guarding the
// writes one by one.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /{$}", s.dashboard)

	mux.HandleFunc("GET /documents", s.documents)
	mux.HandleFunc("GET /documents/new", s.documentNew)
	mux.HandleFunc("GET /documents/{id}", s.documentView)
	mux.HandleFunc("GET /documents/{id}/pay", s.documentPay)

	mux.HandleFunc("POST /documents/invoice", s.createInvoice)
	mux.HandleFunc("POST /documents/payment", s.createPayment)
	mux.HandleFunc("POST /documents/journal", s.createJournal)

	mux.HandleFunc("GET /accounts", s.accounts)
	mux.HandleFunc("GET /accounts/new", s.accountNew)
	mux.HandleFunc("POST /accounts", s.createAccount)

	mux.HandleFunc("GET /reports/trial-balance", s.trialBalance)
	mux.HandleFunc("GET /reports/profit-and-loss", s.profitAndLoss)
	mux.HandleFunc("GET /reports/balance-sheet", s.balanceSheet)

	mux.HandleFunc("GET /settings", s.settings)
	mux.HandleFunc("POST /settings/periods", s.createPeriod)
	mux.HandleFunc("POST /settings/vat-rates", s.createVATRate)

	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.FS(s.static))))

	return promoteIdempotencyKey(s.idempotency.Wrap(mux))
}

// promoteIdempotencyKey lifts a form's key into the request header.
//
// internal/idempotency reads the header, and a form cannot set one. The body is
// read and put back rather than consumed: the middleware hashes the body to tell
// one request from another, so a form whose bytes had already been swallowed
// would hash as empty and every write would look like a different request.
//
// A malformed body is passed through untouched. Whether it is a request the
// handler wants is the handler's judgement, and the middleware still guards it
// under whatever key the header already carried.
func promoteIdempotencyKey(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get(idempotency.Header) != "" || r.Body == nil || !isFormPost(r) {
			next.ServeHTTP(w, r)

			return
		}

		body, err := io.ReadAll(r.Body)
		if err != nil {
			next.ServeHTTP(w, r)

			return
		}

		_ = r.Body.Close()
		r.Body = io.NopCloser(bytes.NewReader(body))

		values, err := url.ParseQuery(string(body))
		if err != nil {
			next.ServeHTTP(w, r)

			return
		}

		if key := strings.TrimSpace(values.Get(keyField)); key != "" {
			r.Header.Set(idempotency.Header, key)
		}

		next.ServeHTTP(w, r)
	})
}

// isFormPost reports whether a request is a form submission.
func isFormPost(r *http.Request) bool {
	if r.Method != http.MethodPost {
		return false
	}

	return strings.HasPrefix(r.Header.Get("Content-Type"), "application/x-www-form-urlencoded")
}

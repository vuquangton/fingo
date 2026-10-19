package web_test

import (
	"crypto/rand"
	"fmt"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"

	"fingo/internal/auth"
	"fingo/internal/store"
	"fingo/internal/testdb"
	"fingo/internal/web"
)

type harness struct {
	server *httptest.Server
	client *http.Client
	pool   *pgxpool.Pool
}

func newHarness(t *testing.T) *harness {
	t.Helper()

	gin.SetMode(gin.TestMode)

	cfg := testdb.Load(t)
	pool := testdb.Open(t, cfg)

	st, err := store.Open(t.Context(), store.Config{DSN: cfg.DSN()})
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(st.Close)

	a := web.NewAuth(st)

	router := gin.New()
	router.GET("/", a.Home)
	router.GET("/login", a.ShowLogin)
	router.POST("/login", a.Login)
	router.POST("/logout", a.Logout)

	protected := router.Group("/", a.RequireUser())
	protected.POST("/clients/:id/select", a.SelectClient)
	protected.GET("/clients", a.ShowClients)
	protected.POST("/clients", a.RequireRole(web.RoleAdmin), a.PostClient)
	protected.GET("/users", a.RequireRole(web.RoleAdmin), a.ShowUsers)
	protected.POST("/users", a.RequireRole(web.RoleAdmin), a.PostUser)
	protected.POST("/password", a.PostPassword)
	protected.GET("/accounts", a.ShowAccounts)
	protected.GET("/reverse", a.ShowReverse)
	protected.POST("/reverse", a.RequireWriter(), a.PostReverse)
	protected.GET("/journal", a.ShowJournal)
	protected.GET("/stock", a.ShowStock)
	protected.POST("/stock/item", a.RequireWriter(), a.PostItem)
	protected.POST("/stock/receive", a.RequireWriter(), a.PostStockReceipt)
	protected.POST("/stock/count", a.RequireWriter(), a.PostStocktake)
	protected.GET("/payment", a.ShowPayments)
	protected.POST("/payment", a.RequireWriter(), a.PostPayment)
	protected.GET("/purchase", a.ShowPurchase)
	protected.POST("/purchase", a.RequireWriter(), a.PostPurchase)
	protected.GET("/handover", a.ExportHandover)
	protected.GET("/opening/stock", a.ShowOpeningStock)
	protected.POST("/opening/stock", a.RequireWriter(), a.PostOpeningStock)
	protected.GET("/opening", a.ShowOpening)
	protected.POST("/opening", a.RequireWriter(), a.PostOpening)
	protected.GET("/receipt", a.ShowReceipts)
	protected.POST("/receipt", a.RequireWriter(), a.PostReceipt)
	protected.GET("/sales", a.ShowSales)
	protected.POST("/sales", a.RequireWriter(), a.PostSales)
	protected.GET("/partners", a.ShowPartners)
	protected.POST("/partners", a.RequireWriter(), a.PostPartner)
	protected.GET("/close", a.ShowClose)
	protected.POST("/close", a.RequireRole(web.RoleAdmin), a.PostClose)
	protected.GET("/aging", a.ShowAging)
	protected.GET("/reports/vat", a.ShowVAT)
	protected.GET("/reports", a.ShowTrialBalance)
	protected.GET("/reports/pnl", a.ShowProfitAndLoss)
	protected.GET("/reports/balance-sheet", a.ShowBalanceSheet)
	protected.GET("/reports/export.xlsx", a.ExportTrialBalance)
	protected.GET("/ledger", a.ShowLedger)
	protected.GET("/entry", a.ShowEntry)
	protected.POST("/entry", a.RequireWriter(), a.PostEntry)
	router.StaticFS("/static", http.FS(web.Static()))

	admin := protected.Group("/admin", a.RequireRole(web.RoleAdmin))
	admin.POST("/periods/lock", func(c *gin.Context) { c.String(http.StatusOK, "locked") })

	server := httptest.NewTLSServer(a.Handler(router))
	t.Cleanup(server.Close)

	// A cookie jar is the test's memory of the session. Without one every
	// request is a new browser, which is exactly what an auth test must not be.
	client := server.Client()
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("cookie jar: %v", err)
	}
	client.Jar = jar

	return &harness{server: server, client: client, pool: pool}
}

func (h *harness) user(t *testing.T, email, password string, role web.Role) {
	t.Helper()

	hash, err := auth.Hash(password)
	if err != nil {
		t.Fatalf("hash: %v", err)
	}

	if _, err := h.pool.Exec(t.Context(),
		`INSERT INTO users (email, display_name, password_hash, role) VALUES ($1, $1, $2, $3)`,
		email, hash, string(role)); err != nil {
		t.Fatalf("insert user: %v", err)
	}
}

func (h *harness) post(t *testing.T, path string, form url.Values) *http.Response {
	t.Helper()

	resp, err := h.client.PostForm(h.server.URL+path, form)
	if err != nil {
		t.Fatalf("POST %s: %v", path, err)
	}
	t.Cleanup(func() { _ = resp.Body.Close() })
	return resp
}

func (h *harness) get(t *testing.T, path string) *http.Response {
	t.Helper()

	resp, err := h.client.Get(h.server.URL + path)
	if err != nil {
		t.Fatalf("GET %s: %v", path, err)
	}
	t.Cleanup(func() { _ = resp.Body.Close() })
	return resp
}

func TestUnauthenticatedRequestIsRejected(t *testing.T) {
	h := newHarness(t)

	if got := h.get(t, "/clients").StatusCode; got != http.StatusUnauthorized {
		t.Fatalf("GET /books without a session = %d, want 401", got)
	}
}

func TestLoginThenReachAProtectedRoute(t *testing.T) {
	h := newHarness(t)
	email := newEmail()
	h.user(t, email, "correct-horse", web.RoleAccountant)

	if got := h.post(t, "/login", url.Values{"email": {email}, "password": {"correct-horse"}}).StatusCode; got != http.StatusNoContent {
		t.Fatalf("login = %d, want 204", got)
	}

	if got := h.get(t, "/clients").StatusCode; got != http.StatusOK {
		t.Fatalf("GET /books with a session = %d, want 200", got)
	}
}

func TestWrongPasswordIsRefusedAndRecorded(t *testing.T) {
	h := newHarness(t)
	email := newEmail()
	h.user(t, email, "correct-horse", web.RoleAccountant)

	if got := h.post(t, "/login", url.Values{"email": {email}, "password": {"wrong"}}).StatusCode; got != http.StatusUnauthorized {
		t.Fatalf("wrong password = %d, want 401", got)
	}
	if got := h.get(t, "/clients").StatusCode; got != http.StatusUnauthorized {
		t.Fatalf("GET /books after a failed login = %d, want 401", got)
	}

	var failures int
	if err := h.pool.QueryRow(t.Context(),
		`SELECT count(*) FROM failed_logins WHERE email = $1`, email).Scan(&failures); err != nil {
		t.Fatalf("count failures: %v", err)
	}
	if failures != 1 {
		t.Fatalf("failed_logins rows = %d, want 1", failures)
	}
}

func TestUnknownEmailAndWrongPasswordAreIndistinguishable(t *testing.T) {
	h := newHarness(t)
	email := newEmail()
	h.user(t, email, "correct-horse", web.RoleAccountant)

	unknown := h.post(t, "/login", url.Values{"email": {newEmail()}, "password": {"x"}})
	wrong := h.post(t, "/login", url.Values{"email": {email}, "password": {"x"}})

	if unknown.StatusCode != wrong.StatusCode {
		t.Fatalf("unknown email = %d, wrong password = %d: the response distinguishes them",
			unknown.StatusCode, wrong.StatusCode)
	}
}

func TestRoleIsEnforced(t *testing.T) {
	h := newHarness(t)
	email := newEmail()
	h.user(t, email, "correct-horse", web.RoleViewer)

	if got := h.post(t, "/login", url.Values{"email": {email}, "password": {"correct-horse"}}).StatusCode; got != http.StatusNoContent {
		t.Fatalf("login = %d, want 204", got)
	}

	if got := h.get(t, "/clients").StatusCode; got != http.StatusOK {
		t.Fatalf("viewer reading /books = %d, want 200", got)
	}

	if got := h.post(t, "/admin/periods/lock", url.Values{}).StatusCode; got != http.StatusForbidden {
		t.Fatalf("viewer locking a period = %d, want 403", got)
	}
}

func TestLogoutEndsTheSession(t *testing.T) {
	h := newHarness(t)
	email := newEmail()
	h.user(t, email, "correct-horse", web.RoleAccountant)

	h.post(t, "/login", url.Values{"email": {email}, "password": {"correct-horse"}})
	if got := h.get(t, "/clients").StatusCode; got != http.StatusOK {
		t.Fatalf("before logout = %d, want 200", got)
	}

	if got := h.post(t, "/logout", url.Values{}).StatusCode; got != http.StatusNoContent {
		t.Fatalf("logout = %d, want 204", got)
	}

	if got := h.get(t, "/clients").StatusCode; got != http.StatusUnauthorized {
		t.Fatalf("after logout = %d, want 401", got)
	}
}

func TestMissingCredentialsAreRejected(t *testing.T) {
	h := newHarness(t)

	if got := h.post(t, "/login", url.Values{"email": {"only-an-email"}}).StatusCode; got != http.StatusBadRequest {
		t.Fatalf("login with no password = %d, want 400", got)
	}
}

func newEmail() string {
	var b [8]byte
	_, _ = rand.Read(b[:])
	return fmt.Sprintf("user-%x@example.test", b)
}

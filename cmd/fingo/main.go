// Command fingo is the FinGo v2 server.
//
// One binary: TLS termination, sessions, and the HTTP surface. It refuses to
// start without a certificate, because a Secure session cookie over plain HTTP
// is a cookie the browser will not send back — a login that silently fails.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"

	"fingo/internal/store"
	"fingo/internal/web"
)

func main() {
	if err := run(); err != nil {
		slog.Error("fingo", "error", err)
		os.Exit(1)
	}
}

func run() error {
	addr := flag.String("addr", envOr("FINGO_ADDR", "127.0.0.1:8443"), "listen address")
	certFile := flag.String("cert", envOr("FINGO_CERT", ".local/tls/cert.pem"), "TLS certificate")
	keyFile := flag.String("key", envOr("FINGO_KEY", ".local/tls/key.pem"), "TLS private key")
	flag.Parse()

	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	if _, err := os.Stat(*certFile); err != nil {
		return fmt.Errorf("no TLS certificate at %s: run `go run ./cmd/devcert`", *certFile)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	dsn := os.Getenv("FINGO_DSN")
	if dsn == "" {
		return errors.New("FINGO_DSN is not set")
	}

	st, err := store.Open(ctx, store.Config{DSN: dsn, MaxOpenConns: 16, ConnMaxLifetime: 30 * time.Minute})
	if err != nil {
		return err
	}
	defer st.Close()

	auth := web.NewAuth(st)

	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	router.Use(gin.Recovery())
	router.GET("/healthz", func(c *gin.Context) { c.String(http.StatusOK, "ok") })
	router.GET("/", auth.Home)
	router.GET("/login", auth.ShowLogin)
	router.POST("/login", auth.Login)
	router.POST("/logout", auth.Logout)

	protected := router.Group("/", auth.RequireUser())
	protected.GET("/clients", auth.ShowClients)
	protected.POST("/clients", auth.RequireRole(web.RoleAdmin), auth.PostClient)
	protected.POST("/clients/:id/select", auth.SelectClient)
	protected.GET("/users", auth.RequireRole(web.RoleAdmin), auth.ShowUsers)
	protected.POST("/users", auth.RequireRole(web.RoleAdmin), auth.PostUser)
	protected.POST("/password", auth.PostPassword)
	protected.GET("/accounts", auth.ShowAccounts)
	protected.GET("/reverse", auth.ShowReverse)
	protected.POST("/reverse", auth.RequireWriter(), auth.PostReverse)
	protected.GET("/journal", auth.ShowJournal)
	protected.GET("/stock", auth.ShowStock)
	protected.POST("/stock/item", auth.RequireWriter(), auth.PostItem)
	protected.POST("/stock/receive", auth.RequireWriter(), auth.PostStockReceipt)
	protected.POST("/stock/count", auth.RequireWriter(), auth.PostStocktake)
	protected.GET("/payment", auth.ShowPayments)
	protected.POST("/payment", auth.RequireWriter(), auth.PostPayment)
	protected.GET("/purchase", auth.ShowPurchase)
	protected.POST("/purchase", auth.RequireWriter(), auth.PostPurchase)
	protected.GET("/handover", auth.ExportHandover)
	protected.GET("/opening", auth.ShowOpening)
	protected.POST("/opening", auth.RequireWriter(), auth.PostOpening)
	protected.GET("/receipt", auth.ShowReceipts)
	protected.POST("/receipt", auth.RequireWriter(), auth.PostReceipt)
	protected.GET("/sales", auth.ShowSales)
	protected.POST("/sales", auth.RequireWriter(), auth.PostSales)
	protected.GET("/partners", auth.ShowPartners)
	protected.POST("/partners", auth.RequireWriter(), auth.PostPartner)
	protected.GET("/close", auth.ShowClose)
	protected.POST("/close", auth.RequireRole(web.RoleAdmin), auth.PostClose)
	protected.GET("/aging", auth.ShowAging)
	protected.GET("/reports/vat", auth.ShowVAT)
	protected.GET("/reports", auth.ShowTrialBalance)
	protected.GET("/reports/pnl", auth.ShowProfitAndLoss)
	protected.GET("/reports/balance-sheet", auth.ShowBalanceSheet)
	protected.GET("/reports/export.xlsx", auth.ExportTrialBalance)
	protected.GET("/ledger", auth.ShowLedger)
	protected.GET("/entry", auth.ShowEntry)
	protected.POST("/entry", auth.RequireWriter(), auth.PostEntry)

	router.StaticFS("/static", http.FS(web.Static()))

	srv := &http.Server{
		Addr:              *addr,
		Handler:           auth.Handler(router),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       2 * time.Minute,
		TLSConfig:         web.TLSConfig(),
	}

	errs := make(chan error, 1)
	go func() {
		slog.Info("listening", "addr", *addr)
		errs <- srv.ListenAndServeTLS(*certFile, *keyFile)
	}()

	select {
	case err := <-errs:
		if !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("serve: %w", err)
		}
	case <-ctx.Done():
		slog.Info("shutting down")
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("shutdown: %w", err)
	}
	return nil
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

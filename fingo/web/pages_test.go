package web_test

import (
	"io"
	"net/http"
	"strings"
	"testing"
)

// Every page renders, and every one of them renders a whole document when no
// script asked for anything less.
//
// A template that does not execute is a broken screen, and it fails at render
// time rather than at build time. This walks the lot, which is the only cheap
// way to catch the page no other test happens to load.
func TestE2E_EveryPageRendersWithoutScripts(t *testing.T) {
	server, client := stack(t)

	pages := []string{
		"/",
		"/documents",
		"/documents?type=invoice",
		"/documents?type=receipt",
		"/documents?type=journal",
		"/documents/new?type=invoice",
		"/documents/new?type=payment",
		"/documents/new?type=journal",
		"/accounts",
		"/accounts/new",
		"/reports/trial-balance",
		"/reports/profit-and-loss",
		"/reports/balance-sheet",
		"/settings",
	}

	for _, page := range pages {
		t.Run(page, func(t *testing.T) {
			status, body := get(t, client, server.URL+page)
			if status != http.StatusOK {
				t.Fatalf("%s answered %d:\n%s", page, status, body)
			}

			mustContain(t, body, "<!doctype html>")
			mustContain(t, body, `id="content"`)
			mustContain(t, body, "</html>")
		})
	}
}

// The stylesheet and the script are served from the binary, not from a disk path
// and not from a CDN. A desktop build may have no network, and a page whose
// stylesheet 404s still looks like a bug in the page.
func TestE2E_TheStaticAssetsAreServedFromTheBinary(t *testing.T) {
	server, client := stack(t)

	for _, asset := range []string{"/static/app.css", "/static/htmx.min.js"} {
		status, body := get(t, client, server.URL+asset)
		if status != http.StatusOK {
			t.Fatalf("%s answered %d", asset, status)
		}
		if len(body) == 0 {
			t.Fatalf("%s is empty", asset)
		}
	}
}

// With HTMX the same URL answers with the inner content, which is the whole of
// the progressive enhancement: the swap target is the element the full page also
// puts the content in.
func TestE2E_HTMXGetsTheFragmentAndNotTheDocument(t *testing.T) {
	server, client := stack(t)

	req, err := http.NewRequest(http.MethodGet, server.URL+"/accounts", nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("HX-Request", "true")

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("the fragment request failed: %v", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read the fragment: %v", err)
	}

	body := string(raw)

	if strings.Contains(body, "<!doctype html>") {
		t.Errorf("the fragment carries the whole document:\n%s", body)
	}
	if strings.Contains(body, "<nav>") {
		t.Errorf("the fragment carries the navigation:\n%s", body)
	}
	if !strings.Contains(body, "<h1>Chart of accounts</h1>") {
		t.Errorf("the fragment does not carry the page's own content:\n%s", body)
	}
}

// A document nobody has is a 404 and not a 500. The distinction matters to a
// client following a stale link, and it is the handler's job to make it.
func TestE2E_AnUnknownDocumentIsNotFound(t *testing.T) {
	server, client := stack(t)

	for _, target := range []string{"/documents/inv-9999", "/documents/inv-9999/pay"} {
		status, body := get(t, client, server.URL+target)
		if status != http.StatusNotFound {
			t.Fatalf("%s answered %d, want 404:\n%s", target, status, body)
		}
		mustContain(t, body, `data-error="1"`)
	}
}

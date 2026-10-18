package web

import (
	"bytes"
	"embed"
	"fmt"
	"html/template"
	"io/fs"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// assets holds the templates and the stylesheet. They are embedded so a
// deployed desktop application needs no loose files beside it — the same reason
// db.Migrations is embedded.
//
//go:embed templates/*.html static/*
var assets embed.FS

// page is what a template is executed with.
//
// Data is the page's own view model and stays `any`, because the alternative is
// a God struct holding every field of every screen. A page template reaches it
// with {{with .Data}} and works in its own type from there.
type page struct {
	Title string
	Nav   string
	// Content is the template name being rendered. It is set by the renderer
	// and never by a handler.
	Content string

	// Key is the idempotency key every form on this page carries.
	//
	// Minted per render and put in a hidden field, so a form submitted with
	// scripts disabled still carries a usable key. The page's script replaces
	// it with crypto.randomUUID() where it can, which is what makes two tabs of
	// the same form two requests rather than one replayed twice.
	Key string

	// Error is what the books said, shown to the operator unchanged. It is not
	// summarised and not replaced with a code: "voucher jnl-0001 debits 100.000
	// and credits 90.000" is a sentence somebody can act on.
	Error string

	// Flash is a note about something that just succeeded.
	Flash string

	Data any
}

// renderer holds the parsed templates.
type renderer struct {
	templates *template.Template

	// newKey mints the idempotency key a form is rendered with.
	newKey func() string
}

// newRenderer parses every template once, at construction.
//
// At construction, not per request: a template that does not parse is a bug in
// the deployment, and finding it on the first page load rather than on the first
// request to that page means a broken screen cannot ship.
func newRenderer(newKey func() string) (*renderer, error) {
	templates, err := template.New("web").Funcs(template.FuncMap{
		"num": func(a Amount) int64 { return a.Units },
		"dict": func(pairs ...any) map[string]any {
			out := make(map[string]any, len(pairs)/2)

			for i := 0; i+1 < len(pairs); i += 2 {
				key, ok := pairs[i].(string)
				if !ok {
					continue
				}

				out[key] = pairs[i+1]
			}

			return out
		},
	}).ParseFS(assets, "templates/*.html")
	if err != nil {
		return nil, fmt.Errorf("parse the page templates: %w", err)
	}

	return &renderer{templates: templates, newKey: newKey}, nil
}

// render writes a page, or the fragment HTMX asked for.
//
// The page is rendered into a buffer first. A template that fails half way
// through would otherwise have already written a 200 and half a document, and
// the browser would show a broken page with no hint that anything went wrong.
//
// Without JavaScript the handler is asked for the whole document and gets it.
// With HTMX the same URL answers with the inner content, which is the entire
// difference between the two modes — there is no second set of URLs and no
// second handler to keep in step.
func (r *renderer) render(w http.ResponseWriter, req *http.Request, status int, content string, model page) {
	model.Content = content

	// Every render carries a key, whether or not the page has a form. Minting
	// it here rather than in each handler is what stops a new form shipping
	// without one.
	if model.Key == "" && r.newKey != nil {
		model.Key = r.newKey()
	}

	name := content
	if isHTMX(req) {
		name = content + "_content"
	}

	var buf bytes.Buffer
	if err := r.templates.ExecuteTemplate(&buf, name, model); err != nil {
		http.Error(w, "the page could not be rendered: "+err.Error(), http.StatusInternalServerError)

		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)

	if _, err := w.Write(buf.Bytes()); err != nil {
		// The client has gone. Nothing to report and nowhere to report it.
		return
	}
}

// isHTMX reports whether the request came from HTMX.
func isHTMX(r *http.Request) bool { return r.Header.Get("HX-Request") == "true" }

// staticFS returns the embedded static assets.
func staticFS() (fs.FS, error) { return fs.Sub(assets, "static") }

// errForm is returned by a form parse that could not make sense of a field. Its
// text is shown to the operator, so it names the field and what was wrong with
// it rather than saying "invalid input".
type errForm struct{ msg string }

func (e errForm) Error() string { return e.msg }

func formError(format string, args ...any) error {
	return errForm{msg: fmt.Sprintf(format, args...)}
}

// formDate parses a date field.
func formDate(field, value string) (time.Time, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return time.Time{}, formError("%s is required", field)
	}

	parsed, err := time.Parse(time.DateOnly, trimmed)
	if err != nil {
		return time.Time{}, formError("%s must be a date like 2026-03-01, not %q", field, trimmed)
	}

	return parsed, nil
}

// formAmount parses a whole-đồng amount.
//
// Grouping is stripped before parsing, because the figure an operator reads off
// the screen is "1.234.567" and the figure they type back may be too. VND has no
// subunit, so a decimal point is never a decimal point here.
func formAmount(field, value string) (int64, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return 0, nil
	}

	clean := strings.NewReplacer(".", "", ",", "", " ", "", "\u00a0", "", "_", "").Replace(trimmed)
	if clean == "" {
		return 0, nil
	}

	amount, err := strconv.ParseInt(clean, 10, 64)
	if err != nil {
		return 0, formError("%s must be a whole number of đồng, not %q", field, trimmed)
	}

	return amount, nil
}

// formText trims a text field.
func formText(value string) string { return strings.TrimSpace(value) }

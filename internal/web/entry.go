// Entry screen: the first write path a person can actually use.
//
// It is deliberately plain server-rendered HTML — a form that posts, a page that
// comes back. htmx is loaded but not required; the keyboard grid (Tabulator) is a
// later, separate decision. Everything that decides whether a voucher is valid
// lives in the ledger service, not here: this file parses a form and reports what
// the ledger said.
package web

import (
	"embed"
	"errors"
	"fmt"
	"html/template"
	"io/fs"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"fingo/internal/domain/gl"
	"fingo/internal/format"
	"fingo/internal/ledger"
	"fingo/internal/money"
	"fingo/internal/store"
)

//go:embed templates/*.html
var templateFS embed.FS

//go:embed static
var staticFS embed.FS

// Static returns the embedded assets, rooted so a caller can serve them as-is.
func Static() fs.FS {
	sub, err := fs.Sub(staticFS, "static")
	if err != nil {
		panic("web: static assets are missing from the binary: " + err.Error())
	}
	return sub
}

var templates = template.Must(template.New("").Funcs(template.FuncMap{
	"money": format.Money,
}).ParseFS(templateFS, "templates/*.html"))

// entryRow is one line of the form as the operator typed it.
type entryRow struct {
	Account string
	Side    string
	Amount  string
}

type entryData struct {
	Email       string
	ClientName  string
	Posted      string
	Error       string
	Date        string
	Description string
	Accounts    []AccountOption
	Rows        []entryRow
}

// AccountOption is an account the operator can pick.
type AccountOption struct {
	Code string
	Name string
}

// ShowEntry renders the entry form.
func (a *Auth) ShowEntry(c *gin.Context) {
	scope, clientName, ok := a.requireScope(c)
	if !ok {
		return
	}

	accounts, err := a.accounts(c, scope)
	if err != nil {
		c.String(http.StatusInternalServerError, "không đọc được hệ thống tài khoản")
		return
	}

	email, _, _ := UserFrom(c)
	data := entryData{
		Email:      email,
		ClientName: clientName,
		Posted:     c.Query("posted"),
		Date:       time.Now().Format("2006-01-02"),
		Accounts:   accounts,
		Rows:       []entryRow{{Side: string(gl.SideDebit)}, {Side: string(gl.SideCredit)}, {Side: string(gl.SideDebit)}, {Side: string(gl.SideCredit)}},
	}
	// Placeholder actions are template-only; the default client name comes from
	// the registry row the scope points at.
	c.Status(http.StatusOK)
	if err := templates.ExecuteTemplate(c.Writer, "entry.html", data); err != nil {
		c.String(http.StatusInternalServerError, "không dựng được trang nhập liệu: %v", err)
	}
}

// PostEntry parses the form, posts through the ledger, and answers with the page.
func (a *Auth) PostEntry(c *gin.Context) {
	scope, clientName, ok := a.requireScope(c)
	if !ok {
		return
	}

	req, rows, err := parseEntry(c)
	if err != nil {
		a.renderEntryError(c, scope, clientName, rows, err)
		return
	}

	posted, err := ledger.Post(c.Request.Context(), a.store, scope, req)
	if err != nil {
		a.renderEntryError(c, scope, clientName, rows, err)
		return
	}

	c.Redirect(http.StatusSeeOther, "/entry?posted="+posted.VoucherNo)
}

func (a *Auth) renderEntryError(c *gin.Context, scope store.Scope, clientName string, rows []entryRow, cause error) {
	accounts, err := a.accounts(c, scope)
	if err != nil {
		c.String(http.StatusInternalServerError, "không đọc được hệ thống tài khoản")
		return
	}

	email, _, _ := UserFrom(c)
	data := entryData{
		Email:       email,
		ClientName:  clientName,
		Error:       cause.Error(),
		Date:        c.PostForm("date"),
		Description: c.PostForm("description"),
		Accounts:    accounts,
		Rows:        rows,
	}

	c.Status(http.StatusUnprocessableEntity)
	if err := templates.ExecuteTemplate(c.Writer, "entry.html", data); err != nil {
		c.String(http.StatusInternalServerError, "không dựng được trang nhập liệu: %v", err)
	}
}

func parseEntry(c *gin.Context) (ledger.Request, []entryRow, error) {
	date, err := time.Parse("2006-01-02", c.PostForm("date"))
	if err != nil {
		return ledger.Request{}, nil, fmt.Errorf("ngày chứng từ không hợp lệ: %w", err)
	}

	accounts := c.PostFormArray("account")
	sides := c.PostFormArray("side")
	amounts := c.PostFormArray("amount")

	if len(accounts) != len(sides) || len(accounts) != len(amounts) {
		return ledger.Request{}, nil, errors.New("form không khớp số dòng")
	}

	var (
		lines []ledger.Line
		rows  []entryRow
	)

	for i, account := range accounts {
		row := entryRow{Account: account, Side: sides[i], Amount: amounts[i]}
		rows = append(rows, row)

		if strings.TrimSpace(account) == "" && strings.TrimSpace(amounts[i]) == "" {
			continue // an untouched row
		}

		side := gl.EntrySide(strings.ToUpper(strings.TrimSpace(sides[i])))
		if side != gl.SideDebit && side != gl.SideCredit {
			return ledger.Request{}, rows, fmt.Errorf("dòng %d: chưa chọn bên Nợ/Có", i+1)
		}

		amount, err := format.Amount(amounts[i])
		if err != nil {
			return ledger.Request{}, rows, fmt.Errorf("dòng %d: %w", i+1, err)
		}
		if amount <= money.Zero {
			return ledger.Request{}, rows, fmt.Errorf("dòng %d: số tiền phải lớn hơn 0", i+1)
		}

		lines = append(lines, ledger.Line{Account: strings.TrimSpace(account), Side: side, Amount: amount})
	}

	if len(lines) < 2 {
		return ledger.Request{}, rows, errors.New("một chứng từ cần ít nhất hai dòng")
	}

	email, _, _ := UserFrom(c)

	return ledger.Request{
		ID:          uuid.NewString(),
		Description: strings.TrimSpace(c.PostForm("description")),
		Actor:       email,
		Date:        date.UTC(),
		Type:        gl.VoucherTypeGeneral,
		Lines:       lines,
	}, rows, nil
}

// accounts lists the postable accounts for the operator's picker.
func (a *Auth) accounts(c *gin.Context, scope store.Scope) ([]AccountOption, error) {
	var out []AccountOption

	err := a.store.Read(c.Request.Context(), scope, func(tx pgx.Tx) error {
		rows, err := tx.Query(c.Request.Context(), `
			SELECT a.code, a.name FROM accounts a
			 WHERE a.active AND NOT EXISTS (SELECT 1 FROM accounts child WHERE child.parent_code = a.code)
			 ORDER BY code`)
		if err != nil {
			return err
		}
		defer rows.Close()

		for rows.Next() {
			var o AccountOption
			if err := rows.Scan(&o.Code, &o.Name); err != nil {
				return err
			}
			out = append(out, o)
		}
		return rows.Err()
	})

	return out, err
}

// requireScope resolves the client a request is for, or answers the request and
// reports false.
//
// Selection is explicit: a session names its client. When there is exactly one
// active client — a fresh install — that is chosen and the session records it,
// because asking an operator to pick from a list of one is a step that teaches
// nothing.
func (a *Auth) requireScope(c *gin.Context) (store.Scope, string, bool) {
	id := a.ClientFrom(c)

	var name string
	ctx := c.Request.Context()

	if id == "" {
		err := a.pool.QueryRow(ctx,
			`SELECT id, name FROM (
					SELECT id, name, count(*) OVER () AS n
					  FROM clients WHERE active
				) AS only_one WHERE n = 1`).Scan(&id, &name)
		if errors.Is(err, pgx.ErrNoRows) {
			c.String(http.StatusConflict, "chưa có khách hàng nào")
			return store.Scope{}, "", false
		}
		if err != nil {
			c.String(http.StatusInternalServerError, "không đọc được danh sách khách hàng")
			return store.Scope{}, "", false
		}

		a.sessions.Put(ctx, keyClient, id)
	} else if err := a.pool.QueryRow(ctx,
		`SELECT name FROM clients WHERE id = $1 AND active`, id).Scan(&name); err != nil {
		// A client that was deactivated or removed while the session held it is
		// forgotten, not silently served.
		a.sessions.Remove(ctx, keyClient)
		c.Status(http.StatusConflict)
		return store.Scope{}, "", false
	}

	scope, err := store.NewScope(id)
	if err != nil {
		c.Status(http.StatusConflict)
		return store.Scope{}, "", false
	}
	return scope, name, true
}

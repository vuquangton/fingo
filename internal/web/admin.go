// Practice administration: the client registry and the people who may sign in.
//
// These are the two lists the application itself is made of. Without them a
// second client needs SQL and a second colleague needs a hand-built password
// hash, which is not a product.
package web

import (
	"errors"
	"net/http"
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"

	"fingo/internal/auth"
	"fingo/internal/books"
)

type clientRow struct {
	ID      string
	Name    string
	TaxID   string
	Active  bool
	Current bool
}

type clientsData struct {
	Email   string
	Rows    []clientRow
	Created string
	Error   string
}

// ShowClients lists the practice's clients and offers to add one.
func (a *Auth) ShowClients(c *gin.Context) {
	clients, err := books.ListClients(c.Request.Context(), a.store)
	if err != nil {
		a.renderClients(c, http.StatusInternalServerError, clientsData{Error: err.Error()})
		return
	}

	current := a.ClientFrom(c)
	rows := make([]clientRow, 0, len(clients))
	for _, cl := range clients {
		rows = append(rows, clientRow{
			ID: cl.ID, Name: cl.Name, TaxID: cl.TaxID,
			Active: cl.Active, Current: cl.ID == current,
		})
	}

	email, _, _ := UserFrom(c)
	a.renderClients(c, http.StatusOK, clientsData{
		Email:   email,
		Rows:    rows,
		Created: c.Query("created"),
	})
}

// PostClient registers a client and provisions its chart.
func (a *Auth) PostClient(c *gin.Context) {
	client, err := books.CreateClient(c.Request.Context(), a.store,
		c.PostForm("name"), c.PostForm("tax_id"))
	if err != nil {
		a.renderClients(c, http.StatusUnprocessableEntity, clientsData{Error: err.Error()})
		return
	}

	c.Redirect(http.StatusSeeOther, "/clients?created="+url.QueryEscape(client.Name))
}

type userRow struct {
	Email       string
	DisplayName string
	Role        string
	Active      bool
	Current     bool
}

type usersData struct {
	Email   string
	Rows    []userRow
	Created string
	Changed string
	Error   string
}

// ShowUsers lists the accounts. Admin only.
func (a *Auth) ShowUsers(c *gin.Context) {
	users, err := auth.ListUsers(c.Request.Context(), a.pool)
	if err != nil {
		a.renderUsers(c, http.StatusInternalServerError, usersData{Error: err.Error()})
		return
	}

	me, _, _ := UserFrom(c)
	rows := make([]userRow, 0, len(users))
	for _, u := range users {
		rows = append(rows, userRow{
			Email: u.Email, DisplayName: u.DisplayName, Role: u.Role,
			Active: u.Active, Current: u.Email == me,
		})
	}

	a.renderUsers(c, http.StatusOK, usersData{
		Email:   me,
		Rows:    rows,
		Created: c.Query("created"),
		Changed: c.Query("changed"),
	})
}

// PostUser creates an account. Admin only.
func (a *Auth) PostUser(c *gin.Context) {
	user, err := auth.CreateUser(c.Request.Context(), a.pool,
		c.PostForm("email"), c.PostForm("display_name"), c.PostForm("role"), c.PostForm("password"))
	if err != nil {
		a.renderUsers(c, http.StatusUnprocessableEntity, usersData{Error: err.Error()})
		return
	}

	c.Redirect(http.StatusSeeOther, "/users?created="+user.Email)
}

// PostPassword changes the signed-in user's own password. It is deliberately not
// an admin action: a session must not be enough to take over another account.
func (a *Auth) PostPassword(c *gin.Context) {
	email, _, _ := UserFrom(c)

	err := auth.ChangePassword(c.Request.Context(), a.pool, email,
		c.PostForm("current"), c.PostForm("next"))
	if err != nil {
		status := http.StatusUnprocessableEntity
		if errors.Is(err, auth.ErrMismatch) {
			status = http.StatusForbidden
		}
		a.renderUsers(c, status, usersData{Email: email, Error: err.Error()})
		return
	}

	// The users page is admin-only, so an accountant who changes their own
	// password must not be redirected into a 403. They are told it worked.
	if _, role, _ := UserFrom(c); role == RoleAdmin {
		c.Redirect(http.StatusSeeOther, "/users?changed=1")
		return
	}

	c.String(http.StatusOK, "Đã đổi mật khẩu.")
}

func (a *Auth) renderClients(c *gin.Context, status int, data clientsData) {
	c.Status(status)
	if err := templates.ExecuteTemplate(c.Writer, "clients.html", data); err != nil {
		c.String(http.StatusInternalServerError, "không dựng được trang khách hàng: %v", err)
	}
}

func (a *Auth) renderUsers(c *gin.Context, status int, data usersData) {
	// The list is context, even on the error path.
	if data.Rows == nil {
		if users, err := auth.ListUsers(c.Request.Context(), a.pool); err == nil {
			me, _, _ := UserFrom(c)
			for _, u := range users {
				data.Rows = append(data.Rows, userRow{
					Email: u.Email, DisplayName: u.DisplayName, Role: u.Role,
					Active: u.Active, Current: u.Email == me,
				})
			}
		}
	}
	if data.Email == "" {
		data.Email, _, _ = UserFrom(c)
	}

	c.Status(status)
	if err := templates.ExecuteTemplate(c.Writer, "users.html", data); err != nil {
		c.String(http.StatusInternalServerError, "không dựng được trang người dùng: %v", err)
	}
}

var _ = strings.TrimSpace

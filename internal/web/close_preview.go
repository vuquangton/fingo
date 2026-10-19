// Two-step period close: preview the journal, then confirm with a typed period and
// a one-time token tied to the previewed figures.
package web

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"fingo/internal/books"
	"fingo/internal/close"
	"fingo/internal/format"
)

const (
	keyCloseToken  = "close_token"
	keyCloseDigest = "close_digest"
	keyClosePeriod = "close_period"
)

type closePreviewData struct {
	Email       string
	Client      string
	PeriodID    string
	PeriodLabel string
	Token       string
	Result      string
	Rows        []closePreviewRow
	Error       string
}

type closePreviewRow struct {
	Account string
	Side    string
	Amount  string
}

// PreviewClose shows what closing the period would post, without posting it.
func (a *Auth) PreviewClose(c *gin.Context) {
	scope, clientName, ok := a.requireScope(c)
	if !ok {
		return
	}

	periodID := c.PostForm("period_id")
	preview, err := books.Open(a.store, scope).PreviewClose(c.Request.Context(), periodID)
	if err != nil {
		a.renderClosePreview(c, http.StatusUnprocessableEntity, closePreviewData{Client: clientName, Error: err.Error()})
		return
	}

	token := newToken()
	a.sessions.Put(c.Request.Context(), keyCloseToken, token)
	a.sessions.Put(c.Request.Context(), keyCloseDigest, digestOf(preview))
	a.sessions.Put(c.Request.Context(), keyClosePeriod, preview.PeriodID)

	email, _, _ := UserFrom(c)
	data := closePreviewData{
		Email:       email,
		Client:      clientName,
		PeriodID:    preview.PeriodID,
		PeriodLabel: preview.PeriodLabel,
		Token:       token,
		Result:      format.Money(preview.Result),
	}
	for _, line := range preview.Lines {
		data.Rows = append(data.Rows, closePreviewRow{
			Account: line.AccountCode,
			Side:    string(line.Side),
			Amount:  format.Money(line.Amount),
		})
	}
	a.renderClosePreview(c, http.StatusOK, data)
}

// ConfirmClose posts the close only if the typed period matches and the figures are
// exactly the ones that were previewed.
func (a *Auth) ConfirmClose(c *gin.Context) {
	scope, clientName, ok := a.requireScope(c)
	if !ok {
		return
	}

	ctx := c.Request.Context()
	expectedToken := a.sessions.GetString(ctx, keyCloseToken)
	if expectedToken == "" || c.PostForm("token") != expectedToken {
		a.renderClosePreview(c, http.StatusConflict, closePreviewData{Client: clientName, Error: "phiên xem trước đã hết hạn: hãy xem trước lại."})
		return
	}

	periodID := a.sessions.GetString(ctx, keyClosePeriod)
	if periodID == "" || c.PostForm("period_id") != periodID {
		a.renderClosePreview(c, http.StatusConflict, closePreviewData{Client: clientName, Error: "kỳ đã thay đổi: hãy xem trước lại."})
		return
	}

	b := books.Open(a.store, scope)

	preview, err := b.PreviewClose(ctx, periodID)
	if err != nil {
		a.renderClosePreview(c, http.StatusUnprocessableEntity, closePreviewData{Client: clientName, Error: err.Error()})
		return
	}

	// Data changed between preview and confirm: the operator would be confirming
	// figures they never saw.
	if digestOf(preview) != a.sessions.GetString(ctx, keyCloseDigest) {
		a.renderClosePreview(c, http.StatusConflict, closePreviewData{Client: clientName, Error: "số liệu đã thay đổi kể từ lần xem trước: hãy xem trước lại."})
		return
	}

	// Typing the period is the deliberate act; a checkbox is too easy to click.
	if strings.TrimSpace(c.PostForm("confirm")) != preview.PeriodLabel {
		a.renderClosePreview(c, http.StatusUnprocessableEntity, closePreviewData{
			Client: clientName, PeriodID: preview.PeriodID, PeriodLabel: preview.PeriodLabel,
			Token: expectedToken, Result: format.Money(preview.Result),
			Error: fmt.Sprintf("hãy gõ đúng kỳ %q để xác nhận", preview.PeriodLabel),
		})
		return
	}

	email, _, _ := UserFrom(c)

	if _, err := b.ClosePeriod(ctx, close.Command{
		PeriodID:  periodID,
		VoucherID: uuid.NewString(),
		Operator:  email,
		At:        time.Now(),
	}); err != nil {
		a.renderClosePreview(c, http.StatusUnprocessableEntity, closePreviewData{
			Client: clientName, PeriodID: preview.PeriodID, PeriodLabel: preview.PeriodLabel,
			Token: expectedToken, Result: format.Money(preview.Result), Error: err.Error(),
		})
		return
	}

	// One-time: the token cannot be replayed even if the browser resubmits.
	a.sessions.Remove(ctx, keyCloseToken)
	a.sessions.Remove(ctx, keyCloseDigest)
	a.sessions.Remove(ctx, keyClosePeriod)

	c.Redirect(http.StatusSeeOther, "/close?posted="+preview.PeriodLabel)
}

func (a *Auth) renderClosePreview(c *gin.Context, status int, data closePreviewData) {
	if data.Email == "" {
		data.Email, _, _ = UserFrom(c)
	}
	c.Status(status)
	if err := templates.ExecuteTemplate(c.Writer, "close_preview.html", data); err != nil {
		c.String(http.StatusInternalServerError, "không dựng được trang xem trước khoá sổ: %v", err)
	}
}

func newToken() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic("web: crypto/rand failed: " + err.Error())
	}
	return hex.EncodeToString(b[:])
}

func digestOf(p books.ClosePreview) string {
	h := sha256.New()
	fmt.Fprintf(h, "%s|%d|", p.PeriodID, p.Result.Int64())
	for _, line := range p.Lines {
		fmt.Fprintf(h, "%s|%s|%d;", line.AccountCode, line.Side, line.Amount.Int64())
	}
	return hex.EncodeToString(h.Sum(nil))
}

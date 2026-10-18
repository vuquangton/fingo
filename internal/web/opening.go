// Opening balances: import a client's starting trial balance from Excel.
//
// It posts one voucher. The engine validates the chart, the balance and the
// period; this file reads a spreadsheet and builds the lines. A second import of
// the same file is refused because the voucher id is derived from the file's
// content, so the primary keys collide instead of doubling the books.
package web

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/xuri/excelize/v2"

	"fingo/internal/books"
	"fingo/internal/domain/gl"
	"fingo/internal/format"
	"fingo/internal/ledger"
	"fingo/internal/money"
)

type openingRowError struct {
	Row     int
	Message string
}

type openingData struct {
	Email  string
	Client string
	Date   string
	Posted string
	Error  string
	Rows   []openingRowError
}

// ShowOpening renders the import form.
func (a *Auth) ShowOpening(c *gin.Context) {
	_, clientName, ok := a.requireScope(c)
	if !ok {
		return
	}

	email, _, _ := UserFrom(c)
	a.renderOpening(c, http.StatusOK, openingData{
		Email:  email,
		Client: clientName,
		Date:   time.Date(time.Now().Year(), 1, 1, 0, 0, 0, 0, time.UTC).Format("2006-01-02"),
		Posted: c.Query("posted"),
	})
}

// PostOpening reads the workbook and posts the opening entry.
func (a *Auth) PostOpening(c *gin.Context) {
	scope, clientName, ok := a.requireScope(c)
	if !ok {
		return
	}

	date, err := time.Parse("2006-01-02", c.PostForm("date"))
	if err != nil {
		a.renderOpening(c, http.StatusUnprocessableEntity, openingData{Client: clientName, Error: "ngày mở sổ không hợp lệ"})
		return
	}

	fileHeader, err := c.FormFile("file")
	if err != nil {
		a.renderOpening(c, http.StatusUnprocessableEntity, openingData{Client: clientName, Error: "chưa chọn tệp Excel"})
		return
	}

	f, err := fileHeader.Open()
	if err != nil {
		a.renderOpening(c, http.StatusUnprocessableEntity, openingData{Client: clientName, Error: "không mở được tệp"})
		return
	}
	defer func() { _ = f.Close() }()

	raw, err := io.ReadAll(io.LimitReader(f, 8<<20))
	if err != nil {
		a.renderOpening(c, http.StatusUnprocessableEntity, openingData{Client: clientName, Error: "không đọc được tệp"})
		return
	}

	lines, rowErrors, err := parseOpeningWorkbook(raw)
	if err != nil {
		a.renderOpening(c, http.StatusUnprocessableEntity, openingData{Client: clientName, Error: err.Error()})
		return
	}
	if len(rowErrors) > 0 {
		a.renderOpening(c, http.StatusUnprocessableEntity, openingData{
			Client: clientName,
			Error:  "tệp có dòng không đọc được",
			Rows:   rowErrors,
		})
		return
	}

	b := books.Open(a.store, scope)

	if _, err := b.EnsureCurrentYear(c.Request.Context(), date); err != nil {
		a.renderOpening(c, http.StatusInternalServerError, openingData{Client: clientName, Error: err.Error()})
		return
	}

	email, _, _ := UserFrom(c)

	posted, err := ledger.Post(c.Request.Context(), a.store, scope, ledger.Request{
		ID:          derivedFromFile(scope.ID(), raw),
		Description: "Số dư đầu kỳ",
		Actor:       email,
		Date:        date.UTC(),
		Type:        gl.VoucherTypeGeneral,
		Lines:       lines,
	})
	if err != nil {
		a.renderOpening(c, http.StatusUnprocessableEntity, openingData{
			Client: clientName,
			Error:  "không ghi sổ được số dư đầu kỳ: " + err.Error(),
		})
		return
	}

	c.Redirect(http.StatusSeeOther, "/opening?posted="+posted.VoucherNo)
}

// parseOpeningWorkbook reads code | debit | credit rows from the first sheet.
//
// The first row is a header when its first cell is not an account number, so a
// file with headings and a file without both work.
func parseOpeningWorkbook(raw []byte) ([]ledger.Line, []openingRowError, error) {
	book, err := excelize.OpenReader(bytes.NewReader(raw))
	if err != nil {
		return nil, nil, fmt.Errorf("tệp không phải Excel hợp lệ: %w", err)
	}
	defer func() { _ = book.Close() }()

	sheets := book.GetSheetList()
	if len(sheets) == 0 {
		return nil, nil, fmt.Errorf("tệp không có sheet nào")
	}

	rows, err := book.GetRows(sheets[0])
	if err != nil {
		return nil, nil, fmt.Errorf("không đọc được sheet: %w", err)
	}

	var (
		lines []ledger.Line
		bad   []openingRowError
	)

	for i, row := range rows {
		if len(row) == 0 {
			continue
		}

		code := strings.TrimSpace(row[0])
		if code == "" {
			continue
		}
		if i == 0 && !isAccountCode(code) {
			continue // a heading row
		}

		debit, credit, err := openingAmounts(row)
		if err != nil {
			bad = append(bad, openingRowError{Row: i + 1, Message: err.Error()})
			continue
		}
		if debit.IsZero() && credit.IsZero() {
			continue
		}

		if !debit.IsZero() {
			lines = append(lines, ledger.Line{Account: code, Side: gl.SideDebit, Amount: debit})
		}
		if !credit.IsZero() {
			lines = append(lines, ledger.Line{Account: code, Side: gl.SideCredit, Amount: credit})
		}
	}

	if len(lines) < 2 {
		return nil, bad, fmt.Errorf("tệp không có dòng số dư nào")
	}

	return lines, bad, nil
}

func openingAmounts(row []string) (money.Dong, money.Dong, error) {
	cell := func(i int) string {
		if i < len(row) {
			return strings.TrimSpace(row[i])
		}
		return ""
	}

	debitText, creditText := cell(1), cell(2)
	if creditText == "" && len(row) >= 2 {
		// Two-column files are common: code | amount, read as a debit.
		debitText = cell(1)
	}

	var debit, credit money.Dong

	if debitText != "" {
		v, err := format.Amount(debitText)
		if err != nil {
			return 0, 0, fmt.Errorf("số dư Nợ không đọc được: %w", err)
		}
		debit = v
	}

	if creditText != "" {
		v, err := format.Amount(creditText)
		if err != nil {
			return 0, 0, fmt.Errorf("số dư Có không đọc được: %w", err)
		}
		credit = v
	}

	if !debit.IsZero() && !credit.IsZero() {
		return 0, 0, fmt.Errorf("một dòng không thể vừa Nợ vừa Có")
	}

	return debit, credit, nil
}

func isAccountCode(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// derivedFromFile ties the voucher to the workbook: the same file imported twice
// is the same voucher, and the second attempt fails at the primary key.
func derivedFromFile(clientID string, raw []byte) string {
	sum := sha256.Sum256(append([]byte(clientID+"|"), raw...))

	var b [16]byte
	copy(b[:], sum[:16])
	b[6] = (b[6] & 0x0f) | 0x50
	b[8] = (b[8] & 0x3f) | 0x80

	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

func (a *Auth) renderOpening(c *gin.Context, status int, data openingData) {
	c.Status(status)
	if err := templates.ExecuteTemplate(c.Writer, "opening.html", data); err != nil {
		c.String(http.StatusInternalServerError, "không dựng được trang số dư đầu kỳ: %v", err)
	}
}

package web_test

import (
	"bytes"
	"mime/multipart"
	"net/http"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/xuri/excelize/v2"
)

// openingWorkbook builds the spreadsheet an accountant would send.
func openingWorkbook(t *testing.T, rows [][]any) []byte {
	t.Helper()

	f := excelize.NewFile()
	const sheet = "Sheet1"

	if err := f.SetSheetRow(sheet, "A1", &[]any{"Tài khoản", "Nợ", "Có"}); err != nil {
		t.Fatalf("header: %v", err)
	}

	for i, row := range rows {
		cell := "A" + itoa(i+2)
		if err := f.SetSheetRow(sheet, cell, &row); err != nil {
			t.Fatalf("row %d: %v", i+2, err)
		}
	}

	var buf bytes.Buffer
	if err := f.Write(&buf); err != nil {
		t.Fatalf("write workbook: %v", err)
	}
	return buf.Bytes()
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var digits []byte
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	return string(digits)
}

func (h *harness) uploadOpening(t *testing.T, workbook []byte, date string) *http.Response {
	t.Helper()

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)

	if err := writer.WriteField("date", date); err != nil {
		t.Fatalf("field date: %v", err)
	}

	part, err := writer.CreateFormFile("file", "opening.xlsx")
	if err != nil {
		t.Fatalf("file part: %v", err)
	}
	if _, err := part.Write(workbook); err != nil {
		t.Fatalf("write file: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close writer: %v", err)
	}

	req, err := http.NewRequest(http.MethodPost, h.server.URL+"/opening", &body)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := h.client.Do(req)
	if err != nil {
		t.Fatalf("upload: %v", err)
	}
	return resp
}

func TestOpeningBalancesImport(t *testing.T) {
	h := newHarness(t)
	email := newEmail()
	h.user(t, email, "correct-horse", "accountant")
	h.login(t, email, "correct-horse")
	client := h.newClient(t)
	h.pickClient(t, client)

	workbook := openingWorkbook(t, [][]any{
		{"1111", 500_000_000, 0},
		{"131", 300_000_000, 0},
		{"331", 0, 200_000_000},
		{"4111", 0, 600_000_000},
	})

	resp := h.uploadOpening(t, workbook, "2026-01-01")
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("opening import = %d\n%s", resp.StatusCode, readAll(t, resp))
	}

	body := readAll(t, resp)
	if !strings.Contains(body, "Đã ghi sổ số dư đầu kỳ") {
		t.Fatalf("the success notice is missing:\n%s", body)
	}

	st := h.storeOf(t)
	scope := h.scopeOf(t, client)

	var lines int
	var debit, credit int64
	if err := st.InClient(t.Context(), scope, func(tx pgx.Tx) error {
		if err := tx.QueryRow(t.Context(), `SELECT count(*) FROM voucher_lines`).Scan(&lines); err != nil {
			return err
		}
		return tx.QueryRow(t.Context(), `
			SELECT COALESCE(SUM(CASE WHEN side = 'DEBIT' THEN amount END), 0),
			       COALESCE(SUM(CASE WHEN side = 'CREDIT' THEN amount END), 0)
			  FROM voucher_lines`).Scan(&debit, &credit)
	}); err != nil {
		t.Fatalf("read the ledger: %v", err)
	}

	if lines != 4 {
		t.Fatalf("opening lines = %d, want 4", lines)
	}
	if debit != credit || debit != 800_000_000 {
		t.Fatalf("debit %d / credit %d, want 800,000,000 each", debit, credit)
	}
}

func TestOpeningBalancesRefuseAnUnbalancedFile(t *testing.T) {
	h := newHarness(t)
	email := newEmail()
	h.user(t, email, "correct-horse", "accountant")
	h.login(t, email, "correct-horse")
	client := h.newClient(t)
	h.pickClient(t, client)

	workbook := openingWorkbook(t, [][]any{
		{"1111", 500_000_000, 0},
		{"4111", 0, 400_000_000},
	})

	resp := h.uploadOpening(t, workbook, "2026-01-01")
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("unbalanced file = %d, want 422", resp.StatusCode)
	}

	var lines int
	if err := h.storeOf(t).InClient(t.Context(), h.scopeOf(t, client), func(tx pgx.Tx) error {
		return tx.QueryRow(t.Context(), `SELECT count(*) FROM voucher_lines`).Scan(&lines)
	}); err != nil {
		t.Fatalf("count: %v", err)
	}
	if lines != 0 {
		t.Fatalf("a refused import wrote %d lines", lines)
	}
}

func TestOpeningBalancesRefuseTheSameFileTwice(t *testing.T) {
	h := newHarness(t)
	email := newEmail()
	h.user(t, email, "correct-horse", "accountant")
	h.login(t, email, "correct-horse")
	client := h.newClient(t)
	h.pickClient(t, client)

	workbook := openingWorkbook(t, [][]any{
		{"1111", 500_000_000, 0},
		{"4111", 0, 500_000_000},
	})

	first := h.uploadOpening(t, workbook, "2026-01-01")
	_ = first.Body.Close()
	if first.StatusCode != http.StatusOK {
		t.Fatalf("first import = %d", first.StatusCode)
	}

	second := h.uploadOpening(t, workbook, "2026-01-01")
	defer func() { _ = second.Body.Close() }()

	if second.StatusCode == http.StatusOK {
		t.Fatal("the same workbook was imported twice")
	}

	var vouchers int
	if err := h.storeOf(t).InClient(t.Context(), h.scopeOf(t, client), func(tx pgx.Tx) error {
		return tx.QueryRow(t.Context(), `SELECT count(*) FROM vouchers`).Scan(&vouchers)
	}); err != nil {
		t.Fatalf("count: %v", err)
	}
	if vouchers != 1 {
		t.Fatalf("vouchers = %d, want 1: the import doubled the books", vouchers)
	}
}

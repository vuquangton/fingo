package web_test

import (
	"bytes"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/xuri/excelize/v2"
)

func TestProfitAndLossScreen(t *testing.T) {
	h := newHarness(t)
	email := newEmail()
	h.user(t, email, "correct-horse", "accountant")
	h.login(t, email, "correct-horse")
	h.pickClient(t, h.newClient(t))
	h.postSale(t)

	body := h.body(t, "/reports/pnl")

	for _, want := range []string{"Kết quả hoạt động kinh doanh", "Doanh thu bán hàng", "1.000.000", "Lợi nhuận"} {
		if !strings.Contains(body, want) {
			t.Fatalf("the P&L does not contain %q\n%s", want, body)
		}
	}
}

func TestBalanceSheetScreenBalances(t *testing.T) {
	h := newHarness(t)
	email := newEmail()
	h.user(t, email, "correct-horse", "accountant")
	h.login(t, email, "correct-horse")
	h.pickClient(t, h.newClient(t))
	h.postSale(t)

	body := h.body(t, "/reports/balance-sheet")

	for _, want := range []string{"Bảng cân đối kế toán", "Tổng tài sản", "Tổng nguồn vốn"} {
		if !strings.Contains(body, want) && !strings.Contains(body, "Tổng vốn chủ sở hữu") {
			t.Fatalf("the balance sheet does not contain %q\n%s", want, body)
		}
	}

	if strings.Contains(body, "KHÔNG cân") {
		t.Fatal("the balance sheet reports itself unbalanced after one balanced voucher")
	}
}

func TestLedgerScreenDrillsIntoAnAccount(t *testing.T) {
	h := newHarness(t)
	email := newEmail()
	h.user(t, email, "correct-horse", "accountant")
	h.login(t, email, "correct-horse")
	h.pickClient(t, h.newClient(t))
	h.postSale(t)

	body := h.body(t, "/ledger?account=131")

	for _, want := range []string{"Sổ chi tiết tài khoản 131", "1.100.000"} {
		if !strings.Contains(body, want) {
			t.Fatalf("the ledger does not contain %q\n%s", want, body)
		}
	}
}

func TestTrialBalanceExportsAnExcelWorkbook(t *testing.T) {
	h := newHarness(t)
	email := newEmail()
	h.user(t, email, "correct-horse", "accountant")
	h.login(t, email, "correct-horse")
	h.pickClient(t, h.newClient(t))
	h.postSale(t)

	resp := h.get(t, "/reports/export.xlsx")
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(resp.Body)
		t.Fatalf("export = %d\n%s", resp.StatusCode, raw)
	}

	if ct := resp.Header.Get("Content-Type"); !strings.Contains(ct, "spreadsheetml") {
		t.Fatalf("Content-Type = %q, want an xlsx mime type", ct)
	}

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read the workbook: %v", err)
	}

	book, err := excelize.OpenReader(bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("open the workbook: %v", err)
	}
	defer func() { _ = book.Close() }()

	rows, err := book.GetRows("CanDoi")
	if err != nil {
		t.Fatalf("read the sheet: %v", err)
	}
	if len(rows) < 3 {
		t.Fatalf("the sheet has %d rows, want a header, data and a total", len(rows))
	}

	if rows[0][0] != "Tài khoản" {
		t.Fatalf("first cell = %q, want the account heading", rows[0][0])
	}

	var sawAccount bool
	for _, row := range rows {
		if len(row) > 0 && row[0] == "131" {
			sawAccount = true
			if row[6] != "1100000" {
				t.Fatalf("account 131 closing debit = %q, want 1100000", row[6])
			}
		}
	}
	if !sawAccount {
		t.Fatalf("the workbook has no row for account 131:\n%v", rows)
	}
}

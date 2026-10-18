# domain-glossary — Vietnamese accounting terms as used in this code

> Hand-maintained. Add a row when a term starts appearing in code or UI copy.
> Code identifiers are authoritative; the Vietnamese is what an operator sees.

| Vietnamese | Code / English | Where |
| :--- | :--- | :--- |
| đồng (VNĐ) | `money.Dong` — `int64`, whole đồng, no subunit | `internal/money/dong.go` |
| Nợ / Có | `DEBIT` / `CREDIT` (`gl.EntrySide`) | `internal/domain/gl/gl.go:20-30` |
| tài khoản | account (`gl.Account`), code + name + type + nature | `internal/domain/gl/account.go:62` |
| hệ thống tài khoản | chart of accounts (`gl.ChartOfAccounts`) | `account.go:72` |
| TK | tài khoản kế toán (account number), e.g. TK 131 | UI copy |
| chứng từ | voucher (`gl.Voucher`) | `gl.go:56` |
| bút toán | journal entry / accounting entry | UI, tests |
| dòng bút toán | voucher line (`gl.VoucherLine`) | `gl.go:33` |
| sổ nhật ký chung | general journal (`GET /journal`) | `internal/web/screens.go` |
| sổ cái / sổ chi tiết | ledger / account ledger (`GET /ledger`) | `internal/web/screens.go` |
| bảng cân đối kế toán | balance sheet | `internal/reports/balancesheet.go` |
| báo cáo kết quả kinh doanh | profit and loss | `internal/reports/profitandloss.go` |
| bảng cân đối số phát sinh | trial balance | `internal/reports/trialbalance.go` |
| kỳ kế toán | accounting period (a month; UTC boundaries) | `internal/domain/gl/periods.go` |
| khoá sổ | period close / lock (`POST /close`) | `internal/close` |
| số dư đầu kỳ | opening balance (`GET/POST /opening`) | `internal/web/opening.go` |
| đảo bút toán | voucher reversal (`POST /reverse`) | `internal/books/reverse.go` |
| phải thu khách hàng | accounts receivable, TK 131 | `documents`, `aging` |
| phải trả người bán | accounts payable, TK 331 | `documents`, `aging` |
| tuổi nợ | AR/AP aging (`GET /aging`) | `internal/books/aging.go` |
| giá vốn hàng bán | cost of goods sold, TK 632 | `internal/inventory/cogs.go` |
| hàng tồn kho | inventory, TK 156 | `internal/inventory` |
| bình quân gia quyền | moving average (valuation method) | `inventory_policies` |
| kiểm kê | stocktake (`POST /stock/count`) | `internal/books/stocktake.go` |
| hóa đơn | invoice (`documents.kind = SALES_INVOICE` / `PURCHASE_BILL`) | DB |
| phiếu thu / phiếu chi | cash receipt / cash payment (books `PT` / `PC`) | `internal/domain/gl/numbering.go` |
| thuế GTGT | VAT — TK 1331 (input), 33311 (output) | `internal/vat` |
| khấu trừ | deduction / credit (input VAT) | UI |
| xác định kết quả kinh doanh | result determination, TK 911 | `internal/close` |
| lợi nhuận sau thuế chưa phân phối | retained earnings, TK 4212 (current) / 4211 (prior) | `close.TT133ClosingAccounts()` |
| dự phòng | provision (bad debt, inventory write-down) | `internal/close/provisions.go` — **not wired** |
| mã số chỉ tiêu | statutory form line code — **out of v2 scope** | `docs/legal-basis.md` |
| chế độ kế toán | accounting regime (TT133 / TT99 / TT58 / IFRS) | `legal-status.md` |

## Naming traps

- **"document"** means two different types: `internal/document.Document` (AR/AP
  ledger record) and `internal/domain/document.Document` (posting-path record).
- **"close"** is both the act (`close.ClosePeriod`) and the package that owns it.
- **"period"** is a month in `gl.Periods` but a free date range in
  `accounting_periods` (start/end dates, one per year by default).
- **"receipt"** is a cash receipt voucher (`CASH_RECEIPT`) and also a goods
  receipt in `inventory`/`trading.Purchasing`.

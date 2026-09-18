# MISA SME / AMIS Keyboard Workflow Smoke Test Checklist

Standard operating test procedure for validating keyboard ergonomics and fast-entry patterns in Accounting WPF.

## 1. Global Navigation Shortcuts

| Shortcut | Expected Action | Status |
|---|---|---|
| `F1` | Open In-App Documentation / Context Help | Ready |
| `F2` | New Voucher Quick Launch (Chứng từ Nghiệp vụ khác / Phiếu mới) | Ready |
| `F3` | Quick Search across Partners / Accounts / Vouchers | Ready |
| `F5` | Refresh current active ledger / report workspace | Ready |
| `F8` | Delete current selected line in DataGrid | Ready |
| `F9` | Post / Unpost current Voucher | Ready |
| `F12` | Open Master Data Manager (Danh mục) | Ready |
| `Ctrl + S` | Save current open voucher / master data record | Ready |
| `Ctrl + P` | Print Voucher preview (Mẫu 01-TT, 02-TT, Mẫu BCTC) | Ready |
| `Ctrl + W` | Close current active tab | Ready |
| `Ctrl + Tab` | Switch between open MDI workspace tabs | Ready |

## 2. In-Grid Keyboard Ergonomics (DataGrid)

1. **Enter Navigation**: Pressing `Enter` commits cell and moves focus to next cell on the right.
2. **Auto-Row Append**: Pressing `Enter` on the last cell of the last row automatically appends a new voucher line.
3. **Fuzzy Lookup**: Typing account prefix (e.g. `111` or `samsung`) opens the fuzzy dropdown without requiring mouse clicks.
4. **Auto-Contra Account Pairing**: In standard VAS vouchers, filling Account Debit automatically selects default reciprocal Credit from preset mapping.

## 3. End-to-End Accounting Verification Matrix

- **Opening Balances (SDĐK)**: Total Debit == Total Credit.
- **Procure-to-Pay**: PO -> GRN -> Purchase Invoice (TK 1561/1331/331) -> Bank Disbursement (TK 331/1121).
- **Order-to-Cash**: Sales Order -> Sales Invoice + Delivery (TK 131/5111/33311 & TK 632/1561) -> Bank Receipt (TK 1121/131).
- **Treasury (Quỹ & Ngân hàng)**: Strictly enforces Anti-Negative Cash rule (Balance >= 0).
- **Closing & Balance Sheet**: Revenue/Expense accounts cleared to 911; Net Profit transferred to 4212; Balance Sheet equation holds Assets == Liabilities + Equity.

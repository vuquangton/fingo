# Page catalogue — FinGo

**What this is.** The screen inventory a Vietnamese SME accounting product has to
have, organised the way the market leader organises it, with each page marked
against what this repository's engine can actually back today.

**What this is not.** A specification. Field-level rules, validation and posting
behaviour belong in the per-module specs; this document decides *which pages
exist*, *what each one answers*, and *what order to build them in*.

**Prepared as:** lead BA + chief accountant. **Reviewed against:**
[reference-misa-sme-flow.md](reference-misa-sme-flow.md) (MISA SME 2026),
[legal-basis.md](legal-basis.md) (confidence-marked), [plan-native-shell.md](plan-native-shell.md) (phases).

**Legal caution.** Where a form name, report code or regime fact is asserted here,
it is asserted at the confidence [legal-basis.md](legal-basis.md) records. Two items
are explicitly **not primary-verified** and are marked inline: the 01/GTGT
indicator numbers under TT 89/2026, and whether TT 46/2025 renamed the B01-DN
family for the TT133 regime.

---

## 0. How to read a row

| Column | Meaning |
| :--- | :--- |
| **Mã** | stable page id. Used in the rail, in tests, in bug reports. Never reused |
| **Trang** | the Vietnamese name the operator sees |
| **Kiểu** | `DS` danh sách · `CT` chứng từ (a document you post) · `SO` sổ/register · `BC` báo cáo · `DM` danh mục (master record) · `RUN` a batch run · `SYS` system |
| **Trả lời** | the question the operator opens it to answer. If you cannot write this, the page should not exist |
| **Máy** | `✓` engine backs it · `~` partial or unwired · `·` nothing |
| **Pha** | the plan phase that builds it |

**`✓` is a claim about this repository, not about ambition.** It was checked against
[plan-native-shell.md](plan-native-shell.md) §P1–P5, which records which packages have
callers outside tests.

---

## 1. Two axes, and MISA's ordering wins

MISA's primary navigation is **cadence**, not module: *Khởi tạo → Hàng ngày →
Hàng tháng/quý → Hàng năm*. The twenty business modules are secondary — they are
where a daily entry is made, not how the work is organised.

FinGo's rail already reads this way. `HÔM NAY` sits above `NGHIỆP VỤ`. **This
catalogue keeps that**, and the cadence pages (§3, §8) are load-bearing rather
than decorative: they are the only place the closing *order* is enforced, and the
closing order is where silent errors live.

The second axis — the modules — exists so an operator who knows they need to
write a phiếu chi can find it without reading a calendar.

---

## 2. Khởi tạo — the one-off

Not a rail section. Reached from `HỆ THỐNG` after the first run, and re-enterable
only while no vouchers exist.

| Mã | Trang | Kiểu | Trả lời | Máy | Pha |
| :--- | :--- | :--- | :--- | :--- | :--- |
| `init.company` | Thông tin doanh nghiệp | `DM` | Tên nào in trên chứng từ? | ✓ | done |
| `init.policy` | Chính sách kế toán | `SYS` | Sổ này giữ theo chế độ nào? | ~ | P1 |
| `init.chart` | Hệ thống tài khoản ban đầu | `DM` | Mở sổ bằng bộ tài khoản nào? | ✓ | P1 |
| `init.opening` | Số dư ban đầu | `RUN` | Đầu kỳ còn nợ ai, ai nợ mình? | ~ | P2 |
| `init.convert99` | Chuyển đổi theo TT 99/2025 | `RUN` | Đổi chế độ thì sổ cũ đi đâu? | · | P5 |

`init.policy` captures five **binding** decisions — not preferences. MISA asks
them at step 4–7 of its wizard and then holds the operator to them:

1. Lĩnh vực hoạt động + which modules are in use
2. Chế độ kế toán (`TT133` as amended by TT 46/2025 · `TT99/2025` · micro `TT58/2026`)
3. Phương pháp tính giá xuất kho — one of four: bình quân cuối kỳ · bình quân tức thời ·
   nhập trước xuất trước · đích danh
4. Phương pháp tính thuế GTGT — khấu trừ · trực tiếp trên doanh thu
5. Năm tài chính, loại tiền hạch toán, chi nhánh

**`init.opening` is ten screens, not one grid.** This is the single largest
correction MISA's flow makes to our design:

| # | Nhóm | Tài khoản kiểm soát |
| ---: | :--- | :--- |
| 1 | Tiền mặt | 111 |
| 2 | Tiền gửi ngân hàng | 112 |
| 3 | Phải thu khách hàng | 131 |
| 4 | Trả trước cho người bán | 331 |
| 5 | Nguyên liệu, công cụ, hàng hóa | 152, 153, 156 |
| 6 | Chi phí sản xuất kinh doanh dở dang | 154 |
| 7 | Công cụ dụng cụ (phân bổ) | 242 |
| 8 | Tài sản cố định | 211, 214 |
| 9 | Chi phí trả trước | 242 |
| 10 | Tài khoản khác | the catch-all, ties to 0 |

Each screen has to tie to its control account. A single trial-balance grid lets
the operator balance to zero while every sub-ledger is wrong — which is the
failure `opening.ValidateBalance` currently cannot detect, because it checks only
that account totals balance.

---

## 3. HÔM NAY — the work-cycle home

| Mã | Trang | Kiểu | Trả lời | Máy | Pha |
| :--- | :--- | :--- | :--- | :--- | :--- |
| `today.home` | Hôm nay | `BC` | Sổ đang ở đâu, còn việc gì? | ~ | P1 |
| `today.tasks` | Việc còn lại của kỳ | `DS` | Tháng này còn thiếu bước nào? | · | P3 |
| `today.recent` | Chứng từ vừa nhập | `DS` | Vừa làm gì? | ~ | P1 |

`today.home` carries four position figures — tiền, phải thu, phải trả, tồn kho —
plus the cadence checklist. It is the answer to "what do I do now", and it is the
page that makes the cadence axis real rather than a heading.

---

## 4. NGHIỆP VỤ — the entry modules

### 4.1 Quỹ

| Mã | Trang | Kiểu | Trả lời | Máy | Pha |
| :--- | :--- | :--- | :--- | :--- | :--- |
| `cash.receipt.list` | Phiếu thu | `DS` | Đã thu những gì? | · | P2 |
| `cash.receipt.form` | Phiếu thu (nhập) | `CT` | Thu của ai, bao nhiêu, vì sao? | · | P2 |
| `cash.payment.list` | Phiếu chi | `DS` | Đã chi những gì? | · | P2 |
| `cash.payment.form` | Phiếu chi (nhập) | `CT` | Chi cho ai, bao nhiêu, vì sao? | · | P2 |
| `cash.book` | Sổ quỹ tiền mặt | `SO` | Quỹ còn bao nhiêu, theo dòng nào? | · | P2 |
| `cash.stocktake` | Kiểm kê quỹ | `CT` | Thực tế có khớp sổ không? | · | P3 |
| `cash.transfer` | Thu chi tiền mặt | `RUN` | Chuyển giữa các quỹ | · | P5 |

### 4.2 Ngân hàng

| Mã | Trang | Kiểu | Trả lời | Máy | Pha |
| :--- | :--- | :--- | :--- | :--- | :--- |
| `bank.receipt.list` | Giấy báo Có | `DS` | Tiền vào tài khoản nào? | · | P2 |
| `bank.payment.list` | Giấy báo Nợ / Ủy nhiệm chi | `DS` | Tiền ra vì việc gì? | · | P2 |
| `bank.book` | Sổ tiền gửi ngân hàng | `SO` | Tài khoản còn bao nhiêu? | · | P2 |
| `bank.reconcile` | Đối chiếu sổ phụ ngân hàng | `RUN` | Sổ mình và sổ ngân hàng lệch đâu? | · | P3 |
| `bank.fx` | Tỷ giá xuất quỹ | `RUN` | Ngoại tệ xuất tính theo giá nào? | · | P3 |

### 4.3 Mua hàng

| Mã | Trang | Kiểu | Trả lời | Máy | Pha |
| :--- | :--- | :--- | :--- | :--- | :--- |
| `purchase.order` | Đơn mua hàng | `DS` | Đã đặt gì, nhận chưa? | · | P2 |
| `purchase.invoice.list` | Hóa đơn mua hàng | `DS` | Đã mua gì, từ ai? | ~ | P2 |
| `purchase.invoice.form` | Hóa đơn mua hàng (nhập) | `CT` | Mua gì, thuế bao nhiêu, ai giao? | ~ | P2 |
| `purchase.service` | Hóa đơn mua dịch vụ | `CT` | Mua dịch vụ, không nhập kho | ~ | P2 |
| `purchase.return` | Trả lại hàng mua | `CT` | Trả lại bao nhiêu, giảm nợ thế nào? | · | P3 |
| `purchase.discount` | Giảm giá hàng mua | `CT` | Giảm giá có hóa đơn điều chỉnh | · | P3 |
| `purchase.payable` | Công nợ phải trả | `BC` | Còn nợ nhà cung cấp nào? | ~ | P2 |
| `purchase.statement` | Đối chiếu công nợ NCC | `BC` | Số mình và số NCC có khớp? | · | P3 |

### 4.4 Bán hàng

| Mã | Trang | Kiểu | Trả lời | Máy | Pha |
| :--- | :--- | :--- | :--- | :--- | :--- |
| `sales.quote` | Báo giá | `DS` | Đã chào giá gì? | · | P5 |
| `sales.order` | Đơn đặt hàng | `DS` | Khách đặt gì, giao chưa? | · | P3 |
| `sales.invoice.list` | Hóa đơn bán hàng | `DS` | Đã bán gì, cho ai? | ~ | P1 |
| `sales.invoice.form` | Hóa đơn bán hàng (nhập) | `CT` | Bán gì, giá vốn bao nhiêu? | ✓ | P1 |
| `sales.service` | Hóa đơn bán dịch vụ | `CT` | Bán dịch vụ, không xuất kho | ~ | P4 |
| `sales.return` | Hàng bán bị trả lại | `CT` | Trả lại bao nhiêu, nhập kho lại | · | P3 |
| `sales.discount` | Giảm giá hàng bán | `CT` | Giảm giá sau khi bán | · | P3 |
| `sales.receivable` | Công nợ phải thu | `BC` | Khách nào còn nợ, quá hạn bao lâu? | ~ | P2 |
| `sales.statement` | Đối chiếu công nợ KH | `BC` | Số mình và số khách khớp chưa? | · | P3 |
| `sales.delivery` | Phiếu xuất kho | `CT` | Giao hàng, giá vốn tính thế nào? | ~ | P2 |

`sales.invoice.form` is the one page in this catalogue the engine fully backs,
**including COGS** — `usecase/sales` is the only sales path in the repository that
posts 632. Three other paths do not, and that inconsistency is recorded in
[accounting-review.md](accounting-review.md).

### 4.5 Hóa đơn điện tử

Regime: **TT 91/2026/TT-BTC** (replaces TT 32/2025, which replaced TT 78/2021).

| Mã | Trang | Kiểu | Trả lời | Máy | Pha |
| :--- | :--- | :--- | :--- | :--- | :--- |
| `einvoice.issue` | Phát hành hóa đơn điện tử | `RUN` | Đẩy hóa đơn lên nhà cung cấp | · | P5 |
| `einvoice.list` | Danh sách hóa đơn điện tử | `DS` | Hóa đơn nào đã phát hành, trạng thái? | · | P5 |
| `einvoice.adjust` | Điều chỉnh / thay thế | `CT` | Sai thì điều chỉnh thế nào? | · | P5 |
| `einvoice.summary` | Bảng tổng hợp hóa đơn điện tử | `BC` | Kỳ này phát hành bao nhiêu? | · | P5 |
| `einvoice.manage` | Quản lý hóa đơn đầu vào | `DS` | Hóa đơn mua vào đã lấy chưa? | · | P5 |

### 4.6 Kho

| Mã | Trang | Kiểu | Trả lời | Máy | Pha |
| :--- | :--- | :--- | :--- | :--- | :--- |
| `stock.in.list` | Phiếu nhập kho | `DS` | Nhập gì, từ đâu? | ~ | P2 |
| `stock.in.form` | Phiếu nhập kho (nhập) | `CT` | Nhập bao nhiêu, đơn giá nào? | ~ | P2 |
| `stock.out.list` | Phiếu xuất kho | `DS` | Xuất gì, cho ai? | ~ | P2 |
| `stock.out.form` | Phiếu xuất kho (nhập) | `CT` | Xuất bao nhiêu, giá vốn nào? | ~ | P2 |
| `stock.transfer` | Phiếu chuyển kho | `CT` | Chuyển giữa hai kho | · | P3 |
| `stock.stocktake` | Kiểm kê kho | `CT` | Thực tế khớp sổ chưa? | · | P3 |
| `stock.costing` | Tính giá xuất kho | `RUN` | Kỳ này xuất theo giá nào? | · | P3 |
| `stock.ledger` | Sổ chi tiết vật tư hàng hóa | `SO` | Một mặt hàng đi qua những gì? | ~ | P3 |
| `stock.summary` | Bảng nhập — xuất — tồn | `BC` | Kỳ này nhập xuất tồn bao nhiêu? | ~ | P3 |

`stock.costing` must run **before** the P&L transfer and before the tax return.
Its method is fixed at `init.policy` and cannot be changed mid-year.

### 4.7 Công cụ dụng cụ

| Mã | Trang | Kiểu | Trả lời | Máy | Pha |
| :--- | :--- | :--- | :--- | :--- | :--- |
| `tools.in` | Ghi tăng công cụ dụng cụ | `CT` | Mua CCDC, phân bổ mấy kỳ? | · | P3 |
| `tools.allocate` | Phân bổ công cụ dụng cụ | `RUN` | Kỳ này phân bổ bao nhiêu vào 242? | · | P3 |
| `tools.out` | Ghi giảm công cụ dụng cụ | `CT` | Thanh lý, mất, chuyển TSCĐ | · | P3 |
| `tools.ledger` | Sổ chi tiết công cụ dụng cụ | `SO` | CCDC nào còn giá trị? | · | P3 |
| `tools.summary` | Bảng phân bổ công cụ dụng cụ | `BC` | Kỳ này phân bổ vào đâu? | · | P3 |

### 4.8 Tài sản cố định

| Mã | Trang | Kiểu | Trả lời | Máy | Pha |
| :--- | :--- | :--- | :--- | :--- | :--- |
| `asset.in` | Ghi tăng TSCĐ | `CT` | Mua TSCĐ, khấu hao bao nhiêu năm? | · | P3 |
| `asset.out` | Ghi giảm TSCĐ | `CT` | Thanh lý, nhượng bán | · | P3 |
| `asset.depreciate` | Tính khấu hao TSCĐ | `RUN` | Kỳ này trích khấu hao bao nhiêu? | · | P3 |
| `asset.adjust` | Điều chỉnh TSCĐ | `CT` | Đánh giá lại, nâng cấp | · | P5 |
| `asset.summary` | Bảng tính khấu hao | `BC` | Mỗi tài sản trích bao nhiêu? | · | P3 |
| `asset.ledger` | Sổ tài sản cố định | `SO` | Nguyên giá, hao mòn, còn lại | · | P3 |

### 4.9 Tiền lương

| Mã | Trang | Kiểu | Trả lời | Máy | Pha |
| :--- | :--- | :--- | :--- | :--- | :--- |
| `payroll.employee` | Hồ sơ nhân viên | `DM` | Ai làm, mã số thuế, người phụ thuộc? | · | P4 |
| `payroll.timesheet` | Bảng chấm công | `DS` | Tháng này ai làm bao nhiêu ngày? | · | P4 |
| `payroll.calc` | Tính lương | `RUN` | Lương gross, bảo hiểm, thuế, net | · | P4 |
| `payroll.sheet` | Bảng lương | `BC` | Tháng này trả ai bao nhiêu? | · | P4 |
| `payroll.insurance` | Bảo hiểm xã hội, y tế, thất nghiệp | `BC` | Phần công ty và phần người lao động | · | P4 |
| `payroll.pit` | Thuế thu nhập cá nhân | `BC` | Giảm trừ, biểu lũy tiến, khấu trừ | · | P4 |
| `payroll.pay` | Trả lương | `CT` | Trả qua tiền mặt hay chuyển khoản? | · | P4 |

PIT parameters changed materially: the new law raises the personal deduction to
**15,500,000** and the dependent deduction to **6,200,000**, and cuts the brackets
from **7 to 5**. The bracket article number is still an open question in
[legal-basis.md](legal-basis.md).

### 4.10 Thuế

| Mã | Trang | Kiểu | Trả lời | Máy | Pha |
| :--- | :--- | :--- | :--- | :--- | :--- |
| `tax.vat.return` | Tờ khai thuế GTGT — 01/GTGT | `BC` | Phải nộp bao nhiêu, khấu trừ bao nhiêu? | · | P4 |
| `tax.vat.input` | Bảng kê hóa đơn mua vào — 01-1 | `BC` | Đầu vào kỳ này gồm những gì? | · | P4 |
| `tax.vat.output` | Bảng kê hóa đơn bán ra — 01-2 | `BC` | Đầu ra kỳ này gồm những gì? | · | P4 |
| `tax.vat.deduction` | Khấu trừ thuế GTGT | `RUN` | Kết chuyển 1331 → 3331 | · | P3 |
| `tax.vat.reduce8` | Giảm thuế GTGT 8% — NQ 204/2025 | `BC` | Phần giảm tách riêng thế nào? | · | P4 |
| `tax.cit.provisional` | Tạm tính thuế TNDN — 03/TNDN | `BC` | Quý này tạm nộp bao nhiêu? | · | P4 |
| `tax.cit.final` | Quyết toán thuế TNDN | `BC` | Cả năm phải nộp bao nhiêu? | · | P5 |
| `tax.pit.return` | Tờ khai thuế TNCN | `BC` | Khấu trừ của người lao động | · | P4 |
| `tax.contractor` | Thuế nhà thầu nước ngoài | `BC` | Trả cho nước ngoài thì nộp gì? | · | P5 |
| `tax.license` | Thuế môn bài | `BC` | Nộp theo vốn điều lệ | · | P5 |
| `tax.payment` | Giấy nộp tiền vào ngân sách | `CT` | Đã nộp chưa, nộp bao nhiêu? | · | P4 |

**Two cautions, both carried from [legal-basis.md](legal-basis.md):**

- The **8% reduction is not a rate line on 01/GTGT.** It is a separate appendix,
  *GIẢM THUẾ GTGT THEO NGHỊ QUYẾT 204/2025/QH15*, mẫu 01, indicators [01]–[09].
  The window runs to **31/12/2026**. A page that models it as a rate will be wrong.
- The 01/GTGT indicator numbers under **TT 89/2026** are **not primary-verified** —
  Phụ lục I could not be read. `tax.vat.return` cannot be built until the mã số
  chỉ tiêu are confirmed. **This is the open question the rail already names**:
  `Thuế — chờ chốt mã số chỉ tiêu`.

### 4.11 Tổng hợp

| Mã | Trang | Kiểu | Trả lời | Máy | Pha |
| :--- | :--- | :--- | :--- | :--- | :--- |
| `gl.journal.list` | Chứng từ nghiệp vụ khác | `DS` | Những bút toán không thuộc module nào | ✓ | P1 |
| `gl.journal.form` | Chứng từ nghiệp vụ khác (nhập) | `CT` | Định khoản Nợ/Có tay | ✓ | P1 |
| `gl.batch` | Nhập, sửa nhiều chứng từ | `DS` | Sửa hàng loạt | · | P5 |
| `gl.recurring` | Bút toán định kỳ | `DM` | Bút toán lặp hằng tháng | · | P5 |

---

## 5. SỔ & BÁO CÁO — the books

| Mã | Trang | Kiểu | Trả lời | Máy | Pha |
| :--- | :--- | :--- | :--- | :--- | :--- |
| `book.journal` | Sổ nhật ký chung | `SO` | Mọi bút toán, theo thứ tự thời gian | ✓ | P1 |
| `book.journal.cash` | Sổ nhật ký thu tiền / chi tiền | `SO` | Dòng tiền theo ngày | ~ | P3 |
| `book.journal.sales` | Sổ nhật ký bán hàng | `SO` | Doanh thu theo ngày | ~ | P3 |
| `book.journal.purchase` | Sổ nhật ký mua hàng | `SO` | Mua vào theo ngày | ~ | P3 |
| `book.ledger` | Sổ cái | `SO` | Một tài khoản, cả kỳ | ✓ | P1 |
| `book.ledger.detail` | Sổ chi tiết tài khoản | `SO` | Một tài khoản, chi tiết theo đối tượng | ✓ | P1 |
| `book.ledger.partner` | Sổ chi tiết công nợ | `SO` | Một khách/NCC đi qua những gì? | ~ | P3 |
| `book.trial` | Bảng cân đối số phát sinh | `BC` | Sổ có cân không? | ✓ | P1 |
| `book.account.detail` | Sổ chi tiết theo tài khoản và đối tượng | `SO` | truy vết hai chiều | ~ | P3 |

**Traceability rule.** Every figure on `book.trial` must be clickable through to
the vouchers that produced it. [plan-native-shell.md](plan-native-shell.md) §P1 states
this as the acceptance condition for the trial balance, and it is the difference
between a report and a filing instrument.

---

## 6. Báo cáo tài chính

Under the TT133 regime. **Flagged uncertainty:** the B01-DN family was renamed to
*Báo cáo tình hình tài chính* in the TT 99/2025 material; whether TT 46/2025 made
the same change for TT133 is **not verified**. Treat the names below as provisional
and confirm before printing.

| Mã | Trang | Kiểu | Trả lời | Máy | Pha |
| :--- | :--- | :--- | :--- | :--- | :--- |
| `fs.b01` | Báo cáo tình hình tài chính (B01-DN) | `BC` | Có gì, nợ bao nhiêu? | ~ | P4 |
| `fs.b02` | Báo cáo kết quả hoạt động kinh doanh (B02-DN) | `BC` | Lãi hay lỗ? | ~ | P4 |
| `fs.b03` | Báo cáo lưu chuyển tiền tệ (B03-DN) | `BC` | Tiền đi đâu? | ~ | P4 |
| `fs.b09` | Bản thuyết minh báo cáo tài chính (B09-DN) | `BC` | Giải thích các chỉ tiêu | · | P4 |

The 200/2014 family contains further variants — non-going-concern `B01-DNKLT`…
`B09-DNKLT`, interim full `B01a-DN`…, interim condensed `B01b-DN`… — and whether
any of them apply under TT133 is part of the same unverified item.

---

## 7. Báo cáo quản trị

| Mã | Trang | Kiểu | Trả lời | Máy | Pha |
| :--- | :--- | :--- | :--- | :--- | :--- |
| `mgmt.income` | Báo cáo kết quả kinh doanh nội bộ | `BC` | Lãi lỗ theo bộ phận, mặt hàng | ~ | P5 |
| `mgmt.receivable` | Phân tích tuổi nợ phải thu | `BC` | Nợ nào quá hạn, bao lâu? | ~ | P3 |
| `mgmt.payable` | Phân tích tuổi nợ phải trả | `BC` | Đến hạn trả ai? | ~ | P3 |
| `mgmt.stock` | Báo cáo tồn kho | `BC` | Mặt hàng nào chậm luân chuyển? | ~ | P3 |
| `mgmt.cost` | Phân tích chi phí và giá thành | `BC` | Giá thành đơn vị gồm những gì? | · | P5 |
| `mgmt.sales` | Báo cáo bán hàng | `BC` | Bán cho ai, mặt hàng nào chạy? | ~ | P3 |
| `mgmt.cashflow` | Báo cáo dòng tiền quản trị | `BC` | Tiền vào ra theo tuần | · | P5 |

---

## 8. Kết chuyển và khóa sổ

This is the section where **order is the product**. MISA documents eight runs that
must complete **before** the P&L transfer and **before** the tax return. A closing
entry computed before depreciation and inventory costing have run is wrong, and it
is wrong silently.

| # | Mã | Trang | Kiểu | Máy | Pha |
| ---: | :--- | :--- | :--- | :--- | :--- |
| 1 | `close.allocate.tools` | Phân bổ công cụ dụng cụ | `RUN` | · | P3 |
| 2 | `close.allocate.prepaid` | Phân bổ chi phí trả trước | `RUN` | · | P3 |
| 3 | `close.depreciate` | Tính khấu hao TSCĐ | `RUN` | · | P3 |
| 4 | `close.cost.stock` | Tính giá xuất kho | `RUN` | · | P3 |
| 5 | `close.fx` | Tính tỷ giá xuất quỹ | `RUN` | · | P3 |
| 6 | `close.payroll` | Tính lương và hạch toán chi phí lương | `RUN` | · | P4 |
| 7 | `close.cost.production` | Tính giá thành sản xuất | `RUN` | · | P5 |
| 8 | `close.vat` | Khấu trừ thuế GTGT | `RUN` | · | P3 |
| — | `close.transfer` | Kết chuyển lãi lỗ | `RUN` | ~ | P3 |
| — | `close.cit.provisional` | Tạm tính thuế TNDN | `RUN` | · | P4 |
| — | `close.reconcile` | Kiểm tra, đối chiếu chứng từ, sổ sách | `RUN` | · | P3 |
| — | `close.print` | In sổ sách, chứng từ | `RUN` | · | P4 |
| — | `close.period` | Khóa sổ kỳ kế toán | `SYS` | ~ | P3 |

`close.transfer` exists in `internal/close` and transfers 911→4212, but it has **no
caller outside tests** and none of runs 1–8 exist. Until they do, that transfer is
the silent error described above. **This is the largest correctness gap in the
product**, and it is a P3 item rather than a P1 one only because nothing posts a
P&L yet.

**Year end, in order:** kiểm kê → đánh giá lại ngoại tệ → kiểm tra đối chiếu →
tính thuế TNDN → lập và kiểm tra BCTC → **nộp** báo cáo → **khóa sổ**. File, then
lock. The lock is irreversible and it is last.

---

## 9. DANH MỤC

| Mã | Trang | Kiểu | Trả lời | Máy | Pha |
| :--- | :--- | :--- | :--- | :--- | :--- |
| `md.account` | Hệ thống tài khoản | `DM` | Bộ tài khoản đang dùng | ✓ | P1 |
| `md.customer` | Khách hàng | `DM` | Bán cho ai | ✓ | P1 |
| `md.supplier` | Nhà cung cấp | `DM` | Mua của ai | ✓ | P1 |
| `md.employee` | Nhân viên | `DM` | Ai làm việc | · | P4 |
| `md.item` | Vật tư, hàng hóa | `DM` | Bán và mua cái gì | ~ | P2 |
| `md.warehouse` | Kho | `DM` | Hàng để ở đâu | ~ | P2 |
| `md.tools` | Loại công cụ dụng cụ | `DM` | Phân bổ mấy kỳ | · | P3 |
| `md.asset` | Loại tài sản cố định | `DM` | Khấu hao bao nhiêu năm | · | P3 |
| `md.department` | Bộ phận, phòng ban | `DM` | Chi phí thuộc bộ phận nào | ~ | P2 |
| `md.bank` | Ngân hàng | `DM` | Tài khoản ngân hàng của mình | · | P2 |
| `md.currency` | Loại tiền và tỷ giá | `DM` | Ngoại tệ quy đổi thế nào | ~ | P4 |
| `md.unit` | Đơn vị tính | `DM` | Cái, hộp, kg — và quy đổi | ~ | P2 |
| `md.taxrate` | Thuế suất | `DM` | 0 / 5 / 8 / 10 | ~ | P4 |
| `md.contract` | Hợp đồng | `DM` | Hợp đồng nào còn hiệu lực | · | P5 |
| `md.stat` | Mã thống kê | `DM` | Báo cáo theo chiều tự định nghĩa | · | P5 |
| `md.term` | Điều khoản thanh toán | `DM` | Bao lâu thì đến hạn | · | P5 |

---

## 10. HỆ THỐNG

| Mã | Trang | Kiểu | Trả lời | Máy | Pha |
| :--- | :--- | :--- | :--- | :--- | :--- |
| `sys.client` | Khách hàng (sổ của ai) | `DS` | Đang mở sổ của doanh nghiệp nào | ~ | P4 |
| `sys.period` | Kỳ kế toán | `SYS` | Kỳ nào đang mở, kỳ nào đã khóa | ~ | P3 |
| `sys.user` | Người dùng và phân quyền | `SYS` | Ai được làm gì | · | P4 |
| `sys.audit` | Nhật ký truy cập | `DS` | Ai đã sửa cái gì | ~ | P4 |
| `sys.options` | Tùy chọn | `SYS` | Mặc định của phần mềm | ~ | P3 |
| `sys.backup` | Sao lưu và phục hồi | `SYS` | Mất máy thì sổ đi đâu | · | P3 |
| `sys.diagnostics` | Chẩn đoán | `SYS` | Máy chủ dữ liệu có khỏe không | ~ | P3 |

`sys.user` matters more than it looks. **Điều 3 of TT 99/2025 makes internal control
an explicit obligation**, and this repository currently has **no authentication, no
segregation of duties, no approval workflow and no persisted audit trail**. Under a
TT133 target that obligation's applicability needs confirming — but the direction of
travel in the regulation is not toward less control.

---

## 11. Counts, and what is not here

| | Pages |
| :--- | ---: |
| Khởi tạo | 5 |
| Hôm nay | 3 |
| Nghiệp vụ — Quỹ | 7 |
| Nghiệp vụ — Ngân hàng | 5 |
| Nghiệp vụ — Mua hàng | 8 |
| Nghiệp vụ — Bán hàng | 10 |
| Nghiệp vụ — Hóa đơn điện tử | 5 |
| Nghiệp vụ — Kho | 9 |
| Nghiệp vụ — CCDC | 5 |
| Nghiệp vụ — TSCĐ | 6 |
| Nghiệp vụ — Tiền lương | 7 |
| Nghiệp vụ — Thuế | 11 |
| Nghiệp vụ — Tổng hợp | 4 |
| Sổ sách | 9 |
| Báo cáo tài chính | 4 |
| Báo cáo quản trị | 7 |
| Kết chuyển và khóa sổ | 13 |
| Danh mục | 16 |
| Hệ thống | 7 |
| **Total** | **141** |

Of those, **8 are `✓`**, roughly **50 are `~`**, and the rest are `·`.

### MISA modules FinGo's rail does not carry

The rail has 16 entries. MISA SME 2026's *Menu Nghiệp vụ* has twenty modules. The
gap is deliberate for v1, but it should be a decision rather than an omission:

| MISA module | FinGo | Why |
| :--- | :--- | :--- |
| Quản lý hóa đơn | folded into `einvoice.manage` | keep separate if e-invoice volume is high |
| Giá thành | absent | production costing — only relevant to the sản xuất/xây lắp overlays |
| Khế ước vay | absent | loan schedules; a real need, no engine |
| Hợp đồng | `md.contract`, P5 | |
| Ngân sách | absent | budgeting; not an accounting obligation |
| Thủ kho | absent | warehouse-role screen; a UI variant of `stock.*`, not a new engine |
| Thủ quỹ | absent | cashier-role screen; same |
| Phân tích tài chính | `mgmt.*` | |

Plus MISA's industry overlays — dược phẩm, thiết bị văn phòng, ô tô/xe máy, vật
liệu xây dựng, vận tải, du lịch, sản xuất, xây lắp, logistics. **None are in scope
for v1** and none should be built speculatively; they are configuration of the same
engine, not new engines.

---

## 12. What gates what

| Gate | Blocks | Status |
| :--- | :--- | :--- |
| 01/GTGT mã số chỉ tiêu (TT 89/2026, Phụ lục I) | the whole `tax.vat.*` family | **unverified** — not readable |
| B01-DN naming under TT 46/2025 vs TT 99/2025 | `fs.b01`…`fs.b09` printing | **unverified** |
| PIT bracket article number | `payroll.pit`, `tax.pit.return` | open |
| Inventory costing method at `init.policy` | `stock.costing`, `close.cost.stock` | decision, not research |
| VAT method (khấu trừ vs trực tiếp) | `tax.vat.*` entirely | decision, not research |
| Sổ quản trị vs Sổ tài chính | the ledger model itself | **architectural fork — decide before the chart and the close freeze** |

That last one is the one to decide early. MISA holds opening balances
**independently for the management books and the financial books**. FinGo has one
ledger. Adding a second after the chart, the close and five report families exist
is a rewrite; adding it before is a column.

---

## 13. Build order

Not the phase order — the order that gets a usable product soonest while never
printing a wrong number.

1. **`gl.journal.*`, `book.*`, `md.account/customer/supplier`** — the pages the
   engine already backs, plus the traceability rule. *(P1)*
2. **`init.policy` + `init.opening` (10 screens)** — because everything after is
   computed from them, and they are the cheapest thing here to get wrong forever.
3. **`cash.*`, `bank.*`, `purchase.*`, `sales.*` remainder, `stock.*`** — the daily
   entry modules. Money in, money out, goods in, goods out. *(P2)*
4. **`close.*` runs 1–8, in MISA's order, enforced by the UI** — before any P&L
   transfer is allowed to run. *(P3)*
5. **`tax.*` and `fs.*`** — gated on §12. *(P4)*
6. **`payroll.*`, `mgmt.*`, `einvoice.*`, the rest.** *(P4–P5)*

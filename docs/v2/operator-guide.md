# FinGo v2 — Hướng dẫn sử dụng cho kế toán

Tài liệu này viết cho người dùng hằng ngày: kế toán dịch vụ giữ sổ cho nhiều khách hàng.
Mỗi mục là một việc, theo đúng thứ tự thường làm.

---

## 1. Đăng nhập và chọn khách hàng

1. Mở `https://<máy chủ>:8443` bằng trình duyệt.
2. Nhập email và mật khẩu được quản trị viên cấp.
3. Hệ thống chỉ mở sổ của **một khách hàng tại một thời điểm**. Nếu tài khoản phụ trách nhiều
   khách hàng, phiên làm việc phải chỉ rõ đang mở sổ của ai — FinGo không tự chọn thay bạn.

> Sổ của khách hàng này không thể nhìn thấy sổ của khách hàng khác, kể cả khi bạn gõ đúng
> đường dẫn. Đó là ràng buộc ở tầng cơ sở dữ liệu, không phải quy ước.

## 2. Mở sổ lần đầu: số dư đầu kỳ

Làm **một lần**, trước khi ghi bất kỳ chứng từ nào.

1. Vào **Số dư đầu kỳ** (`/opening`).
2. Chọn ngày mở sổ (thường là 01/01 của năm tài chính).
3. Tải tệp Excel `.xlsx` gồm ba cột: `Tài khoản`, `Nợ`, `Có`.
   - Dòng tiêu đề được bỏ qua.
   - Có thể chỉ có một cột số tiền — hiểu là bên Nợ.
4. Bấm **Ghi sổ số dư**.

Điều kiện để được ghi sổ:

| Điều kiện | Nếu sai |
| :--- | :--- |
| Tổng Nợ = Tổng Có | Chứng từ bị từ chối, **không có gì được ghi** |
| Tài khoản phải là tài khoản chi tiết (không phải tài khoản tổng hợp) | Báo rõ tài khoản nào không ghi sổ được |
| Kỳ kế toán phải đang mở | Báo kỳ đã khoá |

Tệp đã nhập rồi thì **không nhập lại được** — mỗi tệp là một chứng từ. Nếu cần sửa, lập bút
toán điều chỉnh.

## 3. Danh mục khách hàng / nhà cung cấp

Vào **Danh mục** (`/partners`).

- Thêm tên; mã số thuế có thể để trống (khách lẻ).
- Loại: Khách hàng · Nhà cung cấp · Cả hai.
- Mã số thuế trùng sẽ bị từ chối — một mã số thuế chỉ ứng với một đối tượng.

Hóa đơn bán hàng **bắt buộc** phải chọn khách hàng: TK 131 phải biết ai nợ.

## 4. Bán hàng

Vào **Bán hàng** (`/sales`).

1. Chọn khách hàng, ngày hóa đơn, diễn giải.
2. Mỗi dòng: mã hàng/diễn giải, số lượng, đơn giá **chưa gồm VAT**.
3. Bấm **Ghi sổ hóa đơn**.

Hệ thống ghi:

```
Nợ 131    = tổng thanh toán
Có 5111   = doanh thu trước thuế
Có 33311  = thuế GTGT đầu ra
```

và tạo một **khoản phải thu** đang mở cho hóa đơn đó.

Thuế suất đang áp dụng: 10%. Giá vốn (TK 632) chưa được ghi vì module kho chưa nối — xem
mục *Giới hạn hiện tại*.

## 5. Thu tiền khách hàng

Vào **Thu tiền** (`/receipt`).

1. Bảng liệt kê hóa đơn còn nợ, kèm số còn phải thu.
2. Sửa số tiền nếu thu một phần, rồi bấm **Thu tiền**.

Hệ thống ghi `Nợ 1111 / Có 131` và cấn trừ vào đúng hóa đơn.

- Thu **thiếu**: hóa đơn chuyển sang *thu một phần*.
- Thu **thừa**: phần vượt được ghi thành **số dư có của khách hàng**, không phải lỗi.
- Thu lại cùng một hóa đơn với cùng số tiền: bị từ chối (chống ghi trùng).

## 6. Mua hàng (chi phí, dịch vụ)

Vào **Mua hàng** (`/purchase`).

1. Chọn nhà cung cấp, ngày, diễn giải.
2. Chọn **tài khoản chi phí** (ví dụ 6422 — chi phí quản lý).
3. Nhập số tiền **trước VAT**, bấm **Ghi sổ hóa đơn mua**.

Hệ thống ghi:

```
Nợ 6422   = số tiền trước thuế
Nợ 1331   = thuế GTGT đầu vào
Có 331    = tổng phải trả
```

và tạo khoản **phải trả** đang mở.

> Đây là đường dành cho chi phí và dịch vụ. Hàng mua vào kho đi theo luồng ba chiều
> (đơn mua → nhập kho → hóa đơn), chưa mở ở phiên bản này.

## 7. Trả tiền nhà cung cấp

Vào **Trả tiền** (`/payment`): chọn hóa đơn còn nợ, sửa số tiền nếu trả một phần, bấm
**Trả tiền**. Hệ thống ghi `Nợ 331 / Có 1111` và cấn trừ vào hóa đơn.

## 8. Xem sổ và báo cáo

| Màn hình | Đường dẫn | Trả lời câu hỏi |
| :--- | :--- | :--- |
| Nhật ký chung | `/journal` | Kỳ này đã ghi những chứng từ nào? |
| Bảng cân đối số phát sinh | `/reports` | Tài khoản nào có số dư, và số phát sinh bao nhiêu? |
| Sổ chi tiết tài khoản | `/ledger?account=131` | Tiền này vào từ chứng từ nào? |
| Kết quả kinh doanh | `/reports/pnl` | Kỳ này lãi hay lỗ? |
| Bảng cân đối kế toán | `/reports/balance-sheet` | Tài sản có bằng nguồn vốn? |
| Xuất Excel | `/reports/export.xlsx` | Gửi bảng cân đối cho người khác |

Mọi con số đều cộng từ **bút toán đã ghi sổ**. Không có số dư lưu sẵn để lệch với sổ.

## 9. Khoá sổ kỳ

Vào **Khoá sổ** (`/close`).

1. Kiểm tra không còn chứng từ chưa tất toán trong kỳ.
2. Bấm **Khoá sổ kỳ này**.

Hệ thống kết chuyển doanh thu và chi phí qua TK 911 vào TK 4212, ghi lại báo cáo của kỳ,
rồi **khoá kỳ — không mở lại được**.

Sau khi khoá:

- Không ghi được chứng từ mới vào kỳ đó.
- Chứng từ đã ghi sổ là bất biến; sửa sai bằng **bút toán đảo**, không sửa chứng từ cũ.

## 10. Bàn giao khách hàng

Vào **Bàn giao** (`/handover`) để tải một tệp `.zip` gồm CSV từng bảng và `MANIFEST.json`.
Đây là dữ liệu của **một khách hàng**, không phải bản sao cơ sở dữ liệu của cả văn phòng.
Giao cho kế toán kế nhiệm khi khách hàng rời đi.

## 11. Sao lưu (việc của quản trị viên)

- Hằng ngày: `pg_dump` tự động, giữ 30 ngày (xem `docs/v2/restore-drill.md`).
- Định kỳ: chạy `scripts/restore-drill.ps1` để chứng minh bản sao lưu **khôi phục được**.
  Bản sao lưu chưa từng khôi phục chỉ là một tệp.

## 12. Giới hạn hiện tại — biết trước để không mất thời gian

| Chưa có | Ảnh hưởng |
| :--- | :--- |
| Điều chuyển kho | Chưa điều chuyển hàng giữa các kho trên phần mềm |
| Mua hàng theo luồng ba chiều | Nhập kho và hóa đơn mua là hai bước riêng |
| Nhiều thuế suất (0/5/8/10) | Chỉ áp dụng 10% |
| Bút toán đảo bằng một cú bấm | Phải tự nhập bút toán đảo |
| Sao lưu theo lịch trong ứng dụng | Phải đặt lịch bằng công cụ hệ điều hành |

## 13. Nguyên tắc không đổi

1. **Sổ đã ghi là bất biến.** Sai thì đảo, không sửa.
2. **Kỳ đã khoá là khoá vĩnh viễn.**
3. **Không có đồng lẻ.** Mọi số tiền là đồng chẵn.
4. **Mỗi khách hàng một sổ riêng.** Không nhìn thấy, không ghi được sang sổ khác.
5. **Con số nào cũng phải truy được về chứng từ.** Nếu không truy được, đó là lỗi.

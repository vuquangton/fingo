namespace Accounting.Domain.Enums;

public enum AccountCategory
{
    Asset = 1,          // Loại 1, 2: Tài sản
    Liability = 2,      // Loại 3: Nợ phải trả
    Equity = 3,         // Loại 4: Vốn chủ sở hữu
    Revenue = 4,        // Loại 5: Doanh thu bán hàng & tài chính
    Expense = 5,        // Loại 6: Chi phí SXKD, giá vốn, quản lý
    OtherIncome = 6,    // Loại 7: Thu nhập khác
    OtherExpense = 7,   // Loại 8: Chi phí khác
    BusinessResult = 8  // Loại 9: Xác định kết quả kinh doanh (TK 911)
}

public enum BalanceType
{
    Debit = 1,          // Dư Nợ
    Credit = 2,         // Dư Có
    Bilateral = 3       // Lưỡng tính (TK 131, 331, 421, 333...)
}

public enum VoucherStatus
{
    Draft = 0,          // Nháp
    Pending = 1,        // Chờ duyệt
    Approved = 2,       // Đã duyệt
    Posted = 3,         // Đã ghi sổ
    Cancelled = 4       // Đã hủy
}

public enum VoucherType
{
    GeneralJournal = 1, // Phiếu kế toán chung
    CashReceipt = 2,    // Phiếu thu tiền mặt
    CashPayment = 3,    // Phiếu chi tiền mặt
    BankDeposit = 4,    // Giấy báo Có (Thu ngân hàng)
    BankPayment = 5,    // Ủy nhiệm chi (UNC / Chi ngân hàng)
    SalesInvoice = 6,   // Hóa đơn bán hàng
    PurchaseInvoice = 7,// Hóa đơn mua hàng
    WarehouseIn = 8,    // Phiếu nhập kho
    WarehouseOut = 9,   // Phiếu xuất kho
    Depreciation = 10,  // Khấu hao tài sản / Phân bổ chi phí
    PeriodClosing = 11  // Bút toán kết chuyển cuối kỳ (TK 911)
}

public enum CostingMethod
{
    FIFO = 1,                       // Nhập trước - Xuất trước
    PerpetualMovingAverage = 2,     // Bình quân gia quyền tức thời
    MonthlyPeriodicWeightedAverage = 3 // Bình quân gia quyền cuối kỳ
}

public enum ThreeWayMatchStatus
{
    Unmatched = 0,
    Matched = 1,
    QuantityMismatch = 2,
    PriceMismatch = 3,
    TotalMismatch = 4
}

public enum DepreciationMethod
{
    StraightLine = 1,       // Phương pháp đường thẳng
    DecliningBalance = 2    // Phương pháp số dư giảm dần có điều chỉnh
}

public enum VatInvoiceType
{
    Input = 1,              // Hóa đơn GTGT đầu vào (TK 133)
    Output = 2              // Hóa đơn GTGT đầu ra (TK 3331)
}

public enum VatRate
{
    Exempt = -1,    // Không chịu thuế / Miễn thuế
    Rate0 = 0,      // Thuế suất 0%
    Rate5 = 5,      // Thuế suất 5%
    Rate8 = 8,      // Thuế suất 8% (Nghị quyết giảm thuế GTGT)
    Rate10 = 10     // Thuế suất 10%
}

public enum AuditAction
{
    Create = 1,
    Update = 2,
    Delete = 3,
    Post = 4,
    Unpost = 5,
    Backup = 6,
    Restore = 7,
    Login = 8,
    Logout = 9
}

public enum PartnerType
{
    Customer = 1,
    Vendor = 2,
    Employee = 3,
    Other = 4
}

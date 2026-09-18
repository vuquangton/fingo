namespace Accounting.Domain.WarehouseOperations;

public enum WarehouseVoucherType
{
    InwardPurchase = 1,     // Phiếu nhập kho mua hàng (PNK)
    InwardProduction = 2,   // Phiếu nhập kho thành phẩm sản xuất
    InwardTransfer = 3,     // Phiếu nhập kho điều chuyển nội bộ
    OutwardSales = 4,       // Phiếu xuất kho bán hàng (PXK)
    OutwardProduction = 5,  // Phiếu xuất kho nguyên vật liệu sản xuất
    OutwardInternal = 6     // Phiếu xuất kho điều chuyển / tiêu dùng nội bộ
}

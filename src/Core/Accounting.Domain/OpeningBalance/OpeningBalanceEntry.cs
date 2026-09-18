using Accounting.Domain.Common;
using Accounting.Domain.MasterData.Common;

namespace Accounting.Domain.OpeningBalance;

public class OpeningBalanceEntry : AggregateRoot<Guid>, IAuditableEntity
{
    public int FiscalYear { get; private set; }
    public AccountId AccountId { get; private set; }
    public decimal DebitAmount { get; private set; }
    public decimal CreditAmount { get; private set; }

    // Multi-dimensional sub-ledger associations
    public PartnerId? PartnerId { get; private set; }
    public WarehouseId? WarehouseId { get; private set; }
    public InventoryItemId? InventoryItemId { get; private set; }
    public decimal Quantity { get; private set; }
    public decimal UnitPrice { get; private set; }

    public string? BatchId { get; private set; }
    public string Description { get; private set; } = string.Empty;
    public bool IsCommitted { get; private set; }
    public DateTime? CommittedAtUtc { get; private set; }

    // IAuditableEntity
    public DateTime CreatedAtUtc { get; set; } = DateTime.UtcNow;
    public string? CreatedBy { get; set; }
    public DateTime? UpdatedAtUtc { get; set; }
    public string? UpdatedBy { get; set; }
    public DateTime? LastModifiedAtUtc { get => UpdatedAtUtc; set => UpdatedAtUtc = value; }
    public string? LastModifiedBy { get => UpdatedBy; set => UpdatedBy = value; }

    private OpeningBalanceEntry() { }

    public OpeningBalanceEntry(
        Guid id,
        int fiscalYear,
        AccountId accountId,
        decimal debitAmount,
        decimal creditAmount,
        PartnerId? partnerId = null,
        WarehouseId? warehouseId = null,
        InventoryItemId? inventoryItemId = null,
        decimal quantity = 0m,
        decimal unitPrice = 0m,
        string? batchId = null,
        string? description = null)
    {
        if (fiscalYear < 2000 || fiscalYear > 2100)
            throw new ArgumentOutOfRangeException(nameof(fiscalYear), "Năm tài chính mở số dư không hợp lệ.");

        if (string.IsNullOrWhiteSpace(accountId.Value))
            throw new ArgumentException("Mã tài khoản không được để trống.", nameof(accountId));

        if (debitAmount < 0 || creditAmount < 0)
            throw new ArgumentOutOfRangeException("Số tiền Nợ hoặc Có đầu kỳ không được âm.");

        if (debitAmount == 0 && creditAmount == 0 && quantity == 0)
            throw new ArgumentException("Số dư đầu kỳ phải có ít nhất một giá trị Nợ, Có hoặc Số lượng.");

        if (quantity < 0 || unitPrice < 0)
            throw new ArgumentOutOfRangeException("Số lượng hoặc đơn giá tồn kho đầu kỳ không được âm.");

        if (quantity > 0 && !inventoryItemId.HasValue)
            throw new ArgumentException("Phải chỉ định Mã mặt hàng tồn kho khi có số lượng đầu kỳ.");

        Id = id;
        FiscalYear = fiscalYear;
        AccountId = accountId;
        DebitAmount = debitAmount;
        CreditAmount = creditAmount;
        PartnerId = partnerId;
        WarehouseId = warehouseId;
        InventoryItemId = inventoryItemId;
        Quantity = quantity;
        UnitPrice = unitPrice;
        BatchId = batchId?.Trim();
        Description = description?.Trim() ?? "Số dư ban đầu";
        IsCommitted = false;
    }

    public void MarkCommitted(string committedBy)
    {
        IsCommitted = true;
        CommittedAtUtc = DateTime.UtcNow;
        UpdatedBy = committedBy;
        UpdatedAtUtc = DateTime.UtcNow;
    }
}

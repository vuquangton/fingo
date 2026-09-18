using Accounting.Domain.Common;
using Accounting.Domain.Ledger;
using Accounting.Domain.MasterData.Accounts;
using Accounting.Domain.MasterData.Common;

namespace Accounting.Domain.WarehouseOperations;

public class WarehouseVoucher : AggregateRoot<WarehouseVoucherId>, IAuditableEntity
{
    public string VoucherNumber { get; private set; } = string.Empty;
    public WarehouseVoucherType VoucherType { get; private set; }
    public DateOnly PostingDate { get; private set; }
    public WarehouseId WarehouseId { get; private set; }
    public WarehouseId? DestinationWarehouseId { get; private set; }
    public string Description { get; private set; } = string.Empty;
    public decimal TotalQuantity { get; private set; }
    public decimal TotalAmount { get; private set; }
    public VoucherId? LinkedVoucherId { get; private set; }

    // Audit fields
    public DateTime CreatedAtUtc { get; set; } = DateTime.UtcNow;
    public string? CreatedBy { get; set; }
    public DateTime? UpdatedAtUtc { get; set; }
    public string? UpdatedBy { get; set; }
    public DateTime? LastModifiedAtUtc { get => UpdatedAtUtc; set => UpdatedAtUtc = value; }
    public string? LastModifiedBy { get => UpdatedBy; set => UpdatedBy = value; }

    private readonly List<WarehouseVoucherLine> _lines = [];
    public IReadOnlyCollection<WarehouseVoucherLine> Lines => _lines.AsReadOnly();

    private WarehouseVoucher() { }

    public WarehouseVoucher(
        WarehouseVoucherId id,
        string voucherNumber,
        WarehouseVoucherType voucherType,
        DateOnly postingDate,
        WarehouseId warehouseId,
        string description,
        WarehouseId? destinationWarehouseId = null)
    {
        if (string.IsNullOrWhiteSpace(voucherNumber))
            throw new ArgumentException("Voucher number cannot be empty.", nameof(voucherNumber));
        if (voucherType == WarehouseVoucherType.InwardTransfer && destinationWarehouseId == null)
            throw new ArgumentException("Destination warehouse is required for transfer vouchers.", nameof(destinationWarehouseId));

        Id = id;
        VoucherNumber = voucherNumber.Trim().ToUpperInvariant();
        VoucherType = voucherType;
        PostingDate = postingDate;
        WarehouseId = warehouseId;
        DestinationWarehouseId = destinationWarehouseId;
        Description = description.Trim();
    }

    public void AddLine(
        InventoryItemId inventoryItemId,
        UomId unitOfMeasureId,
        decimal quantity,
        decimal unitPrice,
        AccountId debitAccountId,
        AccountId creditAccountId,
        string description)
    {
        var line = new WarehouseVoucherLine(
            WarehouseVoucherLineId.New(),
            Id,
            inventoryItemId,
            unitOfMeasureId,
            quantity,
            unitPrice,
            debitAccountId,
            creditAccountId,
            description);

        _lines.Add(line);
        TotalQuantity = _lines.Sum(l => l.Quantity);
        TotalAmount = _lines.Sum(l => l.TotalAmount);
    }

    public void LinkGeneralLedgerVoucher(VoucherId glVoucherId)
    {
        LinkedVoucherId = glVoucherId;
        UpdatedAtUtc = DateTime.UtcNow;
    }
}

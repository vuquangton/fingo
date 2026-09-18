using Accounting.Domain.Common;
using Accounting.Domain.MasterData.Accounts;
using Accounting.Domain.MasterData.Common;

namespace Accounting.Domain.Ledger;

public class VoucherLine : Entity<VoucherLineId>
{
    public VoucherId VoucherId { get; private set; }
    public int LineNumber { get; private set; }
    public AccountId AccountId { get; private set; }
    public Account? Account { get; private set; }
    public LedgerEntryType EntryType { get; private set; }
    public decimal AmountOriginal { get; private set; }
    public decimal AmountBase { get; private set; }
    public string Description { get; private set; } = string.Empty;

    // Tracking / Ledger Dimensions
    public PartnerId? PartnerId { get; private set; }
    public WarehouseId? WarehouseId { get; private set; }
    public CostCenterId? CostCenterId { get; private set; }
    public string? ProjectId { get; private set; }

    private VoucherLine() { } // EF Core

    public VoucherLine(
        VoucherLineId id,
        VoucherId voucherId,
        int lineNumber,
        AccountId accountId,
        LedgerEntryType entryType,
        decimal amountOriginal,
        decimal amountBase,
        string description,
        PartnerId? partnerId = null,
        WarehouseId? warehouseId = null,
        CostCenterId? costCenterId = null,
        string? projectId = null)
    {
        if (amountOriginal <= 0)
            throw new ArgumentOutOfRangeException(nameof(amountOriginal), "Line original amount must be strictly positive.");

        if (amountBase <= 0)
            throw new ArgumentOutOfRangeException(nameof(amountBase), "Line base amount must be strictly positive.");

        Id = id;
        VoucherId = voucherId;
        LineNumber = lineNumber;
        AccountId = accountId;
        EntryType = entryType;
        AmountOriginal = decimal.Round(amountOriginal, 2, MidpointRounding.AwayFromZero);
        AmountBase = decimal.Round(amountBase, 2, MidpointRounding.AwayFromZero);
        Description = description.Trim();
        PartnerId = partnerId;
        WarehouseId = warehouseId;
        CostCenterId = costCenterId;
        ProjectId = projectId?.Trim();
    }

    internal void SetLineNumber(int number) => LineNumber = number;
}

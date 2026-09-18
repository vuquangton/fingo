using Accounting.Domain.Common;
using Accounting.Domain.MasterData.Accounts;
using Accounting.Domain.MasterData.Common;

namespace Accounting.Domain.Ledger;

/// <summary>
/// Flat, append-only ledger transaction record for high-speed trial balance and Sổ Cái queries.
/// Generated atomically when a Voucher is posted or reversed.
/// </summary>
public class GeneralLedgerEntry : Entity<Guid>
{
    public VoucherId VoucherId { get; private set; }
    public DateOnly PostingDate { get; private set; }
    public FiscalPeriodId FiscalPeriodId { get; private set; }
    public AccountId AccountId { get; private set; }
    public decimal DebitAmount { get; private set; }
    public decimal CreditAmount { get; private set; }

    // Multi-dimensional reporting dimensions
    public PartnerId? PartnerId { get; private set; }
    public CostCenterId? CostCenterId { get; private set; }
    public WarehouseId? WarehouseId { get; private set; }

    public DateTime CreatedAtUtc { get; private set; } = DateTime.UtcNow;

    private GeneralLedgerEntry() { } // EF Core

    public GeneralLedgerEntry(
        Guid id,
        VoucherId voucherId,
        DateOnly postingDate,
        FiscalPeriodId fiscalPeriodId,
        AccountId accountId,
        decimal debitAmount,
        decimal creditAmount,
        PartnerId? partnerId = null,
        CostCenterId? costCenterId = null,
        WarehouseId? warehouseId = null)
    {
        if (debitAmount < 0 || creditAmount < 0)
            throw new ArgumentOutOfRangeException("Amounts cannot be negative in double-entry ledger entries.");

        if (debitAmount == 0 && creditAmount == 0)
            throw new ArgumentException("Either debit amount or credit amount must be non-zero.");

        if (debitAmount > 0 && creditAmount > 0)
            throw new ArgumentException("A ledger entry cannot have both non-zero debit and credit amounts.");

        Id = id;
        VoucherId = voucherId;
        PostingDate = postingDate;
        FiscalPeriodId = fiscalPeriodId;
        AccountId = accountId;
        DebitAmount = decimal.Round(debitAmount, 2, MidpointRounding.AwayFromZero);
        CreditAmount = decimal.Round(creditAmount, 2, MidpointRounding.AwayFromZero);
        PartnerId = partnerId;
        CostCenterId = costCenterId;
        WarehouseId = warehouseId;
        CreatedAtUtc = DateTime.UtcNow;
    }
}

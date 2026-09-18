using Accounting.Application.Common.Interfaces;
using Accounting.Application.Common.Models;
using Accounting.Domain.Common;
using Accounting.Domain.Ledger;
using Microsoft.EntityFrameworkCore;

namespace Accounting.Application.Common.Services;

public class GlVoucherBridgeService(IAccountingDbContext context) : IGlVoucherBridgeService
{
    public async Task<Result<VoucherId>> PostOperationalVoucherAsync(
        Voucher voucher,
        string postedBy,
        CancellationToken cancellationToken = default)
    {
        // 1. Fiscal Period Lock Boundary Guard
        var period = await context.FiscalPeriods
            .FirstOrDefaultAsync(p => p.Year == voucher.PostingDate.Year && p.PeriodNumber == voucher.PostingDate.Month, cancellationToken);

        if (period != null && period.IsHardLocked)
        {
            throw new FiscalPeriodClosedException(period.Year, period.Month, "hard-locked (closed permanently)");
        }

        // 2. Statutory Compliance & Dimension Guardrails
        var accountIds = voucher.Lines.Select(l => l.AccountId).Distinct().ToList();
        var accounts = await context.MasterAccounts
            .Where(a => accountIds.Contains(a.Id))
            .ToDictionaryAsync(a => a.Id, cancellationToken);

        foreach (var line in voucher.Lines)
        {
            if (!accounts.TryGetValue(line.AccountId, out var account))
                throw new StatutoryComplianceException($"Account '{line.AccountId.Value}' does not exist in Chart of Accounts.");

            if (!account.CanPostOn(voucher.PostingDate))
            {
                var reason = account.IsParent
                    ? "parent account cannot accept direct postings"
                    : !account.IsActive
                        ? "account is inactive"
                        : $"voucher posting date {voucher.PostingDate:yyyy-MM-dd} is outside statutory validity [{account.EffectiveFrom:yyyy-MM-dd}, {account.EffectiveTo:yyyy-MM-dd}]";

                throw new StatutoryComplianceException($"Account '{account.Id.Value}' ('{account.AccountName}') cannot accept postings: {reason}.");
            }

            // Dimension requirements
            if (account.RequiresPartner && !line.PartnerId.HasValue)
                throw new StatutoryComplianceException($"Account '{account.Id.Value}' requires a valid Business Partner dimension.");

            if (account.RequiresWarehouse && !line.WarehouseId.HasValue)
                throw new StatutoryComplianceException($"Account '{account.Id.Value}' requires a valid Warehouse dimension.");

            if (account.RequiresCostCenter && !line.CostCenterId.HasValue)
                throw new StatutoryComplianceException($"Account '{account.Id.Value}' requires a valid Cost Center dimension.");

            if (account.RequiresProject && string.IsNullOrWhiteSpace(line.ProjectId))
                throw new StatutoryComplianceException($"Account '{account.Id.Value}' requires a Project dimension.");
        }

        // 3. Post Voucher (enforces double-entry balance: DebitBase == CreditBase and DebitOriginal == CreditOriginal)
        voucher.Post(postedBy);

        // 4. Register voucher in GlVouchers
        context.AddEntity(voucher);

        // 5. Generate Flat Append-Only General Ledger Entries
        var fiscalPeriodId = FiscalPeriodId.FromYearMonth(voucher.PostingDate.Year, voucher.PostingDate.Month);
        var glEntries = voucher.Lines.Select(line => new GeneralLedgerEntry(
            Guid.NewGuid(),
            voucher.Id,
            voucher.PostingDate,
            fiscalPeriodId,
            line.AccountId,
            line.EntryType == LedgerEntryType.Debit ? line.AmountBase : 0m,
            line.EntryType == LedgerEntryType.Credit ? line.AmountBase : 0m,
            line.PartnerId,
            line.CostCenterId,
            line.WarehouseId)).ToList();

        context.AddRangeEntities(glEntries);
        await context.SaveChangesAsync(cancellationToken);

        return Result<VoucherId>.Success(voucher.Id);
    }
}

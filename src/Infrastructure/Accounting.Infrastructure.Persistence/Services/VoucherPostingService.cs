using Accounting.Application.Common.Interfaces;
using Accounting.Domain.Entities.GeneralLedger;
using Accounting.Domain.Enums;
using Accounting.Domain.Exceptions;
using Microsoft.EntityFrameworkCore;

namespace Accounting.Infrastructure.Persistence.Services;

public class VoucherPostingService : IVoucherPostingService
{
    private readonly IAccountingDbContext _context;
    private readonly IAuditLogService _auditLog;

    public VoucherPostingService(IAccountingDbContext context, IAuditLogService auditLog)
    {
        _context = context;
        _auditLog = auditLog;
    }

    public async Task<Voucher> PostVoucherAsync(Guid voucherId, Guid userId, CancellationToken cancellationToken = default)
    {
        var voucher = await _context.Vouchers
            .Include(v => v.Lines)
            .FirstOrDefaultAsync(v => v.Id == voucherId, cancellationToken);

        if (voucher == null)
            throw new KeyNotFoundException($"Voucher with ID '{voucherId}' not found.");

        // Check if fiscal period is locked
        var period = await _context.FiscalPeriods
            .FirstOrDefaultAsync(p => p.Year == voucher.PostingDate.Year && p.Month == voucher.PostingDate.Month, cancellationToken);

        if (period != null && (period.IsHardLocked || period.IsSoftLocked))
        {
            throw new FiscalPeriodClosedException(period.Year, period.Month, period.IsHardLocked ? "hard-locked (closed permanently)" : "soft-locked");
        }

        voucher.Post(userId);
        await _context.SaveChangesAsync(cancellationToken);

        await _auditLog.LogAsync(
            AuditAction.Post,
            nameof(Voucher),
            voucher.Id.ToString(),
            diffSummary: $"Posted voucher {voucher.VoucherNumber} dated {voucher.PostingDate:dd/MM/yyyy}",
            cancellationToken: cancellationToken);

        return voucher;
    }

    public async Task<Voucher> UnpostVoucherAsync(Guid voucherId, Guid userId, CancellationToken cancellationToken = default)
    {
        var voucher = await _context.Vouchers
            .Include(v => v.Lines)
            .FirstOrDefaultAsync(v => v.Id == voucherId, cancellationToken);

        if (voucher == null)
            throw new KeyNotFoundException($"Voucher with ID '{voucherId}' not found.");

        var period = await _context.FiscalPeriods
            .FirstOrDefaultAsync(p => p.Year == voucher.PostingDate.Year && p.Month == voucher.PostingDate.Month, cancellationToken);

        if (period != null && period.IsHardLocked)
        {
            throw new FiscalPeriodClosedException(period.Year, period.Month, "hard-locked. Unposting is prohibited.");
        }

        voucher.Unpost(userId);
        await _context.SaveChangesAsync(cancellationToken);

        await _auditLog.LogAsync(
            AuditAction.Unpost,
            nameof(Voucher),
            voucher.Id.ToString(),
            diffSummary: $"Unposted voucher {voucher.VoucherNumber}",
            cancellationToken: cancellationToken);

        return voucher;
    }

    public async Task<Voucher> ReverseVoucherAsync(
        Guid voucherId,
        string newVoucherNumber,
        Guid userId,
        DateTime reversalDate,
        string reason,
        CancellationToken cancellationToken = default)
    {
        var voucher = await _context.Vouchers
            .Include(v => v.Lines)
            .FirstOrDefaultAsync(v => v.Id == voucherId, cancellationToken);

        if (voucher == null)
            throw new KeyNotFoundException($"Voucher with ID '{voucherId}' not found.");

        var reversal = voucher.CreateReversal(newVoucherNumber, userId, reversalDate, reason);
        reversal.ValidateBalance();

        _context.AddEntity(reversal);
        await _context.SaveChangesAsync(cancellationToken);

        await _auditLog.LogAsync(
            AuditAction.Create,
            nameof(Voucher),
            reversal.Id.ToString(),
            diffSummary: $"Created reversing voucher {reversal.VoucherNumber} for original voucher {voucher.VoucherNumber}",
            cancellationToken: cancellationToken);

        return reversal;
    }
}

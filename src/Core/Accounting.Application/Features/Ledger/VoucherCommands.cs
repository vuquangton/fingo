using Accounting.Application.Common.Interfaces;
using Accounting.Application.Common.Models;
using Accounting.Domain.Common;
using Accounting.Domain.Ledger;
using Accounting.Domain.MasterData.Accounts;
using Accounting.Domain.MasterData.Common;
using MediatR;
using Microsoft.EntityFrameworkCore;
using CurrencyCode = Accounting.Domain.MasterData.Common.CurrencyCode;

namespace Accounting.Application.Features.Ledger;

public record VoucherLineDto(
    Guid Id,
    int LineNumber,
    string AccountCode,
    LedgerEntryType EntryType,
    decimal AmountOriginal,
    decimal AmountBase,
    string Description,
    Guid? PartnerId = null,
    string? WarehouseId = null,
    string? CostCenterId = null,
    string? ProjectId = null);

public record VoucherDto(
    Guid Id,
    string VoucherNumber,
    VoucherType VoucherType,
    DateOnly VoucherDate,
    DateOnly PostingDate,
    VoucherStatus Status,
    string Description,
    string Currency,
    decimal ExchangeRate,
    decimal TotalDebitBase,
    decimal TotalCreditBase,
    decimal TotalDebitOriginal,
    decimal TotalCreditOriginal,
    DateTime? PostedAtUtc,
    string? PostedBy,
    Guid? ReversalOfVoucherId,
    IReadOnlyList<VoucherLineDto> Lines);

public record CreateVoucherLineDto(
    string AccountCode,
    LedgerEntryType EntryType,
    decimal AmountOriginal,
    string Description,
    Guid? PartnerId = null,
    string? WarehouseId = null,
    string? CostCenterId = null,
    string? ProjectId = null);

public record CreateVoucherCommand(
    string VoucherNumber,
    VoucherType VoucherType,
    DateOnly VoucherDate,
    DateOnly PostingDate,
    string Description,
    string Currency = "VND",
    decimal ExchangeRate = 1.0m,
    IReadOnlyList<CreateVoucherLineDto>? Lines = null) : IRequest<Result<VoucherId>>;

public class CreateVoucherCommandHandler(IAccountingDbContext context) : IRequestHandler<CreateVoucherCommand, Result<VoucherId>>
{
    public async Task<Result<VoucherId>> Handle(CreateVoucherCommand request, CancellationToken cancellationToken)
    {
        var number = request.VoucherNumber.Trim().ToUpperInvariant();
        var exists = await context.GlVouchers.AnyAsync(v => v.VoucherNumber == number, cancellationToken);
        if (exists)
            return Result<VoucherId>.Failure($"Voucher with number '{number}' already exists.");

        var voucherId = VoucherId.New();
        var currencyCode = new CurrencyCode(request.Currency);

        var voucher = new Voucher(
            voucherId,
            number,
            request.VoucherType,
            request.VoucherDate,
            request.PostingDate,
            request.Description,
            currencyCode,
            request.ExchangeRate);

        if (request.Lines != null)
        {
            foreach (var line in request.Lines)
            {
                voucher.AddLine(
                    new AccountId(line.AccountCode),
                    line.EntryType,
                    line.AmountOriginal,
                    line.Description,
                    line.PartnerId.HasValue ? new PartnerId(line.PartnerId.Value) : null,
                    !string.IsNullOrWhiteSpace(line.WarehouseId) ? new WarehouseId(line.WarehouseId) : null,
                    !string.IsNullOrWhiteSpace(line.CostCenterId) ? new CostCenterId(line.CostCenterId) : null,
                    line.ProjectId);
            }
        }

        context.AddEntity(voucher);
        await context.SaveChangesAsync(cancellationToken);

        return Result<VoucherId>.Success(voucher.Id);
    }
}

public record PostVoucherCommand(VoucherId VoucherId, string PostedBy) : IRequest<Result<Unit>>;

public class PostVoucherCommandHandler(IAccountingDbContext context) : IRequestHandler<PostVoucherCommand, Result<Unit>>
{
    public async Task<Result<Unit>> Handle(PostVoucherCommand request, CancellationToken cancellationToken)
    {
        var voucher = await context.GlVouchers
            .Include(v => v.Lines)
            .FirstOrDefaultAsync(v => v.Id == request.VoucherId, cancellationToken);

        if (voucher == null)
            return Result<Unit>.Failure($"Voucher '{request.VoucherId.Value}' not found.");

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

            // CanPostOn: verifies !IsParent, IsActive, and voucher date in [EffectiveFrom, EffectiveTo]
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
        voucher.Post(request.PostedBy);

        // 4. Generate Flat Append-Only General Ledger Entries
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

        return Result<Unit>.Success(Unit.Value);
    }
}

public record ReverseVoucherCommand(
    VoucherId VoucherId,
    string NewVoucherNumber,
    string ReversedBy,
    DateOnly ReversalDate,
    string Reason) : IRequest<Result<VoucherId>>;

public class ReverseVoucherCommandHandler(IAccountingDbContext context) : IRequestHandler<ReverseVoucherCommand, Result<VoucherId>>
{
    public async Task<Result<VoucherId>> Handle(ReverseVoucherCommand request, CancellationToken cancellationToken)
    {
        var originalVoucher = await context.GlVouchers
            .Include(v => v.Lines)
            .FirstOrDefaultAsync(v => v.Id == request.VoucherId, cancellationToken);

        if (originalVoucher == null)
            return Result<VoucherId>.Failure($"Original voucher '{request.VoucherId.Value}' not found.");

        // 1. Period Lock Check for Reversal Date
        var period = await context.FiscalPeriods
            .FirstOrDefaultAsync(p => p.Year == request.ReversalDate.Year && p.PeriodNumber == request.ReversalDate.Month, cancellationToken);

        if (period != null && period.IsHardLocked)
        {
            throw new FiscalPeriodClosedException(period.Year, period.Month, "hard-locked. Reversals are prohibited in closed periods.");
        }

        // 2. Account Validity on Reversal Date
        var accountIds = originalVoucher.Lines.Select(l => l.AccountId).Distinct().ToList();
        var accounts = await context.MasterAccounts
            .Where(a => accountIds.Contains(a.Id))
            .ToDictionaryAsync(a => a.Id, cancellationToken);

        foreach (var line in originalVoucher.Lines)
        {
            if (!accounts.TryGetValue(line.AccountId, out var account) || !account.CanPostOn(request.ReversalDate))
            {
                throw new StatutoryComplianceException($"Account '{line.AccountId.Value}' cannot accept posting on reversal date {request.ReversalDate:yyyy-MM-dd}.");
            }
        }

        // 3. Create Mirror-Image Reversing Voucher
        var reversalVoucher = originalVoucher.CreateReversal(
            request.NewVoucherNumber,
            request.ReversedBy,
            request.ReversalDate,
            request.Reason);

        // 4. Post Reversing Voucher
        reversalVoucher.Post(request.ReversedBy);

        // 5. Append Counter General Ledger Entries
        var fiscalPeriodId = FiscalPeriodId.FromYearMonth(request.ReversalDate.Year, request.ReversalDate.Month);
        var glEntries = reversalVoucher.Lines.Select(line => new GeneralLedgerEntry(
            Guid.NewGuid(),
            reversalVoucher.Id,
            reversalVoucher.PostingDate,
            fiscalPeriodId,
            line.AccountId,
            line.EntryType == LedgerEntryType.Debit ? line.AmountBase : 0m,
            line.EntryType == LedgerEntryType.Credit ? line.AmountBase : 0m,
            line.PartnerId,
            line.CostCenterId,
            line.WarehouseId)).ToList();

        context.AddEntity(reversalVoucher);
        context.AddRangeEntities(glEntries);
        await context.SaveChangesAsync(cancellationToken);

        return Result<VoucherId>.Success(reversalVoucher.Id);
    }
}

public record GetVoucherByIdQuery(VoucherId VoucherId) : IRequest<Result<VoucherDto>>;

public class GetVoucherByIdQueryHandler(IAccountingDbContext context) : IRequestHandler<GetVoucherByIdQuery, Result<VoucherDto>>
{
    public async Task<Result<VoucherDto>> Handle(GetVoucherByIdQuery request, CancellationToken cancellationToken)
    {
        var voucher = await context.GlVouchers
            .AsNoTracking()
            .Include(v => v.Lines)
            .FirstOrDefaultAsync(v => v.Id == request.VoucherId, cancellationToken);

        if (voucher == null)
            return Result<VoucherDto>.Failure($"Voucher '{request.VoucherId.Value}' not found.");

        var linesDto = voucher.Lines
            .OrderBy(l => l.LineNumber)
            .Select(l => new VoucherLineDto(
                l.Id.Value,
                l.LineNumber,
                l.AccountId.Value,
                l.EntryType,
                l.AmountOriginal,
                l.AmountBase,
                l.Description,
                l.PartnerId.HasValue ? l.PartnerId.Value.Value : null,
                l.WarehouseId.HasValue ? l.WarehouseId.Value.Value : null,
                l.CostCenterId.HasValue ? l.CostCenterId.Value.Value : null,
                l.ProjectId))
            .ToList();

        var dto = new VoucherDto(
            voucher.Id.Value,
            voucher.VoucherNumber,
            voucher.VoucherType,
            voucher.VoucherDate,
            voucher.PostingDate,
            voucher.Status,
            voucher.Description,
            voucher.CurrencyId.Value,
            voucher.ExchangeRate,
            voucher.TotalDebitBase,
            voucher.TotalCreditBase,
            voucher.TotalDebitOriginal,
            voucher.TotalCreditOriginal,
            voucher.PostedAtUtc,
            voucher.PostedBy,
            voucher.ReversalOfVoucherId.HasValue ? voucher.ReversalOfVoucherId.Value.Value : null,
            linesDto);

        return Result<VoucherDto>.Success(dto);
    }
}

using Accounting.Application.Common.Interfaces;
using Accounting.Application.Common.Models;
using Accounting.Domain.Entities.GeneralLedger;
using Accounting.Domain.Enums;
using MediatR;
using Microsoft.EntityFrameworkCore;

namespace Accounting.Application.Features.GeneralLedger;

public record VoucherSummaryDto(
    Guid Id,
    string VoucherNumber,
    DateTime VoucherDate,
    DateTime PostingDate,
    VoucherType VoucherType,
    VoucherStatus Status,
    string Description,
    string? ReferenceNumber,
    decimal TotalAmount,
    string Currency,
    bool IsReversal);

public record GetVouchersQuery(
    DateTime? FromDate = null,
    DateTime? ToDate = null,
    VoucherStatus? Status = null,
    VoucherType? VoucherType = null) : IRequest<Result<List<VoucherSummaryDto>>>;

public class GetVouchersQueryHandler : IRequestHandler<GetVouchersQuery, Result<List<VoucherSummaryDto>>>
{
    private readonly IAccountingDbContext _context;

    public GetVouchersQueryHandler(IAccountingDbContext context)
    {
        _context = context;
    }

    public async Task<Result<List<VoucherSummaryDto>>> Handle(GetVouchersQuery request, CancellationToken cancellationToken)
    {
        var query = _context.Vouchers.AsNoTracking().AsQueryable();

        if (request.FromDate.HasValue) query = query.Where(v => v.PostingDate >= request.FromDate.Value);
        if (request.ToDate.HasValue) query = query.Where(v => v.PostingDate <= request.ToDate.Value);
        if (request.Status.HasValue) query = query.Where(v => v.Status == request.Status.Value);
        if (request.VoucherType.HasValue) query = query.Where(v => v.VoucherType == request.VoucherType.Value);

        var list = await query
            .OrderByDescending(v => v.PostingDate)
            .ThenByDescending(v => v.VoucherNumber)
            .Select(v => new VoucherSummaryDto(
                v.Id,
                v.VoucherNumber,
                v.VoucherDate,
                v.PostingDate,
                v.VoucherType,
                v.Status,
                v.Description,
                v.ReferenceNumber,
                v.TotalDebitAmount,
                v.Currency,
                v.IsReversal))
            .ToListAsync(cancellationToken);

        return Result<List<VoucherSummaryDto>>.Success(list);
    }
}

public record AccountDto(
    Guid Id,
    string AccountNumber,
    string Name,
    string? EnglishName,
    Guid? ParentAccountId,
    AccountCategory Category,
    BalanceType BalanceType,
    bool IsActive,
    bool IsDetailAccount);

public record GetChartOfAccountsQuery : IRequest<Result<List<AccountDto>>>;

public class GetChartOfAccountsQueryHandler : IRequestHandler<GetChartOfAccountsQuery, Result<List<AccountDto>>>
{
    private readonly IAccountingDbContext _context;

    public GetChartOfAccountsQueryHandler(IAccountingDbContext context)
    {
        _context = context;
    }

    public async Task<Result<List<AccountDto>>> Handle(GetChartOfAccountsQuery request, CancellationToken cancellationToken)
    {
        var list = await _context.Accounts.AsNoTracking()
            .OrderBy(a => a.AccountNumber)
            .Select(a => new AccountDto(
                a.Id,
                a.AccountNumber,
                a.Name,
                a.EnglishName,
                a.ParentAccountId,
                a.Category,
                a.BalanceType,
                a.IsActive,
                a.IsDetailAccount))
            .ToListAsync(cancellationToken);

        return Result<List<AccountDto>>.Success(list);
    }
}

public record TrialBalanceItemDto(
    string AccountNumber,
    string AccountName,
    decimal OpeningDebit,
    decimal OpeningCredit,
    decimal IncurredDebit,
    decimal IncurredCredit,
    decimal ClosingDebit,
    decimal ClosingCredit);

public record GetTrialBalanceQuery(DateTime FromDate, DateTime ToDate) : IRequest<Result<List<TrialBalanceItemDto>>>;

public class GetTrialBalanceQueryHandler : IRequestHandler<GetTrialBalanceQuery, Result<List<TrialBalanceItemDto>>>
{
    private readonly IAccountingDbContext _context;

    public GetTrialBalanceQueryHandler(IAccountingDbContext context)
    {
        _context = context;
    }

    public async Task<Result<List<TrialBalanceItemDto>>> Handle(GetTrialBalanceQuery request, CancellationToken cancellationToken)
    {
        var accounts = await _context.Accounts.AsNoTracking().OrderBy(a => a.AccountNumber).ToListAsync(cancellationToken);
        
        // Fetch posted voucher lines
        var postedLines = await _context.VoucherLines
            .Include(l => l.DebitAccount)
            .Include(l => l.CreditAccount)
            .Where(l => _context.Vouchers.Any(v => v.Id == l.VoucherId && v.Status == VoucherStatus.Posted && v.PostingDate >= request.FromDate && v.PostingDate <= request.ToDate))
            .ToListAsync(cancellationToken);

        var result = new List<TrialBalanceItemDto>();

        foreach (var acc in accounts)
        {
            var incurredDebit = postedLines.Where(l => l.DebitAccountId == acc.Id).Sum(l => l.Amount);
            var incurredCredit = postedLines.Where(l => l.CreditAccountId == acc.Id).Sum(l => l.Amount);

            decimal closingDebit = 0m;
            decimal closingCredit = 0m;

            if (acc.BalanceType == BalanceType.Debit)
            {
                var net = incurredDebit - incurredCredit;
                if (net >= 0) closingDebit = net; else closingCredit = -net;
            }
            else if (acc.BalanceType == BalanceType.Credit)
            {
                var net = incurredCredit - incurredDebit;
                if (net >= 0) closingCredit = net; else closingDebit = -net;
            }
            else // Bilateral (Lưỡng tính)
            {
                if (incurredDebit >= incurredCredit) closingDebit = incurredDebit - incurredCredit;
                else closingCredit = incurredCredit - incurredDebit;
            }

            result.Add(new TrialBalanceItemDto(
                acc.AccountNumber,
                acc.Name,
                0m,
                0m,
                incurredDebit,
                incurredCredit,
                closingDebit,
                closingCredit));
        }

        return Result<List<TrialBalanceItemDto>>.Success(result);
    }
}

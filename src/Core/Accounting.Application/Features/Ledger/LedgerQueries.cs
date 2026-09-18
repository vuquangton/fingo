using System.Data;
using Accounting.Application.Common.Interfaces;
using Accounting.Application.Common.Models;
using Dapper;
using MediatR;

namespace Accounting.Application.Features.Ledger;

public record TrialBalanceRowDto(
    string AccountCode,
    string AccountName,
    int AccountType,
    decimal OpeningDebit,
    decimal OpeningCredit,
    decimal PeriodDebit,
    decimal PeriodCredit,
    decimal ClosingDebit,
    decimal ClosingCredit);

public record GetTrialBalanceQuery(DateOnly FromDate, DateOnly ToDate) : IRequest<Result<IReadOnlyList<TrialBalanceRowDto>>>;

public class GetTrialBalanceQueryHandler(ISqlConnectionFactory connectionFactory) : IRequestHandler<GetTrialBalanceQuery, Result<IReadOnlyList<TrialBalanceRowDto>>>
{
    private class RawTrialBalanceRow
    {
        public string AccountCode { get; set; } = string.Empty;
        public string AccountName { get; set; } = string.Empty;
        public int AccountType { get; set; }
        public object? RawOpeningDebit { get; set; }
        public object? RawOpeningCredit { get; set; }
        public object? PeriodDebit { get; set; }
        public object? PeriodCredit { get; set; }

        public decimal OpeningDebitVal => Convert.ToDecimal(RawOpeningDebit ?? 0m);
        public decimal OpeningCreditVal => Convert.ToDecimal(RawOpeningCredit ?? 0m);
        public decimal PeriodDebitVal => Convert.ToDecimal(PeriodDebit ?? 0m);
        public decimal PeriodCreditVal => Convert.ToDecimal(PeriodCredit ?? 0m);
    }

    public async Task<Result<IReadOnlyList<TrialBalanceRowDto>>> Handle(GetTrialBalanceQuery request, CancellationToken cancellationToken)
    {
        using var conn = connectionFactory.CreateConnection();
        if (conn.State != ConnectionState.Open)
            conn.Open();

        const string sql = @"
            SELECT 
                a.Id AS AccountCode,
                a.AccountName AS AccountName,
                a.AccountType AS AccountType,
                COALESCE(SUM(CASE WHEN e.PostingDate < @FromDate THEN e.DebitAmount ELSE 0 END), 0) AS RawOpeningDebit,
                COALESCE(SUM(CASE WHEN e.PostingDate < @FromDate THEN e.CreditAmount ELSE 0 END), 0) AS RawOpeningCredit,
                COALESCE(SUM(CASE WHEN e.PostingDate >= @FromDate AND e.PostingDate <= @ToDate THEN e.DebitAmount ELSE 0 END), 0) AS PeriodDebit,
                COALESCE(SUM(CASE WHEN e.PostingDate >= @FromDate AND e.PostingDate <= @ToDate THEN e.CreditAmount ELSE 0 END), 0) AS PeriodCredit
            FROM md_accounts a
            LEFT JOIN gl_entries e ON a.Id = e.AccountId
            WHERE a.IsActive = 1
            GROUP BY a.Id, a.AccountName, a.AccountType
            ORDER BY a.Id;";

        var param = new
        {
            FromDate = request.FromDate.ToString("yyyy-MM-dd"),
            ToDate = request.ToDate.ToString("yyyy-MM-dd")
        };

        var rawRows = (await conn.QueryAsync<RawTrialBalanceRow>(sql, param)).ToList();

        var result = new List<TrialBalanceRowDto>(rawRows.Count);
        foreach (var r in rawRows)
        {
            var rawOpeningDebit = r.OpeningDebitVal;
            var rawOpeningCredit = r.OpeningCreditVal;
            var periodDebit = r.PeriodDebitVal;
            var periodCredit = r.PeriodCreditVal;

            var netOpening = rawOpeningDebit - rawOpeningCredit;
            var openingDebit = netOpening > 0 ? netOpening : 0m;
            var openingCredit = netOpening < 0 ? Math.Abs(netOpening) : 0m;

            var netClosing = (rawOpeningDebit + periodDebit) - (rawOpeningCredit + periodCredit);
            var closingDebit = netClosing > 0 ? netClosing : 0m;
            var closingCredit = netClosing < 0 ? Math.Abs(netClosing) : 0m;

            result.Add(new TrialBalanceRowDto(
                r.AccountCode,
                r.AccountName,
                r.AccountType,
                openingDebit,
                openingCredit,
                periodDebit,
                periodCredit,
                closingDebit,
                closingCredit));
        }

        return Result<IReadOnlyList<TrialBalanceRowDto>>.Success(result);
    }
}

public record AccountLedgerDetailRowDto(
    Guid Id,
    Guid VoucherId,
    string VoucherNumber,
    string PostingDate,
    string Description,
    decimal DebitAmount,
    decimal CreditAmount,
    decimal RunningBalance);

public record GetAccountLedgerDetailQuery(string AccountCode, DateOnly FromDate, DateOnly ToDate)
    : IRequest<Result<IReadOnlyList<AccountLedgerDetailRowDto>>>;

public class GetAccountLedgerDetailQueryHandler(ISqlConnectionFactory connectionFactory)
    : IRequestHandler<GetAccountLedgerDetailQuery, Result<IReadOnlyList<AccountLedgerDetailRowDto>>>
{
    private class RawLedgerDetailRow
    {
        public object? Id { get; set; }
        public object? VoucherId { get; set; }
        public string VoucherNumber { get; set; } = string.Empty;
        public object? PostingDate { get; set; }
        public string Description { get; set; } = string.Empty;
        public object? DebitAmount { get; set; }
        public object? CreditAmount { get; set; }

        public Guid IdVal => Id switch
        {
            Guid g => g,
            string s when Guid.TryParse(s, out var parsed) => parsed,
            byte[] b when b.Length == 16 => new Guid(b),
            _ => Guid.Empty
        };

        public Guid VoucherIdVal => VoucherId switch
        {
            Guid g => g,
            string s when Guid.TryParse(s, out var parsed) => parsed,
            byte[] b when b.Length == 16 => new Guid(b),
            _ => Guid.Empty
        };

        public decimal DebitVal => Convert.ToDecimal(DebitAmount ?? 0m);
        public decimal CreditVal => Convert.ToDecimal(CreditAmount ?? 0m);
    }

    public async Task<Result<IReadOnlyList<AccountLedgerDetailRowDto>>> Handle(GetAccountLedgerDetailQuery request, CancellationToken cancellationToken)
    {
        using var conn = connectionFactory.CreateConnection();
        if (conn.State != ConnectionState.Open)
            conn.Open();

        const string sql = @"
            SELECT 
                e.Id,
                e.VoucherId,
                v.VoucherNumber,
                e.PostingDate,
                v.Description,
                e.DebitAmount,
                e.CreditAmount
            FROM gl_entries e
            INNER JOIN gl_vouchers v ON e.VoucherId = v.Id
            WHERE e.AccountId = @AccountCode 
              AND e.PostingDate >= @FromDate 
              AND e.PostingDate <= @ToDate
            ORDER BY e.PostingDate ASC, e.CreatedAtUtc ASC;";

        var param = new
        {
            AccountCode = request.AccountCode.Trim(),
            FromDate = request.FromDate.ToString("yyyy-MM-dd"),
            ToDate = request.ToDate.ToString("yyyy-MM-dd")
        };

        var rawRows = (await conn.QueryAsync<RawLedgerDetailRow>(sql, param)).ToList();

        var result = new List<AccountLedgerDetailRowDto>(rawRows.Count);
        decimal runningBalance = 0m;

        foreach (var r in rawRows)
        {
            var debit = r.DebitVal;
            var credit = r.CreditVal;
            runningBalance += (debit - credit);
            var dateStr = r.PostingDate switch
            {
                DateOnly d => d.ToString("yyyy-MM-dd"),
                DateTime dt => dt.ToString("yyyy-MM-dd"),
                _ => r.PostingDate?.ToString() ?? string.Empty
            };

            result.Add(new AccountLedgerDetailRowDto(
                r.IdVal,
                r.VoucherIdVal,
                r.VoucherNumber,
                dateStr,
                r.Description,
                debit,
                credit,
                runningBalance));
        }

        return Result<IReadOnlyList<AccountLedgerDetailRowDto>>.Success(result);
    }
}

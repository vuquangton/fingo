using System.Data;
using Accounting.Application.Common.Interfaces;
using Accounting.Application.Common.Models;
using Dapper;
using MediatR;

namespace Accounting.Application.Features.Reporting;

public record DashboardSummaryDto(
    decimal CashBalance,
    decimal ReceivablesTotal,
    decimal PayablesTotal,
    decimal InventoryValue,
    decimal NetRevenueMtd,
    int TotalVouchersCount,
    int OpenInvoicesCount);

public record GetDashboardSummaryQuery(DateOnly AsOfDate) : IRequest<Result<DashboardSummaryDto>>;

public class GetDashboardSummaryQueryHandler(ISqlConnectionFactory connectionFactory) 
    : IRequestHandler<GetDashboardSummaryQuery, Result<DashboardSummaryDto>>
{
    private class BalanceSummaryRow
    {
        public string AccountId { get; set; } = string.Empty;
        public decimal TotalDebit { get; set; }
        public decimal TotalCredit { get; set; }
        public decimal MonthCredit { get; set; }
    }

    public async Task<Result<DashboardSummaryDto>> Handle(GetDashboardSummaryQuery request, CancellationToken cancellationToken)
    {
        using var conn = connectionFactory.CreateConnection();
        if (conn.State != ConnectionState.Open)
            conn.Open();

        var firstDayOfMonth = new DateOnly(request.AsOfDate.Year, request.AsOfDate.Month, 1);

        const string sql = @"
            SELECT 
                AccountId,
                COALESCE(SUM(CASE WHEN PostingDate <= @AsOfDate THEN DebitAmount ELSE 0 END), 0) AS TotalDebit,
                COALESCE(SUM(CASE WHEN PostingDate <= @AsOfDate THEN CreditAmount ELSE 0 END), 0) AS TotalCredit,
                COALESCE(SUM(CASE WHEN PostingDate >= @FirstDayOfMonth AND PostingDate <= @AsOfDate THEN CreditAmount ELSE 0 END), 0) AS MonthCredit
            FROM gl_entries
            WHERE PostingDate <= @AsOfDate
            GROUP BY AccountId;";

        var rows = (await conn.QueryAsync<BalanceSummaryRow>(sql, new 
        { 
            AsOfDate = request.AsOfDate.ToString("yyyy-MM-dd"),
            FirstDayOfMonth = firstDayOfMonth.ToString("yyyy-MM-dd")
        })).ToList();

        decimal cash = 0m;
        decimal receivables = 0m;
        decimal payables = 0m;
        decimal inventory = 0m;
        decimal revenueMtd = 0m;

        foreach (var r in rows)
        {
            var acc = r.AccountId;
            var netDebit = r.TotalDebit - r.TotalCredit;
            var netCredit = r.TotalCredit - r.TotalDebit;

            // 111, 112: Cash & Cash Equivalents
            if (acc.StartsWith("111") || acc.StartsWith("112"))
            {
                cash += netDebit;
            }
            // 131: Receivables
            else if (acc.StartsWith("131"))
            {
                receivables += netDebit > 0 ? netDebit : 0m;
            }
            // 331: Payables
            else if (acc.StartsWith("331"))
            {
                payables += netCredit > 0 ? netCredit : 0m;
            }
            // 15x: Inventory
            else if (acc.StartsWith("15"))
            {
                inventory += netDebit > 0 ? netDebit : 0m;
            }
            // 511: Revenue MTD
            if (acc.StartsWith("511"))
            {
                revenueMtd += r.MonthCredit;
            }
        }

        // Count posted vouchers
        const string countSql = "SELECT COUNT(*) FROM gl_vouchers;";
        int voucherCount = 0;
        try
        {
            voucherCount = await conn.ExecuteScalarAsync<int>(countSql);
        }
        catch { }

        return Result<DashboardSummaryDto>.Success(new DashboardSummaryDto(
            cash,
            receivables,
            payables,
            inventory,
            revenueMtd,
            voucherCount,
            0));
    }
}

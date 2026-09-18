using System.Data;
using System.Text.RegularExpressions;
using Accounting.Application.Common.Interfaces;
using Accounting.Application.Common.Models;
using Accounting.Domain.Reporting;
using Dapper;
using MediatR;
using Microsoft.EntityFrameworkCore;
using StatementType = Accounting.Domain.Reporting.StatementType;

namespace Accounting.Application.Features.Reporting;

public record FinancialStatementLineDto(
    string LineCode,
    int LineNumber,
    string ItemName,
    int PrintStyle,
    int NodeType,
    decimal Amount);

public record FinancialStatementDto(
    string TemplateCode,
    string TemplateName,
    string PeriodTitle,
    IReadOnlyList<FinancialStatementLineDto> Lines,
    bool IsBalanced,
    decimal TotalAssets,
    decimal TotalLiabilitiesAndEquity);

public record GetFinancialStatementQuery(
    string TemplateCode,
    DateOnly FromDate,
    DateOnly ToDate) : IRequest<Result<FinancialStatementDto>>;

public class GetFinancialStatementQueryHandler(
    IAccountingDbContext dbContext,
    ISqlConnectionFactory connectionFactory) : IRequestHandler<GetFinancialStatementQuery, Result<FinancialStatementDto>>
{
    private class AccountBalanceRow
    {
        public string AccountId { get; set; } = string.Empty;
        public decimal TotalDebit { get; set; }
        public decimal TotalCredit { get; set; }
    }

    public async Task<Result<FinancialStatementDto>> Handle(GetFinancialStatementQuery request, CancellationToken cancellationToken)
    {
        // 1. Fetch template definition
        var targetId = new Domain.Common.ReportTemplateId(request.TemplateCode);
        var template = await dbContext.ReportTemplates
            .Include(t => t.Lines)
            .FirstOrDefaultAsync(t => t.TemplateCode == request.TemplateCode || t.Id == targetId, cancellationToken);

        if (template == null)
        {
            return Result<FinancialStatementDto>.Failure($"Report template '{request.TemplateCode}' not found.");
        }

        // 2. Query aggregate account balances via high-speed Dapper SQL
        using var conn = connectionFactory.CreateConnection();
        if (conn.State != ConnectionState.Open)
            conn.Open();

        // For Balance Sheet, balances are cumulative up to ToDate (as-of-date)
        // For Income Statement, balances are within period (FromDate to ToDate)
        bool isBalanceSheet = template.StatementType == StatementType.BalanceSheet;

        string sql = isBalanceSheet
            ? @"SELECT AccountId, 
                       COALESCE(SUM(DebitAmount), 0) AS TotalDebit, 
                       COALESCE(SUM(CreditAmount), 0) AS TotalCredit
                FROM gl_entries
                WHERE PostingDate <= @ToDate
                GROUP BY AccountId;"
            : @"SELECT AccountId, 
                       COALESCE(SUM(DebitAmount), 0) AS TotalDebit, 
                       COALESCE(SUM(CreditAmount), 0) AS TotalCredit
                FROM gl_entries
                WHERE PostingDate >= @FromDate AND PostingDate <= @ToDate
                GROUP BY AccountId;";

        var param = new
        {
            FromDate = request.FromDate.ToString("yyyy-MM-dd"),
            ToDate = request.ToDate.ToString("yyyy-MM-dd")
        };

        var rawBalances = (await conn.QueryAsync<AccountBalanceRow>(sql, param)).ToList();
        var balanceDict = rawBalances.ToDictionary(r => r.AccountId, r => (r.TotalDebit, r.TotalCredit));

        // 3. Process lines in dependency-aware order
        var sortedLines = template.Lines.OrderBy(l => l.LineNumber).ToList();
        var computedValues = new Dictionary<string, decimal>(StringComparer.OrdinalIgnoreCase);

        // Pass A: Evaluate all Leaf Accounts first
        foreach (var line in sortedLines.Where(l => l.NodeType == LineNodeType.LeafAccount))
        {
            decimal lineAmount = 0m;
            if (!string.IsNullOrWhiteSpace(line.AccountPattern))
            {
                var patterns = line.AccountPattern.Split(',', StringSplitOptions.RemoveEmptyEntries | StringSplitOptions.TrimEntries);
                foreach (var (accId, (debit, credit)) in balanceDict)
                {
                    if (patterns.Any(p => accId.StartsWith(p, StringComparison.OrdinalIgnoreCase)))
                    {
                        var val = line.BalanceSide switch
                        {
                            BalanceSide.Debit => debit - credit,
                            BalanceSide.Credit => credit - debit,
                            _ => (debit - credit) // Net
                        };
                        lineAmount += val;
                    }
                }
            }

            if (line.InvertSign)
            {
                lineAmount = -lineAmount;
            }

            computedValues[line.LineCode] = lineAmount;
        }

        // Pass B: Evaluate Calculations iteratively until stable
        var calcLines = sortedLines.Where(l => l.NodeType == LineNodeType.Calculation).ToList();
        for (int iteration = 0; iteration < 10; iteration++)
        {
            bool anyChanged = false;
            foreach (var line in calcLines)
            {
                if (!string.IsNullOrWhiteSpace(line.CalculationLogic))
                {
                    var newVal = EvaluateFormula(line.CalculationLogic, computedValues);
                    if (line.InvertSign)
                    {
                        newVal = -newVal;
                    }

                    if (!computedValues.TryGetValue(line.LineCode, out var oldVal) || oldVal != newVal)
                    {
                        computedValues[line.LineCode] = newVal;
                        anyChanged = true;
                    }
                }
            }
            if (!anyChanged) break;
        }

        var resultLines = sortedLines.Select(line => new FinancialStatementLineDto(
            line.LineCode,
            line.LineNumber,
            line.ItemName,
            (int)line.PrintStyle,
            (int)line.NodeType,
            computedValues.TryGetValue(line.LineCode, out var val) ? val : 0m)).ToList();

        // 4. Verification of Balance Sheet equation
        decimal totalAssets = 0m;
        decimal totalLiabilitiesAndEquity = 0m;
        bool isBalanced = true;

        if (isBalanceSheet)
        {
            computedValues.TryGetValue("270", out totalAssets);
            computedValues.TryGetValue("440", out totalLiabilitiesAndEquity);
            isBalanced = totalAssets == totalLiabilitiesAndEquity;
        }

        var periodTitle = isBalanceSheet
            ? $"T?i ng�y {request.ToDate:dd/MM/yyyy}"
            : $"T? ng�y {request.FromDate:dd/MM/yyyy} d?n ng�y {request.ToDate:dd/MM/yyyy}";

        var statementDto = new FinancialStatementDto(
            template.TemplateCode,
            template.TemplateName,
            periodTitle,
            resultLines,
            isBalanced,
            totalAssets,
            totalLiabilitiesAndEquity);

        return Result<FinancialStatementDto>.Success(statementDto);
    }

    private static decimal EvaluateFormula(string formula, IReadOnlyDictionary<string, decimal> values)
    {
        // Supports linear expressions e.g. "110 + 120 + 130" or "01 - 02" or "20 + 21 - 22 - 25 - 26"
        var tokens = Regex.Matches(formula, @"([+-]?)\s*([a-zA-Z0-9_]+)");
        decimal total = 0m;

        foreach (Match match in tokens)
        {
            var op = match.Groups[1].Value;
            var code = match.Groups[2].Value;
            var val = values.TryGetValue(code, out var v) ? v : 0m;

            if (op == "-")
            {
                total -= val;
            }
            else
            {
                total += val;
            }
        }

        return total;
    }
}

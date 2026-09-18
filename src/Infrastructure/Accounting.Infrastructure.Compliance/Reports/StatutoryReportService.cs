using Accounting.Application.Common.Interfaces;
using Accounting.Domain.Enums;
using Microsoft.EntityFrameworkCore;

namespace Accounting.Infrastructure.Compliance.Reports;

public class StatutoryReportService : IStatutoryReportService
{
    private readonly IAccountingDbContext _context;

    public StatutoryReportService(IAccountingDbContext context)
    {
        _context = context;
    }

    public async Task<FinancialPositionReportDto> GetFinancialPositionReportAsync(DateTime asOfDate, CancellationToken cancellationToken = default)
    {
        var postedLines = await _context.VoucherLines
            .Include(l => l.DebitAccount)
            .Include(l => l.CreditAccount)
            .Where(l => _context.Vouchers.Any(v => v.Id == l.VoucherId && v.Status == VoucherStatus.Posted && v.PostingDate <= asOfDate))
            .ToListAsync(cancellationToken);

        decimal CalculateNetBalance(string accountPrefix, bool isDebitNormal)
        {
            var debit = postedLines.Where(l => l.DebitAccount != null && l.DebitAccount.AccountNumber.StartsWith(accountPrefix)).Sum(l => l.Amount);
            var credit = postedLines.Where(l => l.CreditAccount != null && l.CreditAccount.AccountNumber.StartsWith(accountPrefix)).Sum(l => l.Amount);
            return isDebitNormal ? (debit - credit) : (credit - debit);
        }

        // Assets
        var cash = Math.Max(0, CalculateNetBalance("111", true) + CalculateNetBalance("112", true));
        var receivables = Math.Max(0, CalculateNetBalance("131", true));
        var inventory = Math.Max(0, CalculateNetBalance("152", true) + CalculateNetBalance("153", true) + CalculateNetBalance("156", true));
        var vatDeductible = Math.Max(0, CalculateNetBalance("133", true));
        var shortTermAssets = cash + receivables + inventory + vatDeductible;

        var fixedAssetCost = Math.Max(0, CalculateNetBalance("211", true));
        var depreciation = Math.Max(0, CalculateNetBalance("214", false));
        var netFixedAssets = fixedAssetCost - depreciation;
        var prepaid = Math.Max(0, CalculateNetBalance("242", true));
        var longTermAssets = netFixedAssets + prepaid;

        var totalAssets = shortTermAssets + longTermAssets;

        // Liabilities & Equity
        var payables = Math.Max(0, CalculateNetBalance("331", false));
        var taxes = Math.Max(0, CalculateNetBalance("333", false));
        var salaryPayable = Math.Max(0, CalculateNetBalance("334", false));
        var otherPayables = Math.Max(0, CalculateNetBalance("338", false));
        var liabilities = payables + taxes + salaryPayable + otherPayables;

        var capital = Math.Max(0, CalculateNetBalance("411", false));
        var retainedEarnings = CalculateNetBalance("421", false);
        var equity = capital + retainedEarnings;

        var totalResources = liabilities + equity;

        var lines = new List<ReportLineDto>
        {
            new("100", "A. TÀI SẢN NGẮN HẠN", "I+II+III+IV", shortTermAssets, 0),
            new("110", "I. Tiền và các khoản tương đương tiền", "TK 111, 112", cash, 0),
            new("130", "II. Các khoản phải thu ngắn hạn", "TK 131", receivables, 0),
            new("140", "III. Hàng tồn kho", "TK 152, 153, 156", inventory, 0),
            new("150", "IV. Tài sản ngắn hạn khác", "TK 133", vatDeductible, 0),
            new("200", "B. TÀI SẢN DÀI HẠN", "I+II", longTermAssets, 0),
            new("220", "I. Tài sản cố định hữu hình", "TK 211, 214", netFixedAssets, 0),
            new("221", "   - Nguyên giá", "TK 211", fixedAssetCost, 0),
            new("222", "   - Hao mòn lũy kế", "TK 214", -depreciation, 0),
            new("260", "II. Tài sản dài hạn khác (Chi phí trả trước)", "TK 242", prepaid, 0),
            new("270", "TỔNG CỘNG TÀI SẢN (100 + 200)", "", totalAssets, 0),

            new("300", "C. NỢ PHẢI TRẢ", "I", liabilities, 0),
            new("310", "I. Nợ ngắn hạn", "", liabilities, 0),
            new("311", "   1. Phải trả người bán ngắn hạn", "TK 331", payables, 0),
            new("313", "   2. Thuế và các khoản phải nộp Nhà nước", "TK 333", taxes, 0),
            new("314", "   3. Phải trả người lao động", "TK 334", salaryPayable, 0),
            new("319", "   4. Phải trả ngắn hạn khác", "TK 338", otherPayables, 0),
            new("400", "D. VỐN CHỦ SỞ HỮU", "I", equity, 0),
            new("410", "I. Vốn chủ sở hữu", "", equity, 0),
            new("411", "   1. Vốn đầu tư của chủ sở hữu", "TK 411", capital, 0),
            new("421", "   2. Lợi nhuận sau thuế chưa phân phối", "TK 421", retainedEarnings, 0),
            new("440", "TỔNG CỘNG NGUỒN VỐN (300 + 400)", "", totalResources, 0)
        };

        return new FinancialPositionReportDto(
            asOfDate,
            shortTermAssets,
            longTermAssets,
            totalAssets,
            liabilities,
            equity,
            totalResources,
            lines);
    }

    public async Task<IncomeStatementReportDto> GetIncomeStatementReportAsync(DateTime fromDate, DateTime toDate, CancellationToken cancellationToken = default)
    {
        var postedLines = await _context.VoucherLines
            .Include(l => l.DebitAccount)
            .Include(l => l.CreditAccount)
            .Where(l => _context.Vouchers.Any(v => v.Id == l.VoucherId && v.Status == VoucherStatus.Posted && v.PostingDate >= fromDate && v.PostingDate <= toDate))
            .ToListAsync(cancellationToken);

        decimal CalculateIncurred(string accountPrefix, bool isCredit)
        {
            return postedLines
                .Where(l => isCredit
                    ? (l.CreditAccount != null && l.CreditAccount.AccountNumber.StartsWith(accountPrefix))
                    : (l.DebitAccount != null && l.DebitAccount.AccountNumber.StartsWith(accountPrefix)))
                .Sum(l => l.Amount);
        }

        var grossRevenue = CalculateIncurred("511", true);
        var deductions = CalculateIncurred("521", false);
        var netRevenue = grossRevenue - deductions;
        var cogs = CalculateIncurred("632", false);
        var grossProfit = netRevenue - cogs;
        var financialIncome = CalculateIncurred("515", true);
        var financialExpense = CalculateIncurred("635", false);
        var sellingExpense = CalculateIncurred("641", false);
        var adminExpense = CalculateIncurred("642", false);
        var operatingProfit = grossProfit + financialIncome - financialExpense - sellingExpense - adminExpense;
        var otherIncome = CalculateIncurred("711", true);
        var otherExpense = CalculateIncurred("811", false);
        var otherProfit = otherIncome - otherExpense;
        var profitBeforeTax = operatingProfit + otherProfit;
        var taxExpense = CalculateIncurred("821", false);
        var netProfit = profitBeforeTax - taxExpense;

        var lines = new List<ReportLineDto>
        {
            new("01", "1. Doanh thu bán hàng và cung cấp dịch vụ", "TK 511", grossRevenue, 0),
            new("02", "2. Các khoản giảm trừ doanh thu", "TK 521", deductions, 0),
            new("10", "3. Doanh thu thuần về bán hàng và CCDV (10 = 01 - 02)", "", netRevenue, 0),
            new("11", "4. Giá vốn hàng bán", "TK 632", cogs, 0),
            new("20", "5. Lợi nhuận gộp về bán hàng và CCDV (20 = 10 - 11)", "", grossProfit, 0),
            new("21", "6. Doanh thu hoạt động tài chính", "TK 515", financialIncome, 0),
            new("22", "7. Chi phí tài chính", "TK 635", financialExpense, 0),
            new("25", "8. Chi phí bán hàng", "TK 641", sellingExpense, 0),
            new("26", "9. Chi phí quản lý doanh nghiệp", "TK 642", adminExpense, 0),
            new("30", "10. Lợi nhuận thuần từ hoạt động kinh doanh (30 = 20 + 21 - 22 - 25 - 26)", "", operatingProfit, 0),
            new("31", "11. Thu nhập khác", "TK 711", otherIncome, 0),
            new("32", "12. Chi phí khác", "TK 811", otherExpense, 0),
            new("40", "13. Lợi nhuận khác (40 = 31 - 32)", "", otherProfit, 0),
            new("50", "14. Tổng lợi nhuận kế toán trước thuế (50 = 30 + 40)", "", profitBeforeTax, 0),
            new("51", "15. Chi phí thuế TNDN hiện hành", "TK 821", taxExpense, 0),
            new("60", "16. Lợi nhuận sau thuế thu nhập doanh nghiệp (60 = 50 - 51)", "", netProfit, 0)
        };

        return new IncomeStatementReportDto(
            fromDate,
            toDate,
            grossRevenue,
            deductions,
            netRevenue,
            cogs,
            grossProfit,
            financialIncome,
            financialExpense,
            sellingExpense,
            adminExpense,
            operatingProfit,
            otherIncome,
            otherExpense,
            otherProfit,
            profitBeforeTax,
            taxExpense,
            netProfit,
            lines);
    }

    public async Task<CashFlowReportDto> GetCashFlowReportAsync(DateTime fromDate, DateTime toDate, bool isDirectMethod, CancellationToken cancellationToken = default)
    {
        var cashInflow = await _context.VoucherLines
            .Where(l => l.DebitAccount != null && (l.DebitAccount.AccountNumber.StartsWith("111") || l.DebitAccount.AccountNumber.StartsWith("112")))
            .Where(l => _context.Vouchers.Any(v => v.Id == l.VoucherId && v.Status == VoucherStatus.Posted && v.PostingDate >= fromDate && v.PostingDate <= toDate))
            .SumAsync(l => l.Amount, cancellationToken);

        var cashOutflow = await _context.VoucherLines
            .Where(l => l.CreditAccount != null && (l.CreditAccount.AccountNumber.StartsWith("111") || l.CreditAccount.AccountNumber.StartsWith("112")))
            .Where(l => _context.Vouchers.Any(v => v.Id == l.VoucherId && v.Status == VoucherStatus.Posted && v.PostingDate >= fromDate && v.PostingDate <= toDate))
            .SumAsync(l => l.Amount, cancellationToken);

        var netOperating = cashInflow - cashOutflow;
        var beginningCash = 0m;
        var endingCash = beginningCash + netOperating;

        var lines = new List<ReportLineDto>
        {
            new("01", "1. Tiền thu từ bán hàng, cung cấp dịch vụ và doanh thu khác", "", cashInflow, 0),
            new("02", "2. Tiền chi trả cho người cung cấp hàng hóa và dịch vụ", "", cashOutflow, 0),
            new("20", "Lưu chuyển tiền thuần từ hoạt động kinh doanh", "", netOperating, 0),
            new("50", "Lưu chuyển tiền thuần trong kỳ (20 + 30 + 40)", "", netOperating, 0),
            new("60", "Tiền và tương đương tiền đầu kỳ", "", beginningCash, 0),
            new("70", "Tiền và tương đương tiền cuối kỳ (50 + 60)", "", endingCash, 0)
        };

        return new CashFlowReportDto(
            fromDate,
            toDate,
            isDirectMethod,
            netOperating,
            0m,
            0m,
            netOperating,
            beginningCash,
            endingCash,
            lines);
    }
}

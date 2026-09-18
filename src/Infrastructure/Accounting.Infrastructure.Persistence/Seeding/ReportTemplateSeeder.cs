using Accounting.Domain.Common;
using Accounting.Domain.Reporting;
using Accounting.Infrastructure.Persistence.Context;
using Microsoft.EntityFrameworkCore;

namespace Accounting.Infrastructure.Persistence.Seeding;

public static class ReportTemplateSeeder
{
    public static async Task SeedReportTemplatesAsync(AccountingDbContext context, CancellationToken cancellationToken = default)
    {
        if (await context.ReportTemplates.AnyAsync(cancellationToken))
            return;

        // 1. Template B01-DN: B?ng C�n d?i k? to�n (Th�ng tu 99/2025/TT-BTC)
        var b01Id = new ReportTemplateId("B01-DN-TT99");
        var b01 = new ReportTemplate(
            b01Id,
            StatementType.BalanceSheet,
            "B01-DN",
            "B?NG C�N �?I K? TO�N",
            "Th�ng tu 99/2025/TT-BTC");

        // --- ASSETS ---
        // A. T�I S?N NG?N H?N (M� 100) = 110 + 120 + 130 + 140 + 150
        b01.AddLine(new ReportLine(Guid.NewGuid(), b01Id, "100", 1, "A. T�I S?N NG?N H?N", PrintStyle.Bold, LineNodeType.Calculation, calculationLogic: "110 + 120 + 130 + 140 + 150"));
        b01.AddLine(new ReportLine(Guid.NewGuid(), b01Id, "110", 2, "I. Ti?n v� tuong duong ti?n", PrintStyle.Bold, LineNodeType.LeafAccount, accountPattern: "111,112,113", balanceSide: BalanceSide.Debit));
        b01.AddLine(new ReportLine(Guid.NewGuid(), b01Id, "120", 3, "II. �?u tu t�i ch�nh ng?n h?n", PrintStyle.Bold, LineNodeType.LeafAccount, accountPattern: "121,128", balanceSide: BalanceSide.Debit));
        b01.AddLine(new ReportLine(Guid.NewGuid(), b01Id, "130", 4, "III. C�c kho?n ph?i thu ng?n h?n", PrintStyle.Bold, LineNodeType.LeafAccount, accountPattern: "131,136,138", balanceSide: BalanceSide.Debit));
        b01.AddLine(new ReportLine(Guid.NewGuid(), b01Id, "140", 5, "IV. H�ng t?n kho", PrintStyle.Bold, LineNodeType.LeafAccount, accountPattern: "151,152,153,154,155,156,157,158", balanceSide: BalanceSide.Debit));
        b01.AddLine(new ReportLine(Guid.NewGuid(), b01Id, "150", 6, "V. T�i s?n ng?n h?n kh�c", PrintStyle.Bold, LineNodeType.LeafAccount, accountPattern: "133,242", balanceSide: BalanceSide.Debit));

        // B. T�I S?N D�I H?N (M� 200) = 210 + 220 + 250 + 260
        b01.AddLine(new ReportLine(Guid.NewGuid(), b01Id, "200", 7, "B. T�I S?N D�I H?N", PrintStyle.Bold, LineNodeType.Calculation, calculationLogic: "210 + 220 + 250 + 260"));
        b01.AddLine(new ReportLine(Guid.NewGuid(), b01Id, "210", 8, "I. C�c kho?n ph?i thu d�i h?n", PrintStyle.Bold, LineNodeType.LeafAccount, accountPattern: "211,212", balanceSide: BalanceSide.Debit));
        b01.AddLine(new ReportLine(Guid.NewGuid(), b01Id, "220", 9, "II. T�i s?n c? d?nh", PrintStyle.Bold, LineNodeType.LeafAccount, accountPattern: "211,213,214", balanceSide: BalanceSide.Net));
        b01.AddLine(new ReportLine(Guid.NewGuid(), b01Id, "250", 10, "III. �?u tu t�i ch�nh d�i h?n", PrintStyle.Bold, LineNodeType.LeafAccount, accountPattern: "221,222,228", balanceSide: BalanceSide.Debit));
        b01.AddLine(new ReportLine(Guid.NewGuid(), b01Id, "260", 11, "IV. T�i s?n d�i h?n kh�c", PrintStyle.Bold, LineNodeType.LeafAccount, accountPattern: "241", balanceSide: BalanceSide.Debit));

        // T?NG C?NG T�I S?N (M� 270) = 100 + 200
        b01.AddLine(new ReportLine(Guid.NewGuid(), b01Id, "270", 12, "T?NG C?NG T�I S?N (270 = 100 + 200)", PrintStyle.Bold, LineNodeType.Calculation, calculationLogic: "100 + 200"));

        // --- LIABILITIES & EQUITY ---
        // C. N? PH?I TR? (M� 300) = 310 + 320
        b01.AddLine(new ReportLine(Guid.NewGuid(), b01Id, "300", 13, "C. N? PH?I TR?", PrintStyle.Bold, LineNodeType.Calculation, calculationLogic: "310 + 320"));
        b01.AddLine(new ReportLine(Guid.NewGuid(), b01Id, "310", 14, "I. N? ng?n h?n", PrintStyle.Bold, LineNodeType.LeafAccount, accountPattern: "331,333,334,335,338", balanceSide: BalanceSide.Credit));
        b01.AddLine(new ReportLine(Guid.NewGuid(), b01Id, "320", 15, "II. N? d�i h?n", PrintStyle.Bold, LineNodeType.LeafAccount, accountPattern: "341,342", balanceSide: BalanceSide.Credit));

        // D. V?N CH? S? H?U (M� 400) = 410
        b01.AddLine(new ReportLine(Guid.NewGuid(), b01Id, "400", 16, "D. V?N CH? S? H?U", PrintStyle.Bold, LineNodeType.Calculation, calculationLogic: "410"));
        b01.AddLine(new ReportLine(Guid.NewGuid(), b01Id, "410", 17, "I. V?n ch? s? h?u", PrintStyle.Bold, LineNodeType.LeafAccount, accountPattern: "411,412,413,414,418,421", balanceSide: BalanceSide.Credit));

        // T?NG C?NG NGU?N V?N (M� 440) = 300 + 400
        b01.AddLine(new ReportLine(Guid.NewGuid(), b01Id, "440", 18, "T?NG C?NG NGU?N V?N (440 = 300 + 400)", PrintStyle.Bold, LineNodeType.Calculation, calculationLogic: "300 + 400"));

        // 2. Template B02-DN: B�o c�o K?t qu? ho?t d?ng kinh doanh (Th�ng tu 99/2025/TT-BTC)
        var b02Id = new ReportTemplateId("B02-DN-TT99");
        var b02 = new ReportTemplate(
            b02Id,
            StatementType.IncomeStatement,
            "B02-DN",
            "B�O C�O K?T QU? HO?T �?NG KINH DOANH",
            "Th�ng tu 99/2025/TT-BTC");

        b02.AddLine(new ReportLine(Guid.NewGuid(), b02Id, "01", 1, "1. Doanh thu b�n h�ng v� cung c?p d?ch v?", PrintStyle.Normal, LineNodeType.LeafAccount, accountPattern: "511", balanceSide: BalanceSide.Credit));
        b02.AddLine(new ReportLine(Guid.NewGuid(), b02Id, "02", 2, "2. C�c kho?n gi?m tr? doanh thu", PrintStyle.Normal, LineNodeType.LeafAccount, accountPattern: "521", balanceSide: BalanceSide.Debit));
        b02.AddLine(new ReportLine(Guid.NewGuid(), b02Id, "10", 3, "3. Doanh thu thu?n (10 = 01 - 02)", PrintStyle.Bold, LineNodeType.Calculation, calculationLogic: "01 - 02"));
        b02.AddLine(new ReportLine(Guid.NewGuid(), b02Id, "11", 4, "4. Gi� v?n h�ng b�n", PrintStyle.Normal, LineNodeType.LeafAccount, accountPattern: "632", balanceSide: BalanceSide.Debit));
        b02.AddLine(new ReportLine(Guid.NewGuid(), b02Id, "20", 5, "5. L?i nhu?n g?p (20 = 10 - 11)", PrintStyle.Bold, LineNodeType.Calculation, calculationLogic: "10 - 11"));
        b02.AddLine(new ReportLine(Guid.NewGuid(), b02Id, "21", 6, "6. Doanh thu ho?t d?ng t�i ch�nh", PrintStyle.Normal, LineNodeType.LeafAccount, accountPattern: "515", balanceSide: BalanceSide.Credit));
        b02.AddLine(new ReportLine(Guid.NewGuid(), b02Id, "22", 7, "7. Chi ph� t�i ch�nh", PrintStyle.Normal, LineNodeType.LeafAccount, accountPattern: "635", balanceSide: BalanceSide.Debit));
        b02.AddLine(new ReportLine(Guid.NewGuid(), b02Id, "25", 8, "8. Chi ph� b�n h�ng", PrintStyle.Normal, LineNodeType.LeafAccount, accountPattern: "641", balanceSide: BalanceSide.Debit));
        b02.AddLine(new ReportLine(Guid.NewGuid(), b02Id, "26", 9, "9. Chi ph� qu?n l� doanh nghi?p", PrintStyle.Normal, LineNodeType.LeafAccount, accountPattern: "642", balanceSide: BalanceSide.Debit));
        b02.AddLine(new ReportLine(Guid.NewGuid(), b02Id, "30", 10, "10. L?i nhu?n thu?n t? H�KD (30 = 20 + 21 - 22 - 25 - 26)", PrintStyle.Bold, LineNodeType.Calculation, calculationLogic: "20 + 21 - 22 - 25 - 26"));
        b02.AddLine(new ReportLine(Guid.NewGuid(), b02Id, "31", 11, "11. Thu nh?p kh�c", PrintStyle.Normal, LineNodeType.LeafAccount, accountPattern: "711", balanceSide: BalanceSide.Credit));
        b02.AddLine(new ReportLine(Guid.NewGuid(), b02Id, "32", 12, "12. Chi ph� kh�c", PrintStyle.Normal, LineNodeType.LeafAccount, accountPattern: "811", balanceSide: BalanceSide.Debit));
        b02.AddLine(new ReportLine(Guid.NewGuid(), b02Id, "40", 13, "13. L?i nhu?n kh�c (40 = 31 - 32)", PrintStyle.Bold, LineNodeType.Calculation, calculationLogic: "31 - 32"));
        b02.AddLine(new ReportLine(Guid.NewGuid(), b02Id, "50", 14, "14. T?ng l?i nhu?n k? to�n tru?c thu? (50 = 30 + 40)", PrintStyle.Bold, LineNodeType.Calculation, calculationLogic: "30 + 40"));
        b02.AddLine(new ReportLine(Guid.NewGuid(), b02Id, "51", 15, "15. Chi ph� thu? TNDN hi?n h�nh", PrintStyle.Normal, LineNodeType.LeafAccount, accountPattern: "821", balanceSide: BalanceSide.Debit));
        b02.AddLine(new ReportLine(Guid.NewGuid(), b02Id, "60", 16, "16. L?i nhu?n sau thu? TNDN (60 = 50 - 51)", PrintStyle.Bold, LineNodeType.Calculation, calculationLogic: "50 - 51"));

        context.ReportTemplates.AddRange(b01, b02);
        await context.SaveChangesAsync(cancellationToken);
    }
}

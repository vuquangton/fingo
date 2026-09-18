using System.Globalization;
using System.IO;
using System.Text;

namespace Accounting.WpfApp.Services;

public record ExcelImportVoucherLine(
    string DebitAccount,
    string CreditAccount,
    decimal Amount,
    string Description);

public record ExcelImportResult(
    bool Success,
    IReadOnlyList<ExcelImportVoucherLine> Lines,
    IReadOnlyList<string> Errors);

public class ExcelVoucherInteropService
{
    public static ExcelImportResult ImportFromCsv(string csvContent)
    {
        var lines = new List<ExcelImportVoucherLine>();
        var errors = new List<string>();

        using var reader = new StringReader(csvContent);
        string? line;
        int rowNum = 0;

        while ((line = reader.ReadLine()) != null)
        {
            rowNum++;
            if (string.IsNullOrWhiteSpace(line)) continue;
            if (rowNum == 1 && (line.Contains("Nợ", StringComparison.OrdinalIgnoreCase) || line.Contains("Debit", StringComparison.OrdinalIgnoreCase)))
            {
                // Skip header row
                continue;
            }

            var parts = line.Split([',', ';', '\t']);
            if (parts.Length < 3)
            {
                errors.Add($"Dòng {rowNum}: Không đủ cột dữ liệu (cần ít nhất: TK Nợ, TK Có, Số tiền).");
                continue;
            }

            var debit = parts[0].Trim();
            var credit = parts[1].Trim();
            var amountStr = parts[2].Trim().Replace(".", "").Replace(",", ".");
            var desc = parts.Length > 3 ? parts[3].Trim() : string.Empty;

            if (!decimal.TryParse(amountStr, NumberStyles.Any, CultureInfo.InvariantCulture, out var amount) || amount <= 0)
            {
                errors.Add($"Dòng {rowNum}: Số tiền '{parts[2]}' không hợp lệ.");
                continue;
            }

            if (string.IsNullOrWhiteSpace(debit) || string.IsNullOrWhiteSpace(credit))
            {
                errors.Add($"Dòng {rowNum}: Tài khoản Nợ hoặc Có bị để trống.");
                continue;
            }

            lines.Add(new ExcelImportVoucherLine(debit, credit, amount, desc));
        }

        return new ExcelImportResult(errors.Count == 0, lines, errors);
    }

    public static string ExportToCsv(IEnumerable<ExcelImportVoucherLine> lines)
    {
        var sb = new StringBuilder();
        sb.AppendLine("Tài khoản Nợ,Tài khoản Có,Số tiền,Diễn giải chi tiết");
        foreach (var l in lines)
        {
            var cleanDesc = l.Description.Replace("\"", "\"\"");
            sb.AppendLine($"{l.DebitAccount},{l.CreditAccount},{l.Amount:0},\"{cleanDesc}\"");
        }
        return sb.ToString();
    }
}

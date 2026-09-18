using System.Text;

namespace Accounting.Infrastructure.Compliance.Services;

public interface IPrintDocumentGenerator
{
    string GenerateHtmlPrintout(
        string documentTitle,
        string subtitle,
        IReadOnlyList<string> headers,
        IEnumerable<object[]> rows,
        IReadOnlyList<string>? signatureTitles = null,
        string? companyName = null,
        string? taxCode = null,
        string? address = null,
        string? directorName = null,
        string? chiefAccountantName = null);
}

public class PrintDocumentGenerator : IPrintDocumentGenerator
{
    public string GenerateHtmlPrintout(
        string documentTitle,
        string subtitle,
        IReadOnlyList<string> headers,
        IEnumerable<object[]> rows,
        IReadOnlyList<string>? signatureTitles = null,
        string? companyName = null,
        string? taxCode = null,
        string? address = null,
        string? directorName = null,
        string? chiefAccountantName = null)
    {
        var sb = new StringBuilder();
        sb.AppendLine("<!DOCTYPE html><html><head><meta charset='utf-8'><title>" + documentTitle + "</title>");
        sb.AppendLine("<style>");
        sb.AppendLine("body { font-family: 'Segoe UI', Arial, sans-serif; font-size: 13px; color: #0F172A; margin: 20px; }");
        sb.AppendLine(".company-box { margin-bottom: 20px; font-size: 12px; border-bottom: 1px solid #E2E8F0; padding-bottom: 10px; }");
        sb.AppendLine(".company-name { font-weight: bold; text-transform: uppercase; font-size: 13px; }");
        sb.AppendLine("h2 { text-align: center; margin-bottom: 4px; text-transform: uppercase; }");
        sb.AppendLine(".subtitle { text-align: center; font-style: italic; color: #64748B; margin-bottom: 24px; font-size: 12px; }");
        sb.AppendLine("table { width: 100%; border-collapse: collapse; margin-bottom: 40px; }");
        sb.AppendLine("th { background-color: #F1F5F9; border: 1px solid #CBD5E1; padding: 8px 10px; font-weight: bold; text-align: left; font-size: 12px; }");
        sb.AppendLine("td { border: 1px solid #E2E8F0; padding: 7px 10px; font-size: 12px; }");
        sb.AppendLine(".signatures { display: flex; justify-content: space-around; text-align: center; margin-top: 30px; page-break-inside: avoid; }");
        sb.AppendLine(".sig-block { font-weight: bold; font-size: 13px; min-width: 150px; }");
        sb.AppendLine(".sig-sub { font-weight: normal; font-style: italic; color: #64748B; font-size: 11px; margin-top: 4px; }");
        sb.AppendLine(".sig-name { margin-top: 60px; font-weight: bold; font-size: 13px; }");
        sb.AppendLine("</style></head><body>");

        if (!string.IsNullOrWhiteSpace(companyName))
        {
            sb.AppendLine("<div class='company-box'>");
            sb.AppendLine("<div class='company-name'>" + companyName + "</div>");
            if (!string.IsNullOrWhiteSpace(taxCode)) sb.AppendLine("<div>Mã số thuế: " + taxCode + "</div>");
            if (!string.IsNullOrWhiteSpace(address)) sb.AppendLine("<div>Địa chỉ: " + address + "</div>");
            sb.AppendLine("</div>");
        }

        sb.AppendLine("<h2>" + documentTitle + "</h2>");
        sb.AppendLine("<div class='subtitle'>" + subtitle + "</div>");

        sb.AppendLine("<table><thead><tr>");
        foreach (var h in headers)
        {
            sb.AppendLine("<th>" + h + "</th>");
        }
        sb.AppendLine("</tr></thead><tbody>");

        foreach (var r in rows)
        {
            sb.AppendLine("<tr>");
            foreach (var cell in r)
            {
                var val = cell is decimal dec ? dec.ToString("N2") : cell?.ToString() ?? string.Empty;
                sb.AppendLine("<td>" + val + "</td>");
            }
            sb.AppendLine("</tr>");
        }
        sb.AppendLine("</tbody></table>");

        var sigs = signatureTitles ?? new List<string> { "Người lập biểu", "Kế toán trưởng", "Giám đốc" };
        sb.AppendLine("<div class='signatures'>");
        foreach (var s in sigs)
        {
            var personName = s switch
            {
                "Giám đốc" => directorName,
                "Kế toán trưởng" => chiefAccountantName,
                _ => string.Empty
            };

            sb.AppendLine("<div class='sig-block'>" + s + "<div class='sig-sub'>(Ký, họ tên)</div>" +
                (!string.IsNullOrWhiteSpace(personName) ? $"<div class='sig-name'>{personName}</div>" : "<div style='height: 50px;'></div>") +
                "</div>");
        }
        sb.AppendLine("</div>");

        sb.AppendLine("</body></html>");
        return sb.ToString();
    }
}
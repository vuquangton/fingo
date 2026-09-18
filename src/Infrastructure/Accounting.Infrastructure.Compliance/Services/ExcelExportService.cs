using System.IO;
using ClosedXML.Excel;

namespace Accounting.Infrastructure.Compliance.Services;

public interface IExcelExportService
{
    byte[] ExportToExcel<T>(
        string sheetTitle,
        IEnumerable<string> headers,
        IEnumerable<T> rows,
        Func<T, object[]> rowSelector,
        string? reportSubtitle = null,
        string? companyName = null,
        string? taxCode = null,
        string? address = null);
}

public class ExcelExportService : IExcelExportService
{
    public byte[] ExportToExcel<T>(
        string sheetTitle,
        IEnumerable<string> headers,
        IEnumerable<T> rows,
        Func<T, object[]> rowSelector,
        string? reportSubtitle = null,
        string? companyName = null,
        string? taxCode = null,
        string? address = null)
    {
        using var workbook = new XLWorkbook();
        var worksheet = workbook.Worksheets.Add(sheetTitle.Length > 31 ? sheetTitle.Substring(0, 31) : sheetTitle);

        int currentRow = 1;

        // Authoritative Corporate Header Block
        if (!string.IsNullOrWhiteSpace(companyName))
        {
            worksheet.Cell(currentRow, 1).Value = companyName.ToUpperInvariant();
            worksheet.Cell(currentRow, 1).Style.Font.Bold = true;
            currentRow++;

            if (!string.IsNullOrWhiteSpace(taxCode))
            {
                worksheet.Cell(currentRow, 1).Value = $"Mã số thuế: {taxCode}";
                currentRow++;
            }

            if (!string.IsNullOrWhiteSpace(address))
            {
                worksheet.Cell(currentRow, 1).Value = $"Địa chỉ: {address}";
                currentRow++;
            }

            currentRow++; // Blank line after company header
        }

        // Title Block
        worksheet.Cell(currentRow, 1).Value = sheetTitle.ToUpperInvariant();
        worksheet.Cell(currentRow, 1).Style.Font.Bold = true;
        worksheet.Cell(currentRow, 1).Style.Font.FontSize = 14;
        currentRow++;

        if (!string.IsNullOrWhiteSpace(reportSubtitle))
        {
            worksheet.Cell(currentRow, 1).Value = reportSubtitle;
            worksheet.Cell(currentRow, 1).Style.Font.Italic = true;
            worksheet.Cell(currentRow, 1).Style.Font.FontSize = 11;
            currentRow++;
        }

        currentRow++; // Blank line

        // Headers
        var headerList = headers.ToList();
        for (int col = 0; col < headerList.Count; col++)
        {
            var cell = worksheet.Cell(currentRow, col + 1);
            cell.Value = headerList[col];
            cell.Style.Font.Bold = true;
            cell.Style.Fill.BackgroundColor = XLColor.FromHtml("#F1F5F9");
            cell.Style.Border.OutsideBorder = XLBorderStyleValues.Thin;
            cell.Style.Border.OutsideBorderColor = XLColor.FromHtml("#CBD5E1");
        }
        currentRow++;

        // Rows
        foreach (var item in rows)
        {
            var values = rowSelector(item);
            for (int col = 0; col < values.Length; col++)
            {
                var cell = worksheet.Cell(currentRow, col + 1);
                var val = values[col];

                if (val is decimal decVal)
                {
                    cell.Value = decVal;
                    cell.Style.NumberFormat.Format = "#,##0.00";
                }
                else if (val is int intVal)
                {
                    cell.Value = intVal;
                    cell.Style.NumberFormat.Format = "#,##0";
                }
                else if (val is DateOnly dateOnlyVal)
                {
                    cell.Value = dateOnlyVal.ToString("dd/MM/yyyy");
                }
                else if (val is DateTime dtVal)
                {
                    cell.Value = dtVal.ToString("dd/MM/yyyy");
                }
                else
                {
                    cell.Value = val?.ToString() ?? string.Empty;
                }

                cell.Style.Border.OutsideBorder = XLBorderStyleValues.Thin;
                cell.Style.Border.OutsideBorderColor = XLColor.FromHtml("#E2E8F0");
            }
            currentRow++;
        }

        worksheet.Columns().AdjustToContents();

        using var stream = new MemoryStream();
        workbook.SaveAs(stream);
        return stream.ToArray();
    }
}
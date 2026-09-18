using System.IO;
using System.IO.Packaging;
using System.Windows;
using System.Windows.Documents;
using System.Windows.Media;
using System.Windows.Xps.Packaging;
using System.Windows.Xps;

namespace Accounting.WpfApp.Services;

public record StatutoryVoucherData(
    string FormNumber,
    string Title,
    string CircularTitle,
    string VoucherNumber,
    DateTime VoucherDate,
    string CompanyName,
    string TaxCode,
    string CompanyAddress,
    string PersonName,
    string PersonAddress,
    string Reason,
    decimal Amount,
    string AmountInWords,
    string DebitAccount,
    string CreditAccount,
    IReadOnlyList<StatutoryVoucherLineData> Lines);

public record StatutoryVoucherLineData(
    string Description,
    string DebitAccount,
    string CreditAccount,
    decimal Amount);

public class StatutoryVoucherPrintEngine
{
    public static FlowDocument CreateDocument(StatutoryVoucherData data)
    {
        var doc = new FlowDocument
        {
            PagePadding = new Thickness(48),
            FontFamily = new FontFamily("Times New Roman"),
            FontSize = 13,
            Foreground = Brushes.Black,
            Background = Brushes.White,
            ColumnWidth = 800
        };

        var headerTable = new Table { CellSpacing = 0, Margin = new Thickness(0, 0, 0, 16) };
        headerTable.Columns.Add(new TableColumn { Width = new GridLength(420) });
        headerTable.Columns.Add(new TableColumn { Width = new GridLength(280) });

        var headerRowGroup = new TableRowGroup();
        var headerRow = new TableRow();

        var leftBlock = new Paragraph();
        leftBlock.Inlines.Add(new Bold(new Run(data.CompanyName.ToUpper())) { FontSize = 12 });
        leftBlock.Inlines.Add(new LineBreak());
        leftBlock.Inlines.Add(new Run($"Mã số thuế: {data.TaxCode}") { FontSize = 11 });
        leftBlock.Inlines.Add(new LineBreak());
        leftBlock.Inlines.Add(new Run($"Địa chỉ: {data.CompanyAddress}") { FontSize = 11 });
        headerRow.Cells.Add(new TableCell(leftBlock) { TextAlignment = TextAlignment.Left });

        var rightBlock = new Paragraph();
        rightBlock.Inlines.Add(new Bold(new Run($"Mẫu số {data.FormNumber}")) { FontSize = 12 });
        rightBlock.Inlines.Add(new LineBreak());
        rightBlock.Inlines.Add(new Italic(new Run(data.CircularTitle)) { FontSize = 10, Foreground = Brushes.DimGray });
        headerRow.Cells.Add(new TableCell(rightBlock) { TextAlignment = TextAlignment.Right });

        headerRowGroup.Rows.Add(headerRow);
        headerTable.RowGroups.Add(headerRowGroup);
        doc.Blocks.Add(headerTable);

        var titleBlock = new Paragraph { TextAlignment = TextAlignment.Center, Margin = new Thickness(0, 8, 0, 16) };
        titleBlock.Inlines.Add(new Bold(new Run(data.Title)) { FontSize = 20 });
        titleBlock.Inlines.Add(new LineBreak());
        titleBlock.Inlines.Add(new Italic(new Run($"Ngày {data.VoucherDate:dd} tháng {data.VoucherDate:MM} năm {data.VoucherDate:yyyy}")) { FontSize = 12 });
        titleBlock.Inlines.Add(new LineBreak());
        titleBlock.Inlines.Add(new Run($"Số: {data.VoucherNumber} | Nợ: {data.DebitAccount} | Có: {data.CreditAccount}") { FontSize = 12, FontWeight = FontWeights.SemiBold });
        doc.Blocks.Add(titleBlock);

        var bodyBlock = new Paragraph { LineHeight = 22 };
        bodyBlock.Inlines.Add(new Run($"Họ và tên người nộp/nhận tiền: {data.PersonName}\n"));
        bodyBlock.Inlines.Add(new Run($"Địa chỉ: {data.PersonAddress}\n"));
        bodyBlock.Inlines.Add(new Run($"Lý do nộp/chi: {data.Reason}\n"));
        bodyBlock.Inlines.Add(new Bold(new Run($"Số tiền: {data.Amount:N0} VND\n")));
        bodyBlock.Inlines.Add(new Italic(new Run($"(Viết bằng chữ: {data.AmountInWords})\n")));
        bodyBlock.Inlines.Add(new Run("Kèm theo: 01 chứng từ gốc\n"));
        doc.Blocks.Add(bodyBlock);

        if (data.Lines.Count > 0)
        {
            var lineTable = new Table { CellSpacing = 0, Margin = new Thickness(0, 12, 0, 20) };
            lineTable.Columns.Add(new TableColumn { Width = new GridLength(350) });
            lineTable.Columns.Add(new TableColumn { Width = new GridLength(90) });
            lineTable.Columns.Add(new TableColumn { Width = new GridLength(90) });
            lineTable.Columns.Add(new TableColumn { Width = new GridLength(170) });

            var rowGroup = new TableRowGroup();

            var tableHeader = new TableRow();
            tableHeader.Cells.Add(CreateCell("Nội dung chi tiết", isBold: true, isHeader: true));
            tableHeader.Cells.Add(CreateCell("TK Nợ", isBold: true, isHeader: true));
            tableHeader.Cells.Add(CreateCell("TK Có", isBold: true, isHeader: true));
            tableHeader.Cells.Add(CreateCell("Số tiền (VND)", isBold: true, isHeader: true));
            rowGroup.Rows.Add(tableHeader);

            foreach (var line in data.Lines)
            {
                var r = new TableRow();
                r.Cells.Add(CreateCell(line.Description));
                r.Cells.Add(CreateCell(line.DebitAccount, TextAlignment.Center));
                r.Cells.Add(CreateCell(line.CreditAccount, TextAlignment.Center));
                r.Cells.Add(CreateCell($"{line.Amount:N0}", TextAlignment.Right));
                rowGroup.Rows.Add(r);
            }

            lineTable.RowGroups.Add(rowGroup);
            doc.Blocks.Add(lineTable);
        }

        var sigTable = new Table { CellSpacing = 0, Margin = new Thickness(0, 20, 0, 0) };
        sigTable.Columns.Add(new TableColumn { Width = new GridLength(175) });
        sigTable.Columns.Add(new TableColumn { Width = new GridLength(175) });
        sigTable.Columns.Add(new TableColumn { Width = new GridLength(175) });
        sigTable.Columns.Add(new TableColumn { Width = new GridLength(175) });

        var sigGroup = new TableRowGroup();
        var sigTitleRow = new TableRow();
        sigTitleRow.Cells.Add(CreateSignatureCell("Giám đốc", "(Ký, họ tên, đóng dấu)"));
        sigTitleRow.Cells.Add(CreateSignatureCell("Kế toán trưởng", "(Ký, họ tên)"));
        sigTitleRow.Cells.Add(CreateSignatureCell("Thủ quỹ", "(Ký, họ tên)"));
        sigTitleRow.Cells.Add(CreateSignatureCell("Người lập phiếu", "(Ký, họ tên)"));
        sigGroup.Rows.Add(sigTitleRow);

        sigTable.RowGroups.Add(sigGroup);
        doc.Blocks.Add(sigTable);

        return doc;
    }

    private static TableCell CreateCell(string text, TextAlignment alignment = TextAlignment.Left, bool isBold = false, bool isHeader = false)
    {
        var p = new Paragraph(isBold ? new Bold(new Run(text)) : new Run(text))
        {
            TextAlignment = alignment,
            Margin = new Thickness(0)
        };
        return new TableCell(p)
        {
            BorderBrush = Brushes.Black,
            BorderThickness = new Thickness(0.5),
            Padding = new Thickness(6, 4, 6, 4),
            Background = isHeader ? new SolidColorBrush(Color.FromRgb(245, 245, 245)) : Brushes.Transparent
        };
    }

    private static TableCell CreateSignatureCell(string title, string sub)
    {
        var p = new Paragraph { TextAlignment = TextAlignment.Center };
        p.Inlines.Add(new Bold(new Run(title)) { FontSize = 12 });
        p.Inlines.Add(new LineBreak());
        p.Inlines.Add(new Italic(new Run(sub)) { FontSize = 10, Foreground = Brushes.DimGray });
        p.Inlines.Add(new LineBreak());
        p.Inlines.Add(new Run("\n\n\n\n"));
        return new TableCell(p) { BorderThickness = new Thickness(0) };
    }

    public static byte[] RenderToXpsBytes(FlowDocument doc)
    {
        using var memoryStream = new MemoryStream();
        using (var package = Package.Open(memoryStream, FileMode.Create, FileAccess.ReadWrite))
        {
            var uri = new Uri("memorystream://statutory_voucher.xps");
            PackageStore.AddPackage(uri, package);
            try
            {
                using var xpsDoc = new XpsDocument(package, CompressionOption.Normal, uri.AbsoluteUri);
                var writer = XpsDocument.CreateXpsDocumentWriter(xpsDoc);
                var paginator = ((IDocumentPaginatorSource)doc).DocumentPaginator;
                writer.Write(paginator);
            }
            finally
            {
                PackageStore.RemovePackage(uri);
            }
        }
        return memoryStream.ToArray();
    }
}

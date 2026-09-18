using Accounting.WpfApp.Services;

namespace Accounting.WpfApp.Tests;

public class StatutoryPrintAndExcelInteropTests
{
    [Fact]
    public void StatutoryVoucherPrintEngine_GeneratesValidFlowDocument()
    {
        // Arrange
        var lines = new List<StatutoryVoucherLineData>
        {
            new("Thu tiền bán hàng trực tiếp", "1111", "5111", 50_000_000m),
            new("Thu tiền thuế GTGT phải nộp", "1111", "33311", 5_000_000m)
        };

        var data = new StatutoryVoucherData(
            FormNumber: "01 - TT",
            Title: "PHIẾU THU",
            CircularTitle: "Ban hành theo TT số 99/2025/TT-BTC ngày 25/10/2025 của Bộ Tài chính",
            VoucherNumber: "PT-20260918-001",
            VoucherDate: new DateTime(2026, 9, 18),
            CompanyName: "CÔNG TY CỔ PHẦN KẾ TOÁN MẪU VIỆT NAM",
            TaxCode: "0101234567",
            CompanyAddress: "123 Phố Huế, Q. Hai Bà Trưng, Hà Nội",
            PersonName: "Nguyễn Văn Khách Hàng",
            PersonAddress: "Hà Nội",
            Reason: "Thu tiền thanh toán hợp đồng dịch vụ",
            Amount: 55_000_000m,
            AmountInWords: "Năm mươi lăm triệu đồng chẵn",
            DebitAccount: "1111",
            CreditAccount: "5111, 33311",
            Lines: lines
        );

        // Act
        var doc = StatutoryVoucherPrintEngine.CreateDocument(data);

        // Assert
        Assert.NotNull(doc);
        Assert.True(doc.Blocks.Count >= 4);
    }

    [Fact]
    public void ExcelVoucherInteropService_ImportFromCsv_ValidData_ParsesCorrectly()
    {
        // Arrange
        var csv = @"TK Nợ,TK Có,Số tiền,Diễn giải
1111,5111,10000000,Doanh thu bán hàng
1111,33311,1000000,Thuế GTGT bán hàng
1121,131,25000000,Thu nợ khách hàng ABC";

        // Act
        var result = ExcelVoucherInteropService.ImportFromCsv(csv);

        // Assert
        Assert.True(result.Success);
        Assert.Empty(result.Errors);
        Assert.Equal(3, result.Lines.Count);
        Assert.Equal("1111", result.Lines[0].DebitAccount);
        Assert.Equal("5111", result.Lines[0].CreditAccount);
        Assert.Equal(10_000_000m, result.Lines[0].Amount);
    }

    [Fact]
    public void ExcelVoucherInteropService_ExportToCsv_GeneratesValidCsv()
    {
        // Arrange
        var lines = new List<ExcelImportVoucherLine>
        {
            new("1111", "5111", 10_000_000m, "Doanh thu hàng hóa"),
            new("1121", "131", 20_000_000m, "Thu hồi công nợ")
        };

        // Act
        var csv = ExcelVoucherInteropService.ExportToCsv(lines);

        // Assert
        Assert.Contains("1111,5111,10000000,\"Doanh thu hàng hóa\"", csv);
        Assert.Contains("1121,131,20000000,\"Thu hồi công nợ\"", csv);
    }
}

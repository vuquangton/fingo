using Accounting.Application.Features.EInvoice;
using Accounting.Domain.Common;
using Accounting.Domain.MasterData.Accounts;
using Accounting.Domain.MasterData.Common;
using Accounting.Domain.Receivables;
using Accounting.Infrastructure.Compliance.EInvoice;
using Xunit;

namespace Accounting.Domain.Tests.Compliance;

public class EInvoiceIntegrationTests
{
    [Fact]
    public async Task StandardEInvoiceProvider_ShouldIssueInvoiceSuccessfully()
    {
        var adapter = new StandardEInvoiceProviderAdapter();
        var custId = PartnerId.New();

        var inv = new SalesInvoice(
            SalesInvoiceId.New(),
            "00000123",
            new DateOnly(2026, 3, 1),
            new DateOnly(2026, 3, 31),
            custId);

        inv.AddLine(new AccountId("5111"), 2, 5_000_000m, 10m, "Phan mem quan ly tai chinh");

        var result = await adapter.IssueInvoiceAsync(inv);

        Assert.True(result.Success);
        Assert.NotNull(result.InvoiceCode);
        Assert.StartsWith("HD-00000123", result.InvoiceCode);
        Assert.NotNull(result.ReservationCode);
        Assert.NotNull(result.ProviderTransactionId);
    }

    [Fact]
    public async Task ParseIncomingXml_ShouldNormalizeMof78XmlStructure()
    {
        var adapter = new StandardEInvoiceProviderAdapter();

        var xml = """
        <?xml version="1.0" encoding="utf-8"?>
        <HDon>
            <TTChung>
                <KHMSHDon>1</KHMSHDon>
                <KHHDon>C26TAA</KHHDon>
                <SHDon>00000789</SHDon>
                <NLap>2026-03-15</NLap>
            </TTChung>
            <NDHDon>
                <NBan>
                    <Ten>CONG TY TNHH VIEN THONG FPT</Ten>
                    <MST>0101778163</MST>
                    <DChi>Quan Cau Giay, Ha Noi</DChi>
                </NBan>
                <NMua>
                    <Ten>CONG TY CO PHAN CONG NGHE VIET</Ten>
                    <MST>0312345678</MST>
                </NMua>
                <DSHHDVu>
                    <HHDVu>
                        <Ten>Cuoc Internet FTTH Doanh Nghiep thang 03/2026</Ten>
                        <DVT>Thang</DVT>
                        <SoLuong>1</SoLuong>
                        <DonGia>800000</DonGia>
                        <ThanhTien>800000</ThanhTien>
                        <ThueSuat>10</ThueSuat>
                        <TienThue>80000</TienThue>
                    </HHDVu>
                </DSHHDVu>
                <TTTToan>
                    <TgTCThue>800000</TgTCThue>
                    <TgTThue>80000</TgTThue>
                    <TgTTToan>880000</TgTTToan>
                </TTTToan>
            </NDHDon>
        </HDon>
        """;

        var dto = await adapter.ParseIncomingXmlAsync(xml);

        Assert.NotNull(dto);
        Assert.Equal("1", dto.InvoiceTemplate);
        Assert.Equal("C26TAA", dto.InvoiceSeries);
        Assert.Equal("00000789", dto.InvoiceNumber);
        Assert.Equal(new DateOnly(2026, 3, 15), dto.InvoiceDate);
        Assert.Equal("0101778163", dto.SellerTaxCode);
        Assert.Equal("CONG TY TNHH VIEN THONG FPT", dto.SellerName);
        Assert.Equal("0312345678", dto.BuyerTaxCode);
        Assert.Equal(800_000m, dto.SubTotalAmount);
        Assert.Equal(80_000m, dto.VatAmount);
        Assert.Equal(880_000m, dto.TotalAmount);

        Assert.Single(dto.Lines);
        var line = dto.Lines[0];
        Assert.Equal("Cuoc Internet FTTH Doanh Nghiep thang 03/2026", line.ItemName);
        Assert.Equal(1, line.Quantity);
        Assert.Equal(800_000m, line.UnitPrice);
        Assert.Equal(800_000m, line.Amount);
        Assert.Equal(10m, line.VatRate);
        Assert.Equal(80_000m, line.VatAmount);
    }
}

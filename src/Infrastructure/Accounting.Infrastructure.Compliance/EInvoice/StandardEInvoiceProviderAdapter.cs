using System.Xml.Linq;
using Accounting.Application.Features.EInvoice;
using Accounting.Domain.Receivables;

namespace Accounting.Infrastructure.Compliance.EInvoice;

public class StandardEInvoiceProviderAdapter : IEInvoiceProviderAdapter
{
    public string ProviderKey => "MOCK";

    public Task<EInvoiceSubmissionResult> IssueInvoiceAsync(SalesInvoice invoice, CancellationToken cancellationToken = default)
    {
        var reservationCode = $"MOCK-{invoice.InvoiceNumber}-{DateTime.UtcNow:yyyyMMddHHmmss}";
        var transactionId = Guid.NewGuid().ToString("N");
        var invoiceCode = $"HD-{invoice.InvoiceNumber}";

        var result = new EInvoiceSubmissionResult(
            Success: true,
            ProviderTransactionId: transactionId,
            InvoiceCode: invoiceCode,
            ReservationCode: reservationCode,
            ErrorMessage: null
        );

        return Task.FromResult(result);
    }

    public Task<NormalizedIncomingInvoiceDto> ParseIncomingXmlAsync(string xmlContent, CancellationToken cancellationToken = default)
    {
        var doc = XDocument.Parse(xmlContent);

        string GetElementValue(params string[] names)
        {
            foreach (var name in names)
            {
                var el = doc.Descendants().FirstOrDefault(e => e.Name.LocalName.Equals(name, StringComparison.OrdinalIgnoreCase));
                if (el != null && !string.IsNullOrWhiteSpace(el.Value))
                    return el.Value.Trim();
            }
            return string.Empty;
        }

        var template = GetElementValue("MauSo", "KHMSHDon", "templateCode");
        var series = GetElementValue("KyHieu", "KHHDon", "invoiceSeries");
        var invNum = GetElementValue("SoHoaDon", "SHDon", "invoiceNumber");
        var dateStr = GetElementValue("NgayHoaDon", "NLap", "invoiceDate");

        DateOnly invDate = DateOnly.FromDateTime(DateTime.UtcNow);
        if (DateTime.TryParse(dateStr, out var parsedDt))
        {
            invDate = DateOnly.FromDateTime(parsedDt);
        }

        var nBan = doc.Descendants().FirstOrDefault(e => e.Name.LocalName.Equals("NBan", StringComparison.OrdinalIgnoreCase) || e.Name.LocalName.Equals("Seller", StringComparison.OrdinalIgnoreCase));
        var nMua = doc.Descendants().FirstOrDefault(e => e.Name.LocalName.Equals("NMua", StringComparison.OrdinalIgnoreCase) || e.Name.LocalName.Equals("Buyer", StringComparison.OrdinalIgnoreCase));

        string GetChildValue(XElement? parent, params string[] names)
        {
            if (parent == null) return string.Empty;
            foreach (var name in names)
            {
                var el = parent.Descendants().FirstOrDefault(e => e.Name.LocalName.Equals(name, StringComparison.OrdinalIgnoreCase));
                if (el != null && !string.IsNullOrWhiteSpace(el.Value))
                    return el.Value.Trim();
            }
            return string.Empty;
        }

        var sellerTax = GetChildValue(nBan, "MST", "TaxCode") is { Length: > 0 } st ? st : GetElementValue("NBanMST", "SellerTaxCode");
        var sellerName = GetChildValue(nBan, "Ten", "Name") is { Length: > 0 } sn ? sn : GetElementValue("TenNguoiBan", "NBanTen", "SellerName");
        var sellerAddr = GetChildValue(nBan, "DChi", "Address") is { Length: > 0 } sa ? sa : GetElementValue("DiaChiNguoiBan", "NBanDChi", "SellerAddress");

        var buyerTax = GetChildValue(nMua, "MST", "TaxCode") is { Length: > 0 } bt ? bt : GetElementValue("MSTNguoiMua", "NMuaMST", "BuyerTaxCode");
        var buyerName = GetChildValue(nMua, "Ten", "Name") is { Length: > 0 } bn ? bn : GetElementValue("TenNguoiMua", "NMuaTen", "BuyerName");

        decimal ParseDec(string val)
        {
            if (decimal.TryParse(val, System.Globalization.NumberStyles.Any, System.Globalization.CultureInfo.InvariantCulture, out var d))
                return d;
            if (decimal.TryParse(val, out var d2))
                return d2;
            return 0m;
        }

        var subTotal = ParseDec(GetElementValue("TongTienChuaThue", "TgTCThue", "TotalAmountWithoutVat"));
        var vatAmount = ParseDec(GetElementValue("TongTienThue", "TgTThue", "TotalVatAmount"));
        var totalAmount = ParseDec(GetElementValue("TongTienThanhToan", "TgTTToan", "TotalAmount"));

        var lineElements = doc.Descendants().Where(e =>
            e.Name.LocalName.Equals("HHDVu", StringComparison.OrdinalIgnoreCase) ||
            e.Name.LocalName.Equals("Item", StringComparison.OrdinalIgnoreCase) ||
            e.Name.LocalName.Equals("Line", StringComparison.OrdinalIgnoreCase)
        ).ToList();

        var lines = new List<NormalizedInvoiceLineDto>();
        int lineIdx = 1;

        foreach (var el in lineElements)
        {
            string GetChild(params string[] names)
            {
                foreach (var name in names)
                {
                    var child = el.Descendants().FirstOrDefault(c => c.Name.LocalName.Equals(name, StringComparison.OrdinalIgnoreCase));
                    if (child != null && !string.IsNullOrWhiteSpace(child.Value))
                        return child.Value.Trim();
                }
                return string.Empty;
            }

            var itemName = GetChild("THHDVu", "ItemName", "Ten");
            if (string.IsNullOrWhiteSpace(itemName)) continue;

            var unitName = GetChild("DVTinh", "UnitName", "DVT");
            var qty = ParseDec(GetChild("SLuong", "Quantity", "SoLuong"));
            var unitPrice = ParseDec(GetChild("DGia", "UnitPrice", "DonGia"));
            var amount = ParseDec(GetChild("ThTien", "Amount", "ThanhTien"));
            var vatRate = ParseDec(GetChild("TSuat", "VatRate", "ThueSuat"));
            var lineVat = ParseDec(GetChild("TThue", "VatAmount", "TienThue"));

            lines.Add(new NormalizedInvoiceLineDto(
                lineIdx++,
                itemName,
                unitName,
                qty,
                unitPrice,
                amount,
                vatRate,
                lineVat
            ));
        }

        return Task.FromResult(new NormalizedIncomingInvoiceDto(
            template,
            series,
            invNum,
            invDate,
            sellerTax,
            sellerName,
            sellerAddr,
            buyerTax,
            buyerName,
            subTotal,
            vatAmount,
            totalAmount,
            lines
        ));
    }
}

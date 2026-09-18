using System.Text;
using System.Xml.Linq;
using Accounting.Application.Common.Interfaces;
using Accounting.Domain.Enums;
using Microsoft.EntityFrameworkCore;

namespace Accounting.Infrastructure.Compliance.Tax;

public class TaxReportingService : ITaxReportingService
{
    private readonly IAccountingDbContext _context;

    public TaxReportingService(IAccountingDbContext context)
    {
        _context = context;
    }

    public async Task<VatDeclarationDto> GetVatDeclarationAsync(int year, int period, bool isQuarter, CancellationToken cancellationToken = default)
    {
        DateTime startDate;
        DateTime endDate;

        if (isQuarter)
        {
            var startMonth = (period - 1) * 3 + 1;
            startDate = new DateTime(year, startMonth, 1);
            endDate = startDate.AddMonths(3).AddTicks(-1);
        }
        else
        {
            startDate = new DateTime(year, period, 1);
            endDate = startDate.AddMonths(1).AddTicks(-1);
        }

        var records = await _context.VatTransactionRecords
            .AsNoTracking()
            .Where(r => r.InvoiceDate >= startDate && r.InvoiceDate <= endDate)
            .ToListAsync(cancellationToken);

        var inputs = records.Where(r => r.InvoiceType == VatInvoiceType.Input).Select(r => new VatRecordDto(
            r.InvoiceSeries,
            r.InvoiceNumber,
            r.InvoiceDate,
            r.PartnerTaxCode,
            r.PartnerName,
            r.Description,
            r.TaxableAmount,
            (int)r.VatRate,
            r.VatAmount)).ToList();

        var outputs = records.Where(r => r.InvoiceType == VatInvoiceType.Output).Select(r => new VatRecordDto(
            r.InvoiceSeries,
            r.InvoiceNumber,
            r.InvoiceDate,
            r.PartnerTaxCode,
            r.PartnerName,
            r.Description,
            r.TaxableAmount,
            (int)r.VatRate,
            r.VatAmount)).ToList();

        var totalTaxablePurchase = inputs.Sum(i => i.TaxableAmount);
        var totalVatPurchase = inputs.Sum(i => i.VatAmount);
        var totalTaxableSales = outputs.Sum(o => o.TaxableAmount);
        var totalVatSales = outputs.Sum(o => o.VatAmount);
        var payableVat = Math.Max(0, totalVatSales - totalVatPurchase);

        return new VatDeclarationDto(
            year,
            period,
            isQuarter,
            totalTaxablePurchase,
            totalVatPurchase,
            totalTaxableSales,
            totalVatSales,
            payableVat,
            inputs,
            outputs);
    }

    public async Task<string> GenerateTaxSubmissionXmlAsync(int year, int period, bool isQuarter, CancellationToken cancellationToken = default)
    {
        var declaration = await GetVatDeclarationAsync(year, period, isQuarter, cancellationToken);

        var doc = new XDocument(
            new XDeclaration("1.0", "UTF-8", "yes"),
            new XElement("HSoThueDTu",
                new XAttribute("xmlns", "http://kekhaithue.gdt.gov.vn/TKhaiThue"),
                new XElement("HSoKhaiThue",
                    new XElement("TTinChung",
                        new XElement("PBanXML", "2.1.0"),
                        new XElement("LoaiTKhai", "01/GTGT"),
                        new XElement("KyKKhai", isQuarter ? $"Q{period}/{year}" : $"{period:D2}/{year}"),
                        new XElement("NgayLapTKhai", DateTime.UtcNow.ToString("yyyy-MM-dd")),
                        new XElement("NguoiKy", "NGUYEN VAN A")
                    ),
                    new XElement("CTieuTKhaiChinh",
                        new XElement("ct21", 0),
                        new XElement("ct22", 0),
                        new XElement("ct23", declaration.TotalTaxablePurchase),
                        new XElement("ct24", declaration.TotalVatPurchase),
                        new XElement("ct25", declaration.TotalVatPurchase),
                        new XElement("ct26", 0),
                        new XElement("ct27", declaration.TotalTaxableSales),
                        new XElement("ct28", declaration.TotalVatSales),
                        new XElement("ct34", declaration.TotalTaxableSales),
                        new XElement("ct35", declaration.TotalVatSales),
                        new XElement("ct36", declaration.TotalVatSales),
                        new XElement("ct40", declaration.PayableVatAmount)
                    ),
                    new XElement("BangKeMuaVao",
                        declaration.InputInvoices.Select(inv =>
                            new XElement("HDonMuaVao",
                                new XElement("KHieuHDon", inv.InvoiceSeries),
                                new XElement("SoHDon", inv.InvoiceNumber),
                                new XElement("NgayHDon", inv.InvoiceDate.ToString("yyyy-MM-dd")),
                                new XElement("MSTNBan", inv.PartnerTaxCode),
                                new XElement("TenNBan", inv.PartnerName),
                                new XElement("DSXuatChuaThue", inv.TaxableAmount),
                                new XElement("ThueSuat", inv.VatRatePercent),
                                new XElement("TienThue", inv.VatAmount)
                            )
                        )
                    ),
                    new XElement("BangKeBanRa",
                        declaration.OutputInvoices.Select(inv =>
                            new XElement("HDonBanRa",
                                new XElement("KHieuHDon", inv.InvoiceSeries),
                                new XElement("SoHDon", inv.InvoiceNumber),
                                new XElement("NgayHDon", inv.InvoiceDate.ToString("yyyy-MM-dd")),
                                new XElement("MSTNMua", inv.PartnerTaxCode),
                                new XElement("TenNMua", inv.PartnerName),
                                new XElement("DSXuatChuaThue", inv.TaxableAmount),
                                new XElement("ThueSuat", inv.VatRatePercent),
                                new XElement("TienThue", inv.VatAmount)
                            )
                        )
                    )
                )
            )
        );

        using var memoryStream = new MemoryStream();
        using (var writer = new StreamWriter(memoryStream, Encoding.UTF8))
        {
            doc.Save(writer);
        }

        return Encoding.UTF8.GetString(memoryStream.ToArray());
    }
}

using Accounting.Application.Common.Interfaces;
using Accounting.Domain.Organization;
using Microsoft.EntityFrameworkCore;

namespace Accounting.Infrastructure.Persistence.Services;

public class ReportHeaderProvider : IReportHeaderProvider
{
    private readonly IAccountingDbContext _context;

    public ReportHeaderProvider(IAccountingDbContext context)
    {
        _context = context;
    }

    public async Task<ReportHeaderInfo> GetReportHeaderAsync(CancellationToken cancellationToken = default)
    {
        var company = await _context.CompanySettings.AsNoTracking().FirstOrDefaultAsync(cancellationToken);
        if (company == null)
        {
            return new ReportHeaderInfo(
                CompanyName: "DOANH NGHIỆP CHƯA CẬP NHẬT THÔNG TIN",
                TaxCode: "0000000000",
                Address: "Chưa thiết lập",
                LegalRepresentative: "Giám đốc",
                ChiefAccountant: "Kế toán trưởng",
                CurrencyCode: "VND");
        }

        return new ReportHeaderInfo(
            CompanyName: company.CompanyName,
            TaxCode: company.TaxCode,
            Address: company.Address,
            LegalRepresentative: company.LegalRepresentative,
            ChiefAccountant: company.ChiefAccountant,
            CurrencyCode: company.CurrencyCode);
    }
}

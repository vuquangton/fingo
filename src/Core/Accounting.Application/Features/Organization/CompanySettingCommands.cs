using Accounting.Application.Common.Interfaces;
using Accounting.Application.Common.Models;
using Accounting.Domain.MasterData.Common;
using Accounting.Domain.Organization;
using MediatR;
using Microsoft.EntityFrameworkCore;

namespace Accounting.Application.Features.Organization;

public record CompanySettingDto(
    Guid Id,
    string TaxCode,
    string CompanyName,
    string Address,
    string LegalRepresentative,
    string ChiefAccountant,
    string CurrencyCode,
    GoverningCircular GoverningCircular,
    int FiscalYearStartMonth,
    string? ContactPhone,
    string? ContactEmail,
    string? TaxOffice);

public record GetCompanySettingQuery : IRequest<Result<CompanySettingDto>>;

public record UpdateCompanySettingCommand(
    string TaxCode,
    string CompanyName,
    string Address,
    string LegalRepresentative,
    string ChiefAccountant,
    string CurrencyCode = "VND",
    GoverningCircular GoverningCircular = GoverningCircular.TT99_2025_BTC,
    int FiscalYearStartMonth = 1,
    string? ContactPhone = null,
    string? ContactEmail = null,
    string? TaxOffice = null) : IRequest<Result<CompanySettingDto>>;

public class CompanySettingHandlers :
    IRequestHandler<GetCompanySettingQuery, Result<CompanySettingDto>>,
    IRequestHandler<UpdateCompanySettingCommand, Result<CompanySettingDto>>
{
    private readonly IAccountingDbContext _context;

    public CompanySettingHandlers(IAccountingDbContext context)
    {
        _context = context;
    }

    public async Task<Result<CompanySettingDto>> Handle(GetCompanySettingQuery request, CancellationToken cancellationToken)
    {
        var setting = await _context.CompanySettings.AsNoTracking().FirstOrDefaultAsync(cancellationToken);
        if (setting == null)
        {
            return Result<CompanySettingDto>.Failure("Thông tin doanh nghiệp chưa được thiết lập.");
        }

        return Result<CompanySettingDto>.Success(Map(setting));
    }

    public async Task<Result<CompanySettingDto>> Handle(UpdateCompanySettingCommand request, CancellationToken cancellationToken)
    {
        var setting = await _context.CompanySettings.FirstOrDefaultAsync(cancellationToken);
        if (setting == null)
        {
            setting = new CompanySetting(
                Guid.NewGuid(),
                request.TaxCode,
                request.CompanyName,
                request.Address,
                request.LegalRepresentative,
                request.ChiefAccountant,
                request.CurrencyCode,
                request.GoverningCircular,
                request.FiscalYearStartMonth,
                request.ContactPhone,
                request.ContactEmail,
                request.TaxOffice);

            _context.AddEntity(setting);
        }
        else
        {
            setting.UpdateProfile(
                request.TaxCode,
                request.CompanyName,
                request.Address,
                request.LegalRepresentative,
                request.ChiefAccountant,
                request.ContactPhone,
                request.ContactEmail,
                request.TaxOffice);
        }

        await _context.SaveChangesAsync(cancellationToken);

        return Result<CompanySettingDto>.Success(Map(setting));
    }

    private static CompanySettingDto Map(CompanySetting s) =>
        new(
            s.Id,
            s.TaxCode,
            s.CompanyName,
            s.Address,
            s.LegalRepresentative,
            s.ChiefAccountant,
            s.CurrencyCode,
            s.GoverningCircular,
            s.FiscalYearStartMonth,
            s.ContactPhone,
            s.ContactEmail,
            s.TaxOffice);
}

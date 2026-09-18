using Accounting.Application.Common.Interfaces;
using Accounting.Application.Common.Models;
using Accounting.Application.Features.MasterData.Services;
using Accounting.Domain.MasterData.Common;
using Accounting.Domain.MasterData.Partners;
using FluentValidation;
using MediatR;
using Microsoft.EntityFrameworkCore;

namespace Accounting.Application.Features.MasterData;

public record PartnerBankAccountDto(
    Guid Id,
    string BankName,
    string BankCode,
    string AccountNumber,
    string AccountHolder,
    string? Branch,
    bool IsDefault);

public record PartnerDeliveryAddressDto(
    Guid Id,
    string AddressLine,
    string? Ward,
    string? District,
    string? City,
    string? ReceiverName,
    string? ReceiverPhone,
    bool IsDefault);

public record PartnerContactDto(
    Guid Id,
    string FullName,
    string? Position,
    string? Mobile,
    string? Email,
    bool IsPrimary);

public record BusinessPartnerDto(
    Guid Id,
    string Code,
    string Name,
    PartnerType PartnerType,
    LegalEntityType LegalEntityType,
    string? TaxCode,
    string? TaxAuthorityCode,
    string? Address,
    string? ContactEmail,
    string? ContactPhone,
    string? InvoiceReceivingEmail,
    decimal CreditLimit,
    int PaymentTermDays,
    decimal DiscountRate,
    decimal CurrentReceivableBalance,
    decimal CurrentPayableBalance,
    PartnerRiskTier RiskTier,
    bool IsActive,
    List<PartnerBankAccountDto> BankAccounts,
    List<PartnerDeliveryAddressDto> DeliveryAddresses,
    List<PartnerContactDto> Contacts);

public record CreateBusinessPartnerCommand(
    string PartnerCode,
    string Name,
    PartnerType PartnerType,
    string? TaxCode = null,
    string? Address = null,
    string? ContactEmail = null,
    string? ContactPhone = null,
    decimal CreditLimit = 0m,
    int PaymentTermDays = 0,
    LegalEntityType LegalEntityType = LegalEntityType.Corporate,
    string? InvoiceReceivingEmail = null,
    string? TaxAuthorityCode = null,
    decimal DiscountRate = 0m,
    string? PartnerGroupId = null,
    Guid? AssignedSalesRepId = null) : IRequest<Result<PartnerId>>;

public class CreateBusinessPartnerCommandValidator : AbstractValidator<CreateBusinessPartnerCommand>
{
    public CreateBusinessPartnerCommandValidator()
    {
        RuleFor(x => x.PartnerCode).NotEmpty().MaximumLength(50);
        RuleFor(x => x.Name).NotEmpty().MaximumLength(200);
        RuleFor(x => x.PartnerType).Must(t => t != PartnerType.None).WithMessage("At least one partner type role must be specified.");
        RuleFor(x => x.TaxCode)
            .Matches(@"^\d{10}(-\d{3})?$")
            .When(x => !string.IsNullOrWhiteSpace(x.TaxCode))
            .WithMessage("Tax code must follow Vietnamese statutory format (10 digits or 10 digits with 3 branch digits).");
        RuleFor(x => x.CreditLimit).GreaterThanOrEqualTo(0);
        RuleFor(x => x.PaymentTermDays).GreaterThanOrEqualTo(0);
        RuleFor(x => x.DiscountRate).InclusiveBetween(0, 100);
    }
}

public class CreateBusinessPartnerCommandHandler(IAccountingDbContext context) : IRequestHandler<CreateBusinessPartnerCommand, Result<PartnerId>>
{
    public async Task<Result<PartnerId>> Handle(CreateBusinessPartnerCommand request, CancellationToken cancellationToken)
    {
        var code = request.PartnerCode.Trim().ToUpperInvariant();
        var exists = await context.BusinessPartners.AnyAsync(p => p.PartnerCode == code, cancellationToken);
        if (exists)
            return Result<PartnerId>.Failure($"Business partner code '{code}' already exists.");

        var partner = new BusinessPartner(
            PartnerId.New(),
            code,
            request.Name,
            request.PartnerType,
            request.TaxCode,
            request.Address,
            request.ContactPhone,
            request.ContactEmail,
            request.CreditLimit,
            request.PaymentTermDays,
            request.LegalEntityType,
            request.InvoiceReceivingEmail,
            request.TaxAuthorityCode,
            request.DiscountRate,
            request.PartnerGroupId,
            request.AssignedSalesRepId);

        context.AddEntity(partner);
        await context.SaveChangesAsync(cancellationToken);

        return Result<PartnerId>.Success(partner.Id);
    }
}

public record GetBusinessPartnersQuery(PartnerType? TypeFilter = null, bool IncludeInactive = false) : IRequest<Result<IReadOnlyList<BusinessPartnerDto>>>;

public class GetBusinessPartnersQueryHandler(IAccountingDbContext context) : IRequestHandler<GetBusinessPartnersQuery, Result<IReadOnlyList<BusinessPartnerDto>>>
{
    public async Task<Result<IReadOnlyList<BusinessPartnerDto>>> Handle(GetBusinessPartnersQuery request, CancellationToken cancellationToken)
    {
        var query = context.BusinessPartners.AsNoTracking();
        if (!request.IncludeInactive)
            query = query.Where(p => p.IsActive);

        if (request.TypeFilter.HasValue && request.TypeFilter.Value != PartnerType.None)
        {
            var filter = request.TypeFilter.Value;
            query = query.Where(p => (p.PartnerType & filter) != 0);
        }

        var list = await query
            .OrderBy(p => p.PartnerCode)
            .Select(p => new BusinessPartnerDto(
                p.Id.Value,
                p.PartnerCode,
                p.Name,
                p.PartnerType,
                p.LegalEntityType,
                p.TaxCode,
                p.TaxAuthorityCode,
                p.Address,
                p.ContactEmail,
                p.ContactPhone,
                p.InvoiceReceivingEmail,
                p.CreditLimit,
                p.PaymentTermDays,
                p.DiscountRate,
                p.CurrentReceivableBalance,
                p.CurrentPayableBalance,
                p.RiskTier,
                p.IsActive,
                new List<PartnerBankAccountDto>(),
                new List<PartnerDeliveryAddressDto>(),
                new List<PartnerContactDto>()))
            .ToListAsync(cancellationToken);

        return Result<IReadOnlyList<BusinessPartnerDto>>.Success(list);
    }
}

// GDT/MST Lookup Query
public record LookupVietnamTaxCodeQuery(string TaxCode) : IRequest<Result<VietnamTaxCompanyInfo>>;

public class LookupVietnamTaxCodeQueryHandler(IVietnamTaxLookupService taxService) : IRequestHandler<LookupVietnamTaxCodeQuery, Result<VietnamTaxCompanyInfo>>
{
    public Task<Result<VietnamTaxCompanyInfo>> Handle(LookupVietnamTaxCodeQuery request, CancellationToken cancellationToken)
    {
        var info = taxService.LookupByTaxCode(request.TaxCode);
        if (info == null)
            return Task.FromResult(Result<VietnamTaxCompanyInfo>.Failure($"Không tìm thấy thông tin đăng ký thuế cho MST '{request.TaxCode}'."));

        return Task.FromResult(Result<VietnamTaxCompanyInfo>.Success(info));
    }
}

// Partner Deduplication Check Query
public record CheckPartnerDuplicateQuery(string Name, string? TaxCode = null, string? Phone = null, Guid? CurrentPartnerId = null) 
    : IRequest<Result<List<DuplicatePartnerMatch>>>;

public class CheckPartnerDuplicateQueryHandler(IPartnerDeduplicationEngine engine) : IRequestHandler<CheckPartnerDuplicateQuery, Result<List<DuplicatePartnerMatch>>>
{
    public async Task<Result<List<DuplicatePartnerMatch>>> Handle(CheckPartnerDuplicateQuery request, CancellationToken cancellationToken)
    {
        var matches = await engine.FindDuplicatesAsync(request.Name, request.TaxCode, request.Phone, request.CurrentPartnerId, cancellationToken);
        return Result<List<DuplicatePartnerMatch>>.Success(matches);
    }
}

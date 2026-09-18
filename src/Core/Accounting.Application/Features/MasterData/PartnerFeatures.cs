using Accounting.Application.Common.Interfaces;
using Accounting.Application.Common.Models;
using Accounting.Domain.MasterData.Common;
using Accounting.Domain.MasterData.Partners;
using FluentValidation;
using MediatR;
using Microsoft.EntityFrameworkCore;

namespace Accounting.Application.Features.MasterData;

public record BusinessPartnerDto(
    Guid Id,
    string Code,
    string Name,
    PartnerType PartnerType,
    string? TaxCode,
    string? Address,
    string? ContactEmail,
    string? ContactPhone,
    decimal CreditLimit,
    int PaymentTermDays,
    bool IsActive);

public record CreateBusinessPartnerCommand(
    string PartnerCode,
    string Name,
    PartnerType PartnerType,
    string? TaxCode = null,
    string? Address = null,
    string? ContactEmail = null,
    string? ContactPhone = null,
    decimal CreditLimit = 0m,
    int PaymentTermDays = 0) : IRequest<Result<PartnerId>>;

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
            request.PaymentTermDays);

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
                p.TaxCode,
                p.Address,
                p.ContactEmail,
                p.ContactPhone,
                p.CreditLimit,
                p.PaymentTermDays,
                p.IsActive))
            .ToListAsync(cancellationToken);

        return Result<IReadOnlyList<BusinessPartnerDto>>.Success(list);
    }
}

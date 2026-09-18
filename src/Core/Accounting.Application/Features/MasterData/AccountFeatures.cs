using Accounting.Application.Common.Interfaces;
using Accounting.Application.Common.Models;
using Accounting.Domain.MasterData.Accounts;
using Accounting.Domain.MasterData.Common;
using Accounting.Domain.MasterData.Compliance;
using FluentValidation;
using MediatR;
using Microsoft.EntityFrameworkCore;

namespace Accounting.Application.Features.MasterData;

public record AccountDto(
    string Code,
    string Name,
    AccountType AccountType,
    BalanceNature BalanceNature,
    string? ParentCode,
    int Level,
    bool IsParent,
    bool CanPost,
    bool RequiresPartner,
    bool RequiresWarehouse,
    bool RequiresCostCenter,
    bool RequiresProject,
    bool IsActive,
    GoverningCircular GoverningCircular,
    DateOnly EffectiveFrom,
    DateOnly? EffectiveTo);

public record CreateAccountCommand(
    string Code,
    string Name,
    AccountType AccountType,
    BalanceNature BalanceNature,
    string? ParentCode = null,
    bool RequiresPartner = false,
    bool RequiresWarehouse = false,
    bool RequiresCostCenter = false,
    bool RequiresProject = false,
    GoverningCircular GoverningCircular = GoverningCircular.TT99_2025_BTC,
    DateOnly? EffectiveFrom = null,
    DateOnly? EffectiveTo = null) : IRequest<Result<AccountId>>;

public class CreateAccountCommandValidator : AbstractValidator<CreateAccountCommand>
{
    public CreateAccountCommandValidator()
    {
        RuleFor(x => x.Code)
            .NotEmpty().WithMessage("Account code is required.")
            .Matches(@"^\d{3,20}$").WithMessage("Account code must be numeric and between 3 and 20 digits.")
            .Must(code => !BannedLegacyAccountCodes.IsBanned(code))
            .WithMessage(code => $"Account code '{code}' is abolished under Circular 99/2025/TT-BTC and cannot be used.");

        RuleFor(x => x.Name)
            .NotEmpty().WithMessage("Account name is required.")
            .MaximumLength(200).WithMessage("Account name cannot exceed 200 characters.");

        RuleFor(x => x)
            .Must(x =>
            {
                if (string.IsNullOrWhiteSpace(x.ParentCode)) return true;
                return x.Code.StartsWith(x.ParentCode.Trim(), StringComparison.OrdinalIgnoreCase);
            })
            .WithMessage("Child account code must strictly start with parent account code.");

        RuleFor(x => x)
            .Must(x => !x.EffectiveTo.HasValue || !x.EffectiveFrom.HasValue || x.EffectiveTo.Value >= x.EffectiveFrom.Value)
            .WithMessage("EffectiveTo date cannot be prior to EffectiveFrom date.");
    }
}

public class CreateAccountCommandHandler(IAccountingDbContext context) : IRequestHandler<CreateAccountCommand, Result<AccountId>>
{
    public async Task<Result<AccountId>> Handle(CreateAccountCommand request, CancellationToken cancellationToken)
    {
        var code = request.Code.Trim();
        var accountId = new AccountId(code);

        var exists = await context.MasterAccounts.AnyAsync(a => a.Id == accountId, cancellationToken);
        if (exists)
            return Result<AccountId>.Failure($"Account code '{code}' already exists.");

        AccountId? parentId = null;
        if (!string.IsNullOrWhiteSpace(request.ParentCode))
        {
            var pCode = request.ParentCode.Trim();
            parentId = new AccountId(pCode);
            var parent = await context.MasterAccounts.FirstOrDefaultAsync(a => a.Id == parentId.Value, cancellationToken);
            if (parent == null)
                return Result<AccountId>.Failure($"Parent account '{pCode}' not found.");

            parent.MarkAsParent();
        }

        var account = new Account(
            accountId,
            request.Name,
            request.AccountType,
            request.BalanceNature,
            parentId,
            isParent: false,
            requiresPartner: request.RequiresPartner,
            requiresWarehouse: request.RequiresWarehouse,
            requiresCostCenter: request.RequiresCostCenter,
            requiresProject: request.RequiresProject,
            governingCircular: request.GoverningCircular,
            effectiveFrom: request.EffectiveFrom,
            effectiveTo: request.EffectiveTo);

        context.AddEntity(account);
        await context.SaveChangesAsync(cancellationToken);

        return Result<AccountId>.Success(account.Id);
    }
}

public record UpdateAccountCommand(
    string Code,
    string Name,
    AccountType AccountType,
    BalanceNature BalanceNature,
    bool RequiresPartner,
    bool RequiresWarehouse,
    bool RequiresCostCenter,
    bool RequiresProject,
    bool IsActive,
    GoverningCircular? GoverningCircular = null,
    DateOnly? EffectiveFrom = null,
    DateOnly? EffectiveTo = null) : IRequest<Result<Unit>>;

public class UpdateAccountCommandValidator : AbstractValidator<UpdateAccountCommand>
{
    public UpdateAccountCommandValidator()
    {
        RuleFor(x => x.Code).NotEmpty();
        RuleFor(x => x.Name).NotEmpty().MaximumLength(200);

        RuleFor(x => x)
            .Must(x => !x.EffectiveTo.HasValue || !x.EffectiveFrom.HasValue || x.EffectiveTo.Value >= x.EffectiveFrom.Value)
            .WithMessage("EffectiveTo date cannot be prior to EffectiveFrom date.");
    }
}

public class UpdateAccountCommandHandler(IAccountingDbContext context) : IRequestHandler<UpdateAccountCommand, Result<Unit>>
{
    public async Task<Result<Unit>> Handle(UpdateAccountCommand request, CancellationToken cancellationToken)
    {
        var accountId = new AccountId(request.Code.Trim());
        var account = await context.MasterAccounts.FirstOrDefaultAsync(a => a.Id == accountId, cancellationToken);
        if (account == null)
            return Result<Unit>.Failure($"Account '{request.Code}' not found.");

        account.UpdateDetails(
            request.Name,
            request.AccountType,
            request.BalanceNature,
            request.RequiresPartner,
            request.RequiresWarehouse,
            request.RequiresCostCenter,
            request.RequiresProject,
            request.GoverningCircular ?? account.GoverningCircular,
            request.EffectiveFrom ?? account.EffectiveFrom,
            request.EffectiveTo ?? account.EffectiveTo);

        account.SetActive(request.IsActive);

        await context.SaveChangesAsync(cancellationToken);
        return Result<Unit>.Success(Unit.Value);
    }
}

public record GetAccountsTreeQuery(bool IncludeInactive = false) : IRequest<Result<IReadOnlyList<AccountDto>>>;

public class GetAccountsTreeQueryHandler(IAccountingDbContext context) : IRequestHandler<GetAccountsTreeQuery, Result<IReadOnlyList<AccountDto>>>
{
    public async Task<Result<IReadOnlyList<AccountDto>>> Handle(GetAccountsTreeQuery request, CancellationToken cancellationToken)
    {
        var query = context.MasterAccounts.AsNoTracking();
        if (!request.IncludeInactive)
            query = query.Where(a => a.IsActive);

        var list = await query
            .OrderBy(a => a.Id)
            .Select(a => new AccountDto(
                a.Id.Value,
                a.AccountName,
                a.AccountType,
                a.BalanceNature,
                a.ParentAccountId != null ? a.ParentAccountId.Value.Value : null,
                a.AccountLevel,
                a.IsParent,
                a.CanPost,
                a.RequiresPartner,
                a.RequiresWarehouse,
                a.RequiresCostCenter,
                a.RequiresProject,
                a.IsActive,
                a.GoverningCircular,
                a.EffectiveFrom,
                a.EffectiveTo))
            .ToListAsync(cancellationToken);

        return Result<IReadOnlyList<AccountDto>>.Success(list);
    }
}

public record GetPostingAccountsQuery : IRequest<Result<IReadOnlyList<AccountDto>>>;

public class GetPostingAccountsQueryHandler(IAccountingDbContext context) : IRequestHandler<GetPostingAccountsQuery, Result<IReadOnlyList<AccountDto>>>
{
    public async Task<Result<IReadOnlyList<AccountDto>>> Handle(GetPostingAccountsQuery request, CancellationToken cancellationToken)
    {
        var list = await context.MasterAccounts.AsNoTracking()
            .Where(a => !a.IsParent && a.IsActive)
            .OrderBy(a => a.Id)
            .Select(a => new AccountDto(
                a.Id.Value,
                a.AccountName,
                a.AccountType,
                a.BalanceNature,
                a.ParentAccountId != null ? a.ParentAccountId.Value.Value : null,
                a.AccountLevel,
                a.IsParent,
                a.CanPost,
                a.RequiresPartner,
                a.RequiresWarehouse,
                a.RequiresCostCenter,
                a.RequiresProject,
                a.IsActive,
                a.GoverningCircular,
                a.EffectiveFrom,
                a.EffectiveTo))
            .ToListAsync(cancellationToken);

        return Result<IReadOnlyList<AccountDto>>.Success(list);
    }
}

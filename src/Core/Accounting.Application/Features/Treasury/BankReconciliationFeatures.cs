using Accounting.Application.Common.Interfaces;
using Accounting.Application.Common.Models;
using Accounting.Domain.Entities.Treasury;
using MediatR;
using Microsoft.EntityFrameworkCore;

namespace Accounting.Application.Features.Treasury;

public record BankReconciliationDto(
    Guid Id,
    Guid BankAccountId,
    string BankAccountNumber,
    string BankName,
    DateTime StatementDate,
    decimal StatementBalance,
    decimal BookBalance,
    decimal Difference,
    bool IsReconciled,
    string? Notes);

public record GetBankReconciliationsQuery : IRequest<Result<List<BankReconciliationDto>>>;

public class GetBankReconciliationsQueryHandler(IAccountingDbContext context) : IRequestHandler<GetBankReconciliationsQuery, Result<List<BankReconciliationDto>>>
{
    public async Task<Result<List<BankReconciliationDto>>> Handle(GetBankReconciliationsQuery request, CancellationToken cancellationToken)
    {
        var reconList = await (
            from r in context.BankStatementReconciliations.AsNoTracking()
            join b in context.BankAccounts.AsNoTracking() on r.BankAccountId equals b.Id into bj
            from b in bj.DefaultIfEmpty()
            orderby r.StatementDate descending
            select new BankReconciliationDto(
                r.Id,
                r.BankAccountId,
                b != null ? b.AccountNumber : string.Empty,
                b != null ? b.BankName : string.Empty,
                r.StatementDate,
                r.StatementClosingBalance,
                r.BookClosingBalance,
                r.Difference,
                r.IsReconciled,
                r.Notes)
        ).ToListAsync(cancellationToken);

        return Result<List<BankReconciliationDto>>.Success(reconList);
    }
}

public record CreateBankReconciliationCommand(
    Guid BankAccountId,
    DateTime StatementDate,
    decimal StatementBalance,
    string? Notes = null) : IRequest<Result<Guid>>;

public class CreateBankReconciliationCommandHandler(IAccountingDbContext context) : IRequestHandler<CreateBankReconciliationCommand, Result<Guid>>
{
    public async Task<Result<Guid>> Handle(CreateBankReconciliationCommand request, CancellationToken cancellationToken)
    {
        var bank = await context.BankAccounts.FirstOrDefaultAsync(b => b.Id == request.BankAccountId, cancellationToken);
        if (bank == null)
            return Result<Guid>.Failure("Bank account not found.");

        var recon = new BankStatementReconciliation(
            request.BankAccountId,
            request.StatementDate,
            request.StatementBalance,
            bank.CurrentBalance,
            request.Notes);

        context.AddEntity(recon);
        await context.SaveChangesAsync(cancellationToken);

        return Result<Guid>.Success(recon.Id);
    }
}

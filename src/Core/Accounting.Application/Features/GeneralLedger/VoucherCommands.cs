using Accounting.Application.Common.Interfaces;
using Accounting.Application.Common.Models;
using Accounting.Domain.Entities.GeneralLedger;
using Accounting.Domain.Enums;
using FluentValidation;
using MediatR;
using Microsoft.EntityFrameworkCore;

namespace Accounting.Application.Features.GeneralLedger;

public record CreateVoucherLineDto(
    Guid DebitAccountId,
    Guid CreditAccountId,
    decimal Amount,
    string Description,
    decimal? ForeignAmount = null,
    Guid? PartnerId = null,
    PartnerType? PartnerType = null,
    Guid? CostCenterId = null,
    Guid? ProjectId = null);

public record CreateVoucherCommand(
    string VoucherNumber,
    DateTime VoucherDate,
    DateTime PostingDate,
    VoucherType VoucherType,
    string Description,
    List<CreateVoucherLineDto> Lines,
    string? ReferenceNumber = null,
    string Currency = "VND",
    decimal ExchangeRate = 1.0m) : IRequest<Result<Guid>>;

public class CreateVoucherCommandValidator : AbstractValidator<CreateVoucherCommand>
{
    public CreateVoucherCommandValidator()
    {
        RuleFor(x => x.VoucherNumber).NotEmpty().MaximumLength(50);
        RuleFor(x => x.Description).NotEmpty().MaximumLength(500);
        RuleFor(x => x.VoucherDate).NotEmpty();
        RuleFor(x => x.PostingDate).NotEmpty();
        RuleFor(x => x.Lines).NotEmpty().WithMessage("Voucher must contain at least one line item.");

        RuleForEach(x => x.Lines).ChildRules(line =>
        {
            line.RuleFor(l => l.DebitAccountId).NotEmpty();
            line.RuleFor(l => l.CreditAccountId).NotEmpty();
            line.RuleFor(l => l.Amount).GreaterThan(0).WithMessage("Line amount must be greater than zero.");
            line.RuleFor(l => l.DebitAccountId)
                .NotEqual(l => l.CreditAccountId)
                .WithMessage("Debit account and Credit account cannot be identical.");
        });

        RuleFor(x => x).Must(HaveBalancedLines)
            .WithMessage("Total Debit must equal Total Credit across all voucher lines.");
    }

    private bool HaveBalancedLines(CreateVoucherCommand cmd)
    {
        if (cmd.Lines == null || cmd.Lines.Count == 0) return false;
        // In double-entry lines, each line pairs Debit and Credit with the same amount
        var totalAmount = cmd.Lines.Sum(l => l.Amount);
        return totalAmount > 0;
    }
}

public class CreateVoucherCommandHandler : IRequestHandler<CreateVoucherCommand, Result<Guid>>
{
    private readonly IAccountingDbContext _context;
    private readonly ICurrentUserService _currentUser;
    private readonly IAuditLogService _auditLog;

    public CreateVoucherCommandHandler(IAccountingDbContext context, ICurrentUserService currentUser, IAuditLogService auditLog)
    {
        _context = context;
        _currentUser = currentUser;
        _auditLog = auditLog;
    }

    public async Task<Result<Guid>> Handle(CreateVoucherCommand request, CancellationToken cancellationToken)
    {
        var voucher = new Voucher(
            request.VoucherNumber,
            request.VoucherDate,
            request.PostingDate,
            request.VoucherType,
            request.Description,
            _currentUser.UserId ?? Guid.Empty,
            request.ReferenceNumber,
            request.Currency,
            request.ExchangeRate);

        foreach (var line in request.Lines)
        {
            voucher.AddLine(
                line.DebitAccountId,
                line.CreditAccountId,
                line.Amount,
                line.Description,
                line.ForeignAmount,
                line.PartnerId,
                line.PartnerType,
                line.CostCenterId,
                line.ProjectId);
        }

        voucher.ValidateBalance();

        _context.AddEntity(voucher);
        await _context.SaveChangesAsync(cancellationToken);

        await _auditLog.LogAsync(
            AuditAction.Create,
            nameof(Voucher),
            voucher.Id.ToString(),
            diffSummary: $"Created voucher {voucher.VoucherNumber} with amount {voucher.TotalDebitAmount:N2} {voucher.Currency}",
            cancellationToken: cancellationToken);

        return Result<Guid>.Success(voucher.Id);
    }
}

public record PostVoucherCommand(Guid VoucherId) : IRequest<Result>;

public class PostVoucherCommandHandler : IRequestHandler<PostVoucherCommand, Result>
{
    private readonly IVoucherPostingService _postingService;
    private readonly ICurrentUserService _currentUser;

    public PostVoucherCommandHandler(IVoucherPostingService postingService, ICurrentUserService currentUser)
    {
        _postingService = postingService;
        _currentUser = currentUser;
    }

    public async Task<Result> Handle(PostVoucherCommand request, CancellationToken cancellationToken)
    {
        try
        {
            var userId = _currentUser.UserId ?? Guid.Empty;
            await _postingService.PostVoucherAsync(request.VoucherId, userId, cancellationToken);
            return Result.Success();
        }
        catch (Exception ex)
        {
            return Result.Failure(ex.Message);
        }
    }
}

public record UnpostVoucherCommand(Guid VoucherId) : IRequest<Result>;

public class UnpostVoucherCommandHandler : IRequestHandler<UnpostVoucherCommand, Result>
{
    private readonly IVoucherPostingService _postingService;
    private readonly ICurrentUserService _currentUser;

    public UnpostVoucherCommandHandler(IVoucherPostingService postingService, ICurrentUserService currentUser)
    {
        _postingService = postingService;
        _currentUser = currentUser;
    }

    public async Task<Result> Handle(UnpostVoucherCommand request, CancellationToken cancellationToken)
    {
        try
        {
            var userId = _currentUser.UserId ?? Guid.Empty;
            await _postingService.UnpostVoucherAsync(request.VoucherId, userId, cancellationToken);
            return Result.Success();
        }
        catch (Exception ex)
        {
            return Result.Failure(ex.Message);
        }
    }
}

public record ReverseVoucherCommand(Guid VoucherId, string NewVoucherNumber, DateTime ReversalDate, string Reason) : IRequest<Result<Guid>>;

public class ReverseVoucherCommandHandler : IRequestHandler<ReverseVoucherCommand, Result<Guid>>
{
    private readonly IVoucherPostingService _postingService;
    private readonly ICurrentUserService _currentUser;

    public ReverseVoucherCommandHandler(IVoucherPostingService postingService, ICurrentUserService currentUser)
    {
        _postingService = postingService;
        _currentUser = currentUser;
    }

    public async Task<Result<Guid>> Handle(ReverseVoucherCommand request, CancellationToken cancellationToken)
    {
        try
        {
            var userId = _currentUser.UserId ?? Guid.Empty;
            var reversal = await _postingService.ReverseVoucherAsync(request.VoucherId, request.NewVoucherNumber, userId, request.ReversalDate, request.Reason, cancellationToken);
            return Result<Guid>.Success(reversal.Id);
        }
        catch (Exception ex)
        {
            return Result<Guid>.Failure(ex.Message);
        }
    }
}

using Accounting.Application.Common.Interfaces;
using Accounting.Application.Common.Models;
using Accounting.Domain.Entities.GeneralLedger;
using Accounting.Domain.Entities.Treasury;
using Accounting.Domain.Enums;
using FluentValidation;
using MediatR;
using Microsoft.EntityFrameworkCore;

namespace Accounting.Application.Features.Treasury;

public record CreateCashVoucherCommand(
    string VoucherNumber,
    VoucherType Type, // CashReceipt or CashPayment
    DateTime Date,
    string PersonName,
    string Reason,
    decimal Amount,
    Guid DebitAccountId,
    Guid CreditAccountId,
    string? Address = null,
    int AttachedDocCount = 0) : IRequest<Result<Guid>>;

public class CreateCashVoucherCommandValidator : AbstractValidator<CreateCashVoucherCommand>
{
    public CreateCashVoucherCommandValidator()
    {
        RuleFor(x => x.VoucherNumber).NotEmpty();
        RuleFor(x => x.PersonName).NotEmpty();
        RuleFor(x => x.Reason).NotEmpty();
        RuleFor(x => x.Amount).GreaterThan(0);
        RuleFor(x => x.DebitAccountId).NotEmpty();
        RuleFor(x => x.CreditAccountId).NotEmpty();
        RuleFor(x => x.Type).Must(t => t == VoucherType.CashReceipt || t == VoucherType.CashPayment)
            .WithMessage("Type must be CashReceipt (Phiếu thu) or CashPayment (Phiếu chi).");
    }
}

public class CreateCashVoucherCommandHandler : IRequestHandler<CreateCashVoucherCommand, Result<Guid>>
{
    private readonly IAccountingDbContext _context;
    private readonly ICurrentUserService _currentUser;
    private readonly IAuditLogService _auditLog;

    public CreateCashVoucherCommandHandler(IAccountingDbContext context, ICurrentUserService currentUser, IAuditLogService auditLog)
    {
        _context = context;
        _currentUser = currentUser;
        _auditLog = auditLog;
    }

    public async Task<Result<Guid>> Handle(CreateCashVoucherCommand request, CancellationToken cancellationToken)
    {
        var userId = _currentUser.UserId ?? Guid.Empty;

        // Auto-create GL Voucher
        var glVoucher = new Voucher(
            request.VoucherNumber,
            request.Date,
            request.Date,
            request.Type,
            request.Reason,
            userId);

        glVoucher.AddLine(request.DebitAccountId, request.CreditAccountId, request.Amount, request.Reason);
        glVoucher.ValidateBalance();
        _context.AddEntity(glVoucher);

        var cashVoucher = new CashVoucher(
            request.VoucherNumber,
            request.Type,
            request.Date,
            request.PersonName,
            request.Reason,
            request.Amount,
            glVoucher.Id,
            userId,
            request.Address,
            request.AttachedDocCount);

        _context.AddEntity(cashVoucher);
        await _context.SaveChangesAsync(cancellationToken);

        await _auditLog.LogAsync(
            AuditAction.Create,
            nameof(CashVoucher),
            cashVoucher.Id.ToString(),
            diffSummary: $"Created cash voucher {cashVoucher.VoucherNumber} ({cashVoucher.Type}) for {cashVoucher.Amount:N2} VND",
            cancellationToken: cancellationToken);

        return Result<Guid>.Success(cashVoucher.Id);
    }
}

public record CreatePaymentOrderCommand(
    string OrderNumber,
    DateTime OrderDate,
    Guid BankAccountId,
    string BeneficiaryName,
    string BeneficiaryAccountNumber,
    string BeneficiaryBankName,
    decimal Amount,
    string PaymentPurpose,
    Guid DebitAccountId,
    Guid CreditAccountId) : IRequest<Result<Guid>>;

public class CreatePaymentOrderCommandHandler : IRequestHandler<CreatePaymentOrderCommand, Result<Guid>>
{
    private readonly IAccountingDbContext _context;
    private readonly ICurrentUserService _currentUser;

    public CreatePaymentOrderCommandHandler(IAccountingDbContext context, ICurrentUserService currentUser)
    {
        _context = context;
        _currentUser = currentUser;
    }

    public async Task<Result<Guid>> Handle(CreatePaymentOrderCommand request, CancellationToken cancellationToken)
    {
        var userId = _currentUser.UserId ?? Guid.Empty;

        var glVoucher = new Voucher(
            request.OrderNumber,
            request.OrderDate,
            request.OrderDate,
            VoucherType.BankPayment,
            request.PaymentPurpose,
            userId);

        glVoucher.AddLine(request.DebitAccountId, request.CreditAccountId, request.Amount, request.PaymentPurpose);
        glVoucher.ValidateBalance();
        _context.AddEntity(glVoucher);

        var paymentOrder = new PaymentOrder(
            request.OrderNumber,
            request.OrderDate,
            request.BankAccountId,
            request.BeneficiaryName,
            request.BeneficiaryAccountNumber,
            request.BeneficiaryBankName,
            request.Amount,
            request.PaymentPurpose);

        paymentOrder.LinkVoucher(glVoucher.Id);
        _context.AddEntity(paymentOrder);

        await _context.SaveChangesAsync(cancellationToken);
        return Result<Guid>.Success(paymentOrder.Id);
    }
}

public record CashBookEntryDto(
    DateTime Date,
    string VoucherNumber,
    string PersonName,
    string Reason,
    decimal ReceiptAmount,
    decimal PaymentAmount,
    decimal RunningBalance);

public record GetCashBookQuery(DateTime FromDate, DateTime ToDate) : IRequest<Result<List<CashBookEntryDto>>>;

public class GetCashBookQueryHandler : IRequestHandler<GetCashBookQuery, Result<List<CashBookEntryDto>>>
{
    private readonly IAccountingDbContext _context;

    public GetCashBookQueryHandler(IAccountingDbContext context)
    {
        _context = context;
    }

    public async Task<Result<List<CashBookEntryDto>>> Handle(GetCashBookQuery request, CancellationToken cancellationToken)
    {
        var entries = await _context.CashVouchers
            .AsNoTracking()
            .Where(c => c.Date >= request.FromDate && c.Date <= request.ToDate)
            .OrderBy(c => c.Date)
            .ThenBy(c => c.VoucherNumber)
            .ToListAsync(cancellationToken);

        var result = new List<CashBookEntryDto>();
        decimal balance = 0m;

        foreach (var e in entries)
        {
            decimal receipt = e.Type == VoucherType.CashReceipt ? e.Amount : 0m;
            decimal payment = e.Type == VoucherType.CashPayment ? e.Amount : 0m;
            balance += (receipt - payment);

            result.Add(new CashBookEntryDto(
                e.Date,
                e.VoucherNumber,
                e.PersonName,
                e.Reason,
                receipt,
                payment,
                balance));
        }

        return Result<List<CashBookEntryDto>>.Success(result);
    }
}

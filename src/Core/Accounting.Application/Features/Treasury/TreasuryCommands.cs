using Accounting.Application.Common.Interfaces;
using Accounting.Application.Common.Models;
using Accounting.Domain.Common;
using Accounting.Domain.Ledger;
using Accounting.Domain.MasterData.Accounts;
using Accounting.Domain.MasterData.Common;
using Accounting.Domain.Treasury;
using MediatR;
using CurrencyCode = Accounting.Domain.MasterData.Common.CurrencyCode;
using Voucher = Accounting.Domain.Ledger.Voucher;
using VoucherType = Accounting.Domain.Ledger.VoucherType;

namespace Accounting.Application.Features.Treasury;

public record CreateCashReceiptCommand(
    string VoucherNumber,
    DateOnly TransactionDate,
    string PayerName,
    string Description,
    decimal Amount,
    string DebitAccountCode = "1111",
    string CreditAccountCode = "131",
    string Currency = "VND",
    decimal ExchangeRate = 1.0m,
    Guid? PartnerId = null,
    string PostedBy = "cashier") : IRequest<Result<CashVoucherId>>;

public class CreateCashReceiptCommandHandler(
    IAccountingDbContext context,
    IGlVoucherBridgeService glBridge) : IRequestHandler<CreateCashReceiptCommand, Result<CashVoucherId>>
{
    public async Task<Result<CashVoucherId>> Handle(CreateCashReceiptCommand request, CancellationToken cancellationToken)
    {
        var cashVoucherId = CashVoucherId.New();
        var currency = new CurrencyCode(request.Currency);
        var partnerId = request.PartnerId.HasValue ? new PartnerId(request.PartnerId.Value) : (PartnerId?)null;

        var transaction = new CashTransaction(
            cashVoucherId,
            request.VoucherNumber,
            TreasuryTransactionType.CashReceipt,
            request.TransactionDate,
            request.PayerName,
            request.Description,
            request.Amount,
            currency,
            request.ExchangeRate,
            partnerId);

        // Project Phase 3 GL Voucher (Debit Cash / Credit Counterparty)
        var glVoucher = new Voucher(
            VoucherId.New(),
            request.VoucherNumber,
            VoucherType.CashReceipt,
            request.TransactionDate,
            request.TransactionDate,
            request.Description,
            currency,
            request.ExchangeRate);

        glVoucher.AddLine(
            new AccountId(request.DebitAccountCode),
            LedgerEntryType.Debit,
            request.Amount,
            request.Description);

        glVoucher.AddLine(
            new AccountId(request.CreditAccountCode),
            LedgerEntryType.Credit,
            request.Amount,
            request.Description,
            partnerId: partnerId);

        context.AddEntity(transaction);

        var glResult = await glBridge.PostOperationalVoucherAsync(glVoucher, request.PostedBy, cancellationToken);
        if (!glResult.IsSuccess)
            return Result<CashVoucherId>.Failure(glResult.ErrorMessage ?? "Failed to post GL voucher.");

        transaction.LinkGeneralLedgerVoucher(glVoucher.Id);
        await context.SaveChangesAsync(cancellationToken);

        return Result<CashVoucherId>.Success(transaction.Id);
    }
}

public record CreateCashDisbursementCommand(
    string VoucherNumber,
    DateOnly TransactionDate,
    string ReceiverName,
    string Description,
    decimal Amount,
    string DebitAccountCode,
    string CreditAccountCode = "1111",
    string Currency = "VND",
    decimal ExchangeRate = 1.0m,
    Guid? PartnerId = null,
    string? CostCenterId = null,
    string PostedBy = "cashier") : IRequest<Result<CashVoucherId>>;

public class CreateCashDisbursementCommandHandler(
    IAccountingDbContext context,
    IGlVoucherBridgeService glBridge) : IRequestHandler<CreateCashDisbursementCommand, Result<CashVoucherId>>
{
    public async Task<Result<CashVoucherId>> Handle(CreateCashDisbursementCommand request, CancellationToken cancellationToken)
    {
        var cashVoucherId = CashVoucherId.New();
        var currency = new CurrencyCode(request.Currency);
        var partnerId = request.PartnerId.HasValue ? new PartnerId(request.PartnerId.Value) : (PartnerId?)null;
        var costCenterId = !string.IsNullOrWhiteSpace(request.CostCenterId) ? new CostCenterId(request.CostCenterId) : (CostCenterId?)null;

        var transaction = new CashTransaction(
            cashVoucherId,
            request.VoucherNumber,
            TreasuryTransactionType.CashDisbursement,
            request.TransactionDate,
            request.ReceiverName,
            request.Description,
            request.Amount,
            currency,
            request.ExchangeRate,
            partnerId);

        // Project Phase 3 GL Voucher (Debit Expense or Payable / Credit Cash)
        var glVoucher = new Voucher(
            VoucherId.New(),
            request.VoucherNumber,
            VoucherType.CashDisbursement,
            request.TransactionDate,
            request.TransactionDate,
            request.Description,
            currency,
            request.ExchangeRate);

        glVoucher.AddLine(
            new AccountId(request.DebitAccountCode),
            LedgerEntryType.Debit,
            request.Amount,
            request.Description,
            partnerId: partnerId,
            costCenterId: costCenterId);

        glVoucher.AddLine(
            new AccountId(request.CreditAccountCode),
            LedgerEntryType.Credit,
            request.Amount,
            request.Description);

        context.AddEntity(transaction);

        var glResult = await glBridge.PostOperationalVoucherAsync(glVoucher, request.PostedBy, cancellationToken);
        if (!glResult.IsSuccess)
            return Result<CashVoucherId>.Failure(glResult.ErrorMessage ?? "Failed to post GL voucher.");

        transaction.LinkGeneralLedgerVoucher(glVoucher.Id);
        await context.SaveChangesAsync(cancellationToken);

        return Result<CashVoucherId>.Success(transaction.Id);
    }
}

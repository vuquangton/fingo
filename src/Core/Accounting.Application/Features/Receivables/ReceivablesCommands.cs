using Accounting.Application.Common.Interfaces;
using Accounting.Application.Common.Models;
using Accounting.Domain.Common;
using Accounting.Domain.Exceptions;
using Accounting.Domain.Ledger;
using Accounting.Domain.MasterData.Accounts;
using Accounting.Domain.MasterData.Common;
using Accounting.Domain.Receivables;
using Accounting.Domain.Treasury;
using MediatR;
using Microsoft.EntityFrameworkCore;
using CurrencyCode = Accounting.Domain.MasterData.Common.CurrencyCode;
using Voucher = Accounting.Domain.Ledger.Voucher;
using VoucherType = Accounting.Domain.Ledger.VoucherType;

namespace Accounting.Application.Features.Receivables;

public record CreateSalesInvoiceLineDto(
    string AccountCode,
    decimal Quantity,
    decimal UnitPrice,
    decimal VatRate,
    string Description,
    Guid? InventoryItemId = null);

public record CreateSalesInvoiceCommand(
    string InvoiceNumber,
    DateOnly InvoiceDate,
    DateOnly DueDate,
    Guid CustomerId,
    string CustomerAccountCode = "131",
    string OutputVatAccountCode = "33311",
    IReadOnlyList<CreateSalesInvoiceLineDto>? Lines = null,
    string PostedBy = "ar_accountant") : IRequest<Result<SalesInvoiceId>>;

public class CreateSalesInvoiceCommandHandler(
    IAccountingDbContext context,
    IGlVoucherBridgeService glBridge) : IRequestHandler<CreateSalesInvoiceCommand, Result<SalesInvoiceId>>
{
    public async Task<Result<SalesInvoiceId>> Handle(CreateSalesInvoiceCommand request, CancellationToken cancellationToken)
    {
        if (request.Lines == null || request.Lines.Count == 0)
            return Result<SalesInvoiceId>.Failure("Sales invoice must contain at least one line item.");

        var invoiceId = SalesInvoiceId.New();
        var customerPartnerId = new PartnerId(request.CustomerId);

        var invoice = new SalesInvoice(
            invoiceId,
            request.InvoiceNumber,
            request.InvoiceDate,
            request.DueDate,
            customerPartnerId);

        foreach (var l in request.Lines)
        {
            invoice.AddLine(
                new AccountId(l.AccountCode),
                l.Quantity,
                l.UnitPrice,
                l.VatRate,
                l.Description,
                l.InventoryItemId.HasValue ? new InventoryItemId(l.InventoryItemId.Value) : null);
        }

        // Project Phase 3 GL Voucher with Statutory Output VAT Tax Splitting (TT 99/2025/TT-BTC)
        // Debit: Total Receivable from Customer (TK 131)
        // Credit: Revenue lines (TK 5111 / 5112)
        // Credit: Output VAT (TK 33311)
        var glVoucher = new Voucher(
            VoucherId.New(),
            $"HDB-{request.InvoiceNumber}",
            VoucherType.SalesInvoice,
            request.InvoiceDate,
            request.InvoiceDate,
            $"Sales invoice {request.InvoiceNumber}",
            new CurrencyCode("VND"),
            1.0m);

        // 1. Debit line for Customer Receivable (total amount)
        glVoucher.AddLine(
            new AccountId(request.CustomerAccountCode),
            LedgerEntryType.Debit,
            invoice.TotalAmount,
            $"Receivable for invoice {request.InvoiceNumber}",
            partnerId: customerPartnerId);

        // 2. Credit lines for item revenues
        foreach (var l in invoice.Lines)
        {
            glVoucher.AddLine(
                l.AccountId,
                LedgerEntryType.Credit,
                l.RevenueAmount,
                l.Description,
                partnerId: customerPartnerId);
        }

        // 3. Credit line for Output VAT (if vat amount > 0)
        if (invoice.VatAmount > 0)
        {
            glVoucher.AddLine(
                new AccountId(request.OutputVatAccountCode),
                LedgerEntryType.Credit,
                invoice.VatAmount,
                $"Output VAT for invoice {request.InvoiceNumber}",
                partnerId: customerPartnerId);
        }

        context.AddEntity(invoice);

        var glResult = await glBridge.PostOperationalVoucherAsync(glVoucher, request.PostedBy, cancellationToken);
        if (!glResult.IsSuccess)
            return Result<SalesInvoiceId>.Failure(glResult.ErrorMessage ?? "Failed to post GL voucher for sales invoice.");

        invoice.LinkGeneralLedgerVoucher(glVoucher.Id);
        await context.SaveChangesAsync(cancellationToken);

        return Result<SalesInvoiceId>.Success(invoice.Id);
    }
}

public record SettleCustomerReceiptCommand(
    SalesInvoiceId InvoiceId,
    string ReceiptVoucherNumber,
    DateOnly ReceiptDate,
    decimal Amount,
    string ReceiptAccountCode = "1111", // 1111 (Cash) or 1121 (Bank)
    string CustomerAccountCode = "131",
    string SettleBy = "treasury") : IRequest<Result<Unit>>;

public class SettleCustomerReceiptCommandHandler(
    IAccountingDbContext context,
    IGlVoucherBridgeService glBridge) : IRequestHandler<SettleCustomerReceiptCommand, Result<Unit>>
{
    public async Task<Result<Unit>> Handle(SettleCustomerReceiptCommand request, CancellationToken cancellationToken)
    {
        var invoice = await context.SubSalesInvoices
            .FirstOrDefaultAsync(i => i.Id == request.InvoiceId, cancellationToken);

        if (invoice == null)
            return Result<Unit>.Failure($"Sales invoice '{request.InvoiceId.Value}' not found.");

        // Apply receipt (Domain invariant strictly throws OverSettlementException if amount > remaining)
        invoice.ApplyReceipt(request.Amount);

        // Project Phase 3 GL Voucher for Settlement:
        // Debit: TK 1111 (Cash) or TK 1121 (Bank)
        // Credit: TK 131 (Customer)
        var glVoucher = new Voucher(
            VoucherId.New(),
            request.ReceiptVoucherNumber,
            VoucherType.CashReceipt,
            request.ReceiptDate,
            request.ReceiptDate,
            $"Payment receipt for sales invoice {invoice.InvoiceNumber}",
            new CurrencyCode("VND"),
            1.0m);

        glVoucher.AddLine(
            new AccountId(request.ReceiptAccountCode),
            LedgerEntryType.Debit,
            request.Amount,
            $"Receipt for invoice {invoice.InvoiceNumber}");

        glVoucher.AddLine(
            new AccountId(request.CustomerAccountCode),
            LedgerEntryType.Credit,
            request.Amount,
            $"Settle receivable for invoice {invoice.InvoiceNumber}",
            partnerId: invoice.CustomerId);

        // Also record Treasury CashTransaction
        var cashTx = new CashTransaction(
            CashVoucherId.New(),
            request.ReceiptVoucherNumber,
            TreasuryTransactionType.CashReceipt,
            request.ReceiptDate,
            $"Customer {invoice.CustomerId.Value}",
            $"Receipt for invoice {invoice.InvoiceNumber}",
            request.Amount,
            new CurrencyCode("VND"),
            1.0m,
            invoice.CustomerId);

        context.AddEntity(cashTx);

        var glResult = await glBridge.PostOperationalVoucherAsync(glVoucher, request.SettleBy, cancellationToken);
        if (!glResult.IsSuccess)
            return Result<Unit>.Failure(glResult.ErrorMessage ?? "Failed to post receipt GL voucher.");

        cashTx.LinkGeneralLedgerVoucher(glVoucher.Id);
        await context.SaveChangesAsync(cancellationToken);

        return Result<Unit>.Success(Unit.Value);
    }
}

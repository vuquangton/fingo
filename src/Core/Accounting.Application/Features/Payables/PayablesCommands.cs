using Accounting.Application.Common.Interfaces;
using Accounting.Application.Common.Models;
using Accounting.Domain.Common;
using Accounting.Domain.Exceptions;
using Accounting.Domain.Ledger;
using Accounting.Domain.MasterData.Accounts;
using Accounting.Domain.MasterData.Common;
using Accounting.Domain.Payables;
using Accounting.Domain.Treasury;
using MediatR;
using Microsoft.EntityFrameworkCore;
using CurrencyCode = Accounting.Domain.MasterData.Common.CurrencyCode;
using Voucher = Accounting.Domain.Ledger.Voucher;
using VoucherType = Accounting.Domain.Ledger.VoucherType;

namespace Accounting.Application.Features.Payables;

public record RegisterPurchaseInvoiceLineDto(
    string AccountCode,
    decimal Quantity,
    decimal UnitPrice,
    decimal VatRate,
    string Description,
    Guid? InventoryItemId = null,
    string? WarehouseId = null);

public record RegisterPurchaseInvoiceCommand(
    string InvoiceNumber,
    string InvoiceSeries,
    DateOnly InvoiceDate,
    DateOnly DueDate,
    Guid VendorId,
    string VendorAccountCode = "331",
    string InputVatAccountCode = "1331",
    string? DefaultWarehouseId = null,
    IReadOnlyList<RegisterPurchaseInvoiceLineDto>? Lines = null,
    string PostedBy = "ap_accountant") : IRequest<Result<PurchaseInvoiceId>>;

public class RegisterPurchaseInvoiceCommandHandler(
    IAccountingDbContext context,
    IGlVoucherBridgeService glBridge) : IRequestHandler<RegisterPurchaseInvoiceCommand, Result<PurchaseInvoiceId>>
{
    public async Task<Result<PurchaseInvoiceId>> Handle(RegisterPurchaseInvoiceCommand request, CancellationToken cancellationToken)
    {
        if (request.Lines == null || request.Lines.Count == 0)
            return Result<PurchaseInvoiceId>.Failure("Purchase invoice must contain at least one line item.");

        var invoiceId = PurchaseInvoiceId.New();
        var vendorPartnerId = new PartnerId(request.VendorId);

        var invoice = new PurchaseInvoice(
            invoiceId,
            request.InvoiceNumber,
            request.InvoiceSeries,
            request.InvoiceDate,
            request.DueDate,
            vendorPartnerId);

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

        // Project Phase 3 GL Voucher with Statutory VAT Tax Splitting (TT 99/2025/TT-BTC)
        // Debit: Line costs (e.g. TK 1561 / 642)
        // Debit: Input VAT (TK 1331)
        // Credit: Total Payable to Vendor (TK 331)
        var glVoucher = new Voucher(
            VoucherId.New(),
            $"HDM-{request.InvoiceNumber}",
            VoucherType.PurchaseInvoice,
            request.InvoiceDate,
            request.InvoiceDate,
            $"Purchase invoice {request.InvoiceSeries}-{request.InvoiceNumber}",
            new CurrencyCode("VND"),
            1.0m);

        // 1. Debit lines for item costs
        var invoiceLines = invoice.Lines.ToList();
        for (int i = 0; i < invoiceLines.Count; i++)
        {
            var l = invoiceLines[i];
            var lineDto = request.Lines[i];
            var whId = !string.IsNullOrWhiteSpace(lineDto.WarehouseId)
                ? lineDto.WarehouseId
                : request.DefaultWarehouseId;

            glVoucher.AddLine(
                l.AccountId,
                LedgerEntryType.Debit,
                l.Amount,
                l.Description,
                partnerId: vendorPartnerId,
                warehouseId: !string.IsNullOrWhiteSpace(whId) ? new WarehouseId(whId) : null);
        }

        // 2. Debit line for Input VAT (if vat amount > 0)
        if (invoice.VatAmount > 0)
        {
            glVoucher.AddLine(
                new AccountId(request.InputVatAccountCode),
                LedgerEntryType.Debit,
                invoice.VatAmount,
                $"Input VAT for invoice {request.InvoiceNumber}",
                partnerId: vendorPartnerId);
        }

        // 3. Credit line for Vendor Payable (total amount)
        glVoucher.AddLine(
            new AccountId(request.VendorAccountCode),
            LedgerEntryType.Credit,
            invoice.TotalAmount,
            $"Payable for invoice {request.InvoiceNumber}",
            partnerId: vendorPartnerId);

        context.AddEntity(invoice);

        var glResult = await glBridge.PostOperationalVoucherAsync(glVoucher, request.PostedBy, cancellationToken);
        if (!glResult.IsSuccess)
            return Result<PurchaseInvoiceId>.Failure(glResult.ErrorMessage ?? "Failed to post GL voucher for purchase invoice.");

        invoice.LinkGeneralLedgerVoucher(glVoucher.Id);
        await context.SaveChangesAsync(cancellationToken);

        return Result<PurchaseInvoiceId>.Success(invoice.Id);
    }
}

public record SettleVendorInvoiceCommand(
    PurchaseInvoiceId InvoiceId,
    string PaymentVoucherNumber,
    DateOnly PaymentDate,
    decimal Amount,
    string PaymentAccountCode = "1111", // 1111 (Cash) or 1121 (Bank)
    string VendorAccountCode = "331",
    string SettleBy = "treasury") : IRequest<Result<Unit>>;

public class SettleVendorInvoiceCommandHandler(
    IAccountingDbContext context,
    IGlVoucherBridgeService glBridge) : IRequestHandler<SettleVendorInvoiceCommand, Result<Unit>>
{
    public async Task<Result<Unit>> Handle(SettleVendorInvoiceCommand request, CancellationToken cancellationToken)
    {
        var invoice = await context.SubPurchaseInvoices
            .FirstOrDefaultAsync(i => i.Id == request.InvoiceId, cancellationToken);

        if (invoice == null)
            return Result<Unit>.Failure($"Purchase invoice '{request.InvoiceId.Value}' not found.");

        // Apply payment (Domain invariant strictly throws OverSettlementException if amount > remaining)
        invoice.ApplyPayment(request.Amount);

        // Project Phase 3 GL Voucher for Settlement:
        // Debit: TK 331 (Vendor)
        // Credit: TK 1111 (Cash) or TK 1121 (Bank)
        var glVoucher = new Voucher(
            VoucherId.New(),
            request.PaymentVoucherNumber,
            VoucherType.CashDisbursement,
            request.PaymentDate,
            request.PaymentDate,
            $"Payment settlement for purchase invoice {invoice.InvoiceNumber}",
            new CurrencyCode("VND"),
            1.0m);

        glVoucher.AddLine(
            new AccountId(request.VendorAccountCode),
            LedgerEntryType.Debit,
            request.Amount,
            $"Settle payable for invoice {invoice.InvoiceNumber}",
            partnerId: invoice.VendorId);

        glVoucher.AddLine(
            new AccountId(request.PaymentAccountCode),
            LedgerEntryType.Credit,
            request.Amount,
            $"Payment for invoice {invoice.InvoiceNumber}");

        // Also record Treasury CashTransaction
        var cashTx = new CashTransaction(
            CashVoucherId.New(),
            request.PaymentVoucherNumber,
            TreasuryTransactionType.CashDisbursement,
            request.PaymentDate,
            $"Vendor {invoice.VendorId.Value}",
            $"Payment for invoice {invoice.InvoiceNumber}",
            request.Amount,
            new CurrencyCode("VND"),
            1.0m,
            invoice.VendorId);

        context.AddEntity(cashTx);

        var glResult = await glBridge.PostOperationalVoucherAsync(glVoucher, request.SettleBy, cancellationToken);
        if (!glResult.IsSuccess)
            return Result<Unit>.Failure(glResult.ErrorMessage ?? "Failed to post settlement GL voucher.");

        cashTx.LinkGeneralLedgerVoucher(glVoucher.Id);
        await context.SaveChangesAsync(cancellationToken);

        return Result<Unit>.Success(Unit.Value);
    }
}

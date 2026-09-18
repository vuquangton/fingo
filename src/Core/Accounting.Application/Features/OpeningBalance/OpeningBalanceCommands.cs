using Accounting.Application.Common.Interfaces;
using Accounting.Application.Common.Models;
using Accounting.Domain.Common;
using Accounting.Domain.Ledger;
using Accounting.Domain.MasterData.Common;
using Accounting.Domain.OpeningBalance;
using MediatR;
using Microsoft.EntityFrameworkCore;

namespace Accounting.Application.Features.OpeningBalance;

public record OpeningBalanceItemDto(
    Guid Id,
    int FiscalYear,
    string AccountId,
    decimal DebitAmount,
    decimal CreditAmount,
    Guid? PartnerId,
    string? WarehouseId,
    Guid? InventoryItemId,
    decimal Quantity,
    decimal UnitPrice,
    string? BatchId,
    string Description,
    bool IsCommitted);

public record ValidationIssueDto(string Category, string Description, bool IsFatal);

public record OpeningBalanceValidationResultDto(
    int FiscalYear,
    decimal TotalDebit,
    decimal TotalCredit,
    decimal Difference,
    bool IsTrialBalanceBalanced,
    IReadOnlyList<ValidationIssueDto> Issues);

public record GetOpeningBalancesQuery(int FiscalYear) : IRequest<Result<List<OpeningBalanceItemDto>>>;

public record ValidateOpeningBalancesQuery(int FiscalYear) : IRequest<Result<OpeningBalanceValidationResultDto>>;

public record SaveOpeningBalanceEntryDto(
    string AccountId,
    decimal DebitAmount,
    decimal CreditAmount,
    Guid? PartnerId = null,
    string? WarehouseId = null,
    Guid? InventoryItemId = null,
    decimal Quantity = 0m,
    decimal UnitPrice = 0m,
    string? BatchId = null,
    string? Description = null);

public record BatchSaveOpeningBalancesCommand(
    int FiscalYear,
    IReadOnlyList<SaveOpeningBalanceEntryDto> Entries) : IRequest<Result<int>>;

public record CommitOpeningBalancesCommand(int FiscalYear, string CommittedBy) : IRequest<Result<Guid>>;

public record ImportOpeningBalancesFromExcelCommand(
    int FiscalYear,
    Stream ExcelStream) : IRequest<Result<OpeningBalanceValidationResultDto>>;

public class OpeningBalanceHandlers :
    IRequestHandler<GetOpeningBalancesQuery, Result<List<OpeningBalanceItemDto>>>,
    IRequestHandler<ValidateOpeningBalancesQuery, Result<OpeningBalanceValidationResultDto>>,
    IRequestHandler<BatchSaveOpeningBalancesCommand, Result<int>>,
    IRequestHandler<CommitOpeningBalancesCommand, Result<Guid>>,
    IRequestHandler<ImportOpeningBalancesFromExcelCommand, Result<OpeningBalanceValidationResultDto>>
{
    private readonly IAccountingDbContext _context;
    private readonly IOpeningBalanceExcelParser? _excelParser;

    public OpeningBalanceHandlers(IAccountingDbContext context, IOpeningBalanceExcelParser? excelParser = null)
    {
        _context = context;
        _excelParser = excelParser;
    }

    public async Task<Result<List<OpeningBalanceItemDto>>> Handle(GetOpeningBalancesQuery request, CancellationToken cancellationToken)
    {
        var entries = await _context.OpeningBalanceEntries
            .AsNoTracking()
            .Where(e => e.FiscalYear == request.FiscalYear)
            .OrderBy(e => (string)e.AccountId)
            .Select(e => new OpeningBalanceItemDto(
                e.Id,
                e.FiscalYear,
                e.AccountId.Value,
                e.DebitAmount,
                e.CreditAmount,
                e.PartnerId.HasValue ? e.PartnerId.Value.Value : null,
                e.WarehouseId.HasValue ? e.WarehouseId.Value.Value : null,
                e.InventoryItemId.HasValue ? e.InventoryItemId.Value.Value : null,
                e.Quantity,
                e.UnitPrice,
                e.BatchId,
                e.Description,
                e.IsCommitted))
            .ToListAsync(cancellationToken);

        return Result<List<OpeningBalanceItemDto>>.Success(entries);
    }

    public async Task<Result<OpeningBalanceValidationResultDto>> Handle(ValidateOpeningBalancesQuery request, CancellationToken cancellationToken)
    {
        var entries = await _context.OpeningBalanceEntries
            .AsNoTracking()
            .Where(e => e.FiscalYear == request.FiscalYear)
            .ToListAsync(cancellationToken);

        var issues = new List<ValidationIssueDto>();

        // 1. Double-entry trial balance check: Sum(Debit) == Sum(Credit)
        var totalDebit = entries.Sum(e => e.DebitAmount);
        var totalCredit = entries.Sum(e => e.CreditAmount);
        var diff = Math.Abs(totalDebit - totalCredit);
        var isBalanced = diff < 0.001m;

        if (!isBalanced)
        {
            issues.Add(new ValidationIssueDto(
                "TrialBalance",
                $"Tổng dư Nợ ({totalDebit:N0} đ) không bằng Tổng dư Có ({totalCredit:N0} đ). Chênh lệch: {diff:N0} đ.",
                IsFatal: true));
        }

        // 2. Sub-ledger check: TK 131 (AR) vs Customers
        var arEntries = entries.Where(e => e.AccountId.Value.StartsWith("131")).ToList();
        var arMissingPartner = arEntries.Where(e => !e.PartnerId.HasValue).ToList();
        if (arMissingPartner.Count != 0)
        {
            issues.Add(new ValidationIssueDto(
                "SubLedgerAR",
                $"Có {arMissingPartner.Count} dòng số dư TK 131 chưa gán Khách hàng (PartnerId).",
                IsFatal: true));
        }

        // 3. Sub-ledger check: TK 331 (AP) vs Vendors
        var apEntries = entries.Where(e => e.AccountId.Value.StartsWith("331")).ToList();
        var apMissingPartner = apEntries.Where(e => !e.PartnerId.HasValue).ToList();
        if (apMissingPartner.Count != 0)
        {
            issues.Add(new ValidationIssueDto(
                "SubLedgerAP",
                $"Có {apMissingPartner.Count} dòng số dư TK 331 chưa gán Nhà cung cấp (PartnerId).",
                IsFatal: true));
        }

        // 4. Sub-ledger check: TK 15x (Inventory) vs Warehouse & InventoryItem
        var invEntries = entries.Where(e => e.AccountId.Value.StartsWith("15")).ToList();
        foreach (var inv in invEntries)
        {
            if (!inv.WarehouseId.HasValue)
            {
                issues.Add(new ValidationIssueDto(
                    "SubLedgerInventory",
                    $"Số dư tài khoản kho {inv.AccountId.Value} thiếu Kho (WarehouseId).",
                    IsFatal: true));
            }
            if (!inv.InventoryItemId.HasValue)
            {
                issues.Add(new ValidationIssueDto(
                    "SubLedgerInventory",
                    $"Số dư tài khoản kho {inv.AccountId.Value} thiếu Vật tư hàng hóa (InventoryItemId).",
                    IsFatal: true));
            }
            if (inv.Quantity > 0 && inv.DebitAmount == 0 && inv.CreditAmount == 0)
            {
                issues.Add(new ValidationIssueDto(
                    "SubLedgerInventory",
                    $"Mặt hàng {inv.InventoryItemId?.Value} có số lượng ({inv.Quantity}) nhưng giá trị tiền = 0.",
                    IsFatal: false));
            }
        }

        return Result<OpeningBalanceValidationResultDto>.Success(new OpeningBalanceValidationResultDto(
            request.FiscalYear,
            totalDebit,
            totalCredit,
            diff,
            isBalanced,
            issues));
    }

    public async Task<Result<int>> Handle(BatchSaveOpeningBalancesCommand request, CancellationToken cancellationToken)
    {
        // Remove existing uncommitted entries for this fiscal year
        var existing = await _context.OpeningBalanceEntries
            .Where(e => e.FiscalYear == request.FiscalYear && !e.IsCommitted)
            .ToListAsync(cancellationToken);

        if (existing.Count != 0)
        {
            foreach (var ex in existing)
            {
                _context.RemoveEntity(ex);
            }
        }

        var newEntries = new List<OpeningBalanceEntry>();
        foreach (var dto in request.Entries)
        {
            var entry = new OpeningBalanceEntry(
                Guid.NewGuid(),
                request.FiscalYear,
                new AccountId(dto.AccountId),
                dto.DebitAmount,
                dto.CreditAmount,
                dto.PartnerId.HasValue ? new PartnerId(dto.PartnerId.Value) : null,
                !string.IsNullOrWhiteSpace(dto.WarehouseId) ? new WarehouseId(dto.WarehouseId) : null,
                dto.InventoryItemId.HasValue ? new InventoryItemId(dto.InventoryItemId.Value) : null,
                dto.Quantity,
                dto.UnitPrice,
                dto.BatchId,
                dto.Description);

            newEntries.Add(entry);
        }

        _context.AddRangeEntities(newEntries);
        await _context.SaveChangesAsync(cancellationToken);

        return Result<int>.Success(newEntries.Count);
    }

    public async Task<Result<Guid>> Handle(CommitOpeningBalancesCommand request, CancellationToken cancellationToken)
    {
        var entries = await _context.OpeningBalanceEntries
            .Where(e => e.FiscalYear == request.FiscalYear && !e.IsCommitted)
            .ToListAsync(cancellationToken);

        if (entries.Count == 0)
            return Result<Guid>.Failure($"Không có số dư đầu kỳ chưa khóa sổ cho năm tài chính {request.FiscalYear}.");

        // Validate double-entry
        var totalDebit = entries.Sum(e => e.DebitAmount);
        var totalCredit = entries.Sum(e => e.CreditAmount);
        if (Math.Abs(totalDebit - totalCredit) > 0.001m)
        {
            return Result<Guid>.Failure($"Không thể khóa sổ: Tổng Nợ ({totalDebit:N0}) khác Tổng Có ({totalCredit:N0}).");
        }

        // Post opening entries to GL as a special opening voucher
        var voucherId = Guid.NewGuid();
        var voucherNumber = $"OPN-{request.FiscalYear}";
        var postingDate = new DateOnly(request.FiscalYear, 1, 1);

        // Check if an existing opening voucher exists
        var existingVoucher = await _context.GlVouchers.FirstOrDefaultAsync(v => v.VoucherNumber == voucherNumber, cancellationToken);
        if (existingVoucher != null)
        {
            _context.RemoveEntity(existingVoucher);
        }

        var glVoucher = new Voucher(
            new VoucherId(voucherId),
            voucherNumber,
            VoucherType.GeneralJournal,
            postingDate,
            postingDate,
            $"Số dư ban đầu năm tài chính {request.FiscalYear}",
            Domain.MasterData.Common.CurrencyCode.Vnd,
            1.0m);

        foreach (var entry in entries)
        {
            if (entry.DebitAmount > 0)
            {
                glVoucher.AddLine(
                    entry.AccountId,
                    LedgerEntryType.Debit,
                    entry.DebitAmount,
                    $"Số dư Nợ đầu kỳ: {entry.Description}",
                    entry.PartnerId,
                    entry.WarehouseId);
            }

            if (entry.CreditAmount > 0)
            {
                glVoucher.AddLine(
                    entry.AccountId,
                    LedgerEntryType.Credit,
                    entry.CreditAmount,
                    $"Số dư Có đầu kỳ: {entry.Description}",
                    entry.PartnerId,
                    entry.WarehouseId);
            }

            entry.MarkCommitted(request.CommittedBy);
        }

        glVoucher.Post(request.CommittedBy);

        _context.AddEntity(glVoucher);
        await _context.SaveChangesAsync(cancellationToken);

        return Result<Guid>.Success(voucherId);
    }

    public async Task<Result<OpeningBalanceValidationResultDto>> Handle(ImportOpeningBalancesFromExcelCommand request, CancellationToken cancellationToken)
    {
        if (_excelParser == null)
            return Result<OpeningBalanceValidationResultDto>.Failure("Trình phân tích Excel (IOpeningBalanceExcelParser) chưa được đăng ký.");

        var parseRes = _excelParser.ParseOpeningBalances(request.ExcelStream, request.FiscalYear);
        if (!parseRes.IsSuccess || parseRes.Value == null)
            return Result<OpeningBalanceValidationResultDto>.Failure(parseRes.ErrorMessage ?? "Lỗi phân tích tệp Excel.");

        // Atomic batch save (staging)
        var saveRes = await Handle(new BatchSaveOpeningBalancesCommand(request.FiscalYear, parseRes.Value), cancellationToken);
        if (!saveRes.IsSuccess)
            return Result<OpeningBalanceValidationResultDto>.Failure(saveRes.ErrorMessage ?? "Lỗi lưu số dư từ Excel.");

        // Run validation matrix
        return await Handle(new ValidateOpeningBalancesQuery(request.FiscalYear), cancellationToken);
    }
}

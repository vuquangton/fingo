using Accounting.Application.Common.Interfaces;
using Accounting.Application.Common.Models;
using MediatR;
using Microsoft.EntityFrameworkCore;

namespace Accounting.Application.Features.Books;

public class StatutoryBookQueryHandlers :
    IRequestHandler<GetS03aJournalBookQuery, Result<S03aJournalBookDto>>,
    IRequestHandler<GetS03bGeneralLedgerBookQuery, Result<S03bGeneralLedgerBookDto>>,
    IRequestHandler<GetDetailedPartnerBookQuery, Result<DetailedPartnerBookDto>>,
    IRequestHandler<GetDetailedInventoryBookQuery, Result<DetailedInventoryBookDto>>
{
    private readonly IAccountingDbContext _context;
    private readonly IReportHeaderProvider? _reportHeaderProvider;

    public StatutoryBookQueryHandlers(IAccountingDbContext context, IReportHeaderProvider? reportHeaderProvider = null)
    {
        _context = context;
        _reportHeaderProvider = reportHeaderProvider;
    }

    public async Task<Result<S03aJournalBookDto>> Handle(GetS03aJournalBookQuery request, CancellationToken cancellationToken)
    {
        ReportHeaderInfo? header = null;
        if (_reportHeaderProvider != null)
        {
            header = await _reportHeaderProvider.GetReportHeaderAsync(cancellationToken);
        }
        var vouchers = await _context.GlVouchers
            .AsNoTracking()
            .Include(v => v.Lines)
            .Where(v => v.Status == Domain.Ledger.VoucherStatus.Posted &&
                        v.PostingDate >= request.FromDate &&
                        v.PostingDate <= request.ToDate)
            .OrderBy(v => v.PostingDate)
            .ThenBy(v => v.VoucherNumber)
            .ToListAsync(cancellationToken);

        var lines = new List<S03aJournalLineDto>();
        decimal total = 0m;

        foreach (var v in vouchers)
        {
            var debitLines = v.Lines.Where(l => l.EntryType == Domain.Ledger.LedgerEntryType.Debit).ToList();
            var creditLines = v.Lines.Where(l => l.EntryType == Domain.Ledger.LedgerEntryType.Credit).ToList();

            for (int i = 0; i < Math.Max(debitLines.Count, creditLines.Count); i++)
            {
                var d = i < debitLines.Count ? debitLines[i] : debitLines.LastOrDefault();
                var c = i < creditLines.Count ? creditLines[i] : creditLines.LastOrDefault();
                if (d == null || c == null) continue;

                var amt = Math.Min(d.AmountBase, c.AmountBase);
                lines.Add(new S03aJournalLineDto(
                    v.PostingDate,
                    v.VoucherDate,
                    v.VoucherNumber,
                    !string.IsNullOrWhiteSpace(d.Description) ? d.Description : v.Description,
                    d.AccountId.Value,
                    c.AccountId.Value,
                    amt
                ));
                total += amt;
            }
        }

        return Result<S03aJournalBookDto>.Success(new S03aJournalBookDto(
            "SO NHAT KY CHUNG (MAU S03a-DN)",
            request.FromDate,
            request.ToDate,
            lines,
            total,
            header
        ));
    }

    public async Task<Result<S03bGeneralLedgerBookDto>> Handle(GetS03bGeneralLedgerBookQuery request, CancellationToken cancellationToken)
    {
        var targetAcc = request.AccountNumber.Trim();
        var targetAccountId = new Domain.MasterData.Common.AccountId(targetAcc);
        var targetPattern = targetAcc + "%";

        var account = await _context.MasterAccounts.AsNoTracking()
            .FirstOrDefaultAsync(a => a.Id == targetAccountId, cancellationToken);

        var accName = account?.AccountName ?? $"Tai khoan {targetAcc}";

        var openingEntries = await _context.GeneralLedgerEntries.AsNoTracking()
            .Where(e => EF.Functions.Like((string)e.AccountId, targetPattern) && e.PostingDate < request.FromDate)
            .GroupBy(_ => 1)
            .Select(g => new
            {
                Debit = g.Sum(x => x.DebitAmount),
                Credit = g.Sum(x => x.CreditAmount)
            })
            .FirstOrDefaultAsync(cancellationToken);

        decimal openDebit = 0m;
        decimal openCredit = 0m;
        if (openingEntries != null)
        {
            var net = openingEntries.Debit - openingEntries.Credit;
            if (net > 0) openDebit = net;
            else openCredit = -net;
        }

        var periodVouchers = await _context.GlVouchers.AsNoTracking()
            .Include(v => v.Lines)
            .Where(v => v.Status == Domain.Ledger.VoucherStatus.Posted &&
                        v.PostingDate >= request.FromDate &&
                        v.PostingDate <= request.ToDate &&
                        v.Lines.Any(l => EF.Functions.Like((string)l.AccountId, targetPattern)))
            .OrderBy(v => v.PostingDate)
            .ThenBy(v => v.VoucherNumber)
            .ToListAsync(cancellationToken);

        var lines = new List<S03bGeneralLedgerLineDto>();
        decimal runningBalance = openDebit - openCredit;
        decimal totalDebit = 0m;
        decimal totalCredit = 0m;

        foreach (var v in periodVouchers)
        {
            var matchingLines = v.Lines.Where(l => l.AccountId.Value.StartsWith(targetAcc)).ToList();
            foreach (var match in matchingLines)
            {
                var oppositeType = match.EntryType == Domain.Ledger.LedgerEntryType.Debit
                    ? Domain.Ledger.LedgerEntryType.Credit
                    : Domain.Ledger.LedgerEntryType.Debit;

                var corresp = v.Lines.FirstOrDefault(l => l.EntryType == oppositeType)?.AccountId.Value ?? string.Empty;

                decimal dr = match.EntryType == Domain.Ledger.LedgerEntryType.Debit ? match.AmountBase : 0m;
                decimal cr = match.EntryType == Domain.Ledger.LedgerEntryType.Credit ? match.AmountBase : 0m;

                totalDebit += dr;
                totalCredit += cr;
                runningBalance += (dr - cr);

                lines.Add(new S03bGeneralLedgerLineDto(
                    v.PostingDate,
                    v.VoucherNumber,
                    v.VoucherDate,
                    match.Description,
                    corresp,
                    dr,
                    cr,
                    runningBalance
                ));
            }
        }

        decimal closeDebit = runningBalance > 0 ? runningBalance : 0m;
        decimal closeCredit = runningBalance < 0 ? -runningBalance : 0m;

        ReportHeaderInfo? header = null;
        if (_reportHeaderProvider != null)
        {
            header = await _reportHeaderProvider.GetReportHeaderAsync(cancellationToken);
        }

        return Result<S03bGeneralLedgerBookDto>.Success(new S03bGeneralLedgerBookDto(
            targetAcc,
            accName,
            request.FromDate,
            request.ToDate,
            openDebit,
            openCredit,
            lines,
            totalDebit,
            totalCredit,
            closeDebit,
            closeCredit,
            header
        ));
    }

    public async Task<Result<DetailedPartnerBookDto>> Handle(GetDetailedPartnerBookQuery request, CancellationToken cancellationToken)
    {
        var targetAcc = request.AccountNumber.Trim();
        var partnerIdObj = new Domain.MasterData.Common.PartnerId(request.PartnerId);

        var partner = await _context.BusinessPartners.AsNoTracking()
            .FirstOrDefaultAsync(p => p.Id == partnerIdObj, cancellationToken);

        var partnerCode = partner?.PartnerCode ?? request.PartnerId.ToString();
        var partnerName = partner?.Name ?? "Doi tac";

        var targetPattern = targetAcc + "%";
        var openEntries = await _context.GeneralLedgerEntries.AsNoTracking()
            .Where(e => EF.Functions.Like((string)e.AccountId, targetPattern) &&
                        e.PartnerId == partnerIdObj &&
                        e.PostingDate < request.FromDate)
            .GroupBy(_ => 1)
            .Select(g => new { Debit = g.Sum(x => x.DebitAmount), Credit = g.Sum(x => x.CreditAmount) })
            .FirstOrDefaultAsync(cancellationToken);

        decimal openBal = openEntries != null ? (openEntries.Debit - openEntries.Credit) : 0m;

        var vouchers = await _context.GlVouchers.AsNoTracking()
            .Include(v => v.Lines)
            .Where(v => v.Status == Domain.Ledger.VoucherStatus.Posted &&
                        v.PostingDate >= request.FromDate &&
                        v.PostingDate <= request.ToDate &&
                        v.Lines.Any(l => EF.Functions.Like((string)l.AccountId, targetPattern) && l.PartnerId == partnerIdObj))
            .OrderBy(v => v.PostingDate)
            .ThenBy(v => v.VoucherNumber)
            .ToListAsync(cancellationToken);

        var lines = new List<DetailedPartnerBookLineDto>();
        decimal running = openBal;
        decimal totalDr = 0m;
        decimal totalCr = 0m;

        foreach (var v in vouchers)
        {
            var pLines = v.Lines.Where(l => l.AccountId.Value.StartsWith(targetAcc) && l.PartnerId == partnerIdObj).ToList();
            foreach (var l in pLines)
            {
                var oppositeType = l.EntryType == Domain.Ledger.LedgerEntryType.Debit
                    ? Domain.Ledger.LedgerEntryType.Credit
                    : Domain.Ledger.LedgerEntryType.Debit;

                var corresp = v.Lines.FirstOrDefault(x => x.EntryType == oppositeType)?.AccountId.Value ?? string.Empty;
                decimal dr = l.EntryType == Domain.Ledger.LedgerEntryType.Debit ? l.AmountBase : 0m;
                decimal cr = l.EntryType == Domain.Ledger.LedgerEntryType.Credit ? l.AmountBase : 0m;

                totalDr += dr;
                totalCr += cr;
                running += (dr - cr);

                lines.Add(new DetailedPartnerBookLineDto(
                    v.PostingDate,
                    v.VoucherNumber,
                    l.Description,
                    corresp,
                    dr,
                    cr,
                    running
                ));
            }
        }

        return Result<DetailedPartnerBookDto>.Success(new DetailedPartnerBookDto(
            targetAcc,
            request.PartnerId,
            partnerCode,
            partnerName,
            request.FromDate,
            request.ToDate,
            openBal,
            lines,
            totalDr,
            totalCr,
            running
        ));
    }

    public async Task<Result<DetailedInventoryBookDto>> Handle(GetDetailedInventoryBookQuery request, CancellationToken cancellationToken)
    {
        var whId = new Domain.MasterData.Common.WarehouseId(request.WarehouseId);
        var itemId = new Domain.MasterData.Common.InventoryItemId(request.InventoryItemId);

        var wh = await _context.MasterWarehouses.AsNoTracking().FirstOrDefaultAsync(w => w.Id == whId, cancellationToken);
        var item = await _context.InventoryItems.AsNoTracking().FirstOrDefaultAsync(i => i.Id == itemId, cancellationToken);

        var whName = wh?.WarehouseName ?? "Kho chinh";
        var itemCode = item?.ItemCode ?? request.InventoryItemId.ToString();
        var itemName = item?.ItemName ?? "Hang hoa / Vat tu";
        var uom = item?.BaseUomId.Value.ToString() ?? "Cai";

        var movements = await _context.SubWarehouseVouchers.AsNoTracking()
            .Include(w => w.Lines)
            .Where(w => w.WarehouseId == whId &&
                        w.PostingDate >= request.FromDate &&
                        w.PostingDate <= request.ToDate &&
                        w.Lines.Any(l => l.InventoryItemId == itemId))
            .OrderBy(w => w.PostingDate)
            .ThenBy(w => w.VoucherNumber)
            .ToListAsync(cancellationToken);

        var lines = new List<DetailedInventoryBookLineDto>();
        decimal curQty = 0m;
        decimal curAmt = 0m;
        decimal totalInQty = 0m;
        decimal totalInAmt = 0m;
        decimal totalOutQty = 0m;
        decimal totalOutAmt = 0m;

        foreach (var m in movements)
        {
            var matchLines = m.Lines.Where(l => l.InventoryItemId == itemId).ToList();
            foreach (var l in matchLines)
            {
                bool isIn = m.VoucherType == Domain.WarehouseOperations.WarehouseVoucherType.InwardPurchase ||
                            m.VoucherType == Domain.WarehouseOperations.WarehouseVoucherType.InwardProduction ||
                            m.VoucherType == Domain.WarehouseOperations.WarehouseVoucherType.InwardTransfer;

                decimal inQ = isIn ? l.Quantity : 0m;
                decimal inA = isIn ? l.TotalAmount : 0m;
                decimal outQ = !isIn ? l.Quantity : 0m;
                decimal outA = !isIn ? l.TotalAmount : 0m;

                totalInQty += inQ;
                totalInAmt += inA;
                totalOutQty += outQ;
                totalOutAmt += outA;

                curQty += (inQ - outQ);
                curAmt += (inA - outA);

                lines.Add(new DetailedInventoryBookLineDto(
                    m.PostingDate,
                    m.VoucherNumber,
                    m.Description,
                    isIn ? l.CreditAccountId.Value : l.DebitAccountId.Value,
                    inQ,
                    inA,
                    outQ,
                    outA,
                    curQty,
                    curAmt
                ));
            }
        }

        return Result<DetailedInventoryBookDto>.Success(new DetailedInventoryBookDto(
            request.WarehouseId,
            whName,
            request.InventoryItemId,
            itemCode,
            itemName,
            uom,
            request.FromDate,
            request.ToDate,
            0m,
            0m,
            lines,
            totalInQty,
            totalInAmt,
            totalOutQty,
            totalOutAmt,
            curQty,
            curAmt
        ));
    }
}
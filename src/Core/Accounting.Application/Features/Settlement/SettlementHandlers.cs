using Accounting.Application.Common.Interfaces;
using Accounting.Application.Common.Models;
using Accounting.Domain.Common;
using Accounting.Domain.Ledger;
using Accounting.Domain.MasterData.Common;
using Accounting.Domain.Settlement;
using MediatR;
using Microsoft.EntityFrameworkCore;

namespace Accounting.Application.Features.Settlement;

public class SettlementHandlers :
    IRequestHandler<AllocatePaymentCommand, Result<Guid>>,
    IRequestHandler<ReverseAllocationCommand, Result<Unit>>,
    IRequestHandler<GetArOutstandingInvoicesQuery, Result<IReadOnlyList<OutstandingInvoiceDto>>>,
    IRequestHandler<GetApOutstandingInvoicesQuery, Result<IReadOnlyList<OutstandingInvoiceDto>>>,
    IRequestHandler<GetArAgingReportQuery, Result<AgingReportDto>>,
    IRequestHandler<GetApAgingReportQuery, Result<AgingReportDto>>
{
    private readonly IAccountingDbContext _context;

    public SettlementHandlers(IAccountingDbContext context)
    {
        _context = context;
    }

    public async Task<Result<Guid>> Handle(AllocatePaymentCommand request, CancellationToken cancellationToken)
    {
        if (request.Amount <= 0)
            return Result<Guid>.Failure("So tien phan bo phai lon hon 0.");

        var allocId = InvoicePaymentAllocationId.New();
        var alloc = new InvoicePaymentAllocation(
            allocId,
            request.InvoiceId,
            request.InvoiceType,
            new VoucherId(request.PaymentVoucherId),
            new PartnerId(request.PartnerId),
            request.AllocationDate,
            request.Amount,
            request.Description);

        if (request.InvoiceType == AllocationInvoiceType.SalesInvoice)
        {
            var invId = new SalesInvoiceId(request.InvoiceId);
            var inv = await _context.SubSalesInvoices.FirstOrDefaultAsync(i => i.Id == invId, cancellationToken);
            if (inv == null)
                return Result<Guid>.Failure($"Khong tim thay hoa don ban hang {request.InvoiceId}");

            inv.ApplyReceipt(request.Amount);
        }
        else
        {
            var invId = new PurchaseInvoiceId(request.InvoiceId);
            var inv = await _context.SubPurchaseInvoices.FirstOrDefaultAsync(i => i.Id == invId, cancellationToken);
            if (inv == null)
                return Result<Guid>.Failure($"Khong tim thay hoa don mua hang {request.InvoiceId}");

            inv.ApplyPayment(request.Amount);
        }

        _context.AddEntity(alloc);
        await _context.SaveChangesAsync(cancellationToken);
        return Result<Guid>.Success(allocId.Value);
    }

    public async Task<Result<Unit>> Handle(ReverseAllocationCommand request, CancellationToken cancellationToken)
    {
        var targetId = new InvoicePaymentAllocationId(request.AllocationId);
        var alloc = await _context.SubInvoicePaymentAllocations.FirstOrDefaultAsync(a => a.Id == targetId, cancellationToken);
        if (alloc == null)
            return Result<Unit>.Failure("Khong tim thay ban ghi phan bo can huy.");

        if (alloc.IsReversed)
            return Result<Unit>.Failure("Ban ghi phan bo da duoc huy truoc do.");

        alloc.Reverse(request.ReversedBy);

        if (alloc.InvoiceType == AllocationInvoiceType.SalesInvoice)
        {
            var invId = new SalesInvoiceId(alloc.InvoiceId);
            var inv = await _context.SubSalesInvoices.FirstOrDefaultAsync(i => i.Id == invId, cancellationToken);
            if (inv != null)
            {
                inv.ReverseReceipt(alloc.AllocatedAmount);
            }
        }
        else
        {
            var invId = new PurchaseInvoiceId(alloc.InvoiceId);
            var inv = await _context.SubPurchaseInvoices.FirstOrDefaultAsync(i => i.Id == invId, cancellationToken);
            if (inv != null)
            {
                inv.ReversePayment(alloc.AllocatedAmount);
            }
        }

        await _context.SaveChangesAsync(cancellationToken);
        return Result<Unit>.Success(Unit.Value);
    }

    public async Task<Result<IReadOnlyList<OutstandingInvoiceDto>>> Handle(GetArOutstandingInvoicesQuery request, CancellationToken cancellationToken)
    {
        var query = _context.SubSalesInvoices.AsNoTracking().Where(i => i.Status != Domain.Receivables.SalesInvoiceStatus.Cancelled && i.Status != Domain.Receivables.SalesInvoiceStatus.FullyPaid);

        if (request.CustomerId.HasValue)
        {
            var cid = new PartnerId(request.CustomerId.Value);
            query = query.Where(i => i.CustomerId == cid);
        }

        var invoices = await query.OrderBy(i => i.DueDate).ToListAsync(cancellationToken);
        var partnerIds = invoices.Select(i => i.CustomerId).Distinct().ToList();
        var partners = await _context.BusinessPartners.AsNoTracking().Where(p => partnerIds.Contains(p.Id)).ToDictionaryAsync(p => p.Id, p => p.Name, cancellationToken);

        var today = DateOnly.FromDateTime(DateTime.UtcNow);
        var list = invoices.Select(i => new OutstandingInvoiceDto(
            i.Id.Value,
            i.InvoiceNumber,
            i.InvoiceDate,
            i.DueDate,
            i.CustomerId.Value,
            partners.TryGetValue(i.CustomerId, out var name) ? name : "Khach hang",
            i.TotalAmount,
            i.ReceivedAmount,
            i.RemainingAmount,
            i.DueDate < today ? today.DayNumber - i.DueDate.DayNumber : 0
        )).ToList();

        return Result<IReadOnlyList<OutstandingInvoiceDto>>.Success(list);
    }

    public async Task<Result<IReadOnlyList<OutstandingInvoiceDto>>> Handle(GetApOutstandingInvoicesQuery request, CancellationToken cancellationToken)
    {
        var query = _context.SubPurchaseInvoices.AsNoTracking().Where(i => i.Status != Domain.Payables.InvoiceStatus.Cancelled && i.Status != Domain.Payables.InvoiceStatus.FullyPaid);

        if (request.VendorId.HasValue)
        {
            var vid = new PartnerId(request.VendorId.Value);
            query = query.Where(i => i.VendorId == vid);
        }

        var invoices = await query.OrderBy(i => i.DueDate).ToListAsync(cancellationToken);
        var partnerIds = invoices.Select(i => i.VendorId).Distinct().ToList();
        var partners = await _context.BusinessPartners.AsNoTracking().Where(p => partnerIds.Contains(p.Id)).ToDictionaryAsync(p => p.Id, p => p.Name, cancellationToken);

        var today = DateOnly.FromDateTime(DateTime.UtcNow);
        var list = invoices.Select(i => new OutstandingInvoiceDto(
            i.Id.Value,
            i.InvoiceNumber,
            i.InvoiceDate,
            i.DueDate,
            i.VendorId.Value,
            partners.TryGetValue(i.VendorId, out var name) ? name : "Nha cung cap",
            i.TotalAmount,
            i.PaidAmount,
            i.RemainingAmount,
            i.DueDate < today ? today.DayNumber - i.DueDate.DayNumber : 0
        )).ToList();

        return Result<IReadOnlyList<OutstandingInvoiceDto>>.Success(list);
    }

    public async Task<Result<AgingReportDto>> Handle(GetArAgingReportQuery request, CancellationToken cancellationToken)
    {
        var invoices = await _context.SubSalesInvoices.AsNoTracking()
            .Where(i => i.Status != Domain.Receivables.SalesInvoiceStatus.Cancelled && i.Status != Domain.Receivables.SalesInvoiceStatus.FullyPaid && i.InvoiceDate <= request.AsOfDate)
            .ToListAsync(cancellationToken);

        var partnerIds = invoices.Select(i => i.CustomerId).Distinct().ToList();
        var partners = await _context.BusinessPartners.AsNoTracking()
            .Where(p => partnerIds.Contains(p.Id))
            .ToDictionaryAsync(p => p.Id, cancellationToken);

        return Result<AgingReportDto>.Success(CalculateAgingReport(
            request.AsOfDate,
            invoices.Select(i => (i.CustomerId, i.RemainingAmount, i.DueDate)),
            partners));
    }

    public async Task<Result<AgingReportDto>> Handle(GetApAgingReportQuery request, CancellationToken cancellationToken)
    {
        var invoices = await _context.SubPurchaseInvoices.AsNoTracking()
            .Where(i => i.Status != Domain.Payables.InvoiceStatus.Cancelled && i.Status != Domain.Payables.InvoiceStatus.FullyPaid && i.InvoiceDate <= request.AsOfDate)
            .ToListAsync(cancellationToken);

        var partnerIds = invoices.Select(i => i.VendorId).Distinct().ToList();
        var partners = await _context.BusinessPartners.AsNoTracking()
            .Where(p => partnerIds.Contains(p.Id))
            .ToDictionaryAsync(p => p.Id, cancellationToken);

        return Result<AgingReportDto>.Success(CalculateAgingReport(
            request.AsOfDate,
            invoices.Select(i => (i.VendorId, i.RemainingAmount, i.DueDate)),
            partners));
    }

    private static AgingReportDto CalculateAgingReport(
        DateOnly asOfDate,
        IEnumerable<(PartnerId PartnerId, decimal Remaining, DateOnly DueDate)> items,
        Dictionary<PartnerId, Domain.MasterData.Partners.BusinessPartner> partnerLookup)
    {
        var grouped = items.GroupBy(x => x.PartnerId).ToList();
        var partnerSummaries = new List<PartnerAgingSummaryDto>();

        decimal totCurrent = 0m, tot130 = 0m, tot3160 = 0m, tot6190 = 0m, totAbove90 = 0m;
        int countCurrent = 0, count130 = 0, count3160 = 0, count6190 = 0, countAbove90 = 0;

        foreach (var g in grouped)
        {
            partnerLookup.TryGetValue(g.Key, out var p);
            var pCode = p?.PartnerCode ?? g.Key.ToString();
            var pName = p?.Name ?? "Doi tac";

            decimal pTot = 0m, pCur = 0m, p1 = 0m, p2 = 0m, p3 = 0m, p4 = 0m;

            foreach (var item in g)
            {
                pTot += item.Remaining;
                int overdueDays = asOfDate.DayNumber - item.DueDate.DayNumber;

                if (overdueDays <= 0)
                {
                    pCur += item.Remaining;
                    totCurrent += item.Remaining;
                    countCurrent++;
                }
                else if (overdueDays <= 30)
                {
                    p1 += item.Remaining;
                    tot130 += item.Remaining;
                    count130++;
                }
                else if (overdueDays <= 60)
                {
                    p2 += item.Remaining;
                    tot3160 += item.Remaining;
                    count3160++;
                }
                else if (overdueDays <= 90)
                {
                    p3 += item.Remaining;
                    tot6190 += item.Remaining;
                    count6190++;
                }
                else
                {
                    p4 += item.Remaining;
                    totAbove90 += item.Remaining;
                    countAbove90++;
                }
            }

            partnerSummaries.Add(new PartnerAgingSummaryDto(
                g.Key.Value,
                pCode,
                pName,
                pTot,
                pCur,
                p1,
                p2,
                p3,
                p4
            ));
        }

        var grandTotal = totCurrent + tot130 + tot3160 + tot6190 + totAbove90;
        var buckets = new List<AgingBucketDto>
        {
            new("Chua den han", totCurrent, countCurrent),
            new("Qua han 1-30 ngay", tot130, count130),
            new("Qua han 31-60 ngay", tot3160, count3160),
            new("Qua han 61-90 ngay", tot6190, count6190),
            new("Qua han tren 90 ngay", totAbove90, countAbove90)
        };

        return new AgingReportDto(asOfDate, partnerSummaries, grandTotal, buckets);
    }
}

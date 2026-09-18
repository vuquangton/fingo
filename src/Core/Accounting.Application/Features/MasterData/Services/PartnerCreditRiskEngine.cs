using Accounting.Application.Common.Interfaces;
using Accounting.Domain.Exceptions;
using Accounting.Domain.MasterData.Common;
using Accounting.Domain.MasterData.Partners;
using Microsoft.EntityFrameworkCore;

namespace Accounting.Application.Features.MasterData.Services;

public record PartnerCreditAssessment(
    PartnerId PartnerId,
    string PartnerCode,
    string PartnerName,
    decimal CreditLimit,
    decimal CurrentBalance,
    decimal ProposedAdditionalAmount,
    decimal ProjectedBalance,
    decimal UtilizationRate,
    PartnerRiskTier CurrentRiskTier,
    PartnerRiskTier ProjectedRiskTier,
    bool IsTransactionApproved,
    string RejectionReason);

public interface IPartnerCreditRiskEngine
{
    Task<PartnerCreditAssessment> EvaluateCreditGateAsync(
        PartnerId partnerId,
        decimal additionalAmount,
        CancellationToken cancellationToken = default);

    Task UpdateRiskTiersAsync(CancellationToken cancellationToken = default);
}

public class PartnerCreditRiskEngine : IPartnerCreditRiskEngine
{
    private readonly IAccountingDbContext _context;

    public PartnerCreditRiskEngine(IAccountingDbContext context)
    {
        _context = context;
    }

    public async Task<PartnerCreditAssessment> EvaluateCreditGateAsync(
        PartnerId partnerId,
        decimal additionalAmount,
        CancellationToken cancellationToken = default)
    {
        var partner = await _context.BusinessPartners
            .FirstOrDefaultAsync(p => p.Id == partnerId, cancellationToken);

        if (partner == null)
            throw new InvalidOperationException($"Partner with ID '{partnerId}' not found.");

        var currentBal = partner.CurrentReceivableBalance;
        var limit = partner.CreditLimit;
        var projBal = currentBal + additionalAmount;
        var utilRate = limit > 0 ? (projBal / limit) : 0m;

        PartnerRiskTier projectedTier;
        if (limit <= 0)
        {
            projectedTier = PartnerRiskTier.Low;
        }
        else if (utilRate > 1.0m)
        {
            projectedTier = PartnerRiskTier.Critical;
        }
        else if (utilRate >= 0.85m)
        {
            projectedTier = PartnerRiskTier.High;
        }
        else if (utilRate >= 0.50m)
        {
            projectedTier = PartnerRiskTier.Medium;
        }
        else
        {
            projectedTier = PartnerRiskTier.Low;
        }

        bool approved = true;
        string rejectionReason = string.Empty;

        if (partner.RiskTier == PartnerRiskTier.Critical)
        {
            approved = false;
            rejectionReason = $"Khách hàng '{partner.PartnerCode}' đang bị khóa nợ (Risk Tier: Critical).";
        }
        else if (limit > 0 && projBal > limit)
        {
            approved = false;
            rejectionReason = $"Vượt hạn mức tín dụng cho phép ({projBal:N0} đ > {limit:N0} đ). Tỷ lệ sử dụng hạn mức: {utilRate * 100:F1}%.";
        }

        return new PartnerCreditAssessment(
            partner.Id,
            partner.PartnerCode,
            partner.Name,
            limit,
            currentBal,
            additionalAmount,
            projBal,
            utilRate,
            partner.RiskTier,
            projectedTier,
            approved,
            rejectionReason);
    }

    public async Task UpdateRiskTiersAsync(CancellationToken cancellationToken = default)
    {
        var partners = await _context.BusinessPartners.ToListAsync(cancellationToken);
        foreach (var p in partners)
        {
            p.UpdateRiskTier();
        }
        await _context.SaveChangesAsync(cancellationToken);
    }
}

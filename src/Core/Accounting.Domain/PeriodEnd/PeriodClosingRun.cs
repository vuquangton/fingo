using Accounting.Domain.Common;
using Accounting.Domain.Entities.GeneralLedger;
using Accounting.Domain.Ledger;

namespace Accounting.Domain.PeriodEnd;

public class PeriodClosingRun : AggregateRoot<PeriodClosingRunId>, IAuditableEntity
{
    private readonly List<VoucherId> _generatedVoucherIds = [];

    public FiscalPeriodId FiscalPeriodId { get; private set; }
    public DateTime RunDate { get; private set; }
    public string PerformedBy { get; private set; } = string.Empty;
    public ClosingStep ClosingStep { get; private set; }
    public ClosingStatus Status { get; private set; }
    public string? Notes { get; private set; }
    public IReadOnlyCollection<VoucherId> GeneratedVoucherIds => _generatedVoucherIds.AsReadOnly();

    // IAuditableEntity
    public DateTime CreatedAtUtc { get; set; } = DateTime.UtcNow;
    public string? CreatedBy { get; set; }
    public DateTime? UpdatedAtUtc { get; set; }
    public string? UpdatedBy { get; set; }
    public DateTime? LastModifiedAtUtc { get => UpdatedAtUtc; set => UpdatedAtUtc = value; }
    public string? LastModifiedBy { get => UpdatedBy; set => UpdatedBy = value; }

    private PeriodClosingRun() { } // EF Core

    public PeriodClosingRun(
        PeriodClosingRunId id,
        FiscalPeriodId fiscalPeriodId,
        DateTime runDate,
        string performedBy,
        ClosingStep closingStep,
        string? notes = null)
    {
        if (id == PeriodClosingRunId.Empty)
            throw new ArgumentException("Period closing run ID cannot be empty.", nameof(id));

        if (string.IsNullOrWhiteSpace(performedBy))
            throw new ArgumentException("Performed by operator must be provided.", nameof(performedBy));

        Id = id;
        FiscalPeriodId = fiscalPeriodId;
        RunDate = runDate;
        PerformedBy = performedBy;
        ClosingStep = closingStep;
        Status = ClosingStatus.Success;
        Notes = notes;
    }

    public void AddGeneratedVoucher(VoucherId voucherId)
    {
        if (voucherId == VoucherId.Empty)
            throw new ArgumentException("Generated voucher ID cannot be empty.", nameof(voucherId));

        if (!_generatedVoucherIds.Contains(voucherId))
        {
            _generatedVoucherIds.Add(voucherId);
        }
    }

    public void MarkFailed(string reason)
    {
        Status = ClosingStatus.Failed;
        Notes = string.IsNullOrWhiteSpace(Notes) ? reason : $"{Notes} | Failed: {reason}";
    }
}

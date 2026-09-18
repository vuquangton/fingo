using Accounting.Domain.Common;
using Accounting.Domain.Entities.GeneralLedger;
using Accounting.Domain.Exceptions;
using Accounting.Domain.Ledger;

namespace Accounting.Domain.Payroll;

public class PayrollRun : AggregateRoot<PayrollRunId>, IAuditableEntity
{
    private readonly List<Payslip> _payslips = [];

    public FiscalPeriodId FiscalPeriodId { get; private set; }
    public string Title { get; private set; } = string.Empty;
    public DateTime ExecutionDate { get; private set; }
    public PayrollRunStatus Status { get; private set; }

    public decimal TotalGrossPay { get; private set; }
    public decimal TotalEmployeeInsurance { get; private set; }
    public decimal TotalPersonalIncomeTax { get; private set; }
    public decimal TotalEmployeeDeductions { get; private set; }
    public decimal TotalNetPay { get; private set; }
    public decimal TotalEmployerContributions { get; private set; }
    public decimal TotalEmployerCost { get; private set; }

    public VoucherId? LinkedVoucherId { get; private set; }
    public IReadOnlyCollection<Payslip> Payslips => _payslips.AsReadOnly();

    // IAuditableEntity
    public DateTime CreatedAtUtc { get; set; } = DateTime.UtcNow;
    public string? CreatedBy { get; set; }
    public DateTime? UpdatedAtUtc { get; set; }
    public string? UpdatedBy { get; set; }
    public DateTime? LastModifiedAtUtc { get => UpdatedAtUtc; set => UpdatedAtUtc = value; }
    public string? LastModifiedBy { get => UpdatedBy; set => UpdatedBy = value; }

    private PayrollRun() { }

    public PayrollRun(
        PayrollRunId id,
        FiscalPeriodId fiscalPeriodId,
        string title)
    {
        if (id == PayrollRunId.Empty)
            throw new ArgumentException("Payroll run ID cannot be empty.", nameof(id));

        var t = (title ?? string.Empty).Trim();
        if (string.IsNullOrWhiteSpace(t))
            throw new ArgumentException("Payroll run title cannot be empty.", nameof(title));

        Id = id;
        FiscalPeriodId = fiscalPeriodId;
        Title = t;
        ExecutionDate = DateTime.UtcNow;
        Status = PayrollRunStatus.Draft;
    }

    public void AddPayslip(Payslip payslip)
    {
        if (Status == PayrollRunStatus.PostedToGl)
            throw new PayrollRunAlreadyPostedException(Title, "cannot add payslips to an already posted payroll run");

        _payslips.Add(payslip);
        RecalculateTotals();
        Status = PayrollRunStatus.Calculated;
    }

    public void RecalculateTotals()
    {
        TotalGrossPay = _payslips.Sum(p => p.GrossSalary);
        TotalEmployeeInsurance = _payslips.Sum(p => p.TotalInsuranceEmployee);
        TotalPersonalIncomeTax = _payslips.Sum(p => p.PersonalIncomeTax);
        TotalEmployeeDeductions = _payslips.Sum(p => p.TotalEmployeeDeductions);
        TotalNetPay = _payslips.Sum(p => p.NetSalary);
        TotalEmployerContributions = _payslips.Sum(p => p.TotalEmployerContributions);
        TotalEmployerCost = _payslips.Sum(p => p.TotalEmployerCost);
    }

    public void Approve()
    {
        if (Status == PayrollRunStatus.PostedToGl)
            throw new PayrollRunAlreadyPostedException(Title, "cannot approve an already posted payroll run");

        if (_payslips.Count == 0)
            throw new InvalidOperationException("Cannot approve a payroll run with no payslips.");

        Status = PayrollRunStatus.Approved;
    }

    public void LinkGeneralLedgerVoucher(VoucherId voucherId)
    {
        if (Status == PayrollRunStatus.PostedToGl)
            throw new PayrollRunAlreadyPostedException(Title, "payroll run is already posted to general ledger");

        LinkedVoucherId = voucherId;
        Status = PayrollRunStatus.PostedToGl;
    }
}

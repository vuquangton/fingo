using Accounting.Application.Common.Interfaces;
using Accounting.Application.Common.Models;
using Accounting.Domain.Entities.Payroll;
using MediatR;
using Microsoft.EntityFrameworkCore;

namespace Accounting.Application.Features.Payroll;

public record LegacyPayrollSlipDto(
    Guid Id,
    string EmployeeCode,
    string FullName,
    string Department,
    decimal GrossSalary,
    decimal SocialInsuranceEmployee,
    decimal HealthInsuranceEmployee,
    decimal UnemploymentInsuranceEmployee,
    decimal PersonalIncomeTax,
    decimal NetSalary,
    decimal TotalEmployerCost);

public record GetPayrollSummaryQuery(int Year, int Month) : IRequest<Result<List<LegacyPayrollSlipDto>>>;

public class GetPayrollSummaryQueryHandler(IAccountingDbContext context) : IRequestHandler<GetPayrollSummaryQuery, Result<List<LegacyPayrollSlipDto>>>
{
    public async Task<Result<List<LegacyPayrollSlipDto>>> Handle(GetPayrollSummaryQuery request, CancellationToken cancellationToken)
    {
        var slips = await context.SalarySlips
            .Include(s => s.Employee)
            .AsNoTracking()
            .Where(s => s.Year == request.Year && s.Month == request.Month)
            .Select(s => new LegacyPayrollSlipDto(
                s.Id,
                s.Employee != null ? s.Employee.Code : string.Empty,
                s.Employee != null ? s.Employee.FullName : string.Empty,
                s.Employee != null ? s.Employee.Department : string.Empty,
                s.GrossSalary,
                s.SocialInsuranceEmployee,
                s.HealthInsuranceEmployee,
                s.UnemploymentInsuranceEmployee,
                s.PersonalIncomeTax,
                s.NetSalary,
                s.TotalEmployerCost))
            .ToListAsync(cancellationToken);

        return Result<List<LegacyPayrollSlipDto>>.Success(slips);
    }
}

using Accounting.Application.Common.Models;
using MediatR;

namespace Accounting.Application.Features.PeriodEnd;

public enum HealthCheckSeverity
{
    Info = 1,
    Warning = 2,
    Error = 3
}

public record HealthCheckItemDto(
    string RuleCode,
    string RuleTitle,
    HealthCheckSeverity Severity,
    bool Passed,
    string Details,
    int OffendingRecordsCount,
    IReadOnlyList<string>? OffendingIdentifiers = null);

public record PreClosingCheckReportDto(
    int Year,
    int Month,
    DateOnly CheckDate,
    bool CanProceedToClose,
    int ErrorCount,
    int WarningCount,
    IReadOnlyList<HealthCheckItemDto> Checks);

public record RunPreClosingCheckQuery(int Year, int Month) : IRequest<Result<PreClosingCheckReportDto>>;

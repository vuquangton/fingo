using Accounting.Application.Common.Interfaces;
using Accounting.Application.Common.Models;
using MediatR;

namespace Accounting.Application.Features.Tax;

public record GetFinancialPositionReportQuery(DateTime AsOfDate) : IRequest<Result<FinancialPositionReportDto>>;

public class GetFinancialPositionReportQueryHandler : IRequestHandler<GetFinancialPositionReportQuery, Result<FinancialPositionReportDto>>
{
    private readonly IStatutoryReportService _reportService;

    public GetFinancialPositionReportQueryHandler(IStatutoryReportService reportService)
    {
        _reportService = reportService;
    }

    public async Task<Result<FinancialPositionReportDto>> Handle(GetFinancialPositionReportQuery request, CancellationToken cancellationToken)
    {
        var result = await _reportService.GetFinancialPositionReportAsync(request.AsOfDate, cancellationToken);
        return Result<FinancialPositionReportDto>.Success(result);
    }
}

public record GetIncomeStatementQuery(DateTime FromDate, DateTime ToDate) : IRequest<Result<IncomeStatementReportDto>>;

public class GetIncomeStatementQueryHandler : IRequestHandler<GetIncomeStatementQuery, Result<IncomeStatementReportDto>>
{
    private readonly IStatutoryReportService _reportService;

    public GetIncomeStatementQueryHandler(IStatutoryReportService reportService)
    {
        _reportService = reportService;
    }

    public async Task<Result<IncomeStatementReportDto>> Handle(GetIncomeStatementQuery request, CancellationToken cancellationToken)
    {
        var result = await _reportService.GetIncomeStatementReportAsync(request.FromDate, request.ToDate, cancellationToken);
        return Result<IncomeStatementReportDto>.Success(result);
    }
}

public record GetCashFlowStatementQuery(DateTime FromDate, DateTime ToDate, bool IsDirectMethod = true) : IRequest<Result<CashFlowReportDto>>;

public class GetCashFlowStatementQueryHandler : IRequestHandler<GetCashFlowStatementQuery, Result<CashFlowReportDto>>
{
    private readonly IStatutoryReportService _reportService;

    public GetCashFlowStatementQueryHandler(IStatutoryReportService reportService)
    {
        _reportService = reportService;
    }

    public async Task<Result<CashFlowReportDto>> Handle(GetCashFlowStatementQuery request, CancellationToken cancellationToken)
    {
        var result = await _reportService.GetCashFlowReportAsync(request.FromDate, request.ToDate, request.IsDirectMethod, cancellationToken);
        return Result<CashFlowReportDto>.Success(result);
    }
}

public record GetVatDeclarationReportQuery(int Year, int Period, bool IsQuarter) : IRequest<Result<VatDeclarationDto>>;

public class GetVatDeclarationReportQueryHandler : IRequestHandler<GetVatDeclarationReportQuery, Result<VatDeclarationDto>>
{
    private readonly ITaxReportingService _taxService;

    public GetVatDeclarationReportQueryHandler(ITaxReportingService taxService)
    {
        _taxService = taxService;
    }

    public async Task<Result<VatDeclarationDto>> Handle(GetVatDeclarationReportQuery request, CancellationToken cancellationToken)
    {
        var result = await _taxService.GetVatDeclarationAsync(request.Year, request.Period, request.IsQuarter, cancellationToken);
        return Result<VatDeclarationDto>.Success(result);
    }
}

public record GenerateTaxSubmissionXmlCommand(int Year, int Period, bool IsQuarter) : IRequest<Result<string>>;

public class GenerateTaxSubmissionXmlCommandHandler : IRequestHandler<GenerateTaxSubmissionXmlCommand, Result<string>>
{
    private readonly ITaxReportingService _taxService;

    public GenerateTaxSubmissionXmlCommandHandler(ITaxReportingService taxService)
    {
        _taxService = taxService;
    }

    public async Task<Result<string>> Handle(GenerateTaxSubmissionXmlCommand request, CancellationToken cancellationToken)
    {
        var xml = await _taxService.GenerateTaxSubmissionXmlAsync(request.Year, request.Period, request.IsQuarter, cancellationToken);
        return Result<string>.Success(xml);
    }
}

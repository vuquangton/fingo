using Accounting.Application.Common.Interfaces;
using Accounting.Infrastructure.Compliance.Reports;
using Accounting.Infrastructure.Compliance.Tax;
using Microsoft.Extensions.DependencyInjection;

namespace Accounting.Infrastructure.Compliance;

public static class DependencyInjection
{
    public static IServiceCollection AddComplianceInfrastructure(this IServiceCollection services)
    {
        services.AddScoped<IStatutoryReportService, StatutoryReportService>();
        services.AddScoped<ITaxReportingService, TaxReportingService>();
        services.AddScoped<Services.IExcelExportService, Services.ExcelExportService>();
        services.AddScoped<Services.IPrintDocumentGenerator, Services.PrintDocumentGenerator>();
        services.AddScoped<IOpeningBalanceExcelParser, Services.OpeningBalanceExcelParser>();
        services.AddScoped<EInvoice.IEInvoiceProviderAdapter, EInvoice.StandardEInvoiceProviderAdapter>();

        return services;
    }
}

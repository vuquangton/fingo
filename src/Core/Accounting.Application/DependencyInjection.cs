using System.Reflection;
using Accounting.Application.Common.Behaviors;
using Accounting.Application.Features.MasterData.Services;
using FluentValidation;
using MediatR;
using Microsoft.Extensions.DependencyInjection;

namespace Accounting.Application;

public static class DependencyInjection
{
    public static IServiceCollection AddApplicationServices(this IServiceCollection services)
    {
        var assembly = Assembly.GetExecutingAssembly();

        services.AddValidatorsFromAssembly(assembly);

        services.AddMediatR(cfg =>
        {
            cfg.RegisterServicesFromAssembly(assembly);
            cfg.AddBehavior(typeof(IPipelineBehavior<,>), typeof(AuthorizationBehavior<,>));
            cfg.AddBehavior(typeof(IPipelineBehavior<,>), typeof(ValidationBehavior<,>));
            cfg.AddBehavior(typeof(IPipelineBehavior<,>), typeof(LoggingBehavior<,>));
        });

        services.AddScoped<Common.Interfaces.IGlVoucherBridgeService, Common.Services.GlVoucherBridgeService>();
        services.AddSingleton<IVietnamTaxLookupService, VietnamTaxLookupService>();
        services.AddScoped<IPartnerDeduplicationEngine, PartnerDeduplicationEngine>();
        services.AddScoped<IPartnerCreditRiskEngine, PartnerCreditRiskEngine>();

        return services;
    }
}

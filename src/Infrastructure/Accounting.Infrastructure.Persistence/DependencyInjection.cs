using Accounting.Application.Common.Interfaces;
using Accounting.Infrastructure.Persistence.Connections;
using Accounting.Infrastructure.Persistence.Context;
using Accounting.Infrastructure.Persistence.Interceptors;
using Accounting.Infrastructure.Persistence.Services;
using Microsoft.EntityFrameworkCore;
using Microsoft.Extensions.Configuration;
using Microsoft.Extensions.DependencyInjection;

namespace Accounting.Infrastructure.Persistence;

public static class DependencyInjection
{
    public static IServiceCollection AddPersistenceInfrastructure(this IServiceCollection services, IConfiguration configuration)
    {
        services.AddSingleton(configuration);

        // Interceptors
        services.AddScoped<AuditTrailInterceptor>();
        services.AddScoped<SoftDeleteInterceptor>();

        // DbContext with dynamic MariaDB & SQLite dialect selection
        services.AddDbContext<AccountingDbContext>((sp, options) =>
        {
            var provider = configuration.GetValue<string>("Database:Provider")
                ?? configuration.GetValue<string>("DatabaseProvider")
                ?? "Sqlite";

            if (string.Equals(provider, "MariaDB", StringComparison.OrdinalIgnoreCase) ||
                string.Equals(provider, "MySql", StringComparison.OrdinalIgnoreCase))
            {
                var mariaDbConn = configuration.GetValue<string>("Database:MariaDbConnection")
                    ?? configuration.GetConnectionString("MariaDB")
                    ?? configuration.GetConnectionString("MariaDbConnection")
                    ?? configuration.GetConnectionString("DefaultConnection")
                    ?? "Server=localhost;Port=3306;Database=accounting_core;User=dev;Password=123456;TreatTinyAsBoolean=true;CharSet=utf8mb4;";

                options.UseMySql(mariaDbConn, new MariaDbServerVersion(new Version(12, 3, 0)));
            }
            else
            {
                var sqliteConn = configuration.GetValue<string>("Database:SqliteConnection")
                    ?? configuration.GetConnectionString("Sqlite")
                    ?? configuration.GetConnectionString("SqliteConnection")
                    ?? "Data Source=accounting.db";

                options.UseSqlite(sqliteConn);
            }

            options.AddInterceptors(
                sp.GetRequiredService<AuditTrailInterceptor>(),
                sp.GetRequiredService<SoftDeleteInterceptor>());
        });

        services.AddScoped<IAccountingDbContext>(provider => provider.GetRequiredService<AccountingDbContext>());
        services.AddScoped<IApplicationDbContext>(provider => provider.GetRequiredService<AccountingDbContext>());

        // Connection factories (both ISqlConnectionFactory and IDbConnectionFactory)
        services.AddSingleton<DapperSqlConnectionFactory>();
        services.AddSingleton<ISqlConnectionFactory>(sp => sp.GetRequiredService<DapperSqlConnectionFactory>());
        services.AddSingleton<IDbConnectionFactory>(sp => sp.GetRequiredService<DapperSqlConnectionFactory>());

        // Domain & Infrastructure services
        services.AddScoped<IVoucherPostingService, VoucherPostingService>();
        services.AddScoped<IInventoryCostingEngine, InventoryCostingEngine>();
        services.AddScoped<IPeriodClosingService, PeriodClosingService>();
        services.AddScoped<IAuditLogService, AuditLogService>();
        services.AddScoped<IDatabaseBackupService, DatabaseBackupService>();

        services.AddSingleton<CurrentUserService>();
        services.AddSingleton<ICurrentUserService>(sp => sp.GetRequiredService<CurrentUserService>());
        services.AddSingleton<ICurrentUserContext>(sp => sp.GetRequiredService<CurrentUserService>());
        services.AddSingleton<IDateTimeService, DateTimeService>();
        services.AddSingleton<IPasswordHasher, Accounting.Application.Common.Services.PasswordHasher>();
        services.AddScoped<ICryptoBackupEngine, Accounting.Application.Features.Ops.CryptoBackupEngine>();
        services.AddScoped<IReportHeaderProvider, ReportHeaderProvider>();

        return services;
    }
}

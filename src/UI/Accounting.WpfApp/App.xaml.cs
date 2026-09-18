using System.IO;
using System.Windows;
using Accounting.Application;
using Accounting.Infrastructure.Compliance;
using Accounting.Infrastructure.Persistence;
using Accounting.Infrastructure.Persistence.Context;
using Accounting.Infrastructure.Persistence.Seeding;
using Accounting.WpfApp.Services;
using Accounting.WpfApp.ViewModels;
using Microsoft.Extensions.Configuration;
using Microsoft.Extensions.DependencyInjection;
using Microsoft.Extensions.Hosting;

namespace Accounting.WpfApp;

public partial class App : System.Windows.Application
{
    private IHost? _host;

    public App()
    {
        DispatcherUnhandledException += App_DispatcherUnhandledException;
    }

    private void App_DispatcherUnhandledException(object sender, System.Windows.Threading.DispatcherUnhandledExceptionEventArgs e)
    {
        var logMessage = $"[{DateTime.UtcNow:O}] UI Unhandled Exception: {e.Exception.Message}\nStack: {e.Exception.StackTrace}\n";
        try
        {
            File.AppendAllText("error_log.txt", logMessage);
        }
        catch { }

        MessageBox.Show($"An unexpected error occurred: {e.Exception.Message}", "Application Error", MessageBoxButton.OK, MessageBoxImage.Error);
        e.Handled = true;
    }

    protected override async void OnStartup(StartupEventArgs e)
    {
        base.OnStartup(e);

        _host = Host.CreateDefaultBuilder()
            .ConfigureAppConfiguration((context, config) =>
            {
                config.SetBasePath(Directory.GetCurrentDirectory());
                config.AddJsonFile("appsettings.json", optional: true, reloadOnChange: true);
            })
            .ConfigureServices((context, services) =>
            {
                // Architectural layers
                services.AddApplicationServices();
                services.AddPersistenceInfrastructure(context.Configuration);
                services.AddComplianceInfrastructure();

                // Services & ViewModels
                services.AddSingleton<KeyboardShortcutManager>();
                services.AddSingleton<VoucherEntryViewModel>();
                services.AddSingleton<GeneralLedgerViewModel>();
                services.AddSingleton<StatutoryReportsViewModel>();
                services.AddSingleton<CompanySettingViewModel>();
                services.AddSingleton<OpeningBalanceViewModel>();
                services.AddSingleton<UserManagementViewModel>();
                services.AddSingleton<TreasuryViewModel>();
                services.AddSingleton<PurchasingViewModel>();
                services.AddSingleton<SalesViewModel>();
                services.AddSingleton<InventoryViewModel>();
                services.AddSingleton<FixedAssetsViewModel>();
                services.AddSingleton<PayrollViewModel>();
                services.AddSingleton<MasterDataViewModel>();
                services.AddSingleton<PeriodClosingViewModel>();
                services.AddSingleton<SettingsViewModel>();
                services.AddSingleton<DocumentManagerViewModel>();
                services.AddSingleton<UniversalSearchViewModel>();
                services.AddSingleton<ClosingPipelineWizardViewModel>();
                services.AddSingleton<MainViewModel>();
                services.AddSingleton<MainWindow>();
            })
            .Build();

        await _host.StartAsync();

        // Auto-seed embedded SQLite / initial database schema
        using (var scope = _host.Services.CreateScope())
        {
            var dbContext = scope.ServiceProvider.GetRequiredService<AccountingDbContext>();
            var passwordHasher = scope.ServiceProvider.GetRequiredService<Accounting.Application.Common.Interfaces.IPasswordHasher>();
            await DbInitializer.SeedAsync(dbContext);
            await SecuritySeeder.SeedSecurityAsync(dbContext, passwordHasher);
        }

        var mainWindow = _host.Services.GetRequiredService<MainWindow>();
        mainWindow.Show();
    }

    protected override async void OnExit(ExitEventArgs e)
    {
        if (_host != null)
        {
            await _host.StopAsync();
            _host.Dispose();
        }
        base.OnExit(e);
    }
}

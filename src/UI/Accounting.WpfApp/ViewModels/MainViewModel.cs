using System.Collections.ObjectModel;
using Accounting.Application.Common.Interfaces;
using Accounting.Application.Features.GeneralLedger;
using Accounting.Application.Features.Reporting;
using Accounting.Domain.Entities.GeneralLedger;
using Accounting.Domain.Enums;
using CommunityToolkit.Mvvm.ComponentModel;
using CommunityToolkit.Mvvm.Input;
using MediatR;
using Microsoft.Extensions.Configuration;

namespace Accounting.WpfApp.ViewModels;

public enum AppModule
{
    Dashboard,
    Vouchers,
    GeneralLedger,
    Treasury,
    Purchasing,
    Sales,
    Inventory,
    FixedAssets,
    Payroll,
    StatutoryReports,
    MasterData,
    PeriodEnd,
    CompanySetup,
    OpeningBalance,
    UserManagement,
    Settings
}

public partial class MainViewModel : ObservableObject
{
    private readonly IMediator _mediator;
    private readonly IConfiguration _configuration;
    private readonly IDatabaseBackupService _backupService;

    [ObservableProperty]
    private AppModule _currentModule = AppModule.Dashboard;

    [ObservableProperty]
    private string _databaseProvider = "SQLite (Local Embedded)";

    [ObservableProperty]
    private string _currentUser = "admin (System Administrator)";

    [ObservableProperty]
    private string _fiscalPeriod = "09/2026";

    [ObservableProperty]
    private string _statusMessage = "Ready. Press F2 for New Voucher, F8 to Post, F5 to Refresh.";

    // Dashboard KPIs
    [ObservableProperty]
    private decimal _cashBalance = 1_450_000_000m;

    [ObservableProperty]
    private decimal _receivablesTotal = 850_000_000m;

    [ObservableProperty]
    private decimal _payablesTotal = 620_000_000m;

    [ObservableProperty]
    private decimal _netRevenueMtd = 3_200_000_000m;

    [ObservableProperty]
    private decimal _inventoryValue = 2_150_000_000m;

    public VoucherEntryViewModel VoucherEntry { get; }
    public GeneralLedgerViewModel GeneralLedger { get; }
    public StatutoryReportsViewModel StatutoryReports { get; }
    public CompanySettingViewModel CompanySetup { get; }
    public OpeningBalanceViewModel OpeningBalance { get; }
    public UserManagementViewModel UserManagement { get; }
    public TreasuryViewModel Treasury { get; }
    public PurchasingViewModel Purchasing { get; }
    public SalesViewModel Sales { get; }
    public InventoryViewModel Inventory { get; }
    public FixedAssetsViewModel FixedAssets { get; }
    public PayrollViewModel Payroll { get; }
    public MasterDataViewModel MasterData { get; }
    public PeriodClosingViewModel PeriodClosing { get; }
    public SettingsViewModel Settings { get; }
    public DocumentManagerViewModel DocumentManager { get; }
    public UniversalSearchViewModel UniversalSearch { get; }
    public ClosingPipelineWizardViewModel ClosingWizard { get; }

    public MainViewModel(
        IMediator mediator,
        IConfiguration configuration,
        IDatabaseBackupService backupService,
        VoucherEntryViewModel voucherEntry,
        GeneralLedgerViewModel generalLedger,
        StatutoryReportsViewModel statutoryReports,
        CompanySettingViewModel companySetup,
        OpeningBalanceViewModel openingBalance,
        UserManagementViewModel userManagement,
        TreasuryViewModel treasury,
        PurchasingViewModel purchasing,
        SalesViewModel sales,
        InventoryViewModel inventory,
        FixedAssetsViewModel fixedAssets,
        PayrollViewModel payroll,
        MasterDataViewModel masterData,
        PeriodClosingViewModel periodClosing,
        SettingsViewModel settings,
        DocumentManagerViewModel documentManager,
        UniversalSearchViewModel universalSearch,
        ClosingPipelineWizardViewModel closingWizard)
    {
        _mediator = mediator;
        _configuration = configuration;
        _backupService = backupService;
        VoucherEntry = voucherEntry;
        GeneralLedger = generalLedger;
        StatutoryReports = statutoryReports;
        CompanySetup = companySetup;
        OpeningBalance = openingBalance;
        UserManagement = userManagement;
        Treasury = treasury;
        Purchasing = purchasing;
        Sales = sales;
        Inventory = inventory;
        FixedAssets = fixedAssets;
        Payroll = payroll;
        MasterData = masterData;
        PeriodClosing = periodClosing;
        Settings = settings;
        DocumentManager = documentManager;
        UniversalSearch = universalSearch;
        ClosingWizard = closingWizard;

        // Wire search navigation
        UniversalSearch.OnNavigateToTarget += item =>
        {
            if (Enum.TryParse<AppModule>(item.NavigationTarget, true, out var targetModule))
            {
                Navigate(targetModule);
            }
        };

        // Open Voucher Entry & General Ledger as default MDI documents
        DocumentManager.OpenDocument(VoucherEntry);
        DocumentManager.OpenDocument(GeneralLedger);
        DocumentManager.OpenDocument(ClosingWizard);

        var provider = configuration.GetValue<string>("Database:Provider")
            ?? configuration.GetValue<string>("DatabaseProvider")
            ?? "Sqlite";

        DatabaseProvider = string.Equals(provider, "MariaDB", StringComparison.OrdinalIgnoreCase) ||
                           string.Equals(provider, "MySql", StringComparison.OrdinalIgnoreCase)
            ? "MariaDB v12.3 (Enterprise Core)"
            : string.Equals(provider, "PostgreSql", StringComparison.OrdinalIgnoreCase)
            ? "PostgreSQL v16 (Client-Server)"
            : "SQLite v3 (Zero-Config Embedded)";

        _ = LoadDashboardAsync();
    }

    [RelayCommand]
    public async Task LoadDashboardAsync()
    {
        try
        {
            var res = await _mediator.Send(new GetDashboardSummaryQuery(DateOnly.FromDateTime(DateTime.Today)));
            if (res.IsSuccess && res.Value != null)
            {
                CashBalance = res.Value.CashBalance;
                ReceivablesTotal = res.Value.ReceivablesTotal;
                PayablesTotal = res.Value.PayablesTotal;
                InventoryValue = res.Value.InventoryValue;
                NetRevenueMtd = res.Value.NetRevenueMtd;
            }
        }
        catch { }
    }

    [RelayCommand]
    public void Navigate(AppModule module)
    {
        CurrentModule = module;
        StatusMessage = $"Navigated to {module}. Press F1 for Help.";

        // Also route MDI tabs if applicable
        if (module == AppModule.Vouchers)
        {
            DocumentManager.OpenDocument(VoucherEntry);
        }
        else if (module == AppModule.GeneralLedger)
        {
            DocumentManager.OpenDocument(GeneralLedger);
        }
        else if (module == AppModule.PeriodEnd)
        {
            DocumentManager.OpenDocument(ClosingWizard);
        }
    }

    [RelayCommand]
    public async Task RefreshDataAsync()
    {
        StatusMessage = "Refreshing system data...";
        await LoadDashboardAsync();
        await GeneralLedger.LoadLedgerAsync();
        StatusMessage = "Data refreshed successfully.";
    }
}

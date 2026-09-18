using System.Collections.ObjectModel;
using Accounting.Application.Features.FixedAssets;
using Accounting.Application.Features.GeneralLedger;
using Accounting.Application.Features.Inventory;
using Accounting.Application.Features.MasterData;
using Accounting.Application.Features.Ops;
using Accounting.Application.Features.Payroll;
using Accounting.Application.Features.PeriodEnd;
using Accounting.Application.Features.Purchasing;
using Accounting.Application.Features.Sales;
using Accounting.Application.Features.Security;
using Accounting.Application.Features.Treasury;
using Accounting.Domain.Enums;
using CommunityToolkit.Mvvm.ComponentModel;
using CommunityToolkit.Mvvm.Input;
using MediatR;

namespace Accounting.WpfApp.ViewModels;

// =========================================================================
// 1. TREASURY VIEW MODEL
// =========================================================================
public partial class TreasuryViewModel : ObservableObject
{
    private readonly IMediator _mediator;

    [ObservableProperty]
    private DateTime _fromDate = new(DateTime.Today.Year, DateTime.Today.Month, 1);

    [ObservableProperty]
    private DateTime _toDate = DateTime.Today;

    // Cash Voucher Form
    [ObservableProperty]
    private string _newVoucherNumber = $"PT-{DateTime.Today:yyyyMMdd}-01";

    [ObservableProperty]
    private VoucherType _selectedVoucherType = VoucherType.CashReceipt;

    [ObservableProperty]
    private DateTime _voucherDate = DateTime.Today;

    [ObservableProperty]
    private string _personName = "Nguyễn Văn A";

    [ObservableProperty]
    private string _reason = "Thu tiền bán hàng / Rút tiền nhập quỹ";

    [ObservableProperty]
    private decimal _amount = 10_000_000m;

    [ObservableProperty]
    private string _debitAccount = "1111";

    [ObservableProperty]
    private string _creditAccount = "131";

    [ObservableProperty]
    private string _statusMessage = string.Empty;

    // Bank Reconciliation Form
    [ObservableProperty]
    private BankAccountItemDto? _selectedBankAccount;

    [ObservableProperty]
    private DateTime _statementDate = DateTime.Today;

    [ObservableProperty]
    private decimal _statementClosingBalance;

    public ObservableCollection<CashBookEntryDto> CashBookEntries { get; } = [];
    public ObservableCollection<BankAccountItemDto> BankAccounts { get; } = [];
    public ObservableCollection<BankReconciliationDto> Reconciliations { get; } = [];

    public TreasuryViewModel(IMediator mediator)
    {
        _mediator = mediator;
    }

    [RelayCommand]
    public async Task LoadTreasuryDataAsync()
    {
        var cbRes = await _mediator.Send(new GetCashBookQuery(FromDate, ToDate));
        if (cbRes.IsSuccess && cbRes.Value != null)
        {
            CashBookEntries.Clear();
            foreach (var e in cbRes.Value) CashBookEntries.Add(e);
        }

        var baRes = await _mediator.Send(new GetBankAccountsQuery());
        if (baRes.IsSuccess && baRes.Value != null)
        {
            BankAccounts.Clear();
            foreach (var b in baRes.Value) BankAccounts.Add(b);
            if (SelectedBankAccount == null && BankAccounts.Count > 0)
                SelectedBankAccount = BankAccounts[0];
        }

        var reconRes = await _mediator.Send(new GetBankReconciliationsQuery());
        if (reconRes.IsSuccess && reconRes.Value != null)
        {
            Reconciliations.Clear();
            foreach (var r in reconRes.Value) Reconciliations.Add(r);
        }

        StatusMessage = "Đã nạp số liệu Thủ quỹ & Ngân hàng.";
    }

    [RelayCommand]
    public async Task CreateCashVoucherAsync()
    {
        var coaRes = await _mediator.Send(new GetChartOfAccountsQuery());
        if (!coaRes.IsSuccess || coaRes.Value == null) return;

        var dAcc = coaRes.Value.FirstOrDefault(a => a.AccountNumber == DebitAccount.Trim());
        var cAcc = coaRes.Value.FirstOrDefault(a => a.AccountNumber == CreditAccount.Trim());
        if (dAcc == null || cAcc == null)
        {
            StatusMessage = "Tài khoản định khoản không hợp lệ.";
            return;
        }

        var res = await _mediator.Send(new CreateCashVoucherCommand(
            NewVoucherNumber,
            SelectedVoucherType,
            VoucherDate,
            PersonName,
            Reason,
            Amount,
            dAcc.Id,
            cAcc.Id));

        if (res.IsSuccess)
        {
            StatusMessage = $"Đã lập chứng từ tiền mặt {NewVoucherNumber} thành công!";
            NewVoucherNumber = $"{(SelectedVoucherType == VoucherType.CashReceipt ? "PT" : "PC")}-{DateTime.Today:yyyyMMdd}-{Random.Shared.Next(10, 99)}";
            await LoadTreasuryDataAsync();
        }
        else
        {
            StatusMessage = $"Lỗi lập chứng từ: {res.ErrorMessage}";
        }
    }

    [RelayCommand]
    public async Task ReconcileBankAsync()
    {
        if (SelectedBankAccount == null)
        {
            StatusMessage = "Vui lòng chọn tài khoản ngân hàng.";
            return;
        }

        var res = await _mediator.Send(new CreateBankReconciliationCommand(
            SelectedBankAccount.Id,
            StatementDate,
            StatementClosingBalance,
            "Đối soát tự động qua giao diện"));

        if (res.IsSuccess)
        {
            StatusMessage = "Đã ghi nhận biên bản đối soát sao kê ngân hàng!";
            await LoadTreasuryDataAsync();
        }
        else
        {
            StatusMessage = $"Lỗi: {res.ErrorMessage}";
        }
    }
}

// =========================================================================
// 2. PURCHASING VIEW MODEL
// =========================================================================
public partial class PurchasingViewModel : ObservableObject
{
    private readonly IMediator _mediator;

    [ObservableProperty]
    private string _poNumber = $"PO-{DateTime.Today:yyyyMMdd}-01";

    [ObservableProperty]
    private VendorItemDto? _selectedVendor;

    [ObservableProperty]
    private DateTime _orderDate = DateTime.Today;

    [ObservableProperty]
    private string _itemName = "Vật tư phụ liệu sản xuất A";

    [ObservableProperty]
    private decimal _quantity = 100m;

    [ObservableProperty]
    private decimal _unitPrice = 50_000m;

    [ObservableProperty]
    private string _statusMessage = string.Empty;

    public ObservableCollection<VendorItemDto> Vendors { get; } = [];
    public ObservableCollection<VendorAgingDto> VendorAgings { get; } = [];

    public PurchasingViewModel(IMediator mediator)
    {
        _mediator = mediator;
    }

    [RelayCommand]
    public async Task LoadPurchasingDataAsync()
    {
        var vRes = await _mediator.Send(new GetVendorsQuery());
        if (vRes.IsSuccess && vRes.Value != null)
        {
            Vendors.Clear();
            foreach (var v in vRes.Value) Vendors.Add(v);
            if (SelectedVendor == null && Vendors.Count > 0)
                SelectedVendor = Vendors[0];
        }

        var aRes = await _mediator.Send(new GetVendorAgingQuery());
        if (aRes.IsSuccess && aRes.Value != null)
        {
            VendorAgings.Clear();
            foreach (var a in aRes.Value) VendorAgings.Add(a);
        }

        StatusMessage = "Đã nạp danh sách NCC & Báo cáo công nợ phải trả.";
    }

    [RelayCommand]
    public async Task CreatePurchaseOrderAsync()
    {
        if (SelectedVendor == null)
        {
            StatusMessage = "Vui lòng chọn Nhà Cung Cấp.";
            return;
        }

        var lines = new List<PurchaseOrderLineDto>
        {
            new(Guid.NewGuid(), ItemName, Quantity, UnitPrice)
        };

        var res = await _mediator.Send(new CreatePurchaseOrderCommand(
            PoNumber,
            SelectedVendor.Id,
            OrderDate,
            lines));

        if (res.IsSuccess)
        {
            StatusMessage = $"Đã tạo Đơn Mua Hàng {PoNumber} ({Quantity * UnitPrice:N0} đ) thành công!";
            PoNumber = $"PO-{DateTime.Today:yyyyMMdd}-{Random.Shared.Next(10, 99)}";
            await LoadPurchasingDataAsync();
        }
        else
        {
            StatusMessage = $"Lỗi: {res.ErrorMessage}";
        }
    }
}

// =========================================================================
// 3. SALES VIEW MODEL
// =========================================================================
public partial class SalesViewModel : ObservableObject
{
    private readonly IMediator _mediator;

    [ObservableProperty]
    private CustomerItemDto? _selectedCustomer;

    [ObservableProperty]
    private string _invoiceNumber = $"HD-{DateTime.Today:yyyyMMdd}-001";

    [ObservableProperty]
    private string _invoiceSeries = "1C26TAA";

    [ObservableProperty]
    private DateTime _invoiceDate = DateTime.Today;

    [ObservableProperty]
    private decimal _subtotal = 20_000_000m;

    [ObservableProperty]
    private VatRate _vatRate = VatRate.Rate10;

    [ObservableProperty]
    private string _statusMessage = string.Empty;

    public ObservableCollection<CustomerItemDto> Customers { get; } = [];
    public ObservableCollection<CustomerAgingDto> CustomerAgings { get; } = [];

    public SalesViewModel(IMediator mediator)
    {
        _mediator = mediator;
    }

    [RelayCommand]
    public async Task LoadSalesDataAsync()
    {
        var cRes = await _mediator.Send(new GetCustomersQuery());
        if (cRes.IsSuccess && cRes.Value != null)
        {
            Customers.Clear();
            foreach (var c in cRes.Value) Customers.Add(c);
            if (SelectedCustomer == null && Customers.Count > 0)
                SelectedCustomer = Customers[0];
        }

        var aRes = await _mediator.Send(new GetCustomerAgingQuery());
        if (aRes.IsSuccess && aRes.Value != null)
        {
            CustomerAgings.Clear();
            foreach (var a in aRes.Value) CustomerAgings.Add(a);
        }

        StatusMessage = "Đã nạp danh sách Khách hàng & Báo cáo công nợ phải thu.";
    }

    [RelayCommand]
    public async Task CreateSalesInvoiceAsync()
    {
        if (SelectedCustomer == null)
        {
            StatusMessage = "Vui lòng chọn khách hàng.";
            return;
        }

        var coaRes = await _mediator.Send(new GetChartOfAccountsQuery());
        if (!coaRes.IsSuccess || coaRes.Value == null) return;

        var recAcc = coaRes.Value.FirstOrDefault(a => a.AccountNumber == "131");
        var revAcc = coaRes.Value.FirstOrDefault(a => a.AccountNumber == "5111") ?? coaRes.Value.FirstOrDefault(a => a.AccountNumber == "511");
        var vatAcc = coaRes.Value.FirstOrDefault(a => a.AccountNumber == "33311") ?? coaRes.Value.FirstOrDefault(a => a.AccountNumber == "3331");

        if (recAcc == null || revAcc == null || vatAcc == null)
        {
            StatusMessage = "Thiếu tài khoản kế toán 131, 511 hoặc 33311 trong hệ thống.";
            return;
        }

        var res = await _mediator.Send(new CreateSalesInvoiceCommand(
            InvoiceNumber,
            InvoiceSeries,
            InvoiceDate,
            SelectedCustomer.Id,
            Subtotal,
            VatRate,
            InvoiceDate.AddDays(30),
            recAcc.Id,
            revAcc.Id,
            vatAcc.Id));

        if (res.IsSuccess)
        {
            StatusMessage = $"Đã lập Hóa đơn bán hàng {InvoiceSeries}-{InvoiceNumber} thành công!";
            InvoiceNumber = $"HD-{DateTime.Today:yyyyMMdd}-{Random.Shared.Next(100, 999)}";
            await LoadSalesDataAsync();
        }
        else
        {
            StatusMessage = $"Lỗi: {res.ErrorMessage}";
        }
    }
}

// =========================================================================
// 4. INVENTORY VIEW MODEL
// =========================================================================
public partial class InventoryViewModel : ObservableObject
{
    private readonly IMediator _mediator;

    [ObservableProperty]
    private WarehouseItemDto? _selectedSourceWarehouse;

    [ObservableProperty]
    private WarehouseItemDto? _selectedTargetWarehouse;

    [ObservableProperty]
    private string _transferNumber = $"CK-{DateTime.Today:yyyyMMdd}-01";

    [ObservableProperty]
    private DateTime _transferDate = DateTime.Today;

    [ObservableProperty]
    private string _transferReason = "Điều chuyển hàng phục vụ phân phối chi nhánh";

    [ObservableProperty]
    private int _costingYear = DateTime.Today.Year;

    [ObservableProperty]
    private int _costingMonth = DateTime.Today.Month;

    [ObservableProperty]
    private string _statusMessage = string.Empty;

    public ObservableCollection<StockBalanceDto> StockBalances { get; } = [];
    public ObservableCollection<WarehouseItemDto> Warehouses { get; } = [];

    public InventoryViewModel(IMediator mediator)
    {
        _mediator = mediator;
    }

    [RelayCommand]
    public async Task LoadInventoryDataAsync()
    {
        var sbRes = await _mediator.Send(new GetStockBalanceReportQuery());
        if (sbRes.IsSuccess && sbRes.Value != null)
        {
            StockBalances.Clear();
            foreach (var s in sbRes.Value) StockBalances.Add(s);
        }

        var wRes = await _mediator.Send(new GetWarehousesQuery());
        if (wRes.IsSuccess && wRes.Value != null)
        {
            Warehouses.Clear();
            foreach (var w in wRes.Value) Warehouses.Add(w);
            if (Warehouses.Count >= 2)
            {
                SelectedSourceWarehouse = Warehouses[0];
                SelectedTargetWarehouse = Warehouses[1];
            }
            else if (Warehouses.Count == 1)
            {
                SelectedSourceWarehouse = Warehouses[0];
                SelectedTargetWarehouse = Warehouses[0];
            }
        }

        StatusMessage = "Đã nạp Báo cáo tồn kho & Danh mục kho hàng.";
    }

    [RelayCommand]
    public async Task CalculateCostingAsync()
    {
        StatusMessage = $"Đang tính giá vốn bình quân gia quyền kỳ {CostingMonth:D2}/{CostingYear}...";
        var res = await _mediator.Send(new CalculateInventoryCostingCommand(CostingYear, CostingMonth));
        if (res.IsSuccess)
        {
            StatusMessage = $"Đã tính toán và cập nhật giá vốn xuất kho kỳ {CostingMonth:D2}/{CostingYear} thành công!";
            await LoadInventoryDataAsync();
        }
        else
        {
            StatusMessage = $"Lỗi tính giá vốn: {res.ErrorMessage}";
        }
    }
}

// =========================================================================
// 5. FIXED ASSETS VIEW MODEL
// =========================================================================
public partial class FixedAssetsViewModel : ObservableObject
{
    private readonly IMediator _mediator;

    [ObservableProperty]
    private int _depreciationYear = DateTime.Today.Year;

    [ObservableProperty]
    private int _depreciationMonth = DateTime.Today.Month;

    [ObservableProperty]
    private string _statusMessage = string.Empty;

    public ObservableCollection<FixedAssetSummaryDto> Assets { get; } = [];

    public FixedAssetsViewModel(IMediator mediator)
    {
        _mediator = mediator;
    }

    [RelayCommand]
    public async Task LoadAssetsAsync()
    {
        var res = await _mediator.Send(new GetFixedAssetLedgerQuery());
        if (res.IsSuccess && res.Value != null)
        {
            Assets.Clear();
            foreach (var a in res.Value) Assets.Add(a);
            StatusMessage = $"Đã nạp {Assets.Count} tài sản cố định trong sổ.";
        }
    }

    [RelayCommand]
    public async Task RunDepreciationAsync()
    {
        var coaRes = await _mediator.Send(new GetChartOfAccountsQuery());
        if (!coaRes.IsSuccess || coaRes.Value == null) return;

        var expAcc = coaRes.Value.FirstOrDefault(a => a.AccountNumber == "642" || a.AccountNumber == "6422");
        var depAcc = coaRes.Value.FirstOrDefault(a => a.AccountNumber == "214" || a.AccountNumber == "2141");

        if (expAcc == null || depAcc == null)
        {
            StatusMessage = "Không tìm thấy TK Chi phí 642 hoặc TK Hao mòn 214.";
            return;
        }

        var res = await _mediator.Send(new CalculateDepreciationCommand(
            DepreciationYear,
            DepreciationMonth,
            expAcc.Id,
            depAcc.Id));

        if (res.IsSuccess)
        {
            StatusMessage = $"Đã trích khấu hao TSCĐ kỳ {DepreciationMonth:D2}/{DepreciationYear} và sinh chứng từ kế toán!";
            await LoadAssetsAsync();
        }
        else
        {
            StatusMessage = $"Thông báo: {res.ErrorMessage}";
        }
    }
}

// =========================================================================
// 6. PAYROLL VIEW MODEL
// =========================================================================
public partial class PayrollViewModel : ObservableObject
{
    private readonly IMediator _mediator;

    [ObservableProperty]
    private int _payrollYear = DateTime.Today.Year;

    [ObservableProperty]
    private int _payrollMonth = DateTime.Today.Month;

    [ObservableProperty]
    private string _statusMessage = string.Empty;

    public ObservableCollection<EmployeeItemDto> Employees { get; } = [];
    public ObservableCollection<LegacyPayrollSlipDto> SalarySlips { get; } = [];

    public PayrollViewModel(IMediator mediator)
    {
        _mediator = mediator;
    }

    [RelayCommand]
    public async Task LoadPayrollDataAsync()
    {
        var empRes = await _mediator.Send(new GetEmployeesQuery());
        if (empRes.IsSuccess && empRes.Value != null)
        {
            Employees.Clear();
            foreach (var e in empRes.Value) Employees.Add(e);
        }

        var slipsRes = await _mediator.Send(new GetPayrollSummaryQuery(PayrollYear, PayrollMonth));
        if (slipsRes.IsSuccess && slipsRes.Value != null)
        {
            SalarySlips.Clear();
            foreach (var s in slipsRes.Value) SalarySlips.Add(s);
        }

        StatusMessage = $"Đã nạp danh sách nhân sự và bảng lương kỳ {PayrollMonth:D2}/{PayrollYear}.";
    }
}

// =========================================================================
// 7. SETTINGS & AUDIT VIEW MODEL
// =========================================================================
public partial class SettingsViewModel : ObservableObject
{
    private readonly IMediator _mediator;

    [ObservableProperty]
    private string _backupDestination = "D:\\accounting\\backups";

    [ObservableProperty]
    private string _restoreFilePath = string.Empty;

    [ObservableProperty]
    private string _targetDatabasePath = "D:\\accounting\\accounting.db";

    [ObservableProperty]
    private string _statusMessage = string.Empty;

    public ObservableCollection<Accounting.Application.Features.GeneralLedger.AccountDto> Accounts { get; } = [];
    public ObservableCollection<FiscalPeriodDto> FiscalPeriods { get; } = [];
    public ObservableCollection<AuditTrailItemDto> AuditTrails { get; } = [];

    public SettingsViewModel(IMediator mediator)
    {
        _mediator = mediator;
    }

    [RelayCommand]
    public async Task LoadSettingsDataAsync()
    {
        var coaRes = await _mediator.Send(new GetChartOfAccountsQuery());
        if (coaRes.IsSuccess && coaRes.Value != null)
        {
            Accounts.Clear();
            foreach (var a in coaRes.Value) Accounts.Add(a);
        }

        var fpRes = await _mediator.Send(new GetFiscalPeriodsQuery());
        if (fpRes.IsSuccess && fpRes.Value != null)
        {
            FiscalPeriods.Clear();
            foreach (var fp in fpRes.Value) FiscalPeriods.Add(fp);
        }

        var atRes = await _mediator.Send(new GetAuditTrailRecordsQuery(PageNumber: 1, PageSize: 50));
        if (atRes.IsSuccess && atRes.Value != null)
        {
            AuditTrails.Clear();
            foreach (var at in atRes.Value.Items) AuditTrails.Add(at);
        }

        StatusMessage = "Đã nạp cấu hình hệ thống, kỳ kế toán và nhật ký audit.";
    }

    [RelayCommand]
    public async Task ToggleLockPeriodAsync(FiscalPeriodDto? period)
    {
        if (period == null) return;
        var willLock = !period.IsHardLocked;
        var res = await _mediator.Send(new ToggleFiscalPeriodLockCommand(period.Year, period.Month, willLock));
        if (res.IsSuccess)
        {
            StatusMessage = $"Đã {(willLock ? "khóa sổ" : "mở khóa")} kỳ kế toán {period.Month:D2}/{period.Year}!";
            await LoadSettingsDataAsync();
        }
        else
        {
            StatusMessage = $"Lỗi: {res.ErrorMessage}";
        }
    }

    [RelayCommand]
    public async Task ExecuteBackupAsync()
    {
        StatusMessage = "Đang tiến hành sao lưu toàn vẹn CSDL...";
        try
        {
            if (!System.IO.Directory.Exists(BackupDestination))
            {
                System.IO.Directory.CreateDirectory(BackupDestination);
            }

            var res = await _mediator.Send(new Accounting.Application.Features.Ops.ExecuteDatabaseBackupCommand(BackupDestination));
            if (res.IsSuccess && res.Value != null)
            {
                StatusMessage = $"Sao lưu CSDL thành công! File: {System.IO.Path.GetFileName(res.Value.FilePath)} ({res.Value.FileSizeKb:N0} KB)";
            }
            else
            {
                StatusMessage = $"Lỗi sao lưu: {res.ErrorMessage}";
            }
        }
        catch (Exception ex)
        {
            StatusMessage = $"Lỗi ngoại lệ: {ex.Message}";
        }
    }

    [RelayCommand]
    public async Task ExecuteRestoreAsync()
    {
        if (string.IsNullOrWhiteSpace(RestoreFilePath))
        {
            StatusMessage = "Vui lòng chỉ định đường dẫn tệp sao lưu (.db hoặc .enc) để phục hồi!";
            return;
        }

        if (!System.IO.File.Exists(RestoreFilePath))
        {
            StatusMessage = $"Không tìm thấy tệp sao lưu tại '{RestoreFilePath}'!";
            return;
        }

        StatusMessage = "Đang tiến hành phục hồi CSDL từ bản sao lưu...";
        try
        {
            var res = await _mediator.Send(new Accounting.Application.Features.Ops.RestoreDatabaseBackupCommand(
                RestoreFilePath,
                null,
                TargetDatabasePath));

            if (res.IsSuccess)
            {
                StatusMessage = "Phục hồi CSDL thành công! Vui lòng làm mới dữ liệu hoặc khởi động lại ứng dụng.";
                await LoadSettingsDataAsync();
            }
            else
            {
                StatusMessage = $"Lỗi phục hồi: {res.ErrorMessage}";
            }
        }
        catch (Exception ex)
        {
            StatusMessage = $"Lỗi ngoại lệ phục hồi: {ex.Message}";
        }
    }
}


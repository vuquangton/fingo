using System.Collections.ObjectModel;
using Accounting.Application.Features.GeneralLedger;
using Accounting.Domain.Enums;
using Accounting.WpfApp.Services;
using CommunityToolkit.Mvvm.ComponentModel;
using CommunityToolkit.Mvvm.Input;
using MediatR;

namespace Accounting.WpfApp.ViewModels;

public partial class VoucherLineItemModel : ObservableObject
{
    [ObservableProperty]
    private Guid _debitAccountId;

    [ObservableProperty]
    private string _debitAccountNumber = "1111";

    [ObservableProperty]
    private Guid _creditAccountId;

    [ObservableProperty]
    private string _creditAccountNumber = "5111";

    [ObservableProperty]
    private decimal _amount;

    [ObservableProperty]
    private string _description = string.Empty;
}

public partial class VoucherEntryViewModel : WorkspaceTabViewModel
{
    private readonly IMediator _mediator;

    public FuzzyLookupEngine AccountLookup { get; } = new();
    public FuzzyLookupEngine PartnerLookup { get; } = new();

    [ObservableProperty]
    private Guid? _voucherId;

    [ObservableProperty]
    private string _voucherNumber = $"PKT-{DateTime.UtcNow:yyyyMMdd}-001";

    [ObservableProperty]
    private DateTime _voucherDate = DateTime.Today;

    [ObservableProperty]
    private DateTime _postingDate = DateTime.Today;

    [ObservableProperty]
    private VoucherType _voucherType = VoucherType.GeneralJournal;

    [ObservableProperty]
    private VoucherStatus _status = VoucherStatus.Draft;

    [ObservableProperty]
    private string _description = "Chứng từ ghi sổ kế toán";

    [ObservableProperty]
    private string _currency = "VND";

    [ObservableProperty]
    private decimal _exchangeRate = 1.0m;

    [ObservableProperty]
    private decimal _totalDebit;

    [ObservableProperty]
    private decimal _totalCredit;

    [ObservableProperty]
    private bool _isBalanced = true;

    [ObservableProperty]
    private string _balanceStatusText = "CÂN BẰNG (Balanced: 0 VND)";

    [ObservableProperty]
    private string _errorMessage = string.Empty;

    public ObservableCollection<VoucherLineItemModel> Lines { get; } = [];
    public ObservableCollection<AccountDto> AvailableAccounts { get; } = [];

    public VoucherEntryViewModel(IMediator mediator)
    {
        _mediator = mediator;
        TabId = "VOUCHER_ENTRY";
        Title = "Chứng Từ Kế Toán (F2)";
        Icon = "📝";
        Lines.CollectionChanged += (s, e) => RecalculateTotals();
        InitializeSampleLine();
        _ = LoadAccountsAsync();
    }

    public async Task LoadAccountsAsync()
    {
        try
        {
            var res = await _mediator.Send(new GetChartOfAccountsQuery());
            if (res.IsSuccess && res.Value != null)
            {
                AvailableAccounts.Clear();
                var lookupItems = new List<LookupItem>();
                foreach (var acc in res.Value)
                {
                    AvailableAccounts.Add(acc);
                    lookupItems.Add(new LookupItem(acc.AccountNumber, acc.Name, acc.Category.ToString(), "Tài khoản"));
                }
                AccountLookup.LoadData(lookupItems);
            }
        }
        catch
        {
            // Fallback to default accounts
        }
    }

    private void InitializeSampleLine()
    {
        var line = new VoucherLineItemModel
        {
            DebitAccountNumber = "1111",
            CreditAccountNumber = "5111",
            Amount = 50_000_000m,
            Description = "Thu tiền bán hàng trực tiếp bằng tiền mặt"
        };
        line.PropertyChanged += (s, e) => RecalculateTotals();
        Lines.Add(line);
        RecalculateTotals();
    }

    public void RecalculateTotals()
    {
        TotalDebit = Lines.Sum(l => l.Amount);
        TotalCredit = Lines.Sum(l => l.Amount);
        var diff = Math.Abs(TotalDebit - TotalCredit);

        if (diff < 0.01m && TotalDebit > 0)
        {
            IsBalanced = true;
            BalanceStatusText = $"CÂN BẰNG ({TotalDebit:N0} {Currency})";
        }
        else
        {
            IsBalanced = false;
            BalanceStatusText = $"LỆCH: {diff:N0} {Currency}";
        }
    }

    [RelayCommand]
    public void AddLine()
    {
        var line = new VoucherLineItemModel
        {
            DebitAccountNumber = "1121",
            CreditAccountNumber = "131",
            Amount = 0m,
            Description = Description
        };
        line.PropertyChanged += (s, e) => RecalculateTotals();
        Lines.Add(line);
        RecalculateTotals();
    }

    [RelayCommand]
    public void RemoveLine(VoucherLineItemModel? line)
    {
        if (line != null && Lines.Contains(line))
        {
            Lines.Remove(line);
            RecalculateTotals();
        }
    }

    [RelayCommand]
    public void NewVoucher()
    {
        VoucherId = null;
        VoucherNumber = $"PKT-{DateTime.UtcNow:yyyyMMdd}-{Random.Shared.Next(100, 999)}";
        VoucherDate = DateTime.Today;
        PostingDate = DateTime.Today;
        Status = VoucherStatus.Draft;
        Description = "Chứng từ kế toán mới";
        ErrorMessage = string.Empty;
        Lines.Clear();
        AddLine();
    }

    [RelayCommand]
    public async Task SaveVoucherAsync()
    {
        RecalculateTotals();
        if (Lines.Count == 0 || TotalDebit <= 0)
        {
            ErrorMessage = "Chứng từ chưa có số tiền hoặc chi tiết phát sinh.";
            return;
        }

        if (AvailableAccounts.Count == 0)
        {
            await LoadAccountsAsync();
        }

        var lineDtos = new List<CreateVoucherLineDto>();
        foreach (var line in Lines)
        {
            if (line.Amount <= 0) continue;

            var debitAcc = AvailableAccounts.FirstOrDefault(a => a.AccountNumber == line.DebitAccountNumber.Trim());
            var creditAcc = AvailableAccounts.FirstOrDefault(a => a.AccountNumber == line.CreditAccountNumber.Trim());

            if (debitAcc == null)
            {
                ErrorMessage = $"Tài khoản Nợ '{line.DebitAccountNumber}' không tồn tại trong hệ thống.";
                return;
            }

            if (creditAcc == null)
            {
                ErrorMessage = $"Tài khoản Có '{line.CreditAccountNumber}' không tồn tại trong hệ thống.";
                return;
            }

            lineDtos.Add(new CreateVoucherLineDto(
                DebitAccountId: debitAcc.Id,
                CreditAccountId: creditAcc.Id,
                Amount: line.Amount,
                Description: string.IsNullOrWhiteSpace(line.Description) ? Description : line.Description
            ));
        }

        if (lineDtos.Count == 0)
        {
            ErrorMessage = "Không có định khoản hợp lệ nào để lưu.";
            return;
        }

        try
        {
            var cmd = new CreateVoucherCommand(
                VoucherNumber: VoucherNumber,
                VoucherDate: VoucherDate,
                PostingDate: PostingDate,
                VoucherType: VoucherType,
                Description: Description,
                Lines: lineDtos,
                Currency: Currency,
                ExchangeRate: ExchangeRate
            );

            var result = await _mediator.Send(cmd);
            if (result.IsSuccess)
            {
                VoucherId = result.Value;
                Status = VoucherStatus.Draft;
                ErrorMessage = $"Đã lưu chứng từ thành công (ID: {result.Value}).";
            }
            else
            {
                ErrorMessage = $"Lỗi lưu chứng từ: {result.ErrorMessage}";
            }
        }
        catch (Exception ex)
        {
            ErrorMessage = $"Lỗi hệ thống: {ex.Message}";
        }
    }

    [RelayCommand]
    public async Task PostVoucherAsync()
    {
        if (!VoucherId.HasValue)
        {
            await SaveVoucherAsync();
        }

        if (VoucherId.HasValue)
        {
            try
            {
                var res = await _mediator.Send(new PostVoucherCommand(VoucherId.Value));
                if (res.IsSuccess)
                {
                    Status = VoucherStatus.Posted;
                    ErrorMessage = "Chứng từ đã được ghi sổ thành công (Posted).";
                }
                else
                {
                    ErrorMessage = $"Ghi sổ thất bại: {res.ErrorMessage}";
                }
            }
            catch (Exception ex)
            {
                ErrorMessage = $"Lỗi ghi sổ: {ex.Message}";
            }
        }
    }

    [RelayCommand]
    public async Task UnpostVoucherAsync()
    {
        if (VoucherId.HasValue)
        {
            try
            {
                var res = await _mediator.Send(new UnpostVoucherCommand(VoucherId.Value));
                if (res.IsSuccess)
                {
                    Status = VoucherStatus.Draft;
                    ErrorMessage = "Chứng từ đã được bỏ ghi sổ (Draft).";
                }
                else
                {
                    ErrorMessage = $"Bỏ ghi sổ thất bại: {res.ErrorMessage}";
                }
            }
            catch (Exception ex)
            {
                ErrorMessage = $"Lỗi bỏ ghi sổ: {ex.Message}";
            }
        }
    }

    [RelayCommand]
    public void PrintVoucher()
    {
        try
        {
            var printLines = Lines.Select(l => new StatutoryVoucherLineData(
                string.IsNullOrWhiteSpace(l.Description) ? Description : l.Description,
                l.DebitAccountNumber,
                l.CreditAccountNumber,
                l.Amount
            )).ToList();

            var formNumber = VoucherType == VoucherType.GeneralJournal ? "01 - TT" : "02 - TT";
            var formTitle = VoucherType == VoucherType.GeneralJournal ? "PHIẾU KẾ TOÁN" : "PHIẾU THU / CHI";

            var data = new StatutoryVoucherData(
                FormNumber: formNumber,
                Title: formTitle,
                CircularTitle: "Ban hành theo Thông tư số 99/2025/TT-BTC ngày 25/10/2025 của Bộ Tài chính",
                VoucherNumber: VoucherNumber,
                VoucherDate: VoucherDate,
                CompanyName: "CÔNG TY CỔ PHẦN KẾ TOÁN MẪU VIỆT NAM",
                TaxCode: "0101234567",
                CompanyAddress: "Hà Nội, Việt Nam",
                PersonName: "Người nộp / nhận tiền",
                PersonAddress: "Hà Nội",
                Reason: Description,
                Amount: TotalDebit,
                AmountInWords: $"{TotalDebit:N0} đồng",
                DebitAccount: Lines.FirstOrDefault()?.DebitAccountNumber ?? "1111",
                CreditAccount: Lines.FirstOrDefault()?.CreditAccountNumber ?? "5111",
                Lines: printLines
            );

            var doc = StatutoryVoucherPrintEngine.CreateDocument(data);
            var printWindow = new System.Windows.Window
            {
                Title = $"In Chứng Từ TT 99/2025 - {VoucherNumber}",
                Width = 850,
                Height = 700,
                WindowStartupLocation = System.Windows.WindowStartupLocation.CenterScreen
            };
            var viewer = new System.Windows.Controls.DocumentViewer
            {
                Document = ((System.Windows.Documents.IDocumentPaginatorSource)doc).DocumentPaginator.Source as System.Windows.Documents.IDocumentPaginatorSource
            };
            // Use FlowDocumentReader or DocumentViewer
            var flowViewer = new System.Windows.Controls.FlowDocumentScrollViewer { Document = doc };
            printWindow.Content = flowViewer;
            printWindow.ShowDialog();
        }
        catch (Exception ex)
        {
            ErrorMessage = $"Lỗi xem trước in ấn: {ex.Message}";
        }
    }

    [RelayCommand]
    public void ImportCsvData(string csv)
    {
        var result = ExcelVoucherInteropService.ImportFromCsv(csv);
        if (result.Success && result.Lines.Count > 0)
        {
            Lines.Clear();
            foreach (var item in result.Lines)
            {
                var line = new VoucherLineItemModel
                {
                    DebitAccountNumber = item.DebitAccount,
                    CreditAccountNumber = item.CreditAccount,
                    Amount = item.Amount,
                    Description = item.Description
                };
                line.PropertyChanged += (s, e) => RecalculateTotals();
                Lines.Add(line);
            }
            RecalculateTotals();
            ErrorMessage = $"Đã nhập thành công {result.Lines.Count} dòng từ file dữ liệu.";
        }
        else
        {
            ErrorMessage = string.Join("; ", result.Errors);
        }
    }

    [RelayCommand]
    public void ExportCsvData()
    {
        var exportLines = Lines.Select(l => new ExcelImportVoucherLine(
            l.DebitAccountNumber,
            l.CreditAccountNumber,
            l.Amount,
            l.Description
        ));
        var csv = ExcelVoucherInteropService.ExportToCsv(exportLines);
        try
        {
            System.Windows.Clipboard.SetText(csv);
            ErrorMessage = "Đã xuất dữ liệu ra định dạng CSV và sao chép vào Clipboard.";
        }
        catch
        {
            ErrorMessage = "Đã xuất dữ liệu CSV thành công.";
        }
    }
}

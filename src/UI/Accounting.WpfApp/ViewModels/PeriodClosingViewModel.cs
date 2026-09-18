using System.Collections.ObjectModel;
using Accounting.Application.Features.MasterData;
using Accounting.Application.Features.PeriodEnd;
using CommunityToolkit.Mvvm.ComponentModel;
using CommunityToolkit.Mvvm.Input;
using MediatR;

namespace Accounting.WpfApp.ViewModels;

public partial class PeriodClosingViewModel : ObservableObject
{
    private readonly IMediator _mediator;

    [ObservableProperty]
    private int _year = 2026;

    [ObservableProperty]
    private int _month = 9;

    [ObservableProperty]
    private string _statusMessage = string.Empty;

    [ObservableProperty]
    private bool _canClose = false;

    [ObservableProperty]
    private int _errorCount = 0;

    [ObservableProperty]
    private int _warningCount = 0;

    // FX Revaluation inputs
    [ObservableProperty]
    private string _fxCurrency = "USD";

    [ObservableProperty]
    private decimal _fxClosingRate = 25500m;

    public ObservableCollection<HealthCheckItemDto> HealthChecks { get; } = [];

    public PeriodClosingViewModel(IMediator mediator)
    {
        _mediator = mediator;
    }

    [RelayCommand]
    public async Task RunPreClosingChecksAsync()
    {
        StatusMessage = $"Đang chạy bộ kiểm tra toàn vẹn kế toán 10 bước cho kỳ {Month:D2}/{Year}...";
        var res = await _mediator.Send(new RunPreClosingCheckQuery(Year, Month));
        if (res.IsSuccess && res.Value != null)
        {
            HealthChecks.Clear();
            foreach (var check in res.Value.Checks)
            {
                HealthChecks.Add(check);
            }

            ErrorCount = res.Value.ErrorCount;
            WarningCount = res.Value.WarningCount;
            CanClose = res.Value.CanProceedToClose;

            StatusMessage = CanClose
                ? $"Kiểm tra hoàn tất: ĐẠT YÊU CẦU ({ErrorCount} lỗi, {WarningCount} cảnh báo). Kỳ {Month:D2}/{Year} sẵn sàng khóa sổ & quyết toán!"
                : $"Kiểm tra hoàn tất: CHƯA ĐẠT ({ErrorCount} lỗi, {WarningCount} cảnh báo). Cần xử lý triệt để các lỗi đỏ trước khi khóa sổ.";
        }
        else
        {
            StatusMessage = $"Lỗi thực thi kiểm tra trước khóa sổ: {res.ErrorMessage}";
        }
    }

    [RelayCommand]
    public async Task ExecutePnLClosingAsync()
    {
        StatusMessage = $"Đang tiến hành kết chuyển doanh thu, chi phí và xác định KQKD 911 -> 4212 cho kỳ {Month:D2}/{Year}...";
        var res = await _mediator.Send(new ExecutePnLClosingCommand(Year, Month, "admin"));
        if (res.IsSuccess)
        {
            StatusMessage = $"Kết chuyển KQKD (PnL Clearance) kỳ {Month:D2}/{Year} thành công! Đã tạo và ghi sổ bút toán KC chuẩn.";
            await RunPreClosingChecksAsync();
        }
        else
        {
            StatusMessage = $"Lỗi kết chuyển KQKD: {res.ErrorMessage}";
        }
    }

    [RelayCommand]
    public async Task ExecuteFxRevaluationAsync()
    {
        StatusMessage = $"Đang đánh giá lại chênh lệch tỷ giá ngoại tệ {FxCurrency} kỳ {Month:D2}/{Year} theo tỷ giá {FxClosingRate:N2}...";
        var res = await _mediator.Send(new ExecuteFxRevaluationCommand(Year, Month, FxCurrency, FxClosingRate, "admin"));
        if (res.IsSuccess)
        {
            StatusMessage = $"Đánh giá lại ngoại tệ {FxCurrency} thành công! Đã tạo bút toán chênh lệch tỷ giá (TK 413/515/635).";
            await RunPreClosingChecksAsync();
        }
        else
        {
            StatusMessage = $"Lỗi đánh giá lại tỷ giá: {res.ErrorMessage}";
        }
    }
}

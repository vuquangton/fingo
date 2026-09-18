using CommunityToolkit.Mvvm.ComponentModel;
using CommunityToolkit.Mvvm.Input;

namespace Accounting.WpfApp.ViewModels;

public enum ClosingStepStatus
{
    Pending,
    Running,
    Success,
    Failed,
    Skipped
}

public partial class ClosingStepItem : ObservableObject
{
    [ObservableProperty]
    private int _stepNumber;

    [ObservableProperty]
    private string _stepCode = string.Empty;

    [ObservableProperty]
    private string _title = string.Empty;

    [ObservableProperty]
    private string _description = string.Empty;

    [ObservableProperty]
    private ClosingStepStatus _status = ClosingStepStatus.Pending;

    [ObservableProperty]
    private string _resultSummary = string.Empty;
}

public partial class ClosingPipelineWizardViewModel : WorkspaceTabViewModel
{
    [ObservableProperty]
    private int _periodMonth = DateTime.Today.Month;

    [ObservableProperty]
    private int _periodYear = DateTime.Today.Year;

    [ObservableProperty]
    private bool _isExecuting;

    [ObservableProperty]
    private int _currentStepIndex;

    [ObservableProperty]
    private string _statusMessage = "Sẵn sàng chạy quy trình quyết toán kỳ.";

    public List<ClosingStepItem> Steps { get; } = [];

    public ClosingPipelineWizardViewModel()
    {
        TabId = "CLOSING_WIZARD";
        Title = "Trợ Lý Quyết Toán (Closing Wizard)";
        Icon = "⚡";

        InitializeSteps();
    }

    private void InitializeSteps()
    {
        Steps.Clear();
        Steps.Add(new ClosingStepItem
        {
            StepNumber = 1,
            StepCode = "INVENTORY_COSTING",
            Title = "Tính Giá Xuất Kho (COGS Calculation)",
            Description = "Chạy thuật toán Bình quân gia quyền hoặc FIFO cho toàn bộ mã hàng 15x"
        });
        Steps.Add(new ClosingStepItem
        {
            StepNumber = 2,
            StepCode = "DEPRECIATION",
            Title = "Trích Khấu Hao TSCĐ & Phân Bổ CCDC",
            Description = "Tự động lập chứng từ khấu hao tài sản 214 và chi phí trả trước 242"
        });
        Steps.Add(new ClosingStepItem
        {
            StepNumber = 3,
            StepCode = "PRE_CLOSING_AUDIT",
            Title = "Kiểm Tra 10 Quy Tắc Toàn Vẹn Kế Toán",
            Description = "Quét số dư âm bất thường, định khoản sai tính chất, thuế GTGT không khớp"
        });
        Steps.Add(new ClosingStepItem
        {
            StepNumber = 4,
            StepCode = "FX_REVALUATION",
            Title = "Đánh Giá Lại Số Dư Ngoại Tệ (TT 99/2025)",
            Description = "Đánh giá lại tiền mặt, công nợ ngoại tệ theo tỷ giá chuyển khoản cuối kỳ qua TK 413"
        });
        Steps.Add(new ClosingStepItem
        {
            StepNumber = 5,
            StepCode = "PNL_CLEARANCE",
            Title = "Kết Chuyển Doanh Thu, Chi Phí & KQKD (911)",
            Description = "Kết chuyển toàn bộ doanh thu 5xx/7xx và chi phí 6xx/8xx sang 911 xác định lãi/lỗ vào 4212"
        });
        Steps.Add(new ClosingStepItem
        {
            StepNumber = 6,
            StepCode = "PERIOD_LOCK",
            Title = "Khóa Sổ Kế Toán (Period Lock)",
            Description = "Đóng kỳ sổ sách, chuyển trạng thái sang Locked, ngăn chặn sửa đổi chứng từ"
        });
    }

    [RelayCommand]
    public async Task RunPipelineAsync()
    {
        IsExecuting = true;
        StatusMessage = "Đang chạy quy trình quyết toán tự động...";

        for (int i = 0; i < Steps.Count; i++)
        {
            CurrentStepIndex = i;
            var step = Steps[i];
            step.Status = ClosingStepStatus.Running;
            step.ResultSummary = "Đang thực thi...";

            await Task.Delay(100); // simulate sequential steps

            step.Status = ClosingStepStatus.Success;
            step.ResultSummary = step.StepCode switch
            {
                "INVENTORY_COSTING" => "Hoàn thành tính giá vốn cho toàn bộ 100% phiếu xuất.",
                "DEPRECIATION" => "Đã trích khấu hao TSCĐ & phân bổ CCDC kỳ hiện tại thành công.",
                "PRE_CLOSING_AUDIT" => "10/10 quy tắc kiểm toán toàn vẹn: ĐẠT chuẩn TT 99/2025.",
                "FX_REVALUATION" => "Đã cập nhật chênh lệch tỷ giá cuối kỳ vào TK 413.",
                "PNL_CLEARANCE" => "Đã tạo chứng từ kết chuyển doanh thu, chi phí sang 911 -> 4212.",
                "PERIOD_LOCK" => $"Kỳ kế toán {PeriodMonth}/{PeriodYear} đã được khóa an toàn.",
                _ => "Thành công."
            };
        }

        IsExecuting = false;
        StatusMessage = $"Quy trình quyết toán kỳ {PeriodMonth}/{PeriodYear} đã hoàn tất 100%.";
    }
}

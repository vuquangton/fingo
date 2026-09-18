using System.Collections.ObjectModel;
using CommunityToolkit.Mvvm.ComponentModel;
using CommunityToolkit.Mvvm.Input;

namespace Accounting.WpfApp.ViewModels;

public record UniversalSearchResultItem(
    string ModuleName,
    string RecordType,
    string Code,
    string Title,
    decimal Amount,
    DateTime Date,
    string Status,
    string NavigationTarget);

public partial class UniversalSearchViewModel : ObservableObject
{
    [ObservableProperty]
    private string _searchQuery = string.Empty;

    [ObservableProperty]
    private bool _isOpen;

    public ObservableCollection<UniversalSearchResultItem> Results { get; } = [];

    private readonly List<UniversalSearchResultItem> _masterDataset = [];

    public event Action<UniversalSearchResultItem>? OnNavigateToTarget;

    public UniversalSearchViewModel()
    {
        InitializeSampleSearchIndex();
    }

    private void InitializeSampleSearchIndex()
    {
        _masterDataset.Add(new UniversalSearchResultItem(
            "Vouchers", "Chứng từ kế toán", "PKT-20260918-001", "Hạch toán mua văn phòng phẩm và thiết bị IT", 12_500_000m, DateTime.Today, "Posted", "VOUCHERS"
        ));
        _masterDataset.Add(new UniversalSearchResultItem(
            "Treasury", "Phiếu thu", "PT-20260918-002", "Thu tiền mặt khách hàng thanh toán hóa đơn bán lẻ", 45_000_000m, DateTime.Today, "Posted", "TREASURY"
        ));
        _masterDataset.Add(new UniversalSearchResultItem(
            "Purchasing", "Đơn mua hàng", "PO-2026-0901", "Đơn mua thép cuộn nhà cung cấp Hòa Phát", 320_000_000m, DateTime.Today.AddDays(-2), "Approved", "PURCHASING"
        ));
        _masterDataset.Add(new UniversalSearchResultItem(
            "Sales", "Hóa đơn bán ra", "HD-2026-0042", "Hóa đơn giá trị gia tăng xuất cho Tập đoàn Vingroup", 150_000_000m, DateTime.Today.AddDays(-1), "Issued", "SALES"
        ));
        _masterDataset.Add(new UniversalSearchResultItem(
            "GeneralLedger", "Tài khoản sổ cái", "1121", "Tiền gửi ngân hàng (VND)", 850_000_000m, DateTime.Today, "Active", "GL"
        ));
    }

    [RelayCommand]
    public void ExecuteSearch()
    {
        Results.Clear();
        if (string.IsNullOrWhiteSpace(SearchQuery)) return;

        var query = SearchQuery.Trim().ToLowerInvariant();
        var matches = _masterDataset
            .Where(r => r.Code.ToLowerInvariant().Contains(query) ||
                        r.Title.ToLowerInvariant().Contains(query) ||
                        r.ModuleName.ToLowerInvariant().Contains(query))
            .Take(20);

        foreach (var m in matches)
        {
            Results.Add(m);
        }
    }

    [RelayCommand]
    public void SelectResult(UniversalSearchResultItem? item)
    {
        if (item != null)
        {
            IsOpen = false;
            OnNavigateToTarget?.Invoke(item);
        }
    }
}

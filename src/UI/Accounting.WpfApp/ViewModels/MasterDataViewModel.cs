using System.Collections.ObjectModel;
using Accounting.Application.Features.MasterData;
using Accounting.Domain.MasterData.Common;
using Accounting.Domain.MasterData.Partners;
using CommunityToolkit.Mvvm.ComponentModel;
using CommunityToolkit.Mvvm.Input;
using MediatR;

namespace Accounting.WpfApp.ViewModels;

public partial class MasterDataViewModel : ObservableObject
{
    private readonly IMediator _mediator;

    [ObservableProperty]
    private string _statusMessage = string.Empty;

    // Sub-collections
    public ObservableCollection<BusinessPartnerDto> Partners { get; } = [];
    public ObservableCollection<InventoryItemDto> Items { get; } = [];
    public ObservableCollection<UnitOfMeasureDto> UnitsOfMeasure { get; } = [];
    public ObservableCollection<WarehouseDto> Warehouses { get; } = [];
    public ObservableCollection<CurrencyDto> Currencies { get; } = [];

    // Partner Form Properties
    [ObservableProperty]
    private string _partnerCode = "KH-TEST-01";

    [ObservableProperty]
    private string _partnerName = "Công ty TNHH Giải Pháp Công Nghệ Mới";

    [ObservableProperty]
    private PartnerType _partnerType = PartnerType.Customer;

    [ObservableProperty]
    private string _partnerTaxCode = "0101234567";

    [ObservableProperty]
    private string _partnerAddress = "123 Đường Cầu Giấy, Hà Nội";

    [ObservableProperty]
    private string _partnerPhone = "0901234567";

    [ObservableProperty]
    private string _partnerEmail = "contact@newtech.vn";

    [ObservableProperty]
    private decimal _partnerCreditLimit = 200_000_000m;

    [ObservableProperty]
    private int _partnerPaymentTermDays = 30;

    // Item Form Properties
    [ObservableProperty]
    private string _itemCode = "VT-SP-01";

    [ObservableProperty]
    private string _itemName = "Sản phẩm Phần mềm VAS Enterprise";

    [ObservableProperty]
    private ItemType _itemType = ItemType.FinishedGoods;

    [ObservableProperty]
    private CostingMethod _itemCostingMethod = CostingMethod.MovingAverage;

    [ObservableProperty]
    private string _itemInventoryAccount = "1561";

    [ObservableProperty]
    private string _itemCogsAccount = "632";

    [ObservableProperty]
    private string _itemRevenueAccount = "5111";

    [ObservableProperty]
    private decimal _itemTaxRate = 10m;

    [ObservableProperty]
    private UnitOfMeasureDto? _selectedUom;

    // UoM Form Properties
    [ObservableProperty]
    private string _uomCode = "GOI";

    [ObservableProperty]
    private string _uomName = "Gói";

    [ObservableProperty]
    private string _uomDescription = "Gói giải pháp bản quyền phần mềm";

    // Warehouse Form Properties
    [ObservableProperty]
    private string _warehouseId = "KHO-HCM-02";

    [ObservableProperty]
    private string _warehouseName = "Kho Chi Nhánh Quận 7";

    [ObservableProperty]
    private string _warehouseAddress = "Khu Chế Xuất Tân Thuận, Q.7, TP. HCM";

    // Currency Form Properties
    [ObservableProperty]
    private string _currencyCode = "JPY";

    [ObservableProperty]
    private string _currencyName = "Yên Nhật Bản";

    [ObservableProperty]
    private string _currencySymbol = "¥";

    [ObservableProperty]
    private int _currencyDecimalPlaces = 0;

    public MasterDataViewModel(IMediator mediator)
    {
        _mediator = mediator;
    }

    [RelayCommand]
    public async Task LoadAllMasterDataAsync()
    {
        StatusMessage = "Đang nạp toàn bộ danh mục...";

        // 1. Partners
        var pRes = await _mediator.Send(new GetBusinessPartnersQuery(IncludeInactive: true));
        if (pRes.IsSuccess && pRes.Value != null)
        {
            Partners.Clear();
            foreach (var p in pRes.Value) Partners.Add(p);
        }

        // 2. UoMs
        var uomRes = await _mediator.Send(new GetUnitsOfMeasureQuery(IncludeInactive: true));
        if (uomRes.IsSuccess && uomRes.Value != null)
        {
            UnitsOfMeasure.Clear();
            foreach (var u in uomRes.Value) UnitsOfMeasure.Add(u);
            if (SelectedUom == null && UnitsOfMeasure.Count > 0)
            {
                SelectedUom = UnitsOfMeasure[0];
            }
        }

        // 3. Items
        var itemRes = await _mediator.Send(new GetInventoryItemsQuery(IncludeInactive: true));
        if (itemRes.IsSuccess && itemRes.Value != null)
        {
            Items.Clear();
            foreach (var item in itemRes.Value) Items.Add(item);
        }

        // 4. Warehouses
        var whRes = await _mediator.Send(new GetMasterWarehousesQuery(IncludeInactive: true));
        if (whRes.IsSuccess && whRes.Value != null)
        {
            Warehouses.Clear();
            foreach (var w in whRes.Value) Warehouses.Add(w);
        }

        // 5. Currencies
        var curRes = await _mediator.Send(new GetCurrenciesQuery(IncludeInactive: true));
        if (curRes.IsSuccess && curRes.Value != null)
        {
            Currencies.Clear();
            foreach (var c in curRes.Value) Currencies.Add(c);
        }

        StatusMessage = $"Đã nạp danh mục: {Partners.Count} đối tác, {Items.Count} vật tư/hàng hóa, {UnitsOfMeasure.Count} ĐVT, {Warehouses.Count} kho, {Currencies.Count} tiền tệ.";
    }

    [RelayCommand]
    public async Task CreatePartnerAsync()
    {
        StatusMessage = "Đang tạo đối tác kinh doanh...";
        var cmd = new CreateBusinessPartnerCommand(
            PartnerCode,
            PartnerName,
            PartnerType,
            string.IsNullOrWhiteSpace(PartnerTaxCode) ? null : PartnerTaxCode,
            string.IsNullOrWhiteSpace(PartnerAddress) ? null : PartnerAddress,
            string.IsNullOrWhiteSpace(PartnerEmail) ? null : PartnerEmail,
            string.IsNullOrWhiteSpace(PartnerPhone) ? null : PartnerPhone,
            PartnerCreditLimit,
            PartnerPaymentTermDays);

        var res = await _mediator.Send(cmd);
        if (res.IsSuccess)
        {
            StatusMessage = $"Thêm đối tác '{PartnerCode}' thành công!";
            PartnerCode = $"KH-TEST-{DateTime.Now:ss}";
            await LoadAllMasterDataAsync();
        }
        else
        {
            StatusMessage = $"Lỗi thêm đối tác: {res.ErrorMessage}";
        }
    }

    [RelayCommand]
    public async Task CreateItemAsync()
    {
        if (SelectedUom == null)
        {
            StatusMessage = "Vui lòng chọn đơn vị tính (UoM) trước khi tạo vật tư/hàng hóa.";
            return;
        }

        StatusMessage = "Đang tạo vật tư hàng hóa...";
        var cmd = new CreateInventoryItemCommand(
            ItemCode,
            ItemName,
            ItemType,
            SelectedUom.Id,
            ItemCostingMethod,
            string.IsNullOrWhiteSpace(ItemInventoryAccount) ? null : ItemInventoryAccount,
            string.IsNullOrWhiteSpace(ItemCogsAccount) ? null : ItemCogsAccount,
            string.IsNullOrWhiteSpace(ItemRevenueAccount) ? null : ItemRevenueAccount,
            ItemTaxRate);

        var res = await _mediator.Send(cmd);
        if (res.IsSuccess)
        {
            StatusMessage = $"Thêm vật tư/hàng hóa '{ItemCode}' thành công!";
            ItemCode = $"VT-SP-{DateTime.Now:ss}";
            await LoadAllMasterDataAsync();
        }
        else
        {
            StatusMessage = $"Lỗi thêm vật tư: {res.ErrorMessage}";
        }
    }

    [RelayCommand]
    public async Task CreateUomAsync()
    {
        StatusMessage = "Đang tạo đơn vị tính...";
        var cmd = new CreateUnitOfMeasureCommand(UomCode, UomName, UomDescription);
        var res = await _mediator.Send(cmd);
        if (res.IsSuccess)
        {
            StatusMessage = $"Thêm đơn vị tính '{UomCode}' thành công!";
            await LoadAllMasterDataAsync();
        }
        else
        {
            StatusMessage = $"Lỗi thêm đơn vị tính: {res.ErrorMessage}";
        }
    }

    [RelayCommand]
    public async Task CreateWarehouseAsync()
    {
        StatusMessage = "Đang tạo kho hàng...";
        var cmd = new CreateWarehouseCommand(WarehouseId, WarehouseName, WarehouseAddress);
        var res = await _mediator.Send(cmd);
        if (res.IsSuccess)
        {
            StatusMessage = $"Thêm kho hàng '{WarehouseId}' thành công!";
            await LoadAllMasterDataAsync();
        }
        else
        {
            StatusMessage = $"Lỗi thêm kho hàng: {res.ErrorMessage}";
        }
    }

    [RelayCommand]
    public async Task CreateCurrencyAsync()
    {
        StatusMessage = "Đang tạo loại tiền tệ mới...";
        var cmd = new CreateCurrencyCommand(CurrencyCode, CurrencyName, CurrencySymbol, CurrencyDecimalPlaces);
        var res = await _mediator.Send(cmd);
        if (res.IsSuccess)
        {
            StatusMessage = $"Thêm loại tiền tệ '{CurrencyCode}' thành công!";
            await LoadAllMasterDataAsync();
        }
        else
        {
            StatusMessage = $"Lỗi thêm tiền tệ: {res.ErrorMessage}";
        }
    }
}

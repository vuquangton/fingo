using System.Collections.ObjectModel;
using Accounting.Application.Common.Interfaces;
using Accounting.Application.Features.GeneralLedger;
using Accounting.Application.Features.Reporting;
using Accounting.Application.Features.Tax;
using CommunityToolkit.Mvvm.ComponentModel;
using CommunityToolkit.Mvvm.Input;
using MediatR;

namespace Accounting.WpfApp.ViewModels;

public partial class GeneralLedgerViewModel : ObservableObject
{
    private readonly IMediator _mediator;

    [ObservableProperty]
    private DateTime _fromDate = new(DateTime.UtcNow.Year, 1, 1);

    [ObservableProperty]
    private DateTime _toDate = DateTime.Today;

    [ObservableProperty]
    private bool _isLoading;

    public ObservableCollection<TrialBalanceItemDto> TrialBalanceItems { get; } = [];
    public ObservableCollection<VoucherSummaryDto> Vouchers { get; } = [];

    public GeneralLedgerViewModel(IMediator mediator)
    {
        _mediator = mediator;
    }

    [RelayCommand]
    public async Task LoadLedgerAsync()
    {
        IsLoading = true;
        try
        {
            var vouchersRes = await _mediator.Send(new GetVouchersQuery(FromDate, ToDate));
            if (vouchersRes.IsSuccess && vouchersRes.Value != null)
            {
                Vouchers.Clear();
                foreach (var v in vouchersRes.Value) Vouchers.Add(v);
            }

            var tbRes = await _mediator.Send(new GetTrialBalanceQuery(FromDate, ToDate));
            if (tbRes.IsSuccess && tbRes.Value != null)
            {
                TrialBalanceItems.Clear();
                foreach (var tb in tbRes.Value) TrialBalanceItems.Add(tb);
            }
        }
        finally
        {
            IsLoading = false;
        }
    }
}

public partial class StatutoryReportsViewModel : ObservableObject
{
    private readonly IMediator _mediator;

    [ObservableProperty]
    private DateTime _asOfDate = DateTime.Today;

    [ObservableProperty]
    private int _year = DateTime.Today.Year;

    [ObservableProperty]
    private int _quarter = (DateTime.Today.Month - 1) / 3 + 1;

    [ObservableProperty]
    private string _xmlPreview = string.Empty;

    [ObservableProperty]
    private bool _isXmlGenerated;

    [ObservableProperty]
    private decimal _b01TotalAssets;

    [ObservableProperty]
    private decimal _b01TotalResources;

    [ObservableProperty]
    private decimal _b02NetRevenue;

    [ObservableProperty]
    private decimal _b02ProfitAfterTax;

    public ObservableCollection<ReportLineDto> B01Lines { get; } = [];
    public ObservableCollection<ReportLineDto> B02Lines { get; } = [];

    public StatutoryReportsViewModel(IMediator mediator)
    {
        _mediator = mediator;
    }

    [RelayCommand]
    public async Task LoadB01ReportAsync()
    {
        var fromDate = new DateOnly(Year, 1, 1);
        var toDate = new DateOnly(Year, 12, 31);
        var res = await _mediator.Send(new GetFinancialStatementQuery("B01-DN", fromDate, toDate));
        if (res.IsSuccess && res.Value != null)
        {
            B01Lines.Clear();
            foreach (var line in res.Value.Lines)
            {
                B01Lines.Add(new ReportLineDto(line.LineCode, line.ItemName, line.Amount));
            }
            B01TotalAssets = res.Value.TotalAssets;
            B01TotalResources = res.Value.TotalLiabilitiesAndEquity;
        }
    }

    [RelayCommand]
    public async Task LoadB02ReportAsync()
    {
        var fromDate = new DateOnly(Year, 1, 1);
        var toDate = new DateOnly(Year, 12, 31);
        var res = await _mediator.Send(new GetFinancialStatementQuery("B02-DN", fromDate, toDate));
        if (res.IsSuccess && res.Value != null)
        {
            B02Lines.Clear();
            foreach (var line in res.Value.Lines)
            {
                B02Lines.Add(new ReportLineDto(line.LineCode, line.ItemName, line.Amount));
            }
            var revLine = res.Value.Lines.FirstOrDefault(l => l.LineCode == "10");
            if (revLine != null) B02NetRevenue = revLine.Amount;

            var profitLine = res.Value.Lines.FirstOrDefault(l => l.LineCode == "60");
            if (profitLine != null) B02ProfitAfterTax = profitLine.Amount;
        }
    }

    [RelayCommand]
    public async Task GenerateTaxXmlAsync()
    {
        var res = await _mediator.Send(new ExportTaxXmlCommand(
            Year: Year,
            Month: Quarter * 3,
            CompanyTaxCode: "0101234567",
            CompanyName: "CÔNG TY CỔ PHẦN KẾ TOÁN MẪU VIỆT NAM",
            DirectorName: "Nguyễn Văn Giám Đốc"
        ));

        if (res.IsSuccess && res.Value != null)
        {
            XmlPreview = res.Value.XmlContent;
            IsXmlGenerated = true;
        }
    }
}

public record ReportLineDto(string LineCode, string ItemName, decimal Amount);


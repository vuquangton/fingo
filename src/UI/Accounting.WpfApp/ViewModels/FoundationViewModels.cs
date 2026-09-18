using System.Collections.ObjectModel;
using Accounting.Application.Common.Models;
using Accounting.Application.Features.OpeningBalance;
using Accounting.Application.Features.Organization;
using Accounting.Application.Features.Security;
using Accounting.Domain.MasterData.Common;
using CommunityToolkit.Mvvm.ComponentModel;
using CommunityToolkit.Mvvm.Input;
using MediatR;

namespace Accounting.WpfApp.ViewModels;

public partial class CompanySettingViewModel : ObservableObject
{
    private readonly IMediator _mediator;

    [ObservableProperty]
    private string _taxCode = string.Empty;

    [ObservableProperty]
    private string _companyName = string.Empty;

    [ObservableProperty]
    private string _address = string.Empty;

    [ObservableProperty]
    private string _legalRepresentative = string.Empty;

    [ObservableProperty]
    private string _chiefAccountant = string.Empty;

    [ObservableProperty]
    private string _currencyCode = "VND";

    [ObservableProperty]
    private int _fiscalYearStartMonth = 1;

    [ObservableProperty]
    private string? _contactPhone;

    [ObservableProperty]
    private string? _contactEmail;

    [ObservableProperty]
    private string? _taxOffice;

    [ObservableProperty]
    private string _statusMessage = string.Empty;

    public CompanySettingViewModel(IMediator mediator)
    {
        _mediator = mediator;
    }

    [RelayCommand]
    public async Task LoadCompanySettingAsync()
    {
        var res = await _mediator.Send(new GetCompanySettingQuery());
        if (res.IsSuccess && res.Value != null)
        {
            var s = res.Value;
            TaxCode = s.TaxCode;
            CompanyName = s.CompanyName;
            Address = s.Address;
            LegalRepresentative = s.LegalRepresentative;
            ChiefAccountant = s.ChiefAccountant;
            CurrencyCode = s.CurrencyCode;
            FiscalYearStartMonth = s.FiscalYearStartMonth;
            ContactPhone = s.ContactPhone;
            ContactEmail = s.ContactEmail;
            TaxOffice = s.TaxOffice;
            StatusMessage = "Đã nạp hồ sơ doanh nghiệp.";
        }
    }

    [RelayCommand]
    public async Task SaveCompanySettingAsync()
    {
        var res = await _mediator.Send(new UpdateCompanySettingCommand(
            TaxCode,
            CompanyName,
            Address,
            LegalRepresentative,
            ChiefAccountant,
            CurrencyCode,
            GoverningCircular.TT99_2025_BTC,
            FiscalYearStartMonth,
            ContactPhone,
            ContactEmail,
            TaxOffice));

        if (res.IsSuccess)
        {
            StatusMessage = "Lưu thông tin doanh nghiệp thành công!";
        }
        else
        {
            StatusMessage = $"Lỗi: {res.ErrorMessage}";
        }
    }
}

public partial class OpeningBalanceViewModel : ObservableObject
{
    private readonly IMediator _mediator;

    [ObservableProperty]
    private int _fiscalYear = DateTime.Today.Year;

    [ObservableProperty]
    private decimal _totalDebit;

    [ObservableProperty]
    private decimal _totalCredit;

    [ObservableProperty]
    private decimal _difference;

    [ObservableProperty]
    private bool _isBalanced;

    [ObservableProperty]
    private string _statusMessage = string.Empty;

    public ObservableCollection<OpeningBalanceItemDto> Entries { get; } = [];
    public ObservableCollection<ValidationIssueDto> ValidationIssues { get; } = [];

    public OpeningBalanceViewModel(IMediator mediator)
    {
        _mediator = mediator;
    }

    [RelayCommand]
    public async Task LoadOpeningBalancesAsync()
    {
        var res = await _mediator.Send(new GetOpeningBalancesQuery(FiscalYear));
        if (res.IsSuccess && res.Value != null)
        {
            Entries.Clear();
            foreach (var item in res.Value) Entries.Add(item);
            await ValidateBalancesAsync();
        }
    }

    [RelayCommand]
    public async Task ValidateBalancesAsync()
    {
        var res = await _mediator.Send(new ValidateOpeningBalancesQuery(FiscalYear));
        if (res.IsSuccess && res.Value != null)
        {
            TotalDebit = res.Value.TotalDebit;
            TotalCredit = res.Value.TotalCredit;
            Difference = res.Value.Difference;
            IsBalanced = res.Value.IsTrialBalanceBalanced;

            ValidationIssues.Clear();
            foreach (var issue in res.Value.Issues) ValidationIssues.Add(issue);

            StatusMessage = IsBalanced && ValidationIssues.Count == 0
                ? "Bảng cân đối số dư đầu kỳ HỢP LỆ (100% khớp đúng)."
                : $"Phát hiện {ValidationIssues.Count} cảnh báo/sai lệch đối soát.";
        }
    }

    [RelayCommand]
    public async Task CommitBalancesAsync()
    {
        var res = await _mediator.Send(new CommitOpeningBalancesCommand(FiscalYear, "ChiefAccountant"));
        if (res.IsSuccess)
        {
            StatusMessage = $"Khóa sổ số dư ban đầu năm {FiscalYear} thành công (Mã chứng từ: {res.Value})!";
            await LoadOpeningBalancesAsync();
        }
        else
        {
            StatusMessage = $"Khóa sổ thất bại: {res.ErrorMessage}";
        }
    }
}

public partial class UserManagementViewModel : ObservableObject
{
    private readonly IMediator _mediator;

    [ObservableProperty]
    private string _newUsername = string.Empty;

    [ObservableProperty]
    private string _newPassword = string.Empty;

    [ObservableProperty]
    private string _newFullName = string.Empty;

    [ObservableProperty]
    private string _newEmail = string.Empty;

    [ObservableProperty]
    private RoleDto? _selectedRole;

    [ObservableProperty]
    private UserDto? _selectedUser;

    [ObservableProperty]
    private string _statusMessage = string.Empty;

    public ObservableCollection<UserDto> Users { get; } = [];
    public ObservableCollection<RoleDto> Roles { get; } = [];

    public UserManagementViewModel(IMediator mediator)
    {
        _mediator = mediator;
    }

    [RelayCommand]
    public async Task LoadUsersAndRolesAsync()
    {
        var rolesRes = await _mediator.Send(new GetRolesQuery());
        if (rolesRes.IsSuccess && rolesRes.Value != null)
        {
            Roles.Clear();
            foreach (var r in rolesRes.Value) Roles.Add(r);
            if (Roles.Count != 0 && SelectedRole == null) SelectedRole = Roles[0];
        }

        var usersRes = await _mediator.Send(new GetUsersQuery());
        if (usersRes.IsSuccess && usersRes.Value != null)
        {
            Users.Clear();
            foreach (var u in usersRes.Value) Users.Add(u);
            StatusMessage = $"Đã tải {Users.Count} người dùng.";
        }
    }

    [RelayCommand]
    public async Task CreateUserAsync()
    {
        if (SelectedRole == null)
        {
            StatusMessage = "Vui lòng chọn vai trò.";
            return;
        }

        var res = await _mediator.Send(new CreateUserCommand(
            NewUsername,
            NewPassword,
            NewFullName,
            NewEmail,
            SelectedRole.Id));

        if (res.IsSuccess)
        {
            StatusMessage = $"Tạo người dùng '{NewUsername}' thành công!";
            NewUsername = string.Empty;
            NewPassword = string.Empty;
            NewFullName = string.Empty;
            NewEmail = string.Empty;
            await LoadUsersAndRolesAsync();
        }
        else
        {
            StatusMessage = $"Lỗi: {res.ErrorMessage}";
        }
    }

    [RelayCommand]
    public async Task ToggleUserStatusAsync()
    {
        if (SelectedUser == null) return;

        var res = await _mediator.Send(new SetUserStatusCommand(SelectedUser.Id, !SelectedUser.IsActive));
        if (res.IsSuccess)
        {
            StatusMessage = $"Đã cập nhật trạng thái người dùng {SelectedUser.Username}.";
            await LoadUsersAndRolesAsync();
        }
        else
        {
            StatusMessage = $"Lỗi: {res.ErrorMessage}";
        }
    }
}

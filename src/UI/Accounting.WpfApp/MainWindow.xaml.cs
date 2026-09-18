using System.Windows;
using System.Windows.Input;
using Accounting.WpfApp.Services;
using Accounting.WpfApp.ViewModels;

namespace Accounting.WpfApp;

public partial class MainWindow : Window
{
    private readonly MainViewModel _viewModel;
    private readonly KeyboardShortcutManager _shortcuts;

    public MainWindow(MainViewModel viewModel, KeyboardShortcutManager shortcuts)
    {
        InitializeComponent();
        _viewModel = viewModel;
        _shortcuts = shortcuts;
        DataContext = _viewModel;

        // Wire shortcut handlers
        _shortcuts.OnNewVoucher += () => _viewModel.VoucherEntry.NewVoucher();
        _shortcuts.OnPostVoucher += async () => await _viewModel.VoucherEntry.PostVoucherAsync();
        _shortcuts.OnUnpostVoucher += async () => await _viewModel.VoucherEntry.UnpostVoucherAsync();
        _shortcuts.OnSave += async () => await _viewModel.VoucherEntry.SaveVoucherAsync();
        _shortcuts.OnRefresh += async () => await _viewModel.RefreshDataAsync();
    }

    private void MainWindow_OnKeyDown(object sender, KeyEventArgs e)
    {
        _shortcuts.HandleKeyDown(e);
    }
}

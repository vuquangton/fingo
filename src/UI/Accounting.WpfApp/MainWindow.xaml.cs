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
        _shortcuts.OnCalculateBalance += () =>
        {
            _viewModel.VoucherEntry.RecalculateTotals();
            _viewModel.StatusMessage = $"Đã tính lại cân đối chứng từ: {_viewModel.VoucherEntry.BalanceStatusText}";
        };
        _shortcuts.OnPrint += () => _viewModel.VoucherEntry.PrintVoucher();
        _shortcuts.OnSearch += () => _viewModel.UniversalSearch.IsOpen = !_viewModel.UniversalSearch.IsOpen;
    }

    private void MainWindow_OnKeyDown(object sender, KeyEventArgs e)
    {
        _shortcuts.HandleKeyDown(e);
    }

    private void VoucherDataGrid_PreviewKeyDown(object sender, KeyEventArgs e)
    {
        if (sender is not System.Windows.Controls.DataGrid grid) return;

        if (e.Key == Key.Delete && Keyboard.Modifiers == ModifierKeys.None)
        {
            if (grid.SelectedItem is VoucherLineItemModel selectedLine)
            {
                _viewModel.VoucherEntry.RemoveLine(selectedLine);
                e.Handled = true;
            }
        }
        else if (e.Key == Key.Enter && Keyboard.Modifiers == ModifierKeys.None)
        {
            // Enter key horizontally moves cell-to-cell, or adds a new row if at last cell/row
            var currentCell = grid.CurrentCell;
            var colIndex = currentCell.Column != null ? grid.Columns.IndexOf(currentCell.Column) : -1;
            var rowIndex = grid.SelectedIndex;

            if (colIndex >= 0 && colIndex < grid.Columns.Count - 2) // before delete button col
            {
                // Move to next column
                var nextCol = grid.Columns[colIndex + 1];
                grid.CurrentCell = new System.Windows.Controls.DataGridCellInfo(grid.SelectedItem, nextCol);
                e.Handled = true;
            }
            else
            {
                // Last editable column: add new row or move to next row first editable column
                if (rowIndex == _viewModel.VoucherEntry.Lines.Count - 1)
                {
                    _viewModel.VoucherEntry.AddLine();
                    grid.SelectedIndex = _viewModel.VoucherEntry.Lines.Count - 1;
                    if (grid.Columns.Count > 0)
                    {
                        grid.CurrentCell = new System.Windows.Controls.DataGridCellInfo(grid.SelectedItem, grid.Columns[0]);
                    }
                    e.Handled = true;
                }
            }
        }
    }
}

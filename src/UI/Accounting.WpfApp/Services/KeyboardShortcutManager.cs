using System.Windows.Input;

namespace Accounting.WpfApp.Services;

public class KeyboardShortcutManager
{
    public event Action? OnNewVoucher;
    public event Action? OnSearch;
    public event Action? OnRefresh;
    public event Action? OnPostVoucher;
    public event Action? OnUnpostVoucher;
    public event Action? OnSave;
    public event Action? OnCancel;
    public event Action? OnCalculateBalance;

    public void HandleKeyDown(KeyEventArgs e)
    {
        if (e.Key == Key.F2)
        {
            OnNewVoucher?.Invoke();
            e.Handled = true;
        }
        else if (e.Key == Key.F3)
        {
            OnSearch?.Invoke();
            e.Handled = true;
        }
        else if (e.Key == Key.F5)
        {
            OnRefresh?.Invoke();
            e.Handled = true;
        }
        else if (e.Key == Key.F8)
        {
            if ((Keyboard.Modifiers & ModifierKeys.Shift) == ModifierKeys.Shift)
            {
                OnUnpostVoucher?.Invoke();
            }
            else
            {
                OnPostVoucher?.Invoke();
            }
            e.Handled = true;
        }
        else if (e.Key == Key.F9)
        {
            OnCalculateBalance?.Invoke();
            e.Handled = true;
        }
        else if (e.Key == Key.F12 || (e.Key == Key.S && (Keyboard.Modifiers & ModifierKeys.Control) == ModifierKeys.Control))
        {
            OnSave?.Invoke();
            e.Handled = true;
        }
        else if (e.Key == Key.Escape)
        {
            OnCancel?.Invoke();
            e.Handled = true;
        }
    }
}

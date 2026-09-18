using System.Collections.ObjectModel;
using System.Windows;
using System.Windows.Controls;
using System.Windows.Input;
using Accounting.WpfApp.Services;

namespace Accounting.WpfApp.Controls;

public partial class InGridLookupControl : UserControl
{
    public static readonly DependencyProperty SearchTextProperty =
        DependencyProperty.Register(nameof(SearchText), typeof(string), typeof(InGridLookupControl),
            new FrameworkPropertyMetadata(string.Empty, FrameworkPropertyMetadataOptions.BindsTwoWayByDefault));

    public static readonly DependencyProperty SelectedCodeProperty =
        DependencyProperty.Register(nameof(SelectedCode), typeof(string), typeof(InGridLookupControl),
            new FrameworkPropertyMetadata(string.Empty, FrameworkPropertyMetadataOptions.BindsTwoWayByDefault));

    public static readonly DependencyProperty LookupEngineProperty =
        DependencyProperty.Register(nameof(LookupEngine), typeof(FuzzyLookupEngine), typeof(InGridLookupControl),
            new PropertyMetadata(null));

    public string SearchText
    {
        get => (string)GetValue(SearchTextProperty);
        set => SetValue(SearchTextProperty, value);
    }

    public string SelectedCode
    {
        get => (string)GetValue(SelectedCodeProperty);
        set => SetValue(SelectedCodeProperty, value);
    }

    public FuzzyLookupEngine? LookupEngine
    {
        get => (FuzzyLookupEngine?)GetValue(LookupEngineProperty);
        set => SetValue(LookupEngineProperty, value);
    }

    public ObservableCollection<LookupItem> FilteredItems { get; } = [];

    public event Action<LookupItem>? OnItemSelected;

    public InGridLookupControl()
    {
        InitializeComponent();
    }

    private void SearchTextBox_TextChanged(object sender, TextChangedEventArgs e)
    {
        if (LookupEngine == null || string.IsNullOrWhiteSpace(SearchTextBox.Text))
        {
            LookupPopup.IsOpen = false;
            FilteredItems.Clear();
            return;
        }

        var results = LookupEngine.Search(SearchTextBox.Text, maxResults: 20);
        FilteredItems.Clear();
        foreach (var item in results)
        {
            FilteredItems.Add(item);
        }

        if (FilteredItems.Count > 0)
        {
            LookupPopup.IsOpen = true;
            ResultsListBox.SelectedIndex = 0;
        }
        else
        {
            LookupPopup.IsOpen = false;
        }
    }

    private void SearchTextBox_PreviewKeyDown(object sender, KeyEventArgs e)
    {
        if (LookupPopup.IsOpen)
        {
            if (e.Key == Key.Down)
            {
                if (ResultsListBox.SelectedIndex < FilteredItems.Count - 1)
                {
                    ResultsListBox.SelectedIndex++;
                    ResultsListBox.ScrollIntoView(ResultsListBox.SelectedItem);
                }
                e.Handled = true;
            }
            else if (e.Key == Key.Up)
            {
                if (ResultsListBox.SelectedIndex > 0)
                {
                    ResultsListBox.SelectedIndex--;
                    ResultsListBox.ScrollIntoView(ResultsListBox.SelectedItem);
                }
                e.Handled = true;
            }
            else if (e.Key == Key.Enter || e.Key == Key.Tab)
            {
                CommitSelection();
                e.Handled = true;
            }
            else if (e.Key == Key.Escape)
            {
                LookupPopup.IsOpen = false;
                e.Handled = true;
            }
        }
        else if (e.Key == Key.F3)
        {
            // Force open lookup
            if (LookupEngine != null)
            {
                var results = LookupEngine.Search(SearchTextBox.Text, maxResults: 20);
                FilteredItems.Clear();
                foreach (var item in results) FilteredItems.Add(item);
                LookupPopup.IsOpen = FilteredItems.Count > 0;
            }
            e.Handled = true;
        }
    }

    private void CommitSelection()
    {
        if (ResultsListBox.SelectedItem is LookupItem selected)
        {
            SearchText = selected.Code;
            SelectedCode = selected.Code;
            LookupPopup.IsOpen = false;
            OnItemSelected?.Invoke(selected);
        }
    }

    private void ResultsListBox_PreviewKeyDown(object sender, KeyEventArgs e)
    {
        if (e.Key == Key.Enter)
        {
            CommitSelection();
            SearchTextBox.Focus();
            e.Handled = true;
        }
        else if (e.Key == Key.Escape)
        {
            LookupPopup.IsOpen = false;
            SearchTextBox.Focus();
            e.Handled = true;
        }
    }

    private void ResultsListBox_MouseDoubleClick(object sender, MouseButtonEventArgs e)
    {
        CommitSelection();
        SearchTextBox.Focus();
    }

    private void SearchTextBox_LostFocus(object sender, RoutedEventArgs e)
    {
        if (!LookupPopup.IsKeyboardFocusWithin)
        {
            LookupPopup.IsOpen = false;
        }
    }
}

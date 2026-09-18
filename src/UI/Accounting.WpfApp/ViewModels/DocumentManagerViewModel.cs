using System.Collections.ObjectModel;
using CommunityToolkit.Mvvm.ComponentModel;
using CommunityToolkit.Mvvm.Input;

namespace Accounting.WpfApp.ViewModels;

public abstract partial class WorkspaceTabViewModel : ObservableObject
{
    [ObservableProperty]
    private string _tabId = Guid.NewGuid().ToString("N");

    [ObservableProperty]
    private string _title = "Workspace";

    [ObservableProperty]
    private string _icon = "📄";

    [ObservableProperty]
    private bool _isDirty = false;

    [ObservableProperty]
    private bool _isClosable = true;

    public virtual Task RefreshAsync() => Task.CompletedTask;
    public virtual Task SaveAsync() => Task.CompletedTask;
}

public partial class DocumentManagerViewModel : ObservableObject
{
    public ObservableCollection<WorkspaceTabViewModel> Tabs { get; } = [];

    [ObservableProperty]
    private WorkspaceTabViewModel? _activeTab;

    [RelayCommand]
    public void OpenDocument(WorkspaceTabViewModel tab)
    {
        var existing = Tabs.FirstOrDefault(t => t.TabId == tab.TabId);
        if (existing != null)
        {
            ActiveTab = existing;
            return;
        }

        Tabs.Add(tab);
        ActiveTab = tab;
    }

    [RelayCommand]
    public void CloseDocument(WorkspaceTabViewModel? tab)
    {
        if (tab == null) return;
        if (!tab.IsClosable) return;

        var index = Tabs.IndexOf(tab);
        if (index < 0) return;

        Tabs.Remove(tab);

        if (ActiveTab == tab)
        {
            if (Tabs.Count > 0)
            {
                var nextIndex = Math.Min(index, Tabs.Count - 1);
                ActiveTab = Tabs[nextIndex];
            }
            else
            {
                ActiveTab = null;
            }
        }
    }

    [RelayCommand]
    public void CloseActiveDocument()
    {
        if (ActiveTab != null)
        {
            CloseDocument(ActiveTab);
        }
    }

    [RelayCommand]
    public void NextTab()
    {
        if (Tabs.Count <= 1 || ActiveTab == null) return;
        var idx = Tabs.IndexOf(ActiveTab);
        var nextIdx = (idx + 1) % Tabs.Count;
        ActiveTab = Tabs[nextIdx];
    }

    [RelayCommand]
    public void PreviousTab()
    {
        if (Tabs.Count <= 1 || ActiveTab == null) return;
        var idx = Tabs.IndexOf(ActiveTab);
        var prevIdx = (idx - 1 + Tabs.Count) % Tabs.Count;
        ActiveTab = Tabs[prevIdx];
    }
}

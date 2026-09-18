using Accounting.WpfApp.ViewModels;

namespace Accounting.WpfApp.Tests;

public class DocumentManagerViewModelTests
{
    [Fact]
    public void OpenDocument_ShouldAddTabAndSetAsActive()
    {
        var docManager = new DocumentManagerViewModel();
        var tab1 = new TestTabViewModel("PKT-001", "Phiếu kế toán");

        docManager.OpenDocument(tab1);

        Assert.Single(docManager.Tabs);
        Assert.Equal(tab1, docManager.ActiveTab);
    }

    [Fact]
    public void OpenDocument_IfAlreadyOpen_ShouldSwitchActiveTabWithoutDuplicating()
    {
        var docManager = new DocumentManagerViewModel();
        var tab1 = new TestTabViewModel("GL-111", "Sổ cái TK 111");
        var tab2 = new TestTabViewModel("GL-112", "Sổ cái TK 112");

        docManager.OpenDocument(tab1);
        docManager.OpenDocument(tab2);
        Assert.Equal(2, docManager.Tabs.Count);
        Assert.Equal(tab2, docManager.ActiveTab);

        // Reopen tab1
        docManager.OpenDocument(tab1);
        Assert.Equal(2, docManager.Tabs.Count);
        Assert.Equal(tab1, docManager.ActiveTab);
    }

    [Fact]
    public void CloseDocument_ShouldRemoveTabAndActivateNeighbor()
    {
        var docManager = new DocumentManagerViewModel();
        var tab1 = new TestTabViewModel("T1", "Tab 1");
        var tab2 = new TestTabViewModel("T2", "Tab 2");
        var tab3 = new TestTabViewModel("T3", "Tab 3");

        docManager.OpenDocument(tab1);
        docManager.OpenDocument(tab2);
        docManager.OpenDocument(tab3);

        // Close active tab3
        docManager.CloseDocument(tab3);
        Assert.Equal(2, docManager.Tabs.Count);
        Assert.Equal(tab2, docManager.ActiveTab);

        // Close tab2
        docManager.CloseDocument(tab2);
        Assert.Single(docManager.Tabs);
        Assert.Equal(tab1, docManager.ActiveTab);

        // Close tab1
        docManager.CloseDocument(tab1);
        Assert.Empty(docManager.Tabs);
        Assert.Null(docManager.ActiveTab);
    }

    private class TestTabViewModel : WorkspaceTabViewModel
    {
        public TestTabViewModel(string id, string title)
        {
            TabId = id;
            Title = title;
        }
    }
}

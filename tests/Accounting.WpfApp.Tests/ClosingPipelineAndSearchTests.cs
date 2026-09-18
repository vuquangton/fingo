using Accounting.WpfApp.ViewModels;

namespace Accounting.WpfApp.Tests;

public class ClosingPipelineAndSearchTests
{
    [Fact]
    public async Task ClosingPipelineWizard_RunsAllStepsSequentially()
    {
        // Arrange
        var vm = new ClosingPipelineWizardViewModel();
        Assert.Equal(6, vm.Steps.Count);
        Assert.All(vm.Steps, s => Assert.Equal(ClosingStepStatus.Pending, s.Status));

        // Act
        await vm.RunPipelineAsync();

        // Assert
        Assert.False(vm.IsExecuting);
        Assert.All(vm.Steps, s => Assert.Equal(ClosingStepStatus.Success, s.Status));
        Assert.Contains("100%", vm.StatusMessage);
    }

    [Fact]
    public void UniversalSearchViewModel_FindsVouchersAndNavigates()
    {
        // Arrange
        var vm = new UniversalSearchViewModel();

        // Act
        vm.SearchQuery = "PKT";
        vm.ExecuteSearch();

        // Assert
        Assert.NotEmpty(vm.Results);
        Assert.Contains(vm.Results, r => r.Code.StartsWith("PKT"));
    }
}

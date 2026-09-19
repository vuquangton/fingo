using System;
using System.Windows;
using Accounting.WpfApp.ViewModels;

namespace Accounting.WpfApp.Views
{
    public partial class SetupWizardWindow : Window
    {
        public SetupWizardWindow(SetupWizardViewModel viewModel)
        {
            InitializeComponent();
            DataContext = viewModel;
            viewModel.WizardCompleted += OnWizardCompleted;
        }

        private void OnWizardCompleted(object? sender, EventArgs e)
        {
            this.DialogResult = true;
            this.Close();
        }
    }
}


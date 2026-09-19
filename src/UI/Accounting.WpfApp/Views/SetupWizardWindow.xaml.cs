using System;
using System.Windows;
using Accounting.WpfApp.ViewModels;

namespace Accounting.WpfApp.Views
{
    public partial class SetupWizardWindow : Window
    {
        public SetupWizardWindow()
        {
            InitializeComponent();
            
            if (DataContext is SetupWizardViewModel vm)
            {
                vm.WizardCompleted += OnWizardCompleted;
            }
        }

        private void OnWizardCompleted(object? sender, EventArgs e)
        {
            this.DialogResult = true;
            this.Close();
        }
    }
}


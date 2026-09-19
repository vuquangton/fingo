using System;
using System.ComponentModel;
using System.Runtime.CompilerServices;
using System.Windows.Input;

namespace Accounting.WpfApp.ViewModels
{
    public class SetupWizardViewModel : INotifyPropertyChanged
    {
        private int _currentStep = 1;
        private int _totalSteps = 8;
        
        // Mocks for data binding in different steps
        private string _mst = "";
        private string _companyName = "";
        
        public int CurrentStep
        {
            get => _currentStep;
            set
            {
                if (_currentStep != value)
                {
                    _currentStep = value;
                    OnPropertyChanged();
                    OnPropertyChanged(nameof(StepTitle));
                    OnPropertyChanged(nameof(IsStep1));
                    OnPropertyChanged(nameof(IsStep2));
                    OnPropertyChanged(nameof(IsStep3));
                    OnPropertyChanged(nameof(IsStep4));
                    OnPropertyChanged(nameof(IsStep5));
                    OnPropertyChanged(nameof(IsStep6));
                    OnPropertyChanged(nameof(IsStep7));
                    OnPropertyChanged(nameof(IsStep8));
                    OnPropertyChanged(nameof(CanGoBack));
                    OnPropertyChanged(nameof(NextButtonLabel));
                }
            }
        }

        public string StepTitle
        {
            get
            {
                return CurrentStep switch
                {
                    1 => "Chào m?ng",
                    2 => "Công ty",
                    3 => "Ch? d? k? toán",
                    4 => "H? th?ng tài kho?n",
                    5 => "Danh m?c",
                    6 => "S? du d?u k?",
                    7 => "Tích h?p",
                    8 => "Hoàn t?t",
                    _ => ""
                };
            }
        }
        
        public string Mst
        {
            get => _mst;
            set
            {
                _mst = value;
                OnPropertyChanged();
            }
        }
        
        public string CompanyName
        {
            get => _companyName;
            set
            {
                _companyName = value;
                OnPropertyChanged();
            }
        }

        public bool IsStep1 => CurrentStep == 1;
        public bool IsStep2 => CurrentStep == 2;
        public bool IsStep3 => CurrentStep == 3;
        public bool IsStep4 => CurrentStep == 4;
        public bool IsStep5 => CurrentStep == 5;
        public bool IsStep6 => CurrentStep == 6;
        public bool IsStep7 => CurrentStep == 7;
        public bool IsStep8 => CurrentStep == 8;

        public bool CanGoBack => CurrentStep > 1;
        
        public string NextButtonLabel => CurrentStep == 8 ? "Vào làm vi?c" : (CurrentStep == 7 ? "Hoàn t?t" : (CurrentStep == 1 ? "B?t d?u" : "Ti?p theo"));

        public ICommand NextCommand { get; }
        public ICommand BackCommand { get; }
        public ICommand FinishCommand { get; }

        public event EventHandler? WizardCompleted;

        public SetupWizardViewModel()
        {
            NextCommand = new RelayCommand(_ => NextStep(), _ => CurrentStep < 8);
            BackCommand = new RelayCommand(_ => PreviousStep(), _ => CanGoBack);
            FinishCommand = new RelayCommand(_ => WizardCompleted?.Invoke(this, EventArgs.Empty), _ => CurrentStep == 8);
        }

        private void NextStep()
        {
            if (CurrentStep == 2 && Mst == "0101234567")
            {
                CompanyName = "Công ty TNHH Demo";
            }
            if (CurrentStep < _totalSteps)
                CurrentStep++;
        }

        private void PreviousStep()
        {
            if (CurrentStep > 1)
                CurrentStep--;
        }

        public event PropertyChangedEventHandler? PropertyChanged;
        protected virtual void OnPropertyChanged([CallerMemberName] string? propertyName = null)
        {
            PropertyChanged?.Invoke(this, new PropertyChangedEventArgs(propertyName));
        }
    }

    public class RelayCommand : ICommand
    {
        private readonly Action<object?> _execute;
        private readonly Predicate<object?>? _canExecute;

        public RelayCommand(Action<object?> execute, Predicate<object?>? canExecute = null)
        {
            _execute = execute ?? throw new ArgumentNullException(nameof(execute));
            _canExecute = canExecute;
        }

        public bool CanExecute(object? parameter) => _canExecute == null || _canExecute(parameter);
        public void Execute(object? parameter) => _execute(parameter);
        public event EventHandler? CanExecuteChanged
        {
            add => CommandManager.RequerySuggested += value;
            remove => CommandManager.RequerySuggested -= value;
        }
    }
}


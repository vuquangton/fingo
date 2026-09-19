using System;
using System.ComponentModel;
using System.Runtime.CompilerServices;
using System.Threading.Tasks;
using System.Windows.Input;
using Accounting.Application.Features.Organization;
using MediatR;
using System.Windows;

namespace Accounting.WpfApp.ViewModels
{
    public class SetupWizardViewModel : INotifyPropertyChanged
    {
        private readonly IMediator _mediator;
        private int _currentStep = 1;
        private int _totalSteps = 8;
        
        private string _mst = "";
        private string _companyName = "";
        private string _address = "";
        private string _legalRepresentative = "";
        private string _chiefAccountant = "";
        
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
        
        public string Mst { get => _mst; set { _mst = value; OnPropertyChanged(); } }
        public string CompanyName { get => _companyName; set { _companyName = value; OnPropertyChanged(); } }
        public string Address { get => _address; set { _address = value; OnPropertyChanged(); } }
        public string LegalRepresentative { get => _legalRepresentative; set { _legalRepresentative = value; OnPropertyChanged(); } }
        public string ChiefAccountant { get => _chiefAccountant; set { _chiefAccountant = value; OnPropertyChanged(); } }

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
        public ICommand AutoFillCommand { get; }

        public event EventHandler? WizardCompleted;

        public SetupWizardViewModel(IMediator mediator)
        {
            _mediator = mediator;
            NextCommand = new AsyncRelayCommand(async _ => await NextStepAsync(), _ => CurrentStep < 8);
            BackCommand = new RelayCommand(_ => PreviousStep(), _ => CanGoBack);
            FinishCommand = new AsyncRelayCommand(async _ => await FinishAsync(), _ => CurrentStep == 8);
            AutoFillCommand = new RelayCommand(_ => AutoFill());
        }

        private void AutoFill()
        {
            if (Mst == "0101234567")
            {
                CompanyName = "Công ty TNHH Demo";
                Address = "S? 1 Ð?i C? Vi?t, Hai Bà Trung, Hà N?i";
                LegalRepresentative = "Nguy?n Van A";
                ChiefAccountant = "Tr?n Th? B";
            }
        }

        private async Task NextStepAsync()
        {
            if (CurrentStep < _totalSteps)
                CurrentStep++;
                
            await Task.CompletedTask;
        }

        private void PreviousStep()
        {
            if (CurrentStep > 1)
                CurrentStep--;
        }

        private async Task FinishAsync()
        {
            // Validate minimum required
            if (string.IsNullOrWhiteSpace(Mst) || string.IsNullOrWhiteSpace(CompanyName) || 
                string.IsNullOrWhiteSpace(Address) || string.IsNullOrWhiteSpace(LegalRepresentative) || 
                string.IsNullOrWhiteSpace(ChiefAccountant))
            {
                MessageBox.Show("Vui lòng di?n d?y d? thông tin ? Bu?c 2 tru?c khi hoàn t?t.", "L?i", MessageBoxButton.OK, MessageBoxImage.Error);
                CurrentStep = 2;
                return;
            }

            var cmd = new UpdateCompanySettingCommand(
                Mst,
                CompanyName,
                Address,
                LegalRepresentative,
                ChiefAccountant
            );

            var result = await _mediator.Send(cmd);
            if (result.IsSuccess)
            {
                WizardCompleted?.Invoke(this, EventArgs.Empty);
            }
            else
            {
                MessageBox.Show(result.ErrorMessage ?? "C?p nh?t th?t b?i", "L?i", MessageBoxButton.OK, MessageBoxImage.Error);
            }
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
    
    public class AsyncRelayCommand : ICommand
    {
        private readonly Func<object?, Task> _execute;
        private readonly Predicate<object?>? _canExecute;

        public AsyncRelayCommand(Func<object?, Task> execute, Predicate<object?>? canExecute = null)
        {
            _execute = execute ?? throw new ArgumentNullException(nameof(execute));
            _canExecute = canExecute;
        }

        public bool CanExecute(object? parameter) => _canExecute == null || _canExecute(parameter);
        public async void Execute(object? parameter) => await _execute(parameter);
        public event EventHandler? CanExecuteChanged
        {
            add => CommandManager.RequerySuggested += value;
            remove => CommandManager.RequerySuggested -= value;
        }
    }
}


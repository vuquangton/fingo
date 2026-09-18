using System.Globalization;
using System.Windows;
using System.Windows.Data;
using Accounting.WpfApp.ViewModels;

namespace Accounting.WpfApp.Converters;

public class ModuleToVisibilityConverter : IValueConverter
{
    public object Convert(object? value, Type targetType, object? parameter, CultureInfo culture)
    {
        if (value is AppModule currentModule && parameter is AppModule targetModule)
        {
            return currentModule == targetModule ? Visibility.Visible : Visibility.Collapsed;
        }

        if (value is AppModule curMod && parameter is string paramStr && Enum.TryParse<AppModule>(paramStr, out var parsedModule))
        {
            return curMod == parsedModule ? Visibility.Visible : Visibility.Collapsed;
        }

        return Visibility.Collapsed;
    }

    public object ConvertBack(object? value, Type targetType, object? parameter, CultureInfo culture)
    {
        throw new NotImplementedException();
    }
}

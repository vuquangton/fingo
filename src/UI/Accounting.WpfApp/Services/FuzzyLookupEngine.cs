using System.Globalization;
using System.Text;

namespace Accounting.WpfApp.Services;

public record LookupItem(
    string Code,
    string Name,
    string ExtraInfo,
    string Category);

public class FuzzyLookupEngine
{
    private readonly List<LookupItem> _items = [];
    private readonly List<string> _normalizedKeys = [];
    private readonly object _lock = new();

    public void LoadData(IEnumerable<LookupItem> items)
    {
        lock (_lock)
        {
            _items.Clear();
            _normalizedKeys.Clear();

            foreach (var item in items)
            {
                _items.Add(item);
                var key = $"{Normalize(item.Code)} {Normalize(item.Name)} {Normalize(item.ExtraInfo)}";
                _normalizedKeys.Add(key);
            }
        }
    }

    public IReadOnlyList<LookupItem> Search(string query, int maxResults = 50)
    {
        if (string.IsNullOrWhiteSpace(query))
            return Array.Empty<LookupItem>();

        var normalizedQuery = Normalize(query);
        var tokens = normalizedQuery.Split(' ', StringSplitOptions.RemoveEmptyEntries);
        var results = new List<LookupItem>(maxResults);

        lock (_lock)
        {
            for (int i = 0; i < _normalizedKeys.Count; i++)
            {
                var key = _normalizedKeys[i];
                bool allTokensMatch = true;
                for (int t = 0; t < tokens.Length; t++)
                {
                    if (!key.Contains(tokens[t], StringComparison.Ordinal))
                    {
                        allTokensMatch = false;
                        break;
                    }
                }

                if (allTokensMatch)
                {
                    results.Add(_items[i]);
                    if (results.Count >= maxResults)
                        break;
                }
            }
        }

        return results;
    }

    public static string Normalize(string text)
    {
        if (string.IsNullOrEmpty(text))
            return string.Empty;

        var normalizedString = text.Normalize(NormalizationForm.FormD);
        var stringBuilder = new StringBuilder(normalizedString.Length);

        for (int i = 0; i < normalizedString.Length; i++)
        {
            char c = normalizedString[i];
            var unicodeCategory = CharUnicodeInfo.GetUnicodeCategory(c);
            if (unicodeCategory != UnicodeCategory.NonSpacingMark)
            {
                stringBuilder.Append(char.ToLowerInvariant(c));
            }
        }

        return stringBuilder.ToString().Normalize(NormalizationForm.FormC);
    }
}

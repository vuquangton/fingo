using Accounting.Application.Common.Interfaces;
using Accounting.Domain.MasterData.Partners;
using Microsoft.EntityFrameworkCore;

namespace Accounting.Application.Features.MasterData.Services;

public record DuplicatePartnerMatch(
    Guid PartnerId,
    string PartnerCode,
    string PartnerName,
    string? TaxCode,
    string? Phone,
    double SimilarityScore,
    string MatchReason);

public interface IPartnerDeduplicationEngine
{
    double ComputeNameSimilarity(string s1, string s2);
    Task<List<DuplicatePartnerMatch>> FindDuplicatesAsync(
        string partnerName,
        string? taxCode,
        string? phone,
        Guid? currentPartnerId = null,
        CancellationToken cancellationToken = default);
}

public class PartnerDeduplicationEngine : IPartnerDeduplicationEngine
{
    private readonly IAccountingDbContext? _context;
    private readonly IVietnamTaxLookupService _taxService;

    public PartnerDeduplicationEngine(IAccountingDbContext? context, IVietnamTaxLookupService taxService)
    {
        _context = context;
        _taxService = taxService;
    }

    public async Task<List<DuplicatePartnerMatch>> FindDuplicatesAsync(
        string partnerName,
        string? taxCode,
        string? phone,
        Guid? currentPartnerId = null,
        CancellationToken cancellationToken = default)
    {
        var matches = new List<DuplicatePartnerMatch>();
        if (_context == null) return matches;

        var cleanTax = string.IsNullOrWhiteSpace(taxCode) ? null : taxCode.Trim();
        var cleanPhone = string.IsNullOrWhiteSpace(phone) ? null : new string(phone.Where(char.IsDigit).ToArray());
        var normTargetName = _taxService.NormalizeVietnamese(partnerName);

        var existing = await _context.BusinessPartners.AsNoTracking()
            .Where(p => p.IsActive)
            .ToListAsync(cancellationToken);

        foreach (var p in existing)
        {
            if (currentPartnerId.HasValue && p.Id.Value == currentPartnerId.Value)
                continue;

            // 1. Exact Tax Code match
            if (!string.IsNullOrWhiteSpace(cleanTax) && !string.IsNullOrWhiteSpace(p.TaxCode) &&
                string.Equals(cleanTax, p.TaxCode.Trim(), StringComparison.OrdinalIgnoreCase))
            {
                matches.Add(new DuplicatePartnerMatch(
                    p.Id.Value,
                    p.PartnerCode,
                    p.Name,
                    p.TaxCode,
                    p.ContactPhone,
                    1.0,
                    $"Trùng tuyệt đối Mã số thuế ({cleanTax})"));
                continue;
            }

            // 2. Exact Phone match
            if (!string.IsNullOrWhiteSpace(cleanPhone) && !string.IsNullOrWhiteSpace(p.ContactPhone))
            {
                var existingPhone = new string(p.ContactPhone.Where(char.IsDigit).ToArray());
                if (existingPhone.Length >= 9 && cleanPhone.Length >= 9 && existingPhone == cleanPhone)
                {
                    matches.Add(new DuplicatePartnerMatch(
                        p.Id.Value,
                        p.PartnerCode,
                        p.Name,
                        p.TaxCode,
                        p.ContactPhone,
                        0.95,
                        $"Trùng số điện thoại giao dịch ({p.ContactPhone})"));
                    continue;
                }
            }

            // 3. Name Similarity
            var normExistingName = _taxService.NormalizeVietnamese(p.Name);
            var sim = ComputeNameSimilarity(normTargetName, normExistingName);

            if (sim >= 0.85)
            {
                matches.Add(new DuplicatePartnerMatch(
                    p.Id.Value,
                    p.PartnerCode,
                    p.Name,
                    p.TaxCode,
                    p.ContactPhone,
                    sim,
                    $"Tên doanh nghiệp tương đồng cao ({(int)(sim * 100)}%): '{p.Name}'"));
            }
        }

        return matches.OrderByDescending(m => m.SimilarityScore).ToList();
    }

    public double ComputeNameSimilarity(string s1, string s2)
    {
        if (string.IsNullOrWhiteSpace(s1) || string.IsNullOrWhiteSpace(s2)) return 0.0;
        if (string.Equals(s1.Trim(), s2.Trim(), StringComparison.OrdinalIgnoreCase)) return 1.0;

        // Strip prefixes for standard comparison
        var clean1 = StripCorporatePrefixes(s1.Trim().ToLowerInvariant());
        var clean2 = StripCorporatePrefixes(s2.Trim().ToLowerInvariant());

        if (string.Equals(clean1, clean2, StringComparison.OrdinalIgnoreCase)) return 1.0;

        // Token overlap check (Jaccard similarity on significant tokens)
        var tokens1 = clean1.Split(' ', StringSplitOptions.RemoveEmptyEntries).ToHashSet();
        var tokens2 = clean2.Split(' ', StringSplitOptions.RemoveEmptyEntries).ToHashSet();
        var intersect = tokens1.Intersect(tokens2).Count();
        var union = tokens1.Union(tokens2).Count();
        double jaccard = union > 0 ? (double)intersect / union : 0.0;

        // Jaro-Winkler distance
        double jaro = JaroSimilarity(clean1, clean2);
        int prefixLength = 0;
        int maxPrefix = Math.Min(4, Math.Min(clean1.Length, clean2.Length));
        for (int i = 0; i < maxPrefix; i++)
        {
            if (clean1[i] == clean2[i]) prefixLength++;
            else break;
        }

        double jaroWinkler = jaro + (prefixLength * 0.1 * (1.0 - jaro));
        double combined = Math.Max(jaroWinkler, (jaroWinkler * 0.5) + (jaccard * 0.5));
        return Math.Min(1.0, Math.Max(0.0, combined));
    }

    private static string StripCorporatePrefixes(string name)
    {
        var text = name;
        string[] prefixes = ["cong ty co phan", "cong ty tnhh", "cong ty cp", "cty co phan", "cty tnhh", "cty cp", "doanh nghiep", "tap doan"];
        foreach (var p in prefixes)
        {
            if (text.StartsWith(p))
            {
                text = text.Substring(p.Length).Trim();
                break;
            }
        }
        return text;
    }

    private static double JaroSimilarity(string s1, string s2)
    {
        int len1 = s1.Length;
        int len2 = s2.Length;
        if (len1 == 0 && len2 == 0) return 1.0;
        if (len1 == 0 || len2 == 0) return 0.0;

        int matchDistance = Math.Max(len1, len2) / 2 - 1;
        bool[] s1Matches = new bool[len1];
        bool[] s2Matches = new bool[len2];

        int matches = 0;
        for (int i = 0; i < len1; i++)
        {
            int start = Math.Max(0, i - matchDistance);
            int end = Math.Min(i + matchDistance + 1, len2);

            for (int j = start; j < end; j++)
            {
                if (s2Matches[j] || s1[i] != s2[j]) continue;
                s1Matches[i] = true;
                s2Matches[j] = true;
                matches++;
                break;
            }
        }

        if (matches == 0) return 0.0;

        int transpositions = 0;
        int k = 0;
        for (int i = 0; i < len1; i++)
        {
            if (!s1Matches[i]) continue;
            while (!s2Matches[k]) k++;
            if (s1[i] != s2[k]) transpositions++;
            k++;
        }

        return ((matches / (double)len1) + (matches / (double)len2) + ((matches - transpositions / 2.0) / matches)) / 3.0;
    }
}

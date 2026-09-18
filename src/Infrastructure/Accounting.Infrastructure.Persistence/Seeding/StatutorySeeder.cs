using System.Reflection;
using System.Text.Json;
using System.Text.Json.Serialization;
using Accounting.Domain.MasterData.Accounts;
using Accounting.Domain.MasterData.Common;
using Accounting.Domain.MasterData.Compliance;
using Accounting.Domain.MasterData.Currencies;
using Accounting.Domain.MasterData.Inventory;
using Accounting.Infrastructure.Persistence.Context;
using Microsoft.EntityFrameworkCore;
using CurrencyCode = Accounting.Domain.MasterData.Common.CurrencyCode;

namespace Accounting.Infrastructure.Persistence.Seeding;

public record AccountSeedModel(
    string Code,
    string Name,
    AccountType AccountType,
    BalanceNature BalanceNature,
    string? ParentCode = null,
    bool RequiresPartner = false,
    bool RequiresWarehouse = false,
    bool RequiresCostCenter = false,
    bool RequiresProject = false,
    string? EffectiveFrom = null,
    string? EffectiveTo = null);

public static class StatutorySeeder
{
    private static readonly JsonSerializerOptions JsonOptions = new()
    {
        PropertyNameCaseInsensitive = true,
        Converters = { new JsonStringEnumConverter() }
    };

    public static void ValidateManifest(IReadOnlyList<AccountSeedModel> manifest)
    {
        ArgumentNullException.ThrowIfNull(manifest);

        var codes = new HashSet<string>(StringComparer.OrdinalIgnoreCase);
        foreach (var item in manifest)
        {
            if (string.IsNullOrWhiteSpace(item.Code))
                throw new InvalidOperationException("Manifest contains an account with an empty code.");

            var code = item.Code.Trim();

            // 1. Strict Statutory Abolishment Check: Circular 99/2025 & 89/2026 bans
            BannedLegacyAccountCodes.EnsureNotBanned(code);

            if (!codes.Add(code))
                throw new InvalidOperationException($"Duplicate account code '{code}' found in seed manifest.");
        }

        // 2. Parent-child prefix and existence checks
        foreach (var item in manifest)
        {
            if (!string.IsNullOrWhiteSpace(item.ParentCode))
            {
                var parentCode = item.ParentCode.Trim();
                var childCode = item.Code.Trim();

                if (!childCode.StartsWith(parentCode, StringComparison.OrdinalIgnoreCase))
                    throw new InvalidOperationException($"Child account '{childCode}' must start with parent code '{parentCode}'.");

                if (!codes.Contains(parentCode))
                    throw new InvalidOperationException($"Parent account '{parentCode}' referenced by '{childCode}' does not exist in manifest.");
            }
        }
    }

    public static List<AccountSeedModel> LoadManifest(string? customJson = null)
    {
        if (!string.IsNullOrWhiteSpace(customJson))
        {
            return JsonSerializer.Deserialize<List<AccountSeedModel>>(customJson, JsonOptions)
                ?? throw new InvalidOperationException("Failed to deserialize custom account manifest JSON.");
        }

        var assembly = Assembly.GetExecutingAssembly();
        var resourceName = assembly.GetManifestResourceNames()
            .FirstOrDefault(n => n.EndsWith("statutory_coa_tt99_2025.json", StringComparison.OrdinalIgnoreCase));

        if (resourceName != null)
        {
            using var stream = assembly.GetManifestResourceStream(resourceName);
            if (stream != null)
            {
                using var reader = new StreamReader(stream);
                var content = reader.ReadToEnd();
                return JsonSerializer.Deserialize<List<AccountSeedModel>>(content, JsonOptions)
                    ?? throw new InvalidOperationException("Failed to deserialize embedded statutory manifest JSON.");
            }
        }

        // Fallback to disk relative to current base directory
        var possiblePaths = new[]
        {
            Path.Combine(AppContext.BaseDirectory, "SeedData", "statutory_coa_tt99_2025.json"),
            Path.Combine(Directory.GetCurrentDirectory(), "src", "Infrastructure", "Accounting.Infrastructure.Persistence", "SeedData", "statutory_coa_tt99_2025.json")
        };

        foreach (var p in possiblePaths)
        {
            if (File.Exists(p))
            {
                var content = File.ReadAllText(p);
                return JsonSerializer.Deserialize<List<AccountSeedModel>>(content, JsonOptions)
                    ?? throw new InvalidOperationException($"Failed to deserialize statutory manifest at {p}.");
            }
        }

        throw new FileNotFoundException("Statutory chart of accounts manifest 'statutory_coa_tt99_2025.json' could not be located.");
    }

    public static async Task SeedAsync(AccountingDbContext context, CancellationToken cancellationToken = default)
    {
        // 1. Currencies & Standard Exchange Rates
        if (!await context.Currencies.AnyAsync(cancellationToken))
        {
            var vnd = new Currency(CurrencyCode.Vnd, "Việt Nam Đồng", "₫", decimalPlaces: 0, isBaseCurrency: true);
            var usd = new Currency(CurrencyCode.Usd, "US Dollar", "$", decimalPlaces: 2, isBaseCurrency: false);
            var eur = new Currency(CurrencyCode.Eur, "Euro", "€", decimalPlaces: 2, isBaseCurrency: false);

            context.Currencies.AddRange(vnd, usd, eur);
            await context.SaveChangesAsync(cancellationToken);

            var today = DateTime.UtcNow.Date;
            var usdRate = new ExchangeRate(ExchangeRateId.New(), CurrencyCode.Usd, today, 25400m, 25450m, 25425m);
            var eurRate = new ExchangeRate(ExchangeRateId.New(), CurrencyCode.Eur, today, 27500m, 27600m, 27550m);
            context.ExchangeRates.AddRange(usdRate, eurRate);
            await context.SaveChangesAsync(cancellationToken);
        }

        // 2. Units of Measure
        if (!await context.UnitsOfMeasure.AnyAsync(cancellationToken))
        {
            var uoms = new List<UnitOfMeasure>
            {
                new(UomId.New(), "CAI", "Cái", "Chiếc / Cái"),
                new(UomId.New(), "KG", "Kilôgam", "Khối lượng chuẩn"),
                new(UomId.New(), "HOP", "Hộp", "Quy cách đóng gói hộp"),
                new(UomId.New(), "THUNG", "Thùng", "Quy cách đóng gói thùng"),
                new(UomId.New(), "MET", "Mét", "Đơn vị chiều dài"),
                new(UomId.New(), "LIT", "Lít", "Thể tích chất lỏng")
            };
            context.UnitsOfMeasure.AddRange(uoms);
            await context.SaveChangesAsync(cancellationToken);
        }

        // 3. Master Warehouses
        if (!await context.MasterWarehouses.AnyAsync(cancellationToken))
        {
            var wh = new Warehouse(new WarehouseId("KHO-TONG"), "Kho Tổng Công Ty", "Khu Công Nghiệp Tân Bình, TP. HCM");
            var whHn = new Warehouse(new WarehouseId("KHO-HN"), "Kho Chi Nhánh Hà Nội", "Khu Công Nghiệp Từ Liêm, Hà Nội");
            context.MasterWarehouses.AddRange(wh, whHn);
            await context.SaveChangesAsync(cancellationToken);
        }

        // 4. Statutory Chart of Accounts (Thông tư 99/2025/TT-BTC)
        if (!await context.MasterAccounts.AnyAsync(cancellationToken))
        {
            var manifest = LoadManifest();
            ValidateManifest(manifest);

            var map = new Dictionary<string, Account>(StringComparer.OrdinalIgnoreCase);
            foreach (var item in manifest)
            {
                var fromDate = !string.IsNullOrWhiteSpace(item.EffectiveFrom)
                    ? DateOnly.Parse(item.EffectiveFrom)
                    : new DateOnly(2026, 1, 1);

                DateOnly? toDate = !string.IsNullOrWhiteSpace(item.EffectiveTo)
                    ? DateOnly.Parse(item.EffectiveTo)
                    : null;

                var account = new Account(
                    new AccountId(item.Code),
                    item.Name,
                    item.AccountType,
                    item.BalanceNature,
                    !string.IsNullOrWhiteSpace(item.ParentCode) ? new AccountId(item.ParentCode) : null,
                    isParent: false,
                    requiresPartner: item.RequiresPartner,
                    requiresWarehouse: item.RequiresWarehouse,
                    requiresCostCenter: item.RequiresCostCenter,
                    requiresProject: item.RequiresProject,
                    governingCircular: GoverningCircular.TT99_2025_BTC,
                    effectiveFrom: fromDate,
                    effectiveTo: toDate);

                map[item.Code] = account;
            }

            foreach (var item in manifest)
            {
                if (!string.IsNullOrWhiteSpace(item.ParentCode) && map.TryGetValue(item.ParentCode, out var parentAccount))
                {
                    parentAccount.MarkAsParent();
                }
            }

            var sorted = map.Values
                .OrderBy(a => a.AccountLevel)
                .ThenBy(a => a.Id.Value)
                .ToList();

            context.MasterAccounts.AddRange(sorted);
            await context.SaveChangesAsync(cancellationToken);
        }
    }
}

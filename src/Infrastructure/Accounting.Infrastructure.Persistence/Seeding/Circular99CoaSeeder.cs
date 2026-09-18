using Accounting.Infrastructure.Persistence.Context;

namespace Accounting.Infrastructure.Persistence.Seeding;

/// <summary>
/// Seeds statutory Master Data conforming to Thông tư 99/2025/TT-BTC.
/// Delegates directly to the file-driven <see cref="StatutorySeeder"/>.
/// </summary>
public static class Circular99CoaSeeder
{
    public static Task SeedMasterDataAsync(AccountingDbContext context, CancellationToken cancellationToken = default)
    {
        return StatutorySeeder.SeedAsync(context, cancellationToken);
    }
}

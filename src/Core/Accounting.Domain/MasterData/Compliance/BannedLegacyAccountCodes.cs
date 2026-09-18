using System.Collections.Immutable;

namespace Accounting.Domain.MasterData.Compliance;

/// <summary>
/// Immutable registry of statutory account codes abolished under current Vietnamese regulations
/// (Thông tư 99/2025/TT-BTC & Thông tư 89/2026/TT-BTC).
/// Defense-in-depth: Prevents deprecated statutory structures from polluting master data.
/// </summary>
public static class BannedLegacyAccountCodes
{
    public static readonly ImmutableHashSet<string> AbolishedCodes = ImmutableHashSet.Create(
        StringComparer.OrdinalIgnoreCase,
        "142", // Abolished: Chi phí trả trước ngắn hạn (merged into TK 242)
        "311", // Abolished: Vay ngắn hạn (merged into TK 341)
        "315", // Abolished: Nợ dài hạn đến hạn trả (merged into TK 341)
        "512", // Abolished: Doanh thu bán hàng nội bộ (abolished)
        "001", // Abolished under TT99: Tài sản thuê ngoài
        "002", // Abolished under TT99: Vật tư, hàng hóa giữ hộ
        "003", // Abolished under TT99: Hàng hóa nhận bán đại lý
        "004", // Abolished under TT99: Nợ khó đòi đã xử lý
        "007"  // Abolished under TT99: Ngoại tệ các loại
    );

    public static bool IsBanned(string accountCode)
    {
        if (string.IsNullOrWhiteSpace(accountCode)) return false;
        return AbolishedCodes.Contains(accountCode.Trim());
    }

    public static void EnsureNotBanned(string accountCode)
    {
        if (IsBanned(accountCode))
        {
            throw new InvalidOperationException(
                $"Account code '{accountCode}' is an abolished legacy code banned under Thông tư 99/2025/TT-BTC and Thông tư 89/2026/TT-BTC.");
        }
    }
}

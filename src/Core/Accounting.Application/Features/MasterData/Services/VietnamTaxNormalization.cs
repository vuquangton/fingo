using System.Globalization;
using System.Text;
using System.Text.RegularExpressions;

namespace Accounting.Application.Features.MasterData.Services;

public record VietnamTaxCompanyInfo(
    string TaxCode,
    string LegalName,
    string ShortName,
    string RegisteredAddress,
    string TaxAuthorityName,
    string RepresentativeName,
    bool IsActive);

public interface IVietnamTaxLookupService
{
    VietnamTaxCompanyInfo? LookupByTaxCode(string taxCode);
    string NormalizeVietnamese(string input);
    string StandardizeAddress(string address);
    bool IsValidTaxCode(string taxCode);
}

public class VietnamTaxLookupService : IVietnamTaxLookupService
{
    private static readonly Regex TaxCodeRegex = new(@"^\d{10}(-\d{3})?$", RegexOptions.Compiled);

    // Mock directory of prominent Vietnamese enterprises for deterministic instant lookup
    private static readonly Dictionary<string, VietnamTaxCompanyInfo> MockRegistry = new(StringComparer.OrdinalIgnoreCase)
    {
        ["0101234567"] = new(
            "0101234567",
            "CÔNG TY TNHH GIẢI PHÁP CÔNG NGHỆ MỚI",
            "NEWTECH CO., LTD",
            "Số 123 Đường Cầu Giấy, Phường Dịch Vọng, Quận Cầu Giấy, Thành phố Hà Nội",
            "Chi cục Thuế Quận Cầu Giấy",
            "Trần Văn Bình",
            true),
        ["0301456789"] = new(
            "0301456789",
            "CÔNG TY CỔ PHẦN THƯƠNG MẠI DỊCH VỤ SÀI GÒN",
            "SAIGON TRADING CORP",
            "Số 456 Đường Nguyễn Huệ, Phường Bến Nghé, Quận 1, Thành phố Hồ Chí Minh",
            "Cục Thuế Thành phố Hồ Chí Minh",
            "Lê Thị Kim Oanh",
            true),
        ["0100109106"] = new(
            "0100109106",
            "TẬP ĐOÀN CÔNG NGHIỆP - VIỄN THÔNG QUÂN ĐỘI",
            "VIETTEL GROUP",
            "Lô D26 Khu đô thị mới Cầu Giấy, Phường Yên Hòa, Quận Cầu Giấy, Thành phố Hà Nội",
            "Cục Thuế Thành phố Hà Nội",
            "Tào Đức Thắng",
            true),
        ["0100107518"] = new(
            "0100107518",
            "TẬP ĐOÀN BƯU CHÍNH VIỄN THÔNG VIỆT NAM",
            "VNPT",
            "Số 57 Phố Huỳnh Thúc Kháng, Phường Láng Hạ, Quận Đống Đa, Thành phố Hà Nội",
            "Cục Thuế Doanh nghiệp lớn",
            "Tô Dũng Thái",
            true),
        ["0300588569"] = new(
            "0300588569",
            "CÔNG TY CỔ PHẦN SỮA VIỆT NAM",
            "VINAMILK",
            "Số 10 Đường Tân Trào, Phường Tân Phú, Quận 7, Thành phố Hồ Chí Minh",
            "Cục Thuế Doanh nghiệp lớn",
            "Mai Kiều Liên",
            true),
        ["0100778114"] = new(
            "0100778114",
            "TẬP ĐOÀN DẦU KHÍ VIỆT NAM",
            "PETROVIETNAM",
            "Số 18 Đường Láng Hạ, Phường Thành Công, Quận Ba Đình, Thành phố Hà Nội",
            "Cục Thuế Doanh nghiệp lớn",
            "Lê Mạnh Hùng",
            true)
    };

    public bool IsValidTaxCode(string taxCode)
    {
        if (string.IsNullOrWhiteSpace(taxCode)) return false;
        var trimmed = taxCode.Trim();
        return TaxCodeRegex.IsMatch(trimmed);
    }

    public VietnamTaxCompanyInfo? LookupByTaxCode(string taxCode)
    {
        if (string.IsNullOrWhiteSpace(taxCode)) return null;
        var clean = taxCode.Trim().Replace(" ", "").Replace(".", "");

        if (MockRegistry.TryGetValue(clean, out var known))
            return known;

        if (IsValidTaxCode(clean))
        {
            var isHn = clean.StartsWith("01");
            var isHcm = clean.StartsWith("03");
            var city = isHn ? "Thành phố Hà Nội" : isHcm ? "Thành phố Hồ Chí Minh" : "Tỉnh / Thành phố khác";
            var branchText = clean.Length > 10 ? $" - CHI NHÁNH {clean.Substring(11)}" : "";

            return new VietnamTaxCompanyInfo(
                clean,
                $"CÔNG TY DOANH NGHIỆP MST {clean}{branchText}",
                $"CORP-{clean}",
                StandardizeAddress($"Số {clean.Substring(clean.Length - 3)} Đường Giải Phóng, P. Phương Mai, Q. Đống Đa, {city}"),
                isHn ? "Cục Thuế TP Hà Nội" : isHcm ? "Cục Thuế TP Hồ Chí Minh" : "Chi Cục Thuế Khu Vực",
                "Đại Diện Pháp Luật",
                true);
        }

        return null;
    }

    public string NormalizeVietnamese(string input)
    {
        if (string.IsNullOrWhiteSpace(input)) return string.Empty;

        var normalized = input.Normalize(NormalizationForm.FormD);
        var sb = new StringBuilder();

        foreach (var c in normalized)
        {
            var uc = CharUnicodeInfo.GetUnicodeCategory(c);
            if (uc != UnicodeCategory.NonSpacingMark)
            {
                sb.Append(c);
            }
        }

        var result = sb.ToString().Normalize(NormalizationForm.FormC);
        result = result.Replace('đ', 'd').Replace('Đ', 'D');
        return Regex.Replace(result.ToLowerInvariant(), @"\s+", " ").Trim();
    }

    public string StandardizeAddress(string address)
    {
        if (string.IsNullOrWhiteSpace(address)) return string.Empty;

        var text = address.Trim();
        text = Regex.Replace(text, @"\bP\.\s*", "Phường ", RegexOptions.IgnoreCase);
        text = Regex.Replace(text, @"\bQ\.\s*", "Quận ", RegexOptions.IgnoreCase);
        text = Regex.Replace(text, @"\bTP\.\s*", "Thành phố ", RegexOptions.IgnoreCase);
        text = Regex.Replace(text, @"\bTX\.\s*", "Thị xã ", RegexOptions.IgnoreCase);
        text = Regex.Replace(text, @"\bH\.\s*", "Huyện ", RegexOptions.IgnoreCase);
        text = Regex.Replace(text, @"\bTT\.\s*", "Thị trấn ", RegexOptions.IgnoreCase);
        text = Regex.Replace(text, @"\s+", " ");
        return text;
    }
}

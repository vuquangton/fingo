using Accounting.Domain.MasterData.Common;
using Accounting.Domain.Organization;
using Xunit;

namespace Accounting.Domain.Tests.Organization;

public class CompanySettingTests
{
    [Fact]
    public void Constructor_ValidInputs_ShouldInitializeProperly()
    {
        var id = Guid.NewGuid();
        var company = new CompanySetting(
            id,
            "0101234567",
            "CÔNG TY TNHH ABC VIỆT NAM",
            "123 Nguyễn Huệ, Quận 1, TP. Hồ Chí Minh",
            "Nguyễn Văn Giám Đốc",
            "Trần Thị Kế Toán Trưởng",
            "VND",
            GoverningCircular.TT99_2025_BTC,
            1,
            "0901234567",
            "contact@abc.vn",
            "Cục Thuế TP. Hồ Chí Minh");

        Assert.Equal(id, company.Id);
        Assert.Equal("0101234567", company.TaxCode);
        Assert.Equal("CÔNG TY TNHH ABC VIỆT NAM", company.CompanyName);
        Assert.Equal("123 Nguyễn Huệ, Quận 1, TP. Hồ Chí Minh", company.Address);
        Assert.Equal("Nguyễn Văn Giám Đốc", company.LegalRepresentative);
        Assert.Equal("Trần Thị Kế Toán Trưởng", company.ChiefAccountant);
        Assert.Equal("VND", company.CurrencyCode);
        Assert.Equal(GoverningCircular.TT99_2025_BTC, company.GoverningCircular);
    }

    [Theory]
    [InlineData("0101234567-001")] // 10 digits - 3 digits branch
    [InlineData("0312345678")]     // 10 digits
    public void Constructor_ValidTaxCodes_ShouldSucceed(string validTaxCode)
    {
        var company = new CompanySetting(
            Guid.NewGuid(),
            validTaxCode,
            "Công ty Mẫu",
            "Hà Nội",
            "Giám Đốc",
            "Kế Toán Trưởng");

        Assert.Equal(validTaxCode, company.TaxCode);
    }

    [Theory]
    [InlineData("")]
    [InlineData(" ")]
    [InlineData("12345")]
    [InlineData("ABC1234567")]
    [InlineData("0101234567-12")] // branch must have 3 digits
    public void Constructor_InvalidTaxCode_ShouldThrowArgumentException(string invalidTaxCode)
    {
        Assert.Throws<ArgumentException>(() =>
            new CompanySetting(
                Guid.NewGuid(),
                invalidTaxCode,
                "Công ty Mẫu",
                "Hà Nội",
                "Giám Đốc",
                "Kế Toán Trưởng"));
    }

    [Fact]
    public void UpdateProfile_ValidInputs_ShouldUpdateFieldsAndTimestamp()
    {
        var company = new CompanySetting(
            Guid.NewGuid(),
            "0101234567",
            "Tên cũ",
            "Địa chỉ cũ",
            "GĐ cũ",
            "KTT cũ");

        company.UpdateProfile(
            "0101234567-001",
            "Tên mới",
            "Địa chỉ mới",
            "GĐ mới",
            "KTT mới",
            "0987654321",
            "new@abc.com",
            "Chi cục Thuế Q1");

        Assert.Equal("0101234567-001", company.TaxCode);
        Assert.Equal("Tên mới", company.CompanyName);
        Assert.Equal("Địa chỉ mới", company.Address);
        Assert.Equal("GĐ mới", company.LegalRepresentative);
        Assert.Equal("KTT mới", company.ChiefAccountant);
        Assert.Equal("0987654321", company.ContactPhone);
        Assert.NotNull(company.UpdatedAtUtc);
    }
}

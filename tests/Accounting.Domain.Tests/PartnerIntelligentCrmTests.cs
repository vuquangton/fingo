using Accounting.Application.Features.MasterData.Services;
using Accounting.Domain.Exceptions;
using Accounting.Domain.MasterData.Common;
using Accounting.Domain.MasterData.Partners;
using Xunit;

namespace Accounting.Domain.Tests;

public class PartnerIntelligentCrmTests
{
    [Fact]
    public void BusinessPartner_StatutoryAndCreditControls_ShouldEnforceProperInvariants()
    {
        var partner = new BusinessPartner(
            PartnerId.New(),
            "KH-NEWTECH-01",
            "Công ty Cổ phần Giải pháp Công nghệ Mới",
            PartnerType.Customer | PartnerType.Vendor,
            taxCode: "0101234567-001",
            address: "123 Cầu Giấy, Hà Nội",
            creditLimit: 100_000_000m,
            legalEntityType: LegalEntityType.Corporate,
            invoiceReceivingEmail: "einvoice@newtech.vn",
            taxAuthorityCode: "Cục Thuế Hà Nội");

        Assert.Equal("KH-NEWTECH-01", partner.PartnerCode);
        Assert.Equal(LegalEntityType.Corporate, partner.LegalEntityType);
        Assert.Equal("0101234567-001", partner.TaxCode);
        Assert.Equal(PartnerRiskTier.Low, partner.RiskTier);

        // Multi-Bank accounts
        partner.AddBankAccount("Vietcombank", "VCB", "0011001234567", "CONG TY CONG NGHE MOI", "Hà Nội", isDefault: true);
        partner.AddBankAccount("Techcombank", "TCB", "1903001234567", "CONG TY CONG NGHE MOI", "Cầu Giấy", isDefault: false);
        Assert.Equal(2, partner.BankAccounts.Count);

        // Multi-Addresses & Contacts
        partner.AddDeliveryAddress("Kho A - Lô 5 KCN Từ Liêm", city: "Hà Nội", isDefault: true);
        partner.AddContact("Trần Văn A", position: "Kế toán trưởng", mobile: "0901234567", isPrimary: true);
        Assert.Single(partner.DeliveryAddresses);
        Assert.Single(partner.Contacts);

        // Credit Risk Transitions
        partner.AdjustReceivableBalance(60_000_000m); // 60%
        Assert.Equal(PartnerRiskTier.Medium, partner.RiskTier);

        partner.AdjustReceivableBalance(30_000_000m); // 90%
        Assert.Equal(PartnerRiskTier.High, partner.RiskTier);

        partner.AdjustReceivableBalance(15_000_000m); // 105%
        Assert.Equal(PartnerRiskTier.Critical, partner.RiskTier);

        // Gating Check
        Assert.Throws<CreditLimitExceededException>(() => partner.CheckCreditLimit(5_000_000m));
    }

    [Fact]
    public void VietnamTaxNormalization_ShouldHandleAccentsAndAbbreviations()
    {
        var service = new VietnamTaxLookupService();

        // 1. Text normalization
        var norm = service.NormalizeVietnamese("CÔNG TY CỔ PHẦN THƯƠNG MẠI & DỊCH VỤ SÀI GÒN");
        Assert.Equal("cong ty co phan thuong mai & dich vu sai gon", norm);

        // 2. Address standardization
        var addr = service.StandardizeAddress("Số 123 P. Bến Nghé, Q. 1, TP. HCM");
        Assert.Equal("Số 123 Phường Bến Nghé, Quận 1, Thành phố HCM", addr);

        // 3. Tax code validation
        Assert.True(service.IsValidTaxCode("0101234567"));
        Assert.True(service.IsValidTaxCode("0101234567-001"));
        Assert.False(service.IsValidTaxCode("01012345")); // too short
        Assert.False(service.IsValidTaxCode("ABC0123456")); // non-digit

        // 4. Registry lookup
        var known = service.LookupByTaxCode("0300588569");
        Assert.NotNull(known);
        Assert.Equal("VINAMILK", known.ShortName);
    }

    [Fact]
    public void PartnerDeduplicationEngine_ShouldDetectSimilarEntities()
    {
        var taxService = new VietnamTaxLookupService();
        var engine = new PartnerDeduplicationEngine(null!, taxService);

        // Similar company names with abbreviations
        var sim1 = engine.ComputeNameSimilarity("cong ty tnhh hoa binh", "cty tnhh hoa binh");
        Assert.True(sim1 >= 0.85);

        // High typo match
        var sim2 = engine.ComputeNameSimilarity("cong ty co phan sua viet nam", "cong ty cp sua viet nam");
        Assert.True(sim2 >= 0.85);

        // Unrelated entities have substantially lower similarity
        var sim3 = engine.ComputeNameSimilarity("may mac hung yen", "dau khi viet nam");
        Assert.True(sim3 < 0.65);
    }
}

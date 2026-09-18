using Accounting.Domain.Common;

namespace Accounting.Domain.MasterData.Partners;

public class PartnerBankAccount : Entity<Guid>
{
    public string BankName { get; private set; } = string.Empty;
    public string BankCode { get; private set; } = string.Empty; // e.g. VCB, TCB, MB, CTG
    public string AccountNumber { get; private set; } = string.Empty;
    public string AccountHolder { get; private set; } = string.Empty;
    public string? Branch { get; private set; }
    public bool IsDefault { get; private set; }

    private PartnerBankAccount() { }

    public PartnerBankAccount(
        string bankName,
        string bankCode,
        string accountNumber,
        string accountHolder,
        string? branch = null,
        bool isDefault = false)
    {
        Id = Guid.NewGuid();
        BankName = bankName?.Trim() ?? throw new ArgumentNullException(nameof(bankName));
        BankCode = bankCode?.Trim().ToUpperInvariant() ?? string.Empty;
        AccountNumber = accountNumber?.Trim() ?? throw new ArgumentNullException(nameof(accountNumber));
        AccountHolder = accountHolder?.Trim().ToUpperInvariant() ?? string.Empty;
        Branch = branch?.Trim();
        IsDefault = isDefault;
    }

    public void SetDefault(bool isDefault) => IsDefault = isDefault;
}

public class PartnerDeliveryAddress : Entity<Guid>
{
    public string AddressLine { get; private set; } = string.Empty;
    public string? Ward { get; private set; }
    public string? District { get; private set; }
    public string? City { get; private set; }
    public string? ReceiverName { get; private set; }
    public string? ReceiverPhone { get; private set; }
    public bool IsDefault { get; private set; }

    private PartnerDeliveryAddress() { }

    public PartnerDeliveryAddress(
        string addressLine,
        string? ward = null,
        string? district = null,
        string? city = null,
        string? receiverName = null,
        string? receiverPhone = null,
        bool isDefault = false)
    {
        Id = Guid.NewGuid();
        AddressLine = addressLine?.Trim() ?? throw new ArgumentNullException(nameof(addressLine));
        Ward = ward?.Trim();
        District = district?.Trim();
        City = city?.Trim();
        ReceiverName = receiverName?.Trim();
        ReceiverPhone = receiverPhone?.Trim();
        IsDefault = isDefault;
    }

    public void SetDefault(bool isDefault) => IsDefault = isDefault;
}

public class PartnerContact : Entity<Guid>
{
    public string FullName { get; private set; } = string.Empty;
    public string? Position { get; private set; }
    public string? Mobile { get; private set; }
    public string? Email { get; private set; }
    public bool IsPrimary { get; private set; }

    private PartnerContact() { }

    public PartnerContact(
        string fullName,
        string? position = null,
        string? mobile = null,
        string? email = null,
        bool isPrimary = false)
    {
        Id = Guid.NewGuid();
        FullName = fullName?.Trim() ?? throw new ArgumentNullException(nameof(fullName));
        Position = position?.Trim();
        Mobile = mobile?.Trim();
        Email = email?.Trim();
        IsPrimary = isPrimary;
    }

    public void SetPrimary(bool isPrimary) => IsPrimary = isPrimary;
}

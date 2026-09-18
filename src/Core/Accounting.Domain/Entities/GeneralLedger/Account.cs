using Accounting.Domain.Common;
using Accounting.Domain.Enums;

namespace Accounting.Domain.Entities.GeneralLedger;

public class Account : Entity<Guid>
{
    public string AccountNumber { get; private set; } = string.Empty;
    public string Name { get; private set; } = string.Empty;
    public string? EnglishName { get; private set; }
    public Guid? ParentAccountId { get; private set; }
    public Account? ParentAccount { get; private set; }
    public AccountCategory Category { get; private set; }
    public BalanceType BalanceType { get; private set; }
    public bool IsActive { get; private set; } = true;
    public bool IsDetailAccount { get; private set; } = true;
    public bool IsForeignCurrencyTracked { get; private set; } = false;

    private readonly List<Account> _subAccounts = [];
    public IReadOnlyCollection<Account> SubAccounts => _subAccounts.AsReadOnly();

    private Account() { } // EF Core

    public Account(
        string accountNumber,
        string name,
        AccountCategory category,
        BalanceType balanceType,
        Guid? parentAccountId = null,
        string? englishName = null,
        bool isDetailAccount = true,
        bool isForeignCurrencyTracked = false)
    {
        Id = Guid.NewGuid();
        AccountNumber = accountNumber.Trim();
        Name = name.Trim();
        Category = category;
        BalanceType = balanceType;
        ParentAccountId = parentAccountId;
        EnglishName = englishName?.Trim();
        IsDetailAccount = isDetailAccount;
        IsForeignCurrencyTracked = isForeignCurrencyTracked;
    }

    public void Update(string name, string? englishName, BalanceType balanceType, bool isDetailAccount, bool isForeignCurrencyTracked)
    {
        Name = name.Trim();
        EnglishName = englishName?.Trim();
        BalanceType = balanceType;
        IsDetailAccount = isDetailAccount;
        IsForeignCurrencyTracked = isForeignCurrencyTracked;
    }

    public void Deactivate() => IsActive = false;
    public void Activate() => IsActive = true;

    public void AddSubAccount(Account subAccount)
    {
        _subAccounts.Add(subAccount);
        IsDetailAccount = false; // Parent accounts cannot be posted to directly in VAS
    }
}

using Accounting.Domain.Common;
using Accounting.Domain.MasterData.Common;
using Accounting.Domain.MasterData.Compliance;

namespace Accounting.Domain.MasterData.Accounts;

public class Account : AggregateRoot<AccountId>, IAuditableEntity
{
    public string AccountName { get; private set; } = string.Empty;
    public AccountType AccountType { get; private set; }
    public BalanceNature BalanceNature { get; private set; }
    public AccountId? ParentAccountId { get; private set; }
    public Account? ParentAccount { get; private set; }
    public int AccountLevel { get; private set; }
    public bool IsParent { get; private set; }
    public bool IsActive { get; private set; } = true;

    // Legal Compliance & Temporal Validity Guardrails
    public GoverningCircular GoverningCircular { get; private set; } = GoverningCircular.TT99_2025_BTC;
    public DateOnly EffectiveFrom { get; private set; } = new(2026, 1, 1);
    public DateOnly? EffectiveTo { get; private set; }

    // Tracking / Posting Dimensions
    public bool RequiresPartner { get; private set; }
    public bool RequiresWarehouse { get; private set; }
    public bool RequiresCostCenter { get; private set; }
    public bool RequiresProject { get; private set; }

    // CanPost: Invariant - parent accounts cannot accept direct postings
    public bool CanPost => !IsParent && IsActive;

    /// <summary>
    /// Temporal Posting Guardrail: Verifies that a transaction can post against this account on the voucher date.
    /// Rejects posting if account is a parent, inactive, or date is outside [EffectiveFrom, EffectiveTo].
    /// </summary>
    public bool CanPostOn(DateOnly voucherDate)
    {
        if (!CanPost) return false;
        if (voucherDate < EffectiveFrom) return false;
        if (EffectiveTo.HasValue && voucherDate > EffectiveTo.Value) return false;
        return true;
    }

    // IAuditableEntity
    public DateTime CreatedAtUtc { get; set; } = DateTime.UtcNow;
    public string? CreatedBy { get; set; }
    public DateTime? UpdatedAtUtc { get; set; }
    public string? UpdatedBy { get; set; }
    public DateTime? LastModifiedAtUtc { get => UpdatedAtUtc; set => UpdatedAtUtc = value; }
    public string? LastModifiedBy { get => UpdatedBy; set => UpdatedBy = value; }

    private readonly List<Account> _subAccounts = [];
    public IReadOnlyCollection<Account> SubAccounts => _subAccounts.AsReadOnly();

    private Account() { }

    public Account(
        AccountId id,
        string accountName,
        AccountType accountType,
        BalanceNature balanceNature,
        AccountId? parentAccountId = null,
        bool isParent = false,
        bool requiresPartner = false,
        bool requiresWarehouse = false,
        bool requiresCostCenter = false,
        bool requiresProject = false,
        GoverningCircular governingCircular = GoverningCircular.TT99_2025_BTC,
        DateOnly? effectiveFrom = null,
        DateOnly? effectiveTo = null)
    {
        var code = id.Value.Trim();
        if (string.IsNullOrWhiteSpace(code))
            throw new ArgumentException("Account code cannot be empty.", nameof(id));

        BannedLegacyAccountCodes.EnsureNotBanned(code);

        if (parentAccountId.HasValue && !string.IsNullOrWhiteSpace(parentAccountId.Value.Value))
        {
            var parentCode = parentAccountId.Value.Value.Trim();
            if (!code.StartsWith(parentCode, StringComparison.OrdinalIgnoreCase))
                throw new InvalidOperationException($"Child account '{code}' must start with parent code '{parentCode}'.");
        }

        var fromDate = effectiveFrom ?? new DateOnly(2026, 1, 1);
        if (effectiveTo.HasValue && effectiveTo.Value < fromDate)
            throw new ArgumentException($"EffectiveTo date '{effectiveTo}' cannot be prior to EffectiveFrom date '{fromDate}'.", nameof(effectiveTo));

        Id = new AccountId(code);
        AccountName = accountName.Trim();
        AccountType = accountType;
        BalanceNature = balanceNature;
        ParentAccountId = parentAccountId;
        IsParent = isParent;
        AccountLevel = ComputeLevel(code);

        GoverningCircular = governingCircular;
        EffectiveFrom = fromDate;
        EffectiveTo = effectiveTo;

        RequiresPartner = requiresPartner;
        RequiresWarehouse = requiresWarehouse;
        RequiresCostCenter = requiresCostCenter;
        RequiresProject = requiresProject;
    }

    public static int ComputeLevel(string code)
    {
        var len = code.Trim().Length;
        if (len <= 3) return 1;
        if (len == 4) return 2;
        return 3;
    }

    public void AddSubAccount(Account subAccount)
    {
        if (!subAccount.Id.Value.StartsWith(Id.Value, StringComparison.OrdinalIgnoreCase))
            throw new InvalidOperationException($"Child account '{subAccount.Id.Value}' must start with parent code '{Id.Value}'.");

        _subAccounts.Add(subAccount);
        IsParent = true;
    }

    public void MarkAsParent() => IsParent = true;
    public void UnmarkAsParent() => IsParent = false;
    public void SetActive(bool isActive) => IsActive = isActive;

    public void SetTemporalValidity(DateOnly effectiveFrom, DateOnly? effectiveTo)
    {
        if (effectiveTo.HasValue && effectiveTo.Value < effectiveFrom)
            throw new ArgumentException("EffectiveTo date cannot be prior to EffectiveFrom date.", nameof(effectiveTo));

        EffectiveFrom = effectiveFrom;
        EffectiveTo = effectiveTo;
    }

    public void UpdateDetails(
        string accountName,
        AccountType accountType,
        BalanceNature balanceNature,
        bool requiresPartner,
        bool requiresWarehouse,
        bool requiresCostCenter,
        bool requiresProject,
        GoverningCircular governingCircular = GoverningCircular.TT99_2025_BTC,
        DateOnly? effectiveFrom = null,
        DateOnly? effectiveTo = null)
    {
        AccountName = accountName.Trim();
        AccountType = accountType;
        BalanceNature = balanceNature;
        RequiresPartner = requiresPartner;
        RequiresWarehouse = requiresWarehouse;
        RequiresCostCenter = requiresCostCenter;
        RequiresProject = requiresProject;
        GoverningCircular = governingCircular;

        if (effectiveFrom.HasValue)
            EffectiveFrom = effectiveFrom.Value;
        EffectiveTo = effectiveTo;
    }
}

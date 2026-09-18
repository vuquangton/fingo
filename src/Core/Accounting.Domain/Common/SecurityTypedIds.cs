namespace Accounting.Domain.Common;

public readonly record struct UserId(Guid Value)
{
    public static UserId New() => new(Guid.NewGuid());
    public static UserId Empty => new(Guid.Empty);
    public override string ToString() => Value.ToString();

    public static implicit operator Guid(UserId id) => id.Value;
    public static explicit operator UserId(Guid id) => new(id);
}

public readonly record struct RoleId(Guid Value)
{
    public static RoleId New() => new(Guid.NewGuid());
    public static RoleId Empty => new(Guid.Empty);
    public override string ToString() => Value.ToString();

    public static implicit operator Guid(RoleId id) => id.Value;
    public static explicit operator RoleId(Guid id) => new(id);
}

public readonly record struct BackupRecordId(Guid Value)
{
    public static BackupRecordId New() => new(Guid.NewGuid());
    public static BackupRecordId Empty => new(Guid.Empty);
    public override string ToString() => Value.ToString();

    public static implicit operator Guid(BackupRecordId id) => id.Value;
    public static explicit operator BackupRecordId(Guid id) => new(id);
}

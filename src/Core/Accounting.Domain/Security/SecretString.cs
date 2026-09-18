namespace Accounting.Domain.Security;

public readonly record struct SecretString
{
    private readonly string _value;

    public SecretString(string value)
    {
        _value = value ?? string.Empty;
    }

    public string Reveal() => _value;
    public bool IsEmpty => string.IsNullOrEmpty(_value);

    public override string ToString() => "***";

    public static implicit operator SecretString(string value) => new(value);
}

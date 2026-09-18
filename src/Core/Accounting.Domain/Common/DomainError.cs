namespace Accounting.Domain.Common;

public readonly record struct DomainError(string Code, string Message)
{
    public static DomainError None => new(string.Empty, string.Empty);
    public static DomainError NotFound(string message) => new("NOT_FOUND", message);
    public static DomainError Validation(string message) => new("VALIDATION_FAILED", message);
    public static DomainError Conflict(string message) => new("CONFLICT", message);
    public static DomainError RuleViolation(string message) => new("RULE_VIOLATION", message);
}

public readonly struct Result<T, TError>
{
    public bool IsSuccess { get; }
    public bool IsFailure => !IsSuccess;
    public T Value { get; }
    public TError Error { get; }

    private Result(T value)
    {
        IsSuccess = true;
        Value = value;
        Error = default!;
    }

    private Result(TError error)
    {
        IsSuccess = false;
        Value = default!;
        Error = error;
    }

    public static Result<T, TError> Success(T value) => new(value);
    public static Result<T, TError> Failure(TError error) => new(error);

    public TResult Match<TResult>(Func<T, TResult> onSuccess, Func<TError, TResult> onFailure) =>
        IsSuccess ? onSuccess(Value) : onFailure(Error);
}

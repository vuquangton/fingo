namespace Accounting.Application.Common.Models;

public class Result
{
    public bool IsSuccess { get; }
    public string? ErrorMessage { get; }
    public IReadOnlyList<string> Errors { get; }

    protected Result(bool isSuccess, string? errorMessage, IEnumerable<string>? errors = null)
    {
        IsSuccess = isSuccess;
        ErrorMessage = errorMessage;
        Errors = errors?.ToList().AsReadOnly() ?? (errorMessage != null ? [errorMessage] : []);
    }

    public static Result Success() => new(true, null);
    public static Result Failure(string errorMessage) => new(false, errorMessage);
    public static Result Failure(IEnumerable<string> errors) => new(false, string.Join("; ", errors), errors);
}

public class Result<T> : Result
{
    public T? Value { get; }

    private Result(bool isSuccess, T? value, string? errorMessage, IEnumerable<string>? errors = null)
        : base(isSuccess, errorMessage, errors)
    {
        Value = value;
    }

    public static Result<T> Success(T value) => new(true, value, null);
    public new static Result<T> Failure(string errorMessage) => new(false, default, errorMessage);
    public new static Result<T> Failure(IEnumerable<string> errors) => new(false, default, string.Join("; ", errors), errors);
}

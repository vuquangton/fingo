using FluentValidation;

namespace Accounting.Application.Features.Ledger;

public class CreateVoucherCommandValidator : AbstractValidator<CreateVoucherCommand>
{
    public CreateVoucherCommandValidator()
    {
        RuleFor(x => x.VoucherNumber)
            .NotEmpty().WithMessage("Voucher number is required.")
            .MaximumLength(50).WithMessage("Voucher number cannot exceed 50 characters.");

        RuleFor(x => x.Description)
            .NotEmpty().WithMessage("Voucher description is required.")
            .MaximumLength(500).WithMessage("Voucher description cannot exceed 500 characters.");

        RuleFor(x => x.ExchangeRate)
            .GreaterThan(0).WithMessage("Exchange rate must be strictly positive.");

        RuleFor(x => x.Lines)
            .NotNull().WithMessage("Voucher must contain transaction lines.")
            .Must(lines => lines != null && lines.Count >= 2)
            .WithMessage("A double-entry voucher must contain at least 2 lines (Debit and Credit).");

        RuleForEach(x => x.Lines).ChildRules(line =>
        {
            line.RuleFor(l => l.AccountCode)
                .NotEmpty().WithMessage("Line account code is required.");

            line.RuleFor(l => l.AmountOriginal)
                .GreaterThan(0).WithMessage("Line amount must be greater than zero.");

            line.RuleFor(l => l.Description)
                .NotEmpty().WithMessage("Line description is required.");
        });
    }
}

public class PostVoucherCommandValidator : AbstractValidator<PostVoucherCommand>
{
    public PostVoucherCommandValidator()
    {
        RuleFor(x => x.VoucherId.Value)
            .NotEmpty().WithMessage("Valid VoucherId is required.");

        RuleFor(x => x.PostedBy)
            .NotEmpty().WithMessage("Posted by user is required.");
    }
}

public class ReverseVoucherCommandValidator : AbstractValidator<ReverseVoucherCommand>
{
    public ReverseVoucherCommandValidator()
    {
        RuleFor(x => x.VoucherId.Value)
            .NotEmpty().WithMessage("Valid VoucherId is required.");

        RuleFor(x => x.NewVoucherNumber)
            .NotEmpty().WithMessage("Reversal voucher number is required.");

        RuleFor(x => x.ReversedBy)
            .NotEmpty().WithMessage("Reversed by user is required.");

        RuleFor(x => x.Reason)
            .NotEmpty().WithMessage("Reversal reason is required.");
    }
}

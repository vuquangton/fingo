using Accounting.Application.Common.Models;
using MediatR;

namespace Accounting.Application.Features.Books;

public record CommonBookQuery(
    DateOnly FromDate,
    DateOnly ToDate,
    string? AccountNumber = null,
    Guid? PartnerId = null,
    Guid? WarehouseId = null);

public record S03aJournalLineDto(
    DateOnly PostingDate,
    DateOnly VoucherDate,
    string VoucherNumber,
    string Description,
    string DebitAccountNumber,
    string CreditAccountNumber,
    decimal Amount);

public record S03aJournalBookDto(
    string Title,
    DateOnly FromDate,
    DateOnly ToDate,
    IReadOnlyList<S03aJournalLineDto> Lines,
    decimal TotalAmount,
    Accounting.Application.Common.Interfaces.ReportHeaderInfo? Header = null);

public record GetS03aJournalBookQuery(DateOnly FromDate, DateOnly ToDate) : IRequest<Result<S03aJournalBookDto>>;

public record S03bGeneralLedgerLineDto(
    DateOnly PostingDate,
    string VoucherNumber,
    DateOnly VoucherDate,
    string Description,
    string CorrespAccountNumber,
    decimal DebitAmount,
    decimal CreditAmount,
    decimal RunningBalance);

public record S03bGeneralLedgerBookDto(
    string AccountNumber,
    string AccountName,
    DateOnly FromDate,
    DateOnly ToDate,
    decimal OpeningDebit,
    decimal OpeningCredit,
    IReadOnlyList<S03bGeneralLedgerLineDto> Lines,
    decimal TotalDebitMovement,
    decimal TotalCreditMovement,
    decimal ClosingDebit,
    decimal ClosingCredit,
    Accounting.Application.Common.Interfaces.ReportHeaderInfo? Header = null);

public record GetS03bGeneralLedgerBookQuery(string AccountNumber, DateOnly FromDate, DateOnly ToDate) : IRequest<Result<S03bGeneralLedgerBookDto>>;

public record DetailedPartnerBookLineDto(
    DateOnly PostingDate,
    string VoucherNumber,
    string Description,
    string CorrespAccountNumber,
    decimal DebitAmount,
    decimal CreditAmount,
    decimal ClosingBalance);

public record DetailedPartnerBookDto(
    string AccountNumber,
    Guid PartnerId,
    string PartnerCode,
    string PartnerName,
    DateOnly FromDate,
    DateOnly ToDate,
    decimal OpeningBalance,
    IReadOnlyList<DetailedPartnerBookLineDto> Lines,
    decimal TotalDebit,
    decimal TotalCredit,
    decimal ClosingBalance);

public record GetDetailedPartnerBookQuery(string AccountNumber, Guid PartnerId, DateOnly FromDate, DateOnly ToDate) : IRequest<Result<DetailedPartnerBookDto>>;

public record DetailedInventoryBookLineDto(
    DateOnly PostingDate,
    string VoucherNumber,
    string Description,
    string CorrespAccountNumber,
    decimal InQuantity,
    decimal InAmount,
    decimal OutQuantity,
    decimal OutAmount,
    decimal ClosingQuantity,
    decimal ClosingAmount);

public record DetailedInventoryBookDto(
    string WarehouseId,
    string WarehouseName,
    Guid InventoryItemId,
    string ItemCode,
    string ItemName,
    string UnitName,
    DateOnly FromDate,
    DateOnly ToDate,
    decimal OpeningQuantity,
    decimal OpeningAmount,
    IReadOnlyList<DetailedInventoryBookLineDto> Lines,
    decimal TotalInQuantity,
    decimal TotalInAmount,
    decimal TotalOutQuantity,
    decimal TotalOutAmount,
    decimal ClosingQuantity,
    decimal ClosingAmount);

public record GetDetailedInventoryBookQuery(string WarehouseId, Guid InventoryItemId, DateOnly FromDate, DateOnly ToDate) : IRequest<Result<DetailedInventoryBookDto>>;

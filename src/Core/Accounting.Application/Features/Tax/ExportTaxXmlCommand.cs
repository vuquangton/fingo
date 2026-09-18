using Accounting.Application.Common.Interfaces;
using Accounting.Application.Common.Models;
using Accounting.Domain.Exceptions;
using Accounting.Domain.Tax;
using MediatR;
using Microsoft.EntityFrameworkCore;

namespace Accounting.Application.Features.Tax;

public record ExportTaxXmlCommand(
    int Year,
    int Month,
    string? CompanyTaxCode = null,
    string? CompanyName = null,
    string? DirectorName = null,
    TaxDeclarationType TaxType = TaxDeclarationType.VAT) : IRequest<Result<TaxExportDto>>;

public record TaxExportDto(
    Guid SnapshotId,
    string Period,
    decimal InputVat,
    decimal OutputVat,
    decimal VatPayable,
    decimal VatCarriedForward,
    string XmlContent);

public class ExportTaxXmlCommandHandler(IAccountingDbContext dbContext) : IRequestHandler<ExportTaxXmlCommand, Result<TaxExportDto>>
{
    public async Task<Result<TaxExportDto>> Handle(ExportTaxXmlCommand request, CancellationToken cancellationToken)
    {
        var company = await dbContext.CompanySettings.AsNoTracking().FirstOrDefaultAsync(cancellationToken);

        var taxCode = !string.IsNullOrWhiteSpace(request.CompanyTaxCode) ? request.CompanyTaxCode : company?.TaxCode;
        var compName = !string.IsNullOrWhiteSpace(request.CompanyName) ? request.CompanyName : company?.CompanyName;
        var director = !string.IsNullOrWhiteSpace(request.DirectorName) ? request.DirectorName : company?.LegalRepresentative;

        // 1. Mandatory metadata validation (Matt Pocock type-safety & statutory rule)
        if (string.IsNullOrWhiteSpace(taxCode))
            throw new InvalidTaxDeclarationException("Company tax code (Mã số thuế) is mandatory.");

        if (string.IsNullOrWhiteSpace(compName))
            throw new InvalidTaxDeclarationException("Company name (Tên người nộp thuế) is mandatory.");

        if (string.IsNullOrWhiteSpace(director))
            throw new InvalidTaxDeclarationException("Director name (Người ký đại diện pháp luật) is mandatory.");

        var startDate = new DateOnly(request.Year, request.Month, 1);
        var endDate = startDate.AddMonths(1).AddDays(-1);
        var periodString = $"{request.Month:D2}/{request.Year}";

        // 2. Aggregate VAT entries from general ledger
        var entries = await dbContext.GeneralLedgerEntries
            .Where(e => e.PostingDate >= startDate && e.PostingDate <= endDate)
            .ToListAsync(cancellationToken);

        // Input VAT (TK 133): Net Debit = Debit - Credit
        var inputVatDebit = entries.Where(e => e.AccountId.Value.StartsWith("133")).Sum(e => e.DebitAmount);
        var inputVatCredit = entries.Where(e => e.AccountId.Value.StartsWith("133")).Sum(e => e.CreditAmount);
        var inputVat = Math.Max(0m, inputVatDebit - inputVatCredit);

        // Output VAT (TK 3331): Net Credit = Credit - Debit
        var outputVatCredit = entries.Where(e => e.AccountId.Value.StartsWith("3331")).Sum(e => e.CreditAmount);
        var outputVatDebit = entries.Where(e => e.AccountId.Value.StartsWith("3331")).Sum(e => e.DebitAmount);
        var outputVat = Math.Max(0m, outputVatCredit - outputVatDebit);

        // Net VAT calculation
        decimal vatPayable = 0m;
        decimal vatCarriedForward = 0m;

        if (outputVat >= inputVat)
        {
            vatPayable = outputVat - inputVat;
        }
        else
        {
            vatCarriedForward = inputVat - outputVat;
        }

        // 3. Populate typed HTKK XML Document
        var doc = new HtkkTaxDocument
        {
            TaxReturn = new HtkkTaxReturn
            {
                GeneralInfo = new HtkkGeneralInfo
                {
                    SchemaVersion = "2.1.0",
                    ReturnCode = "01/GTGT",
                    ReturnName = "T? KHAI THU? GI� TR? GIA TANG (M?u 01/GTGT)",
                    TaxPeriod = periodString,
                    TaxCode = taxCode.Trim(),
                    TaxpayerName = compName.Trim(),
                    SignerName = director.Trim(),
                    SignDate = DateTime.UtcNow.ToString("yyyy-MM-dd")
                },
                MainDetails = new HtkkMainDetails
                {
                    InputVatAmount = inputVat,
                    OutputVatAmount = outputVat,
                    VatPayable = vatPayable,
                    VatCarriedForward = vatCarriedForward
                }
            }
        };

        var xmlPayload = HtkkTaxDocument.SerializeToXml(doc);

        // 4. Create and persist TaxDeclarationSnapshot
        var snapshot = new TaxDeclarationSnapshot(
            Guid.NewGuid(),
            request.TaxType,
            periodString,
            taxCode,
            compName,
            director,
            xmlPayload);

        dbContext.AddEntity(snapshot);
        await dbContext.SaveChangesAsync(cancellationToken);

        var dto = new TaxExportDto(
            snapshot.Id,
            periodString,
            inputVat,
            outputVat,
            vatPayable,
            vatCarriedForward,
            xmlPayload);

        return Result<TaxExportDto>.Success(dto);
    }
}

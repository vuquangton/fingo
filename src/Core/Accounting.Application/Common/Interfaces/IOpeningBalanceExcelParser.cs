using Accounting.Application.Common.Models;
using Accounting.Application.Features.OpeningBalance;

namespace Accounting.Application.Common.Interfaces;

public interface IOpeningBalanceExcelParser
{
    Result<List<SaveOpeningBalanceEntryDto>> ParseOpeningBalances(Stream excelStream, int fiscalYear);
}

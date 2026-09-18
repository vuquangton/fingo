using System.IO;
using Accounting.Application.Common.Interfaces;
using Accounting.Application.Common.Models;
using Accounting.Application.Features.OpeningBalance;
using ClosedXML.Excel;

namespace Accounting.Infrastructure.Compliance.Services;

public class OpeningBalanceExcelParser : IOpeningBalanceExcelParser
{
    public Result<List<SaveOpeningBalanceEntryDto>> ParseOpeningBalances(Stream excelStream, int fiscalYear)
    {
        try
        {
            using var workbook = new XLWorkbook(excelStream);
            var worksheet = workbook.Worksheets.FirstOrDefault();
            if (worksheet == null)
            {
                return Result<List<SaveOpeningBalanceEntryDto>>.Failure("Tệp Excel không chứa trang tính (worksheet) nào.");
            }

            var entries = new List<SaveOpeningBalanceEntryDto>();
            var rows = worksheet.RangeUsed()?.RowsUsed()?.ToList();
            if (rows == null || rows.Count <= 1)
            {
                return Result<List<SaveOpeningBalanceEntryDto>>.Failure("Tệp Excel không có dòng dữ liệu số dư.");
            }

            // Detect header row or start at row 2
            // Expected columns:
            // Col 1: Mã TK (AccountId)
            // Col 2: Dư Nợ (DebitAmount)
            // Col 3: Dư Có (CreditAmount)
            // Col 4: Mã Đối Tác (PartnerId - Guid optional)
            // Col 5: Mã Kho (WarehouseId - text optional)
            // Col 6: Mã Vật Tư (InventoryItemId - Guid optional)
            // Col 7: Số Lượng (Quantity optional)
            // Col 8: Đơn Giá (UnitPrice optional)
            // Col 9: Diễn Giải (Description optional)

            for (int r = 2; r <= rows.Count; r++)
            {
                var row = worksheet.Row(r);
                var accCell = row.Cell(1).GetString().Trim();
                if (string.IsNullOrWhiteSpace(accCell)) continue; // skip blank rows

                decimal debit = 0m;
                if (!row.Cell(2).IsEmpty())
                {
                    if (!decimal.TryParse(row.Cell(2).GetString().Trim(), out debit))
                        debit = (decimal)row.Cell(2).GetDouble();
                }

                decimal credit = 0m;
                if (!row.Cell(3).IsEmpty())
                {
                    if (!decimal.TryParse(row.Cell(3).GetString().Trim(), out credit))
                        credit = (decimal)row.Cell(3).GetDouble();
                }

                Guid? partnerId = null;
                var partnerStr = row.Cell(4).GetString().Trim();
                if (Guid.TryParse(partnerStr, out var pId)) partnerId = pId;

                var warehouseId = row.Cell(5).GetString().Trim();
                if (string.IsNullOrWhiteSpace(warehouseId)) warehouseId = null;

                Guid? itemId = null;
                var itemStr = row.Cell(6).GetString().Trim();
                if (Guid.TryParse(itemStr, out var itId)) itemId = itId;

                decimal qty = 0m;
                if (!row.Cell(7).IsEmpty())
                {
                    if (!decimal.TryParse(row.Cell(7).GetString().Trim(), out qty))
                        qty = (decimal)row.Cell(7).GetDouble();
                }

                decimal price = 0m;
                if (!row.Cell(8).IsEmpty())
                {
                    if (!decimal.TryParse(row.Cell(8).GetString().Trim(), out price))
                        price = (decimal)row.Cell(8).GetDouble();
                }

                var desc = row.Cell(9).GetString().Trim();
                if (string.IsNullOrWhiteSpace(desc)) desc = $"Số dư đầu kỳ {accCell}";

                entries.Add(new SaveOpeningBalanceEntryDto(
                    AccountId: accCell,
                    DebitAmount: debit,
                    CreditAmount: credit,
                    PartnerId: partnerId,
                    WarehouseId: warehouseId,
                    InventoryItemId: itemId,
                    Quantity: qty,
                    UnitPrice: price,
                    Description: desc));
            }

            if (entries.Count == 0)
            {
                return Result<List<SaveOpeningBalanceEntryDto>>.Failure("Không tìm thấy dữ liệu số dư hợp lệ trong tệp Excel.");
            }

            return Result<List<SaveOpeningBalanceEntryDto>>.Success(entries);
        }
        catch (Exception ex)
        {
            return Result<List<SaveOpeningBalanceEntryDto>>.Failure($"Lỗi cấu trúc tệp Excel: {ex.Message}");
        }
    }
}

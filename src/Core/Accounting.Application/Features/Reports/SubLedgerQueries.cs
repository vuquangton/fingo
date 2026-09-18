using System.Data;
using Accounting.Application.Common.Interfaces;
using Accounting.Application.Common.Models;
using Dapper;
using MediatR;

namespace Accounting.Application.Features.Reports;

public record CustomerAgingRowDto(
    Guid CustomerId,
    string CustomerName,
    decimal CurrentBalance,
    decimal Overdue1To30,
    decimal Overdue31To60,
    decimal Overdue61To90,
    decimal OverdueOver90,
    decimal TotalBalance);

public record GetCustomerAgingScheduleQuery(DateOnly AsOfDate)
    : IRequest<Result<IReadOnlyList<CustomerAgingRowDto>>>;

public class GetCustomerAgingScheduleQueryHandler(ISqlConnectionFactory connectionFactory)
    : IRequestHandler<GetCustomerAgingScheduleQuery, Result<IReadOnlyList<CustomerAgingRowDto>>>
{
    private class RawCustomerAgingRow
    {
        public object? CustomerId { get; set; }
        public string CustomerName { get; set; } = string.Empty;
        public object? CurrentBalance { get; set; }
        public object? Overdue1To30 { get; set; }
        public object? Overdue31To60 { get; set; }
        public object? Overdue61To90 { get; set; }
        public object? OverdueOver90 { get; set; }
        public object? TotalBalance { get; set; }

        public Guid CustomerIdVal => CustomerId switch
        {
            Guid g => g,
            string s when Guid.TryParse(s, out var parsed) => parsed,
            byte[] b when b.Length == 16 => new Guid(b),
            _ => Guid.Empty
        };

        public decimal CurrentVal => Convert.ToDecimal(CurrentBalance ?? 0m);
        public decimal Overdue1To30Val => Convert.ToDecimal(Overdue1To30 ?? 0m);
        public decimal Overdue31To60Val => Convert.ToDecimal(Overdue31To60 ?? 0m);
        public decimal Overdue61To90Val => Convert.ToDecimal(Overdue61To90 ?? 0m);
        public decimal OverdueOver90Val => Convert.ToDecimal(OverdueOver90 ?? 0m);
        public decimal TotalVal => Convert.ToDecimal(TotalBalance ?? 0m);
    }

    public async Task<Result<IReadOnlyList<CustomerAgingRowDto>>> Handle(GetCustomerAgingScheduleQuery request, CancellationToken cancellationToken)
    {
        using var conn = connectionFactory.CreateConnection();
        if (conn.State != ConnectionState.Open)
            conn.Open();

        var asOfDateStr = request.AsOfDate.ToString("yyyy-MM-dd");

        // SQL portable across SQLite and MySQL
        const string salesSql = @"
            SELECT 
                i.CustomerId,
                COALESCE(p.Name, 'Unknown Customer') AS CustomerName,
                SUM(CASE WHEN i.DueDate >= @AsOfDate THEN (i.TotalAmount - i.ReceivedAmount) ELSE 0 END) AS CurrentBalance,
                SUM(CASE WHEN i.DueDate < @AsOfDate AND i.DueDate >= @D30 THEN (i.TotalAmount - i.ReceivedAmount) ELSE 0 END) AS Overdue1To30,
                SUM(CASE WHEN i.DueDate < @D30 AND i.DueDate >= @D60 THEN (i.TotalAmount - i.ReceivedAmount) ELSE 0 END) AS Overdue31To60,
                SUM(CASE WHEN i.DueDate < @D60 AND i.DueDate >= @D90 THEN (i.TotalAmount - i.ReceivedAmount) ELSE 0 END) AS Overdue61To90,
                SUM(CASE WHEN i.DueDate < @D90 THEN (i.TotalAmount - i.ReceivedAmount) ELSE 0 END) AS OverdueOver90,
                SUM(i.TotalAmount - i.ReceivedAmount) AS TotalBalance
            FROM sub_sales_invoices i
            LEFT JOIN md_business_partners p ON i.CustomerId = p.Id
            WHERE i.Status != 3 AND i.Status != 4 AND (i.TotalAmount - i.ReceivedAmount) > 0
            GROUP BY i.CustomerId, p.Name
            ORDER BY TotalBalance DESC;";

        var param = new
        {
            AsOfDate = asOfDateStr,
            D30 = request.AsOfDate.AddDays(-30).ToString("yyyy-MM-dd"),
            D60 = request.AsOfDate.AddDays(-60).ToString("yyyy-MM-dd"),
            D90 = request.AsOfDate.AddDays(-90).ToString("yyyy-MM-dd")
        };

        var rawRows = (await conn.QueryAsync<RawCustomerAgingRow>(salesSql, param)).ToList();

        var result = rawRows.Select(r => new CustomerAgingRowDto(
            r.CustomerIdVal,
            r.CustomerName,
            r.CurrentVal,
            r.Overdue1To30Val,
            r.Overdue31To60Val,
            r.Overdue61To90Val,
            r.OverdueOver90Val,
            r.TotalVal)).ToList();

        return Result<IReadOnlyList<CustomerAgingRowDto>>.Success(result);
    }
}

public record VendorAgingRowDto(
    Guid VendorId,
    string VendorName,
    decimal CurrentBalance,
    decimal Overdue1To30,
    decimal Overdue31To60,
    decimal OverdueOver60,
    decimal TotalBalance);

public record GetVendorAgingScheduleQuery(DateOnly AsOfDate)
    : IRequest<Result<IReadOnlyList<VendorAgingRowDto>>>;

public class GetVendorAgingScheduleQueryHandler(ISqlConnectionFactory connectionFactory)
    : IRequestHandler<GetVendorAgingScheduleQuery, Result<IReadOnlyList<VendorAgingRowDto>>>
{
    private class RawVendorAgingRow
    {
        public object? VendorId { get; set; }
        public string VendorName { get; set; } = string.Empty;
        public object? CurrentBalance { get; set; }
        public object? Overdue1To30 { get; set; }
        public object? Overdue31To60 { get; set; }
        public object? OverdueOver60 { get; set; }
        public object? TotalBalance { get; set; }

        public Guid VendorIdVal => VendorId switch
        {
            Guid g => g,
            string s when Guid.TryParse(s, out var parsed) => parsed,
            byte[] b when b.Length == 16 => new Guid(b),
            _ => Guid.Empty
        };

        public decimal CurrentVal => Convert.ToDecimal(CurrentBalance ?? 0m);
        public decimal Overdue1To30Val => Convert.ToDecimal(Overdue1To30 ?? 0m);
        public decimal Overdue31To60Val => Convert.ToDecimal(Overdue31To60 ?? 0m);
        public decimal OverdueOver60Val => Convert.ToDecimal(OverdueOver60 ?? 0m);
        public decimal TotalVal => Convert.ToDecimal(TotalBalance ?? 0m);
    }

    public async Task<Result<IReadOnlyList<VendorAgingRowDto>>> Handle(GetVendorAgingScheduleQuery request, CancellationToken cancellationToken)
    {
        using var conn = connectionFactory.CreateConnection();
        if (conn.State != ConnectionState.Open)
            conn.Open();

        const string sql = @"
            SELECT 
                i.VendorId,
                COALESCE(p.Name, 'Unknown Vendor') AS VendorName,
                SUM(CASE WHEN i.DueDate >= @AsOfDate THEN (i.TotalAmount - i.PaidAmount) ELSE 0 END) AS CurrentBalance,
                SUM(CASE WHEN i.DueDate < @AsOfDate AND i.DueDate >= @D30 THEN (i.TotalAmount - i.PaidAmount) ELSE 0 END) AS Overdue1To30,
                SUM(CASE WHEN i.DueDate < @D30 AND i.DueDate >= @D60 THEN (i.TotalAmount - i.PaidAmount) ELSE 0 END) AS Overdue31To60,
                SUM(CASE WHEN i.DueDate < @D60 THEN (i.TotalAmount - i.PaidAmount) ELSE 0 END) AS OverdueOver60,
                SUM(i.TotalAmount - i.PaidAmount) AS TotalBalance
            FROM sub_purchase_invoices i
            LEFT JOIN md_business_partners p ON i.VendorId = p.Id
            WHERE i.Status != 3 AND i.Status != 4 AND (i.TotalAmount - i.PaidAmount) > 0
            GROUP BY i.VendorId, p.Name
            ORDER BY TotalBalance DESC;";

        var param = new
        {
            AsOfDate = request.AsOfDate.ToString("yyyy-MM-dd"),
            D30 = request.AsOfDate.AddDays(-30).ToString("yyyy-MM-dd"),
            D60 = request.AsOfDate.AddDays(-60).ToString("yyyy-MM-dd")
        };

        var rawRows = (await conn.QueryAsync<RawVendorAgingRow>(sql, param)).ToList();

        var result = rawRows.Select(r => new VendorAgingRowDto(
            r.VendorIdVal,
            r.VendorName,
            r.CurrentVal,
            r.Overdue1To30Val,
            r.Overdue31To60Val,
            r.OverdueOver60Val,
            r.TotalVal)).ToList();

        return Result<IReadOnlyList<VendorAgingRowDto>>.Success(result);
    }
}

public record StockBalanceRowDto(
    string WarehouseId,
    Guid InventoryItemId,
    string ItemName,
    string UnitName,
    decimal InwardQuantity,
    decimal OutwardQuantity,
    decimal ClosingQuantity,
    decimal ClosingValue);

public record GetStockBalanceReportQuery(string? WarehouseId = null)
    : IRequest<Result<IReadOnlyList<StockBalanceRowDto>>>;

public class GetStockBalanceReportQueryHandler(ISqlConnectionFactory connectionFactory)
    : IRequestHandler<GetStockBalanceReportQuery, Result<IReadOnlyList<StockBalanceRowDto>>>
{
    private class RawStockBalanceRow
    {
        public string WarehouseId { get; set; } = string.Empty;
        public object? InventoryItemId { get; set; }
        public string ItemName { get; set; } = string.Empty;
        public string UnitName { get; set; } = string.Empty;
        public object? InwardQuantity { get; set; }
        public object? OutwardQuantity { get; set; }
        public object? ClosingValue { get; set; }

        public Guid InventoryItemIdVal => InventoryItemId switch
        {
            Guid g => g,
            string s when Guid.TryParse(s, out var parsed) => parsed,
            byte[] b when b.Length == 16 => new Guid(b),
            _ => Guid.Empty
        };

        public decimal InwardVal => Convert.ToDecimal(InwardQuantity ?? 0m);
        public decimal OutwardVal => Convert.ToDecimal(OutwardQuantity ?? 0m);
        public decimal ValueVal => Convert.ToDecimal(ClosingValue ?? 0m);
    }

    public async Task<Result<IReadOnlyList<StockBalanceRowDto>>> Handle(GetStockBalanceReportQuery request, CancellationToken cancellationToken)
    {
        using var conn = connectionFactory.CreateConnection();
        if (conn.State != ConnectionState.Open)
            conn.Open();

        // Outward voucher types: 4 (OutwardSales), 5 (OutwardProduction), 6 (OutwardInternal)
        const string sql = @"
            SELECT 
                v.WarehouseId,
                l.InventoryItemId,
                COALESCE(i.ItemName, 'Unknown Item') AS ItemName,
                COALESCE(u.UomName, 'Cái') AS UnitName,
                SUM(CASE WHEN v.VoucherType NOT IN (4, 5, 6) THEN l.Quantity ELSE 0 END) AS InwardQuantity,
                SUM(CASE WHEN v.VoucherType IN (4, 5, 6) THEN l.Quantity ELSE 0 END) AS OutwardQuantity,
                SUM(CASE WHEN v.VoucherType NOT IN (4, 5, 6) THEN l.TotalAmount ELSE -l.TotalAmount END) AS ClosingValue
            FROM sub_warehouse_vouchers v
            INNER JOIN sub_warehouse_voucher_lines l ON v.Id = l.VoucherId
            LEFT JOIN md_inventory_items i ON l.InventoryItemId = i.Id
            LEFT JOIN md_units_of_measure u ON l.UnitOfMeasureId = u.Id
            WHERE (@WarehouseId IS NULL OR v.WarehouseId = @WarehouseId)
            GROUP BY v.WarehouseId, l.InventoryItemId, i.ItemName, u.UomName
            ORDER BY v.WarehouseId, l.InventoryItemId;";

        var param = new { WarehouseId = string.IsNullOrWhiteSpace(request.WarehouseId) ? null : request.WarehouseId.Trim() };

        var rawRows = (await conn.QueryAsync<RawStockBalanceRow>(sql, param)).ToList();

        var result = rawRows.Select(r =>
        {
            var inQty = r.InwardVal;
            var outQty = r.OutwardVal;
            var closingQty = inQty - outQty;
            return new StockBalanceRowDto(
                r.WarehouseId,
                r.InventoryItemIdVal,
                r.ItemName,
                r.UnitName,
                inQty,
                outQty,
                closingQty,
                r.ValueVal);
        }).ToList();

        return Result<IReadOnlyList<StockBalanceRowDto>>.Success(result);
    }
}

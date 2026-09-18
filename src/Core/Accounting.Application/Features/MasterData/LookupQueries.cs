using Accounting.Application.Common.Interfaces;
using Accounting.Application.Common.Models;
using Accounting.Domain.Entities.Inventory;
using Accounting.Domain.Entities.Treasury;
using Accounting.Domain.MasterData.Common;
using MediatR;
using Microsoft.EntityFrameworkCore;

namespace Accounting.Application.Features.MasterData;

public record VendorItemDto(Guid Id, string Code, string Name, string TaxCode, string? Phone, decimal CurrentPayableBalance);
public record GetVendorsQuery : IRequest<Result<List<VendorItemDto>>>;

public class GetVendorsQueryHandler(IAccountingDbContext context) : IRequestHandler<GetVendorsQuery, Result<List<VendorItemDto>>>
{
    public async Task<Result<List<VendorItemDto>>> Handle(GetVendorsQuery request, CancellationToken cancellationToken)
    {
        var list = await context.BusinessPartners.AsNoTracking()
            .Where(p => p.IsActive && (p.PartnerType & PartnerType.Vendor) != 0)
            .OrderBy(p => p.PartnerCode)
            .Select(p => new VendorItemDto(p.Id.Value, p.PartnerCode, p.Name, p.TaxCode ?? string.Empty, p.ContactPhone, p.CurrentPayableBalance))
            .ToListAsync(cancellationToken);
        return Result<List<VendorItemDto>>.Success(list);
    }
}

public record CustomerItemDto(Guid Id, string Code, string Name, string TaxCode, string? Phone, decimal CreditLimit, decimal CurrentReceivableBalance);
public record GetCustomersQuery : IRequest<Result<List<CustomerItemDto>>>;

public class GetCustomersQueryHandler(IAccountingDbContext context) : IRequestHandler<GetCustomersQuery, Result<List<CustomerItemDto>>>
{
    public async Task<Result<List<CustomerItemDto>>> Handle(GetCustomersQuery request, CancellationToken cancellationToken)
    {
        var list = await context.BusinessPartners.AsNoTracking()
            .Where(p => p.IsActive && (p.PartnerType & PartnerType.Customer) != 0)
            .OrderBy(p => p.PartnerCode)
            .Select(p => new CustomerItemDto(p.Id.Value, p.PartnerCode, p.Name, p.TaxCode ?? string.Empty, p.ContactPhone, p.CreditLimit, p.CurrentReceivableBalance))
            .ToListAsync(cancellationToken);
        return Result<List<CustomerItemDto>>.Success(list);
    }
}

public record WarehouseItemDto(Guid Id, string Code, string Name, string? Address);
public record GetWarehousesQuery : IRequest<Result<List<WarehouseItemDto>>>;

public class GetWarehousesQueryHandler(IAccountingDbContext context) : IRequestHandler<GetWarehousesQuery, Result<List<WarehouseItemDto>>>
{
    public async Task<Result<List<WarehouseItemDto>>> Handle(GetWarehousesQuery request, CancellationToken cancellationToken)
    {
        var list = await context.Warehouses.AsNoTracking()
            .Where(w => w.IsActive)
            .OrderBy(w => w.Code)
            .Select(w => new WarehouseItemDto(w.Id, w.Code, w.Name, w.Address))
            .ToListAsync(cancellationToken);
        return Result<List<WarehouseItemDto>>.Success(list);
    }
}

public record BankAccountItemDto(Guid Id, string AccountNumber, string BankName, string? Branch, decimal CurrentBalance);
public record GetBankAccountsQuery : IRequest<Result<List<BankAccountItemDto>>>;

public class GetBankAccountsQueryHandler(IAccountingDbContext context) : IRequestHandler<GetBankAccountsQuery, Result<List<BankAccountItemDto>>>
{
    public async Task<Result<List<BankAccountItemDto>>> Handle(GetBankAccountsQuery request, CancellationToken cancellationToken)
    {
        var list = await context.BankAccounts.AsNoTracking()
            .Where(b => b.IsActive)
            .OrderBy(b => b.AccountNumber)
            .Select(b => new BankAccountItemDto(b.Id, b.AccountNumber, b.BankName, b.Branch, b.CurrentBalance))
            .ToListAsync(cancellationToken);
        return Result<List<BankAccountItemDto>>.Success(list);
    }
}

public record EmployeeItemDto(Guid Id, string Code, string FullName, string Department, string Position, decimal BaseSalary, decimal InsuranceSalary, int DependentsCount);
public record GetEmployeesQuery : IRequest<Result<List<EmployeeItemDto>>>;

public class GetEmployeesQueryHandler(IAccountingDbContext context) : IRequestHandler<GetEmployeesQuery, Result<List<EmployeeItemDto>>>
{
    public async Task<Result<List<EmployeeItemDto>>> Handle(GetEmployeesQuery request, CancellationToken cancellationToken)
    {
        var list = await context.Employees.AsNoTracking()
            .Where(e => e.IsActive)
            .OrderBy(e => e.Code)
            .Select(e => new EmployeeItemDto(e.Id, e.Code, e.FullName, e.Department, e.Position, e.BaseSalary, e.InsuranceSalary, e.DependentsCount))
            .ToListAsync(cancellationToken);
        return Result<List<EmployeeItemDto>>.Success(list);
    }
}

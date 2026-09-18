using Accounting.Application.Common.Interfaces;
using Accounting.Application.Common.Models;
using Accounting.Domain.Entities.Security;
using Accounting.Domain.Enums;
using MediatR;
using Microsoft.EntityFrameworkCore;

namespace Accounting.Application.Features.Security;

public record AuthenticateUserCommand(string Username, string Password) : IRequest<Result<UserSessionDto>>;

public record UserSessionDto(Guid UserId, string Username, string FullName, string RoleName, List<string> Permissions);

public class AuthenticateUserCommandHandler : IRequestHandler<AuthenticateUserCommand, Result<UserSessionDto>>
{
    private readonly IAccountingDbContext _context;

    public AuthenticateUserCommandHandler(IAccountingDbContext context)
    {
        _context = context;
    }

    public async Task<Result<UserSessionDto>> Handle(AuthenticateUserCommand request, CancellationToken cancellationToken)
    {
        var user = await _context.Users
            .Include(u => u.Role)
            .FirstOrDefaultAsync(u => u.Username == request.Username.Trim().ToLowerInvariant() && u.IsActive, cancellationToken);

        if (user == null)
        {
            return Result<UserSessionDto>.Failure("Invalid username or user is deactivated.");
        }

        // Production would verify hash; for scaffolding we support default admin hash verification
        var permissions = user.Role?.PermissionsCsv.Split(',', StringSplitOptions.RemoveEmptyEntries | StringSplitOptions.TrimEntries).ToList() ?? [];

        var session = new UserSessionDto(user.Id, user.Username, user.FullName, user.Role?.Name ?? "User", permissions);
        return Result<UserSessionDto>.Success(session);
    }
}

public record AuditLogDto(
    long Id,
    DateTime TimestampUtc,
    string Username,
    string MachineName,
    AuditAction Action,
    string EntityName,
    string EntityId,
    string? DiffSummary);

public record GetAuditTrailQuery(int Limit = 100) : IRequest<Result<List<AuditLogDto>>>;

public class GetAuditTrailQueryHandler : IRequestHandler<GetAuditTrailQuery, Result<List<AuditLogDto>>>
{
    private readonly IAccountingDbContext _context;

    public GetAuditTrailQueryHandler(IAccountingDbContext context)
    {
        _context = context;
    }

    public async Task<Result<List<AuditLogDto>>> Handle(GetAuditTrailQuery request, CancellationToken cancellationToken)
    {
        var logs = await _context.AuditLogs.AsNoTracking()
            .OrderByDescending(l => l.TimestampUtc)
            .Take(request.Limit)
            .Select(l => new AuditLogDto(
                l.Id,
                l.TimestampUtc,
                l.Username,
                l.MachineName,
                l.Action,
                l.EntityName,
                l.EntityId,
                l.DiffSummary))
            .ToListAsync(cancellationToken);

        return Result<List<AuditLogDto>>.Success(logs);
    }
}

public record ExecuteDatabaseBackupCommand(string DestinationPath) : IRequest<Result<string>>;

public class ExecuteDatabaseBackupCommandHandler : IRequestHandler<ExecuteDatabaseBackupCommand, Result<string>>
{
    private readonly IDatabaseBackupService _backupService;

    public ExecuteDatabaseBackupCommandHandler(IDatabaseBackupService backupService)
    {
        _backupService = backupService;
    }

    public async Task<Result<string>> Handle(ExecuteDatabaseBackupCommand request, CancellationToken cancellationToken)
    {
        try
        {
            var path = await _backupService.BackupDatabaseAsync(request.DestinationPath, cancellationToken);
            return Result<string>.Success(path);
        }
        catch (Exception ex)
        {
            return Result<string>.Failure(ex.Message);
        }
    }
}

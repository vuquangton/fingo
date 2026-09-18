using System.Data;
using Accounting.Application.Common.Interfaces;
using Accounting.Application.Common.Models;
using Accounting.Domain.Security;
using Dapper;
using MediatR;

namespace Accounting.Application.Features.Security;

public class AuditTrailItemDto
{
    public string Id { get; set; } = string.Empty;
    public DateTime TimestampUtc { get; set; }
    public string? UserId { get; set; }
    public string Username { get; set; } = string.Empty;
    public string MachineName { get; set; } = string.Empty;
    public string? IpAddress { get; set; }
    public string Action { get; set; } = string.Empty;
    public string EntityName { get; set; } = string.Empty;
    public string EntityId { get; set; } = string.Empty;
    public string? OldValues { get; set; }
    public string? NewValues { get; set; }
    public string? DiffSummary { get; set; }
}

public record AuditTrailPagedResultDto(
    IReadOnlyList<AuditTrailItemDto> Items,
    int TotalCount,
    int PageNumber,
    int PageSize);

[RequirePermission(AppPermission.ViewLedger)]
public record GetAuditTrailRecordsQuery(
    int PageNumber = 1,
    int PageSize = 50,
    string? EntityName = null,
    string? Username = null) : IRequest<Result<AuditTrailPagedResultDto>>;

public class GetAuditTrailRecordsQueryHandler(
    ISqlConnectionFactory connectionFactory) : IRequestHandler<GetAuditTrailRecordsQuery, Result<AuditTrailPagedResultDto>>
{
    public async Task<Result<AuditTrailPagedResultDto>> Handle(GetAuditTrailRecordsQuery request, CancellationToken cancellationToken)
    {
        using var conn = connectionFactory.CreateConnection();
        if (conn.State != ConnectionState.Open)
            conn.Open();

        var whereClauses = new List<string>();
        var parameters = new DynamicParameters();

        if (!string.IsNullOrWhiteSpace(request.EntityName))
        {
            whereClauses.Add("EntityName = @EntityName");
            parameters.Add("EntityName", request.EntityName);
        }

        if (!string.IsNullOrWhiteSpace(request.Username))
        {
            whereClauses.Add("Username = @Username");
            parameters.Add("Username", request.Username);
        }

        var whereSql = whereClauses.Count > 0 ? "WHERE " + string.Join(" AND ", whereClauses) : "";

        var countSql = $"SELECT COUNT(*) FROM audit_trails {whereSql};";
        var totalCount = await conn.ExecuteScalarAsync<int>(countSql, parameters);

        var offset = (Math.Max(1, request.PageNumber) - 1) * request.PageSize;
        parameters.Add("Offset", offset);
        parameters.Add("Limit", request.PageSize);

        var dataSql = $@"
            SELECT 
                Id, 
                TimestampUtc, 
                UserId, 
                Username, 
                MachineName, 
                IpAddress, 
                Action, 
                EntityName, 
                EntityId, 
                OldValues, 
                NewValues, 
                DiffSummary
            FROM audit_trails
            {whereSql}
            ORDER BY TimestampUtc DESC
            LIMIT @Limit OFFSET @Offset;";

        var rows = (await conn.QueryAsync<AuditTrailItemDto>(dataSql, parameters)).ToList();

        return Result<AuditTrailPagedResultDto>.Success(new AuditTrailPagedResultDto(
            rows,
            totalCount,
            request.PageNumber,
            request.PageSize));
    }
}

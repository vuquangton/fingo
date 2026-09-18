using Accounting.Application.Common.Interfaces;
using Accounting.Application.Common.Models;
using Accounting.Domain.Entities.Security;
using MediatR;
using Microsoft.EntityFrameworkCore;

namespace Accounting.Application.Features.Security;

public record UserDto(
    Guid Id,
    string Username,
    string FullName,
    string Email,
    Guid RoleId,
    string RoleName,
    bool IsActive,
    DateTime CreatedAtUtc);

public record RoleDto(Guid Id, string Name, string Description, string PermissionsCsv);

public record GetUsersQuery : IRequest<Result<List<UserDto>>>;

public record GetRolesQuery : IRequest<Result<List<RoleDto>>>;

public record CreateUserCommand(
    string Username,
    string Password,
    string FullName,
    string Email,
    Guid RoleId) : IRequest<Result<UserDto>>;

public record UpdateUserCommand(
    Guid UserId,
    string FullName,
    string Email,
    Guid RoleId) : IRequest<Result<UserDto>>;

public record SetUserStatusCommand(Guid UserId, bool IsActive) : IRequest<Result<bool>>;

public record ResetUserPasswordCommand(Guid UserId, string NewPassword) : IRequest<Result<bool>>;

public class UserManagementHandlers :
    IRequestHandler<GetUsersQuery, Result<List<UserDto>>>,
    IRequestHandler<GetRolesQuery, Result<List<RoleDto>>>,
    IRequestHandler<CreateUserCommand, Result<UserDto>>,
    IRequestHandler<UpdateUserCommand, Result<UserDto>>,
    IRequestHandler<SetUserStatusCommand, Result<bool>>,
    IRequestHandler<ResetUserPasswordCommand, Result<bool>>
{
    private readonly IAccountingDbContext _context;
    private readonly IPasswordHasher _passwordHasher;

    public UserManagementHandlers(IAccountingDbContext context, IPasswordHasher passwordHasher)
    {
        _context = context;
        _passwordHasher = passwordHasher;
    }

    public async Task<Result<List<UserDto>>> Handle(GetUsersQuery request, CancellationToken cancellationToken)
    {
        var users = await _context.Users
            .AsNoTracking()
            .Include(u => u.Role)
            .OrderBy(u => u.Username)
            .Select(u => new UserDto(
                u.Id,
                u.Username,
                u.FullName,
                u.Email,
                u.RoleId,
                u.Role != null ? u.Role.Name : string.Empty,
                u.IsActive,
                u.CreatedAtUtc))
            .ToListAsync(cancellationToken);

        return Result<List<UserDto>>.Success(users);
    }

    public async Task<Result<List<RoleDto>>> Handle(GetRolesQuery request, CancellationToken cancellationToken)
    {
        var roles = await _context.Roles
            .AsNoTracking()
            .OrderBy(r => r.Name)
            .Select(r => new RoleDto(r.Id, r.Name, r.Description, r.PermissionsCsv))
            .ToListAsync(cancellationToken);

        return Result<List<RoleDto>>.Success(roles);
    }

    public async Task<Result<UserDto>> Handle(CreateUserCommand request, CancellationToken cancellationToken)
    {
        var username = request.Username.Trim().ToLowerInvariant();
        if (string.IsNullOrWhiteSpace(username))
            return Result<UserDto>.Failure("Tên đăng nhập không được để trống.");

        if (string.IsNullOrWhiteSpace(request.Password) || request.Password.Length < 6)
            return Result<UserDto>.Failure("Mật khẩu phải có tối thiểu 6 ký tự.");

        var role = await _context.Roles.FirstOrDefaultAsync(r => r.Id == request.RoleId, cancellationToken);
        if (role == null)
            return Result<UserDto>.Failure("Vai trò (Role) không tồn tại.");

        var exists = await _context.Users.AnyAsync(u => u.Username == username, cancellationToken);
        if (exists)
            return Result<UserDto>.Failure($"Tên đăng nhập '{username}' đã tồn tại trong hệ thống.");

        var hashed = _passwordHasher.HashPassword(request.Password);
        var user = new AppUser(username, hashed, request.FullName, request.Email, request.RoleId);

        _context.AddEntity(user);
        await _context.SaveChangesAsync(cancellationToken);

        return Result<UserDto>.Success(new UserDto(
            user.Id,
            user.Username,
            user.FullName,
            user.Email,
            user.RoleId,
            role.Name,
            user.IsActive,
            user.CreatedAtUtc));
    }

    public async Task<Result<UserDto>> Handle(UpdateUserCommand request, CancellationToken cancellationToken)
    {
        var user = await _context.Users.Include(u => u.Role).FirstOrDefaultAsync(u => u.Id == request.UserId, cancellationToken);
        if (user == null)
            return Result<UserDto>.Failure("Người dùng không tồn tại.");

        var role = await _context.Roles.FirstOrDefaultAsync(r => r.Id == request.RoleId, cancellationToken);
        if (role == null)
            return Result<UserDto>.Failure("Vai trò (Role) không tồn tại.");

        user.UpdateProfile(request.FullName, request.Email, request.RoleId);
        await _context.SaveChangesAsync(cancellationToken);

        return Result<UserDto>.Success(new UserDto(
            user.Id,
            user.Username,
            user.FullName,
            user.Email,
            user.RoleId,
            role.Name,
            user.IsActive,
            user.CreatedAtUtc));
    }

    public async Task<Result<bool>> Handle(SetUserStatusCommand request, CancellationToken cancellationToken)
    {
        var user = await _context.Users.FirstOrDefaultAsync(u => u.Id == request.UserId, cancellationToken);
        if (user == null)
            return Result<bool>.Failure("Người dùng không tồn tại.");

        if (user.Username == "admin" && !request.IsActive)
            return Result<bool>.Failure("Không thể khóa tài khoản quản trị viên mặc định (admin).");

        user.SetStatus(request.IsActive);
        await _context.SaveChangesAsync(cancellationToken);

        return Result<bool>.Success(true);
    }

    public async Task<Result<bool>> Handle(ResetUserPasswordCommand request, CancellationToken cancellationToken)
    {
        if (string.IsNullOrWhiteSpace(request.NewPassword) || request.NewPassword.Length < 6)
            return Result<bool>.Failure("Mật khẩu mới phải có tối thiểu 6 ký tự.");

        var user = await _context.Users.FirstOrDefaultAsync(u => u.Id == request.UserId, cancellationToken);
        if (user == null)
            return Result<bool>.Failure("Người dùng không tồn tại.");

        var hashed = _passwordHasher.HashPassword(request.NewPassword);
        user.UpdatePassword(hashed);
        await _context.SaveChangesAsync(cancellationToken);

        return Result<bool>.Success(true);
    }
}

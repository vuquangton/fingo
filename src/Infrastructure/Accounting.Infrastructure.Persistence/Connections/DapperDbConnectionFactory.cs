using Microsoft.Extensions.Configuration;

namespace Accounting.Infrastructure.Persistence.Connections;

public class DapperDbConnectionFactory : DapperSqlConnectionFactory
{
    public DapperDbConnectionFactory(IConfiguration configuration) : base(configuration)
    {
    }
}

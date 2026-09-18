using System.Data;

namespace Accounting.Application.Common.Interfaces;

public interface ISqlConnectionFactory
{
    IDbConnection CreateConnection();
    string ProviderName { get; }
}

using System.Data;
using Accounting.Application.Common.Interfaces;
using Microsoft.Data.Sqlite;
using Microsoft.Extensions.Configuration;
using MySqlConnector;

namespace Accounting.Infrastructure.Persistence.Connections;

public class DapperSqlConnectionFactory : ISqlConnectionFactory, IDbConnectionFactory
{
    private readonly string _providerName;
    private readonly string _connectionString;

    public DapperSqlConnectionFactory(IConfiguration configuration)
    {
        var configuredProvider = configuration.GetValue<string>("Database:Provider")
            ?? configuration.GetValue<string>("DatabaseProvider")
            ?? "Sqlite";

        if (string.Equals(configuredProvider, "MariaDB", StringComparison.OrdinalIgnoreCase) ||
            string.Equals(configuredProvider, "MySql", StringComparison.OrdinalIgnoreCase))
        {
            _providerName = "MariaDB";
            _connectionString = configuration.GetValue<string>("Database:MariaDbConnection")
                ?? configuration.GetConnectionString("MariaDB")
                ?? configuration.GetConnectionString("MariaDbConnection")
                ?? configuration.GetConnectionString("DefaultConnection")
                ?? "Server=localhost;Port=3306;Database=accounting;User=root;Password=root;";
        }
        else
        {
            _providerName = "Sqlite";
            _connectionString = configuration.GetValue<string>("Database:SqliteConnection")
                ?? configuration.GetConnectionString("Sqlite")
                ?? configuration.GetConnectionString("SqliteConnection")
                ?? "Data Source=accounting.db";
        }
    }

    public string ProviderName => _providerName;

    public IDbConnection CreateConnection()
    {
        if (string.Equals(_providerName, "MariaDB", StringComparison.OrdinalIgnoreCase))
        {
            return new MySqlConnection(_connectionString);
        }

        return new SqliteConnection(_connectionString);
    }
}

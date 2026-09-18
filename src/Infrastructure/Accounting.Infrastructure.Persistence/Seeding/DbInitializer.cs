using Accounting.Domain.Entities.GeneralLedger;
using Accounting.Domain.Entities.Inventory;
using Accounting.Domain.Entities.Security;
using Accounting.Domain.Enums;
using Accounting.Infrastructure.Persistence.Context;
using Microsoft.EntityFrameworkCore;

namespace Accounting.Infrastructure.Persistence.Seeding;

public static class DbInitializer
{
    public static async Task SeedAsync(AccountingDbContext context, CancellationToken cancellationToken = default)
    {
        if (context.Database.ProviderName?.Contains("MySql", StringComparison.OrdinalIgnoreCase) == true)
        {
            try
            {
                var connString = context.Database.GetConnectionString();
                if (!string.IsNullOrWhiteSpace(connString))
                {
                    var builder = new MySqlConnector.MySqlConnectionStringBuilder(connString);
                    var targetDb = builder.Database;
                    if (!string.IsNullOrWhiteSpace(targetDb))
                    {
                        var serverBuilder = new MySqlConnector.MySqlConnectionStringBuilder(connString)
                        {
                            Database = string.Empty
                        };
                        using var serverConn = new MySqlConnector.MySqlConnection(serverBuilder.ConnectionString);
                        await serverConn.OpenAsync(cancellationToken);
                        using var cmd = serverConn.CreateCommand();
                        cmd.CommandText = $"CREATE DATABASE IF NOT EXISTS `{targetDb}` CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;";
                        await cmd.ExecuteNonQueryAsync(cancellationToken);
                    }
                }
            }
            catch
            {
                // Fall back to standard EnsureCreated
            }
        }

        await context.Database.EnsureCreatedAsync(cancellationToken);

        // 1. Seed Roles & Admin User
        if (!await context.Roles.AnyAsync(cancellationToken))
        {
            var adminRole = new AppRole("Administrator", "Full System Administrator", ["*"]);
            context.Roles.Add(adminRole);

            var accountantRole = new AppRole("Accountant", "General Accountant", ["Voucher.*", "Report.*", "Cash.*", "Bank.*"]);
            context.Roles.Add(accountantRole);

            if (!await context.Users.AnyAsync(u => u.Username == "admin", cancellationToken))
            {
                var adminUser = new AppUser("admin", "AQAAAAIAAYagAAAAEO9z...", "System Administrator", "admin@enterprise.vn", adminRole.Id);
                context.Users.Add(adminUser);
            }

            await context.SaveChangesAsync(cancellationToken);
        }

        // 2. Seed Vietnamese Chart of Accounts (COA) - Circular 200/2014 & Circular 133/2016
        if (!await context.Accounts.AnyAsync(cancellationToken))
        {
            var accounts = new List<Account>();

            // Class 1: Short-term Assets (Tài sản ngắn hạn)
            var tk111 = new Account("111", "Tiền mặt", AccountCategory.Asset, BalanceType.Debit, null, "Cash on hand", false);
            var tk1111 = new Account("1111", "Tiền Việt Nam", AccountCategory.Asset, BalanceType.Debit, tk111.Id, "VND cash", true);
            var tk1112 = new Account("1112", "Ngoại tệ", AccountCategory.Asset, BalanceType.Debit, tk111.Id, "Foreign currency cash", true, true);

            var tk112 = new Account("112", "Tiền gửi ngân hàng", AccountCategory.Asset, BalanceType.Debit, null, "Cash in bank", false);
            var tk1121 = new Account("1121", "Tiền Việt Nam gửi ngân hàng", AccountCategory.Asset, BalanceType.Debit, tk112.Id, "VND in bank", true);
            var tk1122 = new Account("1122", "Ngoại tệ gửi ngân hàng", AccountCategory.Asset, BalanceType.Debit, tk112.Id, "Foreign currency in bank", true, true);

            var tk131 = new Account("131", "Phải thu của khách hàng", AccountCategory.Asset, BalanceType.Bilateral, null, "Accounts receivable", true);

            var tk133 = new Account("133", "Thuế GTGT được khấu trừ", AccountCategory.Asset, BalanceType.Debit, null, "Deductible VAT", false);
            var tk1331 = new Account("1331", "Thuế GTGT được khấu trừ của HHDV", AccountCategory.Asset, BalanceType.Debit, tk133.Id, "Deductible VAT for goods and services", true);
            var tk1332 = new Account("1332", "Thuế GTGT được khấu trừ của TSCĐ", AccountCategory.Asset, BalanceType.Debit, tk133.Id, "Deductible VAT for fixed assets", true);

            var tk152 = new Account("152", "Nguyên liệu, vật liệu", AccountCategory.Asset, BalanceType.Debit, null, "Raw materials", true);
            var tk153 = new Account("153", "Công cụ, dụng cụ", AccountCategory.Asset, BalanceType.Debit, null, "Tools and supplies", true);
            var tk156 = new Account("156", "Hàng hóa", AccountCategory.Asset, BalanceType.Debit, null, "Merchandise inventory", false);
            var tk1561 = new Account("1561", "Giá mua hàng hóa", AccountCategory.Asset, BalanceType.Debit, tk156.Id, "Purchasing cost of merchandise", true);

            // Class 2: Long-term Assets (Tài sản dài hạn)
            var tk211 = new Account("211", "Tài sản cố định hữu hình", AccountCategory.Asset, BalanceType.Debit, null, "Tangible fixed assets", true);
            var tk214 = new Account("214", "Hao mòn tài sản cố định", AccountCategory.Asset, BalanceType.Credit, null, "Accumulated depreciation", true);
            var tk242 = new Account("242", "Chi phí trả trước", AccountCategory.Asset, BalanceType.Debit, null, "Prepaid expenses", true);

            // Class 3: Liabilities (Nợ phải trả)
            var tk331 = new Account("331", "Phải trả cho người bán", AccountCategory.Liability, BalanceType.Bilateral, null, "Accounts payable", true);
            var tk333 = new Account("333", "Thuế và các khoản phải nộp Nhà nước", AccountCategory.Liability, BalanceType.Credit, null, "Taxes and statutory payables", false);
            var tk3331 = new Account("3331", "Thuế GTGT phải nộp", AccountCategory.Liability, BalanceType.Credit, tk333.Id, "VAT payable", false);
            var tk33311 = new Account("33311", "Thuế GTGT đầu ra", AccountCategory.Liability, BalanceType.Credit, tk3331.Id, "Output VAT", true);
            var tk3334 = new Account("3334", "Thuế thu nhập doanh nghiệp", AccountCategory.Liability, BalanceType.Credit, tk333.Id, "Corporate income tax", true);
            var tk3335 = new Account("3335", "Thuế thu nhập cá nhân", AccountCategory.Liability, BalanceType.Credit, tk333.Id, "Personal income tax", true);

            var tk334 = new Account("334", "Phải trả người lao động", AccountCategory.Liability, BalanceType.Bilateral, null, "Salaries payable", true);

            var tk338 = new Account("338", "Phải trả, phải nộp khác", AccountCategory.Liability, BalanceType.Credit, null, "Other payables", false);
            var tk3382 = new Account("3382", "Kinh phí công đoàn (2%)", AccountCategory.Liability, BalanceType.Credit, tk338.Id, "Trade union fee", true);
            var tk3383 = new Account("3383", "Bảo hiểm xã hội", AccountCategory.Liability, BalanceType.Credit, tk338.Id, "Social insurance", true);
            var tk3384 = new Account("3384", "Bảo hiểm y tế", AccountCategory.Liability, BalanceType.Credit, tk338.Id, "Health insurance", true);
            var tk3386 = new Account("3386", "Bảo hiểm thất nghiệp", AccountCategory.Liability, BalanceType.Credit, tk338.Id, "Unemployment insurance", true);

            // Class 4: Equity (Vốn chủ sở hữu)
            var tk411 = new Account("411", "Vốn đầu tư của chủ sở hữu", AccountCategory.Equity, BalanceType.Credit, null, "Charter capital", true);
            var tk421 = new Account("421", "Lợi nhuận sau thuế chưa phân phối", AccountCategory.Equity, BalanceType.Bilateral, null, "Undistributed earnings", false);
            var tk4211 = new Account("4211", "Lợi nhuận sau thuế chưa phân phối năm trước", AccountCategory.Equity, BalanceType.Bilateral, tk421.Id, "Prior year earnings", true);
            var tk4212 = new Account("4212", "Lợi nhuận sau thuế chưa phân phối năm nay", AccountCategory.Equity, BalanceType.Bilateral, tk421.Id, "Current year earnings", true);

            // Class 5: Revenues (Doanh thu)
            var tk511 = new Account("511", "Doanh thu bán hàng và cung cấp dịch vụ", AccountCategory.Revenue, BalanceType.Credit, null, "Gross revenue", false);
            var tk5111 = new Account("5111", "Doanh thu bán hàng hóa", AccountCategory.Revenue, BalanceType.Credit, tk511.Id, "Merchandise sales revenue", true);
            var tk5112 = new Account("5112", "Doanh thu bán các thành phẩm", AccountCategory.Revenue, BalanceType.Credit, tk511.Id, "Finished goods sales revenue", true);
            var tk5113 = new Account("5113", "Doanh thu cung cấp dịch vụ", AccountCategory.Revenue, BalanceType.Credit, tk511.Id, "Service revenue", true);
            var tk515 = new Account("515", "Doanh thu hoạt động tài chính", AccountCategory.Revenue, BalanceType.Credit, null, "Financial income", true);

            // Class 6: Expenses (Chi phí sản xuất kinh doanh)
            var tk632 = new Account("632", "Giá vốn hàng bán", AccountCategory.Expense, BalanceType.Debit, null, "Cost of goods sold", true);
            var tk635 = new Account("635", "Chi phí tài chính", AccountCategory.Expense, BalanceType.Debit, null, "Financial expenses", true);
            var tk641 = new Account("641", "Chi phí bán hàng", AccountCategory.Expense, BalanceType.Debit, null, "Selling expenses", true);
            var tk642 = new Account("642", "Chi phí quản lý doanh nghiệp", AccountCategory.Expense, BalanceType.Debit, null, "General & administrative expenses", true);

            // Class 7: Other Income
            var tk711 = new Account("711", "Thu nhập khác", AccountCategory.OtherIncome, BalanceType.Credit, null, "Other income", true);

            // Class 8: Other Expenses
            var tk811 = new Account("811", "Chi phí khác", AccountCategory.OtherExpense, BalanceType.Debit, null, "Other expenses", true);

            // Class 9: Business Result
            var tk911 = new Account("911", "Xác định kết quả kinh doanh", AccountCategory.BusinessResult, BalanceType.Bilateral, null, "Business result determination", true);

            accounts.AddRange([
                tk111, tk1111, tk1112,
                tk112, tk1121, tk1122,
                tk131,
                tk133, tk1331, tk1332,
                tk152, tk153, tk156, tk1561,
                tk211, tk214, tk242,
                tk331, tk333, tk3331, tk33311, tk3334, tk3335,
                tk334,
                tk338, tk3382, tk3383, tk3384, tk3386,
                tk411, tk421, tk4211, tk4212,
                tk511, tk5111, tk5112, tk5113, tk515,
                tk632, tk635, tk641, tk642,
                tk711, tk811, tk911
            ]);

            context.Accounts.AddRange(accounts);
            await context.SaveChangesAsync(cancellationToken);
        }

        // 3. Seed Default Warehouse
        if (!await context.Warehouses.AnyAsync(cancellationToken))
        {
            var warehouse = new Warehouse("KHO-TONG", "Kho Tổng Công Ty", "Khu công nghiệp Tân Bình, TP. HCM");
            context.Warehouses.Add(warehouse);
            await context.SaveChangesAsync(cancellationToken);
        }

        // 4. Seed Fiscal Periods for current year
        if (!await context.FiscalPeriods.AnyAsync(cancellationToken))
        {
            var year = DateTime.UtcNow.Year;
            for (int m = 1; m <= 12; m++)
            {
                context.FiscalPeriods.Add(new FiscalPeriod(year, m));
            }
            await context.SaveChangesAsync(cancellationToken);
        }

        // 5. Seed Master Data Management (Thông tư 99/2025/TT-BTC Statutory Manifest, Currencies, UoMs, Warehouses)
        await StatutorySeeder.SeedAsync(context, cancellationToken);
    }
}

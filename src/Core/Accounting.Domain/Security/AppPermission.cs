namespace Accounting.Domain.Security;

public enum AppPermission
{
    ViewLedger = 1,
    CreateVoucher = 2,
    EditVoucher = 3,
    PostVoucher = 4,
    ApproveVoucher = 5,
    DeleteVoucher = 6,
    ExportReport = 7,
    ManageUsers = 8,
    DatabaseBackup = 9,
    DatabaseOptimize = 10,
    SystemAdmin = 99
}

namespace Accounting.Domain.Payroll;

public enum ContractType
{
    Indefinite = 1,  // Hợp đồng không xác định thời hạn
    FixedTerm = 2,   // Hợp đồng xác định thời hạn
    Probation = 3,   // Hợp đồng thử việc
    Freelance = 4    // Hợp đồng khoán / Cộng tác viên
}

public enum PayrollRunStatus
{
    Draft = 1,
    Calculated = 2,
    Approved = 3,
    PostedToGl = 4,
    Cancelled = 5
}

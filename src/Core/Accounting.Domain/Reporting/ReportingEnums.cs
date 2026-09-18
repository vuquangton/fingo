namespace Accounting.Domain.Reporting;

public enum StatementType
{
    BalanceSheet = 1,
    IncomeStatement = 2,
    CashFlowDirect = 3,
    CashFlowIndirect = 4
}

public enum PrintStyle
{
    Normal = 1,
    Bold = 2,
    Italic = 3
}

public enum LineNodeType
{
    Header = 1,
    LeafAccount = 2,
    Calculation = 3
}

public enum BalanceSide
{
    Debit = 1,
    Credit = 2,
    Net = 3
}

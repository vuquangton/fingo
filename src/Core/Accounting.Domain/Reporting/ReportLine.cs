using Accounting.Domain.Common;

namespace Accounting.Domain.Reporting;

public class ReportLine : Entity<Guid>
{
    public ReportTemplateId TemplateId { get; private set; }
    public string LineCode { get; private set; } = string.Empty;
    public int LineNumber { get; private set; }
    public string ItemName { get; private set; } = string.Empty;
    public PrintStyle PrintStyle { get; private set; }
    public LineNodeType NodeType { get; private set; }
    public string? AccountPattern { get; private set; }
    public string? CalculationLogic { get; private set; }
    public BalanceSide BalanceSide { get; private set; }
    public bool InvertSign { get; private set; }

    private ReportLine() { } // EF Core

    public ReportLine(
        Guid id,
        ReportTemplateId templateId,
        string lineCode,
        int lineNumber,
        string itemName,
        PrintStyle printStyle = PrintStyle.Normal,
        LineNodeType nodeType = LineNodeType.LeafAccount,
        string? accountPattern = null,
        string? calculationLogic = null,
        BalanceSide balanceSide = BalanceSide.Net,
        bool invertSign = false)
    {
        Id = id;
        TemplateId = templateId;
        LineCode = lineCode;
        LineNumber = lineNumber;
        ItemName = itemName;
        PrintStyle = printStyle;
        NodeType = nodeType;
        AccountPattern = accountPattern;
        CalculationLogic = calculationLogic;
        BalanceSide = balanceSide;
        InvertSign = invertSign;
    }
}

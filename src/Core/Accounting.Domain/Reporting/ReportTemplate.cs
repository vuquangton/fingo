using Accounting.Domain.Common;

namespace Accounting.Domain.Reporting;

public class ReportTemplate : AggregateRoot<ReportTemplateId>
{
    private readonly List<ReportLine> _lines = [];

    public StatementType StatementType { get; private set; }
    public string TemplateCode { get; private set; } = string.Empty;
    public string TemplateName { get; private set; } = string.Empty;
    public string Circular { get; private set; } = string.Empty;
    public IReadOnlyCollection<ReportLine> Lines => _lines.AsReadOnly();

    private ReportTemplate() { } // EF Core

    public ReportTemplate(
        ReportTemplateId id,
        StatementType statementType,
        string templateCode,
        string templateName,
        string circular)
    {
        Id = id;
        StatementType = statementType;
        TemplateCode = templateCode;
        TemplateName = templateName;
        Circular = circular;
    }

    public void AddLine(ReportLine line)
    {
        _lines.Add(line);
    }
}

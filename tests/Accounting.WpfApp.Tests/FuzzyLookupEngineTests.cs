using System.Diagnostics;
using Accounting.WpfApp.Services;

namespace Accounting.WpfApp.Tests;

public class FuzzyLookupEngineTests
{
    [Fact]
    public void Search_ShouldMatchExactCodeOrNameSubstring()
    {
        var engine = new FuzzyLookupEngine();
        engine.LoadData(new[]
        {
            new LookupItem("1111", "Tiền Việt Nam", "Tiền mặt VND", "Tài sản"),
            new LookupItem("1121", "Tiền gửi ngân hàng VND", "Vietcombank", "Tài sản"),
            new LookupItem("131", "Phải thu của khách hàng", "Công nợ phải thu", "Tài sản"),
            new LookupItem("VIC", "Tập đoàn Vingroup", "MST: 0101245486", "Đối tác"),
            new LookupItem("VNM", "Công ty CP Sữa Vinamilk", "MST: 0300588569", "Đối tác")
        });

        // Search by code
        var resCode = engine.Search("111");
        Assert.Single(resCode);
        Assert.Equal("1111", resCode[0].Code);

        // Search by name / abbreviation
        var resVic = engine.Search("vingroup");
        Assert.Single(resVic);
        Assert.Equal("VIC", resVic[0].Code);

        // Search fuzzy prefix
        var resVin = engine.Search("vin");
        Assert.Equal(2, resVin.Count);
    }

    [Fact]
    public void Search_With10000Items_ShouldRespondInUnder50Milliseconds()
    {
        var engine = new FuzzyLookupEngine();
        var items = new List<LookupItem>(10_000);
        for (int i = 0; i < 10_000; i++)
        {
            items.Add(new LookupItem($"KH-{i:D6}", $"Công ty TNHH Khách Hàng Doanh Nghiệp Số {i}", $"010{i:D7}", "Khách hàng"));
        }
        engine.LoadData(items);

        var sw = Stopwatch.StartNew();
        var results = engine.Search("9999");
        sw.Stop();

        Assert.NotEmpty(results);
        Assert.True(sw.ElapsedMilliseconds < 50, $"Lookup took {sw.ElapsedMilliseconds}ms, exceeding 50ms requirement.");
    }
}

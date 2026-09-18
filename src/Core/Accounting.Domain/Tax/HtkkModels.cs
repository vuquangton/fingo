using System.IO;
using System.Text;
using System.Xml;
using System.Xml.Serialization;

namespace Accounting.Domain.Tax;

[XmlRoot("HSoThueDTu")]
public class HtkkTaxDocument
{
    [XmlElement("HSoKhaiThue")]
    public HtkkTaxReturn TaxReturn { get; set; } = new();

    public static string SerializeToXml(HtkkTaxDocument doc)
    {
        var xmlSerializer = new XmlSerializer(typeof(HtkkTaxDocument));
        var settings = new XmlWriterSettings
        {
            Encoding = new UTF8Encoding(false),
            Indent = true,
            OmitXmlDeclaration = false
        };

        using var memoryStream = new MemoryStream();
        using (var writer = XmlWriter.Create(memoryStream, settings))
        {
            xmlSerializer.Serialize(writer, doc);
        }
        return Encoding.UTF8.GetString(memoryStream.ToArray());
    }

    public static HtkkTaxDocument DeserializeFromXml(string xml)
    {
        var xmlSerializer = new XmlSerializer(typeof(HtkkTaxDocument));
        using var reader = new StringReader(xml);
        return (HtkkTaxDocument)xmlSerializer.Deserialize(reader)!;
    }
}

public class HtkkTaxReturn
{
    [XmlElement("TTinChung")]
    public HtkkGeneralInfo GeneralInfo { get; set; } = new();

    [XmlElement("CTietTKhaiChinh")]
    public HtkkMainDetails MainDetails { get; set; } = new();
}

public class HtkkGeneralInfo
{
    [XmlElement("PBanTKhaiXML")]
    public string SchemaVersion { get; set; } = "2.1.0";

    [XmlElement("MaTKhai")]
    public string ReturnCode { get; set; } = "01/GTGT";

    [XmlElement("TenTKhai")]
    public string ReturnName { get; set; } = "T? KHAI THU? GI� TR? GIA TANG (M?u 01/GTGT)";

    [XmlElement("KyKhaiThue")]
    public string TaxPeriod { get; set; } = string.Empty;

    [XmlElement("MST")]
    public string TaxCode { get; set; } = string.Empty;

    [XmlElement("TenNNT")]
    public string TaxpayerName { get; set; } = string.Empty;

    [XmlElement("NguoiKy")]
    public string SignerName { get; set; } = string.Empty;

    [XmlElement("NgayKy")]
    public string SignDate { get; set; } = string.Empty;
}

public class HtkkMainDetails
{
    [XmlElement("ThueDauVao")]
    public decimal InputVatAmount { get; set; }

    [XmlElement("ThueDauRa")]
    public decimal OutputVatAmount { get; set; }

    [XmlElement("ThuePhaiNop")]
    public decimal VatPayable { get; set; }

    [XmlElement("ThueConDuocKhauTru")]
    public decimal VatCarriedForward { get; set; }
}

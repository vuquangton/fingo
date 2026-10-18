// Package nav is the application's navigation: what the rail shows, in what
// order, and which of it can actually be opened.
//
// It is data, not widgets. The rail's structure is a decision about the product
// and it is asserted in tests; rendering it is the shell's job and is not.
package nav

import (
	"fmt"
	"strings"
)

// Sections, in the order the rail shows them.
const (
	SectionWork       = "HÔM NAY"
	SectionBusiness   = "NGHIỆP VỤ"
	SectionBooks      = "SỔ & BÁO CÁO"
	SectionMasterData = "DANH MỤC"
	SectionSystem     = "HỆ THỐNG"
)

// Entry is one destination in the rail.
type Entry struct {
	ID      string
	Section string
	Label   string

	// Reason says what is behind this entry. For an entry whose engine exists it
	// describes the screen; for one whose engine does not, it says what is
	// missing.
	//
	// It is required for every entry, not only the unavailable ones. An entry
	// that cannot explain itself is an entry that cannot be honestly shown as
	// unavailable — it can only be a dead link or an empty table, and the plan
	// forbids both.
	Reason string
}

// Available is what the shell can actually open.
//
// Declared here, where it is used, so the rail's structure can be tested without
// a shell — and so that an entry's availability is a fact about the running
// application rather than a field somebody has to remember to flip.
type Available interface {
	CanOpen(id string) bool
}

// Open reports whether the entry can be opened, and why not when it cannot.
//
// The refusal leads with "chưa hỗ trợ" because that is the phrase the operator
// reads, and follows it with the entry's own reason. An empty screen teaches
// them the application is broken; this teaches them the feature is not built
// yet, which is a different and true thing.
func (e Entry) Open(a Available) error {
	if a != nil && a.CanOpen(e.ID) {
		return nil
	}

	return fmt.Errorf("%s chưa hỗ trợ — %s", e.Label, strings.TrimSpace(e.Reason))
}

// Rail is every destination, in the order the rail shows them.
//
// The thirteen modules with no engine behind them are present on purpose.
// Leaving them out would be tidier and would tell the operator that FinGo has no
// payroll; showing them with a reason tells them FinGo is an accounting
// application that has not got there yet. The difference decides whether they
// keep looking or stop.
func Rail() []Entry {
	return []Entry{
		{ID: "today", Section: SectionWork, Label: "Hôm nay",
			Reason: "vị thế sổ sách và việc còn lại của kỳ"},

		// NGHIỆP VỤ
		{ID: "sales", Section: SectionBusiness, Label: "Bán hàng",
			Reason: "chứng từ bán hàng, giá vốn và công nợ phải thu"},
		{ID: "purchase", Section: SectionBusiness, Label: "Mua hàng",
			Reason: "cần nối internal/trading: đơn mua, nhập kho và đối chiếu ba chiều"},
		{ID: "cash", Section: SectionBusiness, Label: "Quỹ",
			Reason: "cần dựng internal/domain/cash"},
		{ID: "bank", Section: SectionBusiness, Label: "Ngân hàng",
			Reason: "cần dựng internal/domain/cash và đối chiếu sổ phụ"},
		{ID: "inventory", Section: SectionBusiness, Label: "Kho",
			Reason: "cần nối internal/inventory: nhập, xuất và điều chuyển"},
		{ID: "tools", Section: SectionBusiness, Label: "Công cụ dụng cụ",
			Reason: "cần dựng internal/domain/asset và phân bổ TK 242"},
		{ID: "assets", Section: SectionBusiness, Label: "Tài sản cố định",
			Reason: "cần dựng internal/domain/asset và khấu hao hàng tháng"},
		{ID: "payroll", Section: SectionBusiness, Label: "Tiền lương",
			Reason: "cần dựng internal/domain/payroll: bảo hiểm, thuế TNCN và lương net"},
		{ID: "einvoice", Section: SectionBusiness, Label: "Hóa đơn điện tử",
			Reason: "cần dựng internal/domain/tax và kết nối nhà cung cấp hóa đơn"},
		{ID: "general", Section: SectionBusiness, Label: "Tổng hợp",
			Reason: "chứng từ nghiệp vụ khác — bút toán điều chỉnh không thuộc module nào"},

		// SỔ & BÁO CÁO
		{ID: "books", Section: SectionBooks, Label: "Sổ sách",
			Reason: "nhật ký chung, sổ cái và sổ chi tiết"},
		{ID: "reports", Section: SectionBooks, Label: "Báo cáo",
			Reason: "cân đối thử, kết quả kinh doanh và bảng cân đối"},
		{ID: "tax", Section: SectionBooks, Label: "Thuế",
			Reason: "01/GTGT và 03/TNDN — chờ chốt mã số chỉ tiêu"},

		// DANH MỤC
		{ID: "masterdata", Section: SectionMasterData, Label: "Danh mục",
			Reason: "hệ thống tài khoản, khách hàng, nhà cung cấp và vật tư"},

		// HỆ THỐNG
		{ID: "practice", Section: SectionSystem, Label: "Khách hàng",
			Reason: "cần nối internal/client: sổ khách hàng và chuyển đổi"},
		{ID: "system", Section: SectionSystem, Label: "Hệ thống",
			Reason: "kỳ kế toán, khóa sổ, sao lưu và người dùng"},
	}
}

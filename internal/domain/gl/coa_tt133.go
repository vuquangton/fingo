package gl

// The chart of accounts for the TT133 regime — the 49 accounts of Phụ lục 1 of
// Circular 133/2016/TT-BTC, plus the cấp-2 accounts that appendix prescribes.
//
// Transcribed from the official Công báo text-layer PDF (Công báo số 1103+1104
// ngày 12-10-2016, printed pages 77-82). Vendor and aggregator copies of this
// appendix contain cell-merge typos — "5111 5112 5111 5118" for instance — and
// must not be used as the source for account codes.
//
// Circular 46/2025/TT-BTC did not alter this appendix. It replaced Điều 10,
// moving chart modification from Ministry pre-approval to enterprise
// self-determination. No account was added, renamed or removed, so an
// enterprise that changes nothing uses exactly this list.
//
// Accounts belonging to other regimes are deliberately absent: 521 (no revenue
// deduction account exists under 133 — deductions debit 511 directly), 621 /
// 622 / 627 and 641 (expenses are 6421 and 6422), and 1562 (TK 156 has no
// prescribed cấp-2).

// natureOverrides lists the accounts whose normal balance is not the default
// for their class: contra accounts, and accounts that routinely carry either
// side. Everything else follows its class.
//
// This is a documented project rule, not a citation. The circular groups
// accounts by class; it does not state a normal balance per account.
var natureOverrides = map[string]AccountNature{
	// Contra-asset: accumulated depreciation and provisions sit against assets.
	"214": NatureCredit,
	"229": NatureCredit,

	// Contra-equity: treasury shares reduce equity.
	"419": NatureDebit,

	// Flips with the balance: receivable, payable, FX difference, retained
	// earnings (which carries accumulated losses as a debit), and the profit
	// determination account.
	"131": NatureDual,
	"331": NatureDual,
	"413": NatureDual,
	"421": NatureDual,
	"911": NatureDual,
}

func natureFor(code string, t AccountType) AccountNature {
	if n, ok := natureOverrides[code]; ok {
		return n
	}
	switch t {
	case AccountTypeAsset, AccountTypeExpense:
		return NatureDebit
	default:
		return NatureCredit
	}
}

// tt133Account is one row of Phụ lục 1.
type tt133Account struct {
	code   string
	name   string
	parent string
	typ    AccountType
}

// tt133Accounts is Phụ lục 1 in order: the 49 cấp-1 accounts, each followed by
// the cấp-2 accounts the appendix prescribes beneath it.
var tt133Accounts = []tt133Account{
	// LOẠI TÀI KHOẢN TÀI SẢN — assets (23 accounts)
	{"111", "Tiền mặt", "", AccountTypeAsset},
	{"1111", "Tiền Việt Nam", "111", AccountTypeAsset},
	{"1112", "Ngoại tệ", "111", AccountTypeAsset},
	{"112", "Tiền gửi Ngân hàng", "", AccountTypeAsset},
	{"1121", "Tiền Việt Nam", "112", AccountTypeAsset},
	{"1122", "Ngoại tệ", "112", AccountTypeAsset},
	{"121", "Chứng khoán kinh doanh", "", AccountTypeAsset},
	{"128", "Đầu tư nắm giữ đến ngày đáo hạn", "", AccountTypeAsset},
	{"1281", "Tiền gửi có kỳ hạn", "128", AccountTypeAsset},
	{"1288", "Các khoản đầu tư khác nắm giữ đến ngày đáo hạn", "128", AccountTypeAsset},
	{"131", "Phải thu của khách hàng", "", AccountTypeAsset},
	{"133", "Thuế GTGT được khấu trừ", "", AccountTypeAsset},
	{"1331", "Thuế GTGT được khấu trừ của hàng hóa, dịch vụ", "133", AccountTypeAsset},
	{"1332", "Thuế GTGT được khấu trừ của TSCĐ", "133", AccountTypeAsset},
	{"136", "Phải thu nội bộ", "", AccountTypeAsset},
	{"1361", "Vốn kinh doanh ở đơn vị trực thuộc", "136", AccountTypeAsset},
	{"1368", "Phải thu nội bộ khác", "136", AccountTypeAsset},
	{"138", "Phải thu khác", "", AccountTypeAsset},
	{"1381", "Tài sản thiếu chờ xử lý", "138", AccountTypeAsset},
	{"1386", "Cầm cố, thế chấp, ký quỹ, ký cược", "138", AccountTypeAsset},
	{"1388", "Phải thu khác", "138", AccountTypeAsset},
	{"141", "Tạm ứng", "", AccountTypeAsset},
	{"151", "Hàng mua đang đi đường", "", AccountTypeAsset},
	{"152", "Nguyên liệu, vật liệu", "", AccountTypeAsset},
	{"153", "Công cụ, dụng cụ", "", AccountTypeAsset},
	{"154", "Chi phí sản xuất, kinh doanh dở dang", "", AccountTypeAsset},
	{"155", "Thành phẩm", "", AccountTypeAsset},
	{"156", "Hàng hóa", "", AccountTypeAsset},
	{"157", "Hàng gửi đi bán", "", AccountTypeAsset},
	{"211", "Tài sản cố định", "", AccountTypeAsset},
	{"2111", "TSCĐ hữu hình", "211", AccountTypeAsset},
	{"2112", "TSCĐ thuê tài chính", "211", AccountTypeAsset},
	{"2113", "TSCĐ vô hình", "211", AccountTypeAsset},
	{"214", "Hao mòn tài sản cố định", "", AccountTypeAsset},
	{"2141", "Hao mòn TSCĐ hữu hình", "214", AccountTypeAsset},
	{"2142", "Hao mòn TSCĐ thuê tài chính", "214", AccountTypeAsset},
	{"2143", "Hao mòn TSCĐ vô hình", "214", AccountTypeAsset},
	{"2147", "Hao mòn bất động sản đầu tư", "214", AccountTypeAsset},
	{"217", "Bất động sản đầu tư", "", AccountTypeAsset},
	{"228", "Đầu tư góp vốn vào đơn vị khác", "", AccountTypeAsset},
	{"2281", "Đầu tư vào công ty liên doanh, liên kết", "228", AccountTypeAsset},
	{"2288", "Đầu tư khác", "228", AccountTypeAsset},
	{"229", "Dự phòng tổn thất tài sản", "", AccountTypeAsset},
	{"2291", "Dự phòng giảm giá chứng khoán kinh doanh", "229", AccountTypeAsset},
	{"2292", "Dự phòng tổn thất đầu tư vào đơn vị khác", "229", AccountTypeAsset},
	{"2293", "Dự phòng phải thu khó đòi", "229", AccountTypeAsset},
	{"2294", "Dự phòng giảm giá hàng tồn kho", "229", AccountTypeAsset},
	{"241", "Xây dựng cơ bản dở dang", "", AccountTypeAsset},
	{"2411", "Mua sắm TSCĐ", "241", AccountTypeAsset},
	{"2412", "Xây dựng cơ bản", "241", AccountTypeAsset},
	{"2413", "Sửa chữa lớn TSCĐ", "241", AccountTypeAsset},
	{"242", "Chi phí trả trước", "", AccountTypeAsset},

	// LOẠI TÀI KHOẢN NỢ PHẢI TRẢ — liabilities (10 accounts)
	{"331", "Phải trả cho người bán", "", AccountTypeLiability},
	{"333", "Thuế và các khoản phải nộp Nhà nước", "", AccountTypeLiability},
	{"3331", "Thuế giá trị gia tăng phải nộp", "333", AccountTypeLiability},
	{"33311", "Thuế GTGT đầu ra", "3331", AccountTypeLiability},
	{"33312", "Thuế GTGT hàng nhập khẩu", "3331", AccountTypeLiability},
	{"3332", "Thuế tiêu thụ đặc biệt", "333", AccountTypeLiability},
	{"3333", "Thuế xuất, nhập khẩu", "333", AccountTypeLiability},
	{"3334", "Thuế thu nhập doanh nghiệp", "333", AccountTypeLiability},
	{"3335", "Thuế thu nhập cá nhân", "333", AccountTypeLiability},
	{"3336", "Thuế tài nguyên", "333", AccountTypeLiability},
	{"3337", "Thuế nhà đất, tiền thuê đất", "333", AccountTypeLiability},
	{"3338", "Thuế bảo vệ môi trường và các loại thuế khác", "333", AccountTypeLiability},
	{"33381", "Thuế bảo vệ môi trường", "3338", AccountTypeLiability},
	{"33382", "Các loại thuế khác", "3338", AccountTypeLiability},
	{"3339", "Phí, lệ phí và các khoản phải nộp khác", "333", AccountTypeLiability},
	{"334", "Phải trả người lao động", "", AccountTypeLiability},
	{"335", "Chi phí phải trả", "", AccountTypeLiability},
	{"336", "Phải trả nội bộ", "", AccountTypeLiability},
	{"3361", "Phải trả nội bộ về vốn kinh doanh", "336", AccountTypeLiability},
	{"3368", "Phải trả nội bộ khác", "336", AccountTypeLiability},
	{"338", "Phải trả, phải nộp khác", "", AccountTypeLiability},
	{"3381", "Tài sản thừa chờ giải quyết", "338", AccountTypeLiability},
	{"3382", "Kinh phí công đoàn", "338", AccountTypeLiability},
	{"3383", "Bảo hiểm xã hội", "338", AccountTypeLiability},
	{"3384", "Bảo hiểm y tế", "338", AccountTypeLiability},
	{"3385", "Bảo hiểm thất nghiệp", "338", AccountTypeLiability},
	{"3386", "Nhận ký quỹ, ký cược", "338", AccountTypeLiability},
	{"3387", "Doanh thu chưa thực hiện", "338", AccountTypeLiability},
	{"3388", "Phải trả, phải nộp khác", "338", AccountTypeLiability},
	{"341", "Vay và nợ thuê tài chính", "", AccountTypeLiability},
	{"3411", "Các khoản đi vay", "341", AccountTypeLiability},
	{"3412", "Nợ thuê tài chính", "341", AccountTypeLiability},
	{"352", "Dự phòng phải trả", "", AccountTypeLiability},
	{"3521", "Dự phòng bảo hành sản phẩm hàng hóa", "352", AccountTypeLiability},
	{"3522", "Dự phòng bảo hành công trình xây dựng", "352", AccountTypeLiability},
	{"3524", "Dự phòng phải trả khác", "352", AccountTypeLiability},
	{"353", "Quỹ khen thưởng phúc lợi", "", AccountTypeLiability},
	{"3531", "Quỹ khen thưởng", "353", AccountTypeLiability},
	{"3532", "Quỹ phúc lợi", "353", AccountTypeLiability},
	{"3533", "Quỹ phúc lợi đã hình thành TSCĐ", "353", AccountTypeLiability},
	{"3534", "Quỹ thưởng ban quản lý điều hành công ty", "353", AccountTypeLiability},
	{"356", "Quỹ phát triển khoa học và công nghệ", "", AccountTypeLiability},
	{"3561", "Quỹ phát triển khoa học và công nghệ", "356", AccountTypeLiability},
	{"3562", "Quỹ phát triển khoa học và công nghệ đã hình thành TSCĐ", "356", AccountTypeLiability},

	// LOẠI TÀI KHOẢN VỐN CHỦ SỞ HỮU — equity (5 accounts)
	{"411", "Vốn đầu tư của chủ sở hữu", "", AccountTypeEquity},
	{"4111", "Vốn góp của chủ sở hữu", "411", AccountTypeEquity},
	{"4112", "Thặng dư vốn cổ phần", "411", AccountTypeEquity},
	{"4118", "Vốn khác", "411", AccountTypeEquity},
	{"413", "Chênh lệch tỷ giá hối đoái", "", AccountTypeEquity},
	{"418", "Các quỹ thuộc vốn chủ sở hữu", "", AccountTypeEquity},
	{"419", "Cổ phiếu quỹ", "", AccountTypeEquity},
	{"421", "Lợi nhuận sau thuế chưa phân phối", "", AccountTypeEquity},
	{"4211", "Lợi nhuận sau thuế chưa phân phối năm trước", "421", AccountTypeEquity},
	{"4212", "Lợi nhuận sau thuế chưa phân phối năm nay", "421", AccountTypeEquity},

	// LOẠI TÀI KHOẢN DOANH THU — revenue (2 accounts)
	{"511", "Doanh thu bán hàng và cung cấp dịch vụ", "", AccountTypeRevenue},
	{"5111", "Doanh thu bán hàng hóa", "511", AccountTypeRevenue},
	{"5112", "Doanh thu bán thành phẩm", "511", AccountTypeRevenue},
	{"5113", "Doanh thu cung cấp dịch vụ", "511", AccountTypeRevenue},
	{"5118", "Doanh thu khác", "511", AccountTypeRevenue},
	{"515", "Doanh thu hoạt động tài chính", "", AccountTypeRevenue},

	// LOẠI TÀI KHOẢN CHI PHÍ SẢN XUẤT, KINH DOANH — operating expenses
	// (5 accounts). Note there is no 641: selling and administration costs are
	// the cấp-2 of 642.
	{"611", "Mua hàng", "", AccountTypeExpense},
	{"631", "Giá thành sản xuất", "", AccountTypeExpense},
	{"632", "Giá vốn hàng bán", "", AccountTypeExpense},
	{"635", "Chi phí tài chính", "", AccountTypeExpense},
	{"642", "Chi phí quản lý kinh doanh", "", AccountTypeExpense},
	{"6421", "Chi phí bán hàng", "642", AccountTypeExpense},
	{"6422", "Chi phí quản lý doanh nghiệp", "642", AccountTypeExpense},

	// LOẠI TÀI KHOẢN THU NHẬP KHÁC — other income (1 account)
	{"711", "Thu nhập khác", "", AccountTypeRevenue},

	// LOẠI TÀI KHOẢN CHI PHÍ KHÁC — other expenses (2 accounts)
	{"811", "Chi phí khác", "", AccountTypeExpense},
	{"821", "Chi phí thuế thu nhập doanh nghiệp", "", AccountTypeExpense},

	// TÀI KHOẢN XÁC ĐỊNH KẾT QUẢ KINH DOANH — profit determination
	// (1 account). Phụ lục 1 gives it its own class; the schema's account_type
	// has no such value, so it is carried as EXPENSE with a DUAL nature.
	{"911", "Xác định kết quả kinh doanh", "", AccountTypeExpense},
}

// TT133Chart returns the TT133 chart of accounts as domain accounts.
//
// The result is a fresh slice on every call, so a caller cannot mutate the
// package's data by sorting or appending to what it receives.
func TT133Chart() []Account {
	out := make([]Account, 0, len(tt133Accounts))

	for _, a := range tt133Accounts {
		out = append(out, Account{
			Code:       a.code,
			Name:       a.name,
			ParentCode: a.parent,
			Type:       a.typ,
			Nature:     natureFor(a.code, a.typ),
			IsActive:   true,
		})
	}

	return out
}

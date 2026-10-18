package statutory

import (
	"github.com/shopspring/decimal"

	"fingo/internal/vat"
)

// The instruments this package cites, spelled the way docs/legal-basis.md spells
// them, because the test that guards the catalogue matches on these strings.
const (
	// The VAT law, the return form, and the reduced-VAT appendix.
	authorityVATLaw     = "Luật Thuế GTGT 48/2024/QH15"
	authorityVATReturn  = "TT 89/2026/TT-BTC, Phụ lục I — mẫu 01/GTGT"
	authorityVATCut     = "Nghị quyết 204/2025/QH15 + NĐ 174/2025/NĐ-CP"
	authorityVATCutForm = "NĐ 174/2025/NĐ-CP, Phụ lục III"

	// Articles of the VAT law that the file does place: Điều 5 is the
	// non-taxable list, and Điều 9 khoản 1 điểm d is the 0% exclusions. The
	// file does not say which article carries the 5% list or the standard rate,
	// so those lines cite the law and no article — a pinpoint the file does not
	// support is a pinpoint nobody can check.
	authorityVATNonTaxable = authorityVATLaw + " — Điều 5"
	authorityVATZeroRated  = authorityVATLaw + " — Điều 9 khoản 1 điểm d"

	// Corporate income tax.
	authorityCITLaw  = "Luật Thuế TNDN 67/2025/QH15 — Điều 10 rates: 20% standard; 15% if turnover ≤ 3bn; 17% if >3bn–≤50bn"
	authorityCITForm = "TT 89/2026/TT-BTC, Phụ lục I"
	authorityCITPay  = "NĐ 252/2026/NĐ-CP, Điều 24 — provisional quarterly payments ≥80% of the annual liability"

	// The accounting regimes. TT133 is the regime this repository declares, and
	// the file is explicit that a change to one regime is not a change to
	// another.
	authorityTT133 = "133/2016/TT-BTC as amended by 46/2025/TT-BTC"
	authorityTT99  = "99/2025/TT-BTC — Điều 17, Phụ lục"
)

// The appendix's form code. The file gives this appendix no TT 89 code: it names
// it "GIẢM THUẾ GTGT THEO NGHỊ QUYẾT 204/2025/QH15", mẫu 01. Rather than invent a
// code, the code is the name the file gives it.
const codeVATReduction = "GIẢM THUẾ GTGT THEO NGHỊ QUYẾT 204/2025/QH15"

// The reasons a line is left out, in docs/legal-basis.md's own terms.
const (
	reasonExemption = "No provisional-payment exemption threshold found — do not assert one. The widely-repeated \"≤1 billion\" carve-out was a proposal, not enacted text."
	reasonVATCutEnd = "No 2026 instrument extending it was found — absence of evidence."
	reasonSubItem   = "The file names [23]–[25] as the input-VAT block and does not say what these two indicators carry, so there is nothing to file under them."
	reasonReturnNum = "Phụ lục I of TT 89/2026 was not readable. The file records the mapping against a named secondary source and, in Q5, still counts the 01/GTGT indicator numbers among what remains ?; its rule is not to build on a ? row, so the lines are filed without mã số."
	reasonAppNum    = "The appendix's indicators [01]–[09] come from NĐ 174/2025, Phụ lục III, which was not readable: the numbers are recorded in this filing's own notes, not written into its lines."
	reasonCITNum    = "The file does not record the 03/TNDN indicator numbers at all, so its lines are filed by label."
	reasonNumbering = "Whether FS indicator numbers changed requires Phụ lục IV. The gazette PDF was not machine-readable, so every indicator number is provisional."
	reasonTT133Form = "Statutory book form codes under TT133 as amended: unconfirmed. The statement set here is the regime's, and its numbering is not."
	reasonFormSet   = "The statutory set is B01-DN, B02-DN, B03-DN and B09-DN. This filing carries the first two: the cash-flow statement and the notes are not modelled, and are not filed rather than filed empty."
	reasonPeriod    = "The turnover and the rate it selects are the year's; the profit and the 80% test are the period's. 03/TNDN is a finalisation return and should be filed over a full year."
	reasonWindow    = "The 8% window runs to 31/12/2026 (Nghị quyết 204/2025/QH15), and no 2026 instrument extending it was found, so no reduction is stated for a period that ends after the window closes."
	reasonOutputVAT = "The output VAT on TK 33311 and the tax the rate bands carry differ. The payable follows the ledger; the difference is a reconciliation to clear before filing."
)

// The account codes the statements read.
//
// They are the TT133 chart's, whose authority is the file's own row for Phụ lục
// 1 — the 49 cấp-1 accounts of Circular 133/2016, resolved in Q3 and seeded as
// gl.TT133Chart(). The file does not name these three accounts individually, so
// this package does not claim it does: TestCatalogue_AccountsExistInTheChart
// checks them against the chart, and TestCatalogue_CitesOnlyWhatTheLegalBasisHolds
// checks that the chart's own authority is a row in the file.
const (
	accountInputVAT   = "1331"  // Thuế GTGT được khấu trừ
	accountOutputVAT  = "33311" // Thuế GTGT đầu ra
	accountCITPayable = "3334"  // Thuế thu nhập doanh nghiệp
)

// statementSpec is a statement before its figures are known.
type statementSpec struct {
	code      string
	name      string
	authority string

	// confidence is the file's mark for the authority.
	confidence Confidence

	// withheld is the mã số mapping the file records for this statement, if it
	// records one. It is named in the statement's warning and never written on a
	// line: see the package doc for why.
	withheld string

	// indicatorQuestion and indicatorReason name the open question that leaves
	// the numbering unresolved, and what is missing. Where they are set, no line
	// of the statement is filed with a mã số.
	indicatorQuestion string
	indicatorReason   string

	// warnings are the things the statement itself has to say, before any line
	// is left out.
	warnings []Warning

	lines []lineSpec
}

// numbered reports whether the statement's lines carry mã số. None do today; the
// field exists so that resolving a question is a data change and not a rewrite.
func (s statementSpec) numbered() bool { return s.indicatorReason == "" }

// lineSpec is one line before its figure is known.
type lineSpec struct {
	// indicator is the mã số the file records for this line. It is filed only
	// where the statement's numbering is resolved.
	indicator string

	label      string
	reference  string
	confidence Confidence
	note       string
	amount     func(figures) decimal.Decimal

	// unresolved marks a line the legal basis has not resolved. It is never
	// emitted: it becomes the statement's warning instead.
	unresolved bool

	// question is the open question in docs/legal-basis.md that blocks an
	// unresolved line.
	question string

	// reason is what is missing, for the warning the line leaves behind.
	reason string
}

// vatReturnCatalogue is the 01/GTGT return: what was carried in, what was
// credited, what was paid and what is carried out.
//
// The rates are the law's, and each line cites the article the file places. The
// mã số are the file's secondary mapping, and they are withheld: see the package
// doc.
func vatReturnCatalogue() statementSpec {
	return statementSpec{
		code:              "01/GTGT",
		name:              "Tờ khai thuế GTGT (VAT return)",
		authority:         authorityVATReturn,
		confidence:        Secondary,
		withheld:          "[22], [23], [26], [29], [30]–[33], [40], [43]",
		indicatorQuestion: "Q5",
		indicatorReason:   reasonReturnNum,
		lines: []lineSpec{
			{
				indicator:  "[22]",
				label:      "Thuế GTGT còn được khấu trừ kỳ trước chuyển sang (credit carried in)",
				reference:  authorityVATReturn,
				confidence: Secondary,
				amount:     func(f figures) decimal.Decimal { return f.vatReturn.CarriedIn },
			},
			{
				indicator:  "[23]",
				label:      "Thuế GTGT đầu vào được khấu trừ (deductible input VAT)",
				reference:  authorityVATReturn,
				confidence: Secondary,
				note:       "Taken from the movement on TK " + accountInputVAT + ", which is where deductible input VAT is recorded.",
				amount:     func(f figures) decimal.Decimal { return f.vatReturn.InputCredit },
			},
			{
				// The file names [23]–[25] as the block and not what each one
				// carries. An indicator nobody can describe is not filed with a
				// guessed figure in it, and it is not called "?" either: the
				// block is `S`, and the missing thing is the description.
				indicator:  "[24]",
				label:      "Thuế GTGT đầu vào — chỉ tiêu không được mô tả trong hồ sơ pháp lý (input VAT, indicator not described)",
				reference:  authorityVATReturn,
				confidence: Unresolved,
				question:   "Q5",
				reason:     reasonSubItem,
				unresolved: true,
			},
			{
				indicator:  "[25]",
				label:      "Thuế GTGT đầu vào — chỉ tiêu không được mô tả trong hồ sơ pháp lý (input VAT, indicator not described)",
				reference:  authorityVATReturn,
				confidence: Unresolved,
				question:   "Q5",
				reason:     reasonSubItem,
				unresolved: true,
			},
			{
				indicator:  "[26]",
				label:      "Doanh thu hàng hóa, dịch vụ không chịu thuế GTGT (turnover not subject to VAT)",
				reference:  authorityVATNonTaxable,
				confidence: Primary,
				amount:     func(f figures) decimal.Decimal { return f.vat.NonTaxable },
			},
			{
				indicator:  "[29]",
				label:      "Doanh thu hàng hóa, dịch vụ thuộc mức thuế suất 0% (turnover at 0%)",
				reference:  authorityVATZeroRated,
				confidence: Primary,
				amount:     func(f figures) decimal.Decimal { return f.vat.ZeroRated },
			},
			{
				indicator:  "[30]",
				label:      "Doanh thu hàng hóa, dịch vụ chịu thuế suất 5% (turnover at 5%)",
				reference:  authorityVATLaw,
				confidence: Primary,
				note:       "The 5% list is the law's reduced list; the file does not place the article it sits in.",
				amount:     func(f figures) decimal.Decimal { return f.vatBand(rateFivePercent).Net },
			},
			{
				indicator:  "[31]",
				label:      "Thuế GTGT đầu ra ở mức 5% (output VAT at 5%)",
				reference:  authorityVATLaw,
				confidence: Primary,
				amount:     func(f figures) decimal.Decimal { return f.vatBand(rateFivePercent).Tax },
			},
			{
				indicator:  "[32]",
				label:      "Doanh thu hàng hóa, dịch vụ chịu thuế suất 10% (turnover at 10%)",
				reference:  authorityVATLaw,
				confidence: Primary,
				note:       "10% is the standard rate the file states.",
				amount:     func(f figures) decimal.Decimal { return f.vatBand(rateTenPercent).Net },
			},
			{
				indicator:  "[33]",
				label:      "Thuế GTGT đầu ra ở mức 10% (output VAT at 10%)",
				reference:  authorityVATLaw,
				confidence: Primary,
				amount:     func(f figures) decimal.Decimal { return f.vatBand(rateTenPercent).Tax },
			},
			{
				indicator:  "[40]",
				label:      "Thuế GTGT phải nộp trong kỳ (VAT payable for the period)",
				reference:  authorityVATReturn,
				confidence: Secondary,
				note:       "Output VAT less the credit carried in and the input VAT of the period.",
				amount:     func(f figures) decimal.Decimal { return f.vatReturn.Payable },
			},
			{
				indicator:  "[43]",
				label:      "Thuế GTGT còn được khấu trừ chuyển kỳ sau (credit carried out)",
				reference:  authorityVATReturn,
				confidence: Secondary,
				note:       "What the period's output did not absorb, left for the next one.",
				amount:     func(f figures) decimal.Decimal { return f.vatReturn.CarriedOut },
			},
		},
	}
}

// vatReductionCatalogue is the appendix the 8% window is reported in.
//
// It is a separate appendix and not a rate line on 01/GTGT: docs/legal-basis.md
// is explicit that "the 8% break-out is NOT a rate line on 01/GTGT".
//
// withinWindow says whether the period ends inside the window. The turnover line
// is stated either way — it is what the invoices say — and the reduction is
// stated only where an instrument authorises it.
func vatReductionCatalogue(withinWindow bool) statementSpec {
	spec := statementSpec{
		code:              codeVATReduction,
		name:              "GIẢM THUẾ GTGT THEO NGHỊ QUYẾT 204/2025/QH15 — mẫu 01 (reduced-VAT appendix)",
		authority:         authorityVATCutForm,
		confidence:        Secondary,
		withheld:          "[01]–[09]",
		indicatorQuestion: "Q8",
		indicatorReason:   reasonAppNum,
		lines: []lineSpec{
			{
				label:      "Doanh thu hàng hóa, dịch vụ thuộc diện giảm thuế (turnover in the reduced band)",
				reference:  authorityVATCut,
				confidence: Secondary,
				note:       "Sales the 8% window applies to, at the net value invoiced.",
				amount:     func(f figures) decimal.Decimal { return f.vatBand(rateEightPercent).Net },
			},
			{
				indicator:  "01",
				label:      "Giảm thuế sau ngày 31/12/2026 (reduction after the window closes)",
				reference:  authorityVATCut,
				confidence: Unresolved,
				question:   "Q5",
				reason:     reasonVATCutEnd,
				unresolved: true,
			},
		},
	}

	reduction := lineSpec{
		label:      "Thuế GTGT được giảm (VAT reduced)",
		reference:  authorityVATCut,
		confidence: Secondary,
		note:       "The difference between the standard 10% and the 8% applied, on the same turnover.",
		amount:     func(f figures) decimal.Decimal { return vatReduction(f.vatBand(rateEightPercent).Net) },
	}

	if withinWindow {
		spec.lines = append(spec.lines[:1], append([]lineSpec{reduction}, spec.lines[1:]...)...)
	} else {
		spec.warnings = append(spec.warnings, Warning{
			Item:     "Thuế GTGT được giảm",
			Question: "Q5",
			Reason:   reasonWindow,
		})
	}

	return spec
}

// citCatalogue is the corporate income tax finalisation.
func citCatalogue() statementSpec {
	return statementSpec{
		code:              "03/TNDN",
		name:              "Quyết toán thuế thu nhập doanh nghiệp (corporate income tax finalisation)",
		authority:         authorityCITForm,
		confidence:        Secondary,
		indicatorQuestion: "Q8",
		indicatorReason:   reasonCITNum,
		warnings: []Warning{{
			Item:     "kỳ tính thuế (the period the figures cover)",
			Question: "Q8",
			Reason:   reasonPeriod,
		}},
		lines: []lineSpec{
			{
				label:      "Doanh thu cả năm (turnover for the year)",
				reference:  authorityCITLaw,
				confidence: Secondary,
				note:       "Turnover is what selects the rate, so it is stated before the tax it selects.",
				amount:     func(f figures) decimal.Decimal { return f.turnover },
			},
			{
				label:      "Lợi nhuận kế toán trước thuế (accounting profit before tax)",
				reference:  authorityCITForm,
				confidence: Secondary,
				note:       "The period's result as the books state it, before any tax adjustment: a loss is stated as a negative figure.",
				amount:     func(f figures) decimal.Decimal { return f.profit.NetProfit },
			},
			{
				label:      "Thuế TNDN theo thuế suất áp dụng (corporate income tax at the applicable rate)",
				reference:  authorityCITLaw,
				confidence: Secondary,
				note:       "Rate selected by turnover: 15% at or below 3bn, 17% above 3bn up to 50bn, 20% above that.",
				amount:     func(f figures) decimal.Decimal { return f.citLiability() },
			},
			{
				label:      "Thuế TNDN đã tạm nộp trong kỳ (provisional corporate income tax paid)",
				reference:  authorityCITForm,
				confidence: Secondary,
				note:       "Taken from what was debited to TK " + accountCITPayable + ", which is what was paid: an accrual credited to the same account is a liability, not a payment.",
				amount:     func(f figures) decimal.Decimal { return f.citPaid },
			},
			{
				label:      "Chênh lệch so với mức tạm nộp tối thiểu 80% (shortfall against the 80% test)",
				reference:  authorityCITPay,
				confidence: Secondary,
				note:       "80% of the annual liability less what was paid. Nil means the test is met.",
				amount:     func(f figures) decimal.Decimal { return f.citShortfall() },
			},
			{
				// The file says so in as many words: do not assert one.
				label:      "Ngưỡng miễn tạm nộp thuế TNDN (provisional-payment exemption threshold)",
				reference:  authorityCITPay,
				confidence: Unresolved,
				reason:     reasonExemption,
				unresolved: true,
			},
		},
	}
}

// financialStatementsCatalogue is the annual financial statements, named and
// cited for the regime the books are kept under.
//
// The figures are the books' own accounts. Their presentation as prescribed
// indicator numbers is not claimed, because docs/legal-basis.md does not resolve
// that numbering for either regime.
func financialStatementsCatalogue(regime string) ([]statementSpec, error) {
	balanceSheet, incomeStatement := "Bảng cân đối kế toán", "Báo cáo kết quả hoạt động kinh doanh"

	switch regime {
	case RegimeTT133:
		// TT133 as amended. The form family is the same; its indicator numbers
		// are not resolved in the file.
	case RegimeTT99:
		// TT99 renames the first statement. Reading a TT99 name onto a TT133 set
		// of books would be porting a change between regimes.
		balanceSheet = "Báo cáo tình hình tài chính"
	default:
		return nil, ErrUnknownRegime
	}

	authority := authorityTT133
	if regime == RegimeTT99 {
		authority = authorityTT99
	}

	// Numbering and form codes are both open for the regime, and the set is
	// wider than what this package produces. All three are said, not omitted.
	warnings := []Warning{
		{Item: "mã số", Question: "Q2", Reason: reasonNumbering},
		{Item: "B03-DN, B09-DN", Question: "Q8", Reason: reasonFormSet},
	}
	if regime == RegimeTT133 {
		warnings = append(warnings, Warning{Item: "mã số và mẫu biểu", Question: "Q4", Reason: reasonTT133Form})
	}

	return []statementSpec{
		{
			code:              "B01-DN",
			name:              balanceSheet,
			authority:         authority,
			confidence:        Primary,
			indicatorQuestion: "Q2",
			indicatorReason:   reasonNumbering,
			warnings:          warnings,
			lines: []lineSpec{
				{
					label:      "Tài sản ngắn hạn và dài hạn (assets)",
					reference:  authority,
					confidence: Secondary,
					note:       "Every asset account the books hold at the period's end, net of any credit balance on it.",
					amount:     func(f figures) decimal.Decimal { return f.sheet.TotalAssets },
				},
				{
					label:      "Nợ phải trả (liabilities)",
					reference:  authority,
					confidence: Secondary,
					amount:     func(f figures) decimal.Decimal { return f.sheet.TotalLiabilities },
				},
				{
					label:      "Vốn chủ sở hữu (equity)",
					reference:  authority,
					confidence: Secondary,
					note:       "Equity accounts plus any result the period has not yet carried to them.",
					amount:     func(f figures) decimal.Decimal { return f.sheet.TotalEquity },
				},
			},
		},
		{
			code:              "B02-DN",
			name:              incomeStatement,
			authority:         authority,
			confidence:        Primary,
			indicatorQuestion: "Q2",
			indicatorReason:   reasonNumbering,
			warnings:          warnings,
			lines: []lineSpec{
				{
					label:      "Tổng doanh thu (total revenue)",
					reference:  authority,
					confidence: Secondary,
					amount:     func(f figures) decimal.Decimal { return f.profit.TotalRevenue },
				},
				{
					label:      "Tổng chi phí (total expenses)",
					reference:  authority,
					confidence: Secondary,
					amount:     func(f figures) decimal.Decimal { return f.profit.TotalExpenses },
				},
				{
					label:      "Kết quả kinh doanh trong kỳ (result for the period)",
					reference:  authority,
					confidence: Secondary,
					note:       "The period's revenue less its expenses: a loss is stated as a negative figure, which is how the form carries one.",
					amount:     func(f figures) decimal.Decimal { return f.profit.NetProfit },
				},
			},
		},
	}, nil
}

// vatReduction is the tax the 8% window gives up: the two points between the
// standard ten and the eight applied, on the turnover in the reduced band.
func vatReduction(net decimal.Decimal) decimal.Decimal {
	return roundDong(net.
		Mul(decimal.NewFromInt(int64(standardRateDelta))).
		Div(decimal.NewFromInt(int64(vat.Whole))))
}

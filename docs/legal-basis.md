# Legal Basis — Which Instruments Govern FinGo

**Status:** living document. **Last verified: 2026-10-08.**
**Rule:** any financial rule, rate, form code or account number in this repository must trace to a row in this file. If it does not, it is unverified and must not be relied on.

## Why this file exists

The review in [`accounting-review.md`](accounting-review.md) was written from model recall, not from source documents. That produced findings with correct engineering logic but a stale legal foundation — it reasoned from Circular 200/2014/TT-BTC as though it were current.

**On 1 January 2026 that changed.** We are now in FY2026. Vietnamese corporate accounting has split into **three coexisting regimes**, plus an IFRS track opening in 2027. This file records what is actually in force, with sources, so corrections are auditable rather than another act of recall.

## ⚠ Time-critical for the FY2026 close

**Circular 48/2019/TT-BTC — the instrument governing provisioning for CIT deductibility — is on a statutory countdown.**

Decision 1760/QĐ-BTC (1/7/2026) keeps Circulars 45/2013 and 48/2019 in force *"until replaced or repealed, but **no later than 31/12/2026**"* — [instrument status](https://vanban.chinhphu.vn/?classid=0&docid=218751&pageid=27160).

**Under three months from today.** Two easily-conflated regimes are running in parallel:

| Concern | Instrument | Status |
| :--- | :--- | :--- |
| Accounting presentation | Circular 99/2025 (or 133/2016, or 58/2026) | In force from FY2026 |
| CIT deductibility of provisions | Circular 48/2019 | In force, **expires ≤ 31/12/2026** |

## Confidence legend

| Mark | Meaning |
| :--- | :--- |
| `P` | Verified against primary text (instrument body, gazette, ministry, tax authority) |
| `S` | Verified against a named secondary source citing the instrument |
| `?` | Unresolved — do not build on this |

---

## Decision required: which regime does FinGo target?

**There are now four accounting tracks in Vietnamese law.** This is not a documentation detail — it determines the chart of accounts, the statutory form set, and the accounting books the application must produce. It must be decided before any COA seed is written.

| Regime | Instrument | Applies to | Status | Conf. |
| :--- | :--- | :--- | :--- | :--- |
| **SME** | Circular 133/2016/TT-BTC, as amended by **TT 46/2025/TT-BTC** (20/6/2025, effective 1/7/2025) | Small and medium enterprises | **In force.** Amended, not replaced | `P` |
| **General** | Circular 99/2025/TT-BTC | Enterprises generally; **SMEs may elect it** consistently for at least one fiscal year | In force from FY2026 | `P` |
| **Micro** | **TT 58/2026/TT-BTC** (signed 25/5/2026, effective 1/7/2026) | Micro enterprises | In force; replaces TT 132/2018 | `S` |
| **IFRS** | **TT 118/2026/TT-BTC** (18/8/2026, effective 1/1/2027) | Members of Vietnam's International Financial Centre, voluntary, unmodified IFRS | Issued, not yet effective | `S` |

**The repository currently declares `"TT133"`** at [`system.go:85`](../fingo/internal/domain/system/system.go#L85). **That is a valid choice** — Circular 133/2016 survives and is in force. My earlier flag that it might be superseded was **wrong**; verified against the verbatim text of Điều 31 khoản 1 of TT99, which lists the instruments it replaces and **does not include 133/2016**.

Two consequences follow, and both are real:

1. The default is **un-amended** — the COA and forms must reflect TT 46/2025, not the 2016 original.
2. `AccountingStandard` is a free-form string read by **nothing** in the codebase. Whichever regime is chosen, no behaviour currently switches on it. See *Impact on the repository*.

## Instrument register

| Instrument | Subject | Status | Effective | Source | Conf. |
| :--- | :--- | :--- | :--- | :--- | :--- |
| **99/2025/TT-BTC** | Corporate accounting regime | **In force.** Issued 27/10/2025. Replaces 200/2014/TT-BTC (except khoản 2 Điều 31), **plus TT 75/2015, TT 53/2016, TT 195/2012**. Does **not** replace 133/2016 | **01/01/2026**, fiscal years beginning on/after | [verbatim text](https://dichvuthue24h.com/thong-tu-99-2025-tt-btc/); gazette [PDF](https://congbao.chinhphu.vn/tai-ve-van-ban-so-99-2025-tt-btc-46529-59652) | `P` |
| 99/2025 — Điều 31(2) | 200/2014 survives **only** for state-enterprise equitisation accounting | Partially in force | 01/01/2026 | as above | `P` |
| 99/2025 — Điều 31(3) | SMEs **may elect** TT99, consistently for ≥1 fiscal year | In force | 01/01/2026 | as above | `P` |
| 99/2025 — Điều 29 | **Transition**: mandatory account conversion (138→2281; 2413→2414; 338 dividends→332; 441/466→4118) | Applies in the first period under TT99 | first TT99 period | as above | `P` |
| 99/2025 — Điều 30 | Transition: accounting-policy changes per VAS 29; no continuing accrual of major repairs; FX differences on 412 moved to 4211 | In force | first TT99 period | as above | `P` |
| 99/2025 — Điều 11, Phụ lục II | Chart of accounts. Company may amend names/codes/structure but **must not change FS indicators** | In force | 01/01/2026 | as above | `P` |
| 99/2025 — Điều 12, Phụ lục III | Accounting **books are reference-only**; companies may design their own | In force | 01/01/2026 | as above | `P` |
| 99/2025 — Điều 17 | Statutory form set; companies may **add** indicators but **must not re-number** prescribed Mã số | In force | 01/01/2026 | as above | `P` |
| 99/2025 — Điều 3 | **Internal governance and internal control**: enterprises must establish their own quy chế quản trị nội bộ and organise internal control | In force | 01/01/2026 | [SBLaw, quoting Điều 3](https://vi.sblaw.vn/so-sanh-thong-tu-200-2014-tt-btc-va-thong-tu-99-2025-tt-btc/) | `S` |
| **133/2016/TT-BTC** | SME accounting regime | **In force — survives TT99.** Amended, not replaced | 01/01/2017 | [status record](https://www.luatvietnam.net/vn/che-do-ke-toan-moi-danh-cho-doanh-nghiep-nho-va-vua-vbpl79188.html) | `P` |
| **46/2025/TT-BTC** | Amends Circular 133/2016 (Điều 3 amends Điều 10) | In force | 01/07/2025 | [chinhphu.vn](https://chinhphu.vn/?pageid=27160&docid=214192) | `P` |
| **58/2026/TT-BTC** | Micro-enterprise regime; replaces TT 132/2018. Books only, **no chart of accounts**; only Báo cáo tình hình tài chính + Báo cáo KQKD | In force | 01/07/2026 | [MOF-authored report](https://dangcongsan.vn/botaichinh/tin-tuc-hoat-dong/quy-dinh-moi-ve-che-do-ke-toan-cho-doanh-nghiep-sieu-nho.html) | `S` |
| **118/2026/TT-BTC** | IFRS adoption for International Financial Centre members | Issued, effective 01/01/2027 | 01/01/2027 | [VACPA](https://vacpa.org.vn/vi/van-ban-phap-luat/bo-tai-chinh-ban-hanh-thong-tu-huong-dan-ap-dung-chuan-muc-quoc-te-ve-ke-toan-tai-viet-nam.htm) | `S` |
| 152/2025/TT-BTC | Accounting for household businesses (hộ kinh doanh) | Reported in force | reported 01/01/2026 | as above | `?` |
| Draft circular to replace 133/2016 | — | **Not issued** as of 2026-10-08 | — | [draft](https://www.meinvoice.vn/tin-tuc/52063/du-thao-thong-tu-thay-the-thong-tu-133/) | `?` |
| **No early-adoption right in TT99** | Điều 31 read in full — no "áp dụng sớm" clause | — | — | verbatim text | `P` (negative finding) |
| 200/2014/TT-BTC | Prior general regime | Superseded except equitisation | 01/01/2015 → 31/12/2025 | [luatvietnam](https://luatvietnam.vn/thue-phi-le-phi/thong-tu-200-co-het-hieu-luc-tu-01-01-2026-565-105171-article.html) | `S` |
| **Circular 48/2019/TT-BTC** (am. by **TT 24/2022**) | Provisions: doubtful receivables, inventory write-down. CIT-deductibility basis | **In force — expires ≤ 31/12/2026** | 10/10/2019 | [status](https://vanban.chinhphu.vn/?classid=0&docid=218751&pageid=27160) | `P` (status) / `S` (bands) |
| **QĐ 1760/QĐ-BTC** | Sunset clause on Circulars 45/2013 and 48/2019 | In force | 01/07/2026 | as above | `P` |
| **Luật BHXH 41/2024/QH15** | Compulsory social insurance — rates, base, caps | In force | 01/07/2025 | [chinhphu.vn](https://xaydungchinhsach.chinhphu.vn/print/nghi-dinh-158-2025-nd-cp-quy-dinh-moi-ve-bao-hiem-xa-hoi-bat-buoc-119250626172814006.htm) | `P` |
| **NĐ 158/2025/NĐ-CP** | Details of compulsory BHXH; contribution base; BHTNLĐ-BNN rate | In force | 01/07/2025 | [gazette](https://congbao.chinhphu.vn/van-ban/nghi-dinh-so-158-2025-nd-cp-45240.htm) | `P` |
| NĐ 188/2025/NĐ-CP | BHYT rates (employer ⅔, employee ⅓) | In force | 01/07/2025 | [baohiemxahoi.gov.vn](https://baohiemxahoi.gov.vn/) | `S` |
| **Luật Việc làm 74/2025/QH15** + NĐ 374/2025 | Unemployment insurance; cap = 20× regional minimum wage | In force | 01/01/2026 | [MoJ HCMC](https://sotuphap.hochiminhcity.gov.vn/tin-tuc-su-kien/-/asset_publisher/emZ19pfF3Yyq/content/id/4450174/) | `S` |
| **Luật Công đoàn 50/2024/QH15** (am. by Luật 97/2025) + NĐ 191/2013 | Trade-union fee (KPCĐ) 2%, employer only | In force | 01/07/2025 | [hcc.nghean.gov.vn](https://hcc.nghean.gov.vn/tin-tong-hop/quy-dinh-moi-ve-mien-giam-tam-dung-dong-kinh-phi-cong-doan-1915.html) | `S` |
| **NĐ 161/2026/NĐ-CP** | lương cơ sở → 2,530,000; also "mức tham chiếu" | In force | 01/07/2026 | [baohiemxahoi.gov.vn](https://baohiemxahoi.gov.vn/congkhai/pages/lich-lam-viec-cua-lanh-dao-nganh.aspx?CateID=0&ItemID=26780&OtItem=date) | `P` (figure) / `S` (number) |
| **NĐ 293/2025/NĐ-CP** | Regional minimum wage from 01/1/2026; repeals NĐ 74/2024 | In force | 01/01/2026 | [chinhphu.vn](https://xaydungchinhsach.chinhphu.vn/) | `S` |
| QĐ 345/QĐ-BTC | VAS/IFRS roadmap policy basis | In force | 2020 | [KPMG](https://kpmg.com/vn/vi/insights/2025/08/draft-circular-replace-circular-200-and-vietnam-ifrs-roadmap.html) | `S` |
| **Luật Quản lý thuế 108/2025/QH15** | Framework tax-administration law — **replaces 38/2019/QH14** | In force | 01/07/2026 | [chinhphu.vn](https://xaydungchinhsach.chinhphu.vn/print/mot-so-diem-moi-va-huong-dan-trien-khai-thuc-hien-nghi-dinh-252-2026-nd-cp-119260724164608156.htm) | `S` |
| **NĐ 252/2026/NĐ-CP** | Implementing decree — **replaces NĐ 126/2020, NĐ 91/2022, NĐ 117/2025**. Quarterly provisional CIT ≥80% rule (Điều 24); filing deadlines (Điều 10) | In force | 01/07/2026 | as above | `S` |
| **NĐ 254/2026/NĐ-CP** | E-invoices and e-documents — **replaces NĐ 123/2020 (as amended by NĐ 70/2025)** | In force | 01/07/2026 | [EY](https://www.ey.com/vi_vn/technical/tax/tax-and-law-updates/nghi-dinh-254-2026-nd-cp-ve-hoa-don-dien-tu-va-chung-tu-dien-tu), [chinhphu.vn](https://xaydungchinhsach.chinhphu.vn/nhung-diem-moi-cua-nghi-dinh-254-2026-nd-cp-va-thong-tu-91-2026-tt-btc-ve-hoa-don-dien-tu-chung-tu-dien-tu-119260717143502375.htm) | `S` |
| **Thông tư 91/2026/TT-BTC** | E-invoice implementation — **replaces TT 32/2025, which replaced TT 78/2021** | In force | 01/07/2026 | [EY](https://www.ey.com/vi_vn/technical/tax/tax-and-law-updates/nghi-dinh-254-2026-nd-cp-ve-hoa-don-dien-tu-va-chung-tu-dien-tu) | `S` |
| **Thông tư 89/2026/TT-BTC** | **Return forms.** Phụ lục I carries 01/GTGT, 03/TNDN and others | In force | 01/07/2026 | [MISA](https://amis.misa.vn/282421/tai-to-khai-thue-tncn-thue-gtgt-theo-thong-tu-89/) | `S` |
| **Luật Thuế GTGT 48/2024/QH15** | VAT law — **replaces 13/2008/QH12** | In force | 01/07/2025 | [Công báo PDF](https://datafiles.chinhphu.vn/cpp/files/vbpq/2025/01/luat48.pdf) | `P` |
| **Nghị quyết 204/2025/QH15** + **NĐ 174/2025/NĐ-CP** | **8% reduced-VAT window** for goods/services in khoản 3 Điều 9 Luật 48/2024, excluding telecom, finance/banking/securities/insurance, real estate, metal products, mining (except coal), excise goods (except petrol) | **In force — expires 31/12/2026**, reverting to 10% | 01/07/2025 – 31/12/2026 | [full text](https://xaydungchinhsach.chinhphu.vn/print/toan-van-nghi-quyet-so-204-2025-qh15-ve-giam-thue-gia-tri-gia-tang-119250628082120511.htm) | `P` (resolution) / `S` (decree) |
| **Luật Thuế TNDN 67/2025/QH15** | CIT law. Điều 10 rates: **20%** standard; **15%** if turnover ≤ 3bn; **17%** if >3bn–≤50bn; 25–50% oil & gas; 50% rare resources | In force | **01/10/2025** | [chinhphu.vn](https://xaydungchinhsach.chinhphu.vn/print/thue-suat-thue-thu-nhap-doanh-nghiep-moi-ap-dung-tu-1-10-2025-119250730082233732.htm) | `S` |
| **Luật Thuế TNCN 109/2025/QH15** | **New PIT law — supersedes Luật 04/2007/QH12.** Salary/business-income rules apply from **tax year 2026** | In force | 01/07/2026 | [baochinhphu](https://en.baochinhphu.vn/new-policies-take-effect-in-july-2026-111260629075746181.htm) | `S` |
| **Nghị quyết 110/2025/UBTVQH15** | **Family deductions raised**: self **15,500,000/month** (186,000,000/yr); dependent **6,200,000/month**. Nghị quyết 954/2020 ceases | In force | **01/01/2026** | [full text](http://files.hethongphapluat.com/nghi-quyet-110-2025-ubtvqh15-dieu-chinh-muc-giam-tru-gia-canh-cua-thue-thu-nhap-ca-nhan-do-uy-ban-thuong-vu-quoc-hoi-ban-hanh.html) | `S` |
| **Luật Thuế TNCN 109/2025 — bracket table** | **Progressive brackets reduced from 7 to 5** from tax year 2026 (see reference below) | Applies from tax year 2026 | 01/07/2026 | [chinhphu.vn](https://xaydungchinhsach.chinhphu.vn/print/chi-tiet-bieu-thue-thu-nhap-ca-nhan-luy-tien-tung-phan-119260327091407544.htm) | `S` (table) / `?` (article no.) |

---

## What changed in the chart of accounts and forms (Circular 99/2025)

Numbering is **not** wholesale-reworked — 3-digit cấp-1 codes are essentially retained.

**Removed:**

| Item | Note |
| :--- | :--- |
| **TK 611 "Mua hàng"** | Absent from Phụ lục II. The periodic-inventory account is gone. |
| **TK 1562 "Chi phí thu mua hàng hóa"** | 156 has no prescribed cấp-2; small purchase costs may go straight to 632. |
| Prescribed cấp-2 of **511, 515, 521, 211, 213, 156** | e.g. 5111/5112/5113 and 5211/5212/5213 no longer prescribed — companies open their own detail accounts |

**Added:** 215 "Tài sản sinh học" (2151/21511/21512/2152/2153) · **2295** "Dự phòng tổn thất tài sản sinh học" · 1383 "Thuế TTĐB của hàng nhập khẩu" · **332** "Phải trả cổ tức, lợi nhuận" (split from 338) · 337 · split 2413/2414 · 3431/3432 · 3523 · 3561/3562 · 41111/41112 · 82111/82112

**Retained:** 153 · 242 · 511 · 515 · 521 · 632 · 635 · 641 (+6411–6418) · 642 (+6421–6428) · 711 · 811 · 911 · 4211/4212 · 2293 · 2294

**Statutory financial statements** — still the B01/B02/B03 family, but:

| Code | Name | Change |
| :--- | :--- | :--- |
| **B01-DN** | **Báo cáo tình hình tài chính** | **Renamed** (was "Bảng cân đối kế toán") |
| B02-DN | Báo cáo kết quả hoạt động kinh doanh | — |
| B03-DN | Báo cáo lưu chuyển tiền tệ | — |
| **B09-DN** | Bản thuyết minh BCTC | Now in the set |

New variants: non-going-concern **B01-DNKLT / B02-DNKLT / B03-DNKLT / B09-DNKLT**; interim full **B01a-DN … B09a-DN**; interim condensed **B01b-DN / B02b-DN / B03b-DN**.

**Accounting books** (Phụ lục III, 42 forms, reference-only under Điều 12): **S03a-DN** "Sổ Nhật ký chung" and **S03b-DN** "Sổ Cái" are **retained unchanged**. **S08-DN** renamed "Sổ tiền gửi không kỳ hạn".

> **Correction to the review.** [`accounting-review.md`](accounting-review.md) cites **"S07a-DN (Sổ quỹ tiền mặt)"**. That is **wrong**. Cash-book is **S07-DN**; **S07a-DN** is "Sổ kế toán chi tiết quỹ tiền mặt". This was already the position under Circular 200 — my error, not a change in the law.

---

## Open questions — do not build on these until resolved

**Q2 — Mã số chỉ tiêu numbering.** Whether FS indicator numbers changed vs Circular 200 requires Phụ lục IV, which could not be read (gazette PDF not machine-readable). Secondary sources say only that "mẫu biểu khác với TT200". **Every indicator number in the review is provisional.**

**Q3 — TT133 chart of accounts — RESOLVED.** Verified from the official Công báo text-layer PDF (Công báo số 1103+1104 ngày 12-10-2016, printed pp. 77–82) — the `vanban.chinhphu.vn` signed file is a text-less 190-page scan that **excludes all appendices**, so the gazette is the only official machine-readable route.

| Question | Answer |
| :--- | :--- |
| Does 133 prescribe a chart? | Yes — **Phụ lục 1**, "DANH MỤC HỆ THỐNG TÀI KHOẢN KẾ TOÁN ÁP DỤNG CHO DOANH NGHIỆP NHỎ VÀ VỪA", exactly **49 cấp-1 accounts** (Số TT 01–49) |
| Did Circular 46/2025 change it? | **No.** Điều 3 renamed Điều 10 to "Sửa đổi Chế độ kế toán" and replaced it, moving chart modification from **Ministry pre-approval to enterprise self-determination**. No account added, renamed or removed |
| **TK 611 "Mua hàng"?** | **EXISTS** under 133 (Số TT 41). TT99 removed it — a TT99 fact, not a 133 fact |
| **TK 1562?** | **Does not exist.** TK 156 has no prescribed cấp-2; "Chi phí thu mua hàng hóa" is only a Bên Nợ caption |
| **511 cấp-2?** | **Prescribed:** 5111, 5112, 5113, 5118 |
| **TK 521?** | **Does not exist under 133.** No 5211/5212/5213 — revenue deductions debit TK 511 directly |
| TK 621 / 622 / 627 / 641? | **Absent** from Phụ lục 1. Operating expenses are **6421** (bán hàng) and **6422** (quản lý doanh nghiệp) |
| TK 242 name | **"Chi phí trả trước"** under 133. "Chi phí chờ phân phổ" is the **99/2025** name — a trap |
| TK 2293 / 2294 / 153? | All exist |

**Vendor copies carry typos.** Aggregator/vendor reproductions print `5111 5112 5111 5118`, `4111 4111 4118`, `2141 2142 2141 2147`. The gazette gives `5111/5112/5113/5118`, `4111/4112/4118`, `2141/2142/2143/2147`. **Never take account codes from a vendor copy.**

**Draft replacement, not issued.** A draft replacing 133 exists (Ministry workshop 02/10/2026; issuance expected end-2026, effect proposed 01/01/2028, chart cut 49 → 22). **Not an instrument — do not build on it.**

The seed is implemented as `TT133Chart()` in `fingo/internal/domain/gl/coa_tt133.go`, guarded by tests that assert 49 cấp-1 accounts, the trading accounts, and the **absence** of 521 / 621 / 622 / 627 / 641 / 1562.

**Q4 — Statutory book form codes under TT133** as amended: unconfirmed.

**Q5 — PIT and VAT — RESOLVED, and materially changed.** The new PIT law raises deductions to 15,500,000 / 6,200,000 and cuts the brackets from 7 to 5. The 8% VAT window runs to 31/12/2026. See *Tax* above. Remaining `?`: the PIT bracket article number, and the 01/GTGT indicator numbers (Phụ lục I of TT 89/2026 was not readable).

**Q6 — Historical payroll.** Pre-01/7/2025 employer BHXH was cited as 18%, now 17% (14% + 3%). Which instrument set 18%, and whether any enactment formally reduced it, is **unresolved**.

**Q7 — BHTNLĐ-BNN reduced rate.** A BHTN contribution reduction for employers hiring persons with disabilities is reported. Rate and article **not confirmed**.

**Q8 — Phụ lục I–IV contents generally.** The appendices carry the COA, book forms and indicator numbers. The instrument body was read verbatim; the appendices were not. Everything derived from them is `S` at best.

---

## Impact on the repository

| Item | Current state | Consequence |
| :--- | :--- | :--- |
| `AccountingStandard` default `"TT133"` ([`system.go:85`](../fingo/internal/domain/system/system.go#L85)) | **Valid regime**, but a free-form string read by nothing | Un-amended (pre-TT46/2025), and inert — no behaviour switches on it. A regime decision is required regardless. |
| [README.md](../README.md#L3) claims compliance with 200/2014 and 133/2016 | Stale | FY2026 no longer uses 200/2014 for general presentation |
| [GEMINI.md](../GEMINI.md) cites "VAS/Circular 133/200" | Stale | Same |
| **Internal control: no auth, no segregation of duties, no approval workflow, no persisted audit trail** | Absent | **Newly material.** Điều 3 of TT99 makes internal control an explicit obligation; Circular 200 had no equivalent. Under a TT133 target, verify whether TT46/2025 introduced something comparable before assuming it does not apply. |
| Periodic inventory (TK 611) modelled anywhere? | Not modelled | Under TT99, **TK 611 no longer exists** — which makes the absence moot *if* TT99 is the target. Under TT133, unverified (`Q3`). |
| Purchase cost allocation (TK 1562) | Not modelled | Under TT99, **TK 1562 no longer exists**; small purchase costs may be expensed to 632. This **changes the shape of the review's landed-cost finding** — it does not remove the need for landed cost on imports, but it removes the account. |
| Statement forms and book codes | Modelled on the 200/2014 family | B01-DN renamed; B09-DN added; provisional pending `Q2`/`Q4` |

## Tax — the administration layer was replaced on 01/07/2026

**This is the single most disruptive change found.** Almost every tax citation in the review is obsolete, because the instruments it named were replaced wholesale.

| Replaced | By | Effective |
| :--- | :--- | :--- |
| Luật Quản lý thuế 38/2019/QH14 | **Luật 108/2025/QH15** | 01/07/2026 |
| NĐ 126/2020 · NĐ 91/2022 · NĐ 117/2025 | **NĐ 252/2026/NĐ-CP** | 01/07/2026 |
| NĐ 123/2020 (+ NĐ 70/2025) | **NĐ 254/2026/NĐ-CP** | 01/07/2026 |
| TT 78/2021 → TT 32/2025 | **TT 91/2026/TT-BTC** | 01/07/2026 |
| Return forms under TT 80/2021 | **TT 89/2026/TT-BTC**, Phụ lục I | 01/07/2026 |
| Luật TNCN 04/2007/QH12 | **Luật 109/2025/QH15** | 01/07/2026 |

### PIT — changed materially for tax year 2026

| Item | Was | **Now** |
| :--- | :--- | :--- |
| Personal deduction | 11,000,000/month | **15,500,000/month** (186,000,000/yr) |
| Dependent deduction | 4,400,000/month | **6,200,000/month** |
| Brackets | 7 | **5** |

New bracket table (annual / monthly taxable income → rate):

| # | Taxable income | Rate |
| ---: | :--- | ---: |
| 1 | ≤ 120m / yr (≤10m / month) | 5% |
| 2 | >120m–360m (>10–30m) | 10% |
| 3 | >360m–720m (>30–60m) | 20% |
| 4 | >720m–1,200m (>60–100m) | 30% |
| 5 | >1,200m (>100m) | 35% |

Bracket 2 fell from 15%→10% and bracket 3 from 25%→20%; the 35% top rate is unchanged.

> The review's citation of **11,000,000 / 4,400,000 is now confirmed wrong** — not merely stale, as I hedged. Both figures roughly quadrupled in effect across deductions and bracket structure.

### VAT

- **Standard 10%**; **5%** reduced list; **0%** for exports and listed services (with exclusions in Điều 9 khoản 1 điểm d); non-taxable list in **Điều 5** (~26 groups + khoản 27–28).
- **The 8% window runs to 31/12/2026**, then reverts to 10%. No 2026 instrument extending it was found (`?` — absence of evidence).
- **The 8% break-out is NOT a rate line on 01/GTGT.** It is a **separate appendix** to the return — "GIẢM THUẾ GTGT THEO NGHỊ QUYẾT 204/2025/QH15", mẫu 01, indicators [01]–[09] (NĐ 174/2025, Phụ lục III). `S`
- **01/GTGT under TT 89/2026** (from 01/07/2026): input VAT indicators **[23]–[25]**; output rate lines **[26]** non-taxable, **[29]** 0%, **[30]/[31]** 5%, **[32]/[33]** 10%; **[22]** carry-forward in, **[43]** carry-forward out, **[40]** payable. `S` — **indicator numbers are not primary-verified**; Phụ lục I could not be read.
- **Monthly vs quarterly threshold unchanged: prior-year turnover ≤ 50 billion VND** may elect quarterly. `S`

### CIT

- **20% standard**, with **15%** (turnover ≤ 3bn) and **17%** (>3bn–≤50bn) for smaller enterprises. `S`
- **Provisional quarterly payments must total ≥80%** of the annual finalisation liability, else late-payment interest applies (NĐ 252/2026, Điều 24). `S`
- **No provisional-payment exemption threshold found — do not assert one.** The widely-repeated "≤1 billion" carve-out was a *proposal*, not enacted text. `?`
- Finalisation deadline: **last day of the 3rd month** after period end (31 March for a calendar-year taxpayer). `S`

### E-invoicing

- **Cancellation ("hủy") as a correction mechanism has been abolished.** The operative mechanisms are **điều chỉnh (adjustment)** and **thay thế (replacement)**. `S`
- The general rule is now **e-invoices WITH a tax-authority code**, with a listed-sector exception; household/individual businesses over **1 billion VND/year** must use coded or cash-register e-invoices. `S`
- Tax-authority **pre-printed** invoices ceased to have value from **01/07/2026**. `S`

## Verified figures — payroll and provisions

In force as of 2026-10-08. Primary-sourced unless the row says otherwise.

### Statutory contribution rates

| Component | Employer | Employee | Instrument |
| :--- | ---: | ---: | :--- |
| BHXH — hưu trí & tử tuất | 14% | 8% | Luật BHXH 41/2024/QH15, Điều 33–34 `P` |
| BHXH — ốm đau & thai sản | 3% | — | Luật BHXH 41/2024, Điều 32 & 34 `P` |
| BHYT | 3% | 1.5% | Luật BHYT (sđ 2024) Điều 13; NĐ 188/2025 `S` |
| BHTN | 1% | 1% | Luật Việc làm 74/2025, Điều 33; NĐ 374/2025 `S` |
| BHTNLĐ-BNN | 0.5% | — | NĐ 158/2025, khoản 2 Điều 43 — 0.3% on approval `S` |
| KPCĐ | 2% | — | Luật Công đoàn 50/2024, điểm b khoản 1 Điều 29 `S` |
| **Total** | **23.5%** | **10.5%** | Grand total 34% of base |

KPCĐ applies to **all** employers — no headcount threshold, no requirement to have a grassroots union (NĐ 191/2013, Điều 4–5).

### Base and caps

- **Base = the separately declared insurable salary** (`tiền lương làm căn cứ đóng BHXH`) = contractual salary + salary allowances + other regularly and stably paid supplements. **Not gross salary.** Bonuses, mid-shift meal, fuel/phone/travel/housing/childcare support, and allowances varying with productivity or quality are excluded. `P`
- **Cap for BHXH / BHYT / BHTNLĐ-BNN** = 20 × mức tham chiếu = 20 × 2,530,000 = **50,600,000 VND/month**. `P`
- **Cap for BHTN** = 20 × regional minimum wage: R1 106,200,000 · R2 94,600,000 · R3 82,800,000 · R4 74,000,000. R1/R2 `S`; R3/R4 arithmetic `?`
- **lương cơ sở / mức tham chiếu** = **2,530,000 VND/month** from 01/7/2026. Luật BHXH 41/2024 introduces "mức tham chiếu"; until lương cơ sở is abolished the two are equal (Điều 5 NĐ 158/2025).
- **Regional minimum wage** from 01/1/2026 (NĐ 293/2025, Điều 3): R1 5,310,000 · R2 4,730,000 · R3 4,140,000 · R4 3,700,000 VND/month. NĐ 74/2024 repealed.

### Accounting provisions — Circular 48/2019, expires ≤ 31/12/2026

Doubtful receivables (`S` — bands not primary-verified):

| Overdue | Provision |
| :--- | ---: |
| >6 months – <1 year | 30% |
| 1 – <2 years | 50% |
| 2 – <3 years | 70% |
| ≥3 years | 100% |

- Overdue condition: ≥6 months with documented dunning, or not yet due with evidence of inability to pay.
- Provision = rate × receivable after offsetting payables per the reconciliation record.
- **Inventory write-down** = cost − net realisable value, **item by item**; NRV self-determined as estimated selling price − costs to complete − costs to sell.
- **Reversal:** required = balance → no entry; required > balance → add difference to expense; required < balance → reverse (`hoàn nhập`) and reduce expense.
- **Timing: annual financial-statement date only — not quarterly.**
- Accounts: TK 2293 (doubtful receivables), TK 2294 (inventory). TT99 adds TK 2295 for biological assets.

### Correction to the review's payroll finding

The review recorded the employer rate as a three-way contradiction: code 21.5%, its own comment 23%, README 23.5%. Resolution:

- **21.5% = 17% BHXH + 3% BHYT + 1% BHTN + 0.5% BHTNLĐ-BNN.** The *insurance cluster*. The code's constant is arithmetically **right for that cluster**.
- **23.5% adds the 2% KPCĐ** and is the true employer total.

The constant is defensible; the **comment is wrong** — it claims KPCĐ is included and omits the 0.5% fund. Critical severity stands on three independent grounds: **KPCĐ unmodelled**, **base wrong** (gross instead of insurable salary), **caps absent**.

## Verification protocol

1. Every rate, threshold, form code and account number used in code must cite a row here.
2. A row marked `?` must not be built on. Resolve it first.
3. Re-verify the whole register at each fiscal-year boundary and after any amendment.
4. Prefer gazette text at [congbao.chinhphu.vn](https://congbao.chinhphu.vn/) over any secondary summary, including this one.
5. **Do not port changes between regimes.** A TT99 change is not a TT133 change. The regimes are legally distinct.

### Provenance

Gathered 2026-10-08. The body of Circular 99/2025 was obtained as **verbatim instrument text**; its appendices (Phụ lục I–IV) were **not** machine-readable, which is the source of `Q2`, `Q3` and `Q8`. Gazette PDFs could not be parsed by the fetch tooling used. thuvienphapluat.vn returned HTTP 403 throughout. Secondary sources are professional firms, Vietnamese accounting-software vendors and state-owned press — reliable for *locating* an instrument, not authority for its content. Treat every `S` row as needing promotion to `P` before it justifies a code decision.

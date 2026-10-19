# Pilot scenario — one trading client, one month

The acceptance test for the whole goal. It is scripted so it can be run through the HTTP
interface and its numbers checked by hand; it is not a demo, and a step that cannot be
completed is a defect.

## The client

**Công ty TNHH Sao Mai** — a small trader in Ho Chi Minh City.

| Fact | Value |
| :--- | :--- |
| Tax code | `0312345678` |
| Regime | TT133/2016, VAT **deduction method** |
| Fiscal year | calendar year |
| Month | **05/2026** |
| Opening position | cash 500.000.000 · inventory 120.000.000 (300 units @ 400.000 of `HH-A`) · capital 620.000.000 |
| Suppliers | *Đại Phát* (goods), *Điện lực* (utility) |
| Customers | *Minh Long*, *Hòa Bình* |

## The month, in the order a practice does it

| # | Step | Expected result |
| ---: | :--- | :--- |
| 1 | Create the client, open its chart | 123 accounts |
| 2 | Import opening balances: cash 500.000.000, 156 120.000.000, 4111 620.000.000 | one balanced voucher; trial balance ties |
| 3 | Import **opening stock**: 300 × `HH-A` @ 400.000 | stock list shows 300 on hand, 120.000.000 value; 156 unchanged |
| 4 | Buy goods on credit from Đại Phát: 200 × `HH-A` @ 420.000 (+10% VAT) | stock receipt 156/331 **and** an open AP document of 92.400.000 |
| 5 | Sell 250 units to Minh Long at 700.000 (+10% VAT) | revenue 175.000.000, output VAT 17.500.000, COGS at moving average, stock 250 left |
| 6 | Collect 100.000.000 from Minh Long (partial) | invoice partially paid; aging shows the remainder |
| 7 | Pay Đại Phát 50.000.000 (partial) | AP document partially paid; 331 reduced |
| 8 | Utility bill from Điện lực 3.000.000 at 5% | expense 6422, input VAT 150.000, payable 3.150.000 |
| 9 | Sales return: Minh Long returns 10 units at the invoice price | credit note: 521/33311 reduced, stock +10 back at **original cost**, receivable reduced |
| 10 | Stocktake: 2 units missing | shortage costed at average, 632/156 |
| 11 | Reverse one wrong expense voucher | mirror voucher; expense nets to zero |
| 12 | Reports | trial balance balanced; P&L profit = revenue − COGS − expenses; balance sheet balances |
| 13 | VAT summary | output 17.500.000 − 1.750.000 (return) = 15.750.000; input 150.000 + purchase VAT; net stated |
| 14 | Aging | **zero difference** against 131 and 331 |
| 15 | Close preview | shows closing journal, result, outstanding documents, lock date |
| 16 | Confirm close (typed period) | period locked; P&L accounts zero; 911 zero; result in 4212 |
| 17 | Attempt to post into the closed month | refused with a Vietnamese message, by the app **and** the database |
| 18 | Handover export | zip of CSVs for this client only |
| 19 | Print trial balance and P&L | A4, client name and tax code, `1.234.567`, page numbers |

## Numbers to check by hand

| Figure | Expected |
| :--- | ---: |
| Opening trial balance | debit = credit = 620.000.000 |
| Moving average after the purchase | (120.000.000 + 84.000.000) ÷ 500 = **408.000** |
| COGS on 250 units | 250 × 408.000 = **102.000.000** |
| Stock after sale | 250 units, 102.000.000 |
| Return of 10 units | stock +10 @ original 408.000 = 4.080.000; COGS reversed 4.080.000 |
| Output VAT for the month | 17.500.000 − 1.750.000 = **15.750.000** |
| Input VAT | 8.400.000 (purchase) + 150.000 (utility) = **8.550.000** |
| VAT payable | **7.200.000** |
| Net profit (before stocktake) | revenue 175.000.000 − 10×700.000 = 168.000.000; COGS 102.000.000 − 4.080.000; expenses 3.000.000 |

The numbers are for the scripted dataset; the test asserts **the identities** (131/331 =
subledgers, 156 = stock value, assets = liabilities + equity), not these literal figures
only, so the scenario survives a change in the seed.

## What "done" means for this scenario

Every row above completes through HTTP with no manual SQL, no English error reaching the
operator, and the invariant checks green. Until then, the product is not pilot-ready.

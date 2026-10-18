# Arithmetic: VAT and rounding

## Method

Oracle written independently in `review_tmp/oracle/main.go`: for net `n` and rate `bp`,
expected = `sign(n) × floor((|n|×bp + 5000) / 10000)` in `big.Int` — half away from zero,
integer only. Compared against `vat.VATRate.Tax`, which uses its own `big.Int`
implementation (`internal/vat/vat.go:171`).

## Result: 25/25 match

```
net=1   bp=1000 app=0          oracle=0
net=5   bp=1000 app=1          oracle=1      (half away from zero at exactly .5)
net=15  bp=1000 app=2          oracle=2
net=49  bp=1000 app=5          oracle=5
net=50  bp=1000 app=5          oracle=5
net=51  bp=1000 app=5          oracle=5
net=7   bp=800  app=1          oracle=1      (8% window arithmetic)
net=19  bp=800  app=2          oracle=2
net=1_100_000 bp=1000 app=110_000 oracle=110_000
net=9_223_372_036_854 bp=1000 app=922_337_203_685 oracle=922_337_203_685
net=-5  bp=1000 app=-1         oracle=-1
net=-15 bp=1000 app=-2         oracle=-2
mismatches=0 of 25
```

Raw output: `vat-oracle.out.txt`. The int64-scale case is ~9.2×10¹² đồng, far above any
plausible invoice, and it did not overflow — the implementation multiplies in `big.Int`.

## Boundary cases that matter and passed

| Case | Expected (hand) | App | Verdict |
| :--- | :--- | ---: | :--- |
| 1 đồng at 10% | 0 (0.1 rounds down) | 0 | PASS |
| 5 đồng at 10% | 1 (exactly 0.5, away from zero) | 1 | PASS |
| 4 đồng at 10% | 0 (0.4) | 0 | PASS |
| 15 đồng at 10% | 2 (1.5) | 2 | PASS |
| 7 đồng at 8% | 1 (0.56) | 1 | PASS |
| 6 đồng at 8% | 0 (0.48) | 0 | PASS |
| negative net | magnitude-rounded, sign kept | matches | PASS |

## Not proven in this pass

| Item | Why | Status |
| :--- | :--- | :--- |
| Document total = Σ rounded lines | the engine computes tax per line (`Charge.Gross`), so it should hold by construction; no run in this session | `[UNVERIFIED]` |
| Mixed rates on one document | **the VAT registry cannot hold two active rates covering one date** (documented limitation, `internal/books/sales.go` registers only `vat10`) | **FAIL (scope)** |
| 8% reduction window end date | no rate row exists for 8%, so the window cannot be exercised through the app | **FAIL (scope)** |
| KCT / exempt / non-declared categories | the model has no category field | **FAIL (scope)** |
| Allocation largest-remainder at N=1…1000, amounts to 10¹² | not run in this session | `[UNVERIFIED]` |
| Moving average drift over 1,000 alternating operations; Σ 156 = stock value | not run in this session | `[UNVERIFIED]` |
| Overflow on `Money.Mul` (plain int64, no big.Int at `money.go:90`) | not exercised at the limit | `[UNVERIFIED]`, code-read risk |

## The one arithmetic finding worth recording now

`money.Money.Mul` multiplies two `int64` values directly (`internal/money/money.go:90`),
unlike VAT, allocation and average cost which all use `big.Int`. A quantity × unit-price
product near the int64 ceiling would wrap silently. The sales path multiplies
`quantity × unitPrice` before money is formed, and no test in the suite drives that to
the boundary. **Severity MEDIUM**, because the input would have to be an absurd unit price
or quantity — but "absurd" is a judgement the code does not enforce.

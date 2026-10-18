# Performance and capacity

## Dataset measured

`fingo_test` had accumulated **231,855 vouchers / 464,771 lines** from earlier
load-test seeding — a useful upper bound for a mid-size client, and larger than the
50k-line figure in `docs/v2/performance.md`.

| Query | Plan | Time |
| :--- | :--- | ---: |
| Trial-balance aggregation (`GROUP BY account_code, side`, all posted vouchers in 2026) | `Finalize GroupAggregate` over parallel seq scans, 2 workers, `shared hit=2348 read=9029` | **803 ms** |

`EXPLAIN (ANALYZE, BUFFERS)` excerpt:

```
Finalize GroupAggregate  (cost=18884.35..18891.26 rows=26 width=42)
                         (actual time=788.025..803.562 rows=20 loops=1)
  Buffers: shared hit=2348 read=9029 dirtied=12
  ->  Gather Merge  (cost=… ) (actual time=788.000..803.475 rows=60 loops=1)
        Workers Planned: 2   Workers Launched: 2
        ->  Sort  (actual time=650.961..650.966 rows=20 loops=3)
```

Application-level figures from the project's own test (`TestVolumeIsSurvivable`,
50,000 lines, one client, RLS on):

| Operation | Time |
| :--- | ---: |
| Trial balance (25,000 vouchers) | 767 ms |
| Profit and loss | 759 ms |
| Journal read (50,000 lines) | 690 ms |

## Indexes

| Table | Indexes |
| :--- | :--- |
| `vouchers` | `pkey`, `(client_id, voucher_no)`, `(client_id, book, fiscal_year, seq)`, `(client_id, voucher_date)` |
| `voucher_lines` | `pkey`, `(voucher_id, line_order)`, `(voucher_id)` |

**No index on `voucher_lines (client_id, account_code, side)`.** Every report
aggregates by account, and the plan shows sequential scans. At 465k lines the cost is
0.8 s, which meets the realistic target; at 5–10× that it will not, and the missing
index is the first lever.

## Concurrency

| Test | Result |
| :--- | :--- |
| 20 parallel posts, same book | 20/20 succeeded, 20 distinct `voucher_no`, 20 distinct `seq`, debits = credits |
| Deadlocks | none observed |
| Lost updates | none observed (advisory lock serialises numbering) |

## Capacity assumptions not tested

| Assumption | Status |
| :--- | :--- |
| 50 clients × 5 years × 2,000 vouchers/month (~600k lines per client) | **NOT TESTED**; the largest measured is 465k lines in one database |
| Backup/restore time at production size | not tested (78 KB dump) |
| Memory of the binary under concurrent report generation | not measured |
| Connection pool sizing under 50 concurrent users | not measured; pool default from `store.Config` |

## Verdict for domain 8

**CONDITIONAL PASS.** Single-client report latency is acceptable at the measured size
and degrades linearly; the missing composite index is the known lever. Nothing measured
supports a claim of capacity at the stated upper bound.

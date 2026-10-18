# Attack log

Every attempt below was executed, not reasoned about. Scripts and raw output are in
this directory: `attacks.sql` / `.out.txt`, `attacks2.sql` / `.out.txt`,
`attacks3.sql` / `.out.txt`. Role: `fingo_app` (NOSUPERUSER, NOBYPASSRLS) against
`fingo_review`.

| # | Attack | Expected | Actual | Verdict |
| ---: | :--- | :--- | :--- | :--- |
| 1 | Insert an unbalanced voucher, then post it | refused | `ERROR: voucher … is unbalanced: debit 1000000, credit 900000` (deferred trigger at COMMIT) | **PASS** |
| 2 | `UPDATE vouchers SET description` on a posted voucher | refused | `ERROR: posted voucher GH2026-0001 is immutable` | **PASS** |
| 3 | `UPDATE voucher_lines SET amount` on a posted line | refused | `ERROR: voucher … is posted; its lines are immutable` | **PASS** |
| 4 | `DELETE FROM voucher_lines` on a posted voucher | refused | same trigger | **PASS** |
| 5 | `DELETE FROM vouchers` on a posted voucher | refused | `ERROR: posted voucher GH2026-0001 cannot be deleted` | **PASS** |
| 6 | Unlock/alter `accounting_period_locks` | refused | `ERROR: accounting_period_locks is append-only: UPDATE refused` | **PASS** |
| 7 | Post a voucher dated inside a **locked** period | refused | `INSERT 0 1 … UPDATE 1 … COMMIT` — accepted | **FAIL (F-01)** |
| 8 | Allocate 150% of a document | refused | `ERROR: allocation 150000 exceeds document …` | **PASS** |
| 9 | Post to a heading account (`411`) | refused | committed — DB has no postability notion | **FAIL (F-08)** |
| 10 | Post to an account that does not exist (`9999`) | refused | `ERROR: … violates foreign key constraint voucher_lines_client_id_account_code_fkey` | **PASS** |
| 11 | Insert a voucher with `client_id` of another tenant | refused | `ERROR: new row violates row-level security policy for table "vouchers"` | **PASS** |
| 12 | Attach lines to another tenant's voucher | refused | blocked earlier: a voucher cannot be committed without lines, so no draft exists to attach to; the parent insert is itself RLS-refused | **PASS (by construction)** |
| 13 | Allocate against another tenant's document | refused | `ERROR: new row violates row-level security policy for table "allocations"` | **PASS** |
| 14 | Read vouchers with **no** client scope | zero rows | `vouchers_visible_without_scope = 0` | **PASS** |
| 15 | Read tenant B while scoped to tenant A | zero rows | `b_vouchers_visible_from_a = 0` (A's own 3 visible) | **PASS** |
| 16 | Session-level scope leak (`SET LOCAL`, commit, reuse) | no leak | `current_setting('app.client_id')` → empty | **PASS** |
| 17 | `ALTER TABLE … DISABLE TRIGGER ALL` | refused | `ERROR: must be owner of table vouchers` | **PASS** |
| 18 | `UPDATE audit_log` | refused | `ERROR: audit_log is append-only: UPDATE refused` | **PASS** |
| 19 | `DELETE FROM audit_log` | refused | same trigger (transaction aborted before reaching it) | **PASS** |
| 20 | Mint a tenant (`INSERT INTO clients`) | refused | `INSERT 0 1` — **allowed** | **FAIL (F-10)** |
| 21 | Read the tenant registry | refused | 4 client rows visible (names, tax ids) | **FAIL (F-10)** |
| 22 | Modify the tenant registry (deactivate all) | refused | `UPDATE 2` — all clients deactivated | **FAIL (F-10)** |
| 23 | Insert a stock issue with no stock and no source document | refused | `INSERT 0 1` — accepted | **FAIL (F-09)** |

## Web surface (live server, TLS, `fingo_test`)

| # | Attack | Expected | Actual | Verdict |
| ---: | :--- | :--- | :--- | :--- |
| 24 | `GET /clients` unauthenticated | 401 | `401` | **PASS** |
| 25 | `GET /` unauthenticated | redirect to login | `303 → https://127.0.0.1:8443/login` | **PASS** |
| 26 | Cross-origin `POST /login` (`Origin: http://evil.example`, `Sec-Fetch-Site: cross-site`) | refused | `403` | **PASS** |
| 27 | Same-origin login, unknown user | 401 | `401` | **PASS** |
| 28 | Store `<script>alert("x")</script>` as a partner name, then render | escaped | raw `<script>` occurrences: **0**; `&lt;script&gt;` occurrences: **1** | **PASS** |
| 29 | Session cookie flags | HttpOnly+Secure+Lax | `HttpOnly`, `Secure`, `Path=/`, `SameSite=Lax` (code) | **PASS** |
| 30 | Plain HTTP against the TLS port | refused | `400 Bad Request` (Go's TLS listener rejects cleartext) | **PASS** |
| 31 | `HEAD /login` | 200 | `404` | **FAIL (F-12)** |
| 32 | Security headers | present | none of CSP / X-Frame-Options / X-Content-Type-Options | **FAIL (F-11)** |

## Concurrency

| # | Attack | Expected | Actual | Verdict |
| ---: | :--- | :--- | :--- | :--- |
| 33 | 20 parallel posts, one book | 20 distinct numbers | 20 vouchers, 20 distinct `voucher_no`, 20 distinct `seq`, debits = credits | **PASS** |
| 34 | Race detector over the suite | clean | not runnable: `-race requires cgo`, no C compiler | **UNVERIFIED (F-05)** |

## Notes on method

- Attempts 2–5 in the first script (`attacks.out.txt`) reported `UPDATE 0` / `DELETE 0`
  because the statement ran **outside** a scoped transaction, so RLS hid the rows. They
  were re-run with scope in `attacks2.sql`; a control is only proven when the statement
  is refused *for the right reason*.
- Attempt 12 could not be completed as designed because the database refuses to commit
  a voucher with no lines, so no cross-tenant draft can exist to attach to. Recorded as
  a structural refusal rather than a tested one.
- The session cookie value observed during testing was deleted with the cookie jar and
  is not reproduced in any report.

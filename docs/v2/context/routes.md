# routes — full HTTP surface

> Generated from `cmd/fingo/main.go:65-109`. Regenerate with:
> `Select-String -Path cmd/fingo/main.go -Pattern 'router\.|protected\.'`
> Last generated: 2026-10-18 at `986e572`.

41 routes. `public` = registered outside `RequireUser`. `user` = any signed-in
role. `writer` = `admin` + `accountant`. `admin` = `admin` only. All paths are
relative to `GET /` on the TLS listener.

| # | Method | Path | Handler | Access |
| ---: | :--- | :--- | :--- | :--- |
| 1 | GET | `/healthz` | inline `"ok"` | public |
| 2 | POST | `/login` | `Auth.Login` | public |
| 3 | POST | `/logout` | `Auth.Logout` | public |
| 4 | GET | `/books` | inline stub `"books"` | user |
| 5 | GET | `/clients` | `Auth.ShowClients` | user |
| 6 | POST | `/clients` | `Auth.PostClient` | **admin** |
| 7 | POST | `/clients/:id/select` | `Auth.SelectClient` | user |
| 8 | GET | `/users` | `Auth.ShowUsers` | **admin** |
| 9 | POST | `/users` | `Auth.PostUser` | **admin** |
| 10 | POST | `/password` | `Auth.PostPassword` | user |
| 11 | GET | `/accounts` | `Auth.ShowAccounts` | user |
| 12 | GET | `/reverse` | `Auth.ShowReverse` | user |
| 13 | POST | `/reverse` | `Auth.PostReverse` | writer |
| 14 | GET | `/journal` | `Auth.ShowJournal` | user |
| 15 | GET | `/stock` | `Auth.ShowStock` | user |
| 16 | POST | `/stock/item` | `Auth.PostItem` | writer |
| 17 | POST | `/stock/receive` | `Auth.PostStockReceipt` | writer |
| 18 | POST | `/stock/count` | `Auth.PostStocktake` | writer |
| 19 | GET | `/payment` | `Auth.ShowPayments` | user |
| 20 | POST | `/payment` | `Auth.PostPayment` | writer |
| 21 | GET | `/purchase` | `Auth.ShowPurchase` | user |
| 22 | POST | `/purchase` | `Auth.PostPurchase` | writer |
| 23 | GET | `/handover` | `Auth.ExportHandover` | user |
| 24 | GET | `/opening` | `Auth.ShowOpening` | user |
| 25 | POST | `/opening` | `Auth.PostOpening` | writer |
| 26 | GET | `/receipt` | `Auth.ShowReceipts` | user |
| 27 | POST | `/receipt` | `Auth.PostReceipt` | writer |
| 28 | GET | `/sales` | `Auth.ShowSales` | user |
| 29 | POST | `/sales` | `Auth.PostSales` | writer |
| 30 | GET | `/partners` | `Auth.ShowPartners` | user |
| 31 | POST | `/partners` | `Auth.PostPartner` | writer |
| 32 | GET | `/close` | `Auth.ShowClose` | user |
| 33 | POST | `/close` | `Auth.PostClose` | **admin** |
| 34 | GET | `/aging` | `Auth.ShowAging` | user |
| 35 | GET | `/reports` | `Auth.ShowTrialBalance` | user |
| 36 | GET | `/reports/pnl` | `Auth.ShowProfitAndLoss` | user |
| 37 | GET | `/reports/balance-sheet` | `Auth.ShowBalanceSheet` | user |
| 38 | GET | `/reports/export.xlsx` | `Auth.ExportTrialBalance` | user |
| 39 | GET | `/ledger` | `Auth.ShowLedger` | user |
| 40 | GET | `/entry` | `Auth.ShowEntry` | user |
| 41 | POST | `/entry` | `Auth.PostEntry` | writer |

Static: `GET /static/*` → `web.Static()` (`http.FS`), registered outside the
auth group (`main.go:109`).

## Notes

- 19 of the GET routes render HTML; `/handover` and `/reports/export.xlsx` stream
  files; `/books` is a stub.
- There is **no** `GET /` and **no** login page, so a browser cannot begin a
  session through the UI today.
- `POST /logout` is public but harmless: it destroys the existing session.
- Write routes not covered by `RequireWriter`: `POST /clients`, `POST /users`,
  `POST /close` (admin), `POST /clients/:id/select`, `POST /password` (self).

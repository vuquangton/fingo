# UI prototypes

**Throwaway.** Not production code, not wired into the app, no tests. These exist
to answer three design questions before anybody builds the screens for real.

## How to open them

Double-click any `.html` file — it opens on variant **A**. Then flip with the
floating bar at the bottom, or the ← → arrow keys (they are ignored while a text
field has focus).

To land on a variant directly, put it in the address bar:

```
file:///D:/accounting/fingo/web/prototype/dashboard.html?variant=B
```

**Not** `start dashboard.html?variant=B` or `Start-Process 'file:///…?variant=B'`
— Windows' file-URL launcher drops the query, so you silently get A. That cost
me one misleading screenshot; hence the warning.

No server and no build. `?variant=A|B|C` picks the variant, and the file is
self-contained apart from the two shared files next to it.

They load `../static/app.css`, the app's own stylesheet, so a variant is judged
at the app's real density and palette rather than on a blank page. The header is
the app's real markup but its links are inert: there is nothing behind `file://`.

## What each one is asking

| File | Question | Variants |
| :--- | :--- | :--- |
| `dashboard.html` | What does the accountant see when they open a client's books? | A Numbers first · B Work queue first · C Ledger first |
| `clients.html` | How does the operator move between N clients? | A Table · B Cards · C Search palette |
| `opening-balances.html` | How does a new client's opening position get entered? | A Trial-balance grid · B Guided steps · C Import and check |

The variants disagree about **structure**, not colour. If two of them look like
the same page with different spacing, that is a bug in the prototype.

## Why these three

`docs/requirements-v1.md` lists 18 business events. This set is the intersection
of *not built* and *blocked on a design decision rather than on backend work*:

- **Opening balances** is row 1 of the requirements — "profile + opening balance
  sheet; opening balances → COA; must balance" — and has no screen at all.
- **Client switching** is `internal/client`, which is built, tested and wired to
  nothing. Requirements 5.5 names it a first-class function: "switch client in
  <1 s with 50 clients". Fifty is the number to hold in your head while flipping.
- **The dashboard** exists, so this is the one sub-shape-A set: same route,
  different rendering. It is here because "what should the front door be" is
  still genuinely open — the current one answers *where do the books stand*, and
  an accountant opening a client at 9am is usually asking *what should I do
  next*.

## What is deliberately not here

The other 15 events — cash purchase, credit purchase, landed cost, purchase
return, cash sale, sales return, supplier payment, warehouse transfer, stocktake,
expense, payroll, fixed asset, period close, provisions, VAT preparation. Most of
those are **already decided by the accounting** (the requirements table says
which accounts each one posts to), so they are backend work with a form on top,
not open design questions. Prototyping them would produce fifteen forms nobody
needs to argue about.

If one of them turns out to be contentious, prototype that one on its own.

## When a variant wins

Fold the winner into the real screen, then move the variants and `switcher.js`
onto a throwaway branch — not into `main`, where they rot and confuse the next
reader. Capture the verdict (which variant and why) somewhere durable.

## Not covered, and worth knowing

- Variants are **read-only**. Nothing here mutates, and nothing hits a database.
- The numbers are plausible Vietnamese SME figures, not real books.
- No loading, empty or error states. Each of these screens needs them, and none
  of the three variants shows what it looks like when the query fails.

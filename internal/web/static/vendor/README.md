# Vendored assets

Two files, both served from the same origin as the application. No CDN, no
network access at run time, no build step that fetches them.

| File | Project | Version | Licence | Licence text |
| :--- | :--- | :--- | :--- | :--- |
| `tabler.min.css` | [Tabler](https://tabler.io) | 1.0.0-beta21 | MIT | [`LICENSE.tabler`](LICENSE.tabler) |
| `htmx.min.js` | [htmx](https://htmx.org) | 2.0.4 | BSD-2-Clause | [`LICENSE.htmx`](LICENSE.htmx) |

## Why the licence files are here

These two files are redistributed inside the product, in the same way the Go
modules are linked into the binary. Both licences require the copyright notice to
travel with the copy, so it travels next to the file rather than in a document
nobody ships.

The Go modules' licences are carried by their own source distributions and are
recorded in `docs/v2/decisions.md`.

## htmx is currently inert

`htmx.min.js` is loaded by the pages but no `hx-*` attribute uses it yet. It is
kept because the intent — partial updates without a JavaScript framework — is
still the plan, and removing and re-adding it later is churn.

## Adding an asset

1. Fetch the exact version, not "latest".
2. Commit the file, its upstream `LICENSE`, and a row in the table above.
3. If the licence is not MIT/BSD/Apache-2.0, stop and record the decision first.

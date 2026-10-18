# The connection is configured in the application, and saved settings beat `.env`

**Status:** accepted · 2026-10-09

## The decision

**A first-run screen collects the connection**, and it is what the Wails window
shows whenever no books are open.

**It is structured fields, not a connection string**: host, port, user, password,
database, parameters. `appconfig.Connection` holds `conn.Config` plus the
database name, and the DSN is built by `conn.Config.DSN` through the driver's own
formatter.

**It is saved to `%APPDATA%\FinGo\config.json`.** Not beside the executable, and
not beside the working directory.

**Saved settings win over a `.env`.** A `.env` is a developer's file that happens
to be beside the binary; the screen wrote the other one deliberately. The source
in use is shown on the screen, because two sources is one too many to leave
implicit.

**A settings file that exists and cannot be read is an error, not a fallback.**
Somebody meant something by it. Silently using a different database instead is
the one failure an accounting application must not have.

**The password is stored in plain text**, deliberately and said out loud in the
UI and in `appconfig`'s package doc. On Windows `os.WriteFile` does not set an
ACL — the file inherits `%APPDATA%\<user>`'s, so it is readable by that account
and by administrators of the machine. That is a reasonable trade for a
single-operator application on a machine the operator controls, and not for a
shared machine or a hosted database.

**`Test connection` changes nothing.** It does not create the database, migrate
it or seed the chart. `Connect` does those, and saves **only after** the books
have opened — so what is written down is known to work.

## Why

**The step it replaces was the weakest part of the install.** Before this, an
operator had to hand-edit a text file beside an `.exe` and restart, and the
application's response to getting it wrong was to say so and stop. The window
already had the channel for it — `Status{Ready, Error}`, rendered as a red panel
— and nothing to put behind it.

**A connection string is the wrong unit.** `conn.Config` is deliberately not a
DSN, and `DSN(database)` builds the string through `mysql.Config`, so a password
containing `@`, `:`, `/` or `\` survives the driver's own parser. A free-text DSN
box bypasses that, and its failures arrive as driver errors rather than as "the
host is wrong". The database name is also separate on purpose: one database per
client, so it is not part of the server's description.

**Writing `.env` was the obvious cheap option and is wrong.** An installed
application sits where a standard user cannot write, and `.env` is a developer
artifact. It also cannot be per-user, which is what the settings file is.

**Saving after opening is what makes the screen safe to press.** A settings file
that does not open is an installation that fails at every start and cannot be
talked out of it from the screen that wrote it.

## Considered options

**1. Keep the `.env`, document it better.** Rejected. It is the step that fails
for a non-developer, and documentation does not survive the handover.

**2. A single connection-string field.** Rejected for the escaping reason above.
An "advanced: paste a DSN" box remains possible, but it would have to *parse
into* `conn.Config`, not replace it.

**3. Write the `.env` beside the executable.** Rejected — unwritable once
properly installed, and not per-user.

**4. DPAPI for the password.** Deferred, not rejected. It is the right answer the
day this is deployed where the profile directory is not trusted. Nothing here
commits to plaintext structurally: the password is one field in one file, and
encrypting it changes `appconfig.Store` and nothing else.

## Consequences

**`DB_DRIVER` is dead.** It sits in the local `.env` (`DB_DRIVER=mysql`) and is
read by no line of Go, JavaScript or TypeScript — the driver is imported and
named in `internal/onboarding` and nowhere configurable. It is **not** part of
the settings file, and it has been left in the `.env` rather than removed,
because that file is a developer's and deleting a key from it is not this
decision's business. It should not be carried into anything new.

**`conn.Config` gained JSON tags.** It is what the saved connection is made of,
and that file is meant to be readable and editable by the person who owns the
machine; Go's default field names would have written `"Password"` with no
indication of what it is a password to.

**`App.fail` no longer replaces the whole status.** It keeps `Source`,
`ConfigPath` and `EnvPath`, because the connection screen has to say where the
settings live and which source is in use *especially* when opening them failed.

**The database is created when it is missing**, so a fresh install does not need
a SQL client. `EnsureDatabase` skips the `CREATE` when the database already
exists, so an operator whose login may use a database but not create one is not
refused by the step that was only clearing the way.

### Not decided here

- **More than one client.** `internal/client` is the registry of which clients
  exist and where each database lives, and this file is that registry's first row
  living on its own. When a second client arrives, this screen becomes "add a
  client" and the file becomes a list.
- **Creating the user, granting privileges, or choosing a server.** The screen
  reaches a server that is already installed and a login that already works.
- **A stylesheet for the Wails frontend.** None of `.shell`, `.panel`, `.field`,
  `.hint`, `.meta` or the new `.row` is defined in `style.css`, which is still
  the default Wails template. The existing screens are unstyled too; the
  connection screen matches them rather than being the one that got styled.

### Cost

Two packages, four bound methods, one screen. No existing signature changed.

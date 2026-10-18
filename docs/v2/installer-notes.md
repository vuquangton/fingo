# FinGo v2 — installer and operations notes

For whoever installs and runs the server. The target is one office machine (or one small VM)
running the Go binary and PostgreSQL 16. There is no Docker, no Node, no CDN.

---

## 1. What must be on the machine

| Component | Version | Why |
| :--- | :--- | :--- |
| Windows (or Linux) x64 | Windows 10+ / any current distro | the binary is built for the platform |
| PostgreSQL | **16.x** | schema, row-level security, `COPY`, advisory locks |
| FinGo binary | built from `v2` | `go build ./cmd/fingo` |
| TLS certificate + key | any | the server **refuses to start without one**; a Secure cookie over plain HTTP is a login that silently fails |

Build:

```powershell
cd D:\accounting
go build -o dist\fingo.exe .\cmd\fingo
```

Development certificate (never for production): `go run ./cmd/devcert -dir .local/tls`.

## 2. Prepare the database (once)

```powershell
$env:PGADMIN_PASSWORD = '<postgres password>'   # read from the environment, never stored
.\scripts\pgsetup.cmd                            # creates fingo_dev + fingo_test + fingo_app role
```

That writes a **gitignored** `.env` with the application role's credentials. The application
role is `NOSUPERUSER NOBYPASSRLS` on purpose: a superuser bypasses row-level security, and a
tenant boundary that can be bypassed is not a boundary.

Apply the schema:

```powershell
$env:PGADMIN_PASSWORD = '<postgres password>'
go run .\cmd\migrate -database fingo_dev
```

The migration creates the schema, the triggers, the row-level security policies, and seeds the
TT133 chart (123 accounts) plus one development client. It is safe to rerun; goose applies only
what is pending.

## 3. Run the server

```powershell
$env:FINGO_DSN  = 'host=127.0.0.1 port=5432 user=fingo_app password=<...> dbname=fingo_dev sslmode=disable'
$env:FINGO_ADDR = '127.0.0.1:8443'
.\dist\fingo.exe -cert <cert.pem> -key <key.pem>
```

The process writes JSON logs to stdout and shuts down gracefully on Ctrl-C / SIGTERM
(10-second drain). Health check: `GET /healthz` → `ok`.

> **The DSN belongs in the environment, not in a unit file or a script in the repository.**
> A unit file is world-readable and appears in `systemctl cat`.

### Linux (systemd)

`deploy/fingo.service` is provided. It reads `EnvironmentFile=/etc/fingo/fingo.env`, runs as the
`fingo` user, and is hardened (`ProtectSystem=strict`, `NoNewPrivileges`, no capabilities).
Installation steps are in the file's header. **It has not been exercised on a real Linux host in
this repository** — treat it as reviewed, not proven.

### Windows

No SCM service wrapper is implemented. `sc.exe create` with a plain executable does **not**
work — the binary must call `svc.Run`. Until that wrapper exists, run the server from a
scheduled task ("at startup", run whether user is logged on or not) or under NSSM.

## 4. Backups

```powershell
$env:PGADMIN_PASSWORD = '<postgres password>'
.\scripts\backup.ps1            # pg_dump -Fc, verified with pg_restore --list, 30-day retention
.\scripts\restore-drill.ps1     # restore into a scratch database and compare figures
```

Register it with the OS scheduler, so it does not depend on a human remembering:

```powershell
.\scripts\install-backup-task.ps1              # daily 01:00, runs as the current user
.\scripts\install-backup-task.ps1 -Remove      # unregister
```

The task holds no password on its command line, because a scheduled task's command line is
readable by every process on the machine. `backup.ps1` instead reads the owner password from
the standard libpq password file, created once by that user:

```
%APPDATA%\postgresql\pgpass.conf
host:port:database:user:password
```

Run the **drill** monthly and after any upgrade: a
backup that has not been restored is a file, not a backup. Record the result in
`docs/v2/restore-drill.md`.

## 5. Upgrade

1. Take a backup (`scripts/backup.ps1`).
2. Stop the service.
3. Replace the binary; `go run ./cmd/migrate -database <db>` (as owner).
4. Start the service; check `/healthz` and one report screen.
5. Keep the previous binary until a report has been produced from the new one.

Migrations are forward-only in practice: `goose` can roll back, but a rollback that drops a
table drops the client's rows with it. Restore from the backup instead.

## 6. What must never be done

| Never | Why |
| :--- | :--- |
| Run the application as the database owner | the owner is filtered by `FORCE ROW LEVEL SECURITY`, so writes silently fail or migrations bypass the tenant rule |
| Put the DSN or the certificate key in git | both are credentials; `.env` and `.local/` are gitignored for this reason |
| Delete rows from `vouchers`, `voucher_lines`, `allocations`, `stock_movements`, `accounting_period_locks`, `audit_log` | the database refuses; doing it with triggers disabled destroys the audit trail |
| Point two installations at one data directory | one writes over the other's books |
| Use the development certificate in production | it is trusted by nothing |

## 7. Current operational gaps

| Gap | Consequence | Next step |
| :--- | :--- | :--- |
| No Windows SCM wrapper | server starts from a scheduled task, not as a service | `svc.Run` wrapper |
| No installer / MSI | copying the folder is the install | packaging |
| Backup schedule is a Windows task only | a Linux host needs a cron entry; the unit file does not run the dump | add a systemd timer |
| One database for all clients | a restore restores everybody | per-client export exists (`/handover`) |
| No TLS certificate rotation | expiry is silent | calendar reminder, or renewal job |

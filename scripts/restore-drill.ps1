# Proves a backup restores: dump → scratch database → verify → drop.
#
#   $env:PGADMIN_PASSWORD = '<postgres password>'
#   .\scripts\restore-drill.ps1
#
# The drill is the point of a backup. It restores into a database that did not
# exist, checks that the figures survived (account count, voucher count, and the
# ledger's debit = credit invariant), then drops the scratch database. A green
# drill is the only evidence that the backup is restorable.
#
# Exit code 0 means the restored copy matched. Anything else means it did not.

[CmdletBinding()]
param(
    [string] $SourceDatabase = 'fingo_dev',
    [string] $DrillDatabase  = 'fingo_restore_drill',
    [string] $Dump           = '',
    [string] $PgBin          = ''
)

$ErrorActionPreference = 'Stop'
$root = Split-Path -Parent $PSScriptRoot

if (-not $PgBin) {
    $PgBin = if ($env:FINGO_PG_BIN) { $env:FINGO_PG_BIN } else {
        Get-ChildItem 'C:\Program Files\PostgreSQL' -Directory -ErrorAction SilentlyContinue |
            Sort-Object Name -Descending |
            ForEach-Object { Join-Path $_.FullName 'bin' } |
            Where-Object { Test-Path (Join-Path $_ 'pg_restore.exe') } |
            Select-Object -First 1
    }
    if (-not $PgBin) { throw 'PostgreSQL client tools not found. Set FINGO_PG_BIN.' }
}

if (-not $env:PGADMIN_PASSWORD) { throw 'Set PGADMIN_PASSWORD (the owner password) before running.' }

$user = if ($env:PGADMIN_USER) { $env:PGADMIN_USER } else { 'postgres' }
$hostName = if ($env:PGHOST) { $env:PGHOST } else { '127.0.0.1' }
$port = if ($env:PGPORT) { $env:PGPORT } else { '5432' }

if (-not $Dump) {
    $latest = Get-ChildItem (Join-Path $root 'backups') -Filter "fingo-$SourceDatabase-*.dump" -ErrorAction SilentlyContinue |
        Sort-Object LastWriteTime -Descending | Select-Object -First 1
    if (-not $latest) {
        throw "No dump for $SourceDatabase. Run .\scripts\backup.ps1 first."
    }
    $Dump = $latest.FullName
}

$env:PGPASSWORD = $env:PGADMIN_PASSWORD
$psql = Join-Path $PgBin 'psql.exe'
$createdb = Join-Path $PgBin 'createdb.exe'
$dropdb = Join-Path $PgBin 'dropdb.exe'
$pgrestore = Join-Path $PgBin 'pg_restore.exe'

Write-Host "drill: $Dump → $DrillDatabase"

& $dropdb --host $hostName --port $port --username $user --if-exists --force $DrillDatabase
& $createdb --host $hostName --port $port --username $user --encoding UTF8 --template template0 $DrillDatabase
if ($LASTEXITCODE -ne 0) { throw "createdb $DrillDatabase failed" }

try {
    & $pgrestore --host $hostName --port $port --username $user --dbname $DrillDatabase --no-owner --no-privileges $Dump
    # pg_restore exits 1 on warnings (grants to roles that do not exist). The
    # drill's own assertions below are what decide success, so a warning is
    # reported and not fatal here.
    if ($LASTEXITCODE -ne 0) { Write-Warning "pg_restore reported $LASTEXITCODE; verifying anyway" }

    $query = @"
SELECT
  (SELECT count(*) FROM accounts)  || '|' ||
  (SELECT count(*) FROM vouchers)  || '|' ||
  (SELECT COALESCE(sum(CASE WHEN side = 'DEBIT' THEN amount ELSE -amount END), 0) FROM voucher_lines)
"@

    $before = (& $psql --host $hostName --port $port --username $user -d $SourceDatabase -tAc $query).Trim()
    $after  = (& $psql --host $hostName --port $port --username $user -d $DrillDatabase  -tAc $query).Trim()

    Write-Host "source : $before"
    Write-Host "restore: $after"

    if ($before -ne $after) { throw 'the restored figures do not match the source' }

    if ($after.Split('|')[2] -ne '0') { throw 'the restored ledger does not balance (debit ≠ credit)' }

    Write-Host 'drill: PASS — counts match and the restored ledger balances'
} finally {
    & $dropdb --host $hostName --port $port --username $user --if-exists --force $DrillDatabase
    $env:PGPASSWORD = $null
}

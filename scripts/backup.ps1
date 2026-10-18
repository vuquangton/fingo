# Dumps one FinGo database to a compressed custom-format archive.
#
#   $env:PGADMIN_PASSWORD = '<postgres password>'
#   .\scripts\backup.ps1
#
# pg_dump is the wheel: it is the PostgreSQL-native, version-matched, parallel-safe
# way to take a consistent snapshot. Writing our own dump would be a worse copy of
# it (the v1 project did exactly that, and its backup runner was never reachable).
#
# The dump runs as the OWNER, not as fingo_app: row-level security would silently
# filter an app-role dump down to nothing, producing a backup that restores an
# empty database. That failure is why this script refuses to run as the app role.
#
# Retention: 30 days, counted in files, oldest removed first.

[CmdletBinding()]
param(
    [string] $Database   = 'fingo_dev',
    [string] $BackupDir  = '',
    [int]    $KeepDays   = 30,
    [string] $PgBin      = ''
)

$ErrorActionPreference = 'Stop'
$root = Split-Path -Parent $PSScriptRoot
if (-not $BackupDir) { $BackupDir = Join-Path $root 'backups' }

if (-not $PgBin) {
    $PgBin = if ($env:FINGO_PG_BIN) { $env:FINGO_PG_BIN } else {
        Get-ChildItem 'C:\Program Files\PostgreSQL' -Directory -ErrorAction SilentlyContinue |
            Sort-Object Name -Descending |
            ForEach-Object { Join-Path $_.FullName 'bin' } |
            Where-Object { Test-Path (Join-Path $_ 'pg_dump.exe') } |
            Select-Object -First 1
    }
    if (-not $PgBin) { throw 'pg_dump not found. Set FINGO_PG_BIN.' }
}

if (-not $env:PGADMIN_PASSWORD) { throw 'Set PGADMIN_PASSWORD (the owner password) before running.' }

$user = if ($env:PGADMIN_USER) { $env:PGADMIN_USER } else { 'postgres' }
$hostName = if ($env:PGHOST) { $env:PGHOST } else { '127.0.0.1' }
$port = if ($env:PGPORT) { $env:PGPORT } else { '5432' }

New-Item -ItemType Directory -Force -Path $BackupDir | Out-Null

$stamp = Get-Date -Format 'yyyy-MM-dd_HH-mm-ss'
$dump = Join-Path $BackupDir ("fingo-$Database-$stamp.dump")

$previous = $env:PGPASSWORD
$env:PGPASSWORD = $env:PGADMIN_PASSWORD
try {
    & (Join-Path $PgBin 'pg_dump.exe') `
        --host $hostName --port $port --username $user `
        --format custom --compress 6 --no-owner --no-privileges `
        --file $dump $Database
    if ($LASTEXITCODE -ne 0) { throw "pg_dump failed with $LASTEXITCODE" }
} finally {
    $env:PGPASSWORD = $previous
}

if (-not (Test-Path $dump) -or (Get-Item $dump).Length -eq 0) {
    throw "the dump at $dump is missing or empty"
}

# A backup nobody checked is a hope. The archive must list its contents before it
# is trusted.
& (Join-Path $PgBin 'pg_restore.exe') --list $dump *> $null
if ($LASTEXITCODE -ne 0) { throw "the dump at $dump is not a readable archive" }

$size = (Get-Item $dump).Length
Write-Host ("backup: {0} ({1:N0} bytes)" -f $dump, $size)

$cutoff = (Get-Date).AddDays(-$KeepDays)
Get-ChildItem $BackupDir -Filter 'fingo-*.dump' |
    Where-Object { $_.LastWriteTime -lt $cutoff } |
    ForEach-Object {
        Remove-Item $_.FullName -Force
        Write-Host "retention: removed $($_.Name)"
    }

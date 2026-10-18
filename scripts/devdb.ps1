# Creates and controls a workspace-local PostgreSQL 16 cluster for FinGo v2.
#
# Why not the system service: this machine already runs postgresql-x64-16, but
# its superuser password is not available to a developer session, and editing a
# service's pg_hba.conf is a machine-wide change. A cluster under .local\ is
# reversible (delete the directory), needs no administrator rights, and cannot
# disturb an existing installation.
#
#   .\scripts\devdb.ps1            # create (once) and start
#   .\scripts\devdb.ps1 -Stop      # stop
#   .\scripts\devdb.ps1 -Status    # is it running?
#   .\scripts\devdb.ps1 -Destroy   # stop and delete the data directory
#
# The cluster listens on 127.0.0.1:65432 only, with trust auth, because it holds
# throwaway development data. Tests read PGHOST/PGPORT/PGUSER/PGDATABASE.
#
# Port 65432 is chosen because Windows refuses to bind ports inside its reserved
# ranges (this machine excludes 55302-55501 among others), and the obvious
# 55432 fails with "Permission denied" that looks like a firewall problem.

[CmdletBinding()]
param(
    [switch] $Stop,
    [switch] $Status,
    [switch] $Destroy,
    [int]    $Port = 65432,
    [string] $User = 'fingo',
    [string[]] $Databases = @('fingo_dev', 'fingo_test')
)

$ErrorActionPreference = 'Stop'
$root    = Split-Path -Parent $PSScriptRoot
$dataDir = Join-Path $root '.local\pgdata'
$logFile = Join-Path $root '.local\pg.log'

function Find-PgBin {
    if ($env:FINGO_PG_BIN) {
        if (Test-Path (Join-Path $env:FINGO_PG_BIN 'initdb.exe')) { return $env:FINGO_PG_BIN }
        throw "FINGO_PG_BIN is set but initdb.exe is not there: $env:FINGO_PG_BIN"
    }

    $candidates = Get-ChildItem 'C:\Program Files\PostgreSQL' -Directory -ErrorAction SilentlyContinue |
        Sort-Object Name -Descending |
        ForEach-Object { Join-Path $_.FullName 'bin' }

    foreach ($candidate in $candidates) {
        if (Test-Path (Join-Path $candidate 'initdb.exe')) { return $candidate }
    }

    throw 'PostgreSQL 16 binaries not found. Install PostgreSQL, or set FINGO_PG_BIN.'
}

$bin = Find-PgBin
$pgctl = Join-Path $bin 'pg_ctl.exe'

function Test-Running {
    # A TCP probe, not `pg_ctl status`: status reads postmaster.pid and can block
    # on a stale pid file, which turns a rerun into a hang.
    $client = [System.Net.Sockets.TcpClient]::new()
    try {
        $client.Connect('127.0.0.1', $Port)
        return $client.Connected
    } catch {
        return $false
    } finally {
        $client.Dispose()
    }
}

if ($Status) {
    if (Test-Running) { Write-Host "running: $dataDir on port $Port" } else { Write-Host 'not running' }
    return
}

if ($Stop) {
    if (Test-Running) { & $pgctl -D $dataDir -m fast stop }
    return
}

if ($Destroy) {
    if (Test-Running) { & $pgctl -D $dataDir -m fast stop }
    if (Test-Path $dataDir) { Remove-Item -Recurse -Force $dataDir }
    Write-Host "destroyed $dataDir"
    return
}

if (-not (Test-Path $dataDir)) {
    New-Item -ItemType Directory -Force -Path (Split-Path $dataDir) | Out-Null
    Write-Host "initdb → $dataDir"
    & (Join-Path $bin 'initdb.exe') -D $dataDir -U $User --auth=trust --encoding=UTF8 --locale=C | Out-Null
    if ($LASTEXITCODE -ne 0) { throw 'initdb failed' }

    # 127.0.0.1 only: a development cluster must not be reachable from the LAN.
    Add-Content -Path (Join-Path $dataDir 'postgresql.conf') -Value @"

# --- FinGo v2 development cluster ---
listen_addresses = '127.0.0.1'
port = $Port
max_connections = 50
shared_buffers = 128MB
log_destination = 'stderr'
logging_collector = off
"@
}

if (-not (Test-Running)) {
    Write-Host "starting → 127.0.0.1:$Port (log: $logFile)"
    & $pgctl -D $dataDir -l $logFile -o "-p $Port" -w start
    if ($LASTEXITCODE -ne 0) { throw "pg_ctl start failed; see $logFile" }
}

foreach ($db in $Databases) {
    $exists = & (Join-Path $bin 'psql.exe') -h 127.0.0.1 -p $Port -U $User -d postgres -w -tAc "SELECT 1 FROM pg_database WHERE datname = '$db'"
    if ($exists.Trim() -ne '1') {
        Write-Host "createdb $db"
        & (Join-Path $bin 'createdb.exe') -h 127.0.0.1 -p $Port -U $User -w $db
        if ($LASTEXITCODE -ne 0) { throw "createdb $db failed" }
    }
}

Write-Host "`nready.  PGHOST=127.0.0.1 PGPORT=$Port PGUSER=$User PGDATABASE=fingo_dev"

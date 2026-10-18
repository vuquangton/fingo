# Prepares the system PostgreSQL 16 instance for FinGo v2.
#
#   $env:PGADMIN_PASSWORD = '<postgres superuser password>'
#   .\scripts\pgsetup.cmd
#
# Creates: databases fingo_dev + fingo_test, and a login role fingo_app that is
# NOSUPERUSER NOBYPASSRLS (so row-level security actually applies to it), with a
# generated password. Writes .env (gitignored) with the app credentials.
#
# The admin password is read from the environment and never written to disk,
# never echoed, and never committed. The generated app password goes only into
# the gitignored .env.
#
# Idempotent: rerunning creates nothing that exists and rotates nothing unless
# -Rotate is passed.

[CmdletBinding()]
param(
    [string] $AdminUser = 'postgres',
    [string] $AdminDb   = 'postgres',
    [string] $HostName  = '127.0.0.1',
    [int]    $Port      = 5432,
    [string] $AppUser   = 'fingo_app',
    [string[]] $Databases = @('fingo_dev', 'fingo_test'),
    [switch] $Rotate
)

$ErrorActionPreference = 'Stop'
$root = Split-Path -Parent $PSScriptRoot
$bin  = if ($env:FINGO_PG_BIN) { $env:FINGO_PG_BIN } else {
    $c = Get-ChildItem 'C:\Program Files\PostgreSQL' -Directory -ErrorAction SilentlyContinue |
        Sort-Object Name -Descending |
        ForEach-Object { Join-Path $_.FullName 'bin' } |
        Where-Object { Test-Path (Join-Path $_ 'psql.exe') } |
        Select-Object -First 1
    if (-not $c) { throw 'PostgreSQL client binaries not found. Set FINGO_PG_BIN.' }
    $c
}
$psql = Join-Path $bin 'psql.exe'

if (-not $env:PGADMIN_PASSWORD) {
    throw 'Set PGADMIN_PASSWORD (the postgres superuser password) before running this script.'
}

function Invoke-Sql([string] $sql, [string] $database = $AdminDb) {
    $previous = $env:PGPASSWORD
    $env:PGPASSWORD = $env:PGADMIN_PASSWORD
    try {
        $out = & $psql -h $HostName -p $Port -U $AdminUser -d $database -w -v ON_ERROR_STOP=1 -tAc $sql 2>&1
        if ($LASTEXITCODE -ne 0) { throw "sql failed: $out" }
        return ($out -join "`n")
    } finally {
        $env:PGPASSWORD = $previous
    }
}

# --- databases ---------------------------------------------------------------

foreach ($db in $Databases) {
    $exists = Invoke-Sql "SELECT 1 FROM pg_database WHERE datname = '$db'"
    if (([string]$exists).Trim() -ne '1') {
        Invoke-Sql "CREATE DATABASE $db ENCODING 'UTF8' TEMPLATE template0"
        Write-Host "created database $db"
    } else {
        Write-Host "database $db exists"
    }
}

# --- application role --------------------------------------------------------

$envFile = Join-Path $root '.env'
$appPassword = $null

if ((Test-Path $envFile) -and -not $Rotate) {
    $line = Get-Content $envFile | Where-Object { $_ -match '^PGPASSWORD=' } | Select-Object -First 1
    if ($line) { $appPassword = $line.Substring('PGPASSWORD='.Length) }
}

if (-not $appPassword) {
    $rng = [System.Security.Cryptography.RandomNumberGenerator]::Create()
    try {
        $bytes = New-Object byte[] 24
        $rng.GetBytes($bytes)
    } finally {
        $rng.Dispose()
    }
    $appPassword = -join ($bytes | ForEach-Object { $_.ToString("x2") })
}

$roleExists = Invoke-Sql "SELECT 1 FROM pg_roles WHERE rolname = '$AppUser'"
if (([string]$roleExists).Trim() -ne '1') {
    $sql = "CREATE ROLE $AppUser LOGIN NOSUPERUSER NOBYPASSRLS NOCREATEDB NOCREATEROLE PASSWORD '$appPassword'"
    Invoke-Sql $sql
    Write-Host "created role $AppUser (NOSUPERUSER NOBYPASSRLS)"
} else {
    Invoke-Sql "ALTER ROLE $AppUser PASSWORD '$appPassword'"
    Write-Host "role $AppUser exists; password set from .env"
}

foreach ($db in $Databases) {
    Invoke-Sql "GRANT CONNECT ON DATABASE $db TO $AppUser"
    Invoke-Sql "GRANT USAGE ON SCHEMA public TO $AppUser" $db
    # Tables do not exist yet; the migration grants table privileges to the role.
    Invoke-Sql "ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO $AppUser" $db
    Invoke-Sql "ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT USAGE, SELECT ON SEQUENCES TO $AppUser" $db
}

# --- .env --------------------------------------------------------------------

$lines = @(
    '# FinGo v2 local development database. Gitignored: never commit this file.',
    "PGHOST=$HostName",
    "PGPORT=$Port",
    'PGDATABASE=fingo_dev',
    "PGUSER=$AppUser",
    "PGPASSWORD=$appPassword",
    '# The migration/DDL connection uses the superuser; tests use the app role above',
    '# so that row-level security is actually exercised.',
    "PGADMIN_USER=$AdminUser",
    "PGADMIN_DB=$AdminDb",
    "FINGO_TEST_DATABASE=fingo_test"
)
[System.IO.File]::WriteAllLines($envFile, $lines, (New-Object System.Text.UTF8Encoding($false)))
Write-Host "wrote $envFile (gitignored)"

Write-Host "`nready."
Write-Host "  app role : $AppUser @ ${HostName}:$Port"
Write-Host "  databases: $($Databases -join ', ')"
Write-Host "  next     : `$env:REQUIRE_DB='1'; go test ./..."

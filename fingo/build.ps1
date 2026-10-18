# Builds FinGo, and bundles the database server it runs on. One command.
#
#   build.cmd
#
# The things this script exists to remember:
#
#   1. The manifest must be compiled into the executable BEFORE it is built.
#      Without it walk cannot create a tooltip and window creation fails with
#      "TTM_ADDTOOL failed"; with an invalid one Windows refuses to load the
#      binary with "the application has failed to start because its side-by-side
#      configuration is incorrect". Both are failures at launch, which is the
#      worst place to find out.
#
#   2. -H windowsgui and -s -w. Without windowsgui the application opens a
#      console window behind itself; -s -w strips the symbol table and the DWARF
#      data, which is most of a Go binary's size and none of its behaviour.
#
#   3. The server has to travel with the application. A copy of MariaDB's bin
#      directory is 293.5 MB, and 94.5 MB of that is .pdb debug symbols this
#      ships without — so the bundle is assembled by name rather than copied
#      wholesale, and it is about 36 MB.
#
# The server goes in <out>/server, beside the executable, because it is a
# read-only program file. The data directory is the user's books and lives
# somewhere else entirely.

[CmdletBinding()]
param(
    # Where to put the executable and the server.
    [string] $OutDir = "dist",

    # The MariaDB installation to bundle from. Defaults to the usual places.
    [string] $MariaDbRoot = "",

    # Build the executable only. For a machine with no MariaDB to bundle.
    [switch] $NoBundle
)

$ErrorActionPreference = "Stop"
Set-Location $PSScriptRoot

function Require-Command([string] $Name, [string] $Why) {
    if (-not (Get-Command $Name -ErrorAction SilentlyContinue)) {
        throw "$Name is not on PATH. $Why"
    }
}

function Resolve-MariaDbRoot([string] $Hint) {
    $candidates = @()

    if ($Hint)                    { $candidates += $Hint }
    if ($env:FINGO_MARIADB_BIN)   { $candidates += (Split-Path -Parent $env:FINGO_MARIADB_BIN) }

    $candidates += @(
        "C:\Program Files\MariaDB 12.3",
        "C:\Program Files\MariaDB 11.4",
        "C:\Program Files\MariaDB 10.11"
    )

    foreach ($candidate in $candidates) {
        if ($candidate -and (Test-Path (Join-Path $candidate "bin\mariadbd.exe"))) {
            return (Resolve-Path $candidate).Path
        }
    }

    return $null
}

function Format-Size([long] $Bytes) { "{0:N1} MB" -f ($Bytes / 1MB) }

Require-Command go   "Install Go, or run this from a shell where it is."
Require-Command rsrc "Install it with: go install github.com/akavel/rsrc@latest"

$out = Join-Path $PSScriptRoot $OutDir
New-Item -ItemType Directory -Force -Path $out | Out-Null
$exe = Join-Path $out "FinGo.exe"

Write-Host "1/3  compiling the manifest into cmd/fingo/rsrc.syso" -ForegroundColor Cyan
go generate ./cmd/fingo
if ($LASTEXITCODE -ne 0) { throw "go generate failed" }

Write-Host "2/3  building $exe" -ForegroundColor Cyan
go build -ldflags "-H windowsgui -s -w" -o $exe ./cmd/fingo
if ($LASTEXITCODE -ne 0) { throw "go build failed" }

# --- the server -------------------------------------------------------------

$server = Join-Path $out "server"

if ($NoBundle) {
    Write-Host "3/3  skipping the server (-NoBundle)" -ForegroundColor Yellow
} else {
    $src = Resolve-MariaDbRoot $MariaDbRoot
    if (-not $src) {
        Write-Host "3/3  no MariaDB found to bundle" -ForegroundColor Yellow
        Write-Host "     pass -MariaDbRoot <path>, or set FINGO_MARIADB_BIN, or -NoBundle"
    } else {
        Write-Host "3/3  bundling the server from $src" -ForegroundColor Cyan

        # Rebuilt rather than merged, so a server removed from the source does
        # not survive in the output.
        Remove-Item $server -Recurse -Force -ErrorAction SilentlyContinue
        foreach ($d in "bin", "lib\plugin", "share\english", "share\charsets") {
            New-Item -ItemType Directory -Force -Path (Join-Path $server $d) | Out-Null
        }

        # mariadbd.exe and mysqld.exe are both 13 KB launchers — the server is
        # bin\server.dll. BOTH launchers have to ship, and that is not obvious:
        # the application starts mariadbd.exe, but mariadb-install-db.exe runs
        # the bootstrap by invoking mysqld.exe by name, and without it the first
        # run dies with "Can't write to mysqld's stdin" — which names the wrong
        # problem entirely.
        foreach ($f in "mariadbd.exe", "mysqld.exe", "mariadb-admin.exe", "mariadb-install-db.exe") {
            Copy-Item (Join-Path $src "bin\$f") (Join-Path $server "bin") -Force
        }

        # server.dll's dependencies: the Visual C++ runtime, zlib, libcurl. *.dll
        # and not *.* — the .pdb files beside them are 94.5 MB of symbols.
        Copy-Item (Join-Path $src "bin\*.dll") (Join-Path $server "bin") -Force

        # The server finds share/ and lib/ relative to its own location, so the
        # layout under $server has to mirror the installation's.
        Copy-Item (Join-Path $src "lib\plugin\*")   (Join-Path $server "lib\plugin")   -Recurse -Force
        Copy-Item (Join-Path $src "share\english\*") (Join-Path $server "share\english") -Recurse -Force
        Copy-Item (Join-Path $src "share\charsets\*") (Join-Path $server "share\charsets") -Recurse -Force

        $size = (Get-ChildItem $server -Recurse -File | Measure-Object Length -Sum).Sum
        $count = (Get-ChildItem $server -Recurse -File | Measure-Object).Count
        Write-Host ("     {0} files, {1}" -f $count, (Format-Size $size)) -ForegroundColor Green
    }
}

Write-Host ("`nbuilt {0}  ({1})" -f $exe, (Format-Size (Get-Item $exe).Length)) -ForegroundColor Green

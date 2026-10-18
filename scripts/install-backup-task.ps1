# Registers the nightly backup as a Windows scheduled task.
#
#   .\scripts\install-backup-task.ps1                 # daily at 01:00
#   .\scripts\install-backup-task.ps1 -Time 02:30
#   .\scripts\install-backup-task.ps1 -Remove
#
# The task runs as the current user and needs no password on its command line:
# backup.ps1 reads the database password from the standard libpq password file.
# Create it once, as that user:
#
#   %APPDATA%\postgresql\pgpass.conf
#   host:port:database:user:password
#
# A human still has to run the restore drill. A scheduled backup proves only
# that a file appeared.

[CmdletBinding()]
param(
    [string] $Time = '01:00',
    [string] $TaskName = 'FinGo backup',
    [switch] $Remove
)

$ErrorActionPreference = 'Stop'
$root = Split-Path -Parent $PSScriptRoot
$backup = Join-Path $root 'scripts\backup.ps1'

if ($Remove) {
    Unregister-ScheduledTask -TaskName $TaskName -Confirm:$false -ErrorAction SilentlyContinue
    Write-Host "removed task: $TaskName"
    return
}

if (-not (Test-Path $backup)) { throw "backup.ps1 not found at $backup" }

$pgpass = Join-Path $env:APPDATA 'postgresql\pgpass.conf'
if (-not (Test-Path $pgpass)) {
    Write-Warning "no pgpass file at $pgpass - the task will fail until it exists."
}

$arguments = '-NoProfile -ExecutionPolicy Bypass -File "{0}"' -f $backup
$action = New-ScheduledTaskAction -Execute 'powershell.exe' -Argument $arguments

$trigger = New-ScheduledTaskTrigger -Daily -At $Time

$settings = New-ScheduledTaskSettingsSet -StartWhenAvailable `
    -DontStopOnIdleEnd -ExecutionTimeLimit (New-TimeSpan -Hours 1)

Register-ScheduledTask -TaskName $TaskName -Action $action -Trigger $trigger `
    -Settings $settings -Description 'FinGo nightly pg_dump backup' -Force | Out-Null

Write-Host "registered '$TaskName' daily at $Time"
Write-Host "verify with: Get-ScheduledTaskInfo -TaskName '$TaskName'"
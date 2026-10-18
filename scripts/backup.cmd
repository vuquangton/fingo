@echo off
rem Runs backup.ps1 without changing the machine's execution policy.
powershell -NoProfile -ExecutionPolicy Bypass -File "%~dp0backup.ps1" %*
exit /b %ERRORLEVEL%

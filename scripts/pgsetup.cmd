@echo off
rem Runs pgsetup.ps1 without changing the machine's execution policy.
powershell -NoProfile -ExecutionPolicy Bypass -File "%~dp0pgsetup.ps1" %*
exit /b %ERRORLEVEL%

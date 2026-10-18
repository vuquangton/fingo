@echo off
rem Runs restore-drill.ps1 without changing the machine's execution policy.
powershell -NoProfile -ExecutionPolicy Bypass -File "%~dp0restore-drill.ps1" %*
exit /b %ERRORLEVEL%

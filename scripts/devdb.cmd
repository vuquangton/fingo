@echo off
rem Runs devdb.ps1 without changing the machine's execution policy.
rem A .cmd wrapper exists because a .ps1 cannot be run on a default Windows
rem machine: "running scripts is disabled on this system" is a wall in front of
rem the first thing anybody tries.
powershell -NoProfile -ExecutionPolicy Bypass -File "%~dp0devdb.ps1" %*
exit /b %ERRORLEVEL%

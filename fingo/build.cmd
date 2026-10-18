@echo off
rem Builds FinGo. One command, no configuration, no execution-policy change.
rem
rem A .cmd wrapper exists because a .ps1 cannot be run on a default Windows
rem machine: the execution policy refuses it with "running scripts is disabled on
rem this system", which is a wall in front of the very first thing anybody tries.
rem Bypassing the policy for this one invocation is honest — the script is in the
rem repository and came with the code — and it does not change anything on the
rem machine.
powershell -NoProfile -ExecutionPolicy Bypass -File "%~dp0build.ps1" %*
exit /b %ERRORLEVEL%

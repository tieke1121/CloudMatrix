@echo off
setlocal
powershell.exe -NoProfile -File "%~dp0start.ps1" %*
exit /b %errorlevel%

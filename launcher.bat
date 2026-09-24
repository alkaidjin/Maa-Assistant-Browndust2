@echo off
REM ============================================================
REM  BD2MAA launcher  (recommended entry / zero dependency / no Python)
REM  Flow: check GitHub release, show dialog, download and overwrite, run mxu.exe
REM  Args: -Force force check / -Demo demo dialog / -Test dry-run only / -Repair reinstall latest
REM ============================================================
cd /d "%~dp0"
powershell -NoProfile -ExecutionPolicy Bypass -File "BD2MAA-Updater.ps1" %*

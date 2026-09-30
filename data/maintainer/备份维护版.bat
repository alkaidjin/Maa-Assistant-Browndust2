@echo off
setlocal
chcp 65001 >nul

rem ==============================================================
rem  BD2MAA maintainer backup -- double-click entry
rem  Wraps tools\backup_maintainer.ps1: writes one timestamped folder
rem  containing the full git history (bundle) plus everything git
rem  does NOT track (mxu.exe, OCR models, config\, project memory),
rem  so a lost local folder can be restored on any machine.
rem  Maintainer tool: EXCLUDED from the release zip
rem  -- see EXCLUDE_FILES in tools/build_release_zip.py
rem  Keep pure ASCII + CRLF -- see check_bat_crlf() in that script.
rem ==============================================================

rem ---- CONFIG: cloud-upload target -------------------------------------
rem  Put the local folder your Quark client auto-backs up here. The client
rem  then uploads every new timestamp folder by itself (no credentials in
rem  this script, nothing to log in with).
rem  Leave EMPTY to use the default folder (next to the project):
rem    \MABd2-Maintainer-Backup
rem  Example:
rem    set "MABD2_BACKUP_DIR=D:\QuarkAutoBackup\MABd2"
set "MABD2_BACKUP_DIR="
rem ----------------------------------------------------------------------

cd /d "%~dp0"
powershell -NoProfile -ExecutionPolicy Bypass -File "tools\backup_maintainer.ps1" %*
set "RC=%ERRORLEVEL%"
echo.
if not "%RC%"=="0" echo [i] exit code = %RC%
if "%RC%"=="0" echo [i] done.
pause
exit /b %RC%

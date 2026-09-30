@echo off
REM ============================================================
REM  BD2MAA maintainer  (housekeeping / verify / mirror download)
REM  No args : housekeeping (no network, does NOT launch the app)
REM  -Verify : check installed files / resources / OCR models
REM  -Fetch  : download latest release zip to updates\ (no install)
REM  -Repair : download latest release zip and overwrite-install
REM  Keep pure ASCII + CRLF -- see check_bat_crlf() in build_release_zip.py
REM ============================================================
cd /d "%~dp0"
powershell -NoProfile -ExecutionPolicy Bypass -File "BD2MAA-Maintainer.ps1" %*

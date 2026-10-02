@echo off
setlocal EnableExtensions
chcp 65001 >nul 2>&1
title BD2MAA - Bootstrap External Assets

rem ============================================================
rem  Double-click entry for data/maintainer/tools/bootstrap_assets.py
rem  Pulls git-untracked release assets (OCR models) into the worktree
rem  after a fresh clone. See assets_manifest.json and the
rem  "Release external assets" section in ../README.md for details.
rem
rem  Interpreter lookup is identical to build_release_zip.bat.
rem  NOTE: keep this file pure ASCII with CRLF line endings.
rem ============================================================

set "SCRIPT=%~dp0bootstrap_assets.py"
if not exist "%SCRIPT%" (
  echo [ERROR] script not found: %SCRIPT%
  pause
  exit /b 1
)

set "PY="
if exist "%USERPROFILE%\.venvs\py313\Scripts\python.exe" set "PY=%USERPROFILE%\.venvs\py313\Scripts\python.exe"
if not defined PY if exist "%USERPROFILE%\.venvs\py313\python.exe" set "PY=%USERPROFILE%\.venvs\py313\python.exe"
if not defined PY if exist "%USERPROFILE%\.workbuddy\binaries\python\versions\3.13.12\python.exe" set "PY=%USERPROFILE%\.workbuddy\binaries\python\versions\3.13.12\python.exe"
if not defined PY if exist "%LOCALAPPDATA%\Programs\Python\Python313\python.exe" set "PY=%LOCALAPPDATA%\Programs\Python\Python313\python.exe"

if not defined PY (
  where python >nul 2>&1
  if not errorlevel 1 for /f "delims=" %%i in ('where python') do if not defined PY set "PY=%%i"
)
if not defined PY (
  py -3 -c "import sys" >nul 2>&1
  if not errorlevel 1 set "PY=py -3"
)

if not defined PY (
  echo [ERROR] No Python interpreter found.
  echo Edit this .bat and set PY= explicitly, or install Python 3.9+.
  pause
  exit /b 1
)

echo -----------------------------------------------
echo  Python : %PY%
echo  Script : %SCRIPT%
echo  Mode   : sync (pass --verify / --list / --force as needed)
echo -----------------------------------------------
echo.

if "%PY%"=="py -3" (
  py -3 "%SCRIPT%" %*
) else (
  "%PY%" "%SCRIPT%" %*
)
set "RC=%ERRORLEVEL%"

echo.
if "%RC%"=="0" (echo [DONE] exit code 0) else (echo [FAILED] exit code %RC%)
echo Press any key to close...
pause >nul
exit /b %RC%

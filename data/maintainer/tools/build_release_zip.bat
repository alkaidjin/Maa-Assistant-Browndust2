@echo off
setlocal EnableExtensions
chcp 65001 >nul 2>&1
title BD2MAA - Build Release Zip

rem ============================================================
rem  Double-click entry for data/maintainer/tools/build_release_zip.py
rem  Why not double-click the .py directly: on this machine *.py is
rem  associated with C://Windows//py.exe, but no Python is registered in
rem  the py launcher (system Python 3.10 was uninstalled; venvs and uv
rem  interpreters do not register themselves) -> the window flashes and
rem  closes. This .bat locates a real interpreter itself.
rem  NOTE: keep this file pure ASCII with CRLF line endings.
rem ============================================================

set "SCRIPT=%~dp0build_release_zip.py"
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
echo  Args   : %*
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

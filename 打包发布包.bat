@echo off
setlocal
chcp 65001 >nul

rem ==============================================================
rem  BD2MAA release packager -- double-click entry
rem  Wraps tools\build_release_zip.py so the packager can simply
rem  be double-clicked. With no argument it asks for the version
rem  interactively and then syncs every version-number file.
rem  Maintainer tool: EXCLUDED from the release zip
rem  -- see EXCLUDE_FILES in tools/build_release_zip.py
rem  Keep pure ASCII + CRLF -- see check_bat_crlf() in that script.
rem ==============================================================

set "PYEXE="

rem 1) python on PATH
for %%P in (python.exe) do if not defined PYEXE set "PYEXE=%%~$PATH:P"

rem 2) user main venv
if not defined PYEXE if exist "%USERPROFILE%\.venvs\py313\Scripts\python.exe" set "PYEXE=%USERPROFILE%\.venvs\py313\Scripts\python.exe"

rem 3) bundled managed interpreter
if not defined PYEXE if exist "%USERPROFILE%\.workbuddy\binaries\python\versions\3.13.12\python.exe" set "PYEXE=%USERPROFILE%\.workbuddy\binaries\python\versions\3.13.12\python.exe"

if not defined PYEXE (
  echo [ERROR] No Python interpreter found.
  echo         Install Python 3 and add it to PATH, then run again.
  echo.
  pause
  exit /b 1
)

echo [i] Python : %PYEXE%
echo.
cd /d "%~dp0"
"%PYEXE%" "tools\build_release_zip.py" %*
set "RC=%ERRORLEVEL%"
echo.
if not "%RC%"=="0" echo [i] exit code = %RC%
if "%RC%"=="0" echo [i] done.
pause
exit /b %RC%

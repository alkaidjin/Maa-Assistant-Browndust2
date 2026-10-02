@echo off
setlocal enabledelayedexpansion

REM ============================================================
REM wt_new.bat - create an agent worktree for BD2MAA
REM
REM usage: wt_new.bat ^<name^> [baseline]
REM   wt_new.bat traecode-workspace
REM   wt_new.bat workbuddy-workspace origin/main
REM
REM Pure ASCII + CRLF only (repo rule, see REF section 3.5).
REM ============================================================

set "MAIN=F:\MABd2v26.09.5"
set "WTROOT=F:\MABd2-wt"

if "%~1"=="" (
  echo usage: wt_new.bat ^<name^> [baseline]
  echo   example: wt_new.bat traecode-workspace origin/main
  exit /b 1
)

set "NAME=%~1"
set "BASE=%~2"
if "%BASE%"=="" set "BASE=origin/main"
set "DIR=%WTROOT%\%NAME%"
set "BR=agent/%NAME%"

if exist "%DIR%\" (
  echo FAILED: target already exists: %DIR%
  echo remove it with wt_clean.bat first, or pick another name.
  exit /b 1
)

if not exist "%WTROOT%\" mkdir "%WTROOT%"

echo [1/5] fetch origin ...
git -C "%MAIN%" fetch origin
if errorlevel 1 (
  echo WARNING: fetch failed, continuing with local refs.
)

echo [2/5] git worktree add -b %BR% %BASE%
git -C "%MAIN%" worktree add -b "%BR%" "%DIR%" %BASE%
if errorlevel 1 (
  echo FAILED: worktree add
  exit /b 1
)

REM ------------------------------------------------------------
REM [3/5] OCR model junction.
REM NOTE: resource/model/ocr/README.md is tracked by git, so a
REM fresh worktree already has a REAL ocr directory containing
REM that README. Remove it first, then junction the whole ocr
REM folder back to the main workspace. The README comes back
REM through the junction, so git status stays clean.
REM ------------------------------------------------------------
echo [3/5] link OCR model ...
if exist "%DIR%\resource\model\ocr\" (
  rmdir /S /Q "%DIR%\resource\model\ocr"
  if exist "%DIR%\resource\model\ocr\" (
    echo FAILED: could not remove staged ocr dir:
    echo   %DIR%\resource\model\ocr
    echo remove the worktree manually and retry.
    exit /b 1
  )
)
mklink /J "%DIR%\resource\model\ocr" "%MAIN%\resource\model\ocr" >nul
if errorlevel 1 (
  echo FAILED: mklink junction for OCR model
  exit /b 1
)

echo [4/5] copy config json ...
if not exist "%DIR%\config" mkdir "%DIR%\config"
xcopy /Y /Q "%MAIN%\config\*.json" "%DIR%\config\" >nul 2>nul

REM Copy the WHOLE memory folder (daily logs included, ~400 KB),
REM not just MEMORY.md. It is gitignored, so each workspace owns
REM its own snapshot; durable rules live in the tracked REF doc.
echo [5/5] copy workbuddy memory snapshot ...
if not exist "%DIR%\.workbuddy" mkdir "%DIR%\.workbuddy"
if exist "%MAIN%\.workbuddy\memory\" (
  xcopy /E /I /Y /Q "%MAIN%\.workbuddy\memory" "%DIR%\.workbuddy\memory" >nul 2>nul
)

echo.
echo ============================================================
echo done    : %DIR%
echo branch  : %BR%
echo baseline: %BASE%
echo.
echo verify  : git -C "%DIR%" status
echo remove  : wt_clean.bat %NAME%
echo ============================================================
endlocal

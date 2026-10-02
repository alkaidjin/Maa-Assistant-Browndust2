@echo off
setlocal

REM ============================================================
REM wt_clean.bat - safely remove an agent worktree for BD2MAA
REM
REM usage: wt_clean.bat ^<name^>
REM   wt_clean.bat traecode-workspace
REM
REM Order matters: drop the OCR junction BEFORE
REM "git worktree remove", so the target model files in the
REM main workspace can never be touched.
REM Pure ASCII + CRLF only (repo rule, see REF section 3.5).
REM ============================================================

set "MAIN=F:\MABd2v26.09.5"
set "WTROOT=F:\MABd2-wt"

if "%~1"=="" (
  echo usage: wt_clean.bat ^<name^>
  exit /b 1
)

set "DIR=%WTROOT%\%~1"

if not exist "%DIR%\" (
  echo not found: %DIR%
  exit /b 1
)

echo [1/4] uncommitted-changes guard ...
git -C "%DIR%" status --porcelain > "%TEMP%\wt_st.txt"
for %%A in ("%TEMP%\wt_st.txt") do (
  if %%~zA GTR 0 (
    echo ABORT: uncommitted changes in %DIR%
    echo commit, stash or push first. See git -C "%DIR%" status.
    del "%TEMP%\wt_st.txt"
    exit /b 1
  )
)
del "%TEMP%\wt_st.txt"

echo [2/4] detach OCR junction (plain rmdir, no /S - link only) ...
if exist "%DIR%\resource\model\ocr" rmdir "%DIR%\resource\model\ocr"
if exist "%DIR%\resource\model\ocr" (
  echo ABORT: junction still present, refusing to remove worktree.
  echo close MaaBd2.exe and retry, or inspect the path manually.
  exit /b 1
)

echo [3/4] git worktree remove ...
git -C "%MAIN%" worktree remove "%DIR%"
if errorlevel 1 (
  echo FAILED: worktree remove.
  echo close MaaBd2.exe / MaaPiCli in that workspace, then retry.
  exit /b 1
)

echo [4/4] prune metadata ...
git -C "%MAIN%" worktree prune

echo.
echo removed: %DIR%
echo branch still exists: delete after merge with
echo   git -C "%MAIN%" branch -d agent/%~1
endlocal

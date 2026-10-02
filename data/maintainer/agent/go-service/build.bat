@echo off
REM ===================================================================
REM  Build agent/go-service.exe  (BD2MAA go-service agent)
REM
REM  Trimmed fork of MaaEnd agent/go-service @ 64fac11 (AGPL-3.0).
REM  Only the components used by this repo are kept; see README.md here.
REM
REM  Requires Go 1.25.x. Lookup order:
REM    1. GOROOT environment variable
REM    2. go.exe on PATH
REM    3. <repo>\cache\_gotool\go   (main checkout case)
REM    4. cache\_gotool\go inside a sibling checkout found by walking up
REM       from this repo (git worktree case: the toolchain lives in the
REM       main workspace and the worktree's own cache\ is nearly empty)
REM
REM  The source lives in this folder but is NOT shipped to users;
REM  only the compiled agent/go-service.exe is. See README.md here.
REM ===================================================================
setlocal EnableExtensions EnableDelayedExpansion
cd /d "%~dp0"

REM This script lives at <repo>\data\maintainer\agent\go-service\ -> four levels up.
pushd "%~dp0..\..\..\.." >nul
set "REPO=%CD%\"
popd >nul
set "GOEXE="
set "GOBINDIR="
set "TOOLCACHE="

if defined GOROOT if exist "%GOROOT%\bin\go.exe" set "GOEXE=%GOROOT%\bin\go.exe"
if not defined GOEXE for %%I in (go.exe) do if not "%%~$PATH:I"=="" set "GOEXE=%%~$PATH:I"

REM Step 3: toolchain inside this checkout.
if exist "%REPO%\cache\_gotool\go\bin\go.exe" set "TOOLCACHE=%REPO%"

REM Step 4: worktree fallback - scan immediate children of each ancestor
REM directory (max 6 levels) for another checkout carrying the toolchain.
if not defined TOOLCACHE (
    set "SCAN=%REPO%"
    for /l %%L in (1,1,6) do (
        for /d %%D in ("!SCAN!..\*") do (
            if exist "%%~fD\cache\_gotool\go\bin\go.exe" if not defined TOOLCACHE set "TOOLCACHE=%%~fD"
        )
        set "SCAN=!SCAN!..\"
    )
)
if defined TOOLCACHE if not defined GOEXE set "GOEXE=!TOOLCACHE!\cache\_gotool\go\bin\go.exe"

if not defined GOEXE goto :nogo
for %%A in ("%GOEXE%") do set "GOBINDIR=%%~dpA"
echo [i] go: %GOEXE%

REM When using a cache-rooted toolchain, GOPATH/GOMODCACHE/GOCACHE must
REM live next to that toolchain tree (the main workspace in worktree mode),
REM not in the current (worktree) checkout.
if defined TOOLCACHE if /I "%GOEXE%"=="%TOOLCACHE%\cache\_gotool\go\bin\go.exe" (
    echo [i] tool cache root: %TOOLCACHE%
    set "GOPATH=!TOOLCACHE!\cache\_gotool\gopath"
    set "GOMODCACHE=!TOOLCACHE!\cache\_gotool\gopath\pkg\mod"
    set "GOCACHE=!TOOLCACHE!\cache\_gotool\gopath\build-cache"
)

if not defined GOPROXY set "GOPROXY=https://goproxy.cn,direct"
set "GOSUMDB=off"
set "GOFLAGS=-mod=mod"
set "CGO_ENABLED=0"
set "GOOS=windows"
set "GOARCH=amd64"
set "GOTOOLCHAIN=local"

REM Stamp main.Version with the git short SHA (falls back to "dev").
set "GSVER=dev"
for /f %%S in ('git rev-parse --short HEAD 2^>nul') do set "GSVER=bd2-%%S"

echo [i] gofmt (only files that actually need it, to avoid touching mtimes)
for /f "delims=" %%F in ('"%GOBINDIR%gofmt.exe" -l . 2^>nul') do (
    echo     formatting %%F
    "%GOBINDIR%gofmt.exe" -w "%%F"
)
echo [i] vet
"%GOEXE%" vet ./...
if errorlevel 1 goto :fail

echo [i] building %REPO%\agent\go-service.exe  (version %GSVER%)
"%GOEXE%" build -trimpath -ldflags "-s -w -X main.Version=%GSVER%" -o "%REPO%\agent\go-service.exe" .
if errorlevel 1 goto :fail

echo [ok] built %REPO%\agent\go-service.exe
echo     remember: git add -f agent/go-service.exe
echo               (the repo .gitignore ignores *.exe)
pause
exit /b 0

:nogo
echo [x] Go toolchain not found. Looked in:
echo       1. GOROOT env (%GOROOT%)
echo       2. go.exe on PATH
echo       3. %REPO%\cache\_gotool\go
echo       4. sibling checkouts via parent-directory scan (worktree mode)
echo     Install Go 1.25.x, set GOROOT, or unpack the toolchain to cache\_gotool\go
pause
exit /b 1

:fail
echo [x] build failed
pause
exit /b 1

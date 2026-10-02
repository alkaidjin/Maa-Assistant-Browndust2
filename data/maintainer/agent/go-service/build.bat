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
REM    3. <repo>\cache\_gotool\go   (the maintainer's local toolchain)
REM
REM  The source lives in this folder but is NOT shipped to users;
REM  only the compiled agent/go-service.exe is. See README.md here.
REM ===================================================================
setlocal
cd /d "%~dp0"

set "REPO=%~dp0..\..\.."
set "GOEXE="
set "GOBINDIR="
set "LOCALGO=%REPO%\cache\_gotool\go"

if defined GOROOT if exist "%GOROOT%\bin\go.exe" set "GOEXE=%GOROOT%\bin\go.exe"
if not defined GOEXE for %%I in (go.exe) do if not "%%~$PATH:I"=="" set "GOEXE=%%~$PATH:I"
if not defined GOEXE if exist "%LOCALGO%\bin\go.exe" set "GOEXE=%LOCALGO%\bin\go.exe"

if not defined GOEXE goto :nogo
for %%A in ("%GOEXE%") do set "GOBINDIR=%%~dpA"
echo [i] go: %GOEXE%

set "USELOCAL="
if /I "%GOEXE%"=="%LOCALGO%\bin\go.exe" set "USELOCAL=1"
if defined USELOCAL set "GOPATH=%REPO%\cache\_gotool\gopath"
if defined USELOCAL set "GOMODCACHE=%REPO%\cache\_gotool\gopath\pkg\mod"
if defined USELOCAL set "GOCACHE=%REPO%\cache\_gotool\gopath\build-cache"

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

echo [i] gofmt
"%GOBINDIR%gofmt.exe" -l -w . >nul 2>&1
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
echo [x] Go toolchain not found.
echo     Install Go 1.25.x, or set GOROOT, or unpack it to cache\_gotool\go
pause
exit /b 1

:fail
echo [x] build failed
pause
exit /b 1

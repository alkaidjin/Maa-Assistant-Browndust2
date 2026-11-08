@echo off
REM ===================================================================
REM  Build agent/bd2cli.exe  (AI-facing CLI for this project)
REM
REM  Design: docs/agent-cli-design.md
REM  Source lives here but is not shipped to users; only the compiled
REM  agent/bd2cli.exe is. Requires Go 1.25.x. Lookup order:
REM    1. GOROOT environment variable
REM    2. go.exe on PATH
REM    3. <repo>\cache\_gotool\go
REM ===================================================================
setlocal EnableExtensions EnableDelayedExpansion
cd /d "%~dp0"

REM This script lives at <repo>\data\maintainer\agent\bd2cli\ -> four levels up.
pushd "%~dp0..\..\..\.." >nul
set "REPO=%CD%\"
popd >nul
set "GOEXE="
set "GOBINDIR="
set "TOOLCACHE="

if defined GOROOT if exist "%GOROOT%\bin\go.exe" set "GOEXE=%GOROOT%\bin\go.exe"
if not defined GOEXE for %%I in (go.exe) do if not "%%~$PATH:I"=="" set "GOEXE=%%~$PATH:I"
if exist "%REPO%\cache\_gotool\go\bin\go.exe" set "TOOLCACHE=%REPO%"
if defined TOOLCACHE if not defined GOEXE set "GOEXE=!TOOLCACHE!\cache\_gotool\go\bin\go.exe"

if not defined GOEXE goto :nogo
for %%A in ("%GOEXE%") do set "GOBINDIR=%%~dpA"
echo [i] go: %GOEXE%

if defined TOOLCACHE if /I "%GOEXE%"=="%TOOLCACHE%\cache\_gotool\go\bin\go.exe" (
    set "GOPATH=!TOOLCACHE!\cache\_gotool\gopath"
    set "GOMODCACHE=!TOOLCACHE!\cache\_gotool\gopath\pkg\mod"
    set "GOCACHE=!TOOLCACHE!\cache\_gotool\gopath\build-cache"
)

set "GOSUMDB=off"
set "GOFLAGS=-mod=mod"
set "CGO_ENABLED=0"
set "GOOS=windows"
set "GOARCH=amd64"
set "GOTOOLCHAIN=local"

echo [i] gofmt
for /f "delims=" %%F in ('"%GOBINDIR%gofmt.exe" -l . 2^>nul') do (
    echo     formatting %%F
    "%GOBINDIR%gofmt.exe" -w "%%F"
)
echo [i] vet
"%GOEXE%" vet ./...
if errorlevel 1 goto :fail
echo [i] test
"%GOEXE%" test ./...
if errorlevel 1 goto :fail

echo [i] building %REPO%agent\bd2cli.exe
"%GOEXE%" build -trimpath -ldflags "-s -w" -o "%REPO%agent\bd2cli.exe" .
if errorlevel 1 goto :fail

echo [ok] built %REPO%agent\bd2cli.exe
echo     remember: git add -f agent/bd2cli.exe
echo               (the repo .gitignore ignores *.exe)
exit /b 0

:nogo
echo [x] Go toolchain not found. Looked in:
echo       1. GOROOT env (%GOROOT%)
echo       2. go.exe on PATH
echo       3. %REPO%cache\_gotool\go
exit /b 1

:fail
echo [x] build failed
exit /b 1

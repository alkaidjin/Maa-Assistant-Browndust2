@echo off
REM ===================================================================
REM  Build agent/rock-picker.exe  (BD2MAA rock-picker agent)
REM
REM  Requires Go 1.25.x. Lookup order:
REM    1. GOROOT environment variable
REM    2. go.exe on PATH
REM    3. <repo>\cache\_gotool\go   (the maintainer's local toolchain)
REM
REM  The source lives in this folder but is NOT shipped to users;
REM  only the compiled agent/rock-picker.exe is. See README.md here.
REM ===================================================================
setlocal
cd /d "%~dp0"

set "REPO=%~dp0..\.."
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

echo [i] gofmt
"%GOBINDIR%gofmt.exe" -l -w . >nul 2>&1
echo [i] vet + test
"%GOEXE%" vet ./...
if errorlevel 1 goto :fail
"%GOEXE%" test ./...
if errorlevel 1 goto :fail

echo [i] building ..\rock-picker.exe
"%GOEXE%" build -trimpath -ldflags "-s -w" -o "%~dp0..\rock-picker.exe" .
if errorlevel 1 goto :fail

echo [ok] built %~dp0..\rock-picker.exe
echo     remember: git add -f agent/rock-picker.exe
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

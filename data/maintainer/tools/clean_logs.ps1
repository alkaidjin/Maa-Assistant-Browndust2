# clean_logs.ps1
# ---------------------------------------------------------------------------
# BD2MAA log housekeeping: keep only the most recent N days of debug/ logs.
#
# MXU / MaaFramework write a steady stream of logs into <root>\debug:
#   - mxu-web-YYYY-MM-DD.log   (rotated daily by MXU itself)
#   - mxu-agent.log, mxu-tauri.log, maa.log, go-service.log  (rolling)
# Left alone they grow without bound. This script trims anything whose
# LastWriteTime is older than the retention window (default 7 days).
#
# Safe to run at any time: it never touches tasks/, config/, resource/ or
# any file outside <root>\debug.
#
# Usage:
#   powershell -NoProfile -ExecutionPolicy Bypass -File maintainer\tools\clean_logs.ps1
#   powershell -NoProfile -ExecutionPolicy Bypass -File maintainer\tools\clean_logs.ps1 -Days 14
#   powershell -NoProfile -ExecutionPolicy Bypass -File maintainer\tools\clean_logs.ps1 -DryRun
# ---------------------------------------------------------------------------
param(
    [string]$Root,
    [int]$Days = 7,
    [switch]$DryRun
)

$ErrorActionPreference = 'SilentlyContinue'

# 脚本住在 <项目根>/maintainer/tools/ 下（维护者档案目录，不入用户包），
# 所以项目根 = 脚本所在目录的**上两级**。
if (-not $Root) { $Root = Split-Path -Parent (Split-Path -Parent $PSScriptRoot) }

$debugDir = Join-Path $Root 'debug'
if (-not (Test-Path -LiteralPath $debugDir)) {
    Write-Host "clean_logs: no debug/ directory under $Root - nothing to do."
    exit 0
}

$cutoff  = (Get-Date).AddDays(-$Days)
$files   = @(Get-ChildItem -LiteralPath $debugDir -Recurse -File -ErrorAction SilentlyContinue)

$deleted = 0
$freed   = 0
$kept    = 0

foreach ($f in $files) {
    if ($f.LastWriteTime -ge $cutoff) { $kept++; continue }

    if ($DryRun) {
        Write-Host ("  [dry] {0}  ({1:N0} bytes, {2:yyyy-MM-dd})" -f $f.Name, $f.Length, $f.LastWriteTime)
        $deleted++
        $freed += $f.Length
        continue
    }

    $size = $f.Length
    Remove-Item -LiteralPath $f.FullName -Force -ErrorAction SilentlyContinue
    if (-not (Test-Path -LiteralPath $f.FullName)) {
        $deleted++
        $freed  += $size
    } else {
        # handle held by a running MXU instance -> leave it, try next launch
        $kept++
    }
}

$afterBytes = 0
foreach ($f in @(Get-ChildItem -LiteralPath $debugDir -Recurse -File -ErrorAction SilentlyContinue)) {
    $afterBytes += $f.Length
}
$afterMB = [math]::Round($afterBytes / 1MB, 2)

if ($DryRun) {
    Write-Host ("clean_logs [dry-run]: {0} file(s) older than {1} day(s); {2} within window. debug/ = {3} MB" -f $deleted, $Days, $kept, $afterMB)
} else {
    Write-Host ("clean_logs: removed {0} file(s), freed {1:N0} bytes; kept {2}. debug/ = {3} MB" -f $deleted, $freed, $kept, $afterMB)
}

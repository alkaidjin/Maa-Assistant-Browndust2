# apply_icon.ps1
# ---------------------------------------------------------------------------
# Applies mxu.ico to mxu.exe (embedded PE icon) and (re)creates a shortcut to
# the launcher with the same icon. Safe to re-run: it backs mxu.exe up once and
# refuses to touch the exe while MXU is running.
#
# Usage:  powershell -NoProfile -ExecutionPolicy Bypass -File apply_icon.ps1
# ---------------------------------------------------------------------------
param(
    [string]$Base
)

$ErrorActionPreference = 'SilentlyContinue'

if (-not $Base) {
    $Base = Split-Path -Parent $PSScriptRoot
}

$mxu   = Join-Path $Base 'mxu.exe'
$ico   = Join-Path $Base 'mxu.ico'
$rcdir = Join-Path $Base 'tools'
$rc    = Join-Path $rcdir 'rcedit-x64.exe'
$rcAlt = Join-Path $rcdir 'rcedit.exe'
$cache = Join-Path $Base 'cache'
$bak   = Join-Path $cache 'mxu.exe.bak'
$lnk   = Join-Path $Base 'MaaBd2.lnk'
$bat   = Join-Path $Base 'launcher.bat'

function Say($m) { Write-Host $m }

Say '=== BD2MAA icon helper ==='

if (-not (Test-Path -LiteralPath $ico)) { Say "ERROR: missing icon $ico"; exit 1 }
if (-not (Test-Path -LiteralPath $mxu)) { Say "ERROR: missing exe $mxu";  exit 1 }

if (-not (Test-Path -LiteralPath $rc)) {
    if (Test-Path -LiteralPath $rcAlt) { $rc = $rcAlt } else { $rc = $null }
}

# ---- 1. embed the icon into mxu.exe -------------------------------------
$running = @(Get-Process -Name 'mxu' -ErrorAction SilentlyContinue).Count
if ($running -gt 0) {
    Say 'WARN: mxu.exe is running - close MXU first, then re-run this tool.'
} elseif (-not $rc) {
    Say 'WARN: tools\rcedit-x64.exe not found - cannot patch mxu.exe.'
} else {
    if (-not (Test-Path -LiteralPath $cache)) { New-Item -ItemType Directory -Path $cache -Force | Out-Null }
    if (-not (Test-Path -LiteralPath $bak)) {
        Copy-Item -LiteralPath $mxu -Destination $bak -Force
        Say "backup: $bak"
    }
    & $rc $mxu --set-icon $ico 2>&1 | ForEach-Object { Say $_ }
    if ($LASTEXITCODE -eq 0) {
        $fi = Get-Item -LiteralPath $mxu
        ('{0}|{1}|{2}|{3}' -f $fi.Length, $fi.LastWriteTimeUtc.Ticks, (Get-Item -LiteralPath $ico).Length, (Get-Item -LiteralPath $ico).LastWriteTimeUtc.Ticks) |
            Set-Content -LiteralPath (Join-Path $cache 'icon_stamp.txt') -Encoding ASCII
        Say 'OK: icon embedded into mxu.exe'
    } else {
        Say ("ERROR: rcedit failed (exit {0})" -f $LASTEXITCODE)
    }
}

# ---- 2. shortcut with the icon -----------------------------------------
try {
    $ws  = New-Object -ComObject WScript.Shell
    $s   = $ws.CreateShortcut($lnk)
    $s.TargetPath       = $bat
    $s.WorkingDirectory = $Base
    $s.IconLocation     = "$ico,0"
    $s.Description      = 'BD2MAA launcher'
    $s.Save()
    Say "OK: shortcut -> $lnk"
} catch {
    Say ("ERROR: shortcut failed: {0}" -f $_.Exception.Message)
}

Say 'done.'

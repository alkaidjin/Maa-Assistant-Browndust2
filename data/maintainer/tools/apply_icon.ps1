# apply_icon.ps1
# ---------------------------------------------------------------------------
# Applies mxu.ico to MaaBd2.exe (embedded PE icon) and (re)creates a shortcut
# to the exe with the same icon. Safe to re-run: it backs the exe up once and
# refuses to touch it while the app is running.
#
# v26.09.14: root cleanup + rename. exe = MaaBd2.exe (was mxu.exe); the icon
# sources live next to this script (data/maintainer/tools/). Normally you do
# NOT need to run this by hand: build_release_zip.py's ensure_icon() re-checks
# and re-applies the icon before every build. Keep this script as the manual
# fallback.
#
# Usage:  powershell -NoProfile -ExecutionPolicy Bypass -File apply_icon.ps1
# ---------------------------------------------------------------------------
param(
    [string]$Base
)

$ErrorActionPreference = 'SilentlyContinue'

if (-not $Base) {
    # 脚本住在 <项目根>/data/maintainer/tools/ 下（维护者档案目录，不入用户包），
    # 所以项目根 = 脚本所在目录的上三级。
    $Base = Split-Path -Parent (Split-Path -Parent (Split-Path -Parent $PSScriptRoot))
}

$exeName = 'MaaBd2.exe'
$mxu   = Join-Path $Base $exeName
if (-not (Test-Path -LiteralPath $mxu)) { $mxu = Join-Path $Base 'mxu.exe'; $exeName = 'mxu.exe' }
$ico   = Join-Path $PSScriptRoot 'mxu.ico'
$rc    = Join-Path $PSScriptRoot 'rcedit-x64.exe'
$rcAlt = Join-Path $PSScriptRoot 'rcedit.exe'
$cache = Join-Path $Base 'date\cache'
$bak   = Join-Path $cache ($exeName + '.bak')
$lnk   = Join-Path $Base 'MaaBd2.lnk'
$target = $mxu   # 快捷方式直接指向软件本体

function Say($m) { Write-Host $m }

Say '=== BD2MAA icon helper ==='

if (-not (Test-Path -LiteralPath $ico)) { Say "ERROR: missing icon $ico"; exit 1 }
if (-not (Test-Path -LiteralPath $mxu)) { Say "ERROR: missing exe $mxu";  exit 1 }

if (-not (Test-Path -LiteralPath $rc)) {
    if (Test-Path -LiteralPath $rcAlt) { $rc = $rcAlt } else { $rc = $null }
}

# ---- 1. embed the icon into the exe --------------------------------------
$procName = [IO.Path]::GetFileNameWithoutExtension($exeName)
$running = @(Get-Process -Name $procName -ErrorAction SilentlyContinue).Count
if ($running -gt 0) {
    Say "WARN: $exeName is running - close the app first, then re-run this tool."
} elseif (-not $rc) {
    Say 'WARN: rcedit-x64.exe not found - cannot patch the exe.'
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
        Say "OK: icon embedded into $exeName"
    } else {
        Say ("ERROR: rcedit failed (exit {0})" -f $LASTEXITCODE)
    }
}

# ---- 2. shortcut with the icon -----------------------------------------
try {
    $ws  = New-Object -ComObject WScript.Shell
    $s   = $ws.CreateShortcut($lnk)
    $s.TargetPath       = $target
    $s.WorkingDirectory = $Base
    $s.IconLocation     = "$ico,0"
    $s.Description      = 'BD2MAA'
    $s.Save()
    Say "OK: shortcut -> $lnk"
} catch {
    Say ("ERROR: shortcut failed: {0}" -f $_.Exception.Message)
}

Say 'done.'

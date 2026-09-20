# backup_maintainer.ps1
# ---------------------------------------------------------------------------
# BD2MAA maintainer backup -- keeps a restorable copy of everything that a
# bare `git clone` can NOT give you back.
#
# Why this exists: the GitHub repo holds the tracked files only. A maintainer
# copy that can actually RUN and be maintained additionally needs
#   * mxu.exe                 (~31 MB, untracked -- MXU itself)
#   * resource\model\ocr\     (~21 MB, untracked -- OCR models)
#   * local-only state        (config\, .workbuddy\ memory, scratch tooling)
# Those three are exactly the parts that vanish if the local folder is lost.
#
# Produced per run, into  <OutDir>\<yyyyMMdd-HHmmss>\ :
#   MABd2-repo-<stamp>.bundle     full git history, scalable/standalone clone
#   MABd2-runtime-<stamp>.zip     mxu.exe + resource\model\ocr
#   MABd2-local-<stamp>.zip       config\, .workbuddy\, cache\_gatetest, cache\old
#   README-restore.txt            manifest + restore steps
#
# Restore on any machine:
#   1. git clone MABd2-repo-<stamp>.bundle MABd2
#   2. unpack MABd2-runtime-<stamp>.zip into MABd2\   (adds mxu.exe + OCR models)
#   3. unpack MABd2-local-<stamp>.zip   into MABd2\   (adds config\ and .workbuddy\)
#
# Usage:
#   powershell -NoProfile -ExecutionPolicy Bypass -File tools\backup_maintainer.ps1
#   ... -OutDir "G:\MABd2-Maintainer-Backup"   (NAS / USB drive / cloud folder)
#   ... -NoRuntime        skip mxu.exe + OCR models (~52 MB raw)
#   ... -NoBundle         skip the git bundle (~150 MB)
#   ... -Keep 3           how many older backups to retain (default 5)
#
# Cloud upload: this script never talks to a network drive provider itself.
# Point -OutDir (or the MABD2_BACKUP_DIR environment variable, which the
# double-click entry sets) at a folder that the Quark client auto-backs up,
# and the client uploads every new timestamp folder on its own -- this script
# never touches your credentials.
#
# OutDir MUST live outside the project folder: the release packager walks the
# whole working tree, so a backup dropped inside it would end up in the user
# zip. The script refuses to run in that case.
#
# ASCII-only on purpose: Windows PowerShell 5.1 reads BOM-less files as ANSI,
# so non-ASCII literals here would be garbled. Same rule as tools\clean_logs.ps1.
# ---------------------------------------------------------------------------
param(
    [string]$Root,
    [string]$OutDir,
    [switch]$NoBundle,
    [switch]$NoRuntime,
    [int]$Keep = 5
)

$ErrorActionPreference = 'Stop'

# ZipArchiveMode / CompressionLevel live in System.IO.Compression; ZipFile in
# System.IO.Compression.FileSystem. Both are needed, and loading them up front
# keeps the helper function free of repeated Add-Type calls.
Add-Type -AssemblyName System.IO.Compression | Out-Null
Add-Type -AssemblyName System.IO.Compression.FileSystem | Out-Null

if (-not $Root) { $Root = Split-Path -Parent $PSScriptRoot }
$rootFull = [System.IO.Path]::GetFullPath($Root)

if (-not (Test-Path -LiteralPath (Join-Path $rootFull 'interface.json'))) {
    Write-Host "[ERROR] interface.json not found under: $rootFull"
    exit 1
}

# Resolve the output folder. Order matters:
#   1. -OutDir parameter (the scheduled tasks pass it explicitly)
#   2. MABD2_BACKUP_DIR in the process environment (inherited from a fresh shell)
#   3. MABD2_BACKUP_DIR in HKCU:\Environment -- needed because Explorer does not
#      pick up a newly set user variable until it restarts, and a double-clicked
#      .bat is spawned by Explorer. Reading the registry directly makes the
#      double-click entry trustworthy.
#   4. default: a folder next to the project
if (-not $OutDir) { $OutDir = $env:MABD2_BACKUP_DIR }
if (-not $OutDir) {
    $reg = Get-ItemProperty -Path 'HKCU:\Environment' -Name 'MABD2_BACKUP_DIR' -ErrorAction SilentlyContinue
    if ($reg) { $OutDir = $reg.MABD2_BACKUP_DIR }
}
if (-not $OutDir) { $OutDir = Join-Path (Split-Path -Parent $rootFull) 'MABd2-Maintainer-Backup' }
$outFull = [System.IO.Path]::GetFullPath($OutDir)

if ($outFull.TrimEnd('\') -eq $rootFull.TrimEnd('\') -or
    $outFull.StartsWith($rootFull.TrimEnd('\') + '\', [System.StringComparison]::OrdinalIgnoreCase)) {
    Write-Host "[ERROR] OutDir must be OUTSIDE the project folder."
    Write-Host "        Inside it, the release packager would sweep the backup into the user zip."
    Write-Host "        Got: $outFull"
    exit 1
}

if (-not (Get-Command git -ErrorAction SilentlyContinue)) {
    Write-Host "[ERROR] git was not found on PATH."
    exit 1
}

# A full run and a local-only run are counted separately at retention time, so
# the folder name carries the kind. Without it, six days of tiny local folders
# would push the weekly full backup out of a shared keep window and delete it.
$kind = 'full'
if ($NoBundle -and $NoRuntime) { $kind = 'local' }

$stamp = Get-Date -Format 'yyyyMMdd-HHmmss'
$dest  = Join-Path $outFull "$kind-$stamp"
New-Item -ItemType Directory -Path $dest -Force | Out-Null

Write-Host "[i] project : $rootFull"
Write-Host "[i] target  : $dest"
Write-Host ""

$manifest = New-Object System.Collections.Generic.List[string]
$manifest.Add("BD2MAA maintainer backup")
$manifest.Add("created : " + (Get-Date -Format 'yyyy-MM-dd HH:mm:ss'))
$manifest.Add("project : $rootFull")
$manifest.Add("target  : $dest")
$manifest.Add("")

function Add-ToZip {
    param([string]$ZipPath, [string]$BaseDir, [string[]]$Items)
    $zip = [System.IO.Compression.ZipFile]::Open($ZipPath, [System.IO.Compression.ZipArchiveMode]::Create)
    $count = 0
    try {
        foreach ($item in $Items) {
            $full = (Resolve-Path -LiteralPath $item).Path
            if (Test-Path -LiteralPath $full -PathType Container) {
                $files = @(Get-ChildItem -LiteralPath $full -Recurse -File -Force)
            } else {
                $files = @(Get-Item -LiteralPath $full -Force)
            }
            foreach ($f in $files) {
                # keep the path relative to the project root, so unzipping over an
                # existing clone restores every file in place
                $rel = $f.FullName.Substring($BaseDir.TrimEnd('\').Length).TrimStart('\', '/').Replace('\', '/')
                [void][System.IO.Compression.ZipFileExtensions]::CreateEntryFromFile(
                    $zip, $f.FullName, $rel, [System.IO.Compression.CompressionLevel]::Optimal)
                $count++
            }
        }
    } finally {
        $zip.Dispose()
    }
    return $count
}

$step = 0
$total = 3

# ---- 1) git bundle: full history, clone-able standalone -------------------
if (-not $NoBundle) {
    $step++
    $bundle = Join-Path $dest ("MABd2-repo-$stamp.bundle")
    Write-Host "[$step/$total] git bundle (full history) ..."
    $head = (& git -C $rootFull rev-parse --short HEAD 2>$null)
    $branch = (& git -C $rootFull rev-parse --abbrev-ref HEAD 2>$null)
    $dirty = (& git -C $rootFull status --porcelain 2>$null)
    & git -C $rootFull bundle create $bundle --all 2>&1 | Out-Null
    if ((Test-Path -LiteralPath $bundle) -and $LASTEXITCODE -eq 0) {
        $mb = [math]::Round((Get-Item -LiteralPath $bundle).Length / 1MB, 2)
        Write-Host ("      OK  MABd2-repo-$stamp.bundle  ($mb MB)")
        $manifest.Add("[1] MABd2-repo-$stamp.bundle    $mb MB")
        $manifest.Add("    git HEAD   : $head (branch $branch)")
        if ($dirty) {
            $manifest.Add("    NOTE       : HEAD had uncommitted changes at backup time")
            Write-Host "      [W] uncommitted changes were present -- commit/push them to be safe"
        }
    } else {
        Write-Host "      [W] git bundle failed"
        $manifest.Add("[1] git bundle FAILED")
    }
}

# ---- 2) runtime assets: untracked, but required to actually run -----------
if (-not $NoRuntime) {
    $step++
    $rtItems = @()
    foreach ($rel in @('mxu.exe', 'resource\model\ocr')) {
        $p = Join-Path $rootFull $rel
        if (Test-Path -LiteralPath $p) { $rtItems += $p }
    }
    if ($rtItems.Count -gt 0) {
        $zip = Join-Path $dest ("MABd2-runtime-$stamp.zip")
        Write-Host "[$step/$total] runtime assets -> MABd2-runtime-$stamp.zip ..."
        $n = Add-ToZip -ZipPath $zip -BaseDir $rootFull -Items $rtItems
        $mb = [math]::Round((Get-Item -LiteralPath $zip).Length / 1MB, 2)
        Write-Host "      OK  ($mb MB, $n files)"
        $manifest.Add("[2] MABd2-runtime-$stamp.zip  $mb MB, $n files")
        foreach ($p in $rtItems) {
            $manifest.Add("    + " + $p.Substring($rootFull.TrimEnd('\').Length).TrimStart('\', '/'))
        }
    } else {
        Write-Host "[$step/$total] runtime assets ... none found, skipped"
    }
}

# ---- 3) local-only state -------------------------------------------------
$step++
$localItems = @()
foreach ($rel in @('config', '.workbuddy', 'cache\_gatetest', 'cache\old')) {
    $p = Join-Path $rootFull $rel
    if (Test-Path -LiteralPath $p) { $localItems += $p }
}
if ($localItems.Count -gt 0) {
    $zip = Join-Path $dest ("MABd2-local-$stamp.zip")
    Write-Host "[$step/$total] local state -> MABd2-local-$stamp.zip ..."
    $n = Add-ToZip -ZipPath $zip -BaseDir $rootFull -Items $localItems
    $mb = [math]::Round((Get-Item -LiteralPath $zip).Length / 1MB, 2)
    Write-Host "      OK  ($mb MB, $n files)"
    $manifest.Add("[3] MABd2-local-$stamp.zip    $mb MB, $n files")
    foreach ($p in $localItems) {
        $manifest.Add("    + " + $p.Substring($rootFull.TrimEnd('\').Length).TrimStart('\', '/'))
    }
} else {
    Write-Host "[$step/$total] local state ... none found, skipped"
}

# ---- manifest ------------------------------------------------------------
$manifest.Add("")
$manifest.Add("Restore:")
$manifest.Add("  1) git clone MABd2-repo-$stamp.bundle MABd2")
$manifest.Add("  2) unpack MABd2-runtime-$stamp.zip into MABd2\   (mxu.exe + OCR models)")
$manifest.Add("  3) unpack MABd2-local-$stamp.zip   into MABd2\   (config\ + .workbuddy\ + scratch)")
$manifest.Add("")
$manifest.Add("Not included on purpose:")
$manifest.Add("  updates\*.zip        re-downloadable from GitHub Releases")
$manifest.Add("  cache\ binaries      MXU / MaaFramework downloads, re-obtainable upstream")
$manifest.Add("  debug\ logs          transient")
$manifestPath = Join-Path $dest 'README-restore.txt'
$manifest | Out-File -FilePath $manifestPath -Encoding ascii

# ---- retention: drop older backups we created ourselves -------------------
# Counted per kind. Anything that is not '-' + <stamp> is left alone, so a
# folder a human put there is never swept away by an unattended run.
function Get-BackupFolders {
    param([string]$Dir, [string]$Kind)
    $found = New-Object System.Collections.Generic.List[object]
    foreach ($d in (Get-ChildItem -LiteralPath $Dir -Directory -ErrorAction SilentlyContinue)) {
        $key = $null
        if ($d.Name -match '^(full|local)-(\d{8}-\d{6})$') {
            if ($Matches[1] -eq $Kind) { $key = $Matches[2] }
        } elseif ($d.Name -match '^(\d{8}-\d{6})$') {
            # folders from before the full-/local- prefix: always full backups
            if ($Kind -eq 'full') { $key = $Matches[1] }
        }
        if ($key) { $found.Add([pscustomobject]@{ Path = $d.FullName; Key = $key }) }
    }
    return $found
}

$mine  = @(Get-BackupFolders -Dir $outFull -Kind $kind)
$stale = @($mine | Sort-Object Key -Descending | Select-Object -Skip $Keep)
foreach ($s in $stale) {
    Remove-Item -LiteralPath $s.Path -Recurse -Force -ErrorAction SilentlyContinue
}
if ($stale.Count -gt 0) {
    Write-Host "[i] removed $($stale.Count) older '$kind' backup folder(s), kept newest $Keep"
}

# ---- summary -------------------------------------------------------------
Write-Host ""
Write-Host "[i] backup complete:"
Get-ChildItem -LiteralPath $dest | Sort-Object Name | ForEach-Object {
    Write-Host ("      {0,-34} {1,8:N2} MB" -f $_.Name, ($_.Length / 1MB))
}
Write-Host ""
Write-Host "[i] project folder : $rootFull"
exit 0

# install_backup_task.ps1
# ---------------------------------------------------------------------------
# Registers (or removes) the Windows Task Scheduler jobs that run the
# maintainer backup unattended. Two jobs, because the two halves cost very
# different amounts:
#
#   MABd2-Backup-Daily     every day   local-only state (~1 MB, seconds)
#   MABd2-Backup-Weekly    weekly      full run: + git bundle + mxu.exe/OCR
#
# Both run hidden (no console window) and at below-normal priority: they fire
# at 07:00, when the game script is already driving the machine, so the backup
# must yield CPU and disk to it. The two jobs are 30 minutes apart on purpose:
# they share one settings object with MultipleInstances=IgnoreNew, so a trigger
# collision would silently swallow the later one.
#
# Why Task Scheduler and not some app-level scheduler: a backup must not
# depend on another program being open. Task Scheduler runs even when nothing
# else is, and it runs as you -- so the Quark client, already running in your
# session, notices the new files and uploads them on its own.
#
# Usage:
#   powershell -NoProfile -ExecutionPolicy Bypass -File tools\install_backup_task.ps1
#   ... -BackupDir "D:\QuarkAutoBackup\MABd2"   folder the Quark client backs up
#   ... -DailyTime 07:00  -WeeklyDay Sunday -WeeklyTime 07:30
#   ... -Action Status      show whether the jobs exist and their last result
#   ... -Action Uninstall   remove both jobs
#
# ASCII-only on purpose: Windows PowerShell 5.1 reads BOM-less files as ANSI,
# so non-ASCII literals here would come out garbled.
# ---------------------------------------------------------------------------
param(
    [ValidateSet('Install', 'Uninstall', 'Status')]
    [string]$Action = 'Install',
    [string]$BackupDir,
    [string]$DailyTime = '07:00',
    [string]$WeeklyDay = 'Sunday',
    [string]$WeeklyTime = '07:30',
    [int]$DailyKeep = 10,
    [int]$WeeklyKeep = 6,
    [string]$Root
)

$ErrorActionPreference = 'Stop'

if (-not $Root) { $Root = Split-Path -Parent $PSScriptRoot }
$Root = [System.IO.Path]::GetFullPath($Root)
$ps1  = Join-Path $Root 'tools\backup_maintainer.ps1'

$dailyName  = 'MABd2-Backup-Daily'
$weeklyName = 'MABd2-Backup-Weekly'

if ($Action -eq 'Status') {
    foreach ($n in @($dailyName, $weeklyName)) {
        $t = Get-ScheduledTask -TaskName $n -ErrorAction SilentlyContinue
        if (-not $t) {
            Write-Host "[ ] $n : not registered"
            continue
        }
        $i = Get-ScheduledTaskInfo -TaskName $n
        Write-Host ("[x] {0} : state={1} last={2} result={3} next={4}" -f $n, $t.State, $i.LastRunTime, $i.LastTaskResult, $i.NextRunTime)
    }
    exit 0
}

if ($Action -eq 'Uninstall') {
    foreach ($n in @($dailyName, $weeklyName)) {
        if (Get-ScheduledTask -TaskName $n -ErrorAction SilentlyContinue) {
            Unregister-ScheduledTask -TaskName $n -Confirm:$false
            Write-Host "[i] removed: $n"
        } else {
            Write-Host "[ ] not present: $n"
        }
    }
    exit 0
}

# ---- Install -------------------------------------------------------------
if (-not (Test-Path -LiteralPath $ps1)) {
    Write-Host "[ERROR] backup script not found: $ps1"
    exit 1
}

# -NonInteractive: no prompt may ever block an unattended run.
# -WindowStyle Hidden: powershell.exe hides the console it creates, so nothing
# flashes on screen at 07:00 while the game script is running.
$base = "-NoProfile -NonInteractive -WindowStyle Hidden -ExecutionPolicy Bypass -File `"$ps1`""
if ($BackupDir) {
    $bd = [System.IO.Path]::GetFullPath($BackupDir)
    $base += " -OutDir `"$bd`""
    Write-Host "[i] backup dir : $bd"
} else {
    Write-Host "[i] backup dir : default (project parent)\MABd2-Maintainer-Backup"
}

# Priority 5 = below normal: the backup yields to the game script.
$settings = New-ScheduledTaskSettingsSet -StartWhenAvailable -MultipleInstances IgnoreNew `
            -ExecutionTimeLimit (New-TimeSpan -Minutes 45) -Priority 5 `
            -AllowStartIfOnBatteries -DontStopIfGoingOnBatteries

$dailyAction  = New-ScheduledTaskAction -Execute 'powershell.exe' -WorkingDirectory $Root `
                -Argument "$base -NoBundle -NoRuntime -Keep $DailyKeep"
$dailyTrigger = New-ScheduledTaskTrigger -Daily -At $DailyTime
Register-ScheduledTask -TaskName $dailyName -Action $dailyAction -Trigger $dailyTrigger `
    -Settings $settings -Description 'BD2MAA maintainer backup: local-only state (fast daily run)' -Force | Out-Null
Write-Host "[i] registered: $dailyName  (daily at $DailyTime, local state only, keep $DailyKeep)"

$weeklyAction  = New-ScheduledTaskAction -Execute 'powershell.exe' -WorkingDirectory $Root `
                 -Argument "$base -Keep $WeeklyKeep"
$weeklyTrigger = New-ScheduledTaskTrigger -Weekly -DaysOfWeek $WeeklyDay -At $WeeklyTime
Register-ScheduledTask -TaskName $weeklyName -Action $weeklyAction -Trigger $weeklyTrigger `
    -Settings $settings -Description 'BD2MAA maintainer backup: full run incl. git bundle and runtime assets' -Force | Out-Null
Write-Host "[i] registered: $weeklyName  (every $WeeklyDay at $WeeklyTime, full backup, keep $WeeklyKeep)"

Write-Host ""
Write-Host "[i] done. -Action Status to inspect, -Action Uninstall to remove."
exit 0

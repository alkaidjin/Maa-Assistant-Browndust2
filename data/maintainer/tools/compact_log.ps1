<#
.SYNOPSIS
    精简 MaaFramework 生成的 debug 日志（debug\maa.log / debug\maa.bak.log）。

.DESCRIPTION
    为什么需要这个脚本：

      MaaFramework 的文件日志固定以 Trace 级别全量落盘，没有配置项可以降低
      （config\maa_option.json 里的 stdout_level 只影响控制台，不影响文件）。
      实测跑一轮日常任务会产生 9~16 MB / 2~5 万行日志，其中约 99% 是事件通知、
      识别过程、双进程重复投递等对排错没有帮助的内容。

    本脚本把"消防水管"式的原始日志压缩成一份能从头读到尾的「任务时间线」：

      保留  警告 / 错误（WRN / ERR / FATAL）
      保留  任务边界（Tasker.Task.Starting | Succeeded | Failed）
      保留  节点边界（Node.PipelineNode.Starting | Succeeded | Failed）
      保留  失败标记（Task timeout / invalid node id / save on error 路径）
      保留  会话标记（MAA Process Start / Close log）
      丢弃  全部 TRC / DBG 行
      丢弃  AgentClient::ctx_event_sink 的事件回显
      丢弃  NextList.* / Recognition.* / Action.* 事件
      折叠  连续重复的同类警告（例如 "Wrong ocr_result size" 一次能刷几百行）
      去重  同一事件被主进程 / Agent 进程重复投递的记录（时间差 <= 20ms）
      改写  事件行压成 `<时间>  <任务|节点> <状态>  <名字>` 的单行形式

    改写前会把原始日志 gzip 归档到 debug\archive\，不会丢任何信息
    （实测 8.9 MB 的原始日志压缩后约 380 KB）。

.PARAMETER Root
    项目根目录。默认取本脚本的上一级目录。

.PARAMETER RetainDays
    debug\archive\ 下归档文件的保留天数，默认 7。设为 0 或负数表示不清理。

.PARAMETER IncludeRecognition
    额外保留每次识别的结果行（Recognizer.cpp ... reco [result=...]）。
    排查"识别不到 / 识别错位置"时有用，但行数会明显增加。默认关闭。

.PARAMETER NoArchive
    不归档原始日志。不建议使用，除非你确定原始日志已另有备份。

.PARAMETER DryRun
    只统计、只报告，不改写任何文件。

.PARAMETER Report
    把本次统计结果写到指定文件，方便自动化流程读取。

.EXAMPLE
    .\maintainer\tools\compact_log.ps1 -DryRun
    先看一眼能压掉多少，不动文件。

.EXAMPLE
    .\maintainer\tools\compact_log.ps1 -IncludeRecognition
    精简时保留识别结果行，适合正在调识别参数的时候。

.NOTES
    必须在 mxu.exe 未运行时执行——日志文件被占用时改写会破坏文件。
    脚本检测到 mxu.exe 在跑会直接跳过（不报错，保证启动流程不被阻断）。

    文件必须以 UTF-8 with BOM 保存，否则 Windows PowerShell 5.1 会按 ANSI 代码页
    解析本文件里的中文注释而报语法错误。
#>
[CmdletBinding()]
param(
    [string] $Root = (Split-Path -Parent (Split-Path -Parent $PSScriptRoot)),
    [int]    $RetainDays = 7,
    [switch] $IncludeRecognition,
    [switch] $NoArchive,
    [switch] $DryRun,
    [string] $Report,
    [switch] $SkipLockCheck
)

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'

# ---------------------------------------------------------------------------
# 匹配规则
# ---------------------------------------------------------------------------

# 决定「保不保留」的正则
$keepPattern =
    '\[(WRN|ERR|FATAL)\]' +
    '|\[msg=(?:Tasker\.Task\.(?:Starting|Succeeded|Failed)' +
    '|Node\.PipelineNode\.(?:Starting|Succeeded|Failed))\]' +
    '|MAA Process Start|Close log' +
    '|save on error to' +
    '|Task timeout|invalid node id|PipelineTask bad next'

if ($IncludeRecognition) {
    $keepPattern += '|Recognizer\.cpp\]\[L\d+\]\[MaaNS::TaskNS::Recognizer::recognize\] reco \[result='
}

# 已经精简过的行也要认得出来——否则对同一个文件跑第二遍会把事件时间线清空。
$keepPattern +=
    '|^\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}\.\d{3}  (任务|  节点)' +
    '|^\s+\.\.\. 同类警告共 \d+ 条，已折叠'

$KeepRe = [regex]::new($keepPattern, 'Compiled')

# 事件行（EventDispatcher 的 !!!OnEventNotify!!!）
$EventRe = [regex]::new(
    '^\[(?<ts>\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}\.\d{3})\]\[(?:INF|TRC|DBG)\]\[Px\d+\]\[Tx\d+\]' +
    '\[Utils/EventDispatcher\.hpp\]\[L\d+\]\[MaaNS::EventDispatcher::notify\] ' +
    '!!!OnEventNotify!!! \[handle=(?:true|false)\] \[msg=(?<msg>[^\]]+)\] \[details=(?<det>\{.*\})\]\s*$',
    'Compiled')

# 形状归一化：剥掉「时间戳 + 级别 + 线程」和「源文件 + 行号 + 符号」
$ShapePrefix = [regex]::new(
    '^\[\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}\.\d{3}\]\[(?:WRN|ERR|FATAL)\]\[[^\]]*\]\[[^\]]*\]',
    'Compiled')
$ShapeHead = [regex]::new('^\[[^\]]*\](\[L\d+\])?(\[[^\]]*\])?:?\s*', 'Compiled')

# 行首时间戳（用于取时间 / 生成去重键）
$TsRe  = [regex]::new('^\[?(\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}\.\d{3})', 'Compiled')
$KeyRe = [regex]::new('^\[?\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}\.\d{3}\]?\s*', 'Compiled')
$PxRe  = [regex]::new('\[Px\d+\]\[Tx\d+\]', 'Compiled')

# 事件名 -> 短标签
$TagMap = @{
    'Tasker.Task.Starting'        = '任务    开始'
    'Tasker.Task.Succeeded'       = '任务    完成'
    'Tasker.Task.Failed'          = '任务    失败 <<<'
    'Node.PipelineNode.Starting'  = '  节点  开始'
    'Node.PipelineNode.Succeeded' = '  节点  完成'
    'Node.PipelineNode.Failed'    = '  节点  失败 <<<'
}

# 去重窗口：主进程与 Agent 进程投递同一事件的时间差（毫秒）
$DedupWindowMs = 20

# ---------------------------------------------------------------------------
# 辅助函数
# ---------------------------------------------------------------------------

function Test-MxuRunning {
    return [bool](Get-Process -Name 'mxu' -ErrorAction SilentlyContinue)
}

function Test-FileWritable {
    <#
        判断日志文件当前能不能安全改写。
        比"看进程在不在"更准：MaaFramework 只在运行期间持有 maa.log 的写句柄，
        轮转出来的 maa.bak.log 是已关闭的，即使 mxu 在跑也可以安全精简。
    #>
    param([string] $Path)
    try {
        $fs = [System.IO.File]::Open(
            $Path,
            [System.IO.FileMode]::Open,
            [System.IO.FileAccess]::ReadWrite,
            [System.IO.FileShare]::None)
        $fs.Close()
        $fs.Dispose()
        return $true
    }
    catch {
        return $false
    }
}

function ConvertTo-CompactLine {
    <# 事件行压成单行；其它行原样去掉尾部空白 #>
    param([string] $Line)

    $m = $EventRe.Match($Line)
    if ($m.Success) {
        $msg  = $m.Groups['msg'].Value
        $det  = $m.Groups['det'].Value
        $name = ''
        $nm = [regex]::Match($det, '"(?:entry|name)":"([^"]+)"')
        if ($nm.Success) { $name = $nm.Groups[1].Value }

        $tag = $TagMap[$msg]
        if (-not $tag) { $tag = $msg }

        if ($name) {
            return ('{0}  {1}  {2}' -f $m.Groups['ts'].Value, $tag, $name)
        }
        return ('{0}  {1}' -f $m.Groups['ts'].Value, $tag)
    }

    return $Line.TrimEnd()
}

function Get-LineShape {
    <# 只保留「消息本体」，用于判断若干条警告是否属于同一类 #>
    param([string] $Line)

    if ($Line -notmatch '\[(WRN|ERR|FATAL)\]') { return $Line }

    $s = $ShapePrefix.Replace($Line, '')
    $s = $ShapeHead.Replace($s, '')
    $i = $s.IndexOf(' [')
    if ($i -ge 0) { $s = $s.Substring(0, $i) }
    return $s.Trim()
}

function Get-LineTimestamp {
    param([string] $Line)
    $m = $TsRe.Match($Line)
    if ($m.Success) { return $m.Groups[1].Value }
    return ''
}

function Get-DedupKey {
    <# 去掉时间戳与线程号后的比较键 #>
    param([string] $Line)
    $k = $KeyRe.Replace($Line, '')
    return $PxRe.Replace($k, '')
}

function Compress-Archive-Gzip {
    <# 用 GZip 归档一个文件，返回归档路径 #>
    param([string] $Path, [string] $ArchiveDir)

    if (-not (Test-Path -LiteralPath $ArchiveDir)) {
        New-Item -ItemType Directory -Path $ArchiveDir -Force | Out-Null
    }
    $stamp = (Get-Item -LiteralPath $Path).LastWriteTime.ToString('yyyyMMdd-HHmmss')
    $dest  = Join-Path $ArchiveDir ("{0}.{1}.log.gz" -f (Split-Path $Path -Leaf), $stamp)

    $in = [System.IO.File]::OpenRead($Path)
    try {
        $out = [System.IO.File]::Create($dest)
        try {
            $gz = New-Object System.IO.Compression.GZipStream(
                $out, [System.IO.Compression.CompressionMode]::Compress, $true)
            try     { $in.CopyTo($gz) }
            finally { $gz.Dispose() }
        }
        finally { $out.Dispose() }
    }
    finally { $in.Dispose() }

    return $dest
}

function Invoke-LogCompaction {
    <# 精简单个日志文件，返回统计信息 #>
    param([string] $Path)

    $raw = [System.IO.File]::ReadAllLines($Path, [System.Text.Encoding]::UTF8)

    # --- 第 1 遍：过滤 + 转成精简行 ---------------------------------------
    $rows = New-Object System.Collections.Generic.List[object]
    foreach ($line in $raw) {
        if ($line.StartsWith('# ')) { continue }         # 上一轮写进去的说明头
        if (-not $KeepRe.IsMatch($line)) { continue }    # 不在保留清单里

        $isWarn = ($line -match '\[(WRN|ERR|FATAL)\]')
        $shape = ''
        if ($isWarn) { $shape = Get-LineShape $line }
        $rows.Add([pscustomobject]@{
            IsWarn = $isWarn
            Shape  = $shape
            Text   = (ConvertTo-CompactLine $line)
        })
    }

    # --- 第 2 遍：连续同类警告折叠 ---------------------------------------
    $folded = New-Object System.Collections.Generic.List[string]
    $i = 0
    while ($i -lt $rows.Count) {
        $row = $rows[$i]
        if (-not $row.IsWarn) {
            [void]$folded.Add($row.Text)
            $i++
            continue
        }

        $j = $i
        while (($j + 1) -lt $rows.Count -and $rows[$j + 1].IsWarn -and $rows[$j + 1].Shape -eq $row.Shape) {
            $j++
        }
        $count = $j - $i + 1
        [void]$folded.Add($row.Text)
        if ($count -gt 1) {
            $lastTs = Get-LineTimestamp $rows[$j].Text
            $tail = ''
            if ($lastTs) { $tail = "（截至 $lastTs）" }
            [void]$folded.Add(('                        ... 同类警告共 {0} 条，已折叠{1}' -f $count, $tail))
        }
        $i = $j + 1
    }

    # --- 第 3 遍：双进程重复投递去重（同一键且时间差 <= 20ms） ------------
    $out  = New-Object System.Collections.Generic.List[string]
    $seen = @{}
    foreach ($l in $folded) {
        $tsStr = Get-LineTimestamp $l
        $key   = Get-DedupKey $l

        if ($tsStr -and $seen.ContainsKey($key)) {
            $gap = ([datetime]::ParseExact($tsStr, 'yyyy-MM-dd HH:mm:ss.fff', $null) -
                    $seen[$key]).TotalMilliseconds
            if ($gap -ge 0 -and $gap -le $DedupWindowMs) { continue }
        }
        if ($tsStr) {
            $seen[$key] = [datetime]::ParseExact($tsStr, 'yyyy-MM-dd HH:mm:ss.fff', $null)
        }
        [void]$out.Add($l)
    }

    # 内容是否真的变了？（只丢了说明头也算"没变"，避免每次启动都重写一遍）
    $before = New-Object System.Collections.Generic.List[string]
    foreach ($l in $raw) {
        if (-not $l.StartsWith('# ')) { [void]$before.Add($l) }
    }
    $changed = $true
    if ($before.Count -eq $out.Count) {
        $changed = (($before -join "`n") -ne ($out -join "`n"))
    }

    return [pscustomobject]@{
        RawLines = $raw.Count
        NewLines = $out.Count
        Changed  = $changed
        Output   = $out
    }
}

# ---------------------------------------------------------------------------
# 主流程
# ---------------------------------------------------------------------------

$reportLines = New-Object System.Collections.Generic.List[string]
function Say([string] $s) {
    [void]$script:reportLines.Add($s)
    Write-Host $s
}

Say '=== BD2MAA debug 日志精简 ==='
Say ("时间      : {0}" -f (Get-Date).ToString('yyyy-MM-dd HH:mm:ss'))
Say ("项目根目录: {0}" -f $Root)
Say ("参数      : DryRun={0}  IncludeRecognition={1}  NoArchive={2}  RetainDays={3}" -f `
        [bool]$DryRun, [bool]$IncludeRecognition, [bool]$NoArchive, $RetainDays)
Say ''

if (Test-MxuRunning) {
    Say '[提示] 检测到 mxu.exe 正在运行。'
    Say '       正在被写入的日志会自动跳过，轮转出来的备份日志照常精简。'
    Say ''
}
if ($true) {
    $debugDir = Join-Path $Root 'debug'
    if (-not (Test-Path -LiteralPath $debugDir)) {
        Say "[跳过] 未找到目录：$debugDir"
    }
    else {
        $targets = @()
        foreach ($n in @('maa.log', 'maa.bak.log')) {
            $p = Join-Path $debugDir $n
            if (Test-Path -LiteralPath $p -PathType Leaf) { $targets += $p }
        }

        if ($targets.Count -eq 0) {
            Say '[跳过] 没有找到 maa.log / maa.bak.log。'
        }

        foreach ($path in $targets) {
            $name   = Split-Path $path -Leaf
            $before = (Get-Item -LiteralPath $path).Length
            Say ("--- {0} ---" -f $name)

            if (-not $SkipLockCheck -and -not (Test-FileWritable $path)) {
                Say ("  原始    : {0,12:N0} 字节" -f $before)
                Say '  [跳过]  文件正被占用（MXU 或本次运行仍在写日志）。'
                Say '          等下次启动时会被自动精简。'
                Say ''
                continue
            }

            Say ("  原始    : {0,12:N0} 字节" -f $before)

            try {
                $r = Invoke-LogCompaction -Path $path
            }
            catch {
                Say ("  [失败] {0}" -f $_.Exception.Message)
                continue
            }

            $after   = [System.Text.Encoding]::UTF8.GetByteCount(($r.Output -join "`r`n"))
            $linePct = 0.0
            if ($r.RawLines -gt 0) { $linePct = 100.0 * (1 - $r.NewLines / [double]$r.RawLines) }
            $bytePct = 0.0
            if ($before -gt 0) { $bytePct = 100.0 * (1 - $after / [double]$before) }

            Say ("  精简后  : {0,12:N0} 字节   ({1:N0} 行 -> {2:N0} 行)" -f $after, $r.RawLines, $r.NewLines)
            Say ("  下降    : 行数 {0:N1}%   体积 {1:N1}%" -f $linePct, $bytePct)

            if (-not $r.Changed) {
                Say '  [无需处理] 已经是精简格式。'
                Say ''
                continue
            }

            if ($DryRun) {
                Say '  [DryRun] 未改写文件。'
            }
            else {
                if (-not $NoArchive) {
                    $gzp   = Compress-Archive-Gzip -Path $path -ArchiveDir (Join-Path $debugDir 'archive')
                    $gzLen = (Get-Item -LiteralPath $gzp).Length
                    Say ("  原始归档: {0}  ({1:N0} 字节, 为原始的 {2:N1}%)" -f `
                            (Split-Path $gzp -Leaf), $gzLen, (100.0 * $gzLen / $before))
                }

                # 注意：数组字面量里不要写 'a' + ('b' * 2) 这种表达式——
                # PowerShell 的逗号优先级高于加号，会被解析成「字符串 + 数组」，
                # 结果所有元素被空格拼成一个字符串。每个元素都要单独加括号。
                $header = @(
                    ('# ' + ('=' * 61)),
                    ('# BD2MAA 精简日志   (maintainer\tools\compact_log.ps1 生成)'),
                    ("# 生成时间  : {0}" -f (Get-Date).ToString('yyyy-MM-dd HH:mm:ss')),
                    ("# 原始      : {0:N0} 字节 / {1:N0} 行" -f $before, $r.RawLines),
                    ("# 精简后    : {0:N0} 字节 / {1:N0} 行" -f $after, $r.NewLines),
                    ('# 保留内容  : 警告/错误 + 任务边界 + 节点边界 + 失败标记'),
                    ('# 完整原始日志: debug\archive\'),
                    ('# ' + ('=' * 61))
                )
                $utf8 = New-Object System.Text.UTF8Encoding($false)
                [System.IO.File]::WriteAllLines($path, [string[]]($header + $r.Output), $utf8)
                Say '  已改写  : OK'
            }
            Say ''
        }

        # 归档目录清理
        if ($RetainDays -gt 0 -and -not $DryRun) {
            $archiveDir = Join-Path $debugDir 'archive'
            if (Test-Path -LiteralPath $archiveDir) {
                $cutoff = (Get-Date).AddDays(-$RetainDays)
                $stale  = @(Get-ChildItem -LiteralPath $archiveDir -File -ErrorAction SilentlyContinue |
                            Where-Object { $_.LastWriteTime -lt $cutoff })
                foreach ($f in $stale) {
                    Remove-Item -LiteralPath $f.FullName -Force -ErrorAction SilentlyContinue
                }
                Say ("归档清理: 删除 {0} 个超过 {1} 天的归档" -f $stale.Count, $RetainDays)
            }
        }
    }
}

Say ''
Say '完成。'

if ($Report) {
    $dir = Split-Path $Report -Parent
    if ($dir -and -not (Test-Path -LiteralPath $dir)) {
        New-Item -ItemType Directory -Path $dir -Force | Out-Null
    }
    Set-Content -LiteralPath $Report -Value ($reportLines -join "`r`n") -Encoding UTF8
}

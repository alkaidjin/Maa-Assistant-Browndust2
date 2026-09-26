# BD2MAA-Updater.ps1
# =============================================================================
# BD2MAA 启动器 / 更新助手（纯 PowerShell 实现，零依赖，无需 Python）
#
# 功能：
#   1. 软件开启时自动检测 GitHub Releases 是否有新版本。
#   2. 发现新版本弹出选择对话框（含版本号与更新日志）。20 秒无操作自动跳过更新并进入软件，防止无人值守时卡在窗口。
#   3. 用户选“一键更新”：前台显示下载进度 -> 下载 -> 自动覆盖旧版本。
#      （更新时会保留用户的 config/ 配置：已有配置不覆盖，仅新增缺失的默认配置；
#        更新完还会按 retired_files.json 删掉旧版遗留的说明文档 / 教学视频）
#      更新全程有三道校验，任何一道不过都不会说“更新完成”：
#        [装前] 解压出来的包内 interface.json 版本号必须等于 release 的 tag，且必须含 mxu.exe；
#        [装中] 逐文件重试 3 次；失败的**具体文件名**会被列出来（不再是一个笼统的报错）；
#        [装后] 复核磁盘上的 interface.json 版本号 == tag，且关键文件都在。
#      另外：更新前若 mxu.exe / go-service.exe 还在运行，会先提示关闭它们
#      （被占用的文件覆盖会失败，那正是“更新完版本号却没变/装了一半”的根因）。
#   4. 用户选“暂不更新”、或已是最新、或检测失败：直接启动 mxu.exe。
#
# 可回溯性（v26.09.11+）：更新全程写 debug/updater.log（BASE / 当前版本 / 缓存状态 /
#   选中的 release 与资产 / 逐个下载源的结果与字节数 / 包身份校验 / 覆盖失败文件清单 /
#   装前装后版本号 / 最终结论）。用户报「更新完版本号没变」时，让他把这份日志发回来即可定位。
# 连不上 GitHub 时**不再完全静默**：日志会记下原因，并（最多每天一次）弹一条提示说明
#   「本次没有做任何更新，当前仍是 vX」，附发布页入口。
#
# 最新版本的取法（v26.09.11 起）：先取 /releases/latest；若它还没有可用的 .zip
#   （刚建 release、资产还在上传），退回到最近若干 release 里“版本最高且有可用 zip”的那个。
#
# 启动时的“家务”（全部失败静默、不阻断启动）：
#   - 主流程一开始：清理旧版遗留的说明文档与教学视频（retired_files.json 清单，幂等，
#     任何分支都会执行到；开发树存在 .git 时整体跳过）
#   - Launch-Mxu 里：自愈 launcher.bat 的编码 / 恢复 mxu.exe 图标 /
#     重建 MaaBd2.lnk / 精简并清理 debug 日志
#
# 用法：
#   .\BD2MAA-Updater.ps1            # 正常启动（检测更新 -> 弹窗 -> 启动 mxu）
#   .\BD2MAA-Updater.ps1 -Force     # 忽略缓存，强制重新检测
#   .\BD2MAA-Updater.ps1 -Demo      # 演示弹窗（即使已最新也弹）
#   .\BD2MAA-Updater.ps1 -Test      # 仅打印信息，不弹窗、不启动、不下载
#   .\BD2MAA-Updater.ps1 -Repair    # 修复模式：不管版本是否相同，重新下载最新包并覆盖安装
# =============================================================================

param(
    [switch]$Demo,
    [switch]$Force,
    [switch]$Test,
    [switch]$Repair
)

Add-Type -AssemblyName System.Windows.Forms
Add-Type -AssemblyName System.Drawing
[System.Windows.Forms.Application]::EnableVisualStyles()

# ZIP 校验用到的程序集：**加载失败也必须能继续启动**（用 try/catch 兜住，
# 并在 Test-ValidZip 里自动降级为"字节数 + PK 魔数"两级校验）。
$script:ZipArchiveAvailable = $false
try {
    Add-Type -AssemblyName System.IO.Compression -ErrorAction Stop
    $script:ZipArchiveAvailable = $true
} catch { $script:ZipArchiveAvailable = $false }

# ----------------------------------------------------------------------------
# 路径
# ----------------------------------------------------------------------------
$BASE       = Split-Path -Parent $MyInvocation.MyCommand.Definition
$CACHE_FILE = Join-Path $BASE 'updater_cache.json'
$CONFIG_FILE= Join-Path $BASE 'updater_config.json'
$MXU        = Join-Path $BASE 'mxu.exe'
$INTERFACE  = Join-Path $BASE 'interface.json'

# ----------------------------------------------------------------------------
# 默认配置（可被 updater_config.json 覆盖）
# ----------------------------------------------------------------------------
$DEFAULTS = @{
    repo                   = 'alkaidjin/Maa-Assistant-Browndust2'
    check_interval_hours   = 6
    asset_keywords_include = @('MABd2', 'win', 'BD2', 'Browndust')
    asset_keywords_exclude = @('source code', 'src', 'debug', 'linux', 'macos', 'darwin')
    protected_dirs         = @('config')   # 更新时这些目录的"已有文件"不覆盖
    download_dir           = 'updates'
    log_retention_days     = 7             # debug/ 日志与调试截图的保留天数
    open_folder_after      = $false
    # 国内访问 GitHub Releases 普遍被严重限速（实测有时仅 10~30 KB/s）。
    # 这份列表是**自动重试链**：原 URL 永远放在最后一个做兜底，前面按顺序尝试镜像前缀。
    # 镜像 = "https://<前缀>/" + 原 URL 去掉 https://。
    #
    # 2026-09-11 实测（对 release 资产直链）：
    #   - ghps.cc            ❌ 已失效。返回 HTTP 206 + 4181 字节的 text/html 错误页，
    #                           而 WebClient 不报错 → 半截 HTML 会被误当成 zip 下载成功，
    #                           直到解压才炸（"找不到中央目录结尾记录"）。已从默认列表移除。
    #   - mirror.ghproxy.com ❌ 连接超时，已移除。
    #   - ghfast.top         ✅ 返回正确 zip（PK\x03\x04，字节数与 asset 一致）
    #   - gh-proxy.com       ✅ 同上
    # 用户仍可在 updater_config.json 里替换/追加（公司 HTTP 代理、自建 OSS 反代等，格式同上），
    # 例如 "https://cdn.your-corp.com/gh"。留空数组 [] 可关闭镜像、只用官方源。
    mirror_url_prefixes    = @(
        'https://ghfast.top',
        'https://gh-proxy.com'
    )
}

$cfg = @{} + $DEFAULTS
if (Test-Path $CONFIG_FILE) {
    try {
        $j = Get-Content $CONFIG_FILE -Raw -Encoding UTF8 | ConvertFrom-Json
        $j.PSObject.Properties | ForEach-Object { $cfg[$_.Name] = $_.Value }
    } catch { }
}

# ----------------------------------------------------------------------------
# TLS：GitHub 早已强制 TLS 1.2。老机器上的 Windows PowerShell 5.1 默认只开 Ssl3|Tls，
#   不显式打开 Tls12 会直接连不上 api.github.com —— 表现为「更新检查毫无动静、版本号
#   永远停在旧版」，而旧代码把这种失败静默吞掉了。这里只做「补开」，不动其它协议位。
# ----------------------------------------------------------------------------
try {
    [Net.ServicePointManager]::SecurityProtocol = [Net.ServicePointManager]::SecurityProtocol -bor [Net.SecurityProtocolType]::Tls12
} catch {
    try { [Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12 } catch { }
}

# ----------------------------------------------------------------------------
# 工具函数
# ----------------------------------------------------------------------------
function Write-UpdaterLog($msg) {
    # 更新全程落盘（debug/updater.log）。为什么必须写：用户报「更新完版本号没变」时，
    # 弹窗本来是唯一线索，可静默分支根本不弹窗 —— 没有日志就只能靠猜。
    # 日志会被 Invoke-LogCleanup 按 log_retention_days 自然淘汰（持续追加所以时间戳常新）；
    # 任何写失败都绝不阻断启动。
    try {
        $lf = Join-Path $BASE 'debug\updater.log'
        $dir = Split-Path -Parent $lf
        if (-not (Test-Path -LiteralPath $dir)) { New-Item -ItemType Directory -Path $dir -Force | Out-Null }
        Add-Content -LiteralPath $lf -Value ('[{0}] {1}' -f (Get-Date -Format 'yyyy-MM-dd HH:mm:ss'), $msg) -Encoding UTF8 -ErrorAction Stop
    } catch { }
}

function Initialize-UpdaterLog {
    # 启动时做一次体积裁剪（>512 KB 只留最后 300 行），避免长期挂着把磁盘写满。
    try {
        $lf = Join-Path $BASE 'debug\updater.log'
        if ((Test-Path -LiteralPath $lf) -and ((Get-Item -LiteralPath $lf).Length -gt 512KB)) {
            $keep = @('...（日志已裁剪，仅保留最近 300 行）') + @(Get-Content -LiteralPath $lf -Tail 300 -ErrorAction Stop)
            Set-Content -LiteralPath $lf -Value $keep -Encoding UTF8 -ErrorAction Stop
        }
    } catch { }
}

function Read-CurrentVersion {
    if (Test-Path $INTERFACE) {
        try {
            $d = Get-Content $INTERFACE -Raw -Encoding UTF8 | ConvertFrom-Json
            return [string]$d.version
        } catch { }
    }
    return ''
}

function Parse-Version($s) {
    $s = ($s -replace '^[vV]', '').Trim()
    $parts = @()
    foreach ($p in $s.Split('.')) {
        $num = ''
        foreach ($c in $p.ToCharArray()) { if ($c -match '\d') { $num += $c } else { break } }
        if ($num -eq '') { $num = '0' }
        $parts += [int]$num
    }
    return $parts
}

function Compare-Version($a, $b) {
    $pa = Parse-Version $a; $pb = Parse-Version $b
    $n = [math]::Max($pa.Count, $pb.Count)
    for ($i = 0; $i -lt $n; $i++) {
        $xa = if ($i -lt $pa.Count) { $pa[$i] } else { 0 }
        $xb = if ($i -lt $pb.Count) { $pb[$i] } else { 0 }
        if ($xa -ne $xb) { return $xa - $xb }
    }
    return 0
}

function Read-Cache {
    if (Test-Path $CACHE_FILE) {
        try {
            $c = Get-Content $CACHE_FILE -Raw -Encoding UTF8 | ConvertFrom-Json
            $h = @{}
            $c.PSObject.Properties | ForEach-Object { $h[$_.Name] = $_.Value }
            if (-not $h['last_check_ts']) { $h['last_check_ts'] = 0 }
            return $h
        } catch { }
    }
    return @{ last_check_ts = 0; latest_tag = '' }
}

function Save-Cache($c) {
    try { $c | ConvertTo-Json -Compress | Set-Content $CACHE_FILE -Encoding UTF8 } catch { }
}

function Get-LatestRelease($repo) {
    $url = "https://api.github.com/repos/$repo/releases/latest"
    return Invoke-RestMethod -Uri $url `
        -Headers @{ 'User-Agent' = 'BD2MAA-Updater'; 'Accept' = 'application/vnd.github+json' } `
        -TimeoutSec 15
}

function Select-Asset($assets, $cfgRef) {
    $cands = @()
    foreach ($a in $assets) {
        $name = $a.name.ToLower()
        if ($name -like '*source code*') { continue }
        if (-not $name.EndsWith('.zip')) { continue }
        $skip = $false
        foreach ($k in $cfgRef['asset_keywords_exclude']) {
            if ($name.Contains($k.ToLower())) { $skip = $true; break }
        }
        if ($skip) { continue }
        $score = 0
        foreach ($k in $cfgRef['asset_keywords_include']) { if ($name.Contains($k.ToLower())) { $score++ } }
        $cands += [pscustomobject]@{ score = $score; asset = $a }
    }
    if ($cands.Count -eq 0) {
        foreach ($a in $assets) {
            if ($a.name.ToLower().EndsWith('.zip') -and $a.name.ToLower() -notlike '*source code*') {
                $cands += [pscustomobject]@{ score = 0; asset = $a }
            }
        }
    }
    $sorted = @($cands | Sort-Object -Descending score)
    if ($sorted.Count -gt 0) { return $sorted[0].asset }
    return $null
}

function Get-ReleaseList($repo, $count = 15) {
    $url = "https://api.github.com/repos/$repo/releases?per_page=$count"
    return Invoke-RestMethod -Uri $url `
        -Headers @{ 'User-Agent' = 'BD2MAA-Updater'; 'Accept' = 'application/vnd.github+json' } `
        -TimeoutSec 15
}

function Select-NewestReleaseWithAsset($repo, $cfgRef, $tryLatest) {
    # 返回 @{ release; asset } —— 保证 asset 一定存在，取不到则 $null。
    # 先认 /releases/latest（通常这里就命中）；它没有可用 zip 时（刚建 release、资产还在上传，
    # 实测 v26.09.4 差 25 分钟、v26.09.7 差 3.5 小时），退回到列表里版本最高且有可用 zip 的那个。
    if ($tryLatest) {
        $a = Select-Asset $tryLatest.assets $cfgRef
        if ($a) { return @{ release = $tryLatest; asset = $a } }
    }
    try { $list = Get-ReleaseList $repo } catch { return $null }
    $best = $null
    foreach ($r in $list) {
        if ($r.draft -or $r.prerelease) { continue }
        $a = Select-Asset $r.assets $cfgRef
        if (-not $a) { continue }
        if (-not $best -or (Compare-Version $r.tag_name $best.release.tag_name) -gt 0) {
            $best = @{ release = $r; asset = $a }
        }
    }
    return $best
}

function Find-ProjectRoot($base) {
    if (Test-Path (Join-Path $base 'interface.json')) { return $base }
    $sub = Get-ChildItem -Path $base -Directory |
        Where-Object { Test-Path (Join-Path $_.FullName 'interface.json') } |
        Select-Object -First 1
    if ($sub) { return $sub.FullName }
    return $base
}

# ----------------------------------------------------------------------------
# UI：发现新版本对话框（返回 $true=更新, $false=暂不更新）
# 20 秒无任何操作 → 自动视为「暂不更新」→ DialogResult=Cancel → caller 走 Launch-Mxu
#   设计动机：无人值守场景（挂机 / 远程启动 / 用户离开）不应被卡在更新窗口。
# ----------------------------------------------------------------------------
function Show-UpdateDialog($release, $current) {
    $TIMEOUT_SEC = 20   # 自动跳过倒计时（秒）；用户主动点按钮立即终止倒计时

    $form = New-Object System.Windows.Forms.Form
    $form.Text = 'BD2MAA · 发现新版本'
    $form.Size = New-Object System.Drawing.Size(580, 480)
    $form.StartPosition = 'CenterScreen'
    $form.FormBorderStyle = 'FixedDialog'
    $form.MaximizeBox = $false

    $header = New-Object System.Windows.Forms.Panel
    $header.BackColor = [System.Drawing.Color]::FromArgb(16, 185, 129)
    $header.Dock = 'Top'; $header.Height = 50
    $hlabel = New-Object System.Windows.Forms.Label
    $hlabel.Text = '🟢 发现新版本可用'
    $hlabel.ForeColor = [System.Drawing.Color]::White
    $hlabel.Font = New-Object System.Drawing.Font('Microsoft YaHei UI', 12, [System.Drawing.FontStyle]::Bold)
    $hlabel.Location = New-Object System.Drawing.Point(16, 12)
    $header.Controls.Add($hlabel)
    $form.Controls.Add($header)

    $info = New-Object System.Windows.Forms.Label
    $info.Location = New-Object System.Drawing.Point(16, 60)
    $info.Size = New-Object System.Drawing.Size(540, 30)
    $cur = if ($current) { $current } else { '未知' }
    $info.Text = "最新版本：$($release.tag_name)    当前版本：$cur"
    $form.Controls.Add($info)

    $logLabel = New-Object System.Windows.Forms.Label
    $logLabel.Location = New-Object System.Drawing.Point(16, 92)
    $logLabel.Text = '更新日志'
    $form.Controls.Add($logLabel)

    $box = New-Object System.Windows.Forms.TextBox
    $box.Location = New-Object System.Drawing.Point(16, 112)
    $box.Size = New-Object System.Drawing.Size(540, 250)
    $box.Multiline = $true
    $box.ScrollBars = 'Vertical'
    $box.ReadOnly = $true
    $box.Text = if ($release.body) { $release.body } else { '（无更新说明）' }
    $form.Controls.Add($box)

    # 倒计时提示（位于日志框与按钮之间）
    $lblTimer = New-Object System.Windows.Forms.Label
    $lblTimer.Location = New-Object System.Drawing.Point(16, 372)
    $lblTimer.Size = New-Object System.Drawing.Size(280, 24)
    $lblTimer.ForeColor = [System.Drawing.Color]::FromArgb(120, 120, 120)
    $lblTimer.Text = "无操作 ${TIMEOUT_SEC} 秒后自动跳过更新"
    $form.Controls.Add($lblTimer)

    $btnUpdate = New-Object System.Windows.Forms.Button
    $btnUpdate.Text = '⬇ 一键更新'
    $btnUpdate.Location = New-Object System.Drawing.Point(310, 408)
    $btnUpdate.Size = New-Object System.Drawing.Size(120, 36)
    $btnUpdate.BackColor = [System.Drawing.Color]::FromArgb(16, 185, 129)
    $btnUpdate.ForeColor = [System.Drawing.Color]::White
    $btnUpdate.DialogResult = 'OK'
    $form.Controls.Add($btnUpdate)

    $btnLater = New-Object System.Windows.Forms.Button
    $btnLater.Text = '暂不更新'
    $btnLater.Location = New-Object System.Drawing.Point(446, 408)
    $btnLater.Size = New-Object System.Drawing.Size(110, 36)
    $btnLater.DialogResult = 'Cancel'
    $form.Controls.Add($btnLater)

    $form.AcceptButton = $btnUpdate
    $form.CancelButton = $btnLater

    # 倒计时计时器（WinForms Timer：自动在 UI 线程跑，无需 Invoke；form 关闭后自动停）
    # 用 $script: 让 Tick handler 在自己的子作用域也能读到当前秒数
    $script:countdown = $TIMEOUT_SEC
    $timer = New-Object System.Windows.Forms.Timer
    $timer.Interval = 1000   # ms
    $timer.Add_Tick({
        $script:countdown--
        if ($script:countdown -le 0) {
            $timer.Stop()
            $form.DialogResult = 'Cancel'   # 与「暂不更新」按钮等价 → caller 走 Launch-Mxu
            $form.Close()
            return
        }
        $lblTimer.Text = "无操作 $script:countdown 秒后自动跳过更新"
    })

    # 用户主动点按钮 → 立即停 timer，避免关闭瞬间的尾随 tick 又触发 Close
    $stopTimer = { $timer.Stop() }
    $btnUpdate.Add_Click($stopTimer)
    $btnLater.Add_Click($stopTimer)

    $timer.Start()
    try {
        $result = $form.ShowDialog()
    } finally {
        $timer.Stop()    # 兜底：正常点击 / X 关 / 自动到时 三条路径都覆盖
    }
    return ($result -eq 'OK')
}

# ----------------------------------------------------------------------------
# UI：下载进度窗口（前台显示），返回 $null 成功 / 错误信息
# ----------------------------------------------------------------------------

# 给一条 GitHub 原 URL 按配置生成候选下载列表：镜像前缀在先，原 URL 兜底放最后。
# 镜像 = "<prefix>" + "/" + 原 URL 去掉 https://。空白前缀自动跳过。
function ConvertTo-DownloadUrls($origUrl, $cfgRef) {
    $origUrl = [string]$origUrl
    if ([string]::IsNullOrWhiteSpace($origUrl)) { return @() }

    $prefixes = @()
    if ($cfgRef.ContainsKey('mirror_url_prefixes')) {
        try { $prefixes = @($cfgRef['mirror_url_prefixes']) } catch { $prefixes = @() }
    }

    $trimmed = ''
    try {
        $u = [Uri]$origUrl
        $trimmed = $u.Host + $u.AbsolutePath  # e.g. github.com/alkaidjin/.../releases/.../MABd2.zip
    } catch {
        $i = $origUrl.IndexOf('://')
        $trimmed = if ($i -ge 0) { $origUrl.Substring($i + 3) } else { $origUrl }
    }
    $trimmed = $trimmed.TrimStart('/')

    $list = @()
    foreach ($p in $prefixes) {
        $pp = ([string]$p).Trim().TrimEnd('/')
        if ([string]::IsNullOrWhiteSpace($pp)) { continue }
        # 自动补 https://（用户填 bare host 也行）
        if ($pp -notmatch '^https?://') { $pp = "https://$pp" }
        $list += "$pp/$trimmed"
    }
    # 永远把官方放在最后做终极兜底
    $list += $origUrl
    return $list
}

# 把 URL 显示成简短可读文字（用于进度窗）
function Shorten-Url($u) {
    try {
        $uri = [Uri]$u
        $host = $uri.Host
        $path = $uri.AbsolutePath
        # 把常见镜像 host 缩写
        switch -Regex ($host) {
            '^ghps\.cc$'      { return 'ghps.cc' }
            '^ghfast\.top$'   { return 'ghfast.top' }
            '^gh-proxy\.com$' { return 'gh-proxy.com' }
            '^ghproxy\.net$'  { return 'ghproxy.net' }
            '^mirror\.ghproxy\.com$' { return 'mirror.ghproxy.com' }
            default {
                if ($path.Length -gt 30) { $path = '...' + $path.Substring($path.Length - 30) }
                return "$host$path"
            }
        }
    } catch { return $u }
}

# 一次下载尝试：弹出进度窗做单次下载；返回 $null = 成功 / 错误字符串 = 失败
function Start-DownloadOne($url, $dest, $totalBytes) {
    $form = New-Object System.Windows.Forms.Form
    $form.Text = 'BD2MAA 更新中'
    $form.Size = New-Object System.Drawing.Size(460, 170)
    $form.StartPosition = 'CenterScreen'
    $form.FormBorderStyle = 'FixedDialog'
    $form.ControlBox = $false

    $label = New-Object System.Windows.Forms.Label
    $label.Location = New-Object System.Drawing.Point(16, 12)
    $label.Size = New-Object System.Drawing.Size(420, 36)
    $label.Text = '正在下载新版本，请稍候…'
    $form.Controls.Add($label)

    $src = New-Object System.Windows.Forms.Label
    $src.Location = New-Object System.Drawing.Point(16, 44)
    $src.Size = New-Object System.Drawing.Size(420, 18)
    $src.ForeColor = [System.Drawing.Color]::FromArgb(96, 96, 96)
    $src.Text = ('源：' + (Shorten-Url $url))
    $form.Controls.Add($src)

    $bar = New-Object System.Windows.Forms.ProgressBar
    $bar.Location = New-Object System.Drawing.Point(16, 66)
    $bar.Size = New-Object System.Drawing.Size(420, 22)
    $bar.Style = 'Continuous'
    $form.Controls.Add($bar)

    $status = New-Object System.Windows.Forms.Label
    $status.Location = New-Object System.Drawing.Point(16, 100)
    $status.Size = New-Object System.Drawing.Size(420, 24)
    $form.Controls.Add($status)

    $global:dlDone = $false
    $global:dlError = $null

    $web = New-Object System.Net.WebClient
    $web.Headers.Add('User-Agent', 'BD2MAA-Updater')
    Register-ObjectEvent $web DownloadFileCompleted -Action {
        if ($eventArgs.Error) { $global:dlError = $eventArgs.Error.Message }
        $global:dlDone = $true
    } | Out-Null

    $timer = New-Object System.Windows.Forms.Timer
    $timer.Interval = 200
    $timer.Add_Tick({
        if (Test-Path $dest) {
            $sz = (Get-Item $dest).Length
            if ($totalBytes -gt 0) {
                $pct = [math]::Min(100, [int]($sz / $totalBytes * 100))
                $bar.Value = $pct
                $status.Text = ('{0:F1} / {1:F1} MB ({2}%)' -f ($sz / 1MB), ($totalBytes / 1MB), $pct)
            } else {
                $status.Text = ('已下载 {0:F1} MB' -f ($sz / 1MB))
            }
        }
        if ($global:dlDone) {
            $timer.Stop()
            $form.Close()
        }
    }) | Out-Null
    $timer.Start()

    try {
        $web.DownloadFileAsync($url, $dest)
        [System.Windows.Forms.Application]::Run($form)
    } finally {
        $timer.Stop(); $timer.Dispose()
        try { $web.CancelAsync() } catch { }
    }

    if ($global:dlError) { return $global:dlError }
    return $null
}

# 备注：单个源的下载结果由 Start-Download 统一记录（含实际字节数与校验结论），
# 这里不再重复写日志 —— 避免一次下载产生两行互相矛盾的口径。

# 校验下载结果是否是"完整且可解压的 zip"。
# 为什么需要：部分 GitHub 加速镜像对失效直链会返回 HTTP 206/200 + text/html 的几 KB 错误页，
#   WebClient 不会抛异常 —— 会被误判为"下载成功"，直到 Expand-Archive 才炸（报
#   "找不到中央目录结尾记录"，即 ZipArchive 的 .ctor 抛 End of Central Directory not found）。
# 三层校验：字节数 == 期望值 → PK 魔数 → 中央目录可读（Entries 非空）。
function Test-ValidZip($path, $expectedBytes) {
    try {
        if (-not (Test-Path -LiteralPath $path)) { return $false }
        $len = (Get-Item -LiteralPath $path).Length
        if ($expectedBytes -gt 0 -and $len -ne $expectedBytes) { return $false }
        if ($len -lt 22) { return $false }   # 空 zip 的 EOCD 也占 22 字节

        $fs = [System.IO.File]::OpenRead($path)
        try {
            $sig = New-Object byte[] 2
            [void]$fs.Read($sig, 0, 2)
            if ($sig[0] -ne 0x50 -or $sig[1] -ne 0x4B) { return $false }   # 'PK'
            # 第三层：中央目录可读（需要 System.IO.Compression 程序集）。
            # 若该类型在当前环境不可用，就跳过这一层，只靠"字节数 + PK 魔数"——
            # 绝不能因为类型缺失而把好文件误判成坏文件。
            if ($script:ZipArchiveAvailable) {
                $fs.Position = 0
                try {
                    $za = New-Object System.IO.Compression.ZipArchive($fs, [System.IO.Compression.ZipArchiveMode]::Read, $true)
                    $n = $za.Entries.Count
                    $za.Dispose()
                    if ($n -le 0) { return $false }
                } catch { return $false }
            }
        } finally { $fs.Dispose() }
        return $true
    } catch { return $false }
}

# 下载入口：接受 URL 数组（一个接一个试），返回 $null 成功 / 错误信息失败
function Start-Download($urls, $dest, $totalBytes) {
    if ($urls -is [string]) { $urls = @($urls) }
    $list = @($urls | Where-Object { $_ })
    if ($list.Count -eq 0) { return '无可用下载源（请检查 mirror_url_prefixes 配置）' }

    $lastErr = ''
    for ($i = 0; $i -lt $list.Count; $i++) {
        $u = $list[$i]
        # 切下一个源前先清残留，避免 .new 的竞争
        if (Test-Path -LiteralPath $dest) { Remove-Item -LiteralPath $dest -Force -ErrorAction SilentlyContinue }
        Write-UpdaterLog ('尝试下载源 [' + ($i + 1) + '/' + $list.Count + ']: ' + $u)
        $err = Start-DownloadOne $u $dest $totalBytes
        if (-not $err) {
            # 下载完成 ≠ 内容正确：镜像可能返回 HTML 错误页。校验通过才算成功，
            # 否则清掉残留、记下原因、继续尝试下一个源。
            $gotBytes = 0
            try { $gotBytes = (Get-Item -LiteralPath $dest).Length } catch { }
            Write-UpdaterLog ('  下载返回成功，实际 ' + $gotBytes + ' bytes（期望 ' + $totalBytes + '）')
            if (Test-ValidZip $dest $totalBytes) {
                Write-UpdaterLog ('  校验通过（字节数 + zip 格式）')
                return $null
            }
            $err = ('文件校验失败（字节数或 zip 格式不符，源可能返回了错误页）')
        }
        Write-UpdaterLog ('  该源失败: ' + $err)
        $lastErr = $err
        if (Test-Path -LiteralPath $dest) { Remove-Item -LiteralPath $dest -Force -ErrorAction SilentlyContinue }
        # 最后一源（官方）失败就不必继续
        if ($i -eq ($list.Count - 1)) { break }
    }
    return $lastErr
}

# ----------------------------------------------------------------------------
# 更新：解压 + 选择性覆盖（保留 config/ 已有文件）
# ----------------------------------------------------------------------------
function Copy-Update($src, $dst, $cfgRef) {
    # 返回失败清单（相对路径数组）。**不再中途 throw** —— 一个文件被占用/被杀软拦，
    # 其余文件照样覆盖完：这样不会再出现「更新到一半、版本号却没变」这种最难查的状态。
    # 每个文件最多试 3 次（杀软/索引器常常只锁住几百毫秒）。
    $protected = $cfgRef['protected_dirs']
    # 大文件（>=1 MB）走"先 .new 再 Move-Item"的原子覆盖；小文件直接 Copy-Item。
    # 这样中途断电 / 磁盘满时，mxu.exe 这类二进制不会被写到一半。
    $atomicThreshold = 1MB
    $failed = @()
    foreach ($f in (Get-ChildItem -Path $src -Recurse -File)) {
        $full = $f.FullName
        $rel  = $full.Substring($src.Length).TrimStart('\', '/')
        $relNorm = $rel.Replace('\', '/').ToLower()
        $target = Join-Path $dst $rel

        $isProtected = $false
        foreach ($p in $protected) {
            $pl = $p.Replace('\', '/').TrimEnd('/').ToLower()
            if ($relNorm -eq $pl -or $relNorm.StartsWith("$pl/")) { $isProtected = $true; break }
        }

        $td = Split-Path $target
        if (-not (Test-Path $td)) { New-Item -ItemType Directory -Path $td -Force | Out-Null }

        if ($isProtected) {
            # 仅新增不存在的默认配置；已有用户配置不覆盖
            if (-not (Test-Path $target)) { Copy-Item $full $target -Force }
            continue
        }

        $tmp = "$target.new"
        for ($try = 1; $try -le 3; $try++) {
            try {
                if ($f.Length -ge $atomicThreshold) {
                    # 原子覆盖：写到 .new，再 rename。失败时清理 .new 不留垃圾。
                    Copy-Item $full $tmp -Force
                    Move-Item -LiteralPath $tmp -Destination $target -Force
                } else {
                    Copy-Item $full $target -Force
                }
                break
            } catch {
                Remove-Item -LiteralPath $tmp -Force -ErrorAction SilentlyContinue
                if ($try -eq 3) {
                    $failed += $rel
                } else {
                    Start-Sleep -Milliseconds 600
                }
            }
        }
    }
    return $failed
}

function Test-PackageIdentity($srcRoot, $tagName) {
    # 装前校验：解压出来的这包，到底是不是 release 说的那个版本？
    # 为什么必须查：发布资产可能在 release 建立几分钟后才传上去（甚至事后被替换），
    # 用户若拿到「上一个版本改名过来的包」，装完就会出现“升级了但版本号没变”。
    # 这里宁可拒绝安装并让他重新下载，也不要把一个身份不符的包覆盖进软件目录。
    $res = @{ ok = $true; inner = ''; why = '' }
    $ifp = Join-Path $srcRoot 'interface.json'
    if (-not (Test-Path $ifp)) {
        $res.ok = $false; $res.why = '压缩包里没有 interface.json'
        return $res
    }
    try {
        $res.inner = [string]((Get-Content $ifp -Raw -Encoding UTF8 | ConvertFrom-Json).version)
    } catch {
        $res.ok = $false; $res.why = '包内 interface.json 解析失败'
        return $res
    }
    if (-not (Test-Path (Join-Path $srcRoot 'mxu.exe'))) {
        $res.ok = $false; $res.why = '压缩包里没有 mxu.exe（包不完整）'
        return $res
    }
    if ((Compare-Version $res.inner $tagName) -ne 0) {
        $res.ok = $false
        $res.why = "包内版本号是 $($res.inner)，与发布号 $tagName 不一致"
        return $res
    }
    return $res
}

function Test-InstalledVersion($base, $tagName) {
    # 装后复核：磁盘上的版本号与关键文件是否真的就位（"更新完成"这四个字必须有证据支撑）。
    $problems = @()
    $now = Read-CurrentVersion
    if ($now -ne $tagName) {
        $problems += "界面读到的版本号仍是 $now（期望 $tagName）"
    }
    foreach ($rel in @('mxu.exe', 'interface.json', 'launcher.bat', 'BD2MAA-Updater.ps1')) {
        if (-not (Test-Path (Join-Path $base $rel))) { $problems += "缺少 $rel" }
    }
    return $problems
}

function Stop-RunningMxu {
    # 更新前先关掉软件，否则：
    #   mxu.exe 被运行中的进程占用 → 覆盖失败；
    #   agent\go-service.exe 由 MXU 拉起、同样被占用 → 覆盖失败。
    # 被占用时的覆盖失败正是「更新完版本号却没变 / 装了一半」最现实的原因。
    $names = @('mxu', 'go-service')
    $running = @()
    foreach ($n in $names) { $running += @(Get-Process -Name $n -ErrorAction SilentlyContinue) }
    if ($running.Count -eq 0) { return $true }

    $r = [System.Windows.Forms.MessageBox]::Show(
        ("检测到软件正在运行。`n`n更新需要先关闭它 —— 程序文件被占用时覆盖会失败，`n" +
         "那正是「更新完版本号却没变 / 只装了一半」的常见原因。`n`n" +
         "点「确定」= 自动关闭并继续更新`n点「取消」= 暂不更新（本次不覆盖任何文件）"),
        'BD2MAA · 更新前需要先关闭软件', 'OKCancel', 'Warning')
    if ($r -ne 'OK') { return $false }

    foreach ($p in $running) { try { $p.CloseMainWindow() | Out-Null } catch { } }
    $deadline = (Get-Date).AddSeconds(8)
    while ((Get-Date) -lt $deadline) {
        $left = @()
        foreach ($n in $names) { $left += @(Get-Process -Name $n -ErrorAction SilentlyContinue) }
        if ($left.Count -eq 0) { break }
        Start-Sleep -Milliseconds 400
    }
    foreach ($n in $names) {
        foreach ($p in @(Get-Process -Name $n -ErrorAction SilentlyContinue)) {
            try { Stop-Process -Id $p.Id -Force -ErrorAction Stop } catch { }
        }
    }
    Start-Sleep -Milliseconds 800
    $left = 0
    foreach ($n in $names) { $left += @(Get-Process -Name $n -ErrorAction SilentlyContinue).Count }
    return ($left -eq 0)
}

# ----------------------------------------------------------------------------
# 清理「退休文件」：旧版随包派发、如今已改名 / 删除的说明文档与教学视频
# ----------------------------------------------------------------------------
# 动机：Copy-Update 只做「覆盖 + 新增」，从不删除。于是文档一旦改名（例：v26.09.7 的
# 四张注意事项图 → 合并成 PDF；v26.09.11 的 PDF → DOCX、教学视频换名），老用户目录里
# 就会新旧两份并存：既占体积，又让人不知道该看哪一份。
# 清单由随包派发的 retired_files.json 驱动 —— 更新完成后读到的自然就是新版清单。
# 安全边界（任何一条不满足就跳过该项）：
#   * 只删清单里的**显式相对路径**；含 .. 或绝对路径一律跳过（防目录穿越）
#   * 跳过受保护目录（config/，见 updater_config.json 的 protected_dirs）
#   * 清单文件缺失 / 解析失败：静默跳过，绝不阻断启动
#   * **存在 .git 的开发树整体跳过** —— 维护者本机的仓库里还有别的文件，
#     不能让它在这儿误删东西（用户包内不含 .git，所以对用户端无影响）
function Remove-RetiredFiles($base, $cfgRef) {
    if (Test-Path (Join-Path $base '.git')) { return }

    $listFile = Join-Path $base 'retired_files.json'
    if (-not (Test-Path $listFile)) { return }

    try {
        $j = Get-Content $listFile -Raw -Encoding UTF8 | ConvertFrom-Json
    } catch { return }
    if (-not $j -or -not $j.files) { return }

    $protected = $cfgRef['protected_dirs']
    foreach ($item in $j.files) {
        $rel = ([string]$item).Replace('\', '/').TrimStart('/')
        if (-not $rel -or $rel.Contains('..')) { continue }

        $low = $rel.ToLower()
        $skip = $false
        foreach ($p in $protected) {
            $pl = $p.Replace('\', '/').TrimEnd('/').ToLower()
            if ($low -eq $pl -or $low.StartsWith("$pl/")) { $skip = $true; break }
        }
        if ($skip) { continue }

        $full = Join-Path $base ($rel.Replace('/', '\'))
        if (Test-Path -LiteralPath $full -PathType Leaf) {
            Remove-Item -LiteralPath $full -Force -ErrorAction SilentlyContinue
        }
    }
}

function ReInject-XLaunch($base) {
    $p = Join-Path $base 'interface.json'
    if (-not (Test-Path $p)) { return }
    try {
        $d = Get-Content $p -Raw -Encoding UTF8 | ConvertFrom-Json
        if (-not $d.x_launch) {
            $d | Add-Member -MemberType NoteProperty -Name 'x_launch' -Value ([pscustomobject]@{
                launcher    = 'launcher.bat'
                auto_update = '软件启动时通过 launcher.bat 检测 GitHub 新版本并弹窗提示，支持一键下载（仅下载、不覆盖本地 config/resource 配置）'
                note        = '为保证用户获得新版本自动检测与更新弹窗，请引导用户使用 launcher.bat 而非直接双击 mxu.exe；x_ 前缀字段为扩展约定，MXU 会忽略未知字段。'
            })
            $d | ConvertTo-Json -Depth 10 | Set-Content $p -Encoding UTF8
        }
    } catch { }
}

function Apply-ExeIcon {
    # 让 mxu.exe 常驻本项目的程序图标。MXU 自更新会替换 exe，这里用“文件尺寸+时间戳”指纹
    # 判断是否需要重新打补丁（依赖 tools\rcedit-x64.exe + mxu.ico）。任何失败都静默、不阻断启动。
    try {
        $rc  = Join-Path $BASE 'tools\rcedit-x64.exe'
        $ico = Join-Path $BASE 'mxu.ico'
        if (-not (Test-Path -LiteralPath $rc))  { return }
        if (-not (Test-Path -LiteralPath $ico)) { return }
        if (-not (Test-Path -LiteralPath $MXU)) { return }
        if (Get-Process -Name 'mxu' -ErrorAction SilentlyContinue) { return }

        $cdir = Join-Path $BASE 'cache'
        if (-not (Test-Path -LiteralPath $cdir)) { New-Item -ItemType Directory -Path $cdir -Force | Out-Null }
        $stampFile = Join-Path $cdir 'icon_stamp.txt'

        $fi  = Get-Item -LiteralPath $MXU
        $ic  = Get-Item -LiteralPath $ico
        $sig = '{0}|{1}|{2}|{3}' -f $fi.Length, $fi.LastWriteTimeUtc.Ticks, $ic.Length, $ic.LastWriteTimeUtc.Ticks
        if ((Test-Path -LiteralPath $stampFile) -and ((Get-Content -LiteralPath $stampFile -Raw).Trim() -eq $sig)) { return }

        Start-Process -FilePath $rc -ArgumentList @("`"$MXU`"", '--set-icon', "`"$ico`"") -WindowStyle Hidden -Wait -ErrorAction Stop
        $fi2 = Get-Item -LiteralPath $MXU
        ('{0}|{1}|{2}|{3}' -f $fi2.Length, $fi2.LastWriteTimeUtc.Ticks, $ic.Length, $ic.LastWriteTimeUtc.Ticks) |
            Set-Content -LiteralPath $stampFile -Encoding ASCII
    } catch { }
}

function Invoke-LogCleanup {
    # 两步处理 debug/，避免长期使用无限增长、也让日志能用来排错。
    #
    # 第一步 精简：MaaFramework 的文件日志固定以 Trace 级别全量落盘（config\maa_option.json
    #   的 stdout_level 只管控制台，管不到文件），跑一轮日常任务就有 9~16 MB / 2~5 万行。
    #   tools\compact_log.ps1 会把它压成一份可读的「任务时间线」（约 99% 缩减），
    #   原始日志 gzip 归档到 debug\archive\。开关：log_compact（默认 true）。
    #   注意：正在被 MXU 写入的 maa.log 会被自动跳过，轮转出来的 maa.bak.log 照常精简。
    #
    # 第二步 清理：删除 debug/ 下超过保留天数的
    #   - mxu-web-YYYY-MM-DD.log / mxu-agent.log / maa.log / go-service.log …
    #   - MaaFramework 在 save_on_error 时写出的调试截图（*.png）
    #   - debug\archive\ 里的 gzip 归档
    # 保留窗口由 updater_config.json 的 log_retention_days 控制（默认 7 天）。
    #
    # 任何失败都静默、绝不阻断启动。
    try {
        $dbg = Join-Path $BASE 'debug'
        if (-not (Test-Path -LiteralPath $dbg)) { return }

        $days = 7
        if ($cfg.ContainsKey('log_retention_days')) {
            try { $days = [int]$cfg['log_retention_days'] } catch { $days = 7 }
        }

        # --- 第一步：日志精简 ---
        $doCompact = $true
        if ($cfg.ContainsKey('log_compact')) {
            try { $doCompact = [bool]$cfg['log_compact'] } catch { $doCompact = $true }
        }

        if ($doCompact) {
            $compactor = Join-Path $BASE 'tools\compact_log.ps1'
            if (Test-Path -LiteralPath $compactor) {
                # 注意：splatting 必须用哈希表（@{}）；用数组 @() 会变成位置传参，
                # 参数名被当成值，直接抛异常并被外层 catch 静默吞掉。
                $cargs = @{ Root = $BASE; RetainDays = $days }
                if ($cfg.ContainsKey('log_compact_keep_recognition')) {
                    try {
                        if ([bool]$cfg['log_compact_keep_recognition']) { $cargs['IncludeRecognition'] = $true }
                    } catch { }
                }
                # 在当前进程的子作用域里执行，省掉再拉一个 powershell.exe
                & $compactor @cargs *> $null
            }
        }

        if ($days -le 0) { return }

        # --- 第二步：按保留天数清理 ---
        $cutoff = (Get-Date).AddDays(-$days)
        Get-ChildItem -LiteralPath $dbg -Recurse -File -ErrorAction SilentlyContinue |
            Where-Object { $_.LastWriteTime -lt $cutoff } |
            ForEach-Object { Remove-Item -LiteralPath $_.FullName -Force -ErrorAction SilentlyContinue }
    } catch { }
}

function Repair-LauncherShortcut {
    # ---- 重建根目录的「MaaBd2.lnk」快捷方式 ----
    # 原因：发布 zip 里不打包 MaaBd2.lnk（不同用户的安装路径不同，写死路径的 lnk 在别人机器上
    #       是失效的）。新用户首次启动 launcher.bat 时，按当前 $BASE 自动生成；老用户的旧 lnk
    #       如果指向 launcher.bat 也直接覆盖重建——保证永远指向正确的 launcher.bat 与图标。
    # 与 Apply-ExeIcon 的区别：后者只 patch mxu.exe 的图标指纹，不重建快捷方式。
    try {
        if (-not (Test-Path $BASE)) { return }
        $ws  = New-Object -ComObject WScript.Shell -ErrorAction SilentlyContinue
        if (-not $ws) { return }
        $lnk = Join-Path $BASE 'MaaBd2.lnk'
        $bat = Join-Path $BASE 'launcher.bat'
        $ico = Join-Path $BASE 'mxu.ico'
        $s   = $ws.CreateShortcut($lnk)
        $s.TargetPath       = $bat
        $s.WorkingDirectory = $BASE
        $s.WindowStyle      = 1   # 1=正常窗口（避免 PowerShell 弹黑色控制台）
        if (Test-Path -LiteralPath $ico) { $s.IconLocation = "$ico,0" }
        $s.Description      = 'BD2MAA launcher (auto-update + log cleanup)'
        $s.Save()
    } catch { }
}

function Repair-LauncherBat {
    # ---- 自愈 launcher.bat 的编码（v26.09.7 事故后的运行时兜底）----
    # 背景：cmd 逐字节解析 .bat，遇到非 ASCII 字节（中文注释）会错位解码，把注释后半段
    #   当成命令执行 -> 刷「不是内部或外部命令，也不是可运行的程序或批处理文件」；
    #   严重时整行边界被打乱，脚本被截断，后面的行（含真正的 PowerShell 调用）根本不执行。
    #   LF 行尾会显著加剧（实测 LF+长中文注释可导致 rc=1 + 调用被吞）。
    #   根治 = 文件 100% ASCII + CRLF。打包器 tools\build_release_zip.py 有
    #   check_bat_crlf() 预警 + normalize_crlf() 强制规范；这里是包外的第二道防线：
    #   只在检测到「含非 ASCII 字节」时重写（这是真正的杀伤源），
    #   不因行尾问题改写 —— 避免误伤用户自己加过自定义行的 launcher.bat。
    try {
        $bat = Join-Path $BASE 'launcher.bat'
        if (-not (Test-Path -LiteralPath $bat)) { return }

        $bytes = [System.IO.File]::ReadAllBytes($bat)
        $bad = $false
        foreach ($b in $bytes) {
            if ($b -gt 127) { $bad = $true; break }
        }
        if (-not $bad) { return }

        $canon = @(
            '@echo off',
            'REM ============================================================',
            'REM  BD2MAA launcher  (recommended entry / zero dependency / no Python)',
            'REM  Check GitHub release, show dialog, download and overwrite, then run mxu.exe',
            'REM  Args: -Force force check / -Demo demo dialog / -Test dry-run only',
            'REM ============================================================',
            'cd /d "%~dp0"',
            'powershell -NoProfile -ExecutionPolicy Bypass -File "BD2MAA-Updater.ps1" %*'
        ) -join "`r`n"
        [System.IO.File]::WriteAllBytes($bat, [System.Text.Encoding]::ASCII.GetBytes($canon + "`r`n"))
    } catch { }
}

function Launch-Mxu {
    if (-not (Test-Path $MXU)) { return }

    Repair-LauncherBat      # 自愈被中文注释/编码污染过的 launcher.bat（幂等，纯 ASCII 时直接返回）
    Apply-ExeIcon           # MXU 自更新后自动恢复程序图标
    Repair-LauncherShortcut # 重建 MaaBd2.lnk（zip 不打包，按当前 $BASE 自动生成；幂等）
    Invoke-LogCleanup       # 清理 debug/ 下超过保留天数的日志与调试截图

    # 直接 CreateProcess 启动 mxu.exe（工作目录 = 仓库根目录）。
    # 注意：WMI / Win32_Process.Create 明确不用——它会以 LocalSystem 身份拉起游戏客户端，危险。
    try {
        Start-Process -FilePath $MXU -WorkingDirectory $BASE -ErrorAction Stop
        return
    } catch { }

    # 最后兜底（极少触发）：通过 Shell COM 的 ShellExecute 启动。
    try {
        $shell = New-Object -ComObject Shell.Application
        $shell.ShellExecute($MXU, '', $BASE, 'open', 1) | Out-Null
    } catch { }
}

# ----------------------------------------------------------------------------
# 主流程
# ----------------------------------------------------------------------------
try {
    Initialize-UpdaterLog
    $cache = Read-Cache
    $current = Read-CurrentVersion
    Write-UpdaterLog ('==== 启动器启动 ==== 参数=[' + (($PSBoundParameters.Keys | ForEach-Object { '-' + $_ }) -join ' ') + '] PS=' + $PSVersionTable.PSVersion.ToString())
    Write-UpdaterLog ('BASE=' + $BASE)
    Write-UpdaterLog ('当前版本=' + $current + '  缓存 latest_tag=' + $cache.latest_tag)
    # 清理旧版遗留的说明文档 / 教学视频（幂等、静默）。放在主流程最前面：
    # 之后的每个分支（-Test / 无网络 / 不更新 / 更新）都能覆盖到。
    Remove-RetiredFiles $BASE $cfg

    $iv = [timespan]::FromHours($cfg['check_interval_hours']).Ticks
    $needCheck = $Force -or $Demo -or $Test -or $Repair -or ((Get-Date).Ticks - $cache.last_check_ts) -gt $iv
    $minsSince = 0
    try { $minsSince = [int](((Get-Date).Ticks - [int64]$cache.last_check_ts) / [timespan]::TicksPerMinute) } catch { }
    Write-UpdaterLog ('needCheck=' + $needCheck + '（距上次检查 ' + $minsSince + ' 分钟 / 间隔 ' + $cfg['check_interval_hours'] + ' 小时）')

    $release = $null
    $asset = $null
    if ($needCheck) {
        try {
            $release = Get-LatestRelease $cfg['repo']
            # latest 必须真的带可用 zip 才认；否则回退到「版本最高且有可用 zip」的 release
            $pick = Select-NewestReleaseWithAsset $cfg['repo'] $cfg $release
            if ($pick) { $release = $pick.release; $asset = $pick.asset }
            $cache.last_check_ts = (Get-Date).Ticks
            $cache.latest_tag = $release.tag_name
            Save-Cache $cache
            Write-UpdaterLog ('检测到 release=' + $release.tag_name + '  资产=' + $(if ($asset) { $asset.name + ' (' + $asset.size + ' bytes)' } else { '无可用 zip' }))
        } catch {
            $release = $null
            $asset = $null
            Write-UpdaterLog ('[失败] 版本检测异常: ' + $_.Exception.Message)
            if ($_.Exception.Message -match '403|429|rate limit|禁止|禁止访问') {
                Write-UpdaterLog ('[提示] 疑似 GitHub API 未认证限流（60 次/小时/IP）或被网络策略拦截')
            }
        }
    }

    if ($Test) {
        Write-UpdaterLog ('-Test 模式：只打印不动作')
        Write-Host "== BD2MAA Updater 逻辑自检 =="
        Write-Host ("当前版本 (interface.json): {0}" -f $current)
        if ($release) {
            Write-Host ("GitHub 最新版本: {0}" -f $release.tag_name)
            if ($asset) {
                Write-Host ("选定资源: {0} ({1} bytes)" -f $asset.name, $asset.size)
                Write-Host ("下载地址: {0}" -f $asset.browser_download_url)
            } else {
                Write-Host "选定资源: 无（该 release 还没有可用 zip，将回退到发布页）"
            }
            Write-Host ("需更新: {0}" -f ((Compare-Version $release.tag_name $current) -gt 0))
        } else {
            Write-Host "无法获取最新版本（网络不可达 / GitHub 限流 / 需代理）"
        }
        return
    }

    # 无法检测更新：以前完全静默（用户会以为"更新过了、但版本号没变"）。
    # 现在至少留证据，并在"确实跑了检测却失败"时明确告知（同一天只打扰一次）。
    if (-not $release) {
        Write-UpdaterLog ('未取到版本信息 → 直接以旧版本启动（当前 ' + $current + '）；本次没有做任何更新')
        if ($needCheck) {
            $lastNotify = 0
            try { $lastNotify = [int64]$cache.last_notify_ts } catch { $lastNotify = 0 }
            if (((Get-Date).Ticks - $lastNotify) -gt [timespan]::FromHours(24).Ticks) {
                $cache.last_notify_ts = (Get-Date).Ticks
                Save-Cache $cache
                $r = [System.Windows.Forms.MessageBox]::Show(
                    ("没能连上 GitHub 检查更新（网络不可达 / 被限流 / 需要代理）。`n`n" +
                     "本次没有做任何更新，你现在的版本仍是 $current。`n`n" +
                     "点「确定」= 前往发布页手动下载`n点「取消」= 直接打开软件"),
                    'BD2MAA · 未能检查更新', 'OKCancel', 'Warning')
                if ($r -eq 'OK') { Start-Process ('https://github.com/' + $cfg['repo'] + '/releases/latest') }
                Write-UpdaterLog ('已提示用户"未能检查更新"')
            }
        }
        Launch-Mxu
        return
    }

    $hasUpdate = (Compare-Version $release.tag_name $current) -gt 0
    Write-UpdaterLog ('版本比较: 最新=' + $release.tag_name + ' 当前=' + $current + ' → hasUpdate=' + $hasUpdate)
    if ($Demo) { $hasUpdate = $true }
    if ($Repair) {
        $ask = [System.Windows.Forms.MessageBox]::Show(
            ("修复模式：将重新下载并覆盖安装 $($release.tag_name)（当前 $current）。`n`n" +
             "用途：上一次更新只装了一半、或感觉文件没更新时，用它把整个软件目录按该版本重装一遍。`n" +
             "config/ 里的用户配置不会被覆盖。`n`n继续？"),
            'BD2MAA · 修复安装', 'OKCancel', 'Question')
        if ($ask -ne 'OK') { Launch-Mxu; return }
        $hasUpdate = $true
    }

    if (-not $hasUpdate) { Launch-Mxu; return }

    # 新版本已发布但资产还没传完（实测可能差几分钟到几小时）：明确告知，不要静默跳过
    if (-not $asset) {
        [System.Windows.Forms.MessageBox]::Show(
            ("$($release.tag_name) 这个版本还没有可用的压缩包（发布资产可能还在上传）。`n`n" +
             "你可以稍后再启动一次本软件；也可以点「确定」前往发布页手动下载。"),
            '更新 · 暂无可下载的包', 'OKCancel', 'Information') | Out-Null
        Start-Process $release.html_url
        Launch-Mxu
        return
    }

    $doUpdate = Show-UpdateDialog $release $current
    Write-UpdaterLog ('更新弹窗结果: ' + $(if ($doUpdate) { '用户点了「一键更新」' } else { '跳过（点了「暂不更新」或 20 秒无操作自动关闭）' }))
    if (-not $doUpdate) { Launch-Mxu; return }

    # 更新前先关掉正在运行的软件：进程占用文件会让覆盖失败，产出“更新完版本号却没变”的半截状态
    if (-not (Stop-RunningMxu)) {
        Write-UpdaterLog ('用户拒绝了"关闭软件后再更新" → 取消更新，本次不覆盖任何文件')
        Launch-Mxu
        return
    }

    $ddir = Join-Path $BASE $cfg['download_dir']
    New-Item -ItemType Directory -Path $ddir -Force | Out-Null
    $dest = Join-Path $ddir $asset.name

    # 生成候选下载 URL：配置的镜像前缀按顺序尝试，官方原 URL 放最后兜底
    $urls = ConvertTo-DownloadUrls $asset.browser_download_url $cfg
    Write-UpdaterLog ('下载目标: ' + $dest + '  期望字节数=' + ([int]($asset.size)))
    Write-UpdaterLog ('候选下载源 ' + @($urls).Count + ' 个: ' + ((@($urls) | ForEach-Object { Shorten-Url $_ }) -join ' | '))
    $err = Start-Download $urls $dest ([int]($asset.size))
    if ($err) {
        Write-UpdaterLog ('[失败] 所有下载源均未成功: ' + $err)
        [System.Windows.Forms.MessageBox]::Show(
            ("下载失败：{0}`n请前往发布页手动下载。" -f $err), "更新失败", 'OK', 'Error')
        Start-Process $release.html_url
        # 清理失败下载留下的半截 zip，避免 updates/ 越来越胖
        Remove-Item -LiteralPath $dest -Force -ErrorAction SilentlyContinue
        Launch-Mxu
        return
    }

    # 解压 + 校验包身份 + 选择性覆盖 + 装后复核
    $tmp = Join-Path $env:TEMP ("BD2MAA_update_" + [guid]::NewGuid().ToString('N'))
    $problems = @()
    try {
        Write-UpdaterLog ('解压到: ' + $tmp)
        Expand-Archive -Path $dest -DestinationPath $tmp -Force
        $srcRoot = Find-ProjectRoot $tmp
        $pkgCount = @(Get-ChildItem -Path $srcRoot -Recurse -File -ErrorAction SilentlyContinue).Count
        Write-UpdaterLog ('解压完成，包根=' + $srcRoot + '，文件数=' + $pkgCount)

        # [装前] 包的身份必须对得上：包内 interface.json 版本 == release 的 tag，且含 mxu.exe
        $pkg = Test-PackageIdentity $srcRoot $release.tag_name
        Write-UpdaterLog ('[装前] 包身份校验: ok=' + $pkg.ok + ' 包内版本=' + $pkg.inner + ' 期望=' + $release.tag_name + $(if (-not $pkg.ok) { ' 原因=' + $pkg.why } else { '' }))
        if (-not $pkg.ok) {
            [System.Windows.Forms.MessageBox]::Show(
                ("这次下载到的包身份不符：$($pkg.why)。`n`n" +
                 "已取消覆盖，你的软件目录一个字都没动。`n" +
                 "多见于「该 release 的压缩包正在被替换 / 刚上传」—— 稍后再启动一次本软件重试即可；`n" +
                 "也可以点「确定」前往发布页手动下载。"),
                '更新 · 包身份不符', 'OKCancel', 'Warning') | Out-Null
            Start-Process $release.html_url
            Launch-Mxu
            return
        }

        $fail = @(Copy-Update $srcRoot $BASE $cfg)
        Write-UpdaterLog ('覆盖完成，失败文件 ' + $fail.Count + ' 个' + $(if ($fail.Count -gt 0) { ': ' + ($fail -join '、') } else { '' }))
        ReInject-XLaunch $BASE
        # 更新完成后立刻清理旧版遗留文件：retired_files.json 也刚被覆盖成新版，
        # 所以这一步用的就是本次发布的最新清单（清单之外的旧文件不动）。
        Remove-RetiredFiles $BASE $cfg

        # [装后] 复核：磁盘上的版本号与关键文件是否真的就位
        $problems = @(Test-InstalledVersion $BASE $release.tag_name)
        Write-UpdaterLog ('[装后] 复核: 磁盘版本=' + (Read-CurrentVersion) + ' 期望=' + $release.tag_name + ' 问题数=' + $problems.Count + $(if ($problems.Count -gt 0) { ': ' + ($problems -join ' / ') } else { '' }))
        if ($fail.Count -gt 0) {
            $problems = @('以下文件未能覆盖（多半被杀毒软件或其它程序占用）：' + ($fail -join '、')) + $problems
        }
    } catch {
        Write-UpdaterLog ('[失败] 解压或覆盖阶段异常: ' + $_.Exception.Message)
        Write-UpdaterLog ('  位置: 第 ' + $_.InvocationInfo.ScriptLineNumber + ' 行 :: ' + ($_.InvocationInfo.Line -replace "`r?`n", ' '))
        # 注意：$dest 会在 finally 里被清掉，所以这里不要再让用户"手动解压该文件"。
        [System.Windows.Forms.MessageBox]::Show(
            # 注意：多段字符串拼接后再 -f，必须把整个拼接括起来 —— `-f` 优先级高于 `+`，
            # 不括就只格式化最后一段，前面的 {0} 会原样显示成 "{0}"（本行就是踩过这个坑改的）。
            (("解压或覆盖失败：{0}`n`n可重新启动 launcher.bat 再试一次（会自动切换下载源）；`n" +
             "若反复出现，请先关闭软件（mxu.exe）与杀毒软件的实时防护，再用 `"launcher.bat -Repair`" 重新下载；`n" +
             "也可以点「确定」前往发布页手动下载：{1}") -f $_.Exception.Message, $release.html_url),
            "更新失败", 'OKCancel', 'Error') | Out-Null
        Start-Process $release.html_url
        Launch-Mxu
        return
    } finally {
        Remove-Item $tmp -Recurse -Force -ErrorAction SilentlyContinue
        Remove-Item $dest -Force -ErrorAction SilentlyContinue
    }

    if ($problems.Count -gt 0) {
        Write-UpdaterLog ('[结论] 更新未完成（目标 ' + $release.tag_name + '）：' + ($problems -join ' / '))
        [System.Windows.Forms.MessageBox]::Show(
            ("更新没有完整完成（本次目标是 $($release.tag_name)）：`n`n - " + ($problems -join "`n - ") +
             "`n`n建议：1) 先关闭本软件与杀毒软件的实时防护；2) 运行 `"launcher.bat -Repair`" 重新下载并覆盖；`n" +
             "3) 仍不行就前往发布页手动下载压缩包，解压到本软件目录覆盖。"),
            '更新未完成', 'OK', 'Error')
    } else {
        Write-UpdaterLog ('[结论] 更新完成，已应用到 ' + $release.tag_name)
        [System.Windows.Forms.MessageBox]::Show(
            ("更新完成！已应用到 $($release.tag_name)，你的 config/ 配置已保留。"),
            "更新完成", 'OK', 'Information')
    }
    Launch-Mxu

} catch {
    # 任何异常都不应阻断用户进入软件。
    # 但**绝不能再静默**：旧版这里什么都不说，用户看到的就是"跑了更新、版本号却没变"，
    # 本人也无从排查。现在写日志 + 给一条明确提示。
    try {
        Write-UpdaterLog ('[严重] 主流程未捕获异常: ' + $_.Exception.Message)
        Write-UpdaterLog ('  位置: 第 ' + $_.InvocationInfo.ScriptLineNumber + ' 行 :: ' + ($_.InvocationInfo.Line -replace "`r?`n", ' '))
        Write-UpdaterLog ('  当前版本=' + (Read-CurrentVersion) + '（若与最新版不一致，说明本次更新并未生效）')
        [System.Windows.Forms.MessageBox]::Show(
            (("启动器在检查 / 更新过程中出错了，本次可能没有完成更新。`n`n" +
             "错误：{0}`n`n当前版本仍是 {1}。详细信息已写入 debug\updater.log。`n`n" +
             "可以重新运行 launcher.bat 再试一次；也可用 `"launcher.bat -Repair`" 重新下载并覆盖。") -f $_.Exception.Message, (Read-CurrentVersion)),
            'BD2MAA · 启动器出错', 'OK', 'Warning')
    } catch { }
    try { Launch-Mxu } catch { }
}


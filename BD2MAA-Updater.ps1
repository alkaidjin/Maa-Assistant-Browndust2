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
#   4. 用户选“暂不更新”、或已是最新、或检测失败：直接启动 mxu.exe。
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
# =============================================================================

param(
    [switch]$Demo,
    [switch]$Force,
    [switch]$Test
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
# 工具函数
# ----------------------------------------------------------------------------
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
        $err = Start-DownloadOne $u $dest $totalBytes
        if (-not $err) {
            # 下载完成 ≠ 内容正确：镜像可能返回 HTML 错误页。校验通过才算成功，
            # 否则清掉残留、记下原因、继续尝试下一个源。
            if (Test-ValidZip $dest $totalBytes) { return $null }
            $err = ('文件校验失败（字节数或 zip 格式不符，源可能返回了错误页）')
        }
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
    $protected = $cfgRef['protected_dirs']
    # 大文件（>=1 MB）走"先 .new 再 Move-Item"的原子覆盖；小文件直接 Copy-Item。
    # 这样中途断电 / 磁盘满时，mxu.exe 这类二进制不会被写到一半。
    $atomicThreshold = 1MB
    Get-ChildItem -Path $src -Recurse -File | ForEach-Object {
        $full = $_.FullName
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
            return
        }

        if ($_.Length -ge $atomicThreshold) {
            # 原子覆盖：写到 .new，再 rename。失败时清理 .new 不留垃圾。
            $tmp = "$target.new"
            try {
                Copy-Item $full $tmp -Force
                Move-Item -LiteralPath $tmp -Destination $target -Force
            } catch {
                Remove-Item -LiteralPath $tmp -Force -ErrorAction SilentlyContinue
                throw
            }
        } else {
            Copy-Item $full $target -Force
        }
    }
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
    $cache = Read-Cache
    $current = Read-CurrentVersion
    # 清理旧版遗留的说明文档 / 教学视频（幂等、静默）。放在主流程最前面：
    # 之后的每个分支（-Test / 无网络 / 不更新 / 更新）都能覆盖到。
    Remove-RetiredFiles $BASE $cfg

    $iv = [timespan]::FromHours($cfg['check_interval_hours']).Ticks
    $needCheck = $Force -or $Demo -or $Test -or ((Get-Date).Ticks - $cache.last_check_ts) -gt $iv

    $release = $null
    if ($needCheck) {
        try {
            $release = Get-LatestRelease $cfg['repo']
            $cache.last_check_ts = (Get-Date).Ticks
            $cache.latest_tag = $release.tag_name
            Save-Cache $cache
        } catch {
            $release = $null
        }
    }

    if ($Test) {
        Write-Host "== BD2MAA Updater 逻辑自检 =="
        Write-Host ("当前版本 (interface.json): {0}" -f $current)
        if ($release) {
            Write-Host ("GitHub 最新版本: {0}" -f $release.tag_name)
        $asset = Select-Asset $release.assets $cfg
        if ($asset) {
            Write-Host ("选定资源: {0} ({1} bytes)" -f $asset.name, $asset.size)
                Write-Host ("下载地址: {0}" -f $asset.browser_download_url)
            } else {
                Write-Host "选定资源: 无（将回退到发布页）"
            }
            Write-Host ("需更新: {0}" -f ((Compare-Version $release.tag_name $current) -gt 0))
        } else {
            Write-Host "无法获取最新版本（网络不可达 / GitHub 限流 / 需代理）"
        }
        return
    }

    # 无法检测更新：直接进入软件
    if (-not $release) { Launch-Mxu; return }

    $hasUpdate = (Compare-Version $release.tag_name $current) -gt 0
    if ($Demo) { $hasUpdate = $true }

    if (-not $hasUpdate) { Launch-Mxu; return }

    $doUpdate = Show-UpdateDialog $release $current
    if (-not $doUpdate) { Launch-Mxu; return }

    # 用户选择更新 ----------------------------------------------------------
    $asset = Select-Asset $release.assets $cfg
    if (-not $asset) {
        [System.Windows.Forms.MessageBox]::Show(
            "未在新版本中找到合适的压缩包资源，请前往发布页手动下载。", "更新", 'OK', 'Information')
        Start-Process $release.html_url
        Launch-Mxu
        return
    }

    $ddir = Join-Path $BASE $cfg['download_dir']
    New-Item -ItemType Directory -Path $ddir -Force | Out-Null
    $dest = Join-Path $ddir $asset.name

    # 生成候选下载 URL：配置的镜像前缀按顺序尝试，官方原 URL 放最后兜底
    $urls = ConvertTo-DownloadUrls $asset.browser_download_url $cfg
    $err = Start-Download $urls $dest ([int]($asset.size))
    if ($err) {
        [System.Windows.Forms.MessageBox]::Show(
            ("下载失败：{0}`n请前往发布页手动下载。" -f $err), "更新失败", 'OK', 'Error')
        Start-Process $release.html_url
        # 清理失败下载留下的半截 zip，避免 updates/ 越来越胖
        Remove-Item -LiteralPath $dest -Force -ErrorAction SilentlyContinue
        Launch-Mxu
        return
    }

    # 解压 + 选择性覆盖
    $tmp = Join-Path $env:TEMP ("BD2MAA_update_" + [guid]::NewGuid().ToString('N'))
    try {
        Expand-Archive -Path $dest -DestinationPath $tmp -Force
        $srcRoot = Find-ProjectRoot $tmp
        Copy-Update $srcRoot $BASE $cfg
        ReInject-XLaunch $BASE
        # 更新完成后立刻清理旧版遗留文件：retired_files.json 也刚被覆盖成新版，
        # 所以这一步用的就是本次发布的最新清单（清单之外的旧文件不动）。
        Remove-RetiredFiles $BASE $cfg
    } catch {
        # 注意：$dest 会在 finally 里被清掉，所以这里不要再让用户"手动解压该文件"。
        [System.Windows.Forms.MessageBox]::Show(
            ("解压或覆盖失败：{0}`n`n下载的压缩包可能不完整。可重新启动 launcher.bat 再试一次（会自动切换下载源），或前往发布页手动下载：`n{1}" -f $_.Exception.Message, $release.html_url),
            "更新失败", 'OK', 'Error')
        Launch-Mxu
        return
    } finally {
        Remove-Item $tmp -Recurse -Force -ErrorAction SilentlyContinue
        Remove-Item $dest -Force -ErrorAction SilentlyContinue
    }

    [System.Windows.Forms.MessageBox]::Show(
        "更新完成！已应用新版本，你的 config/ 配置已保留。", "更新完成", 'OK', 'Information')
    Launch-Mxu

} catch {
    # 任何异常都不应阻断用户进入软件
    try { Launch-Mxu } catch { }
}


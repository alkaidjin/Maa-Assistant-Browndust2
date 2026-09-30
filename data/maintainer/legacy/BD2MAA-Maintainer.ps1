# BD2MAA-Maintainer.ps1
# =============================================================================
# BD2MAA 维护器（纯 PowerShell 实现，零依赖，无需 Python）
#
# 定位：**不再负责启动软件**。MXU 2.5.3 起自带「设置 - 更新」的软件内自动更新
#   （v26.09.13 已实测跑通），日常升级请以它为准。本脚本退居为「维护 / 急救」工具：
#
#   1. 家务（默认，无参数，不联网）：
#        - 按 retired_files.json 清理旧版遗留文件（幂等；开发树存在 .git 时整体跳过）
#        - 自愈 maintainer.bat 的编码（v26.09.7 事故的运行时兜底）
#        - 恢复 mxu.exe 图标（MXU 自更新替换 exe 后图标会被打回）
#        - 重建 / 修正 MaaBd2.lnk（指向 mxu.exe）
#        - 精简并清理 debug 日志（按 log_retention_days）
#      任何一项失败都静默跳过，绝不阻断。
#
#   2. 完整性验证（-Verify，不联网）：
#        逐项检查运行时硬依赖（mxu.exe / go-service.exe / rock-picker.exe / OCR 模型 /
#        pipeline 与 image 资源 / i18n 与告警模板 / 维护器自身），并解析 interface.json
#        读出版本号。缺什么列什么，收尾给出「用 -Repair 重装」的建议。
#
#   3. 多源镜像下载（-Fetch / -Repair，联网）：
#        从 GitHub Releases 取最新包，按 updater_config.json 的镜像前缀顺序尝试
#        （ghfast.top -> gh-proxy.com -> 官方原链兜底），每个源下载完都做三层校验
#        （字节数 / PK 魔数 / 中央目录），不合格自动换下一个源 —— 国内直连 GitHub
#        常被限速到 10~30 KB/s，镜像链是主要的下载通道。
#        - -Fetch  只把包下载到 updates\，不解压、不覆盖（想手动重装时用）
#        - -Repair 下载并覆盖安装，全程三道校验：
#            [装前] 包内 interface.json 版本号 == release tag，且含 mxu.exe
#            [装中] 逐文件重试 3 次，失败的**具体文件名**列出来
#            [装后] 复核磁盘版本号 == tag，关键文件在位
#          装前会提示关闭 mxu.exe / go-service.exe（占用即覆盖失败，是"装了一半"的根因）；
#          config/ 已有配置不覆盖，仅新增缺失的默认配置。
#
# 可回溯性：全程写 debug\maintainer.log（BASE / 当前版本 / 选中的 release 与资产 /
#   逐个下载源的结果与字节数 / 包身份校验 / 覆盖失败文件清单 / 装前装后版本号 / 结论）。
#   用户报「重装完版本号没变」时，让他把这份日志发回来即可定位。
#
# 用法：
#   .\BD2MAA-Maintainer.ps1           # 跑一遍家务（默认，不联网、不启动软件）
#   .\BD2MAA-Maintainer.ps1 -Verify   # 完整性验证
#   .\BD2MAA-Maintainer.ps1 -Fetch    # 只下载最新包到 updates\
#   .\BD2MAA-Maintainer.ps1 -Repair   # 下载最新包并覆盖安装（保留 config/）
# =============================================================================

param(
    [switch]$Verify,
    [switch]$Fetch,
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
# 文件名沿用 updater_config.json：里面存着用户自定义的镜像列表，改名会让用户配置失效。
$CONFIG_FILE= Join-Path $BASE 'updater_config.json'
$MXU        = Join-Path $BASE 'mxu.exe'
$INTERFACE  = Join-Path $BASE 'interface.json'
$ENTRY_BAT  = Join-Path $BASE 'maintainer.bat'

# ----------------------------------------------------------------------------
# 默认配置（可被 updater_config.json 覆盖）
# ----------------------------------------------------------------------------
$DEFAULTS = @{
    repo                   = 'alkaidjin/Maa-Assistant-Browndust2'
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
function Write-MaintainerLog($msg) {
    # 更新全程落盘（debug/maintainer.log）。为什么必须写：用户报「更新完版本号没变」时，
    # 弹窗本来是唯一线索，可静默分支根本不弹窗 —— 没有日志就只能靠猜。
    # 日志会被 Invoke-LogCleanup 按 log_retention_days 自然淘汰（持续追加所以时间戳常新）；
    # 任何写失败都绝不阻断启动。
    try {
        $lf = Join-Path $BASE 'debug\maintainer.log'
        $dir = Split-Path -Parent $lf
        if (-not (Test-Path -LiteralPath $dir)) { New-Item -ItemType Directory -Path $dir -Force | Out-Null }
        Add-Content -LiteralPath $lf -Value ('[{0}] {1}' -f (Get-Date -Format 'yyyy-MM-dd HH:mm:ss'), $msg) -Encoding UTF8 -ErrorAction Stop
    } catch { }
}

function Initialize-MaintainerLog {
    # 启动时做一次体积裁剪（>512 KB 只留最后 300 行），避免长期挂着把磁盘写满。
    try {
        $lf = Join-Path $BASE 'debug\maintainer.log'
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

function Get-LatestRelease($repo) {
    $url = "https://api.github.com/repos/$repo/releases/latest"
    return Invoke-RestMethod -Uri $url `
        -Headers @{ 'User-Agent' = 'BD2MAA-Maintainer'; 'Accept' = 'application/vnd.github+json' } `
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
        -Headers @{ 'User-Agent' = 'BD2MAA-Maintainer'; 'Accept' = 'application/vnd.github+json' } `
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
    $web.Headers.Add('User-Agent', 'BD2MAA-Maintainer')
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
        Write-MaintainerLog ('尝试下载源 [' + ($i + 1) + '/' + $list.Count + ']: ' + $u)
        $err = Start-DownloadOne $u $dest $totalBytes
        if (-not $err) {
            # 下载完成 ≠ 内容正确：镜像可能返回 HTML 错误页。校验通过才算成功，
            # 否则清掉残留、记下原因、继续尝试下一个源。
            $gotBytes = 0
            try { $gotBytes = (Get-Item -LiteralPath $dest).Length } catch { }
            Write-MaintainerLog ('  下载返回成功，实际 ' + $gotBytes + ' bytes（期望 ' + $totalBytes + '）')
            if (Test-ValidZip $dest $totalBytes) {
                Write-MaintainerLog ('  校验通过（字节数 + zip 格式）')
                return $null
            }
            $err = ('文件校验失败（字节数或 zip 格式不符，源可能返回了错误页）')
        }
        Write-MaintainerLog ('  该源失败: ' + $err)
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
            if (-not (Test-Path -LiteralPath $target)) {
                try { Copy-Item -LiteralPath $full -Destination $target -Force -ErrorAction Stop } catch { }
            }
            continue
        }

        $tmp = "$target.new"
        for ($try = 1; $try -le 3; $try++) {
            try {
                # -ErrorAction Stop 是**必须的**：Copy-Item / Move-Item 写失败时抛的是
                # 非终止错误，不加它根本进不了 catch —— 于是"mxu.exe 没覆盖成功"会被
                # 一路当成成功，最后弹出"更新完成"，而用户手上其实是半截安装。
                # （这正是「更新完版本号没变 / 只装了一半」最难查的一类成因。）
                if ($f.Length -ge $atomicThreshold) {
                    # 原子覆盖：写到 .new，再 rename。失败时清理 .new 不留垃圾。
                    Copy-Item -LiteralPath $full -Destination $tmp -Force -ErrorAction Stop
                    Move-Item -LiteralPath $tmp -Destination $target -Force -ErrorAction Stop
                } else {
                    Copy-Item -LiteralPath $full -Destination $target -Force -ErrorAction Stop
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
    foreach ($rel in @('mxu.exe', 'interface.json', 'maintainer.bat', 'BD2MAA-Maintainer.ps1')) {
        if (-not (Test-Path (Join-Path $base $rel))) { $problems += "缺少 $rel" }
    }
    return $problems
}

function Test-Integrity($base) {
    # 完整性验证：把「运行时硬依赖」逐项点一遍，缺什么列什么。
    # 清单与打包器 REQUIRED_FILES 对齐（tools\build_release_zip.py），避免两处口径走偏。
    # 只读不写、不联网；任何一项缺失都是「-Repair 重装」的直接理由。
    $issues = @()

    $files = @(
        'mxu.exe', 'interface.json', 'maintainer.bat', 'BD2MAA-Maintainer.ps1',
        'retired_files.json', 'updater_config.json',
        'agent/go-service.exe',              # go-service：周门禁 / 自定义识别 / 自定义动作
        'agent/rock-picker.exe',             # 圣石洞穴「刷数量最少的石头」识别器
        'tools/rcedit-x64.exe',              # 家务链：恢复 mxu.exe 图标
        'tools/compact_log.ps1',             # 家务链：日志精简
        'misc/locales/zh_cn.json',           # MXU 界面 i18n
        'locales/go-service/zh_cn.json',     # go-service 提示文案
        'locales/go-service/HTML/aspect-ratio-warning.html',  # 分辨率守护告警模板
        'maafw/LICENSE.md'                   # LGPL-3.0 履约
    )
    foreach ($rel in $files) {
        if (-not (Test-Path -LiteralPath (Join-Path $base ($rel.Replace('/', '\'))))) {
            $issues += "缺少 $rel"
        }
    }

    $dirs = @('resource/pipeline', 'resource/image', 'maafw', 'misc/locales', 'locales/go-service/HTML')
    foreach ($d in $dirs) {
        if (-not (Test-Path -LiteralPath (Join-Path $base ($d.Replace('/', '\'))))) {
            $issues += "缺少目录 $d"
        }
    }

    # OCR 模型：缺了所有文字识别都会失败，且报错信息往往指向别处，单独点名
    $ocrDir = Join-Path $base 'resource\model\ocr'
    if (-not (Test-Path -LiteralPath $ocrDir)) {
        $issues += '缺少目录 resource/model/ocr'
    } else {
        $onnx = @(Get-ChildItem -LiteralPath $ocrDir -Recurse -Filter '*.onnx' -ErrorAction SilentlyContinue)
        if ($onnx.Count -eq 0) { $issues += 'resource/model/ocr 下没有 .onnx 模型文件' }
    }

    # interface.json 必须能解析（损坏 / 被写成带 BOM 时 MXU 会直接资源加载失败）
    $ifp = Join-Path $base 'interface.json'
    if (Test-Path -LiteralPath $ifp) {
        try {
            $null = Get-Content -LiteralPath $ifp -Raw -Encoding UTF8 | ConvertFrom-Json
        } catch {
            $issues += 'interface.json 解析失败（文件可能被写坏或带了 BOM）'
        }
        if (-not (Read-CurrentVersion)) { $issues += 'interface.json 里读不到 version' }
    }
    return $issues
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
# 清单有两个字段：
#   * files            —— 永久条目，每次都按它清理（幂等，删过就跳过）
#   * versioned_files  —— 只在指定版本生效一次：键是版本号（如 v26.09.13），只有当前
#                         interface.json 版本号与键**相等**时才清理。给「这一版刚把某个
#                         文件换掉、之后不可能再出现」的场景用，避免条目长期挂着。
# 安全边界（任何一条不满足就跳过该项）：
#   * 只删清单里的**显式相对路径**；含 .. 或绝对路径一律跳过（防目录穿越）
#   * 跳过受保护目录（config/，见 updater_config.json 的 protected_dirs）
#   * 清单文件缺失 / 解析失败：静默跳过，绝不阻断启动
#   * **存在 .git 的开发树整体跳过** —— 维护者本机的仓库里还有别的文件，
#     不能让它在这儿误删东西（用户包内不含 .git，所以对用户端无影响）
#   * 删掉的每一条都写 debug/maintainer.log，便于回溯「我的 xx 文件怎么没了」
function Remove-RetiredFiles($base, $cfgRef, $ver) {
    # 整段兜底：清理只是"卫生工作"，任何意外都绝不能把用户挡在软件门外，
    # 更不能让异常冒泡到主流程的 catch（那会被误报成"更新失败"）。
    try {
        if (Test-Path (Join-Path $base '.git')) { return }

        $listFile = Join-Path $base 'retired_files.json'
        if (-not (Test-Path $listFile)) { return }

        try {
            $j = Get-Content $listFile -Raw -Encoding UTF8 | ConvertFrom-Json
        } catch {
            try { Write-MaintainerLog ('[retired] 清单解析失败，本次不清理: ' + $listFile) } catch { }
            return
        }
        if (-not $j) { return }

        $protected = $cfgRef['protected_dirs']

        Remove-RetiredList -base $base -items $j.files -protected $protected
        if ($j.versioned_files -and $ver) {
            foreach ($prop in $j.versioned_files.PSObject.Properties) {
                $want = [string]$prop.Name
                if ($want.StartsWith('_')) { continue }          # _ 开头的是说明字段，不是版本号
                try { if ((Compare-Version $ver $want) -ne 0) { continue } } catch { continue }
                Remove-RetiredList -base $base -items $prop.Value -protected $protected
            }
        }
    } catch {
        try { Write-MaintainerLog ('[retired] 清理阶段异常（已跳过，不影响启动）: ' + $_.Exception.Message) } catch { }
    }
}

function Remove-RetiredList($base, $items, $protected) {
    if (-not $items) { return }

    # 越界防线用的基准目录：规范化一次，后面每个条目都拿它比对。
    $baseFull = ''
    try { $baseFull = ([System.IO.Path]::GetFullPath($base)).TrimEnd('\', '/') } catch { return }

    foreach ($item in $items) {
        # 逐项兜底：一条清单把某个文件锁住 / 路径畸形，不能连累其它条目，更不能中断启动。
        try {
            $rel = ([string]$item).Replace('\', '/').TrimStart('/')
            if (-not $rel) { continue }

            # --- 越界防线（三重，任一命中即跳过） ---
            if ($rel.Contains('..')) { continue }                                  # 目录穿越
            if ($rel -match '^[A-Za-z]:') { continue }                             # 盘符绝对路径
            if ($rel.StartsWith('\\') -or $rel.StartsWith('//')) { continue }      # UNC / 网络路径

            $low = $rel.ToLower()
            $skip = $false
            foreach ($p in $protected) {
                $pl = ([string]$p).Replace('\', '/').TrimEnd('/').ToLower()
                if ($pl -and ($low -eq $pl -or $low.StartsWith("$pl/"))) { $skip = $true; break }
            }
            if ($skip) { continue }

            $full = Join-Path $base ($rel.Replace('/', '\'))

            # 最后一道：规范化后必须真的落在软件根目录之内（吃掉任何拼接花招）
            try {
                $resolved = [System.IO.Path]::GetFullPath($full)
                if (-not $resolved.StartsWith($baseFull + '\', [StringComparison]::OrdinalIgnoreCase)) {
                    Write-MaintainerLog ('[retired] 跳过越界路径: ' + $rel)
                    continue
                }
            } catch { continue }

            if (Test-Path -LiteralPath $full -PathType Leaf) {
                Remove-Item -LiteralPath $full -Force -ErrorAction SilentlyContinue
                if (Test-Path -LiteralPath $full -PathType Leaf) {
                    # 删不掉多半是被占用（杀软 / 索引器 / 正在运行的程序）。
                    # 这里**不重试也不报错**：遗留文件只是占地方，重试反而拖慢启动。
                    Write-MaintainerLog ('[retired] 删除失败（多半被占用）: ' + $rel)
                } else {
                    Write-MaintainerLog ('[retired] 已删除旧版遗留文件: ' + $rel)
                }
            }
        } catch {
            # 走到这里说明这一步抛了异常（典型：文件正被杀软 / 索引器 / 运行中的程序占用，
            # 连 Test-Path / Remove-Item 都会抛）。同样只记不拦 —— 一个文件删不掉，
            # 后果只是它继续留在磁盘上，绝不能因此不让用户进软件。
            try {
                Write-MaintainerLog ('[retired] 删除失败（多半被占用，已跳过该项）: ' + $item +
                                  ' :: ' + $_.Exception.Message)
            } catch { }
        }
    }
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

        # rcedit 把报错直接写到 stderr（"can't open file for write !" 之类），会漏到用户
        # 控制台上吓人一跳。这里重定向到 cache\ 下的临时文件：图标写不进去只是「还用默认图标」，
        # 不值得打扰用户；stamp 不更新，下次跑维护器还会再试一次。
        $errFile = Join-Path $cdir 'rcedit_err.txt'
        Start-Process -FilePath $rc -ArgumentList @("`"$MXU`"", '--set-icon', "`"$ico`"") `
            -WindowStyle Hidden -Wait -ErrorAction Stop -RedirectStandardError $errFile
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

function Repair-AppShortcut {
    # ---- 重建根目录的「MaaBd2.lnk」快捷方式 ----
    # 原因：发布 zip 里不打包 MaaBd2.lnk（不同用户的安装路径不同，写死路径的 lnk 在别人机器上
    #       是失效的），所以每次跑维护器时按当前 $BASE 自动生成。
    # v26.09.14 起：维护器不再负责启动软件（MXU 自带软件内更新），快捷方式因此改为
    #       直接指向 mxu.exe 本体 —— 用户双击它就是打开软件，符合直觉。
    #       老用户目录里指向旧 launcher.bat 的 lnk 会被一并覆盖。
    # 与 Apply-ExeIcon 的区别：后者只 patch mxu.exe 的图标指纹，不重建快捷方式。
    try {
        if (-not (Test-Path $BASE)) { return }
        $ws  = New-Object -ComObject WScript.Shell -ErrorAction SilentlyContinue
        if (-not $ws) { return }
        $lnk = Join-Path $BASE 'MaaBd2.lnk'
        $ico = Join-Path $BASE 'mxu.ico'
        if (-not (Test-Path -LiteralPath $MXU)) { return }
        $s   = $ws.CreateShortcut($lnk)
        $s.TargetPath       = $MXU
        $s.WorkingDirectory = $BASE
        $s.WindowStyle      = 1   # 1=正常窗口
        if (Test-Path -LiteralPath $ico) { $s.IconLocation = "$ico,0" }
        $s.Description      = 'BD2MAA（棕色尘埃2 自动化助手）'
        $s.Save()
    } catch { }
}

function Repair-EntryBat {
    # ---- 自愈 maintainer.bat 的编码（v26.09.7 事故后的运行时兜底）----
    # 背景：cmd 逐字节解析 .bat，遇到非 ASCII 字节（中文注释）会错位解码，把注释后半段
    #   当成命令执行 -> 刷「不是内部或外部命令，也不是可运行的程序或批处理文件」；
    #   严重时整行边界被打乱，脚本被截断，后面的行（含真正的 PowerShell 调用）根本不执行。
    #   LF 行尾会显著加剧（实测 LF+长中文注释可导致 rc=1 + 调用被吞）。
    #   根治 = 文件 100% ASCII + CRLF。打包器 tools\build_release_zip.py 有
    #   check_bat_crlf() 预警 + normalize_crlf() 强制规范；这里是包外的第二道防线：
    #   只在检测到「含非 ASCII 字节」时重写（这是真正的杀伤源），
    #   不因行尾问题改写 —— 避免误伤用户自己加过自定义行的 bat。
    try {
        $bat = Join-Path $BASE 'maintainer.bat'
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
            'REM  BD2MAA maintainer  (housekeeping / verify / mirror download)',
            'REM  No args: housekeeping  -Verify check  -Fetch download  -Repair reinstall',
            'REM  Keep pure ASCII + CRLF -- see check_bat_crlf() in build_release_zip.py',
            'REM ============================================================',
            'cd /d "%~dp0"',
            'powershell -NoProfile -ExecutionPolicy Bypass -File "BD2MAA-Maintainer.ps1" %*'
        ) -join "`r`n"
        [System.IO.File]::WriteAllBytes($bat, [System.Text.Encoding]::ASCII.GetBytes($canon + "`r`n"))
    } catch { }
}

function Invoke-Housekeeping {
    # 家务链：全部失败静默、绝不阻断。
    # 注意（v26.09.14）：这里**不再启动 mxu.exe** —— 软件内更新已由 MXU「设置-更新」承担，
    # 维护器退居为维护 / 急救工具。用户打开软件请双击 mxu.exe 或 MaaBd2.lnk。
    Repair-EntryBat        # 自愈被中文注释/编码污染过的 maintainer.bat（幂等，纯 ASCII 时直接返回）
    Apply-ExeIcon          # MXU 自更新后自动恢复程序图标
    Repair-AppShortcut     # 重建 MaaBd2.lnk（zip 不打包，按当前 $BASE 自动生成；指向 mxu.exe）
    Invoke-LogCleanup      # 清理 debug/ 下超过保留天数的日志与调试截图
}

# ----------------------------------------------------------------------------
# 主流程
# ----------------------------------------------------------------------------
try {
    Initialize-MaintainerLog
    $current = Read-CurrentVersion
    Write-MaintainerLog ('==== 维护器启动 ==== 参数=[' + (($PSBoundParameters.Keys | ForEach-Object { '-' + $_ }) -join ' ') + '] PS=' + $PSVersionTable.PSVersion.ToString())
    Write-MaintainerLog ('BASE=' + $BASE)
    Write-MaintainerLog ('当前版本=' + $current)

    # 家务第一步：清理旧版遗留文件（幂等、静默）。放在最前面，之后每个分支都覆盖得到。
    # 第三个参数是当前版本号，供清单里的 versioned_files（只在指定版本生效）比对。
    Remove-RetiredFiles $BASE $cfg $current

    # ---------------- -Verify：完整性验证（不联网）----------------
    if ($Verify) {
        $issues = @(Test-Integrity $BASE)
        Write-MaintainerLog ('[完整性] 问题数=' + $issues.Count + $(if ($issues.Count -gt 0) { ': ' + ($issues -join ' / ') } else { '' }))
        if ($issues.Count -eq 0) {
            [System.Windows.Forms.MessageBox]::Show(
                (("完整性校验通过。`n`n当前版本 {0}`n关键文件、资源与 OCR 模型都在位。") -f $current),
                'BD2MAA · 完整性校验', 'OK', 'Information') | Out-Null
        } else {
            [System.Windows.Forms.MessageBox]::Show(
                (("完整性校验发现 {0} 项问题：`n`n - {1}`n`n" +
                  "建议：用 `"maintainer.bat -Repair`" 重新下载并覆盖安装（不会动你的 config/）。" +
                  "`n详细清单见 debug\maintainer.log。") -f $issues.Count, ($issues -join "`n - ")),
                'BD2MAA · 完整性校验发现问题', 'OK', 'Warning') | Out-Null
        }
        Invoke-Housekeeping
        return
    }

    # ---------------- 默认：只跑家务（不联网、不启动软件）----------------
    if (-not ($Fetch -or $Repair)) {
        Invoke-Housekeeping
        Write-MaintainerLog ('[结论] 家务完成（未联网、未启动软件），版本 ' + $current)
        [System.Windows.Forms.MessageBox]::Show(
            (("维护完成。`n`n已清理旧版遗留文件、恢复 mxu.exe 图标与 MaaBd2 快捷方式、整理 debug 日志。`n`n" +
              "当前版本 {0}`n`n日常升级请用软件内的「设置 - 更新」；软件打不开或文件缺失时，" +
              "再回来用 -Verify / -Repair。") -f $current),
            'BD2MAA · 维护器', 'OK', 'Information') | Out-Null
        return
    }

    # ---------------- 联网分支：取最新 release ----------------
    $release = $null
    $asset   = $null
    try {
        $release = Get-LatestRelease $cfg['repo']
        # latest 必须真的带可用 zip 才认；否则回退到「版本最高且有可用 zip」的 release
        $pick = Select-NewestReleaseWithAsset $cfg['repo'] $cfg $release
        if ($pick) { $release = $pick.release; $asset = $pick.asset }
        Write-MaintainerLog ('检测到 release=' + $release.tag_name + '  资产=' + $(if ($asset) { $asset.name + ' (' + $asset.size + ' bytes)' } else { '无可用 zip' }))
    } catch {
        $release = $null
        $asset   = $null
        Write-MaintainerLog ('[失败] 版本检测异常: ' + $_.Exception.Message)
        $r = [System.Windows.Forms.MessageBox]::Show(
            (("没能连上 GitHub 获取版本信息（网络不可达 / 被限流 / 需要代理）。`n`n" +
              "错误：{0}`n`n点「确定」= 前往发布页手动下载`n点「取消」= 关闭") -f $_.Exception.Message),
            'BD2MAA · 维护器', 'OKCancel', 'Warning')
        if ($r -eq 'OK') { Start-Process ('https://github.com/' + $cfg['repo'] + '/releases/latest') }
        return
    }

    if (-not $release) {
        Write-MaintainerLog ('[结论] 未取到版本信息，结束（当前 ' + $current + '）')
        return
    }

    # 新版本已发布但资产还没传完（实测可能差几分钟到几小时）：明确告知，不要静默跳过
    if (-not $asset) {
        Write-MaintainerLog ('[提示] ' + $release.tag_name + ' 还没有可用 zip 资产 → 结束（当前 ' + $current + '）')
        $r = [System.Windows.Forms.MessageBox]::Show(
            (("{0} 这个版本还没有可用的压缩包（发布资产可能还在上传）。`n`n" +
              "点「确定」= 前往发布页查看`n点「取消」= 关闭") -f $release.tag_name),
            'BD2MAA · 暂无可下载的包', 'OKCancel', 'Information')
        if ($r -eq 'OK') { Start-Process $release.html_url }
        return
    }

    # ---------------- 下载（多源镜像链）----------------
    $ddir = Join-Path $BASE $cfg['download_dir']
    New-Item -ItemType Directory -Path $ddir -Force | Out-Null
    $dest = Join-Path $ddir $asset.name

    $urls = ConvertTo-DownloadUrls $asset.browser_download_url $cfg
    Write-MaintainerLog ('下载目标: ' + $dest + '  期望字节数=' + ([int]($asset.size)))
    Write-MaintainerLog ('候选下载源 ' + @($urls).Count + ' 个: ' + ((@($urls) | ForEach-Object { Shorten-Url $_ }) -join ' | '))
    $err = Start-Download $urls $dest ([int]($asset.size))
    if ($err) {
        Write-MaintainerLog ('[失败] 所有下载源均未成功: ' + $err)
        $r = [System.Windows.Forms.MessageBox]::Show(
            (("下载失败：{0}`n`n点「确定」= 前往发布页手动下载`n点「取消」= 关闭") -f $err),
            'BD2MAA · 下载失败', 'OKCancel', 'Error')
        if ($r -eq 'OK') { Start-Process $release.html_url }
        # 清理失败下载留下的半截 zip，避免 updates/ 越来越胖
        Remove-Item -LiteralPath $dest -Force -ErrorAction SilentlyContinue
        return
    }

    # ---------------- -Fetch：只下载，不解压不覆盖 ----------------
    if ($Fetch) {
        Write-MaintainerLog ('[结论] 已下载到 ' + $dest + '（未安装）')
        $r = [System.Windows.Forms.MessageBox]::Show(
            (("已下载 {0}`n`n保存位置：{1}`n`n这是完整的发布包：需要重装时把它解压、覆盖到软件目录即可" +
              "（config/ 里的配置不要覆盖）。`n`n点「确定」= 打开所在文件夹`n点「取消」= 关闭") -f $asset.name, $dest),
            'BD2MAA · 下载完成', 'OKCancel', 'Information')
        if ($r -eq 'OK') { Start-Process 'explorer.exe' (('/select,"{0}"' -f $dest)) }
        return
    }

    # ---------------- -Repair：覆盖安装 ----------------
    $ask = [System.Windows.Forms.MessageBox]::Show(
        (("修复模式：将重新下载并覆盖安装 {0}（当前 {1}）。`n`n" +
          "用途：软件打不开、文件缺失、或上一次更新只装了一半时，把整个软件目录按该版本重装一遍。`n" +
          "config/ 里的用户配置不会被覆盖。`n`n继续？") -f $release.tag_name, $current),
        'BD2MAA · 修复安装', 'OKCancel', 'Question')
    if ($ask -ne 'OK') {
        Write-MaintainerLog ('用户取消了修复安装（目标 ' + $release.tag_name + '，当前 ' + $current + '）')
        return
    }

    # 覆盖前先关掉正在运行的软件：进程占用文件会让覆盖失败，产出“装了一半”的状态
    if (-not (Stop-RunningMxu)) {
        Write-MaintainerLog ('用户拒绝了"关闭软件后再修复" → 取消，本次不覆盖任何文件')
        return
    }

    $tmp = Join-Path $env:TEMP ("BD2MAA_repair_" + [guid]::NewGuid().ToString('N'))
    $problems = @()
    try {
        Write-MaintainerLog ('解压到: ' + $tmp)
        Expand-Archive -Path $dest -DestinationPath $tmp -Force -ErrorAction Stop
        $srcRoot = Find-ProjectRoot $tmp
        $pkgCount = @(Get-ChildItem -Path $srcRoot -Recurse -File -ErrorAction SilentlyContinue).Count
        Write-MaintainerLog ('解压完成，包根=' + $srcRoot + '，文件数=' + $pkgCount)

        # [装前] 包的身份必须对得上：包内 interface.json 版本 == release 的 tag，且含 mxu.exe
        $pkg = Test-PackageIdentity $srcRoot $release.tag_name
        Write-MaintainerLog ('[装前] 包身份校验: ok=' + $pkg.ok + ' 包内版本=' + $pkg.inner + ' 期望=' + $release.tag_name + $(if (-not $pkg.ok) { ' 原因=' + $pkg.why } else { '' }))
        if (-not $pkg.ok) {
            [System.Windows.Forms.MessageBox]::Show(
                (("这次下载到的包身份不符：{0}。`n`n" +
                  "已取消覆盖，你的软件目录一个字都没动。`n" +
                  "多见于「该 release 的压缩包正在被替换 / 刚上传」—— 稍后再试一次即可；`n" +
                  "也可以点「确定」前往发布页手动下载。") -f $pkg.why),
                'BD2MAA · 包身份不符', 'OKCancel', 'Warning') | Out-Null
            Start-Process $release.html_url
            return
        }

        $fail = @(Copy-Update $srcRoot $BASE $cfg)
        Write-MaintainerLog ('覆盖完成，失败文件 ' + $fail.Count + ' 个' + $(if ($fail.Count -gt 0) { ': ' + ($fail -join '、') } else { '' }))
        # 装完立刻清理旧版遗留文件：retired_files.json 也刚被覆盖成新版，
        # 所以这一步用的就是本次发布的最新清单（清单之外的旧文件不动）。
        # 此处必须用**装好之后**的版本号（interface.json 刚被覆盖）。
        Remove-RetiredFiles $BASE $cfg (Read-CurrentVersion)

        # [装后] 复核：磁盘上的版本号与关键文件是否真的就位
        $problems = @(Test-InstalledVersion $BASE $release.tag_name)
        Write-MaintainerLog ('[装后] 复核: 磁盘版本=' + (Read-CurrentVersion) + ' 期望=' + $release.tag_name + ' 问题数=' + $problems.Count + $(if ($problems.Count -gt 0) { ': ' + ($problems -join ' / ') } else { '' }))
        if ($fail.Count -gt 0) {
            $problems = @('以下文件未能覆盖（多半被杀毒软件或其它程序占用）：' + ($fail -join '、')) + $problems
        }
    } catch {
        Write-MaintainerLog ('[失败] 解压或覆盖阶段异常: ' + $_.Exception.Message)
        Write-MaintainerLog ('  位置: 第 ' + $_.InvocationInfo.ScriptLineNumber + ' 行 :: ' + ($_.InvocationInfo.Line -replace "`r?`n", ' '))
        # $dest 会在 finally 里被清掉，所以这里不要再让用户"手动解压该文件"。
        [System.Windows.Forms.MessageBox]::Show(
            (("解压或覆盖失败：{0}`n`n建议先关闭软件（mxu.exe）与杀毒软件的实时防护，再用 " +
              "`n`"maintainer.bat -Repair`" 重新下载并覆盖；`n也可以点「确定」前往发布页手动下载：{1}") -f $_.Exception.Message, $release.html_url),
            'BD2MAA · 修复失败', 'OKCancel', 'Error') | Out-Null
        Start-Process $release.html_url
        return
    } finally {
        Remove-Item $tmp -Recurse -Force -ErrorAction SilentlyContinue
        Remove-Item $dest -Force -ErrorAction SilentlyContinue
    }

    if ($problems.Count -gt 0) {
        Write-MaintainerLog ('[结论] 修复未完成（目标 ' + $release.tag_name + '）：' + ($problems -join ' / '))
        [System.Windows.Forms.MessageBox]::Show(
            (("修复没有完整完成（本次目标是 {0}）：`n`n - {1}`n`n" +
              "建议：1) 先关闭本软件与杀毒软件的实时防护；2) 再运行 `"maintainer.bat -Repair`"；`n" +
              "3) 仍不行就前往发布页手动下载压缩包，解压到本软件目录覆盖。") -f $release.tag_name, ($problems -join "`n - ")),
            'BD2MAA · 修复未完成', 'OK', 'Error') | Out-Null
    } else {
        Write-MaintainerLog ('[结论] 修复完成，已应用到 ' + $release.tag_name + '（未启动软件，请自行打开 mxu.exe）')
        [System.Windows.Forms.MessageBox]::Show(
            (("修复完成！已应用到 {0}，你的 config/ 配置已保留。`n`n" +
              "维护器不会帮你打开软件 —— 请双击 mxu.exe（或 MaaBd2 快捷方式）进入。") -f $release.tag_name),
            'BD2MAA · 修复完成', 'OK', 'Information') | Out-Null
    }

} catch {
    # 任何异常都写日志 + 给一条明确提示，绝不再静默。
    try {
        Write-MaintainerLog ('[严重] 主流程未捕获异常: ' + $_.Exception.Message)
        Write-MaintainerLog ('  位置: 第 ' + $_.InvocationInfo.ScriptLineNumber + ' 行 :: ' + ($_.InvocationInfo.Line -replace "`r?`n", ' '))
        Write-MaintainerLog ('  当前版本=' + (Read-CurrentVersion))
        [System.Windows.Forms.MessageBox]::Show(
            (("维护器出错了，本次操作可能没有完成。`n`n" +
              "错误：{0}`n`n当前版本仍是 {1}。详细信息已写入 debug\maintainer.log。`n`n" +
              "可以重新运行 maintainer.bat 再试一次。") -f $_.Exception.Message, (Read-CurrentVersion)),
            'BD2MAA · 维护器出错', 'OK', 'Warning') | Out-Null
    } catch { }
}

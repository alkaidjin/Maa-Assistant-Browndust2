# -*- coding: utf-8 -*-
"""
BD2MAA 发布包构建工具

用法：
    python tools/build_release_zip.py                 # 无参 → 交互式询问版本号
    python tools/build_release_zip.py v26.09.9        # 显式指定版本号
    python tools/build_release_zip.py --dry-run       # 只打印要做什么，不动磁盘也不出包
    python tools/build_release_zip.py --no-bump       # 不改任何版本号文件，按磁盘现状打包
    python tools/build_release_zip.py --out D:\\x.zip  # 指定输出 zip 路径
    python tools/build_release_zip.py --verify        # 只校验已有 zip

双击入口：**项目根目录的 `打包发布包.bat`**（纯 ASCII + CRLF，只是本脚本的包装）。
  为什么需要它：本机 `.py` 的双击关联是 `C:\\Windows\\py.exe`，而 py launcher 里
  没有注册任何 Python（系统 Python 3.10 已卸载、venv / uv 环境不向它注册）→
  直接双击 `.py` 会「窗口闪一下就没了」，看起来毫无反应。
  该 .bat 会依次在 PATH / 本机 venv / 托管解释器里找可用 Python，跑完 pause 不闪退。
  它是维护者工具，**不进发布包**（见 EXCLUDE_FILES）。

约定：
  1. **默认会把所有「版本锚点」同步改写为目标版本号**（见 VERSION_ANCHORS），
     即 interface.json 的 version 字段 + 更新功能说明.md 里内嵌的那份 interface.json 片段。
     不必再一个个打开文件手改版本号；git 提交记录里也自带版本号变更。
     ⚠️ 锚点带 `expect` 防呆：某文件里这类 version 字段的处数与预期不符时**整体跳过**
     并告警（宁可漏改也不误改），文档正文里的历史叙述（如「v26.09.7 修掉了 X」）**永不被改写**。
  2. 直接读磁盘 → **未提交的改动也会进包**（所以发版前先确认工作区状态）。
  3. 排除 config/ → MXU 首次启动自动生成默认实例，避免覆盖用户已有配置。
  4. 排除 MaaBd2.lnk → 写死路径的快捷方式在别人机器上无效，首次启动自动重建。
  5. 排除 updater_cache.json / cache / debug / updates / tools/build_release_zip.py 自身，
     以及 Office 临时锁文件 `~$*`（打开 Verlog.xlsx 时会生成 `~$Verlog.xlsx`）。
     v26.09.8 起额外排除「非维护者不需要」的开发脚本与仓库元数据 —— 见 EXCLUDE_FILES 注释。
  6. 中文文件名必须带 UTF-8 标志位（0x800），否则 Windows 解压乱码。
  7. zip 内条目时间戳 = **打包时刻（秒级）**，不是固定值 → 同源两次构建 sha256 必然不同。
     校验请比「条目内容 / 条目数 / interface.json 版本号」，**不要比 zip 整体 hash、也不要复用旧 zip**。
  8. 批处理（.bat/.cmd）入包前强制规范化为 CRLF 行尾（见 normalize_crlf 注释）。
  9. verify() 逐项校验 REQUIRED_FILES / REQUIRED_DIRS。v26.09.8 起把「启动器硬依赖」
     与「MXU 直接读取的配置 / 资源」也列进 REQUIRED_FILES —— 缺一项就在打包阶段报错，
     不再像原先那样只查目录非空、缺文件却静默通过。
 10. verify() 还会**在产出的 zip 里**逐个锚点复核版本号（[V3]）——磁盘改对了、
     但包内文件是旧的这种情况也会被抓到。

关于「还有哪些文件带版本号」——本工具的处理分三档：
  ✅ 自动改写：VERSION_ANCHORS 列出的（interface.json、更新功能说明.md）
  🔔 只提示：  Verlog.xlsx（版本发布日志表）——本版没有记录时提醒你补一条。
              它记的是「这一版改了什么」，内容必须由人写，工具不去猜。
  ⛔ 刻意不碰：version.json（依赖指纹 maafw/mxu，不是业务版本号，升级底层后人工同步，
               见下方 check_version_json）；updater_cache.json（运行时缓存）；
               各文档正文里的历史叙述（v26.09.7 / v26.09.6 … 是事实，改了就是篡改历史）。
"""
import os, sys, json, time, re, zipfile, hashlib, argparse, subprocess

BASE = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))

EXCLUDE_DIRS  = {'.git', '.workbuddy', 'cache', 'config', 'debug', 'updates', '_stage',
                 # GitHub Actions 配置（PR#3 引入的 mirrorchyan_release*.yml）：只在仓库侧
                 # 生效，用户包里毫无用处，纯属冗余文件。
                 '.github',
                 # 第二个 agent（圣石洞穴「刷数量最少的石头」）的 Go 源码：与 agent/go-service.exe
                 # 同级的 `.exe` 才是运行件（见 REQUIRED_FILES），源码只入仓库、不进用户包。
                 'rock-picker'}
# 注意：EXCLUDE_DIRS 是**按目录名**（不是按相对路径）匹配的 —— 任何层级下叫这些名字的
# 目录都会被整体跳过。当前无副作用；但若将来在 resource/ 等目录下新建名为 config / cache /
# debug 的**合法**子目录，会被静默吞掉、不进包。新增此类目录名时请先确认这里。
# 按包内相对路径匹配
EXCLUDE_FILES = {
    'MaaBd2.lnk', 'updater_cache.json', 'tools/build_release_zip.py',
    # 打包器 / 备份器的双击入口与本体：纯维护者工具，用户侧毫无用处
    # （还会暴露本机解释器路径）。它们**入仓库**，但绝不进用户包。
    '打包发布包.bat',
    '备份维护版.bat',
    'tools/backup_maintainer.ps1',
    'tools/install_backup_task.ps1',
    # ---- v26.09.8 起：非维护者不需要的文件（用户侧无用，且 start.py 写死了作者本机游戏路径）----
    'agent/start.bat', 'agent/start.py',   # agent 开发辅助脚本（写死 C:\Neowiz\... 本机游戏路径）
    'tools/make_icon.py',                  # 生成 mxu.ico / mxu_icon.png 的开发脚本
    'tools/apply_icon.ps1',                # 手动打图标工具（启动器内联 Apply-ExeIcon，不依赖它）
    # MaaPipelineEditor（MPE）Desktop v2.0.0 在项目根生成的编辑器清单：里面全是
    # 编辑器自身二进制的下载 URL 与 sha256，运行时（MXU / MaaFramework）根本不读它。
    # 它既不该进用户包，也不该进仓库 —— 每位维护者装不同版本 MPE 就会变。
    'm2.json',
    # 仓库元数据：collect() 对 EXCLUDE_FILES 是「相对路径 或 文件名」双匹配，
    # 所以这两项会连同嵌套的 git 元数据一起排除（如 resource/model/.gitignore，
    # 其内容只是 "ocr"，与根 .gitignore:29 重复）。这是预期行为 —— 用户包里不该有 git 元数据。
    '.gitattributes', '.gitignore',
    # ---- v26.09.11 起：旧版遗留文件（改名 / 下架的老文档、老视频）不再在这里逐条维护 ----
    # 它们全部由 retired_files.json 兜底排除（见下方 `EXCLUDE_FILES |= RETIRED_FILES`）：
    # 那份清单与启动器的「清理旧版遗留文件」逻辑**同源**，一处维护、两边生效。
    # 历史条目（现已由清单覆盖）：注意事项1/2/3/4 四件套、旧 PDF、旧 DOCX、旧教学视频、
    # github地址.txt。新增退役文件时只改 retired_files.json，别再往这里抄名字。
}
EXCLUDE_EXT   = {'.lnk', '.tmp', '.pyc'}


def load_retired(base):
    """读 retired_files.json → 旧版遗留文件名集合（文件缺失 / 损坏时返回空集）。

    清单里的都是「以前发给过用户、现在不再派发」的文件。它们一旦被恢复、或在本机留了
    副本，重新收进包就是双重错误：既白占体积，又会在用户端被启动器按同一份清单立刻删掉。
    与启动器同源，避免「排除表 / 清理清单」两处手工维护走偏。"""
    try:
        with open(os.path.join(base, 'retired_files.json'), 'rb') as f:
            data = json.loads(f.read().decode('utf-8-sig'))
        return {str(x).replace('\\', '/') for x in (data.get('files') or [])}
    except Exception:
        return set()


RETIRED_FILES = load_retired(BASE)
EXCLUDE_FILES |= RETIRED_FILES

# ---- 必备清单（verify 逐项检查，缺一项即打包失败）----
# 原则：凡是「启动器运行时硬依赖」或「MXU 直接按路径读取」的文件，都必须在这里。
# 只靠 REQUIRED_DIRS 的目录级检查是不够的 —— 目录非空就算过，缺具体文件不会被发现。
REQUIRED_FILES = [
    # 基础入口 / 版本 / 授权
    'interface.json', 'updater_config.json', 'version.json', 'launcher.bat',
    'BD2MAA-Updater.ps1', 'mxu.exe', 'mxu.ico', 'mxu_icon.png', 'LICENSE', 'README.md',
    '更新功能说明.md', '#请务必打开此文档查阅#含使用方式以及常见问题解答.docx',
    # 旧版遗留文件清理清单：启动器按它删掉改名 / 下架的老文档与老视频（v26.09.11 起）
    'retired_files.json',
    # v26.09.7 起：Verlog + 教学视频也跟着入包（用户私维护，不要 gitignore）；
    # v26.09.11 起文档与视频一并改名（带 `#…#` 前后缀，用户打开软件目录就能注意到）
    'Verlog.xlsx',
    '#使用本软件自动打开游戏并设定游戏分辨率的教学视频# .mp4',
    # LGPL-3.0 履约：MaaFramework 的许可证全文必须随包派发（源自上游 release zip 的 LICENSE.md）
    'maafw/LICENSE.md',

    # ---- v26.09.8 起新增：运行时硬依赖，缺了包就是坏的 ----
    # 启动器家务链（BD2MAA-Updater.ps1）
    'tools/rcedit-x64.exe',     # Apply-ExeIcon：给 mxu.exe 写图标（:548）
    'tools/compact_log.ps1',    # Invoke-LogCleanup：把 maa.log 压成任务时间线（:603）
    'tools/clean_logs.ps1',     # README 推荐用户手动执行的日志清理工具
    # agent：周门禁 / 自定义识别 / 自定义动作全靠它，agent.child_exec = agent/go-service
    'agent/go-service.exe',
    # 第二个 agent（PI v2 的 agent 支持对象数组，MXU 2.5.3 逐个启动）：
    # 狩猎场-圣石洞穴「自动刷数量最少的圣石」用的自定义识别器 LeastRockPicker。
    # child_exec = agent/rock-picker（CreateProcess 自动补 .exe）。
    'agent/rock-picker.exe',
    # MXU 直接按 interface.json 的路径读取：icon / license / languages
    'misc/MaaEnd-Tiny.png',
    'misc/LICENSE_SHORT.md',
    'misc/locales/zh_cn.json', 'misc/locales/zh_tw.json', 'misc/locales/en_us.json',
    'misc/locales/ja_jp.json', 'misc/locales/ko_kr.json',
    # agent(go-service) 的 i18n 文案：缺失时 MXU 焦点提示会显示原始 key
    'locales/go-service/zh_cn.json', 'locales/go-service/zh_tw.json',
    'locales/go-service/en_us.json', 'locales/go-service/ja_jp.json',
    'locales/go-service/ko_kr.json',
    # go-service 的告警 HTML 模板（v26.09.11 起）：它按
    # locales/go-service/HTML/<name>.html 读取，读到就直接渲染成提示。
    # 缺文件时 i18n.RenderHTML 只在 go-service.log 留一行 warn —— 分辨率守护
    # 明明把任务 PostStop 掉了，用户却看不到任何原因，只当「任务莫名结束」。
    # 已随包的 3 个 = 本项目真正会触发的 tasker 级模板：
    #   aspect-ratio-warning  分辨率不符（守护，任务启动即被掐）
    #   process-warning       检测到黑名单进程
    #   task-failed-feedback  任务失败时的反馈引导
    # 其余模板（essencefilter-* / autostockpile-* / dijiangrewards-* / interruptible-sleep-*）
    # 属 MaaEnd「终末地」专属，本项目不触发，故不随包。
    'locales/go-service/HTML/aspect-ratio-warning.html',
    'locales/go-service/HTML/process-warning.html',
    'locales/go-service/HTML/task-failed-feedback.html',
    # 新任务引用的模板图（v26.09.11 新增，被 pipeline 引用、必须随包）。
    # 之所以显式列在这里：REQUIRED_DIRS 只检查「目录非空」，缺具体图片不会被发现；
    # 而打包器是 os.walk 读磁盘，本机有图就能出包 —— 别人 clone 出来会打出「缺图却不报错」的坏包。
    'resource/image/Absorb/C4.png', 'resource/image/Absorb/S1.png', 'resource/image/Absorb/S2.png',
    'resource/image/Absorb/S3.png', 'resource/image/Absorb/S4.png', 'resource/image/Absorb/S5.png',
    'resource/image/Absorb/S15.png',
    'resource/image/Sociaty/GONGHUI3.png',
    # v26.09.12 新增：PVP「镜中之战」改从广场直接点入口，靠这张图定位入口按钮。
    'resource/image/Square/PP.png',
    'resource/image/Warcraft/LVDown.png', 'resource/image/Warcraft/LVUP.png',
    # OCR 推理模型（README 第 4 节声明随 release zip 派发；仓库因体积不追踪）
    'resource/model/ocr/det.onnx', 'resource/model/ocr/rec.onnx', 'resource/model/ocr/keys.txt',
    # 核心运行库（mxu.exe 依赖；整目录 maafw/ 其余文件由 REQUIRED_DIRS 兜底）
    'maafw/MaaFramework.dll',
]
REQUIRED_DIRS = ['agent/', 'maafw/', 'misc/', 'tasks/', 'resource/', 'tools/', 'locales/',
                 'locales/go-service/HTML/']

VERSION_RE = re.compile(r'^v\d+\.\d+\.\d+$')

# ---- 版本锚点：一次版本 bump 需要同步改写的位置 ----
# 每项 = (相对路径, 说明, expect)；expect = 该文件里「version 字段」必须恰好出现的处数。
# 只列「表示当前版本」的地方；文档正文里的历史叙述（「v26.09.7 修掉了 X」）不在此列。
# expect 是防呆闸：处数不符 → 该文件**整体跳过**并告警，逼人看一眼再决定，
# 避免文档结构变动后把历史引用的版本号一起改掉。
VERSION_ANCHORS = [
    ('interface.json',
     'PI 主版本号（MXU 界面 / 启动器 / GitHub 更新比对都读它）', 1),
    ('更新功能说明.md',
     '第六节内嵌的 interface.json 片段（当前状态副本，须与主版本号一致）', 1),
]

# 只匹配「被引号包住的 JSON 字段」形态：  "version": "v26.09.8"
# 用 bytes 正则 → 正文里裸露的 v26.09.7 一个都不碰，且原样保留编码 / BOM / 缩进 / 换行。
VERSION_FIELD_RE = re.compile(rb'("version"\s*:\s*")(v\d+\.\d+\.\d+)(")')


def log(msg):
    sys.stdout.write(str(msg) + '\n')
    sys.stdout.flush()


def head_version(base):
    """HEAD 中 interface.json 的版本号（仓库里最近一次发布的标记）"""
    try:
        raw = subprocess.check_output(
            ['git', '-C', base, 'show', 'HEAD:interface.json'], stderr=subprocess.DEVNULL)
        return json.loads(raw.decode('utf-8-sig')).get('version')
    except Exception:
        return None


def disk_version(base):
    """磁盘上 interface.json 当前的版本号（工作区）"""
    try:
        with open(os.path.join(base, 'interface.json'), 'rb') as f:
            raw = f.read()
        return json.loads(raw.decode('utf-8-sig')).get('version')
    except Exception:
        return None


def scan_version_anchors(base):
    """只读扫描所有版本锚点。返回 {rel: {'desc':…, 'expect':…, 'hits':[(行号, 旧版本)]}}。
       不做任何写入 —— 用于「发布计划」展示与改后复核。"""
    sites = {}
    for rel, desc, expect in VERSION_ANCHORS:
        path = os.path.join(base, rel)
        hits = []
        if os.path.exists(path):
            data = open(path, 'rb').read()
            for m in VERSION_FIELD_RE.finditer(data):
                hits.append((data.count(b'\n', 0, m.start()) + 1,
                             m.group(2).decode('ascii')))
        sites[rel] = {'desc': desc, 'expect': expect, 'hits': hits}
    return sites


def sync_version_anchors(base, new_version, apply=True):
    """把所有锚点里的 `"version": "vX.Y.Z"` 改写成 new_version。

        - 逐锚点做 expect 防呆：命中处数 != 期望 → **该文件整体跳过**（不改一行），
          并在 problems 里说明原因。宁可漏改让人看见，也不误改历史引用。
        - apply=False 时只算不写，返回同样的报告（供 --dry-run 使用）。
        - 已是目标版本的文件判为「未变」，不写盘 → mtime 不被动，git 里不产生空 diff。

       返回 (report, problems)：
         report   = [(rel, 行号, 旧版本, 新版本, 动作)]  动作 ∈ {已改写, 待改写, 未变, 跳过}
         problems = [(rel, 原因)]"""
    report, problems = [], []
    for rel, desc, expect in VERSION_ANCHORS:
        path = os.path.join(base, rel)
        if not os.path.exists(path):
            problems.append((rel, '文件不存在'))
            continue
        data = open(path, 'rb').read()
        hits = list(VERSION_FIELD_RE.finditer(data))
        if len(hits) != expect:
            problems.append((rel, '该类 version 字段命中 %d 处，期望 %d 处 → 整体跳过不改；'
                                   '请人工确认后同步修改 VERSION_ANCHORS 的 expect'
                             % (len(hits), expect)))
            for m in hits:
                report.append((rel, data.count(b'\n', 0, m.start()) + 1,
                               m.group(2).decode('ascii'), None, '跳过'))
            continue

        olds = [m.group(2).decode('ascii') for m in hits]
        if all(o == new_version for o in olds):
            for m, o in zip(hits, olds):
                report.append((rel, data.count(b'\n', 0, m.start()) + 1, o, new_version, '未变'))
            continue

        if apply:
            new_data = data
            for m in reversed(hits):        # 从后往前替换，避免偏移错乱
                new_data = (new_data[:m.start(2)] + new_version.encode('ascii')
                            + new_data[m.end(2):])
            with open(path, 'wb') as f:
                f.write(new_data)
        for m, o in zip(hits, olds):
            report.append((rel, data.count(b'\n', 0, m.start()) + 1, o, new_version,
                           '已改写' if apply else '待改写'))
    return report, problems


def check_verlog(base, version):
    """只读检查 Verlog.xlsx 里有没有「本版」的记录，有就报最新一条、没有就提醒补。
       xlsx 本身就是个 zip，直接读 xl/sharedStrings.xml 取文本，
       不引入 openpyxl 依赖，也**绝不改写**这个文件（更新内容是人的活，工具不猜）。"""
    p = os.path.join(base, 'Verlog.xlsx')
    if not os.path.exists(p):
        log('[W] Verlog.xlsx 不存在（版本发布日志表）；发布前建议补上')
        return
    try:
        with zipfile.ZipFile(p) as z:
            xml = z.read('xl/sharedStrings.xml').decode('utf-8', 'replace')
        vals = re.findall(r'<t[^>]*>(.*?)</t>', xml, re.S)
        vers = [v.strip() for v in vals if re.match(r'^v\d+\.\d+\.\d+', v.strip(), re.I)]
        latest = vers[-1] if vers else '（无）'
        if any(re.match(r'^v%s\b' % re.escape(version[1:]), v, re.I) for v in vers):
            log('  Verlog.xlsx: 已有 %s 的记录  OK' % version)
        else:
            log('  Verlog.xlsx: 最新记录 = %s，**尚无 %s 的记录** ← 记得补一条更新说明（本工具不改 xlsx）'
                % (latest, version))
    except Exception as e:
        log('[W] Verlog.xlsx 读取失败: %s' % e)


def check_version_json(base):
    """读取 version.json 并打印现状 + 格式校验。
       这是「包内依赖指纹」（maafw/mxu），不参与 interface.json 的业务版本号 bump；
       升级底层 DLL / mxu.exe 后必须人工同步这个文件，否则发布出去的和实际不符。

       不自动改写：版本号语义不同（业务号 = interface.json.version；依赖号 = 各自上游 tag），
       改写应由人脑拍板。本工具只在「打印发布计划」阶段做提醒 + 格式校验。"""
    p = os.path.join(base, 'version.json')
    if not os.path.exists(p):
        log('[W] version.json 不存在！发布前必须创建（maafw / mxu 两个字段）')
        return
    try:
        with open(p, 'rb') as f:
            data = json.loads(f.read().decode('utf-8-sig'))
        vs = data.get('versions') or {}
        maafw = vs.get('maafw', '?')
        mxu = vs.get('mxu', '?')
        log('  version.json: maafw=%s  mxu=%s  （发布前请确认与磁盘 maafw/、mxu.exe 实际版本一致）'
            % (maafw, mxu))
        for k, v in (('maafw', maafw), ('mxu', mxu)):
            if v == '?' or not VERSION_RE.match(v):
                log('[W] version.json.versions.%s 格式不规范: %r（应为 v<主>.<次>.<修订>，例 v5.13.0）' % (k, v))
    except Exception as e:
        log('[W] version.json 解析失败: %s' % e)


def normalize_crlf(data):
    """把 LF-only 行尾规范化为 CRLF（输入已是 CRLF 时幂等）。

    cmd 逐字节解析 .bat/.cmd：LF-only 行尾 + 非 ASCII 字节时，它按 GBK 解码
    UTF-8 多字节会切错命令边界，把注释的后半段当成新命令执行 —— 用户双击时
    报「'<乱码>' 不是内部或外部命令」。2026-09-15 实测矩阵：
        LF   + 纯 ASCII -> 正常
        LF   + 中文     -> 报错
        CRLF + 中文     -> 正常
    因此打包时对批处理文件强制 CRLF，保证发布包在任何机器上都能双击。"""
    return data.replace(b'\r\n', b'\n').replace(b'\n', b'\r\n')


def check_bat_crlf(base):
    """校验 .bat/.cmd 的行尾 / 编码，异常时打 [W] 提醒修磁盘源文件。
       （打包时 build() 会自动规范化，这里只是让开发者知道源头有问题。）"""
    bad = []
    for root, dirs, files in os.walk(base):
        dirs[:] = [d for d in dirs if d not in EXCLUDE_DIRS]
        for name in files:
            if not name.lower().endswith(('.bat', '.cmd')):
                continue
            p = os.path.join(root, name)
            with open(p, 'rb') as f:
                data = f.read()
            crlf = data.count(b'\r\n')
            lf = data.count(b'\n') - crlf
            nonascii = sum(1 for b in data if b > 127)
            if crlf == 0 and lf > 0:
                bad.append((os.path.relpath(p, base),
                            'LF-only 行尾（%d 个 \\n / 0 个 \\r\\n）' % lf))
            elif nonascii:
                bad.append((os.path.relpath(p, base),
                            '含 %d 个非 ASCII 字节' % nonascii))
            # REM 注释里的 ">" 是隐患：行尾一旦退化成 LF，cmd 会把它当重定向符，
            # 既报「不是内部或外部命令」又会在当前目录建出乱码名的 0 字节垃圾文件
            for line in data.split(b'\n'):
                s = line.strip()
                if s.upper().startswith(b'REM') and b'>' in s:
                    bad.append((os.path.relpath(p, base),
                                'REM 注释含 ">"：%r' % s[:48]))
                    break
    if not bad:
        log('  .bat/.cmd 行尾: 全部 CRLF + 纯 ASCII  OK')
        return
    for rel, why in bad:
        log('[W] %s: %s —— 双击可能报「不是内部或外部命令」，请改为 CRLF + 纯 ASCII'
            % (rel, why))


def check_retired(base):
    """校验 retired_files.json —— 「旧版遗留文件」的清理清单。

    启动器（BD2MAA-Updater.ps1 的 Remove-RetiredFiles）每次启动、以及每次自动更新完成后
    都会按这份清单从用户目录里删文件。所以**清单写错 = 用户磁盘被删错**，四条铁律：

      [R1] 清单里的路径**不得存在于磁盘** —— 否则就是「一边派发、一边删除」，用户更新完
           会立刻少一个文件。（它们本来就不会进包：`EXCLUDE_FILES |= RETIRED_FILES`
           已把整份清单兜底排除，所以 [R1] 检查的是磁盘，而不是包内。）
      [R2] 必须能在 git 历史里检出「曾经被添加过」—— 挡住拼错文件名。
           拼错的路径 R1 会因为「文件不存在」而误判通过，只有历史能证伪。
      [R3] 若仍被 git 跟踪（HEAD 里还有），给个提醒：仓库里留着这份文件，别人 clone 后
           会带着它；维护者本机则由启动器的「存在 .git 就跳过」逻辑兜住，不会被误删。
    """
    p = os.path.join(base, 'retired_files.json')
    if not os.path.exists(p):
        log('[W] retired_files.json 不存在 —— 启动器没法清理旧版遗留的说明文档 / 视频；发布前补上')
        return []
    try:
        with open(p, 'rb') as f:
            data = json.loads(f.read().decode('utf-8-sig'))
    except Exception as e:
        log('[!] retired_files.json 解析失败：%s' % e)
        return ['retired_files.json 解析失败']

    rels = [str(x).replace('\\', '/') for x in (data.get('files') or [])]
    shipped = {r for _, r in collect(base)}
    problems = []
    log('  retired_files.json：%d 项（启动器会从用户目录里删掉这些）' % len(rels))
    for rel in rels:
        why = []
        if os.path.exists(os.path.join(base, rel)):
            why.append('磁盘上仍存在 → 会一边派发一边删除')
        if rel in shipped:
            why.append('仍在打包清单里')
        rc, out = _git(base, 'log', '--all', '--diff-filter=A', '--format=%h', '--', rel)
        if rc != 0 or not out.strip():
            why.append('git 历史里查不到 → 疑似拼错文件名')
        rc2, out2 = _git(base, 'ls-files', '--error-unmatch', '--', rel)
        if rc2 == 0:
            log('    [W] %s：仓库里仍跟踪着这份文件（维护者本机由启动器的 .git 判断跳过；'
                '别人 clone 后会带着它）' % rel)
        if why:
            problems.append((rel, '；'.join(why)))
            log('    [!] %s：%s' % (rel, '；'.join(why)))
        else:
            log('    OK  %s' % rel)
    return problems


def _git(base, *args):
    """跑一条只读 git 命令，返回 (exitcode, stdout)。git 不存在时返回 (1, '')。"""
    try:
        r = subprocess.run(['git', '-C', base] + list(args),
                           stdout=subprocess.PIPE, stderr=subprocess.DEVNULL)
        return r.returncode, r.stdout.decode('utf-8', 'replace')
    except Exception:
        return 1, ''


def prompt_version(current, head_v):
    """交互式询问版本号（仅在 stdin 是 TTY 时调用）。
       无效输入会循环追问；空回车返回 None（视为取消）。"""
    cur = current or '?'
    while True:
        if head_v and head_v != current:
            sys.stdout.write('[?] 当前: %s  (HEAD 上次发布: %s)\n' % (cur, head_v))
        else:
            sys.stdout.write('[?] 当前: %s\n' % cur)
        sys.stdout.write('[?] 目标版本号 (例 v26.09.6，回车取消): ')
        sys.stdout.flush()
        try:
            line = sys.stdin.readline()
        except (EOFError, KeyboardInterrupt):
            return None
        line = line.strip()
        if not line:
            return None
        v = line if line.startswith('v') else 'v' + line
        if VERSION_RE.match(v):
            return v
        sys.stdout.write('[!] 格式不对，应为 v<主>.<次>.<修订>，例如 v26.09.6\n')


def confirm(msg):
    """仅在 stdin 是 TTY 时询问 y/N；非交互环境默认 True（CI / 管道场景）。"""
    if not sys.stdin.isatty():
        return True
    sys.stdout.write('[?] %s [y/N] ' % msg)
    sys.stdout.flush()
    try:
        ans = sys.stdin.readline().strip().lower()
    except (EOFError, KeyboardInterrupt):
        return False
    return ans in ('y', 'yes')


def collect(base):
    """按排除规则收集要打包的文件，返回 [(绝对路径, 包内相对路径)]"""
    out = []
    for root, dirs, fnames in os.walk(base):
        dirs[:] = [d for d in dirs if d not in EXCLUDE_DIRS]
        for fn in fnames:
            full = os.path.join(root, fn)
            rel = os.path.relpath(full, base).replace('\\', '/')
            if rel.split('/')[0] in EXCLUDE_DIRS:
                continue
            if rel in EXCLUDE_FILES or fn in EXCLUDE_FILES:
                continue
            # Office 临时锁文件：Excel/Word/PPT 打开文档时在同目录生成 `~$<原名>`
            # （例：用户开着 Verlog.xlsx 时会出现 `~$Verlog.xlsx`）。它是 0 字节左右的
            # 隐藏状态文件，用户侧毫无用处；写死的名字清单挡不住它（打开哪个文档就生成哪个），
            # 所以按前缀统一排除。
            if fn.startswith('~$'):
                continue
            if os.path.splitext(fn)[1].lower() in EXCLUDE_EXT:
                continue
            out.append((full, rel))
    out.sort(key=lambda x: x[1])
    return out


def build(base, out_path, version):
    """按磁盘状态打包。版本号已在上游通过 sync_version_anchors 写进各锚点文件。"""
    files = collect(base)
    log('[1] 待打包文件 = %d' % len(files))

    if os.path.exists(out_path):
        os.remove(out_path)
    os.makedirs(os.path.dirname(out_path) or '.', exist_ok=True)

    dt = time.localtime()[:6]          # 条目时间戳 = 打包时刻（秒级）：同源两次构建 sha256 必然不同，校验请比条目内容
    raw = 0
    t0 = time.time()
    with zipfile.ZipFile(out_path, 'w', zipfile.ZIP_DEFLATED, compresslevel=6) as z:
        for full, rel in files:
            data = open(full, 'rb').read()
            if rel.lower().endswith(('.bat', '.cmd')):
                data = normalize_crlf(data)   # 批处理强制 CRLF，见 normalize_crlf 注释
            zi = zipfile.ZipInfo(rel, date_time=dt)
            zi.compress_type = zipfile.ZIP_DEFLATED
            zi.external_attr = 0o644 << 16
            z.writestr(zi, data)
            raw += len(data)

    size = os.path.getsize(out_path)
    log('[2] 原始 %d 字节 -> zip %d 字节 (%.2f MiB)  用时 %.1fs'
        % (raw, size, size / 1048576.0, time.time() - t0))
    return out_path


def verify(path, version):
    ok = True
    with zipfile.ZipFile(path) as z:
        bad = z.testzip()
        names = z.namelist()
        log('[V1] testzip() = %s' % bad)
        if bad:
            ok = False
        log('[V2] 条目数 = %d' % len(names))

        # 在**产出的 zip 内**逐个锚点复核版本号：磁盘改对了但包内是旧文件的情况也会被抓到
        for rel, desc, expect in VERSION_ANCHORS:
            if rel not in names:
                log('[V3] %-16s 不在包里！' % rel)
                ok = False
                continue
            hits = [m.group(2).decode('ascii')
                    for m in VERSION_FIELD_RE.finditer(z.read(rel))]
            good = (len(hits) == expect and all(h == version for h in hits))
            log('[V3] %-16s 版本 = %s  期望 %s  %s'
                % (rel, hits, version, 'OK' if good else '<= 不符预期'))
            if not good:
                ok = False

        miss = [k for k in REQUIRED_FILES if k not in names]
        miss += ['%s(%d)' % (d, sum(1 for x in names if x.startswith(d)))
                 for d in REQUIRED_DIRS if not any(x.startswith(d) for x in names)]
        log('[V4] 缺失必需项 = %s' % (miss if miss else '无'))
        if miss:
            ok = False

        leaked = [x for x in names
                  if x.split('/')[0] in ('config', 'cache', 'debug', 'updates', '.git', '.workbuddy')
                  or x.endswith('.lnk') or x in EXCLUDE_FILES]
        log('[V5] 排除项泄漏 = %s' % (leaked[:5] if leaked else '无'))
        if leaked:
            ok = False

        # [V9] 退休清单里的旧文件不得出现在包内 —— 否则用户更新完立刻被启动器删掉，
        # 等于白占一次下载体积（同时也是 check_retired [R1] 的产物侧复核）。
        retired, hit = [], []
        if 'retired_files.json' in names:
            try:
                retired = [str(x).replace('\\', '/') for x in
                           (json.loads(z.read('retired_files.json').decode('utf-8-sig'))
                            .get('files') or [])]
            except Exception as e:
                log('[V9] retired_files.json 读不出来：%s' % e)
                ok = False
            hit = [r for r in retired if r in names]
        log('[V9] 退休清单（%d 项）撞车 = %s' % (len(retired), hit if hit else '无'))
        if hit:
            ok = False

        nonascii = [x for x in names if any(ord(c) > 127 for c in x)]
        bad_flag = [x for x in nonascii if not (z.getinfo(x).flag_bits & 0x800)]
        log('[V6] 中文名条目 = %d，缺 UTF-8 标志位 = %s'
            % (len(nonascii), bad_flag if bad_flag else '无'))
        if bad_flag:
            ok = False

    h = hashlib.sha256()
    with open(path, 'rb') as f:
        for chunk in iter(lambda: f.read(1 << 20), b''):
            h.update(chunk)
    log('[V7] sha256 = %s' % h.hexdigest())
    log('[V8] size   = %d bytes' % os.path.getsize(path))
    log('=== %s ===' % ('全部通过' if ok else '存在问题'))
    return ok


def main():
    ap = argparse.ArgumentParser(add_help=True)
    ap.add_argument('version', nargs='?', help='目标版本号，如 v26.09.9；不传则交互询问')
    ap.add_argument('--out', help='输出 zip 路径，默认 updates/MABd2<version>.zip')
    ap.add_argument('--verify', action='store_true', help='只校验已有 zip')
    ap.add_argument('--no-verify', action='store_true', help='只构建不校验')
    ap.add_argument('--no-bump', action='store_true',
                    help='不改任何版本号文件（跳过锚点同步），按磁盘现状打包')
    ap.add_argument('--dry-run', action='store_true', help='只打印发布计划，不动磁盘也不出包')
    args = ap.parse_args()

    head_v = head_version(BASE)
    cur_v = disk_version(BASE)

    # ---- 1. 版本号解析 ----
    version = args.version
    if not version:
        if sys.stdin.isatty():
            version = prompt_version(cur_v, head_v)
            if not version:
                log('已取消（无输入）')
                return 1
        else:
            log('无版本号且 stdin 非 TTY，无法交互询问。请显式传入，例如 v26.09.9')
            return 2

    if not version.startswith('v'):
        version = 'v' + version
    if not VERSION_RE.match(version):
        log('版本号格式不对：%r（应为 v<主>.<次>.<修订>，例 v26.09.9）' % version)
        return 2

    out = args.out or os.path.join(BASE, 'updates', 'MABd2%s.zip' % version)

    # ---- 2. 打印发布计划 ----
    log('=== 发布计划 ===')
    log('  目标版本: %s' % version)
    log('  HEAD   :  %s' % (head_v or '?'))
    log('  磁盘    :  %s' % (cur_v or '?'))

    sites = scan_version_anchors(BASE)
    pending = []          # 需要改写的 (rel, 行号, 旧版本)
    log('  版本锚点:')
    for rel, desc, expect in VERSION_ANCHORS:
        info = sites[rel]
        if not info['hits']:
            log('    %-16s (无 version 字段 —— %s)' % (rel, desc))
            continue
        for lineno, old in info['hits']:
            if args.no_bump:
                mark = '保持（--no-bump）'
            elif old == version:
                mark = '已是目标版本'
            else:
                mark = '-> %s' % version
                pending.append((rel, lineno, old))
            log('    %-16s :%-4d %s  %s' % (rel, lineno, old, mark))

    will_bump = (not args.no_bump) and bool(pending)
    if pending and args.no_bump:
        log('[!] --no-bump：包内这 %d 处版本号将不是 %s，发布前请确认这是你要的' % (len(pending), version))

    log('  输出    :  %s' % out)
    check_version_json(BASE)
    check_verlog(BASE, version)
    check_bat_crlf(BASE)
    check_retired(BASE)

    if args.dry_run:
        log('[dry-run] 已打印计划，未执行任何写入')
        return 0

    # ---- 3. 只校验 ----
    if args.verify:
        if not os.path.exists(out):
            log('找不到 zip：%s' % out)
            return 2
        return 0 if verify(out, version) else 1

    # ---- 4. 改动前确认 ----
    if will_bump and not confirm('将同步改写 %d 处版本号（%s）为 %s 并开始打包？'
                                 % (len(pending), '、'.join(sorted({r for r, _, _ in pending})), version)):
        log('已取消')
        return 1

    # ---- 5. 写盘：同步所有版本锚点 ----
    report, problems = sync_version_anchors(BASE, version, apply=will_bump)
    for rel, lineno, old, new, act in report:
        if act == '未变':
            continue
        log('[0] %s:%-4d  %s -> %s  (%s)' % (rel, lineno, old, new or '-', act))
    for rel, why in problems:
        log('[W] 版本锚点异常 %s：%s' % (rel, why))
    if not will_bump:
        log('[0] 版本号未改动（%s）' % ('--no-bump' if args.no_bump else '已是目标版本'))

    # ---- 6. 打包 + 校验 ----
    build(BASE, out, version)
    if not args.no_verify:
        ok = verify(out, version)
    else:
        ok = True

    if ok:
        log('')
        log('下一步：')
        log('  git add -A && git commit -m "%s" && git push' % version)
        log('  把 %s 上传到 GitHub Release 的 assets（**不要改文件名**）' % os.path.basename(out))
        return 0
    return 1


if __name__ == '__main__':
    sys.exit(main())

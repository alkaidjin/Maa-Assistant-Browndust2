# -*- coding: utf-8 -*-
"""
BD2MAA 发布包构建工具

用法：
    python data/maintainer/tools/build_release_zip.py                 # 无参 → 交互式询问版本号
    python data/maintainer/tools/build_release_zip.py v26.09.9        # 显式指定版本号
    python data/maintainer/tools/build_release_zip.py --dry-run       # 只打印要做什么，不动磁盘也不出包
    python data/maintainer/tools/build_release_zip.py --no-bump       # 不改任何版本号文件，按磁盘现状打包
    python data/maintainer/tools/build_release_zip.py --out D:\\x.zip  # 指定输出 zip 路径
    python data/maintainer/tools/build_release_zip.py --verify        # 只校验已有 zip

🖱️ **双击入口**：同目录下的 `build_release_zip.bat`（v26.09.14 重建）。
  为什么必须有它 —— 本机 `.py` 的双击关联是 `C:\\Windows\\py.exe`，而 py launcher 里
  没有注册任何 Python（系统 Python 3.10 已卸载、venv / uv 环境不向它注册）
  → 直接双击 `.py` 只会「窗口闪一下就没了」。.bat 负责自己找解释器（依次尝试
  `%USERPROFILE%\\.venvs\\py313\\python.exe`、WorkBuddy 托管解释器、`py -3`、`python`），
  找到后调本脚本，跑完 `pause` 等你按键才关窗，出错也会停住让你看得见。

  命令行 / agent 直调：
      # 本机主 Python 环境
      "C:\\Users\\ALKAID\\.venvs\\py313\\python.exe" data/maintainer/tools/build_release_zip.py
      # 托管解释器（一定存在）
      "C:\\Users\\ALKAID\\.workbuddy\\binaries\\python\\versions\\3.13.12\\python.exe" \
          data/maintainer/tools/build_release_zip.py
      # 批处理（agent 代跑）时 stdin 不可交互，加 --no-bump / 显式版本号
      printf 'y\\n' | <python> data/maintainer/tools/build_release_zip.py --no-bump

  本脚本与其余维护者工具都住在 `data/maintainer/` —— 那是**不入包的档案目录**（见 EXCLUDE_DIRS）。
  为什么必须搬进去：MXU 的软件内更新走「全量覆盖」，会把解压包里出现的根级同名条目
  **整个搬到 cache/old、搬不动就删**，`tools/` / `agent/` 这类目录会被整目录换掉 ——
  放在项目根下的维护者文件（打包器、备份器、图标工具）就是这么被抹掉的（2026-09-30 实测）。

约定：
  1. **默认会把所有「版本锚点」同步改写为目标版本号**（见 VERSION_ANCHORS），
     即 interface.json 的 version 字段 + 更新功能说明.md 里内嵌的那份 interface.json 片段。
     不必再一个个打开文件手改版本号；git 提交记录里也自带版本号变更。
     ⚠️ 锚点带 `expect` 防呆：某文件里这类 version 字段的处数与预期不符时**整体跳过**
     并告警（宁可漏改也不误改），文档正文里的历史叙述（如「v26.09.7 修掉了 X」）**永不被改写**。
  2. 直接读磁盘 → **未提交的改动也会进包**（所以发版前先确认工作区状态）。
  3. 排除 config/ → MXU 首次启动自动生成默认实例，避免覆盖用户已有配置。
  4. 排除 MaaBd2.lnk → 写死路径的快捷方式在别人机器上无效，首次启动自动重建。
  5. 排除 updater_cache.json / cache / debug / updates / maintainer（维护者档案目录，现住 data/ 下）自身，
     以及 Office 临时锁文件 `~$*`（打开 Verlog.xlsx 时会生成 `~$Verlog.xlsx`）。
     v26.09.8 起额外排除「非维护者不需要」的开发脚本与仓库元数据 —— 见 EXCLUDE_FILES 注释。
  6. 中文文件名必须带 UTF-8 标志位（0x800），否则 Windows 解压乱码。
  7. zip 内条目时间戳 = **打包时刻（秒级）**，不是固定值 → 同源两次构建 sha256 必然不同。
     校验请比「条目内容 / 条目数 / interface.json 版本号」，**不要比 zip 整体 hash、也不要复用旧 zip**。
  8. 批处理（.bat/.cmd）入包前强制规范化为 CRLF 行尾（见 normalize_crlf 注释）。
  9. verify() 逐项校验 REQUIRED_FILES / REQUIRED_DIRS。v26.09.8 起把「运行时硬依赖」
     与「MXU 直接读取的配置 / 资源」也列进 REQUIRED_FILES —— 缺一项就在打包阶段报错，
     不再像原先那样只查目录非空、缺文件却静默通过。
 10. verify() 还会**在产出的 zip 里**逐个锚点复核版本号（[V3]）——磁盘改对了、
     但包内文件是旧的这种情况也会被抓到。

关于「还有哪些文件带版本号」——本工具的处理分三档：
  ✅ 自动改写：VERSION_ANCHORS 列出的（interface.json、更新功能说明.md）
  🔔 只提示：  Verlog.xlsx（版本发布日志表，**维护者文档**：maintainer/Verlog.xlsx，
              不进用户包）——本版没有记录时提醒你补一条。
              它记的是「这一版改了什么」，内容必须由人写，工具不去猜。
  ⛔ 刻意不碰：version.json（依赖指纹 maafw/mxu，不是业务版本号，升级底层后人工同步，
               见下方 check_version_json）；updater_cache.json（运行时缓存）；
               各文档正文里的历史叙述（v26.09.7 / v26.09.6 … 是事实，改了就是篡改历史）。
"""
import os, sys, json, time, re, zipfile, hashlib, argparse, subprocess

def _find_base():
    """定位项目根：从脚本所在目录逐级向上找「同时含 interface.json 与 MaaBd2.exe」的目录。

    本工具住在 `data/maintainer/tools/`（不入包的维护者档案目录，见 EXCLUDE_DIRS），
    所以「脚本上一级的上一级」不再等于项目根 —— 写死层数会在再次挪动时静默打错包。
    改用向上探测，换目录 / 换深度都不用改代码。探测不到时退回旧的两层逻辑并让后续
    校验报错（BASE 错了会立刻在 VERSION_ANCHORS / REQUIRED_FILES 处暴露）。
    v26.09.14 起 exe 改名 MaaBd2.exe（图标已固化进 PE 资源）；这里兼容回退名 mxu.exe，
    万一哪天要回滚改名也不至于静默打错包。"""
    d = os.path.dirname(os.path.abspath(__file__))
    for _ in range(6):
        if (os.path.isfile(os.path.join(d, 'interface.json'))
                and any(os.path.isfile(os.path.join(d, x))
                        for x in ('MaaBd2.exe', 'mxu.exe'))):
            return d
        parent = os.path.dirname(d)
        if parent == d:
            break
        d = parent
    return os.path.dirname(os.path.dirname(os.path.abspath(__file__)))


BASE = _find_base()

EXCLUDE_DIRS  = {'.git', '.workbuddy', 'cache', 'config', 'debug', 'updates', '_stage',
                 # GitHub Actions 配置（PR#3 引入的 mirrorchyan_release*.yml）：只在仓库侧
                 # 生效，用户包里毫无用处，纯属冗余文件。
                 '.github',
                 # 第二个 agent（圣石洞穴「刷数量最少的石头」）的 Go 源码：与 agent/go-service.exe
                 # 同级的 `.exe` 才是运行件（见 REQUIRED_FILES），源码只入仓库、不进用户包。
                 # v26.09.14 起它住在 data/maintainer/agent/ 下，本条是护栏 —— 万一有人把它挪回
                 # agent/，仍然不会被扫进包。
                 'rock-picker',
                 # 第三个 agent（钓鱼小游戏 FishingMinigame）的 Go 源码，同上：
                 # 只有 agent/fishing.exe 进包，源码只入仓库。
                 'fishing',
                 # ⚠️ 维护者档案目录（打包器 / 备份器 / 图标工具 / 开发辅助脚本 / 退役件存档）。
                 # 整目录一次排除：它里面既有本机路径，也**绝不能**出现在用户包里。
                 # 更关键的理由见文件头「双击入口」处的说明 —— MXU 全量更新会整目录换掉
                 # 包内存在的根级同名目录，留在项目根下的维护者工具会被当场抹掉。
                 # v26.09.14 起整个目录又搬进了 data/（见本文件头「根目录瘦身」），同名匹配照旧生效。
                 'maintainer'}
# 注意：EXCLUDE_DIRS 是**按目录名**（不是按相对路径）匹配的 —— 任何层级下叫这些名字的
# 目录都会被整体跳过。当前无副作用；但若将来在 resource/ 等目录下新建名为 config / cache /
# debug 的**合法**子目录，会被静默吞掉、不进包。新增此类目录名时请先确认这里。
# 按包内相对路径匹配
EXCLUDE_FILES = {
    'MaaBd2.lnk', 'updater_cache.json',
    # v26.09.14 起根目录瘦身：除 interface.json / LICENSE / MaaBd2.exe / README.md 外
    # 全部搬进 data/。旧根级文件名登记进 retired_files.json（老用户升级时由 changes.json
    # 自动清掉），这里只留护栏。
    'mxu.exe',
    # 打包器 / 备份器的双击入口与本体：纯维护者工具，用户侧毫无用处
    # （还会暴露本机解释器路径）。它们**入仓库**，但绝不进用户包。
    # v26.09.14 起全部搬进 maintainer/（由 EXCLUDE_DIRS 整目录排除），
    # 这里保留逐条路径当护栏 —— 双保险，挪出来也进不了包。
    # （打包入口 .bat 已于 v26.09.14 删除：打包交给 agent，或直接用绝对路径调 python）
    '备份维护版.bat',
    'maintainer/tools/backup_maintainer.ps1',
    'maintainer/tools/install_backup_task.ps1',
    # ---- v26.09.8 起：非维护者不需要的文件（用户侧无用，且 start.py 写死了作者本机游戏路径）----
    'agent/start.bat', 'agent/start.py',   # agent 开发辅助脚本（写死 C:\Neowiz\... 本机游戏路径）
    'maintainer/tools/make_icon.py',        # 生成 mxu.ico / mxu_icon.png 的开发脚本
    'maintainer/tools/apply_icon.ps1',      # 手动打图标工具（打包器内嵌 ensure_icon，不依赖它）
    # 打包双击入口（v26.09.14 重建）：只对维护者有意义，且写死了本机 Python 探测顺序
    'maintainer/tools/build_release_zip.bat',
    # MaaPipelineEditor（MPE）Desktop v2.0.0 在项目根生成的编辑器清单：里面全是
    # 编辑器自身二进制的下载 URL 与 sha256，运行时（MXU / MaaFramework）根本不读它。
    # 它既不该进用户包，也不该进仓库 —— 每位维护者装不同版本 MPE 就会变。
    'm2.json',
    # ---- v26.09.14 起 ----
    # 维护者文档：按**文件名**匹配（collect() 支持「相对路径 或 文件名」双匹配），
    # 所以 maintainer/Verlog.xlsx 与误挪回根目录的 Verlog.xlsx 都进不了包。
    'Verlog.xlsx',
    # 教学视频（已下架 + 已停止 git 跟踪）：万一有人从历史里取回来放回根目录，也不进包。
    '#使用本软件自动打开游戏并设定游戏分辨率的教学视频# .mp4',
    # 本地导出的资源压缩包（resource/ 目录的临时打包产物）：20 MB 且运行时根本不读它，
    # 只会出现在维护者的工作区里。2026-09-28 发现它差点被 os.walk 扫进 v26.09.13 包体。
    'resource.zip',
    # 仓库元数据：collect() 对 EXCLUDE_FILES 是「相对路径 或 文件名」双匹配，
    # 所以这两项会连同嵌套的 git 元数据一起排除（如 resource/model/.gitignore，
    # 其内容只是 "ocr"，与根 .gitignore:29 重复）。这是预期行为 —— 用户包里不该有 git 元数据。
    '.gitattributes', '.gitignore',
    # ---- v26.09.11 起：旧版遗留文件（改名 / 下架的老文档、老视频）不再在这里逐条维护 ----
    # 它们全部由 retired_files.json 兜底排除（collect() 按 rel 精确匹配 RETIRED_FILES，见下）：
    # 那份清单与「清理旧版遗留文件」逻辑**同源**，一处维护、两边生效。
    # 历史条目（现已由清单覆盖）：注意事项1/2/3/4 四件套、旧 PDF、旧 DOCX、旧教学视频、
    # github地址.txt。新增退役文件时只改 retired_files.json，别再往这里抄名字。
}
# ---- v26.09.14 起：仓库照旧跟踪、但**不进用户包**的运行时二进制（减 MXU 更新包体）----
# 判据（都做了源码级核实，不是拍脑袋）：
#   * MaaFramework 的控制器 DLL 是**按需 LoadLibrary** 的 —— 见上游
#     source/LibraryHolder/ControlUnit/ControlUnit.cpp：每个
#     `create_control_unit()` 里才 `load_library(library_dir() / libname_)`。
#     本仓 interface.json 的 controller 只有 win32（UnityWndClass / BrownDust II），
#     ⇒ ADB / 手柄 / 自定义 / 录制 / 回放五种控制器永远不会被创建，DLL 也就永远不会被加载。
#   * MaaNode.node / MaaNodeServer.node 是 **Node.js** 的 native addon；MXU 是 Tauri(Rust)，
#     包里也没有 node 运行时 → 无人加载（反查：除它们自己外无任何二进制引用这两个名字）。
#   * MaaPiCli.exe 是命令行调试器，MXU 不调它。
# 合计 ≈ 5.1 MiB（未压缩）；这些文件**仍在 git 里**（维护者随时可放回），只是不派发。
# ⚠️ 不要顺手加 `maafw/DirectML.dll`（17.7 MiB，看起来最肥）：
#    MaaInferenceDevice 默认 **Auto**，而 Ort::GetAvailableProviders() 是否含
#    DmlExecutionProvider 是**编译期**决定的（本仓 onnxruntime_maa.dll 里确实编译进去了），
#    ⇒ 有合适 GPU 的机器上 use_directml() 会真的去 LoadLibrary 它，缺文件会在资源加载
#    阶段炸掉整个任务（见上游 source/MaaFramework/Resource/ResourceMgr.cpp）。
PRUNED_BINARIES = {
    'maafw/MaaNode.node', 'maafw/MaaNodeServer.node', 'maafw/MaaPiCli.exe',
    'maafw/MaaAdbControlUnit.dll', 'maafw/MaaGamepadControlUnit.dll',
    'maafw/ViGEmClient.dll',            # 手柄控制器 ViGEm 的客户端，随 Gamepad 一起无用
    'maafw/MaaCustomControlUnit.dll', 'maafw/MaaRecordControlUnit.dll',
    'maafw/MaaReplayControlUnit.dll',
}
EXCLUDE_FILES |= PRUNED_BINARIES

EXCLUDE_EXT   = {'.lnk', '.tmp', '.pyc'}


def load_retired(base):
    """读 retired_files.json → 旧版遗留文件名集合（文件缺失 / 损坏时返回空集）。

    (
    清单里的都是「以前发给过用户、现在不再派发」的文件。它们一旦被恢复、或在本机留了
    副本，重新收进包就是双重错误：既白占体积，又会在用户端被按同一份清单立刻删掉
    （以前是维护器删，v26.09.14 起MXU 增量更新也会按 changes.json 删）。
    同源，避免「排除表 / 清理清单」两处手工维护走偏。

    两个字段都要收：`files`（每次都按它清理，幂等）+ `versioned_files`（只在指定版本
    生效一次）。排除表里不区分它们 —— 只要迟早要被删，就不该再进包。"""
    try:
        with open(os.path.join(base, 'data', 'retired_files.json'), 'rb') as f:
            data = json.loads(f.read().decode('utf-8-sig'))
    except Exception:
        return set()

    rels = {str(x).replace('\\', '/') for x in (data.get('files') or [])}
    vfiles = data.get('versioned_files') or {}
    if isinstance(vfiles, dict):
        for key, items in vfiles.items():
            if str(key).startswith('_'):      # _ 开头的是说明字段，不是版本号
                continue
            rels |= {str(x).replace('\\', '/') for x in (items or [])}
    return rels


def load_retired_due(base, version):
    """retired_files.json 里「本次发布应当清理」的条目（有序、去重），供 changes.json 用。

    (
    与 load_retired() 的区别：后者只返回集合、供**排除表**用（只要迟早要删就不进包）；
    这里要给 changes.json 的 deleted 用，需要把两类条目合并成一份：
      * files           —— 常驻条目，每版都该出现（幂等：MXU 安装时文件不存在就跳过）
      * versioned_files —— 只在键 == 版本时生效的一次性清理
    另外这里比运行时清理多一层：versioned_files 的键 **<= 本次版本** 的一律收进来。
    运行时清理是「相等才删」，用户跳版升级时（v26.09.12 直接装 v26.09.14）那一版的一次性
    清理就永远不会发生；而 changes.json 由打包器生成、只发这一次，索性补齐。）"""
    try:
        with open(os.path.join(base, 'data', 'retired_files.json'), 'rb') as f:
            data = json.loads(f.read().decode('utf-8-sig'))
    except Exception:
        return []

    out, seen = [], set()

    def _add(items):
        for x in (items or []):
            rel = str(x).replace('\\', '/')
            if rel and rel not in seen:
                seen.add(rel)
                out.append(rel)

    _add(data.get('files') or [])

    vfiles = data.get('versioned_files') or {}
    if isinstance(vfiles, dict):
        for key, items in vfiles.items():
            if str(key).startswith('_'):      # _ 开头的是说明字段，不是版本号
                continue
            # 「键 <= 本次版本」即收：避免跳版用户漏掉某一版的一次性清理
            if _ver_key_le(str(key), version):
                _add(items)
    return out


def _ver_key_le(a, b):
    """版本号 a <= b？（v26.09.13 这类三段式；解析失败视为 False，宁可漏清理不误删）"""
    def _num(s):
        s = str(s).lstrip('vV')
        return tuple(int(p) for p in s.split('.') if p.isdigit())
    try:
        return _num(a) <= _num(b)
    except Exception:
        return False


def _assert_safe_deleted(items):
    """changes.json 的 deleted 安全断言。

    MXU 的增量安装**没有 config 保护**（它会按 deleted 直接把文件移进 cache/old），
    所以这份清单生成后必须再过一遍：越界路径 / 绝对路径 / 受保护目录一律不许出现。
    宁可在打包阶段崩掉，也不能发出一份会误删用户数据的包。"""
    for rel in items or []:
        s = str(rel).replace('\\', '/')
        assert '..' not in s.split('/'), 'deleted 含越界路径: %s' % s
        assert not s.startswith('/'), 'deleted 含绝对路径: %s' % s
        assert ':' not in s.split('/')[0], 'deleted 含盘符: %s' % s
        assert s.split('/')[0].lower() != 'config', 'deleted 命中受保护目录: %s' % s


RETIRED_FILES = load_retired(BASE)
# ⚠️ retired 条目**不并入 EXCLUDE_FILES**（v26.09.14）：清单里是「老用户根目录的相对路径」，
# 而部分名字（version.json）在新结构里住进了 data/、且要照常派发（根级那条只是老用户残留）。
# EXCLUDE_FILES 是「相对路径 或 文件名」双匹配，并进来会把 data/version.json 一并误杀。
# 所以 retired 走独立通道：collect() 里按 rel **精确匹配**根级路径（见下）。
# 需要按名护栏的个别条目（如 mxu.exe）已在上方 EXCLUDE_FILES 里显式登记。

# ---- 必备清单（verify 逐项检查，缺一项即打包失败）----
# 原则：凡是「运行时硬依赖」或「MXU 直接按路径读取」的文件，都必须在这里。
# 只靠 REQUIRED_DIRS 的目录级检查是不够的 —— 目录非空就算过，缺具体文件不会被发现。
REQUIRED_FILES = [
    # 基础入口 / 版本 / 授权
    # v26.09.14 最终布局：resource / tasks / misc / agent 四个业务目录**留在根目录**
    # （MXU 靠 interface.json 里的相对路径找它们，动不得位置无所谓但不动最省心）；
    # 只有「维护者档案 + 版本指纹 + 次要文档」住进 data/（version.json / retired 清单 /
    # 更新功能说明.md / maintainer）。NOTICE.md 留在根目录（LGPL 履约要显眼）。
    # exe 改名 MaaBd2.exe 且图标已固化进 PE 资源（打包前 ensure_icon 会复核并自动补打）。
    'interface.json', 'data/version.json', 'MaaBd2.exe', 'LICENSE',
    'README.md', 'NOTICE.md',
    'data/更新功能说明.md',
    # 注意事项 docx **刻意留在根目录**（v26.09.14 用户决定）：它是「使用前必看」，
    # 根目录就那么几个文件，解压后第一眼能看见；塞进子目录就没人翻了。
    '#请务必打开此文档查阅#含使用方式以及常见问题解答.docx',
    # 旧版遗留文件清理清单：MXU 增量更新（changes.json）按它删掉改名 / 下架的老文档与老视频
    'data/retired_files.json',
    # ⚠️ Verlog.xlsx（版本发布日志表）是**维护者文档**：data/maintainer/Verlog.xlsx，
    #    不进用户包（用户只看 data/更新功能说明.md 与 README）。
    #    它在 EXCLUDE_FILES 里留了一条同名护栏，挪到哪都进不了包。
    # LGPL-3.0 履约：MaaFramework 的许可证全文必须随包派发（源自上游 release zip 的 LICENSE.md）
    'maafw/LICENSE.md',

    # ---- v26.09.8 起新增：运行时硬依赖，缺了包就是坏的 ----
    # agent：周门禁 / 自定义识别 / 自定义动作全靠它，agent.child_exec = agent/go-service
    'agent/go-service.exe',
    # 第二个 agent（PI v2 的 agent 支持对象数组，MXU 2.5.3 逐个启动）：
    # 狩猎场-圣石洞穴「自动刷数量最少的圣石」用的自定义识别器 LeastRockPicker。
    # child_exec = agent/rock-picker（CreateProcess 自动补 .exe）。
    'agent/rock-picker.exe',
    # 第三个 agent：钓鱼小游戏。提供 custom action FishingMinigame
    # （resource/pipeline/AutoFishing.json 的 Fishing_Minigame 节点）。
    # child_exec = agent/fishing（CreateProcess 自动补 .exe）。
    'agent/fishing.exe',
    # MXU 直接按 interface.json 的路径读取：icon / license / languages
    'misc/MaaEnd-Tiny.png',
    'misc/LICENSE_SHORT.md',
    # v26.09.14 起界面仅声明简体中文（任务文案本来就是中文硬编码，其余 4 份
    # locale 只有 7 个键、长期无人维护）；go-service 守护页的 5 语言在下段保留。
    'misc/locales/zh_cn.json',
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
    # 核心运行库（MaaBd2.exe 依赖；整目录 maafw/ 其余文件由 REQUIRED_DIRS 兜底）
    'maafw/MaaFramework.dll',
]
# v26.09.14：tools/ 已移出（维护器退役，见上方 REQUIRED_FILES 注释）
# v26.09.14 最终布局：resource/tasks/misc/agent **留在根目录**（interface.json 的
# resource.path / import / agent.child_exec 指向的就是根级路径）；
# maafw/ 与 locales/ 因 MXU / go-service 硬编码相对 exe 目录的路径，也必须留在根。
REQUIRED_DIRS = ['agent/', 'maafw/', 'misc/', 'tasks/', 'resource/',
                 'locales/', 'locales/go-service/HTML/']

VERSION_RE = re.compile(r'^v\d+\.\d+\.\d+$')

# ---- 版本锚点：一次版本 bump 需要同步改写的位置 ----
# 每项 = (相对路径, 说明, expect)；expect = 该文件里「version 字段」必须恰好出现的处数。
# 只列「表示当前版本」的地方；文档正文里的历史叙述（「v26.09.7 修掉了 X」）不在此列。
# expect 是防呆闸：处数不符 → 该文件**整体跳过**并告警，逼人看一眼再决定，
# 避免文档结构变动后把历史引用的版本号一起改掉。
VERSION_ANCHORS = [
    ('interface.json',
     'PI 主版本号（MXU 界面 / 软件内更新 / GitHub 更新比对都读它）', 1),
    ('data/更新功能说明.md',
     '第二节内嵌的 interface.json 片段（当前状态副本，须与主版本号一致）', 1),
]

# 只匹配「被引号包住的 JSON 字段」形态：  "version": "v26.09.8"
# 用 bytes 正则 → 正文里裸露的 v26.09.7 一个都不碰，且原样保留编码 / BOM / 缩进 / 换行。
VERSION_FIELD_RE = re.compile(rb'("version"\s*:\s*")(v\d+\.\d+\.\d+)(")')


def log(msg):
    sys.stdout.write(str(msg) + '\n')
    sys.stdout.flush()


# ---- 图标固化（v26.09.14 起）----
# MaaBd2.exe 的默认图标 = 本项目图标（rcedit 写进 PE 资源）。MXU 自更新走「覆盖包内 exe」，
# 包里的 exe 自带图标 → 用户更新后图标**不会**回默认，这才是「永久固化」的实现方式。
# 本函数在打包前复核：exe 内嵌的 256×256 图标若与 data/maintainer/tools/mxu_icon.png
# 字节不一致（例如升级 MXU 官方新 exe 后图标丢了），自动调 rcedit 补打。
def _read_pe_icon_png(exe_path):
    """从 PE 资源里取出最大的 RT_ICON 条目（256×256 通常是内嵌 PNG）。取不到返回 None。"""
    import struct
    try:
        with open(exe_path, 'rb') as f:
            data = f.read()
        pe = struct.unpack_from('<I', data, 0x3C)[0]
        if data[pe:pe + 4] != b'PE\0\0':
            return None
        coff = pe + 4
        numsec = struct.unpack_from('<H', data, coff + 2)[0]
        optsize = struct.unpack_from('<H', data, coff + 16)[0]
        opt = coff + 20
        magic = struct.unpack_from('<H', data, opt)[0]
        if magic not in (0x10b, 0x20b):
            return None
        res_rva = struct.unpack_from('<I', data,
                                     opt + (112 if magic == 0x20b else 96) + 16)[0]
        sec0 = opt + optsize
        secs = []
        for i in range(numsec):
            vsz, va, rsz, rptr = struct.unpack_from('<IIII', data, sec0 + i * 40 + 8)
            secs.append((va, vsz, rptr, rsz))

        def rva2off(rva):
            for va, vsz, rptr, rsz in secs:
                if va <= rva < va + max(vsz, rsz):
                    return rptr + (rva - va)
            return None

        base = rva2off(res_rva)
        if base is None:
            return None

        def pdir(off):
            nn, ni = struct.unpack_from('<HH', data, off + 12)
            return [struct.unpack_from('<II', data, off + 16 + i * 8)
                    for i in range(nn + ni)]

        rt_icon = next((ofs & 0x7FFFFFFF for nm, ofs in pdir(base) if nm == 3), None)
        if rt_icon is None:
            return None
        best = None
        for _nm2, ofs2 in pdir(base + rt_icon):
            for _nm3, ofs3 in pdir(base + (ofs2 & 0x7FFFFFFF)):
                drva, dsz = struct.unpack_from('<II', data, base + ofs3)
                o = rva2off(drva)
                if o is None:
                    continue
                blob = data[o:o + dsz]
                if best is None or len(blob) > len(best):
                    best = blob
        return best
    except Exception:
        return None


def ensure_icon(base):
    """复核 MaaBd2.exe 的内嵌图标；不是我们的（或缺失）就用 rcedit 补打。
    图标源与 rcedit 都在 data/maintainer/tools/ 下（维护者档案目录，不入包）。"""
    exe = None
    for name in ('MaaBd2.exe', 'mxu.exe'):
        p = os.path.join(base, name)
        if os.path.isfile(p):
            exe = p
            break
    if not exe:
        log('[W] 找不到 MaaBd2.exe，跳过图标复核')
        return
    want_png = os.path.join(base, 'data', 'maintainer', 'tools', 'mxu_icon.png')
    if not os.path.isfile(want_png):
        log('[W] %s 不存在，跳过图标复核' % os.path.relpath(want_png, base))
        return
    want = open(want_png, 'rb').read()
    cur = _read_pe_icon_png(exe)
    if cur == want:
        log('  图标: %s 内嵌 256px 图标与源图一致  OK' % os.path.basename(exe))
        return
    rcedit = os.path.join(base, 'data', 'maintainer', 'tools', 'rcedit-x64.exe')
    ico = os.path.join(base, 'data', 'maintainer', 'tools', 'mxu.ico')
    if not (os.path.isfile(rcedit) and os.path.isfile(ico)):
        log('[W] rcedit / mxu.ico 缺失，无法自动补图标 —— 请跑 data/maintainer/tools/apply_icon.ps1')
        return
    try:
        # 运行中的 exe 无法以写模式打开 —— 先探测，给出可读的告警而不是让 rcedit 报错
        with open(exe, 'r+b'):
            pass
    except PermissionError:
        log('[W] %s 正在运行，无法补打图标 —— 关闭后重新打包' % os.path.basename(exe))
        return
    r = subprocess.run([rcedit, exe, '--set-icon', ico],
                       stdout=subprocess.PIPE, stderr=subprocess.STDOUT)
    cur2 = _read_pe_icon_png(exe) if r.returncode == 0 else None
    if cur2 == want:
        log('  图标: 已用 rcedit 重新固化到 %s  OK' % os.path.basename(exe))
    else:
        log('[W] 图标固化失败 (rcedit exit=%s) —— 请手动跑 apply_icon.ps1' % r.returncode)


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
    # v26.09.14 起 Verlog.xlsx 是**维护者文档**，住在 maintainer/ 下、不进用户包。
    # （这里仍按新路径读；万一有人挪回根目录，下面再兜一次，不让检查失效。）
    p = os.path.join(base, 'data', 'maintainer', 'Verlog.xlsx')
    if not os.path.exists(p):
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
    p = os.path.join(base, 'data', 'version.json')
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


def check_retired(base, version=None):
    """校验 retired_files.json —— 「旧版遗留文件」的清理清单。

    v26.09.14 起：这份清单会被写进包内的 changes.json（deleted），由 **MXU 软件内更新**
    在增量安装时执行删除；此前是维护器（BD2MAA-Maintainer.ps1 的 Remove-RetiredFiles）
    每次运行时删。所以**清单写错 = 用户磁盘被删错**，四条铁律：

      [R1] 清单里的路径**不得存在于磁盘** —— 否则就是「一边派发、一边删除」，用户更新完
           会立刻少一个文件。（它们本来就不会进包：collect() 按精确路径匹配 RETIRED_FILES
           已把整份清单兜底排除，所以 [R1] 检查的是磁盘，而不是包内。）
      [R2] 必须能在 git 历史里检出「曾经被添加过」—— 挡住拼错文件名。
           拼错的路径 R1 会因为「文件不存在」而误判通过，只有历史能证伪。
      [R3] 若仍被 git 跟踪（HEAD 里还有），给个提醒：仓库里留着这份文件，别人 clone 后
           会带着它；维护者本机则由清理逻辑的「存在 .git 就整体跳过」兜住，不会被误删。

      [R4] versioned_files 的**版本键必须对得上本次发布的目标版本**。启动器只在自己读到
           的 interface.json 版本号与键相等时才清理，键写错（比如发布到了别的版本号）
           不会删错任何东西，但清理会静默失效 —— 老文件继续留在用户目录里。"""
    p = os.path.join(base, 'data', 'retired_files.json')
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
    vfiles = data.get('versioned_files') or {}
    vgroups = {}
    if isinstance(vfiles, dict):
        for key, items in vfiles.items():
            if str(key).startswith('_'):
                continue
            paths = [str(x).replace('\\', '/') for x in (items or [])]
            vgroups[str(key)] = paths
            rels += paths
    shipped = {r for _, r in collect(base)}
    problems = []
    log('  retired_files.json：常驻 %d 项%s（启动器会从用户目录里删掉这些）'
        % (len(rels) - sum(len(v) for v in vgroups.values()),
           ''.join(' + %s 一次性 %d 项' % (k, len(v)) for k, v in sorted(vgroups.items()))))
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

    # [R4] 一次性清理的版本键必须与本次发布的目标版本对得上，否则启动器一律跳过
    if vgroups:
        if version and version in vgroups:
            log('    OK  一次性清理 %s：版本键与本次发布目标一致，装完会在用户端生效' % version)
        elif version:
            log('    [W] versioned_files 里没有键等于本次目标版本 %s（现有：%s）'
                ' —— 这批清理在用户端不会触发，老文件会留在磁盘上'
                % (version, '、'.join(sorted(vgroups)) or '无'))
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
            # retired 清单（老用户根级遗留）只按相对路径精确匹配：
            # 'NOTICE.md' 挡根级残留，不挡 NOTICE.md（新版照常派发）
            if rel in RETIRED_FILES:
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
    # [1b] 报告被裁剪的运行时二进制（PRUNED_BINARIES）：它们**仍在 git 里**，只是不派发。
    # 打出来是为了让「包体为什么变小了」可追溯 —— 别哪天有人疑惑 maafw/ 里少了东西。
    hit = []
    for rel in sorted(PRUNED_BINARIES):
        full = os.path.join(base, rel.replace('/', os.sep))
        if os.path.exists(full):
            hit.append((rel, os.path.getsize(full)))
    if hit:
        log('[1b] 裁剪运行时未使用二进制 %d/%d 个，省 %.2f MiB（未压缩）'
            % (len(hit), len(PRUNED_BINARIES), sum(s for _, s in hit) / 1048576.0))
        for rel, s in hit:
            log('       - %s (%.2f MiB)' % (rel, s / 1048576.0))
    else:
        log('[1b] PRUNED_BINARIES 在磁盘上一个都不存在（仓库里还有？）—— 确认是否误删')

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

        # [C1] changes.json（MXU 增量更新清单）：deleted = retired 累积清单。
        # 背景：MXU 的软件内更新「只覆盖、从不删除」，而 retired 遗留清理原本只有
        # 启动器会执行。用户改用 MXU「设置-更新」通道后（v26.09.13 起已跑通），
        # 靠这份清单让 MXU 走增量路径、安装时把清单内文件移进 cache/old
        # （不存在即跳过，幂等）。added/modified 留空：MXU 增量模式本来就复制整个
        # 解压目录，起作用的只有 deleted。
        deleted = load_retired_due(base, version)
        _assert_safe_deleted(deleted)
        z.writestr('changes.json', json.dumps(
            {'added': [], 'modified': [], 'deleted': deleted},
            ensure_ascii=False, indent=2))
        log('[C1] changes.json: deleted = %d 项' % len(deleted))

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
                  if x.split('/')[0] in ('config', 'cache', 'debug', 'updates', '.git', '.workbuddy', 'maintainer')
                  or x.startswith('cache/') or x.startswith('data/updates/')
                  or x.startswith('data/maintainer/') or x.startswith('debug/')
                  or x.startswith('config/')
                  or x.endswith('.lnk') or x in EXCLUDE_FILES]
        log('[V5] 排除项泄漏 = %s' % (leaked[:5] if leaked else '无'))
        if leaked:
            ok = False

        # [V9] 退休清单里的旧文件不得出现在包内 —— 否则用户更新完立刻被启动器删掉，
        # 等于白占一次下载体积（同时也是 check_retired [R1] 的产物侧复核）。
        retired, hit = [], []
        if 'data/retired_files.json' in names:
            try:
                # files（常驻）+ versioned_files.<版本>（一次性）都不能出现在包内
                rdata = json.loads(z.read('data/retired_files.json').decode('utf-8-sig'))
                retired = [str(x).replace('\\', '/') for x in (rdata.get('files') or [])]
                _vf = rdata.get('versioned_files') or {}
                if isinstance(_vf, dict):
                    for _k, _items in _vf.items():
                        if not str(_k).startswith('_'):
                            retired += [str(x).replace('\\', '/') for x in (_items or [])]
            except Exception as e:
                log('[V9] retired_files.json 读不出来：%s' % e)
                ok = False
            hit = [r for r in retired if r in names]
        log('[V9] 退休清单（%d 项）撞车 = %s' % (len(retired), hit if hit else '无'))
        if hit:
            ok = False

        # [V10] changes.json（MXU 增量删除清单）：必须与 retired 清单严格一致，
        # 且不得含越界 / 受保护目录路径 —— MXU 的增量删除没有 config 保护，
        # 这份清单发出去就是「用户目录里会被移走的文件列表」，不能有任何意外。
        if 'changes.json' not in names:
            log('[V10] 包内缺少 changes.json！')
            ok = False
        else:
            try:
                cj = json.loads(z.read('changes.json').decode('utf-8-sig'))
                got = [str(x).replace('\\', '/') for x in (cj.get('deleted') or [])]
                want = load_retired_due(BASE, version)
                log('[V10] changes.json deleted = %d 项（期望 %d）' % (len(got), len(want)))
                if sorted(got) != sorted(want):
                    log('[V10] 与 retired 清单不一致：多=%s 少=%s'
                        % (sorted(set(got) - set(want)), sorted(set(want) - set(got))))
                    ok = False
                _assert_safe_deleted(got)
            except Exception as e:
                log('[V10] changes.json 校验失败：%s' % e)
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
    # 资产名必须含 OS 与架构关键字（win / x64）：MXU「设置-更新」的 GitHub 回退通道
    # 用 matchGitHubAsset() 按这两个关键字匹配资产，缺一个就匹配不到、只剩"发现新版本"
    # 却没有下载按钮。打包器出的名字同时满足打包器自身与 mirrorchyan workflow 的 glob。
    ap.add_argument('--out', help='输出 zip 路径，默认 updates/MABd2<version>-win-x64.zip')
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

    out = args.out or os.path.join(BASE, 'data', 'updates', 'MABd2%s-win-x64.zip' % version)

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
    ensure_icon(BASE)
    check_version_json(BASE)
    check_verlog(BASE, version)
    check_bat_crlf(BASE)
    check_retired(BASE, version)

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

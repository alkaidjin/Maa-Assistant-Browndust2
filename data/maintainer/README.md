# data/maintainer/ — 维护者档案目录（不入包；v26.09.14 起随根目录瘦身搬进 data/）

> **本目录整体不进发布包**（打包器 `EXCLUDE_DIRS` 含 `maintainer`，
> 见 `data/maintainer/tools/build_release_zip.py`）。它只入仓库，是「维护者工具 + 退役件存档」的家。

## 为什么必须有个不入包的目录

MXU 的软件内更新（`apply_full_update`）走**全量覆盖**：它遍历解压包的**根级条目**，
把目标目录里同名的文件 / 目录**整个搬到 `cache/old`，搬不动就直接删**，再复制新包进去。

后果：**任何躺在项目根下、且名字与包内条目同名的维护者文件，都会被一次更新抹掉。**
2026-09-30 实测就是这么丢的 —— `tools/`（打包器、备份器、图标工具、rcedit）、
`agent/rock-picker/`（Go 源码）、`agent/start.*`、`resource/model/.gitignore` 共 22 个文件凭空消失，
`cache/old` 是空的（搬不动 → 走了删除分支）。`.git` / `config/` / `cache/` / `debug/` 因为不在包内才活下来。

所以：**维护者文件一律放这里**，别放回项目根或 `tools/` / `agent/` / `resource/` 这些会被整目录替换的地方。

## 目录结构

| 路径 | 内容 |
|---|---|
| `tools/build_release_zip.py` | **发布包构建器**（版本号锚点同步、排除表、V1–V10 校验、`changes.json` 生成） |
| `tools/backup_maintainer.ps1` | 维护版备份（git bundle + 未跟踪文件打包到带时间戳目录） |
| `tools/install_backup_task.ps1` | 把上面那个备份注册成计划任务 |
| `tools/build_release_zip.bat` | **打包双击入口**（自己找 Python，见下） |
| `tools/apply_icon.ps1` | 给 `MaaBd2.exe` 打图标（rcedit 包装；打包器 ensure_icon 每次打包自动复核，本脚本是手动兜底） |
| `tools/make_icon.py` | 由源图生成 `mxu.ico` / `mxu_icon.png` |
| `tools/clean_logs.ps1` | 按天数清理 `debug/` 日志 |
| `tools/compact_log.ps1` | 把 `maa.log` 压成任务时间线 |
| `tools/rcedit-x64.exe` | 打图标用的第三方工具（electron/rcedit，MIT） |
| `Verlog.xlsx` | **版本发布日志表**（维护者文档，v26.09.14 起不再派发给用户；打包器仍会读它提醒「本版有没有记录」） |
| `备份维护版.bat` | 双击备份入口 |
| `agent/start.bat` / `agent/start.py` | agent 开发辅助（**写死了作者本机游戏路径**，仅本机可用） |
| `agent/rock-picker/` | 第二个 agent（圣石洞穴「刷数量最少的圣石」）的**Go 源码**；运行件是 `agent/rock-picker.exe` |
| `legacy/` | **退役件存档** —— 万一要重写时照着改，不要再直接拿去用 |

## legacy/ —— 退役件

| 文件 | 曾经的作用 | 为什么退役 |
|---|---|---|
| `BD2MAA-Updater.ps1` | 老启动器：启动即检测 GitHub 新版本 → 弹窗 → 多源镜像下载 → 覆盖安装 | MXU 自带「设置 - 更新」已能承担更新（2026-09-30 实测跑通）；它还要负责启动软件 |
| `launcher.bat` | 老启动器的双击入口 | 同上 |
| `BD2MAA-Maintainer.ps1` | 维护器：家务 + `-Verify` 完整性校验 + `-Fetch`/`-Repair` 镜像下载重装 | 更新彻底交给 MXU 后，家务（清遗留 / 打图标 / 重建 lnk / 日志整理）不再有人调用 |
| `maintainer.bat` | 维护器的双击入口 | 同上 |
| `updater_config.json` | 启动器 / 维护器的配置（repo、镜像源、资产关键字） | 随二者一起退役，已登记进 `retired_files.json`，老用户升级时自动清掉 |
| `resource-model.gitignore` | `resource/model/.gitignore`（内容只是 `ocr`） | 根 `.gitignore` 第 29 行的 `resource/model/ocr/` 已覆盖；留在 `resource/` 下会被 MXU 更新删掉 |

> 这些文件的**能力**若将来要复活（比如要做离线镜像下载、完整性自检），
> 优先看 `BD2MAA-Maintainer.ps1` 的 `-Verify` / `-Repair` 实现 —— 三道校验（装前包身份 / 装中逐文件重试 /
> 装后复核版本号）都在里面，值得照抄。注意它们假设「项目根 = 脚本所在目录的上一级」，
> 搬回 `maintainer/tools/` 需按本目录其它脚本那样再退一级。

## 常用命令

```bash
# 打包（无参 → 交互式询问版本号；会同步所有版本锚点）
python data/maintainer/tools/build_release_zip.py
python data/maintainer/tools/build_release_zip.py --dry-run        # 只看要做什么
python data/maintainer/tools/build_release_zip.py --no-bump --out data/cache/_pkgtest/x.zip v26.09.14
```
> 🖱️ **双击打包入口 = `tools/build_release_zip.bat`**（v26.09.14 重建，与 .py 同目录）。
> 它自己找解释器（`.venvs\py313` → WorkBuddy 托管解释器 → `%LOCALAPPDATA%` → `where python`
> → `py -3`），跑完 `pause` 等按键才关窗，失败也会停住。
> ⚠️ 别直接双击 `.py`：本机 `*.py` 关联到 `C://Windows//py.exe`，而 py launcher 里没注册
> 任何 Python（系统 Python 3.10 已卸载、venv / uv 不向它注册）→ 只会「闪一下就没了」。

```bash
"C:\Users\ALKAID\.workbuddy\binaries\python\versions\3.13.12\python.exe" data/maintainer/tools/build_release_zip.py
"C:\Users\ALKAID\.venvs\py313\python.exe" data/maintainer/tools/build_release_zip.py
```
交互式确认需要 stdin：`printf 'y\n' | <python> ... --no-bump`。

## 包体裁剪（PRUNED_BINARIES）

打包器里有一份 `PRUNED_BINARIES`：**仓库照样跟踪、但不进用户包的运行时二进制**
（v26.09.14 起，9 个 ≈ 5.1 MiB 未压缩 / 2 MiB 压缩包体）。判据都做了源码级核实：

| 裁剪项 | 依据 |
|---|---|
| `maafw/MaaAdbControlUnit.dll`、`MaaGamepadControlUnit.dll`(+`ViGEmClient.dll`)、`MaaCustomControlUnit.dll`、`MaaRecordControlUnit.dll`、`MaaReplayControlUnit.dll` | 控制器 DLL 是**按需 LoadLibrary**（上游 `LibraryHolder/ControlUnit/ControlUnit.cpp` 的 `load_library` 在 `create_control_unit()` 里）。本仓 `interface.json` 只有 win32 控制器 ⇒ 这五种永远不会被加载 |
| `maafw/MaaNode.node`、`maafw/MaaNodeServer.node` | Node.js addon；MXU 是 Tauri/Rust，包里也没 node 运行时 |
| `maafw/MaaPiCli.exe` | 命令行调试器，MXU 不调用 |

⚠️ **不要顺手裁 `maafw/DirectML.dll`（17.7 MiB，看着最肥）**：`MaaInferenceDevice` 默认 **Auto**，
而 `Ort::GetAvailableProviders()` 是否含 `DmlExecutionProvider` 是**编译期**定的（本仓 onnxruntime 确实编译进去了），
有合适 GPU 的机器上 `use_directml()` 会真的去加载它 —— 缺文件会在资源加载阶段炸掉整个任务。

注意：它们**没有**被写进 `changes.json` 的 `deleted`（只排除新包、不删用户磁盘上的旧副本），
避免万一判断有误时把用户的文件也弄没。旧副本不占下载体积，只是留在磁盘上。

### 不派发的维护者文档 / 已下架资产

| 项 | 处置 | 备注 |
|---|---|---|
| `Verlog.xlsx` | v26.09.14 移到本目录 | 维护者文档。打包器 `check_verlog()` 改读 `data/maintainer/Verlog.xlsx`（找不到时回退根目录），仍会提醒补记录 |
| 教学视频 `#…教学视频# .mp4` | v26.09.14 下架 + `git rm --cached` 停止跟踪（14.2 MB） | 旧名在 `retired_files.json` 里，老用户升级由 `changes.json` 清掉；仓库历史（`f3e5cd2`）仍可取回 |

### 注意事项 docx 的无损压缩（2026-09-30）

`3.959 MiB → 2.537 MiB（-35.9%）`，**画质零变化**：只对 `word/media/*.png` 做
`PIL` 无损重编码（`optimize=True, compress_level=9`），再整体 deflate-9 重打包。
分辨率、色深、**每一个像素**都与原件一致（脚本逐张做了 `numpy` 逐像素断言，
不一致就放弃该张保留原图）。XML 部分逐字节未改。

方案取舍时实测过有损路线，数据备查（image3 是那张 1920×1080 大图，占 3.298 MiB）：

| 方案 | image3 | 画质 |
|---|---|---|
| **PNG 无损重编码（采用）** | 2.007 MiB | 逐像素完全一致 |
| JPEG q=98，4:4:4 | 0.896 MiB | PSNR 48.75 dB，最大误差 9/255 |
| JPEG q=95，4:4:4 | 0.602 MiB | PSNR 46.02 dB，最大误差 19/255 |

有损路线还能再省约 1 MiB 包体，但「不能让图片变糊」是硬要求，故不采用。
将来若改主意，脚本在 `cache/_shrink_docx.py`（备份原件在 `cache/_docx_backup/`）。

## 铁律

1. **本目录里的东西绝不能进用户包** —— 里面有本机路径（游戏安装目录、Python 解释器路径）。
2. 新建维护者脚本**就放这里**，不要放项目根；回项目根 = 下次 MXU 自更新后被删。
3. 打包器新增本地目录 / 脚本时，同步检查 `EXCLUDE_DIRS` / `EXCLUDE_FILES`
   （打包器是 `os.walk` 扫本地工作区，**未跟踪文件照样进包**）。
4. 每次 MXU 自更新后先 `git status` 扫一遍，确认没有文件被静默抹掉。

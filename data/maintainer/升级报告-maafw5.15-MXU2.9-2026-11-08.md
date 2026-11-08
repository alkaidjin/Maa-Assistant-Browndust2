# 底层依赖升级报告：MaaFramework v5.13.0 → v5.15.0、MXU v2.5.3 → v2.9.0

| 项 | 内容 |
|---|---|
| 报告日期 | 2026-11-08 |
| 项目基线 | `v26.11.1`（`main` @ `1c19f16`） |
| 升级分支 | `upgrade/maafw-5.15_mxu-2.9`（`main` 未改动） |
| 提交链 | `dcd375f`（升级前快照）→ `62a9c5a`（换底层）→ `e8573df`（同步指纹 + 堵出包泄漏） |
| 状态 | 代码/文档/打包已就绪并验证；**游戏内 8 项实测待排期**（见 §9） |
| 净代价 | 发布包 **66.30 → 69.36 MiB（+3.09 MiB / +4.7%）**，文件数 270 → 272 |

---

## 1. 摘要

本次升级把两件长期滞后的第三方依赖同时推进到当前正式版：

- **MaaFramework v5.13.0 → v5.15.0**：跨 5 个正式版（5.14.0/1/2/3、5.15.0），修掉 3 条**直击本项目代码路径**的问题
  （`RunTask`+`WaitFreezes` 停止后卡死、批量 OCR 跨节点缩放污染、agent 响应错位崩溃）。
- **MXU v2.5.3 → v2.9.0**：跨 7 个正式版，恢复 `focus` 用户提示在 Windows 的显示，并修掉多实例日志串台、
  配置被旧快照覆盖、"任务显示完成但其实没完"三类稳定性问题。

两条工程结论：

1. **可以只换二进制，不改任何业务代码**。已用仓库自带的 `rock-picker` 集成测试在 5.15.0 上跑通真实 OCR + pipeline
   （5/5 PASS），证明 `resource/`、PP-OCRv5 模型、Go agent 绑定（`maa-framework-go v4.0.0-beta.18`）三者的兼容性。
2. **顺带发现并修掉了一个正在发生的发布事故**：打包器把维护者本机 `logs/`（含游戏画面截图、实例配置快照）
   与 `agent/bd2cli.exe` 一起打进了发布包，多出 **44.3 MiB**。若不清掉，这次"只该 +9 MiB"的升级会变成 +35 MiB，
   并且会把本机截图发给全体用户。

---

## 2. 版本对照

| 依赖 | 升级前 | 升级后 | 上游发布时间 | 落后版本数 |
|---|---|---|---|---|
| MaaFramework | v5.13.0 | **v5.15.0** | 2026-11-02 | 5 个正式版 |
| MXU | v2.5.3 | **v2.9.0** | 2026-11-05 | 7 个正式版 |
| PaddleOCR 模型 | PP-OCRv5 mobile | 不变 | — | 上游已有 PP-OCRv6（**本次不动**，见 §8） |
| maa-framework-go（agent 绑定） | v4.0.0-beta.18 | 不变 | 上游已发 v4.0.0（**本次不动**，见 §8） |
| rcedit | v2.0.0 | 不变 | 上游无更新 | — |

候选但**未采用**：`MaaFramework v5.16.0-beta.2`（11-05 预发布）、`MXU v2.10.0-beta.2`（11-07 预发布）。

---

## 3. 为什么要升：上游变更 × 本项目用法

判据不是"看更新日志"，而是先统计本项目实际用了什么：

- `resource/pipeline/**/*.json`：971 个识别/动作节点，其中 **OCR 占绝大多数**，其次 `TemplateMatch`，少量 `ColorMatch`、
  Go 侧 `Custom`；**没有** `NeuralNetwork`、**没有** `wait_freezes` 节点、**没有** `DirectHit`；节点普遍带 `roi/max_hit/on_error/focus`。
- `data/maintainer/agent/**`：实际调用 `ctx.RunTask`（trader）、`ctx.WaitFreezes`（trader/nav.go）、
  `ctx.OverrideNext`（fishing、rock-picker），四个 agent 子进程 `timeout: 15000` + `auto_reconnect`。

| 上游变更 | 对本项目的作用 | 维度 |
|---|---|---|
| **5.15.0** `post_stop` 打断 `wait_freezes`/`sleep`，避免 `MaaContextRunTask` 永不返回 | 本项目正好同时用 `RunTask`+`WaitFreezes` → 修掉"点停止后 agent 卡住" | **稳定性** |
| **5.15.0** 批量 OCR 按 `det max_side_len` 分组，保住每节点的 det 缩放 | OCR 节点最多、ROI 尺寸各异 → 修掉跨节点缩放污染导致的阈值漂移 | **效果** |
| **5.14.3** Agent 响应按请求编号匹配 | 4 个 agent + 15 s 超时 + 自动重连，正是响应错位的高发场景 | **稳定性** |
| **5.15.0** DirectML 初始化失败后回退 CPU（不再闪退） | `DirectML.dll` 随包派发，有 GPU 的机器会真的加载它 | **稳定性**（条件） |
| 5.14.1 Zmq 消息阻塞修复；schema 补 `pretask/global_option/hotkey` | 通用稳定性 + 为将来引入标准 pretask 铺路 | 小幅 |
| 5.15.0 `Input` 选项新增 `input_type`（文件/文件夹选择器）、Custom 识别多结果 + `index` | 可用于"启动器路径"这类输入项；向后兼容 | 功能（可选） |
| **MXU 2.8.0** 修 `focus` 的 toast/notification 在 Windows 静默失效；**2.9.0** 补全 `focus` 展示渠道 | 本项目大量用 `focus` 显示"正在钓哪片水域 / 为什么跳过" → **用户可直接感知** | **效果** |
| **MXU 2.9.0** 日志按实例记录（修串台）、防 Web 陈旧快照覆盖配置、任务完成状态改为真正空闲后发布 | 多实例 + 多 agent 的稳定性 | **稳定性** |
| MXU 2.9.0 外部更新后处理过时待更新包；支持每日定时检查更新 | 与"软件内更新"主通道契合 | 稳定性/功能 |
| MXU 2.7.1 / 2.8.0 修"游戏关闭后进程不退出"、定时任务热键键码、Pretask 隐藏控制台窗口 | 常见投诉类型 | 稳定性 |
| 不适用：`DirectHit` 跳截图、神经检测约束到 ROI、`auto_up`、多资源包同名模板优先级、`wait_freezes` ROI 重解 | 本项目未使用这些特性 | — |

**结论**：功能收益小，**效果（OCR 一致性 + focus 可见性）与稳定性（agent 不卡/不崩、多实例不串）两条都命中**。

---

## 4. 具体改了什么

### 4.1 `maafw/` 目录（运行件 20 → 19 个文件）

新增 3 个、上游移除 1 个、内容变更 16 个：

| 变化 | 文件 | 旧 → 新 (MiB) | 处置 |
|---|---|---|---|
| 新增 | `webgpu_dawn_maa.dll` | — → 6.55 | **随包**（被 `onnxruntime_maa.dll` 静态导入，缺了 `MaaFramework.dll` 直接加载失败） |
| 新增 | `dxcompiler.dll` | — → 17.15 | 从磁盘删除（只服务 WebGPU 着色器编译，实测可剪） |
| 新增 | `dxil.dll` | — → 1.43 | 同上 |
| 移除（上游不再发布） | `MaaPiCli.exe` | 0.93 → — | 从仓库删除；`PRUNED_BINARIES` 里保留护栏条目 |
| 变更 | `onnxruntime_maa.dll` | 18.12 → 23.93 | 随包（ORT 升级 + 编入 WebGPU） |
| 变更 | `opencv_world4_maa.dll` | 12.99 → 8.85 | 随包（模块裁剪） |
| 变更 | `MaaFramework.dll` | 2.44 → 2.51 | 随包 |
| 变更 | `MaaAgentClient.dll` / `MaaAgentServer.dll` | 1.17→1.19 / 1.32→1.25 | 随包（**必须同批替换**，协议两端要一致） |
| 变更 | `MaaWin32ControlUnit.dll` / `MaaUtils.dll` / `MaaToolkit.dll` / `fastdeploy_ppocr_maa.dll` | 小幅 | 随包 |
| 变更 | `MaaAdb/Gamepad/Custom/Record/ReplayControlUnit.dll`、`MaaNode(.Server).node`、`ViGEmClient.dll` | 内容变（大小基本相同） | **不派发**（原有剪除策略） |
| 不变 | `DirectML.dll`（17.67） | 逐字节相同 | 随包（有 GPU 的机器会真加载） |
| 不变 | `MaaAgentBinary/**`（6 个文件） | 逐字节相同 | 随包 |

**随包运行件合计：56.7 → 65.0 MiB（+8.3 MiB）**；`LICENSE.md` 同步为 5.15.0 上游文本（LGPL 履约）。

完整指纹表见 §11。

### 4.2 `MaaBd2.exe`

| 项 | 旧 | 新 |
|---|---|---|
| 来源 | MXU 2.5.3 的 `mxu.exe` 改名 | MXU 2.9.0 的 `mxu.exe` 改名 |
| 大小 | 31,342,080 B | **32,542,208 B**（官方 32,365,056 + 图标资源 177,152） |
| `FileVersion` | 2.5.3 | **2.9.0** |
| 内嵌图标 | 本项目图标 | 本项目图标（用 `ensure_icon()` 复核：与 `mxu_icon.png` **字节一致**，120,875 B） |
| 快捷方式 | `MaaBd2.lnk` | 已重建 |

> ⚠️ `MaaBd2.exe` 的 sha256 每次用 rcedit 重写图标都会变（PE 时间戳），**不能当版本指纹**。核对请用
> 「大小 32,542,208 B + FileVersion 2.9.0 + 内嵌图标字节一致」三件套。

### 4.3 `data/maintainer/tools/build_release_zip.py`

| 位置 | 改动 | 理由 |
|---|---|---|
| `PRUNED_BINARIES` | + `maafw/dxcompiler.dll`、`maafw/dxil.dll`；`MaaPiCli.exe` 条目保留并注明上游已不发 | 省 18.6 MiB；护栏防"重装 zip 后又被扫进包" |
| `REQUIRED_FILES` | + `maafw/webgpu_dawn_maa.dll` | 缺它 100% 启动失败，必须硬校验 |
| `EXCLUDE_DIRS` | + `logs`、`assets`、`docs` | **修出包泄漏**（实测混入 107 文件 / 34.35 MiB） |
| `EXCLUDE_FILES` | + `agent/bd2cli.exe` | AI CLI 外壳，不在 `interface.json` 的 agent 列表 → 省 9.96 MiB |
| 注释 | 更新体积合计；把三条实测依据（MXU 无 WebGPU 开关 / 默认 Auto→DirectML / 删掉后 OCR 测试仍 PASS；`webgpu_dawn` 是静态导入不能剪）写进代码 | 防止后人"顺手优化"掉关键文件 |

### 4.4 文档与指纹

| 文件 | 改动 |
|---|---|
| `data/version.json` | `maafw: v5.15.0`、`mxu: v2.9.0`（打包器 dry-run 已确认读到） |
| `NOTICE.md` | 表格版本号；§1「版本固定」；§2 标题与正文改 **`MaaXYZ/MXU`** 并加迁移说明；内核版本 2.9.0；附录下载链接；补"`webgpu_dawn` 必须随包 / `dxcompiler`、`dxil` 不派发"；末行更新日期 |
| `data/更新功能说明.md` §6 | 版本号；**11 行 sha256 指纹表**；三条 ⚠️（别用文件大小、`MaaBd2.exe` 不能用 sha、`webgpu_dawn` 不能删）；新增本次升级 8 条收益 + 升级/回滚步骤；旧历史块降级保留 |

---

## 5. 三个关键取舍

**① 为什么 maafw 与 MXU 必须成对升级**
`MaaAgentClient.dll`（客户端）与 `MaaAgentServer.dll`（agent 侧）来自同一个 zip，成对替换时两端天然一致；
只换一半才会出现协议/ABI 错位——这正是 2026-09-19 诊断过的 `status 0` 失败形态。
另外 MXU 只校验**最低**版本（2.5.3 与 2.9.0 都是 `MIN_MAAFW_VERSION = "5.12.1"`），不会挡住升级，所以更要把两端锁在一起。

**② 为什么敢剪 `dxcompiler.dll` + `dxil.dll`（18.6 MiB）**
三条证据：① `onnxruntime_maa.dll` 的静态导入表里**只有** `webgpu_dawn_maa.dll`，没有这两个；② 删掉后
真实 OCR + pipeline 集成测试仍 PASS；③ MXU 2.9.0 的 `mxu.exe` 里 `WebGPU`/`webgpu`/`DirectML` 字符串**0 命中**
（GUI 根本没有选择 WebGPU 的入口），而 `MaaInferenceDevice` 默认 Auto → 有 GPU 走 DirectML、否则 CPU。
`webgpu_dawn_maa.dll` 则相反：删掉后 `MaaFramework.dll` 直接 `The specified module could not be found`。

**③ 为什么升到 2.9.0 而不是 2.10.0-beta.2**
2.10.0-beta.1 引入了 **「移除 PI 协议外扩展 (#405)」**，而本项目 `interface.json` 的 `agent[].timeout`、
`agent[].auto_reconnect` 正属"协议外扩展"（PI v2 文档没有、5.15 schema 也拒收）。
2.9.0 是当前稳定的最后一版；跳 2.10 前必须先确认 agent 自动重连是否仍生效。

---

## 6. 验证与证据

### 6.1 兼容性（不需要游戏）

| # | 验证项 | 方法 | 结果 |
|---|---|---|---|
| 1 | 安装包完整性 | zip sha256 | MAA 5.15 = `245ee8b5fed7f659…`，**与 GitHub release digest 一致** |
| 2 | MXU 2.9 需要的 API 是否都在新 maafw 里 | 解析 6 张导出表（MaaFramework 172 / AgentClient 17 / AgentServer 182 / Toolkit 41 / Utils 90 / Win32ControlUnit 3）与 `mxu.exe` 中 246 个 `Maa*` 符号比对 | **0 缺失** |
| 3 | 5.15 在真实资源 + 真实模型上能否工作 | `go test -run TestRock`（合成 1280×720 截图 → 5 个 ROI OCR → argmin → `OverrideNext` 真路由） | **PASS**，路由 4.09 s（旧 4.99 s） |
| 4 | Go 绑定 beta.18 能否加载 5.15 | 同上测试（`maa.Init(WithLibDir(maafw))` 走 purego） | **能**，agent 源码零改动 |
| 5 | 剪掉 `dxcompiler/dxil` 是否安全 | 删掉重跑同一测试 | **PASS**（4.37 s） |
| 6 | 剪掉 `webgpu_dawn` 是否安全 | 删掉重跑 | **失败**（模块加载失败）→ 必须随包 |
| 7 | MXU 2.9 是否要求更高 maafw | 读 `src-tauri/src/commands/maa_core.rs` | `MIN_MAAFW_VERSION = "5.12.1"`，与 2.5.3 相同 |
| 8 | 改名 + rcedit 打图标 | 对 2.9.0 实跑 `apply_icon.ps1` | 成功；`ensure_icon()` 复核图标字节一致 |
| 9 | exe 能否正常启动 | 同字节拷到非 `RUNASADMIN` 路径跑 `--help` | `rc=0`，输出 657 B 中文帮助 |
| 10 | `MaaAgentBinary` / `DirectML.dll` 是否需变 | sha256 比对 | 与 5.13 逐字节相同，不用动 |

### 6.2 配置 schema 审计（用 5.15 随包的 `tools/*.schema.json`）

| 对象 | 结果 |
|---|---|
| `interface.json` | 2 处不符合（`win32.screencap_target_short_side`、`agent[].auto_reconnect/timeout`） |
| `tasks/*.json`（9 个） | 每文件 1–5 处，同一类（option 用 `des` 而非 `description`） |
| `resource/pipeline/**/*.json`（36 个） | 可完整校验的 28 个里 2 处（`PVP.json::Worship` 节点级 `des`、`Bag.json::ClickAnniu` 识别 param 内 `retry`） |

**判定：全部是既有字段漂移，不是本次升级引入，且运行时被宽容忽略。**
证据：阶段 4 的 `res.PostBundle(resource)` 会把 36 个 pipeline 文件全量加载，测试通过。
其中 `screencap_target_short_side` 特别值得记一笔：PI v2 文档里 Win32 只允许
`class_regex/window_regex/mouse/keyboard/screencap`，而 2.5.3 与 2.9.0 的 `mxu.exe`、两个 maafw DLL 里
**都搜不到这个字符串** → 它从来没生效过（真实缩放走 MaaFramework 的控制器选项，PI 未暴露；本项目靠"游戏设为 16:9"约束）。

### 6.3 全量出包（端到端）

修复泄漏后的构建结果：

```
[1] 待打包文件 = 272
[2] 原始 152,591,989 B -> zip 72,732,198 B (69.36 MiB)  耗时 5.4 s
[V1] testzip 完好        [V2] 目标包 273 条目        [V3] 两处版本锚点 = v26.11.1 OK
[V4] 缺失必需项 = 无      [V5] 排除项泄漏 = 无        [V9] 退役清单 41 项不冲突
[V10] changes.json deleted = 41                        [V6] BOM 问题 = 无
[V7] sha256 = 1c2e071e703bece5ffb7cf3d1f4b118f8c7e81a9ba27e6276ba3ca8ef9acd9ff
[V8] size   = 72,732,198 bytes
=== 全部通过 ===
```

包内复核：`logs/`=0、`assets/`=0、`docs/`=0、`agent/bd2cli.exe` 不在、`webgpu_dawn_maa.dll` 在、
`dxcompiler/dxil` 不在、`maafw` 运行件 12 个（含 `plugins/MaaPluginDemo.dll`）。

### 6.4 包体对比

| 版本 | 文件数 | zip | 说明 |
|---|---|---|---|
| 上一版 `MABd2v26.11.1-win-x64.zip` | 270 | 66.27 MiB | 升级前 |
| 本次（含泄漏修复） | 272 | **69.36 MiB** | **+3.09 MiB / +4.7%** |
| 本次（若不清泄漏） | 384 | 101.22 MiB | +34.9 MiB —— 已避免 |

---

## 7. 本次顺带修掉的缺陷

| # | 缺陷 | 影响 | 处置 |
|---|---|---|---|
| 1 | **出包泄漏**：`logs/`（107 文件 / 34.35 MiB，含本机游戏画面截图、`config-before-*.json`）+ `agent/bd2cli.exe`（9.96 MiB）+ `assets/docs` 被扫进发布包 | 若发出，等于把维护者截图与配置发给全体用户，且包体 +44 MiB | `EXCLUDE_DIRS`/`EXCLUDE_FILES` 已补，重新出包验证通过 |
| 2 | `apply_icon.ps1` 在"刚写完 32 MB 新 exe"时假失败（rcedit exit 1，报错被 `SilentlyContinue` 吞掉） | 手动流程会被误导；`ensure_icon()` 同样单次调用，失败只打 `[W]` 并产出无图标包 | **已记录建议未改**（加 2–3 次重试 + 让 rcedit 输出可见）。当前图标状态已用字节级复核确认正确 |
| 3 | 上一版审计报告里"刻意不派发 `DirectML.dll`"的描述是错的 | 会误导后人剪掉必需 DLL | 已在报告与本报告中更正：`build_release_zip.py` 注释明确要求**保留** |
| 4 | `NOTICE.md` 仍指向 `MistEO/MXU`（上游已迁到 `MaaXYZ/MXU`） | 链接 404 / 误导贡献者 | 已全局改为新地址并加迁移说明 |
| 5 | `.gitignore` 里 `!maafw/MaaPiCli.exe` 白名单已成死条目 | 无害但误导 | **未改**（下次清理可删） |

---

## 8. 明确**没有**做的事（及理由）

| 项 | 理由 | 建议窗口 |
|---|---|---|
| `maa-framework-go` beta.18 → **v4.0.0** | 破坏性：构造函数/`Post*`/`CustomController`/`NextItem`/`WaitFreezes`/`TaskDetail` 全改签名，需改 4 个 agent 源码 | 下一次独立发版，可复用本次门禁流程 |
| PP-OCRv6（PaddleOCR v3.7.0） | 与 maafw 升级同批会失去归因能力；且需先验证 MaaFramework 的 OCR 三件套契约（预处理/CTC 解码） | 之后单独 A/B（用 `debug/` 历史截图统计 `expected` 命中率） |
| MXU 2.10.0-beta.2 | 预发布 + 移除 PI 扩展（见 §5③） | 等正式版，且先确认 agent 重连 |
| 清理 §6.2 的 5 类 schema 漂移 | 不影响运行；改 `des`→`description` 前应先确认 UI 上描述是否显示 | 单独一次清理提交 |

---

## 9. 未完成：游戏内 8 项实测（待排期）

| # | 任务 | 关注点 | 通过标准 |
|---|---|---|---|
| 1 | 启动链路（LaunchGame） | 2.9 改了单实例/前置程序逻辑 | 冷启动进游戏，无「未搜索到任何窗口」 |
| 2 | 圣石洞穴（最少石头） | 5 行 OCR + argmin + `OverrideNext` | 数量读对；读不到时按设计失败 |
| 3 | 渔场 AutoFishing | `focus` 提示（2.8/2.9 修复点） | 界面上能看到 `focus` 文案 |
| 4 | 跑商 MapTrade | OCR 价格/次数（5.15 改了 OCR 分批） | 高价卖出判定与上一版一致 |
| 5 | 赛季奖励 / 日常收菜 | 红点、次数 OCR | 无重复领取、无卡死 |
| 6 | 塔 / PVP | 模板 + OCR 混合 | 通过 |
| 7 | **停止 → 再开始** | 5.15 `post_stop` 修复点 | 停止后 agent 立刻回收；再开始不报 `Tasker not properly initialized` |
| 8 | 多实例（开 2 个） | 2.9 日志隔离 / 配置快照 | 日志不串台、设置不互相覆盖 |

跑之前先看 `debug/maafw.log` 是否有 `Protocol version mismatch` 或 `Function ... not loaded` —— 升级后最容易出、也最好定位的两条。

---

## 10. 回滚

```powershell
$b   = 'F:\MABd2v26.09.5'
$bak = "$b\cache\backup_v26.11.1_pre_upgrade"
robocopy "$bak\maafw" "$b\maafw" /MIR /NFL /NDL /NJH /NJS
Copy-Item "$bak\MaaBd2.exe" "$b\MaaBd2.exe" -Force      # 旧 exe 自带图标，无需重打
# 校验：应为 5.13.0 的指纹
(Get-FileHash "$b\maafw\MaaFramework.dll" -Algorithm SHA256).Hash.Substring(0,16)   # d4503d525561a7be
```

`config/` 不受影响（升级不碰用户实例配置）。备份内容：32 文件 / 91.9 MiB（`maafw/` 全量 + `MaaBd2.exe`），
已验证其中 `MaaFramework.dll` 前缀 = `d4503d525561a7be`（5.13.0）、exe = 2.5.3。

---

## 11. 附录 A：`maafw/` 指纹表（升级后）

| 文件 | 大小 (MiB) | sha256 前 16 位 | 是否派发 |
|---|---|---|---|
| `MaaFramework.dll` | 2.51 | `1c5e4112ca1dca12` | 随包 |
| `MaaAgentClient.dll` | 1.19 | `3dca30e236b104d2` | 随包 |
| `MaaAgentServer.dll` | 1.25 | `2eb62827e9a51c47` | 随包 |
| `MaaToolkit.dll` | 0.53 | `cf348cac100f6e3c` | 随包 |
| `MaaUtils.dll` | 0.60 | `3ccb53be5e2c4ceb` | 随包 |
| `MaaWin32ControlUnit.dll` | 0.72 | `7a883c3f4c719060` | 随包 |
| `fastdeploy_ppocr_maa.dll` | 1.17 | `671feaeeaa4d8f34` | 随包 |
| `onnxruntime_maa.dll` | 23.93 | `29c654b2b6ba9eed` | 随包 |
| `opencv_world4_maa.dll` | 8.85 | `34524a1b28b343e1` | 随包 |
| `webgpu_dawn_maa.dll` | 6.55 | `de591ae76b7db6da` | **随包（不可删）** |
| `DirectML.dll` | 17.67 | `9c9e6d822561c6c4` | 随包 |
| `LICENSE.md` | 0.01 | `446e755fae55ff03` | 随包 |
| `MaaNode.node` | 1.01 | `13c410bbcf8e9b7b` | 不派发 |
| `MaaNodeServer.node` | 0.96 | `b8f9a34828b4f099` | 不派发 |
| `MaaAdbControlUnit.dll` | 0.75 | `9df6198dc75f739c` | 不派发 |
| `MaaGamepadControlUnit.dll` | 0.33 | `d73323c8da79a158` | 不派发 |
| `MaaCustomControlUnit.dll` | 0.30 | `1491052403447314` | 不派发 |
| `MaaRecordControlUnit.dll` | 0.33 | `a72ee39f538e9f75` | 不派发 |
| `MaaReplayControlUnit.dll` | 0.38 | `6c54fc9b25a7612b` | 不派发 |
| `ViGEmClient.dll` | 0.12 | `2bf0cb1d80903957` | 不派发 |
| `MaaAgentBinary/**`（6 个） | 0.17 | 与 5.13 一致 | 随包 |

回滚基准（5.13.0）：`MaaFramework.dll` = `d4503d525561a7be`。
`MaaBd2.exe`：32,542,208 B / `FileVersion` 2.9.0（sha256 `73c8acfd15e84390…`，**会随 rcedit 变动，勿作指纹**）。

## 12. 附录 B：产物与物料

| 路径 | 内容 |
|---|---|
| `cache/backup_v26.11.1_pre_upgrade/` | 回滚备份（32 文件 / 91.9 MiB） |
| `cache/_upgrade_probe/maa515/` | MaaFramework 5.15.0 解包（bin + share） |
| `cache/_upgrade_probe/mxu290/` | MXU 2.9.0 解包（`mxu.exe`） |
| `cache/_upgrade_probe/repo515/` | 隔离沙箱（5.15 + 真实 `resource/` junction + rock-picker 源码） |
| `cache/_upgrade_probe/repo513/` | 对照组（5.13） |
| `cache/_upgrade_probe/MABd2v26.11.1-upgrade-test2.zip` | 本次验证构建的包（69.36 MiB，可直接删） |
| `cache/审核报告-第三方依赖版本-2026-11-08.md` | 依赖版本审核（上游变更逐条比对） |
| `cache/升级方案-maafw515-mxu290-2026-11-08.md` | 升级方案（阶段划分、门禁、风险表） |
| `cache/执行记录-升级0-4-2026-11-08.md` / `-5-6-` | 分阶段执行记录与验收数据 |

## 13. 附录 C：环境备注（与升级无关但会误导排障）

1. **本机给 `F:\MABd2v26.09.5\MaaBd2.exe` 设了「以管理员身份运行」**
   （`HKCU\...\AppCompatFlags\Layers` = `~ RUNASADMIN`）。因此从命令行 `CreateProcess` 直接拉起会返回
   `WinError 740`，**这不是程序问题**：同字节拷到别的路径就能正常跑 `--help`。真实启动请双击（会弹 UAC）。
2. **`apply_icon.ps1` 偶发假失败**（见 §7-2）；图标是否真的写进 PE，用 `build_release_zip.ensure_icon()` 复核最可靠。
3. 出包前请确认 `logs/`、`debug/`、`config/` 里没有你想保留的东西——`logs/` 现在已被排除，但更稳的做法是发版前跑一次 `clean_logs.ps1`。

# Notice / 第三方致谢

本软件**链接、调用或重新发布**了以下第三方开源项目的代码、模型与运行时。完整许可证文本以仓库根目录的 [LICENSE](LICENSE) 文件为本项目自身条款，以下为各上游条款摘要。

| # | 项目 | 许可证 | 在本项目中的角色 |
|---|---|---|---|
| 1 | [MaaXYZ/MaaFramework](https://github.com/MaaXYZ/MaaFramework)（v5.13.0）| **LGPL-3.0** | 运行时核心（C++ 库 + Node 绑定） |
| 2 | [MistEO/MXU](https://github.com/MistEO/MXU)（v2.5.3） | **AGPL-3.0** | GUI 前端 + Go Agent 子进程 + 实例/设置/自动更新 |
| 3 | [MaaEnd/MaaEnd](https://github.com/MaaEnd/MaaEnd) | **AGPL-3.0** | 最初提供**工程骨架**（目录约定、`interface.json` 结构、多语言键）；该仓库是《明日方舟：终末地》工具，**不含《棕色尘埃2》任何内容** |
| 4 | [essinn-1/maa-assistant](https://github.com/essinn-1/maa-assistant) | **AGPL-3.0** | **本项目的直接上游**：《棕色尘埃2》PC 端早期适配、最初一批 pipeline 编排与图像素材 |
| 5 | [PaddlePaddle/PaddleOCR](https://github.com/PaddlePaddle/PaddleOCR)（PP-OCRv5 移动端）| **Apache-2.0** | OCR 模型（`resource/model/ocr/det.onnx`、`rec.onnx`、`keys.txt`） |
| 6 | [electron/rcedit](https://github.com/electron/rcedit)（v2.0.0）| **MIT** | 启动器给 `mxu.exe` 写图标的附加工具（`tools/rcedit-x64.exe`） |

---

## 1. MaaXYZ/MaaFramework — LGPL-3.0

### 用途
提供图像识别、控制器抽象、任务编排、自定义识别/动作注册、节点间通信等核心能力。本项目以**动态链接**的形式使用其预编译二进制作为运行时：

- 静态路由：所有调用均通过 Windows 动态链接库（`.dll` / `.node`）解析，未静态连接，亦未对 MaaFramework 本身的源码进行修改
- 调用入口：见 `resource/pipeline/*.json` 与 `tasks/*.json` 中节点级调用

### 本项目中的位置
二进制：`maafw/*.dll`、`maafw/MaaNode.node`、`maafw/MaaNodeServer.node`、`maafw/MaaAgent*`
完整 LGPL-3.0 条款文本随 MaaFramework 官方 release zip 以 `LICENSE.md` 派发；本项目仓库根目录的本文件（`NOTICE.md`）承担**汇总与履约说明**，逐字条款文本可从 [上游 LICENSE.md](https://github.com/MaaXYZ/MaaFramework/blob/main/LICENSE.md) 或官方 release zip 根目录取得。

### LGPL 履约
根据 LGPL-3.0 §6「Anti-Distortion」及 §4「Combined Works」要求：

1. **本项目自身（`BD2MAA`）**采用 **AGPL-3.0**；LGPL 允许与 AGPL 组合发行（AGPL-3.0 §13 / LGPL-3.0 §3「Compatibility with other licenses」）
2. **MaaFramework 库的源码**可在其官方仓库获取（链接见上表）；LGPL-3.0 完整条款文本同样由上游维护，见 [MaaFramework LICENSE.md](https://github.com/MaaXYZ/MaaFramework/blob/main/LICENSE.md)
3. **重新链接能力**：如果你想替换或更新 MaaFramework 版本，只需：
   - 关闭 `mxu.exe`
   - 用对应平台、对应主版本号的 `MAA-<platform>-v<version>.zip` 解压并覆盖 `maafw/` 目录（**仅替换 `*.dll/*.node/*.exe`**；保留 `MaaAgentBinary/` 与 `plugins/` 子目录中你不想覆盖的文件）
   - 重新启动 `launcher.bat`
4. **工程加密 / 闭源修改**：本项目对 MaaFramework 库的源码**未做任何修改**，因此 LGPL §6 的「提供源码供用户重新链接」义务不存在

### 版本固定
本仓库当前锁定的 MaaFramework 版本为 `v5.13.0`。升级到新版本时需要同步维护两处：

1. 本文件上方表格中的版本号（版本指纹核对方法见 [`更新功能说明.md`](更新功能说明.md) 第十一节）；
2. **`version.json`** —— 它记录包内依赖指纹（`maafw` / `mxu` 各自的上游 tag），是打包器 `tools/build_release_zip.py` 在发布计划阶段校验的对象（`check_version_json()` 会提示版本是否与磁盘实际一致，但**不会自动改写**，需要人工确认）。

历史版本备份放在 `cache/backup_<版本>/`。注意 `cache/` 目录**不随发布包派发**（升级与回滚步骤见 [`更新功能说明.md`](更新功能说明.md) 第十一节）。

---

## 2. MistEO/MXU — AGPL-3.0

### 用途
提供桌面 GUI（Tauri / 前端 TypeScript）、设备连接管理、实例配置读写、GitHub Release 检测与下载、`maafw` 子进程（`go-service.exe`）的拉起。

### 在本项目中的位置
单二进制：根目录 `mxu.exe`（v2.5.3）

### AGPL 履约
MXU 与本项目**均为 AGPL-3.0**，因此在 §13 「Remote Network Interaction」的范围内：

1. 本项目以及任何衍生项目若对外提供服务（提供 MXU 前端访问能力），必须同时**完整公开所运行的、与 MXU 相关的全部源代码**，包括任何自定修改
2. MXU 自身的修改若已合并到上游，则「Relicensing」条款不单独适用

### 启动器交互
本项目通过 `launcher.bat` → `BD2MAA-Updater.ps1` **自动拉起** `mxu.exe`：先检查 GitHub Releases 是否有新版本（可一键下载覆盖），再做启动前的「家务」（自愈 `launcher.bat` 编码、按需重写 `mxu.exe` 图标、重建 `MaaBd2.lnk`、精简并清理 `debug/` 日志），最后启动 `mxu.exe`。

**启动器不会修改你的 `config/`**（唯一的例外是更新覆盖完成后给 `interface.json` 补一个纯说明性的 `x_launch` 字段，MXU 会忽略未知字段）。启动器的用法、命令行参数与全部「家务」细节见 [`更新功能说明.md`](更新功能说明.md)。

---

## 3. MaaEnd/MaaEnd — AGPL-3.0

### 用途
MaaEnd 是《明日方舟：终末地》的自动化工程。**它本身不包含《棕色尘埃2》的任何任务定义、pipeline 或图像素材**；
本项目的棕2 内容全部来自下一节 `essinn-1/maa-assistant`。

本项目从 MaaEnd 继承的是**工程骨架**：目录约定（`resource/pipeline/`、`tasks/`）、`interface.json` 的
ProjectInterface v2 结构、多语言键的组织方式，以及 `misc/MaaEnd-Tiny.png` 等少量占位资源。

### 继承路径

```
MaaEnd（工程骨架）
   └─ essinn-1/maa-assistant（《棕色尘埃2》PC 端适配，AGPL-3.0）
         └─ 本项目 Maa-Assistant-Browndust2（AGPL-3.0）
```

严格说，本项目**不是** GitHub 意义上的 fork 关系，而是直接承接 `essinn-1/maa-assistant` 的 Git 历史继续开发，
因此仓库页不显示 `forked from` 标记；真实的来源关系以本节与下一节的文字说明为准。

### AGPL 履约
- 本项目的 AGPL-3.0 许可证与 MaaEnd 同源，**合规继承无缺口**：继承文件与本项目新增文件统一适用 AGPL-3.0
- MaaEnd 上游仍在活跃更新（与本项目无关）；本项目**不跟踪**其后续变更，骨架之外的改动均为自研

---

## 4. essinn-1/maa-assistant — AGPL-3.0

### 用途
**本项目最直接的上游。** `essinn-1/maa-assistant` 是把 MaaEnd 骨架改造成「《棕色尘埃2》PC 端自动化」的早期工程。
本项目使用的**最初一批任务定义与 pipeline 编排**（`resource/pipeline/*.json` 的早期形态、`tasks/*.json`，
以及「资源吸收 / 召集」「PVP 入口」「EvilCastle 塔」「快速狩猎」「魔兽追踪者」等棕2 各系统入口）
与**初始图像素材**，均来自该仓库。

### 许可与授权
- 该仓库以 **AGPL-3.0** 发布（其首次提交即包含完整的 AGPL-3.0 条款文本）。
- AGPL-3.0 授予任何人**不可撤回**的复制、修改与再分发权利（§2「irrevocable」）——
  上游是否继续维护、是否停更，均**不影响**该授权的有效性。
- 本项目在其成果之上继续开发并公开发布，属**许可证明确授权的行为**。

### AGPL 履约与署名
本仓库的 Git 历史**完整保留**上游作者的提交记录与署名，未作 squash、未重写历史：

- 作者：`essinn-1 <3181517909@qq.com>`
- 提交区间：`feaaeb1`（2026-03-24，本仓库首个提交）… `eaa63ad`（2026-04-26）
- 本仓库自 2026-08-24 起由维护者 `alkaidjin` 接续提交

衍生作品整体仍以 **AGPL-3.0** 发布，源码完整公开于
[alkaidjin/Maa-Assistant-Browndust2](https://github.com/alkaidjin/Maa-Assistant-Browndust2)；
源代码的修改说明与日期见仓库根目录 [`更新功能说明.md`](更新功能说明.md) 与 Git 提交历史。

### 致谢
感谢 `essinn-1` 把《棕色尘埃2》PC 端从零跑通并开源——本项目正是站在这份工作的基础上继续维护的。

---

## 5. PaddlePaddle/PaddleOCR — Apache-2.0

### 用途
提供 OCR 文本检测与识别模型。PP-OCRv5 移动端模型基于 PaddleOCR 训练并导出，**模型文件以二进制形式跟随本 release zip 派发**：

- `resource/model/ocr/det.onnx`（4.53 MiB）
- `resource/model/ocr/rec.onnx`（15.75 MiB）
- `resource/model/ocr/keys.txt`（92 KiB，字符字典）

### Apache 2.0 履约
1. **版权声明保留**：上述模型来自 PaddleOCR 项目，Apache-2.0 要求「保留版权声明、保留 LICENSE 文件」。模型来自训练产物，已在上方表格列明出处；如需逐文件 LICENSE 副本，可在 PaddleOCR 仓库 `LICENSE` 文件查阅
2. **`NOTICE` 文件**（Notice for PaddleOCR）：如果你在 Apache-2.0 §4(d) 要求的 NOTICE 文件由 PaddleOCR 项目提供，请参阅 [PaddleOCR NOTICE](https://github.com/PaddlePaddle/PaddleOCR/blob/develop/NOTICE)；本项目不创建衍生 NOTICE 文件，只在本文件中汇总
3. **使用方式**：本项目不修改模型权重，仅通过 MaaFramework 的 `MaaOCR` 节点调用推理结果，使用方式见 `resource/pipeline/*.json` 中 type 为 `OCR` 的节点
4. **可替换性**：若要换用其他 OCR 模型（如 PaddleOCR 更新版本或其他 OCR 框架），把新导出的 `det.onnx` / `rec.onnx` / `keys.txt` 覆盖到 `resource/model/ocr/` 即可（保持文件名一致——MaaFramework 的 `OCR` 节点按固定路径加载）。注意这三个文件体积较大、已在 `.gitignore` 中排除，仓库不追踪它们，替换后需要重新打包才会生效。

---

## 6. electron/rcedit — MIT

### 用途
启动器在启动时给 `mxu.exe` 写入图标（`launcher.bat` → `BD2MAA-Updater.ps1` 的 `Apply-ExeIcon`），
使快捷方式与任务栏显示 `mxu.ico`；MXU 自更新替换 `mxu.exe` 后由启动器自动补写，无需用户手动操作。

### 在本项目中的位置
- 二进制：`tools/rcedit-x64.exe`（v2.0.0，GitHub, Inc 官方预编译 x64 构建）
- 随仓库跟踪，并随 release zip 派发；调用点为 `BD2MAA-Updater.ps1` 的 `Apply-ExeIcon`

### MIT 履约
MIT 许可要求保留版权声明与许可声明。本项目以**未经修改**的官方预编译二进制形式再分发，
版权归其原作者所有；上游源码与 `LICENSE` 全文见 [electron/rcedit](https://github.com/electron/rcedit)。

---

## 附录：复现 / 重新链接清单

如果你想从源码重建本项目的 `maafw/` / `mxu.exe`：

### MaaFramework（LGPL）
```bash
# 仅当你需要替换 maafw/* 时执行；本项目默认随 release zip 派发预编译版本
curl -L -O https://github.com/MaaXYZ/MaaFramework/releases/download/v5.13.0/MAA-win-x86_64-v5.13.0.zip
# 解压后取 bin/* + share/MaaAgentBinary/*，覆盖到 <BD2MAA>/maafw/
```

### MXU（AGPL）
```bash
git clone https://github.com/MistEO/MXU.git
# 按 MXU 仓库 README 中 AGPL 源的编译说明构建
# 替换 <BD2MAA>/mxu.exe
```

### PaddleOCR（Apache-2.0）
```bash
# 仅当你需要更新 OCR 模型时执行；本项目默认随 release zip 派发
# 从 PaddleOCR 官方仓库拉取对应版本导出 .onnx
# 覆盖 <BD2MAA>/resource/model/ocr/ 下三个文件
```

---

最后更新：2026-09-24

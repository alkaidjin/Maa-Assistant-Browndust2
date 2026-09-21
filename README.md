<!-- markdownlint-disable MD033 MD041 -->

<p align="center">
  <img src="mxu_icon.png" width="180" alt="BD2MAA 图标" />
</p>

<div align="center">

# Maa-Assistant-Browndust2

_✨ 《棕色尘埃2》PC 端日常自动化小助手 ✨_

基于 [MaaEnd](https://github.com/MaaEnd/MaaEnd) 改造 · Powered by [MaaFramework](https://github.com/MaaXYZ/MaaFramework) & [MXU](https://github.com/MistEO/MXU)

绝赞开发中 🎉……

</div>

## 📖 使用须知

> ⚠️ **启动方式（重要）**：请通过根目录的 **`launcher.bat`** 启动本软件，**不要直接双击 `mxu.exe`**。
> 首次启动时，launcher 会自动在根目录生成带图标的 **`MaaBd2.lnk`** 快捷方式（指向 launcher.bat）。之后你可以双击这个 lnk，或把它拖到桌面。
> 该启动器会在软件开启时**自动检测 GitHub 新版本**，发现更新时弹出提示窗口，并支持一键下载最新版；**弹窗 20 秒无操作会自动跳过更新、直接进入软件**，挂机 / 无人值守时不会卡在更新窗口。
> 更新检测基于 GitHub Releases；选择「一键更新」后会**自动下载并自动覆盖旧版本文件**（`resource/` 等软件文件随版本一并更新），**不会冲掉你的 `config/mxu-MaaBrownDust2.json` 用户配置**——新用户由 MXU 自动生成默认实例；老用户的真实任务实例100% 保留。
> 详细说明见 [更新功能说明](更新功能说明.md)。

- 游戏内语言请使用**简体中文**，并以**窗口化 16:9、1920×1080** 运行（分辨率、方向键等前置设置详见 [使用前必看（含常见问题解答）](%23请务必打开此文档查阅%23含使用方式以及常见问题解答.docx)）。
- 软件根目录里附带两个必读件：**[#请务必打开此文档查阅#含使用方式以及常见问题解答.docx](%23请务必打开此文档查阅%23含使用方式以及常见问题解答.docx)**（使用方式 + 常见问题 Q1–Q12）与 **[教学视频](%23使用本软件自动打开游戏并设定游戏分辨率的教学视频%23%20.mp4)**（跟着点一遍，1080P 窗口 + 前置程序就设好了）。
- 根目录可能出现一个 0 字节的隐藏文件「启动」：它来自**旧版 `launcher.bat`** 注释里的 `->`（被 cmd 当成重定向符而生成的），v26.09.7 已去掉这个来源；启动器仍会顺手清理 / 隐藏这类残留文件，`.gitignore` 也已忽略该文件名，对使用无影响。
- 更新只做「覆盖 + 新增」，**不会动你自己的文件**；但**旧版本遗留的说明文档 / 教学视频**（改过名、已下架的那几份）会在启动时按 `retired_files.json` 自动清掉，目录里永远只留当前版本这一份。想留旧档请先复制到软件目录之外。详见 [更新功能说明](更新功能说明.md) 第十二节。

## 📦 项目简介

本项目承接 [essinn-1/maa-assistant](https://github.com/essinn-1/maa-assistant)（把 [MaaEnd](https://github.com/MaaEnd/MaaEnd) 骨架改造为《棕色尘埃2》PC 端自动化的早期工程）继续开发，运行在 [MaaFramework](https://github.com/MaaXYZ/MaaFramework)（v5.13）运行时之上、由 [MXU](https://github.com/MistEO/MXU)（v2.5）作为 GUI 前端，支持自动日常任务、资源收集、战斗循环等功能。

> **来源与致谢**：本项目建立在两层上游工作之上 —— **MaaEnd** 提供了工程骨架（目录约定、`interface.json` 结构、多语言键），**`essinn-1/maa-assistant`** 完成了《棕色尘埃2》PC 端的早期适配与最初的 pipeline / 图像素材，两者均为 AGPL-3.0。
> 本仓库的 Git 历史**完整保留**了上游全部提交与作者署名，未作 squash、未重写历史；完整来源链与许可履约说明见 [NOTICE](NOTICE.md) 第 3、4 节。

## ✨ 部分功能说明

- **资源吸收和召集**：自动进入剧情卡带完成“探查 / 吸收 / 召集 / 制伏”，需先按说明配置天赋技能与传送阵（见 [使用前必看](%23请务必打开此文档查阅%23含使用方式以及常见问题解答.docx)）。
- **分解装备 + 精炼**：一键强化当天“免费抽抽乐”抽出的非 UR 装备后自动分解，并精炼第一页的 18 分装备。
- **启动即检测更新**：打开软件时自动检查 GitHub Releases 新版本，弹窗提示 + 一键下载+自动解压覆盖。覆盖采用"先 .new 再 rename"的原子模式，**中途断电不会留下半截文件**。
- **`debug/` 自动瘦身**：每次启动自动把 MaaFramework 的 `maa.log`（一轮任务 9~16 MB / 2~5 万行）**精简成可读的任务时间线**（约 99% 缩减），原始日志 gzip 归档备查；同时清理超过 7 天的日志、调试截图与归档（可用 `tools\compact_log.ps1` / `tools\clean_logs.ps1` 手动执行，天数见 `updater_config.json` 的 `log_retention_days`）。

## ⚖️ 开源许可证

### 本项目许可证

本项目基于 **AGPL-3.0** 协议开源。

[AGPL-3.0 License](LICENSE)

### 使用的开源项目

本项目使用了以下开源项目（详细致谢、各项目版本与「重新链接」履行说明见 [NOTICE](NOTICE.md)）：

#### 1. MaaXYZ/MaaFramework

- **项目地址**: https://github.com/MaaXYZ/MaaFramework
- **许可证**: **LGPL-3.0**
- **用途**: 运行时核心——图像识别、控制器抽象、任务编排、自定义识别/动作、节点通信
- **使用方式**: 以**动态链接库**形式调用 `maafw/*.dll` / `MaaNode.node`；未修改 MaaFramework 自身源码（LGPL §6「Anti-Distortion」义务最小化）。详见 [NOTICE](NOTICE.md) 第 1 节

#### 2. MistEO/MXU

- **项目地址**: https://github.com/MistEO/MXU
- **许可证**: AGPL-3.0
- **用途**: 桌面 GUI（Tauri）+ Go Agent 子进程 + 实例/设置持久化 + GitHub Releases 检测与下载
- **使用方式**: 单二进制 `mxu.exe`，由 `BD2MAA-Updater.ps1` 自动拉起；与本项目同 AGPL-3.0

#### 3. MaaEnd/MaaEnd

- **项目地址**: https://github.com/MaaEnd/MaaEnd
- **许可证**: AGPL-3.0
- **用途**: 本项目的起点仓库（BD2MAA fork 自 MaaEnd）；保留其工程骨架、目录约定与 AGPL §13 合规继承
- **使用方式**: 继承其任务框架；task-level JSON / pipeline / 图像资源均为本项目重写与扩充

#### 4. essinn-1/maa-assistant

- **项目地址**: https://github.com/essinn-1/maa-assistant
- **许可证**: **AGPL-3.0**
- **用途**: **本项目的直接上游** —— 把 MaaEnd 骨架改造为《棕色尘埃2》PC 端自动化的早期工程；本项目最初的 `resource/pipeline/*.json`、`tasks/*.json` 与图像素材均源自此处
- **使用方式**: 承接其 Git 历史继续开发（上游提交与作者署名完整保留），衍生作品整体仍以 AGPL-3.0 发布；详见 [NOTICE](NOTICE.md) 第 4 节

#### 5. PaddlePaddle/PaddleOCR（PP-OCRv5）

- **项目地址**: https://github.com/PaddlePaddle/PaddleOCR
- **许可证**: Apache License 2.0
- **用途**: 游戏内文字识别（OCR）模型
- **使用方式**: 静态模型文件 `resource/model/ocr/{det,rec}.onnx + keys.txt` 随 release zip 派发；由 MaaFramework 的 `OCR` 节点调用推理结果。详见 [NOTICE](NOTICE.md) 第 5 节

## 📜 声明

根据 AGPL-3.0 协议要求：

- 本项目及其衍生项目必须开源
- 任何对本项目的修改必须公开源代码
- 本项目仅供学习交流使用

根据 LGPL-3.0 要求：

- 本项目通过 `maafw/*.dll` 等运行时**动态链接** MaaFramework；运行库自身的源码已在 [MaaXYZ/MaaFramework](https://github.com/MaaXYZ/MaaFramework) 公开
- 用户可通过替换 `maafw/` 目录内的二进制**重新链接**到任意兼容版本的 MaaFramework；步骤见 [NOTICE](NOTICE.md) 「复现/重新链接清单」一节
- 本项目未对 MaaFramework 源码作任何衍生修改，因此 §6 的源码提供义务仅限于原上游

根据 Apache License 2.0 要求：

- 本项目使用了 PaddleOCR 训练的 PP-OCRv5 模型，保留其版权声明于 [NOTICE](NOTICE.md) 第 5 节

### 关于 Mirror酱（MirrorChyan）

**本项目已正式接入 [Mirror酱](https://mirrorchyan.com/zh/projects?rid=Maa-Assistant-Browndust2)，资源 ID：`Maa-Assistant-Browndust2`。**

- 软件内置更新：MXU「设置 → 更新」分区已启用。在 Mirror酱 官网为本项目购买 CDK 后填入其中，即可使用 Mirror酱的高速下载与自动更新。
- 🚀 **Mirror酱 项目页**：<https://mirrorchyan.com/zh/projects?rid=Maa-Assistant-Browndust2> —— 已购 CDK 的用户可在此页快速下载 / 更新。
- 双渠道并存：**不购买 CDK 也完全不影响使用** —— [GitHub Releases](https://github.com/alkaidjin/Maa-Assistant-Browndust2/releases) 永久免费下载，`launcher.bat` / `BD2MAA-Updater.ps1` 的版本检测与自动更新照常工作，两条渠道互不冲突。
- ⚠️ **购买 CDK 时认准资源 ID `Maa-Assistant-Browndust2`**：Mirror酱上另有一个与本项目**名称相似、但毫无关系**的第三方项目 —— 名称 `MaaBD2`、资源 ID `MFABD2`、仓库 [sunyink/MFABD2](https://github.com/sunyink/MFABD2)。
  它**不是本项目**（本项目仓库为 `alkaidjin/Maa-Assistant-Browndust2`），为其购买的 CDK 与本项目无关、也无法用于本项目。

## 📥 下载与使用

### 方式一：下载发布版本（推荐）

访问 [Releases](https://github.com/alkaidjin/Maa-Assistant-Browndust2/releases) 页面，下载最新版本的压缩包并解压即可使用。

### 方式二：从源码编译

```bash
# 克隆仓库
git clone https://github.com/alkaidjin/Maa-Assistant-Browndust2.git

# 具体编译步骤请参考 MaaFramework / MXU 官方文档
```

> 说明：仓库**不包含**体积最大的两样——预编译的 `mxu.exe` 与 OCR 模型 `resource/model/ocr/`（已在 `.gitignore` 中排除），它们随 [Releases](https://github.com/alkaidjin/Maa-Assistant-Browndust2/releases) 的 zip 派发。
> `maafw/` 下的 MaaFramework 运行时、`agent/go-service.exe`、`tools/rcedit-x64.exe` 等其余运行时**均已随仓库跟踪**，clone 后可直接打包。
> 需要自行补齐运行时，请按 [NOTICE](NOTICE.md) 附录的「复现 / 重新链接清单」下载对应版本并覆盖到对应目录。

### 自动更新（启动即检测 GitHub 新版本）

本项目的启动器（`launcher.bat` / `BD2MAA-Updater.ps1`）内置 **GitHub Releases 自动更新检测**：

- 每次启动都会先检查仓库是否有新版本；
- 发现新版本会**弹窗提示**（含版本号与更新日志），并可**一键下载**最新压缩包，或前往 GitHub 发布页；
- 检测完成后自动启动 `mxu.exe`；无网络 / 检测失败时也会照常进入软件，不会卡住；
- 选择「一键更新」后会**自动下载、解压并覆盖**旧版本（无需你手动解压合并）；更新时**自动保留 `config/` 下的用户配置**，不会冲掉你自建的任务实例。

### 使用方法

1. **下载/解压后，双击根目录的 `launcher.bat` 启动**（首次启动会自动在根目录生成带图标的 `MaaBd2.lnk`；之后可以双击这个 lnk，或把它拖到桌面）。也可以右键 `launcher.bat` → 发送到桌面快捷方式。
2. 想**预览更新弹窗效果**（即使当前已是最新），用 `launcher.bat -Demo` 运行。
3. 想**强制重新检测**（忽略缓存），用 `launcher.bat -Force` 运行。
4. 只想看检测结果、不弹窗不下载不启动软件，用 `launcher.bat -Test` 运行（会打印当前版本、GitHub 最新版本、选中的资源与是否需更新）。
5. 仓库地址、检测间隔、镜像源等可在 `updater_config.json` 中修改。
6. **程序图标**：图标源文件为 `mxu.ico`。MXU 自更新替换 `mxu.exe` 后，`launcher.bat` 下次启动会自动重新写入图标（无需手动操作）。详见 [更新功能说明](更新功能说明.md)。

> 说明：启动器为**纯 PowerShell 实现，零依赖、无需 Python**，详见 [更新功能说明](更新功能说明.md)。

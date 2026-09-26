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

## 📖 三份必读

| 文档 | 看它解决什么 |
|---|---|
| [#请务必打开此文档查阅#含使用方式以及常见问题解答.docx](%23请务必打开此文档查阅%23含使用方式以及常见问题解答.docx) | **使用前必看**：使用方式 + 常见问题 Q1–Q13（分辨率、前置程序、任务勾选…） |
| [教学视频](%23使用本软件自动打开游戏并设定游戏分辨率的教学视频%23%20.mp4) | 跟着点一遍，1080P 窗口 + 前置程序就设好了 |
| [更新功能说明](更新功能说明.md) | 启动器、自动更新、命令行参数、配置项、日志清理、常见故障排查 |

## 🚀 快速开始

1. **双击根目录的 `launcher.bat` 启动**（不要直接双击 `mxu.exe`）。首次启动会自动生成带图标的 `MaaBd2.lnk` 快捷方式，之后双击它、或拖到桌面都行。
2. 游戏内语言设为**简体中文**，以**窗口化 16:9** 运行（推荐 1920×1080）。只能用固定档位（1280×720 / 1600×900 / 1920×1080 / 2560×1440 / 3840×2160），**不要用鼠标拖窗口边框改大小**——差几个像素软件就会在任务开始前把它停掉。
3. 打开软件 → 勾选任务 → 开始。每项任务的可选项（执行周期、刷本策略、购买清单等）都在界面里勾，无需改文件。

> `launcher.bat` 每次启动都会先检测 GitHub 新版本：发现新版弹窗提示、可一键下载并自动覆盖，**不会冲掉你的 `config/` 用户配置**；弹窗 20 秒无操作自动跳过，挂机 / 无人值守不会被卡住。想预览弹窗用 `launcher.bat -Demo`。完整说明见 [更新功能说明](更新功能说明.md)。

## 📦 项目简介

本项目承接 [essinn-1/maa-assistant](https://github.com/essinn-1/maa-assistant)（把 [MaaEnd](https://github.com/MaaEnd/MaaEnd) 骨架改造为《棕色尘埃2》PC 端自动化的早期工程）继续开发，运行在 [MaaFramework](https://github.com/MaaXYZ/MaaFramework)（v5.13）运行时之上、由 [MXU](https://github.com/MistEO/MXU)（v2.5）作为 GUI 前端，支持自动日常任务、资源收集、战斗循环等功能。

> **来源与致谢**：本项目建立在两层上游工作之上 —— **MaaEnd** 提供了工程骨架（目录约定、`interface.json` 结构、多语言键），**`essinn-1/maa-assistant`** 完成了《棕色尘埃2》PC 端的早期适配与最初的 pipeline / 图像素材，两者均为 AGPL-3.0。
> 本仓库的 Git 历史**完整保留**了上游全部提交与作者署名，未作 squash、未重写历史；完整来源链与许可履约说明见 [NOTICE](NOTICE.md) 第 3、4 节。

## ✨ 功能概览

- **日常一条龙**：`日常收菜`（公会低保 + 经营管理 + 广场女神像）、`奖励一键领取`（每日任务 + 通行证 + 邮箱 + 活动奖励）。
- **资源吸收和召集**：`首轮` / `次轮` 两个独立任务，各自可选执行周期（按游戏日勾选，非执行日整任务跳过）。
- **装备分解 + 精炼**：把当天「免费抽抽乐」抽出的非 UR 装备一键强化后自动分解，并精炼 18 / 19 / 20 分装备；已穿戴装备自动屏蔽。
- **狩猎场**：「刷取的圣石属性」新增**「自动：刷数量最少的圣石」**——自动读取右上角资源栏五种圣石数量，刷当前最少的一种，长期各属性自然均衡；一旦读不到数量会弹提示并中止该任务，绝不会乱刷。
- **赛季活动**：推图 + 快速战斗、回归赛季魔兽快速战斗、赛季活动商店一键购买（商品与执行周期全部自选）。
- **其它**：救赎之塔（肉鸽塔快速战斗）、镜中之战 PVP、每日免费抽抽乐、自动进入游戏。

## 📥 下载与安装

### 方式一：下载发布版本（推荐）

访问 [Releases](https://github.com/alkaidjin/Maa-Assistant-Browndust2/releases) 页面，下载最新版本的压缩包并解压即可使用。

### 方式二：从源码

```bash
git clone https://github.com/alkaidjin/Maa-Assistant-Browndust2.git
# 编译步骤请参考 MaaFramework / MXU 官方文档
```

> 仓库**不包含**体积最大的两样——预编译的 `mxu.exe` 与 OCR 模型 `resource/model/ocr/`（已在 `.gitignore` 中排除），它们随 [Releases](https://github.com/alkaidjin/Maa-Assistant-Browndust2/releases) 的 zip 派发。
> `maafw/` 下的 MaaFramework 运行时、`agent/*.exe`、`tools/rcedit-x64.exe` 等其余运行时**均已随仓库跟踪**，clone 后可直接打包。需要自行补齐运行时，请按 [NOTICE](NOTICE.md) 附录的「复现 / 重新链接清单」下载对应版本覆盖。

## ⚖️ 开源许可证

本项目基于 **AGPL-3.0** 协议开源 —— [LICENSE](LICENSE)。

本项目使用了以下开源项目，**详细致谢、各项目版本与「重新链接」履行说明见 [NOTICE](NOTICE.md)**：

| # | 项目 | 许可证 | 用途 |
|---|---|---|---|
| 1 | [MaaXYZ/MaaFramework](https://github.com/MaaXYZ/MaaFramework) | **LGPL-3.0** | 运行时核心——图像识别、控制器抽象、任务编排 |
| 2 | [MistEO/MXU](https://github.com/MistEO/MXU) | AGPL-3.0 | 桌面 GUI + Go Agent 子进程 + 实例/设置持久化 |
| 3 | [MaaEnd/MaaEnd](https://github.com/MaaEnd/MaaEnd) | AGPL-3.0 | 本项目的起点仓库，提供工程骨架与目录约定 |
| 4 | [essinn-1/maa-assistant](https://github.com/essinn-1/maa-assistant) | **AGPL-3.0** | **本项目的直接上游**——棕2 PC 端早期适配与最初的资源 |
| 5 | [PaddlePaddle/PaddleOCR](https://github.com/PaddlePaddle/PaddleOCR) | Apache-2.0 | OCR 文本识别模型（PP-OCRv5） |
| 6 | [electron/rcedit](https://github.com/electron/rcedit) | MIT | 启动器给 `mxu.exe` 写图标的工具 |

- 本项目及其衍生项目必须开源；任何对本项目的修改必须公开源代码；仅供学习交流使用。
- 本项目通过 `maafw/*.dll` 等运行时**动态链接** MaaFramework，未对其源码作任何衍生修改；你可随时替换 `maafw/` 内的二进制**重新链接**到任意兼容版本（步骤见 [NOTICE](NOTICE.md) 附录）。
- 本项目使用了 PaddleOCR 训练的 PP-OCRv5 模型，其版权声明保留于 [NOTICE](NOTICE.md) 第 5 节。

### 关于 Mirror酱（MirrorChyan）

**本项目已正式接入 [Mirror酱](https://mirrorchyan.com/zh/projects?rid=Maa-Assistant-Browndust2)，资源 ID：`Maa-Assistant-Browndust2`。**

- 在 Mirror酱官网为本项目购买 CDK 后，填入 MXU「设置 → 更新」即可使用高速下载与自动更新（新版本由仓库内置的 GitHub Actions 自动上传，无需人工搬运）。
- **不购买 CDK 也完全不影响使用**：[GitHub Releases](https://github.com/alkaidjin/Maa-Assistant-Browndust2/releases) 永久免费下载，`launcher.bat` 的版本检测与自动更新照常工作，两条渠道互不冲突。
- ⚠️ **购买 CDK 时认准资源 ID `Maa-Assistant-Browndust2`**：Mirror酱上另有一个与本项目**名称相似、但毫无关系**的第三方项目 —— 名称 `MaaBD2`、资源 ID `MFABD2`、仓库 [sunyink/MFABD2](https://github.com/sunyink/MFABD2)。它**不是本项目**，为其购买的 CDK 与本项目无关、也无法用于本项目。

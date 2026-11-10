<!-- markdownlint-disable MD033 MD041 -->

<p align="center">
  <img src="data/maintainer/tools/mxu_icon.png" width="180" alt="BD2MAA 图标" />
</p>

<div align="center">

# Maa-Assistant-Browndust2

_✨ 《棕色尘埃2》PC 端日常自动化小助手 ✨_

基于 [MaaEnd](https://github.com/MaaEnd/MaaEnd) 改造 · Powered by [MaaFramework](https://github.com/MaaXYZ/MaaFramework) & [MXU](https://github.com/MistEO/MXU)

绝赞开发中 🎉……

</div>

> **免费开源，请勿付费购买**
>
> 本程序是**免费开源的自由软件**。如您通过任何渠道**付费购买**了本程序，请**立即退款**。
> 如您有意支持本项目的开发，欢迎通过订阅 **Mirror酱** 赞助开发者（[Mirror酱项目页](https://mirrorchyan.com/zh/projects?rid=Maa-Assistant-Browndust2)）。

## 📖 必读

使用前请先打开根目录的：**[#请务必打开此文档查阅#含使用方式以及常见问题解答.docx](%23请务必打开此文档查阅%23含使用方式以及常见问题解答.docx)**

内含使用方式与常见问题 Q1–Q13（分辨率、前置程序、任务勾选等）。

## 🚀 快速开始

1. 双击根目录 **`MaaBd2.exe`** 启动软件。
2. 游戏内语言设为**简体中文**，窗口固定 **窗口化 1920×1080**（不要用鼠标拖边框改大小）。
3. 在软件里勾选任务，点「开始任务」。执行周期、刷本策略、购买清单等都在任务卡片里选。
4. 推荐加一张「启动游戏」卡片并保持勾选：开始时会自动找到游戏路径并以 1920×1080 拉起；游戏已开着则直接绑定窗口。

## 📦 项目简介

本项目承接 [essinn-1/maa-assistant](https://github.com/essinn-1/maa-assistant)（把 [MaaEnd](https://github.com/MaaEnd/MaaEnd) 骨架改造为《棕色尘埃2》PC 端自动化的早期工程）继续开发，运行在 [MaaFramework](https://github.com/MaaXYZ/MaaFramework) 运行时之上、由 [MXU](https://github.com/MistEO/MXU) 作为 GUI 前端，支持自动日常任务、资源收集、战斗循环等功能。

上游来源、许可履约与「重新链接」说明见 [NOTICE](NOTICE.md)。

## ✨ 功能概览

- **启动游戏（推荐置顶启用）**：自动检索本机游戏安装路径，启动并以窗口化 1920×1080 拉起游戏；游戏已开着则直接绑定窗口。首次需在「添加任务」面板添加一次，之后保持勾选即可。
- **一条龙日常**：完成每日抽抽乐、公会低保、经营管理收菜、广场女神像奖励，镜中之战 PVP、快速狩猎、分解和精炼装备、赛季活动推关扫荡等流程。
- **奖励一键领取**：完成每日 / 每周任务奖励、通行证、邮箱物品和活动奖励的自动领取。
- **每日跑商**：在广场商人洛兹处完成收藏列表重建（可选）与一键购买；可选制作高价值料理（带星期门禁）；按内置的高价品出售表出售当日高价商品。
- **其余每周任务**：按顺序完成装备制作、末日之书、小屋点赞等任务（有星期门禁自选）。
- **救赎之塔**：快速战斗扫荡刷取黑曜石（需先手动通关任意一层解锁快速战斗）。
- **赛季活动商店购买**：自动购买选定的赛季活动商品（有星期门禁自选，不购买赛季装备和强化材料）。
- **地图资源收集**：含两轮次的地图资源的吸收 + 召集 + 制伏（天赋技能请自行配置，地图和传送阵自行解锁，有星期门禁）。
- **自动钓鱼（低效版）**：纯图像识别的钓鱼闭环（HSV 颜色追踪 + 速度预测点击），提供多个次数档位，建议使用弱点判定区域大的钓竿。

## 📥 下载与安装

### 方式一：下载发布版本（推荐）

访问 [Releases](https://github.com/alkaidjin/Maa-Assistant-Browndust2/releases) 页面，下载最新版本的压缩包并解压即可使用。

### 方式二：从源码

```bash
git clone https://github.com/alkaidjin/Maa-Assistant-Browndust2.git
# 编译步骤请参考 MaaFramework / MXU 官方文档
```

### 更新

打开软件 →「设置 - 更新」。有 CDK 走 Mirror酱，无 CDK 自动回退 GitHub（不会冲掉 `config/`）。
详细说明见 [更新功能说明](data/更新功能说明.md)。

从源码构建、仓库结构与运行时补齐方式见 [NOTICE](NOTICE.md) 与 [data/maintainer/README](data/maintainer/README.md)。

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
| 6 | [electron/rcedit](https://github.com/electron/rcedit) | MIT | 维护者给 `MaaBd2.exe` 写图标的工具（只在 `data/maintainer/` 下，**不随发布包派发**） |

- 本项目及其衍生项目必须开源；任何对本项目的修改必须公开源代码；仅供学习交流使用。
- 本项目通过 `maafw/*.dll` 等运行时**动态链接** MaaFramework，未对其源码作任何衍生修改；你可随时替换 `maafw/` 内的二进制**重新链接**到任意兼容版本（步骤见 [NOTICE](NOTICE.md) 附录）。
- 本项目使用了 PaddleOCR 训练的 PP-OCRv5 模型，其版权声明保留于 [NOTICE](NOTICE.md) 第 5 节。

### 关于 Mirror酱（MirrorChyan）

**本项目已正式接入 [Mirror酱](https://mirrorchyan.com/zh/projects?rid=Maa-Assistant-Browndust2)，资源 ID：`Maa-Assistant-Browndust2`。**

- 在 Mirror酱官网为本项目购买 CDK 后，填入 MXU「设置 → 更新」即可使用高速下载与自动更新（新版本由仓库内置的 GitHub Actions 自动上传，无需人工搬运）。
- 无需 CDK 也能用：[GitHub Releases](https://github.com/alkaidjin/Maa-Assistant-Browndust2/releases) 免费下载，软件内更新在无 CDK 时自动回退到 GitHub。
- ⚠️ **购买 CDK 时认准资源 ID `Maa-Assistant-Browndust2`**：Mirror酱上另有一个与本项目**名称相似、但毫无关系**的第三方项目 —— 名称 `MaaBD2`、资源 ID `MFABD2`、仓库 [sunyink/MFABD2](https://github.com/sunyink/MFABD2)。它**不是本项目**，为其购买的 CDK 与本项目无关、也无法用于本项目。

## 🙏 致谢

感谢每一位使用、反馈和测试本项目的玩家——你们的 bug 报告、失败截图和实际使用场景，是这个工具能持续变可靠的原因。

感谢开源社区：本项目的上游 [MaaEnd](https://github.com/MaaEnd/MaaEnd) 与 [essinn-1/maa-assistant](https://github.com/essinn-1/maa-assistant)，以及 [MaaFramework](https://github.com/MaaXYZ/MaaFramework)、[MXU](https://github.com/MistEO/MXU)、[PaddleOCR](https://github.com/PaddlePaddle/PaddleOCR) 等项目的作者与贡献者。

如果你觉得这个项目有用，最好的支持是把它用起来，把遇到的问题反馈回来。

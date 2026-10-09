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

## 📖 三份必读

| 文档 | 看它解决什么 |
|---|---|
| [#请务必打开此文档查阅#含使用方式以及常见问题解答.docx](%23请务必打开此文档查阅%23含使用方式以及常见问题解答.docx) | **使用前必看**：使用方式 + 常见问题 Q1–Q13（分辨率、前置程序、任务勾选…） |
| [更新功能说明](data/更新功能说明.md) | 软件内更新（MXU「设置 - 更新」/ Mirror酱 / GitHub）、手动更新、旧版遗留文件清理、故障排查 |

## 🚀 快速开始

1. **双击根目录的 `MaaBd2.exe` 打开软件**（图标已固化在程序内，不需要再手动改图标）。想从桌面进：右键 `MaaBd2.exe` →「发送到」→「桌面快捷方式」即可。
   > ⚠️ **老用户（v26.09.13 及更早）请注意**：以前是靠 `launcher.bat` / `mxu.exe` 启动的，**v26.10.1 起前者已移除、后者已改名为 `MaaBd2.exe`**。
   > ① 若你桌面的快捷方式指向旧名字，它会提示“文件已不存在” —— 删掉它，重新建一个指向 `MaaBd2.exe` 的。
   > ② **软件内更新完成后可能不会自动重启**：更新程序记住的还是旧名字 `mxu.exe`，改名后它找不到。看到“更新完成”却没弹出软件时，**自己去双击 `MaaBd2.exe` 就行** —— 只有这次升级会遇到，之后再更新就正常了。
2. 游戏内语言设为**简体中文**，游戏窗口固定用**窗口化 1920×1080**（目前只实机验证过这一档，其他分辨率暂不支持）。**不要用鼠标拖窗口边框改大小**——差几个像素软件就会在任务开始前把它停掉。
3. 打开软件 → 勾选任务 → 开始。每项任务的可选项（执行周期、刷本策略、购买清单等）都在界面里勾，无需改文件。
4. **（可选，推荐）一键启动游戏**：在「添加任务」面板添加「启动游戏」卡片并保持勾选 —— 点开始后会自动检索本机游戏安装路径（启动器注册表 → 各盘默认路径 → 卸载信息 → 上次成功路径记忆，支持其他盘位与中文路径）并以窗口化 1920×1080 拉起游戏（已开着则直接绑定），不再需要手动配置旧版「前置程序」。失败原因见 `debug/go-service.log`。

> **更新走软件内通道**：打开软件 →「设置 - 更新」→ 有新版会提示下载并安装（**不会冲掉你的 `config/` 配置**）。
> 更新过程中它还会按包内的 `changes.json` 自动清掉旧版遗留文件。程序图标已固化在 exe 内，更新后不会变回默认图标。
> 软件内更新走不通时，去 [Releases](https://github.com/alkaidjin/Maa-Assistant-Browndust2/releases) 手动下载 zip 解压覆盖即可（同样不会动 `config/`）。完整说明见 [更新功能说明](data/更新功能说明.md)。

## 📦 项目简介

本项目承接 [essinn-1/maa-assistant](https://github.com/essinn-1/maa-assistant)（把 [MaaEnd](https://github.com/MaaEnd/MaaEnd) 骨架改造为《棕色尘埃2》PC 端自动化的早期工程）继续开发，运行在 [MaaFramework](https://github.com/MaaXYZ/MaaFramework)（v5.13）运行时之上、由 [MXU](https://github.com/MistEO/MXU)（v2.5）作为 GUI 前端，支持自动日常任务、资源收集、战斗循环等功能。

> 🌐 **语言**：软件界面与任务文案**仅提供简体中文**（v26.10.1 起界面不再声明其他语种；游戏内也需设为简体中文）。go-service 守护进程的分辨率 / 进程告警页仍内置简繁英日韩 5 语言资源，那是沿用上游的运行时组件，不代表软件界面支持多语言。

> **来源与致谢**：本项目建立在两层上游工作之上 —— **MaaEnd** 提供了工程骨架（目录约定、`interface.json` 结构、多语言键），**`essinn-1/maa-assistant`** 完成了《棕色尘埃2》PC 端的早期适配与最初的 pipeline / 图像素材，两者均为 AGPL-3.0。
> 本仓库的 Git 历史**完整保留**了上游全部提交与作者署名，未作 squash、未重写历史；完整来源链与许可履约说明见 [NOTICE](NOTICE.md) 第 3、4 节。

## ✨ 功能概览

- **一条龙日常**：一张卡按固定顺序跑完每日抽抽乐 → 日常收菜（公会低保 + 小屋 + 经营管理 + 广场女神像）→ 镜中之战 PVP → 快速狩猎 → 分解 + 精炼装备 → 赛季活动推关 / 快速战斗。狩猎刷圣石支持「自动：刷数量最少的一种」（读不到数量会提示并中止，绝不乱刷）；分解会一键强化吃掉非 UR 装备、精炼 18 / 19 / 20 分装备，已穿戴装备自动屏蔽。
- **奖励一键领取**：每日任务 → 通行证 → 邮箱 → 活动奖励，一条链依次领完。
- **资源吸收和召集**：首轮 / 次轮合并为一张卡，两轮执行周期可分别自选（按游戏日门控，非执行日整轮跳过；同一天两轮都勾时当天只跑首轮）。⚠️ 从旧版升级后**次轮周期需在卡片上重新勾选一次**（默认周二）。
- **每日跑商**：前往广场找商人洛兹，砍价后一键购买全部收藏；可选做高价值料理再高价卖出；可选「重建商店收藏清单」，自动按路线完成 205 个商品的收藏；商品售罄有提示兜底，不会卡在商店界面。
- **其余每周任务**：装备制作、末日之书、小屋点赞等，按星期门控一周只跑一次，执行星期可在卡片上自选（默认周一）。
- **救赎之塔**：肉鸽塔快速战斗刷黑曜石（需先手动通关任意一层开启快速战斗）。
- **赛季活动商店**：女神之泪、抽抽乐券等购买商品与执行周期全部自选（魔兽装备及强化道具需自行购买）。
- **自动钓鱼（测试）**：纯图像识别的钓鱼闭环（HSV 颜色追踪 + 速度预测点击），提供多个次数档位，建议使用弱点判定区域大的钓竿。
- **启动游戏（前置任务）**：点开始后自动检索本机游戏安装路径（启动器注册表 → 各盘默认路径 → 卸载信息 → 上次成功路径记忆），以窗口化 1920×1080 拉起游戏；游戏已开着则直接绑定窗口。首次需在「添加任务」面板添加一次，之后保持勾选即可。

## 📥 下载与安装

### 方式一：下载发布版本（推荐）

访问 [Releases](https://github.com/alkaidjin/Maa-Assistant-Browndust2/releases) 页面，下载最新版本的压缩包并解压即可使用。

### 方式二：从源码

```bash
git clone https://github.com/alkaidjin/Maa-Assistant-Browndust2.git
# 编译步骤请参考 MaaFramework / MXU 官方文档
```

> 主程序 `MaaBd2.exe`（31 MB，图标已固化在程序内）**已随仓库跟踪**，clone 下来即可直接运行 / 打包；仓库**不包含**的只有 OCR 模型 `resource/model/ocr/`（体积最大，已在 `.gitignore` 中排除），它随 [Releases](https://github.com/alkaidjin/Maa-Assistant-Browndust2/releases) 的 zip 派发。
> `maafw/` 下的 MaaFramework 运行时、`agent/*.exe` 等其余运行时**均已随仓库跟踪**，clone 后可直接打包。需要自行补齐运行时，请按 [NOTICE](NOTICE.md) 附录的「复现 / 重新链接清单」下载对应版本覆盖。
> 维护者工具（发布包构建器、备份器、图标工具等）集中在 **`data/maintainer/`** 目录，它们**只入仓库、不进发布包**（详见 `data/maintainer/README.md`）。
> **目录布局**：`resource/` `tasks/` `misc/` `agent/` `maafw/` `locales/` 六个运行时目录留在根目录（MXU 与 go-service 按相对 exe 目录 / `interface.json` 里的路径找它们），`interface.json` / `LICENSE` / `README.md` / `NOTICE.md` / `MaaBd2.exe` / 注意事项 docx 六个文件也在根目录（注意事项 docx 是「使用前必看」，刻意留在最显眼的位置）；只有维护者档案与次要文档收进 `data/`（`version.json` / `retired_files.json` / `更新功能说明.md` / `maintainer/`）。

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
- **不购买 CDK 也完全不影响使用**：[GitHub Releases](https://github.com/alkaidjin/Maa-Assistant-Browndust2/releases) 永久免费下载，软件内「设置 - 更新」在无 CDK 时会自动回退到 GitHub 下载；国内直连 GitHub 慢的话，也可以手动下载 zip 后解压覆盖，两条渠道互不冲突。
- ⚠️ **购买 CDK 时认准资源 ID `Maa-Assistant-Browndust2`**：Mirror酱上另有一个与本项目**名称相似、但毫无关系**的第三方项目 —— 名称 `MaaBD2`、资源 ID `MFABD2`、仓库 [sunyink/MFABD2](https://github.com/sunyink/MFABD2)。它**不是本项目**，为其购买的 CDK 与本项目无关、也无法用于本项目。

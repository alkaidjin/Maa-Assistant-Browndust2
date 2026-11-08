---
name: maa-browndust2
description: 操作「Maa-Assistant-Browndust2 / MaaBrownDust2 / BD2 小助手」（棕色尘埃2 BrownDust II 自动化工具）：查状态、跑任务并等它结束、自动关窗口、把结果通知给用户、改配置、读日志排查失败。用户提到棕色尘埃、BD2、小助手、一条龙日常、每周任务、自动钓鱼、跑商，或说「帮我跑一下任务」「跑完告诉我」时使用。
metadata:
  version: "2"
---

# Maa-Assistant-Browndust2

## 名字要分清（说错会被用户纠正）

- **「棕色尘埃2日常」= 一整套自动化**（多个任务按顺序跑一轮），记录与卡片上的"这一轮"就显示这个名字
- **「一条龙日常」= 其中的第一个任务**（`name: DailyAll`，入口 `StartGame`）
- 别再把它俩当同一件事；汇报时说"棕色尘埃2日常这轮跑完了"，而不是"一条龙日常跑完了"

自动化名称可在 `config\bd2cli.json` 的 `automation_name` 改（默认「棕色尘埃2日常」）。

自动化由 **MXU（图形界面）+ MaaFramework（识别引擎）** 完成；你通过项目根目录的 **`agent/bd2cli.exe`** 指挥它。

**核心原则：你不需要会玩游戏，也不要自己去点游戏窗口。**
低权限进程点不动游戏（Windows 会拦），正确做法永远是让 MXU 去跑、你只负责发起和读结果。

## 1. 找到项目位置

1. 用户给的路径。
2. 含 `interface.json` 的目录（`bd2cli` 会自动向上查找，也可 `--root` 指定）。
3. 常见：`D:\Maa-Assistant-Browndust2`。**不要全盘搜索**，找不到就问。

目录里应有：`interface.json`、`mxu.exe`、`maafw\`、`resource\`、`agent\`、`config\`、`debug\`、`logs\`。

## 1.5 用户还没装（两种情况，前提都是用户自备 agent）

**A. 用户只给了仓库地址** → 从 Releases 下载最新用户包（zip 里已含 `mxu.exe`、`maafw\`、`agent\*.exe`、`resource\`），解压到用户指定目录。
不要用 `git clone` 的源码目录直接跑：源码里 `mxu.exe` 只在 `cache\_mxu_253\`，不在根目录，MXU 会报"加载失败"。若只有源码，先把 `mxu.exe` 放到根目录。

**B. 用户已自己下载了用户包** → 解压后按第 2 节做一次性准备即可。

装完必做两步验证：

```bat
agent\bdcli.exe selfcheck        :: 必须 bundle_ok: true
agent\bdcli.exe elevate install  :: 一次 UAC（用户点"是"）
```

**首次安装一定要主动提醒用户**（否则任务会"秒成功但不干活"）：

1. **星期门禁**：`其余每周任务`、`赛季活动商店`、`资源吸收和召集`、`每日跑商` 都按选中的星期执行，
   不选中当天就**直接跳过**（任务秒结束、状态成功但没做事）。让用户按需求勾选星期。
2. **刷取策略**（影响收益，默认值不一定合适）：
   - `一条龙日常 → 快速狩猎`：`ChooseHuntStrategy`（如 MaterialOnly / 装备刷取）
   - `一条龙日常 → 镜中之战`：`设置鸡尾酒倍数`（消耗倍数）
   - `一条龙日常 → 挖矿/木材`：`ChooseWood`、`ChooseRock`
   - `赛季活动商店 → 购买物品`：`WarcraftShopBuyItems` / `WarcraftShopBuyLeft`
3. **改法**：`bdcli config set mxu.instances.0.tasks.<i>.optionValues...` 比较绕，更稳的是让用户在 MXU 界面里点选；
   AI 可先用 `bdcli --status --json` 读出当前值再解释。

## 2. 一次性准备（只在第一次做）

```bat
agent\bd2cli.exe elevate install
```
会弹一次 UAC（请用户点"是"），创建计划任务 `MaaBrownDust2NoUAC`。之后**关窗口、提权操作都不再弹窗**。
用 `elevate status` 确认 `installed: true`。**这一步必须由用户本人授权，不要替他点。**

其余（`autoRunOnLaunch`、`autoStartInstanceId`）`bd2cli run` 会自动写好，不用管。

## 3. 命令

在项目根目录执行：

| 命令 | 作用 |
|---|---|
| `bd2cli --status --json` | 版本、MXU 是否在跑、游戏窗口、实例与任务勾选、正在运行的任务、最近 5 次运行、日志尾部 |
| `bd2cli --list --json` | 全部任务：`name`/`label`/`entry`/`description` |
| `bd2cli run <任务名> --json` | **默认：启动 → 等结束 → 判定成败 → 自动关 MXU 和游戏 → 通知用户** |
| `bd2cli run <任务名> --no-close` | 同上但不关窗口 |
| `bd2cli run <任务名> --keep-game` | 只关 MXU，保留游戏 |
| `bd2cli run <任务名> --no-wait` | 启动后立刻返回（不推荐，就拿不到结果了） |
| `bd2cli only <任务名>` | 临时只跑这一个任务（其余非前置任务关掉） |
| `bd2cli only --restore` | 还原任务勾选 |
| `bd2cli unlock` | 清"正在运行"的残留锁（进程被强杀后用） |
| `bd2cli elevate status\|install\|remove\|kill` | 提权任务管理；`kill` 立即结束 MXU/游戏 |
| `bd2cli diagnose [--id <编号>]` | 打包失败证据（截图/日志尾部/脱敏配置），让用户转给开发者 |
| `bd2cli report [--id <编号>] [--embed] [--history]` | **只出最近一轮**的简洁结果卡片（结论 + 3~5 行 + 页脚），不列节点流程 |
| `bd2cli autostart` | 查看自动化状态；**主动告诉用户有两种免手动方案**（见 §5） |
| `bd2cli autostart daily 08:30` | 每天定时自动跑（时间可改） |
| `bd2cli autostart logon` | 开机/登录后自动跑 |
| `bd2cli autostart off` | 关闭自动化 |
| `bd2cli config get mxu.<路径>` / `set ... <值>` | 改 MXU 配置单键（自动 .bak） |
| `bd2cli logs tail --lines 80` | 打印 `debug\maafw.log` 尾部 |
| `bd2cli selfcheck` | 只验证引擎初始化与资源加载（不需要游戏） |
| `bd2cli doctor` | **环境体检**：interface / mxu.exe 位置 / maafw / resource / agent / 授权 / 运行锁 / 首次成功 / 游戏路径，9 项，逐项给修法 |
| `bd2cli follow [--id X]` | 跟随当前这一轮直到结束（边跑边看输出） |
| `bd2cli control stop` | **停止整轮**（退出码 6）；`control status` 看支持情况 |
| `bd2cli result` / `wait` | 直接取最近一轮结果 / 阻塞等到结束 |
| `bd2cli ends` | 打印每个任务的完成标志节点（核对判定依据） |

任务名可写 `name`（`DailyAll`）或中文 `label`（`一条龙日常`）。

### 退出码

| 码 | 含义 | 你该做什么 |
|---|---|---|
| 0 | 任务成功 | 汇报结果 |
| 1 | 任务失败 | 读 `logs\runs\<id>.json` 的 `reason`，再按第 6 节排查 |
| 2 | 参数/未知任务 | `--list` 核对 |
| 3 | 前置不满足 | 没装 MXU、任务未勾选、游戏窗口没有等 |
| 4 | 已有任务在跑 | 等它结束，或 `unlock` 清残留锁 |
| 5 | 权限/能力不支持 | 让用户 `elevate install` |
| 6 | 被中止 | 问用户是否重跑 |

## 4. 结果怎么给用户看（默认：看板，不推手机）

**硬性要求：跑完必须主动出结果，不能让用户来问。**

每轮结束 `bd2cli` 会自动写 `logs\latest-result.json`（含状态、耗时、逐任务结果、卡片路径）。
取结果有两种方式：

```bat
agent\bd2cli.exe result        :: 直接读最近一轮结果（JSON，含 card 路径）
agent\bd2cli.exe wait --timeout 3h   :: 阻塞到这一轮结束，再输出结果
```

推荐做法：`run` 或 `wait` 放后台跑，**进程一结束就立刻把卡片呈给用户**，
并按模板汇报"哪些成功、哪些失败、失败卡在哪个节点、诊断包在哪"。

跑完**默认在看板里展示**，不推送手机：

```bat
agent\bd2cli.exe report --embed
```
生成 `logs\report-<运行编号>.html`：**只针对最近一轮**——大字状态、任务名、起止时间、耗时、
这一轮执行过的节点链（失败的最后节点标红）、失败原因。
**不要用历史图表汇总**：同一个任务跑多次会出现多个同名条目，用户分不清哪轮最新。
需要指定某一轮用 `report --id <编号>`；`--history` 仅用于长期分析。

**手机推送是可选的扩展**，默认关闭（`config\bd2cli.json` 里 `provider: "none"`）。
用户明确想要时，由你引导他自己开通，流程：

1. 问用户想用哪种：`qmsg`（QQ 私聊）/ `webhook`（企业微信机器人等）/ `file`
2. 让用户自己拿到凭据（Qmsg key、QQ 号；或 webhook 地址）
3. 你把凭据写进 `config\bd2cli.json` 的 `notify` 段（**不要显示在对话里**）
4. 跑 `bd2cli notify test` 验证；成功后再在 `run` 时加 `--notify`（不加 `--no-notify` 才发）

推送内容与看板一致：任务、状态、耗时、记录路径、失败原因。

## 5. 跑任务前确认

1. `--status --json`：
   - `mxu.running`：MXU 是否已在跑（在跑就别再启动第二个）
   - `game.window_found`：游戏窗口；没有也行，前置任务会拉起
   - `active_run`：非空说明有任务在跑，不要叠
2. 不确定就先 `run <任务> --dry-run`（只打印，不动手）
3. 任务会操控游戏，提醒用户期间不要抢操作
4. 耗时参考：`奖励一键领取` 2–3 分钟；`DailyAll` 20–60 分钟；`WeeklyTasks` 3–15 分钟。
   长任务把 `--timeout` 调大（默认 30m）。

## 5.5 任务失败（程序已自动处理，你别重复造轮子）

**先看失败截图再下结论**：`debug\on_error\*.png`（或诊断包里的 `evidence/`）能直接看出是哪个界面卡住。

已知的两类"不是 bug 的失败"：

- **救赎之塔**：如果截图里「快速战斗」按钮是灰的，说明**该层没解锁**或**当天快速战斗次数用完了**（右上角 `0/10`）。
  这是游戏侧限制，重试没有意义——已在 `no_retry_tasks` 名单里，程序不会重试。如实告诉用户"今天次数用完了/这层没解锁"即可，
  **不要**去改 pipeline 绕过，也**不要**反复重跑。
- **星期门禁**：`其余每周任务`/`赛季活动商店`/`地图资源收集`/`每日跑商` 未选中当天会直接跳过，
  状态是"完成但跳过"，不是失败，也不要重跑。

1. `run` 失败会**自动重试一次**（`--no-retry` 可关）。重试前会先把上一次的失败截图与日志尾部
   固化到 `logs\evidence\<编号>\`——因为重试要重启 MXU，而 MXU 启动会清空 `debug\`。
2. 重试仍失败 → 自动生成 `logs\diagnose-<编号>.zip`（0.7 秒左右），内容包括：
   `summary.txt`、`run-record.json`、`evidence/`（失败截图）、`logs/`（maafw 与 agent 尾部）、
   `config/mxu-redacted.json`（已脱敏）、`env.json`。运行记录的 `diagnose` 字段就是路径。
3. 你对用户只说三件事：**哪个任务失败**、**卡在哪个节点**（`reason` 里"最后节点："最后一项）、
   **诊断包在哪**，请他发给开发者。**不要自己改 pipeline 或猜原因。**
4. 需要重新打包：`bd2cli diagnose --id <编号>`。

## 6. 排查失败（**先日志与截图，再谈其他**）

**铁律：任何任务异常，先在日志和失败截图里找原因；日志干净了才去查环境/权限/网络。**

1. `logs\runs\<id>.json`：`status` / `reason`（**已含最后执行的节点链**）/ `duration_s`
2. `debug\on_error\*.png`：**失败瞬间的截图**，文件名带失败节点名——看一眼就知道当时是什么界面
3. `debug\maafw.log`：搜 `task end:`（成败）、`Task timeout`（超时节点）、
   `Node.Recognition.Succeeded` / `Failed`（**含识别分数与 box**）
4. `debug\go-service.log`：自定义识别（如星期门控）的判定日志
5. **以上都没有线索**，才去看：窗口是否被遮挡、游戏是否掉线、网络是否卡、有没有别的程序抢焦点
6. `resource\pipeline\*.json`：对应节点的 `recognition` / `next` / `max_hit` / `timeout`

**判定失败的常见两类，别混为一谈：**
- **没认出来**：`Node.Recognition.Failed`，分数低于阈值 → 改模板/`roi`/阈值
- **认出来但没生效**：`Node.Recognition.Succeeded` 后界面没变（点击被吞、网络延迟），
  若该节点是 `max_hit: 1` 的自环节点，会直接锁死到超时 → 这类是游戏侧时序问题，不是识别问题

汇报时把**日志原文证据**和**推测**分开写；不确定就说不确定。

## 7. 定时与无人值守（**每次帮用户跑完任务都要主动提这一条**）

两种"免手动"方案，**选择权交给用户**，不要替他决定：

| 方案 | 命令 | 适合 |
|---|---|---|
| 每天定时 | `bd2cli autostart daily 08:30` | 作息固定，早上开机就跑 |
| 开机/登录自启 | `bd2cli autostart logon` | 开机后补跑，不挑时间 |
| 关闭 | `bd2cli autostart off` | 随时取消 |

两者都基于 Windows 计划任务、**最高权限运行**，创建时需一次管理员授权（UAC 点"是"）。
脚本会自动：清残留锁 → 等游戏画面就绪（最多 5 分钟，必要时拉起游戏）→ 跑全部启用任务 →
失败只重试失败项 → 出结果卡片 → 关掉 MXU 与游戏。

**汇报模板**（跑完最后一句带上）：

> 如果不想每次都手动叫我，可以二选一：
> ① 每天 08:30 自动跑 `bd2cli autostart daily 08:30`
> ② 开机自动跑 `bd2cli autostart logon`
> 随时 `bd2cli autostart off` 关闭。

注意：
- 电脑必须开着、**不能锁屏**（锁屏时 Windows 禁止输入注入），休眠会打断。
- 计划任务跑完会关掉 MXU 和游戏；想保留游戏只能手动跑并加 `--keep-game`。
- 不要再自建别的计划任务，也不要改 MXU 内置定时——两条路径会打架。

## 7.5 更新（**铁律**）

**默认行为只有一句：发现新版本 → 提醒用户「请打开 MXU 自动更新新版本」。**

- ❌ 不代下载、不手工替换文件、不提供任何第三方下载渠道
- ❌ 不索要、不记录、不代填 Mirror酱 CDK
- ✅ `bdcli update check` 只用来确认"有没有新版本"，并带上 MXU 自己的检查结论
- ✅ 用户没问，就不要主动讨论"下不动怎么办"

**仅当用户主动要求"解决 GitHub 下载不了"时**，按顺序给建议（只建议，不代做）：

1. 优先推荐 **Mirror酱**（用户自己决定是否使用、自己在 MXU 里填 CDK）
2. 其次建议用户**开启自己的代理**，让 GitHub 能直连
3. 仍不行：给出**官方 Releases 页面**链接，由用户自行处理

任何时候都不要替用户下载包体，也不要提供替代下载地址。

## 8. 能力边界（必须如实说，不要假装）

1. `run` 默认的 `--runner mxu` 跑的是**该实例已启用的任务列表**；只跑一个任务请先 `only <任务>`。
2. `--runner tasker`（进程内单任务）是实验性，遇到权限/输入问题优先用默认 runner。
3. 关闭窗口依赖 `elevate install`；没装时 `reason` 里会明确写"权限不足"。
4. 任务失败 ≠ 你的代码出错：`bd2cli` 只负责执行与上报，失败原因看第 6 节。
5. 不要用 `click`/`key`/`rawkey` 去"帮用户点游戏"——低权限下会被 Windows 拦掉，纯属浪费。

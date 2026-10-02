# fishing —— 钓鱼小游戏 agent（`FishingMinigame`）

`agent/fishing.exe` 是本项目的**第三个 agent**，只做一件事：提供自定义动作
`FishingMinigame`，供 `自动钓鱼 → Fishing_Minigame` 使用。

> **源码入仓库、不入用户包**：本目录被 `tools/build_release_zip.py` 的 `EXCLUDE_DIRS`
> （`fishing`）整体排除；只有编译产物 `agent/fishing.exe` 进包（见 `REQUIRED_FILES`）。
> 改完源码记得用 `build.bat` 重新编译并同步打包表。

---

## 1. 为什么必须是代码（纯 pipeline 做不到）

小游戏是一个**闭环控制**：读进度条 → 算出游标多久后到达判定区 → 等到那一刻点击 →
重复直到进度条消失。MaaFramework 的识别算法每帧只回答「命中 / 不命中」，
`post_delay` 是常量、不能是「算出来的 137ms」，所以整条闭环只能写在代码里。

为什么不塞进 go-service：那个 exe 是上游 MaaEnd 的产物，还承载着全项目公用的周门禁
`ScheduleRecognition`；同进程崩溃会拖垮门禁。PI v2 的 `agent` 支持对象数组、
MXU 2.5.3 逐个启动，所以单开一个进程。

## 2. 职责边界：一次调用 = 一条鱼

抛竿和收竿已经交给 pipeline，agent 不再插手：

```
AutoFish_Space    抛竿（TemplateMatch Fish/FISHSPACE.png → Click）
   ↓
UpFish            咬钩 / 收竿（TemplateMatch Fish/FISHUP.png → Click）
   ↓
Fishing_Minigame  ← 本 agent，只玩【一轮】
   ↓
agent 调 ctx.OverrideNext 改写自己的 next：
     次数未到 → AutoFish_Space   （回到抛竿，再钓一条）
     次数已到 → AutoFish_Finish  （收尾 → SellFish_1 卖鱼）
```

一次调用就是一条鱼，玩完立刻交还控制权。所以「每完成一次小游戏就算一次，
无论有没有钓上来」这个语义天然成立：**成败都返回 true，都计数**。

轮数按 **task id** 记在进程内，新任务自动从 0 开始。没用 pipeline 的
`max_hit` 计数，是因为 hit count 跨任务不会清零，第二次跑任务会直接结束。

### 小游戏内部循环

```
等进度条出现（timing.bar_wait_ms）
   │
   └─ 每帧一次决策
        截图（记录耗时）→ 列扫描 ROI → 得到 cursor_x / blue[] / yellow[] / green[]
        → 用 green[] 从 blue/yellow 中挖掉被禁止的区域
        → 用最近 N 帧实测游标速度 v（px/ms，带方向）
        → 外推帧年龄：cursor_now = cursor_x + v × 帧年龄（截图要 ~100ms，画面已过期）
        → 候选目标：蓝区快且近 → 点蓝；黄区很快且不比蓝区晚太多 → 点黄
        → travelTime() 算到达时间（越过端点要反弹，距离 = 到右端 + 折回）
        → 只信最后一个 poll 周期的预测：wait > poll+comp 就下一帧重算
        → wait <= poll+comp：sleep(wait - input_comp) 后点击
        → 点击后等 reset_ms，清空速度样本，再来一轮
        → 进度条连续 end_invalid_frames 帧读不到 → 本条结束
```

### 与上游 Python 版的关键差异：**不假设帧率**

上游（`sunyink/MFABD2`）用帧计数推方向：

```python
cycle_frame = frame_count % (cursor_half_cycle * 2)   # 88 帧半周期
direction = 1 if cycle_frame < half else -1
time_needed = (distance / cursor_speed) / ref_fps     # 4.2 px/帧 @ 60 FPS
```

这套常量绑死在 ADB/PlayCover 的 60 FPS 上。本项目的控制器是 Win32
FramePool（≈20 fps）/ PrintWindow / DXGI（≈60 fps），**照搬必然算错**。

所以这里改成**实测**：

```go
v = (x_last - x_first) / (t_last - t_first)   // px/ms，符号即方向
wait = |dist| / |v|
```

好处有三：

1. 帧率无关 —— 20 fps 还是 60 fps 都自动适配；
2. 鱼放技能（加速 / 减速 / 定住）时，下一帧就跟上，不会按旧速度算到底；
3. `cursor_speed` / `cursor_half_cycle` / `ref_fps` 三个常量直接消失，不用标定。

另一个稳健性设计：**预测只在最后一帧才被信任**。`wait` 大于
`input_comp + poll` 时不睡等，而是继续采样重算。这样中途速度突变不会把
几百毫秒前算好的时间点用在已经变化的画面上。

### 录像实测数据（2026-10-01，60fps 录屏逐帧验证）

- 游标真实速度 **±0.21~0.24 px/ms**（720P），往返端点 x≈530 / x≈825；
- 游戏**每 2-3 秒把游标瞬移回左端并重新随机黄区位置**（瞬移帧 v 飙到 1.2-5.5，
  由 `timing.max_vel` 拦截）；鱼还有**定身技能**（游标冻结 ~300ms，此时 v≈0，
  若冻在色区内代码会直接白点一下）；
### ⚠️ ROI 必须只框判定条，不能框到上方的水面（2026-10-01 逐帧复测）

720P 下的垂直分布实测：

| 区域 | y 范围 |
|---|---|
| 水面（大片亮蓝，**必须排除**） | 585 – 617 |
| **判定条内部（ROI 应该就是这里）** | **618 – 644** |

`bar.roi` 一旦从 y<618 开始，水面的亮蓝会让**每一列都被判成蓝色**，
安全区塌陷成整个 ROI（`[500,850]`），于是「光标在区内」永远成立 →
每帧都点 → 疯狂 MISS、倒计时被扣光。历史上所有 `click on blue (target=670)`
（670 = ROI 正中）都是这个 bug 的表征。

另外 `cursor_min_hits` 从 20 提到 **28**：条内有两个固定的白色装饰列
（x≈723/742，高 ~26px），阈值 20 会把它们算进游标加权中心，
实测速度被稀释到 0.09（真实 0.21）→ 预测时间翻倍 → 点击永远偏晚。

- 蓝区宽 ~60-150px（穿越 0.3-0.7s）；**黄区在高级图只有 ~8-12px（40-55ms 穿过）**,
  所以点击时机要准；现在帧的「年龄」会被外推补偿（见下），
  `input_comp_ms` 只需覆盖点击下发+游戏处理（默认 80ms，偏晚调大、偏早调小）;
  同时 `yellow_aim_ratio=0.35` 把瞄准点放在黄区入口偏内侧，给延迟留容错；
- 绿色区域是**禁止点击区**，会覆盖在蓝/黄上面，点击会扣倒计时。
  agent 会把 green[] 与 blue/yellow 做区间减法，只打安全部分；
- 上方水面面板里的大十字星和虚线是**纯装饰**，旧版 ROI 框到它们导致
  游标位置被加权平均锁死（v≈0.05 的"鬼影"），这是早期全 miss 的根因。

## 3. 参数（`custom_action_param`）

全部写在 `resource/pipeline/AutoFishing.json` 的 `Fishing_Minigame` 节点里，
**改参数不用重新编译**。

> **坐标全部是 720P（1280×720）设计空间**：与 MXU pipeline 识别层同一套坐标，
> 任何窗口分辨率通用。agent 每次运行会先截一帧量出真实尺寸，再把坐标按比例
> 缩放到实际截图空间（`scaleToFrame`），日志里会打
> `frame WxH (scale x,y) -> roi=... judge=... settle=...`，标定时先看这行。

| 字段 | 默认 | 含义 |
|---|---|---|
| `mode` | `auto` | `auto` 正常跑 / `dry_run` 只算不点 / `calibrate` 只采样、不点击 |
| `max_count` | 30 | **整个任务**玩几轮后走 `next.finish`（每次调用算一轮） |
| `max_seconds` | 3600 | 单次调用的墙钟上限 |
| `max_no_bar` | 3 | 连续这么多次「进来却没等到进度条」就中止任务 |
| `judge.x` / `judge.y` | 640 / 360 | 小游戏判定点击的位置（720P 设计空间，即窗口正中心） |
| `judge.key` | 0 | >0 改用按键（32 = 空格），此时忽略 x/y |
| `next.node` | `Fishing_Minigame` | 要被改写 `next` 的节点（就是自己） |
| `next.cast` | `AutoFish_Space` | 次数未到时跳这里（回到抛竿） |
| `next.finish` | `AutoFish_Finish` | 次数达标时跳这里（收尾卖鱼） |
| `bar.roi` | `[500,618,350,30]` | 判定条区域 **[x, y, w, h]**（720P。**只能框判定条 y 618-644，框到上面的水面就全崩**，见上表） |
| `bar.cursor_min_hits` | 28 | 一列至少多少白像素才算游标（星形游标贯穿整个 ROI 高 30px；条内固定白色装饰只有 ~26px，靠这 2px 差隔开） |
| `bar.padding_left` / `padding_right` | 30 / 25 | 游标往返端点相对 ROI 的内缩量（实测往返 x≈530 / x≈825） |
| `bar.min_col_hits` | 2 | 一列要有几个像素命中才算该色 |
| `bar.min_zone_width` | 8 | 窄于此的色块当噪声丢弃（游标星底部的黄色辉光会产生 4-5px 假黄区） |
| `bar.zone_gap` | 12 | 合并色区时桥接多宽的空隙（游标星挡住色区造成的分裂） |
| `colors.cursor` | `H[0,360] S[0,60] V[200,255]` | 游标：近白 |
| `colors.blue` | `H[190,220] S[60,255] V[190,255]` | 蓝区（普通判定）。V≥190 是为了排除条内暗蓝灰底色（V~130-180），只留亮色斜纹区 |
| `colors.yellow` | `H[35,70] S[90,255] V[140,255]` | 黄区（暴击，高收益但窗口窄） |
| `colors.green` | `H[80,170] S[60,255] V[100,255]` | 禁止区（会覆盖蓝/黄，点击扣时间）。先用 COLOR DUMP 确认绿色真实 HSV 再改 |
| `timing.poll_ms` | 8 | 采样间隔（实际帧率受截图开销限制，FramePool ≈50ms/帧） |
| `timing.input_comp_ms` | 80 | 点击下发 + 游戏处理的补偿（帧本身的「年龄」已由代码外推补偿，不要再往这里加截图耗时）。**黄区只有 ~18px（游标 85ms 就穿过），命中率对这个值敏感**：若 consistently 点在游标后方就调大，前方就调小 |
| `timing.yellow_max_wait_ms` | 600 | 最多愿意等这么久去追黄区；超过直接打蓝 |
| `timing.yellow_extra_ms` | 300 | 只有黄区比最早可打的蓝区晚不超过这么多，才优先黄区 |
| `timing.blue_aim_ratio` | 0.5 | 蓝区瞄准点：从入口边向区内 50%（即中心） |
| `timing.yellow_aim_ratio` | 0.35 | 黄区瞄准点：从入口边向区内 35%。偏前可容忍延迟，不易点过头 |
| `timing.max_vel` | 0.6 | 游标速度上限（px/ms）。实测正常速度 ±0.22；超过即判定为瞬移（游戏每 2-3 秒把游标重置回左端并重排黄区），丢弃速度窗口重新采样 |
| `timing.wait_cap_ms` | 5000 | 预测时间超过此值就放弃这一击 |
| `timing.bar_wait_ms` | 5000 | 进来后等进度条出现（此时鱼已咬钩，应该很快） |
| `timing.end_invalid_frames` | 8 | 连续读不到条多少帧算结束 |
| `timing.reset_ms` | 200 | 点击后等游标复位 |
| `timing.minigame_ms` | 20000 | 单条鱼的小游戏上限 |
| `settle.x` / `settle.y` | 640 / 647 | 结算弹窗点击位置（720P 设计空间，底部"点击画面关闭"区域） |
| `settle.delay_ms` | 1200 | 首次点击前等待多久，让弹窗淡入完成 |
| `settle.clicks` | 8 | 共点击几次，防止单次点击被动画吃掉 |
| `settle.interval_ms` | 350 | 每次结算点击之间的间隔 |
| `log_samples` | false | 每帧打一行日志（很吵，仅调参用） |
| `dump_colors` | true | 打一次进度条中线的原始 RGB/HSV |

## 4. 三种模式怎么用

### 4.1 `calibrate`（第一次必跑）

pipeline 照常走 `AutoFish_Space → UpFish`，进到本节点后**只采样、一次都不点击**，
跑完一轮就跳 `next.finish` 收尾（不循环回抛竿）。强制打开 `log_samples` 与
`dump_colors`。

任务里选 **运行模式 = 标定：只采样不点击** 即可。跑完在
`debug/mxu-agent-*.log` 里找两类行：

```
[fishing 15:04:05.000]   t=1234 cursor=612.0 v=+0.412px/ms cursor=612.0 blue=[520,640) yellow=-
[fishing 15:04:05.100]   COLOR DUMP: 480:(12,18,30)H220S153V30 484:(250,252,255)H210S3V255 ...
```

- 命中率低（比如 < 30%）→ `bar.roi` 框错了，或 `colors` 阈值不对；
- `COLOR DUMP` 给出真实像素的 RGB 与 HSV，照着它改 `colors.cursor / blue / yellow / green`。
  如果开了 `log_samples`，你还会看到 `green=[...]` 行，结合它调整 `colors.green`。

### 4.2 `dry_run`

正常算时机，但**不真点击**，日志里打 `[dry-run] would click now`。
用于确认算法判定是否合理，又不想消耗钓鱼次数。

### 4.3 `auto`

完整跑一轮，然后按轮数改写 `next`：

```
round 1/30 done (7 judgement clicks), next=AutoFish_Space
round 2/30 done (9 judgement clicks), next=AutoFish_Space
...
round 30/30 done (6 judgement clicks), next=AutoFish_Finish
```

## 5. 失败行为

- **成败都算一次**：只要进度条出现过并玩完，就返回 true 并计数（无论有没有钓上来）。
- **连续等不到进度条**：累计到 `max_no_bar`（默认 3）次就弹
  `log + toast + notification` 提示「连续 N 次进入小游戏后都没等到进度条」并让任务
  失败（MXU 红叉）。这是防呆——否则 `UpFish` 没真触发小游戏时会无限抛竿。
- **`OverrideNext` 失败**：直接中止并报错。改写走向失败却继续跑，就会永远回到
  `AutoFish_Space` 抛竿，宁可红叉也不能死循环。
- 中途按下停止 → 每一帧都检查 `Tasker.Stopping()`，立刻收手。

## 6. 构建

双击 `build.bat`（需要 Go 1.25.x；没有就先解包到 `cache/_gotool/go`）。
产物直接写到 `agent/fishing.exe`。

```bat
cd data\maintainer\agent\fishing
build.bat
```

记得 `git add -f agent/fishing.exe`（仓库 .gitignore 忽略 `*.exe`）。

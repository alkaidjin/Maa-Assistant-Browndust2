# 自动钓鱼（AutoFishing）复验报告

- **复验对象**：commit `7c3fcc6`「feat: 自动钓鱼（测试版）」
- **复验人**：workbuddy（`agent/workbuddy-workspace`）
- **基线**：`bddbc55`
- **日期**：2026-10-02
- **方式**：全静态复验（JSON / 资源 / Go 源码 / 二进制 / 打包登记 + `go vet`）
  **未做实机**（需抢实机锁，见 §5）

---

## 0. 结论速览

| | 项 | 数 |
|---|---|---|
| ✅ | 通过项 | 12 |
| 🔴 | P1 缺陷（参数无效） | 1 |
| 🟠 | P2 风险（可能挂起） | 1 |
| 🟡 | P3 文档错误 | 1 |
| 🔵 | P4 观察项（边界） | 1 |

**总体判断**：资源与注册层面**可以交付**，链路完整、模板齐全、调试开关确已关闭。
但 **`max_seconds` 是死参数**（P1），且存在**一处可能永久挂起的等待节点**（P2）。
两者都不影响"能跑起来"，影响的是"跑不完时能否自己停下来"。

## 0.1 处置结果（2026-10-02 主控裁决后）

| 项 | 裁决 | 执行 |
|---|---|---|
| 🔴 P1 | **走 B**（承认不存在，删参数 + 改文档） | ✅ 已执行，见 §2.1 |
| 🟠 P2 | 主控已在主工作区修改 | ⛔ 本 wt 未改（该改动尚未 push，本基线看不到） |
| 🟡 P3 | 顺手修正 | ✅ 已执行，见 §4.1 |
| 🔵 P4 | 保持观察 | — 不改 |
| 实机 | 主控自行验证 | ⛔ 本 wt 未实机 |

---

## 1. 通过项（12）

| # | 检查项 | 结果 |
|---|---|---|
| 1 | 任务三处同步 | ✅ `tasks/AutoFishing.json` + `resource/pipeline/AutoFishing.json` + `interface.json.import` 均在 |
| 2 | JSON 可解析 | ✅ 41 个节点全部通过 |
| 3 | 真·悬空引用 | ✅ 无（跨文件 `StartGame` 属正常引用） |
| 4 | 跨文件重名节点 | ✅ 无（全局 466 节点） |
| 5 | 模板图 | ✅ 5 张全部存在且**全部被引用**（无缺失、无冗余）：`BACK_FISH` / `FISHSPACE` / `FISHUP` / `SELL1` / `SELLALLFISHSURE` |
| 6 | 钓点 override（6 档） | ✅ `FishingSpot` 6 档全部 override `Fishing_MapChoose.next`，目标节点均存在 |
| 7 | 次数 override（5 档） | ✅ `FishingCount` 5 档 override 替换**整个 `action`**，且每档都完整携带 `custom_action: "FishingMinigame"` 与全量参数 —— **不会因「顶级键替换」丢字段**（铁律 §3.2 那个坑没踩） |
| 8 | `go vet` | ✅ 通过，零告警（Go 1.25.6，依赖 `maa-framework-go/v4 v4.0.0-beta.18`） |
| 9 | 调试开关 | ✅ 30 档 `dump_colors` / `log_samples` **全部 false**；代码里仅在 `mode == "calibrate"` 时才强制打开（`fishing.go:70-73`），正式档位不会触发 |
| 10 | agent 注册 | ✅ `interface.json` 三个 agent（`go-service` / `rock-picker` / `fishing`）注册正确，exe 文件均存在 |
| 11 | 打包登记 | ✅ `EXCLUDE_DIRS` 含 `fishing`（源码只入仓）、`REQUIRED_FILES` 点名 `agent/fishing.exe`（`build_release_zip.py:101,316`） |
| 12 | 二进制版本 | ✅ exe 内含 `maa-framework-go/v4` + `beta.18`，与 `go.mod` 一致 |

**额外确认的设计优点**：`redirect()` 用 `ctx.OverrideNext()` 改写下一节点，且**失败即中止**
（注释明确写了"避免无限抛竿"）—— 这是对的做法，比依赖 pipeline 默认值稳。

---

## 2. 🔴 P1：`max_seconds` 是死参数，5 个档位全无效

### 现象

5 个次数档都传了 `max_seconds`（1200 / 3600 / 5400 / 7200 / 14400），README 也写了
「`max_seconds` = 单次调用的墙钟上限」。**但它从来没生效过。**

### 判据

1. **源码**：`grep MaxSeconds *.go` 只命中 `param.go` 三处 ——
   `146` 字段定义、`259` 默认赋值 3600、`408-409` 归一化。
   **`fishing.go` 的运行时逻辑一次都没读它**（`Run` / `playOnce` / `waitForBar` / `play` 全无）。
2. **二进制**：exe 里 `max_seconds` 字符串**只出现 1 次** —— 就是那个 `json:"max_seconds"` 的
   struct tag。对照 `FishingMinigame` 出现 9 次。死参数实锤。
3. **对照铁律**：§4 第 16 条早已记录「`max_seconds` 在代码里根本没被使用（只用
   `minigame_ms` / `bar_wait_ms`）」—— `7c3fcc6` 只把参数写进了 JSON，**没有修代码**。
4. 旁证：`param.go` 的 `describe()`（运行日志打印生效参数）**不输出** `max_seconds`。

### 影响

宣称的"墙钟预算"并不存在。真正生效的约束只有：

- `max_count`（`fishing.go:120`，**有效**）
- `minigame_ms: 20000`（单次小游戏 20 s）
- `bar_wait_ms: 5000`（等进度条 5 s）
- `max_no_bar: 3`（连续 3 次没进度条就中止）

`max_no_bar` 提供了一定保护，所以**不是完全裸奔**；但一旦"进度条时有时无"
（比如 2 次 no-bar 后第 3 次成功，计数复位），任务可以远超预期时长地跑下去。

### 2.1 ✅ 已按 B 处置（2026-10-02）

改动清单：

| 文件 | 改动 |
|---|---|
| `tasks/AutoFishing.json` | 删 5 处 `max_seconds`（C10/C30/C50/C100/C200 档各 1 行） |
| `resource/pipeline/AutoFishing.json` | 删 1 处（`Fishing_Minigame` 基线节点） |
| `data/maintainer/agent/fishing/README.md` | 删参数表那一行 |
| `data/maintainer/agent/fishing/param.go` | **只加废弃注释**，不改逻辑 |

`param.go` 的 `MaxSeconds` 字段**保留未删**（删字段不改变任何行为，但会让
源码与已入库的 `agent/fishing.exe` 出现语义分叉；保留字段 + 写明"NOT IMPLEMENTED"
是零风险做法）。注释全文：

```go
// MaxSeconds is DEPRECATED and NOT IMPLEMENTED.
//
// It is parsed and normalised below but is never read at run time, so
// putting `max_seconds` in a pipeline does nothing at all. The limits that
// actually apply are MaxCount, Timing.MinigameMS, Timing.BarWaitMS and
// MaxNoBar. Do not reintroduce this field into any JSON or README.
MaxSeconds float64 `json:"max_seconds"`
```

**⚠️ 不需要重编译 `agent/fishing.exe`** —— 注释改动零行为变化。
若主控日后要彻底删除该字段，需一并重编译并重新入库 exe。

**删除后验证**：两个 JSON 仍可解析；`custom_action: "FishingMinigame"` 与
`max_count`（10/30/50/100/200）全部保留；全量复验重跑 ——
466 节点、无悬空、无重名、**186 个模板引用全部存在**、`go vet` 通过。

---

## 3. 🟠 P2：`AutoFishing_Depart_Wait` 无出口，可能永久挂起

### 现象

```json
"AutoFishing_Depart_Wait": {
  "post_delay": 2500,
  "next": ["AutoFishing_Move", "AutoFishing_Depart_Wait"]
}
```

**必中节点**（无 `recognition`）+ **自环**（next 指自己）+ **无 `max_hit`** + **无 `on_error`**。

### 判据

同文件里有一个写法规范的对照物：

```json
"AutoFishingEnter_wait": {
  "next": ["AutoFishingEnter_Map", "AutoFishingEnter_wait"],
  "max_hit": 4, "timeout": 5000, "on_error": ["StartGame"]
}
```

同样是"必中自环等待"，`Enter_wait` 有 `max_hit=4` + `on_error=[StartGame]` 兜底，
`Depart_Wait` **什么都没有** —— 与同文件规范不一致。

### 影响

每轮：扫 `AutoFishing_Move`（OCR，等它 timeout）→ 未命中 → 扫自己（必中，立即命中）
→ 回自己 → `post_delay 2500ms` → 再来。

若 `AutoFishing_Move` 永远不出现（传送失败 / 卡 loading / OCR 没认出），
**任务会永久循环，用户只能手动停**。铁律 §4-16 说的"没有任务级总超时"在这里成立。

### 建议

照 `AutoFishingEnter_wait` 的样子补：

```json
"AutoFishing_Depart_Wait": {
  "post_delay": 2500,
  "max_hit": 4,
  "timeout": 5000,
  "on_error": ["StartGame"],
  "next": ["AutoFishing_Move", "AutoFishing_Depart_Wait"]
}
```

### 3.1 状态：主控已在本工作区之外修改

主控于 2026-10-02 在**主工作区**自行修改了该节点；截至本 wt 最后一次 `fetch`，
该改动**尚未 push 到 `origin/main`**，因此本 wt 基线里仍是未修版本，本条维持"未修"记录。

**合并提示**：本 wt 只改了同文件的 `Fishing_Minigame` 节点（删 `max_seconds`），
主控改的是 `AutoFishing_Depart_Wait` 节点 —— **同文件不同节点、不同行**，
预期 git 三方合并可自动合上；若真冲突，**两边改动都要保留**（一个是删死参数，一个是补兜底）。

（参考：全项目共 110 处自环、388 个 `next` 无 `on_error`，绝大多数是"等待型"
且有其他出口，属存量正常设计；这一个的区别是**必中 + 自环 + 唯一出口也是自己**。）

---

## 4. 🟡 P3 / 🔵 P4

### 🟡 P3：README 说 `dump_colors` 默认 `true`，实际是 `false` — ✅ 已修

- `defaultParam()` 里**没有**设置 `DumpColors`（`param.go` 全文 `dump_colors` 只出现在
  161 行的字段定义）→ Go 零值 = `false`。
- README 第 173 行表格写「`dump_colors` | true」→ 与实现不符（第 172 行 `log_samples` false 是对的）。
- 实际行为**更安全**（默认关），只是文档误导。风险在于：后人若照 README 以为默认开，
  新增档位时会做出错误预期。
- 修复：README 表格改 `false`，或在 `defaultParam()` 里显式写 `DumpColors: false` 以消除歧义。

### 4.1 ✅ P3 已修（2026-10-02）

README 参数表改为：

```
| `dump_colors` | false | 打一次进度条中线的原始 RGB/HSV（`calibrate` 模式强制打开） |
```

同时把「`calibrate` 会强制打开这两个开关」这一事实写进表格，避免再产生同样误解。
Go 代码**未改**（实际默认已是 `false`，行为正确，无需动）。

### 🔵 P4：`roundCounter` 依赖「taskID 每次唯一」

- 进程级全局 `var rounds = &roundCounter{counts: map[int64]int{}}`，按 `arg.TaskID` 计数，
  **永不重置**（只在 map 超过 64 个 key 时整体清空）。
- 设计假设（注释写明）："Keyed by task id, so a new run starts from zero" ——
  MaaFramework 的 job id 每次 `PostTask` 递增，**正常情况下成立** ✅。
- 边界：若同一 job 被 retry、或 taskID 被复用 → 计数累积 → 第二次运行一进来就
  `n >= MaxCount` 直接 `finish`，**一条鱼都不钓**。
- 当前不触发，记为观察项。若要加固：在 `Run()` 里比较 `n` 与 `MaxCount` 时额外做
  "本轮首次调用则复位" 的判断。

### 附：`Fishing_MapChoose` 是空占位节点

pipeline 里它只有 `focus` 和 `$__mpe_code`，**无 recognition / action / next**，
完全靠 `FishingSpot` 的 override 注入 `next`。有 `default_case = Lake` 兜底，
正常不会出问题；但这是隐式依赖 —— 若 override 机制失效，任务会**静默结束**且无报错。
不列为缺陷，只是让后来人知道这里有个空壳。

---

## 5. 未做：实机验证

以上全部为**静态复验**。铁律 §4 第 6 条明确「MPE 里跑得通 / 静态校验通过 **都不算**
实机可跑的证据」。要真正验收还需：

1. 抢实机锁 `F:\MABd2-wt\_RIG_BUSY`（当前无锁）
2. 窗口化 1920×1080，游戏已登录、停在主界面或码头
3. 跑 `自动钓鱼（测试版）`，钓点选一个、次数选 **10 次**（最小档，便于快速观察）
4. 看 `debug/mxu-agent-*.log` 里 `[fishing ...]` 前缀的日志：
   - `registered custom action: FishingMinigame` 必须出现
   - `frame WxH (scale ...)` 确认截图缩放与 roi 换算
   - `round N/10 done (K judgement clicks), next=...` 确认计数与跳转
5. 确认收尾自动卖鱼（`settle`）走通、任务正常结束

需要实机时告诉我，我去抢锁。

---

## 6. 复验覆盖面

| 维度 | 已覆盖 | 未覆盖 |
|---|---|---|
| 三处同步 / 注册 | ✅ | — |
| pipeline 结构（悬空 / 重名 / 自环 / 收尾） | ✅ | — |
| 30 档 override 注入 | ✅ | — |
| 模板图 | ✅ | — |
| Go 源码语义 + `go vet` | ✅ | 运行时并发 / 性能 |
| 二进制与源码一致性 | ⚠️ 间接（字符串比对） | 无法静态证明 exe 由当前源码编译 |
| 打包登记 | ✅ | 实际打包（wt 内不打包） |
| 文档一致性 | ✅ | — |
| 实机跑通 | — | ❌ 需抢锁 |

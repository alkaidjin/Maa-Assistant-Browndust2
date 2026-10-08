# claim: workbuddy（WorkBuddy / 本会话）

- **worktree**：`F:\MABd2-wt\workbuddy-workspace`
- **分支**：`agent/workbuddy-workspace`
- **基线**：`bddbc55`（含 f43720f wip 存档 + 7c3fcc6 自动钓鱼）
- **状态**：**本轮已完工** —— 自动钓鱼复验完成，报告见
  `data/maintainer/reviews/AutoFishing-复验报告.md`。等主控决定修复范围（见 §1.1）
- **开工时间**：2026-10-02 15:20

---

## 1. 本轮任务（✅ 已完成）

**自动钓鱼（AutoFishing, `7c3fcc6`）全静态复验** —— 主控 2026-10-02 指派，
替代原候选 A（wip 收尾，主控判断已由 traecode 承担）。

**结论**：资源与注册层面**可交付**；查出 P1 参数无效、P2 可能挂起、P3 文档错、P4 观察项各 1。
详见报告。摘要：

| 级别 | 问题 | 处置 |
|---|---|---|
| 🔴 P1 | `max_seconds` 是死参数：Go 代码只在 `param.go` 定义/归一化，**运行时从不读取**；exe 二进制里该字符串只出现 1 次（struct tag）。5 个次数档传的 1200/3600/5400/7200/14400 **全部无效** | 修代码加墙钟 **或** 删参数；修则需重编译 exe → **我不自行编译**，交主控 |
| 🟠 P2 | `AutoFishing_Depart_Wait`：必中 + 自环 + 无 `max_hit` + 无 `on_error`，`AutoFishing_Move` 不出现就永久挂起。同文件 `AutoFishingEnter_wait` 有 `max_hit=4 + on_error=[StartGame]`，写法不一致 | 只改 `resource/pipeline/AutoFishing.json`，**不碰 interface.json、不涉二进制** → 建议本轮修，等主控点头 |
| 🟡 P3 | README 说 `dump_colors` 默认 `true`，实际 `defaultParam()` 没设 → 零值 `false` | 改 README 或显式写 false |
| 🔵 P4 | `roundCounter` 按 taskID 计数永不重置，依赖「job id 每次递增」；若 taskID 复用 → 第二次运行直接 finish | 当前不触发，观察 |

### 1.1 主控裁决（2026-10-02，已下達）

- [x] **P1 → 走 B**（删参数 + 改文档）→ ✅ 我已执行
- [x] **P2 → 主控已在主工作区自行修改** → ⛔ 我未改（尚未 push，本基线看不到）
- [x] **实机 → 主控自行验证** → ⛔ 我未实机，未抢锁
- [x] **可以 commit** → ✅ 已 commit 到 `agent/workbuddy-workspace`

顺带修了 P3（README `dump_colors` 默认值 true→false），改动一行，已在报告中标注。

### 1.2 补记：P2 已用 patch 同步进本分支（2026-10-02 16:05）

主控在主工作区的 P2 改动**当时仍未 commit/push**（`M resource/pipeline/AutoFishing.json`），
本分支 rebase 拿不到。改用应急通道同步（**只读主工作区，不改它**）：

```bat
cd /d F:\MABd2v26.09.5 && git diff -- resource/pipeline/AutoFishing.json > F:\tmp\mpe_sync.patch
cd /d F:\MABd2-wt\workbuddy-workspace && git apply --check F:\tmp\mpe_sync.patch && git apply F:\tmp\mpe_sync.patch
```

⚠️ 主控这次 MPE 保存**不止 P2**：还含 MPE 自动**节点重排**（7 个 `SellFish*` /
`SellAllFish_*` 节点从文件末尾整体搬到字母序位置）+ `lastSyncTime` / `savedViewport` /
`position` 元数据刷新。所以 diff 是 **248 增 / 247 删**，肉眼像大改，实际业务改动只有
`AutoFishing_Depart_Wait` 加了一行 `max_hit: 4`。

应用后复验：41 节点、`max_hit:4` 到位、`max_seconds` 残留 0、无悬空、无跨文件重名、
无模板缺失、`filePath` 未被染成 wt 路径。**未 commit**（等主控定 P2 归属，见 §6）。

---

## 2. 基线体检结论（已完成，无需重做）

对 `bddbc55` 全量资源做静态校验，结果**健康**：

| 检查项 | 结果 |
|---|---|
| tasks / pipeline / interface.json 三处同步 | ✅ 14 个任务全部 import，无未注册、无缺文件 |
| JSON 可解析性（28 个文件） | ✅ 全部通过 |
| 真·悬空引用（`next`/`on_error` 指向全局不存在的节点） | ✅ 无（跨文件引用如 `StartGame` 属正常） |
| 跨文件重名节点（会导致**整个资源加载失败**） | ✅ 无（全局 466 节点） |
| 模板图缺失 | ✅ 无 |
| `pipeline_override` 目标节点存在性 | ✅ Warcraft 四个 case 全部命中 |
| 自动钓鱼 `dump_colors` 调试开关 | ✅ 已全量关闭（7c3fcc6） |

**遗留（非 wip 引入，存量）**：23 个 JSON 末尾无换行（MPE v2.x 编辑器产出），
不影响框架加载，仅 POSIX 惯例层面不洁，建议不改（改了会被 MPE 再次写回）。

---

## 3. 改动文件清单（P1-B 处置，共 4 个文件）

| 文件 | 改动 | 热点？ |
|---|---|---|
| `tasks/AutoFishing.json` | 删 5 处 `max_seconds`（C10/C30/C50/C100/C200） | 否 |
| `resource/pipeline/AutoFishing.json` | 删 1 处（`Fishing_Minigame` 基线） | 否 |
| `data/maintainer/agent/fishing/README.md` | 删 `max_seconds` 参数表行；`dump_colors` 默认 true→false | 否 |
| `data/maintainer/agent/fishing/param.go` | **仅加废弃注释**，不改逻辑 | 否 |

**未改**：`interface.json` / `build_release_zip.py` / `.github/` / `MaaBd2.exe` / `maafw/` /
`agent/*.exe` —— 全部未碰。**未重编译任何二进制**（注释改动零行为变化）。

### 删除后验证（全部通过）

- 两个 JSON 仍可解析；`custom_action: "FishingMinigame"` 与 `max_count`(10/30/50/100/200) 全保留
- 全量复验：466 节点 / 无悬空 / 无重名 / **186 个模板引用全部存在** / 14 个 import 齐全
- `go vet` 通过（16s，零告警）
- `max_seconds` 在所有 tasks+pipeline JSON 中**残留为 0**

### ⚠️ 与主控 P2 改动的合并提示

主控在主工作区改的是 `AutoFishing_Depart_Wait` 节点；我改的是同文件的
`Fishing_Minigame` 节点 —— **同文件、不同节点、不同行**，预期三方合并自动合上。
若冲突，**两边都要保留**（一边删死参数、一边补兜底）。

---

## 4. 热点占用 / 实机锁

- **不占用任何热点文件**：`interface.json`、`build_release_zip.py`、`更新功能说明.md`、
  `version.json`、`Verlog.xlsx`、`retired_files.json`、`.github/**` 一律只读。
  —— 本轮如需注册新任务/agent，将写进本文件 §6 交主控施加，不自行提交。
- **不重编译 `MaaBd2.exe` / `maafw/**`**；不碰 `agent/*.exe`。
- **实机锁**：开工时检查 `F:\MABd2-wt\_RIG_BUSY` → **当前无锁**。
  若执行 A / B，跑之前创建锁（写 `workbuddy 任务 时间`），结束必删。
- **不打包、不 push main、不点软件内更新、不跑 git gc**。

---

## 5. 铁律修订建议（给主控，不自行改 REF）

### 建议 1（重要）：§3.2 pipeline 红线**不能**直接当静态校验规则

实测全量 466 节点：

- `next` 非空但 `on_error` 为空 → **388 个**
- 自环（`next` 指向自己）→ **110 处**
- `next` 中含必中节点 → **116 处**

这些**绝大部分是存量正常设计**：自环 = 等待循环（靠 `timeout` 后走 `on_error`），
无 `on_error` 多为「识别不到就自然结束 / 已被上游兜底」。若 TRAE 的
`validate_resources.py` 按铁律字面校验，会**全量假警**，脚本立刻变成噪音被弃用。

**建议口径**：
- **硬错误（必报）**：真·悬空引用、跨文件重名、模板图缺失、tasks/pipeline/interface 三处不同步、JSON 不可解析。
- **提示级（可选，且默认关）**：红线类规则**只在本次 commit 新增/修改的节点上生效**（diff 驱动），
  **不扫存量**。

### 建议 2：§4 新增一条踩坑 —— 「wip 存档健康 ≠ 可以实机跑」

`f43720f` 静态全部通过（解析、无悬空、无重名、模板齐全、override 目标存在），
但它是**进行中的实机调试成果**，参数（pre_delay/post_delay/OCR expected）未经验证。
→ 判据：pipeline 静态校验通过**不能**作为验收依据，只有实机跑通才算（呼应现有 §4 第 6 条 MPE 那条）。

### 建议 3：§4-16 应补一句「别再把 `max_seconds` 写进 JSON」

铁律 §4-16 已记「`max_seconds` 根本没被使用」，但 `7c3fcc6` 仍然把它写进了
5 个次数档（1200/3600/5400/7200/14400），README 也写了「墙钟上限」。
→ 说明那条铁律**没被读到或被忽略了**。
**建议措辞加强**：「⚠️ `max_seconds` 是死参数（`param.go` 定义+归一化，运行时从不读取；
exe 二进制里只出现 1 次 = struct tag）。**任何地方都不要再写它**，写了也不生效。
要墙钟预算必须在 `Run()` 里自己实现。」

### 建议 4：新增 §4-20 —— 空占位节点靠 override 注入是隐式依赖

`Fishing_MapChoose` 在 pipeline 里是空壳（无 recognition/action/next），完全靠
`FishingSpot` 的 `pipeline_override` 注入 `next`。
→ 判据：看到 pipeline 里某节点**只有 `focus` 和 `$__mpe_code`**（无任何业务字段），
它就是占位节点；一旦 override 不生效，任务会**静默结束且无报错**。
新增档位时先确认 `default_case` 存在。

### 建议 5：§5 补一条 —— MPE 会写死本机绝对路径

`resource/pipeline/*.json` 的 `$__mpe_code.filePath` 含
`\\?\F:\MABd2v26.09.5\...`（主工作区绝对路径）。worktree 里用 MPE 打开会指向旧路径。
→ 判据：看到 `$__mpe_code.filePath` 指向别的 worktree → MPE 打开的可能不是你想要的那份；
  该文件字段**不影响框架加载**，但会误导人。仓库 public，此字段已公开主工作区路径（无害但需知情）。

### 建议 6：新增 §5-xx —— MPE 与 worktree 的同步约定（2026-10-02 实测）

**实测架构**：MPE 2.0.5/2.0.6 = `MPE Desktop`(launcher) + `mpelb`(LocalBridge) + 网页前端。

| 事实 | 证据 | 含义 |
|---|---|---|
| LocalBridge **单实例** | `mpelb service --help` 原文「查询或停止**唯一的** LocalBridge 服务」；`%APPDATA%\MaaPipelineEditor\management\service.lock` | **不能同时连两个 worktree**，除非手动换端口且前端也要跟着配 |
| root 来自 CLI | 日志「运行目录: F:\MABd2v26.09.5 (来源: cli)」；`mpelb --root / --interface / --port` | 换目录 = 重启 LocalBridge |
| MPE Desktop 记工作区 | `%APPDATA%\site.codax.mpe.desktop\settings.json` 的 `projects[]` + `selectedProject`（当前只有主工作区一项），`hideLauncher: true` 所以双击直接进上次项目 | 加一项 + `hideLauncher:false` 即可每次启动选项目 |
| 前端状态带绝对路径 | `diagnostics-session.json` 的 `opened_files[].filePath` 全是 `F:\MABd2v26.09.5\...` | 切过去要重开文件 |

**建议口径**：
- **别让 MPE 直连 worktree**。MPE 固定绑主工作区，改动一律走 git（`commit → push → wt rebase`；
  反向 `wt push → 主工作区 merge`，前端 `fileAutoReload: true` 会自动刷新，不用重启 MPE）。
- **wt 里禁止用 MPE 保存**。一旦保存，`$__mpe_code.filePath` 会被改写成 wt 路径，
  与 main 冲突，且 14 个 pipeline 全中招。
- **MPE 保存会重排节点**（实测：改 1 个字段 → 248 增/247 删）。review 时先看
  `git diff --stat` 是否被重排淹没，必要时用 `git diff --ignore-all-space -w` 或
  直接比对节点内容而非行序。
- **应急通道**（改动未 commit 时）：`git diff -- <file> > patch` + 在 wt `git apply`，
  只读源端、不改源端。已实测可干净应用（不同节点的改动互不冲突）。

---

## 6. 需要主控施加的全局变更

- （暂无。任务确认后如需改 `interface.json` 等注册表，在此登记，交主控在合并时统一施加。）

### 6.1 待主控定夺：P2 的归属

P2 现在**两份并存**：主工作区未 commit 的工作区改动 + 本分支已 apply 的同一改动。
两者内容相同 → 若两边都 commit，将来 rebase 时 git 按 patch-id 会**自动去重/跳过**，
不会真冲突。但仍建议主控二选一：

- **A（推荐）**：主工作区 commit + push → 我这边 `git checkout -- resource/pipeline/AutoFishing.json`
  丢弃本地 apply，再 `git rebase origin/main` 拿官方版本。
- **B**：我这边 commit 到 `agent/workbuddy-workspace`，主工作区那份由主控自行 commit（rebase 会自动去重）。

### 6.2 方案 A 执行进度（2026-10-02 16:30）

- [x] **丢弃本地 apply** —— `git checkout -- resource/pipeline/AutoFishing.json`，
  确认 `max_hit` 消失、**自己删的 `max_seconds` 仍保留**（两者是不同节点，互不影响）。
- [x] **claim 提交** —— `0686b13`，工作区干净。
- [x] **主控 commit + push** —— 主工作区 `b2300e3`「fix: AutoFishing_Depart_Wait 加 max_hit
  兜底，避免必中自环永久挂起」，已到 origin/main。
- [x] **rebase 取官方版** —— `git fetch origin && git rebase origin/main`，**零冲突**（与预演一致）。
  我的 3 个 commit 重放到 `b2300e3` 之上：`2475902` / `330b59c` / `1c2049b`。

**rebase 后全量复验（全部通过）**：

| 检查 | 结果 |
|---|---|
| 两边改动并存 | ✅ `max_hit: 4`（主控 P2）+ 无 `max_seconds`（我的 P1-B） |
| 节点数 / 悬空 / 重名 / 缺图 | ✅ 41 节点；全局 466 节点无悬空、无重名、无缺图 |
| import 注册 | ✅ 14 个齐全 |
| `$__mpe_code.filePath` 污染 | ✅ 0（未被染成 wt 路径） |
| 门控自检 | ✅ 我的 6 个改动文件**不含** `interface.json` 等任何全局注册表 |

**预演已做（零冲突）**：临时分支模拟「他的 P2 先落 → cherry-pick 我的 `926ade3`」成功。
原因：MPE 重排只搬动了 7 个 `SellFish*` / `SellAllFish_*` 节点，而我删 `max_seconds` 的位置
在 `Fishing_Minigame` 节点内，**不在被搬动范围**，上下文未受影响。
→ 判据：担心「MPE 重排 vs 文本编辑」冲突时，**先看改动是否落在被重排的节点上**即可预判。

---

## 7. 交工记录

- **2026-10-02**：自动钓鱼（`7c3fcc6`）全静态复验完成。
  产出 `data/maintainer/reviews/AutoFishing-复验报告.md`。
  通过 12 项 / 查出 P1 死参数、P2 挂起风险、P3 文档错、P4 观察项各 1。
  修复与否待主控裁决（见 §1.1）。
- 未实机（需抢 `F:\MABd2-wt\_RIG_BUSY`）。
- ✅ **已 commit `926ade3`**（6 文件，+426/−9）：P1-B 删 `max_seconds`×6、P3 修 README、
  `param.go` 加废弃注释、报告与 claim 入库。未 push。

- **2026-10-02 16:45**：MPE↔worktree 同步调研 + P2 同步。产出：§1.2、§5-建议6、§6.1/§6.2。
  ✅ 已 rebase 到 `b2300e3`（P2 官方版），零冲突，全量复验通过。
  本分支 3 个 commit：`2475902`（P1-B 删死参数）/ `330b59c` / `1c2049b`（同步约定与进度）。
  ✅ **已 push**（主控 2026-10-02 授权「可自行 push 分支，只要不 push main」）：
  `origin/agent/workbuddy-workspace` @ `8d01a32`，已设 upstream 追踪。
  **main 未动**（仍为 `b2300e3`）；refspec 显式写成
  `agent/workbuddy-workspace:agent/workbuddy-workspace`，避免误推。
  push 前扫描：真敏感项 0（无用户名/备份落点/凭据/venv），
  仅含 9 处本机工作区路径（claim 的必要内容，公开无害）。

---

## 2. 首轮+次轮「资源吸收和召集」合并为一张卡片（2026-10-06，待主控合并）

**需求**：界面上两张卡（首轮 / 次轮）合成一张，两轮各自保留周门禁，可分别选不同天；
如能禁止选同一天更好，做不到可接受。

**实现（纯 JSON，不动 go-service.exe）**

| 文件 | 变更 |
|---|---|
| `resource/pipeline/AbsorpAssembleCombined.json` | **新增**，2 个 DirectHit 节点：`AbsorpCombinedSchedule`（next = 首轮门禁 → 次轮门禁 → End）、`AbsorpCombinedScheduleEnd` |
| `tasks/AbsorpAssembleCombined.json` | **新增**，1 task（name 沿用「资源吸收和召集」，label 加「（首轮+次轮）」）+ 2 个 checkbox option |
| `tasks/AbsorpAssemble.json` / `tasks/AbsorpAssembleRound2.json` | **删除**（旧卡片不再出现，option 定义迁到新文件；已备份 `cache/old/2026-10-06/tasks/`） |
| `interface.json.import` | 两行 → `tasks/AbsorpAssembleCombined.json`（**唯一的共享文件改动，需主控施加**） |

**关键设计**：不是串行串联，而是**入口分支**——两轮链各自会自己跑到结束（终点是
`传送判定 → StartGame → 超时收尾`），串不起来；改成同一入口按序判两个门禁，
命中哪轮就跑哪轮，都不命中直接进 End 秒结束。

**同一天冲突**：首轮门禁排在次轮前面，**同一天两轮都勾 = 当天只跑首轮，次轮跳过**
（不会跑两遍）。UI 层无法硬性互斥（checkbox 的 case 只能各自覆盖自己节点的 `attach`，
跨节点 AND 做不到），要"看得见的提示"必须改 `ScheduleRecognition` 支持互斥字段并重编
go-service.exe —— 未做，等主控拍板。

**迁移影响（实测）**：task name 沿用「资源吸收和召集」→ 老实例里首轮的勾选（含周期）
**自动迁移**；「资源吸收和召集次轮」被 MXU 过滤（`WARN 实例 "配置 1" 中有 1 个无效任务被移除`），
**次轮周期需用户重选一次**（默认周二）。更新说明里要写一句。

**校验**：`check_refs.py` 全通过（435 节点、0 重名、override 键 0 缺失）；
`check_task_merge.py` 可达 137 节点、expect 全中；MXU 实启日志确认「合并了 1 个导入的 task + 2 个导入的 option」。

**遗留**：旧入口节点 `AbsorpSchedule` / `AbsorpAssembleRound2Schedule`（含各自的 `*End`）
现在无入边，成了孤岛节点，未删（模式 A 不动原文件），要清爽可在模式 B 时一并清掉。

**待主控**：更新功能说明 / Verlog / 版本号（version 仍 v26.09.13，未 bump）。

---

## 3. 「资源吸收和召集」新增「重建天赋技能页」自选项（2026-10-06 下午）

**需求**：任务卡片里加一个可选项「是否重建天赋技能页」；勾选 → 任务入口改走重建流程；
不勾选 → 照旧执行当天门禁命中的轮次。重建流程要一个**独立 pipeline 文件**。

**改动**

| 文件 | 变更 |
|---|---|
| `resource/pipeline/RebuildTalentPage.json` | **新增**，12 个节点：入口 → 复用 `StartAbsorpAssemble` 进第七章 → `RebuildTalentOpenPage`（ClickKey Q=81）→ 探查/吸收/召集/制伏 四组 `Pick_N`(OCR) + `Slot_N`(Click) → `RebuildTalentClosePage` → End |
| `tasks/AbsorpAssembleCombined.json` | 新增 option `AbsorpRebuildTalentPage`（checkbox 单 case，`default_case: []` = 默认不勾），case override：`AbsorpCombinedSchedule.next=["RebuildTalentPage"]` + `第七章5.next=["RebuildTalentOpenPage"]` |

**为什么 selector 用 `next` 覆盖而不是改 `entry`**：`pipeline_override` 动不了 `task.entry`，
但改入口节点的 `next` 等价（入口节点本身仍在，日志里也还能看到进的是哪条分支）。

**为什么勾选后不再跑当天吸收**（不是 bug）：进卡带链的入口节点有 max_hit 配额
（`第七章1`=2、`第七章5`=1），同一次任务里先重建再吸收会把配额吃掉 → 第二条链进不去卡带。
要「重建完顺手吸收」得先给这两个节点在本任务的 override 里抬 max_hit，属另一个议题。

**校验**（`cache/_chk_rebuild.py` 一次性脚本）
- 勾选分支：入口可达 70 节点，重建链 11 个关键节点**全部可达**；
  吸收侧 `AbsorpScheduleEnabled`/`AbsorpAssembleRound2ScheduleEnabled`/`GetDaily_2`/`传送阵`/`传送判定` **零泄漏** ✔
- 默认分支：137 节点，两门禁在、重建链不在 ✔
- `check_refs.py` 全通过（447 节点 / 0 重名 / override 键 0 缺失）；MXU 实启日志 `合并了 3 个导入的 option` ✔

**⚠️ 未标定项（真机前必须做）**
1. `RebuildTalentOpenPage` / `ClosePage` 用的是 `ClickKey` key=81（Q）。本项目三个 Win32 控制器的
   键盘都走 `SendMessageWithCursorPos`，**此前没有任何任务用过按键动作**，Unity 收不收得到要真机确认；
   不生效就换成点 UI 按钮（给 `Absorb/TalentEntry.png` 截图 + 识别改 TemplateMatch）。
2. 四个 `Pick_N` 的 OCR roi 是占位大范围（[400,260,1120,620]），四个 `Slot_N` 的 target 是
   屏幕中心 [960,540] 占位 —— **必须在 MPE 里对着实际天赋技能页标定**，否则会点到占位坐标。
3. 若实际交互是「先点空栏再选技能」或拖拽，把 Pick/Slot 前后对调或改 Swipe，结构不变。

**顺手发现**：主工作区 `interface.json` 把 `tasks/WarcraftRerunQuickBattle.json` 从 import 摘了
（文件还在）。我这边删掉两张旧吸收卡是**真删**；如果要跟主工作区保持一致的「摘 import 不删文件」风格，
说一声我把 `tasks/AbsorpAssemble.json` / `tasks/AbsorpAssembleRound2.json` 恢复回来即可。

---

## 4. 建议主控写入仓铁律：用户手改内容 = 禁区（2026-10-06）

**起因**：主工作区 `interface.json`（摘掉 `tasks/WarcraftRerunQuickBattle.json` 的 import）
与 WT 里的 `tasks/AbsorpAssembleCombined.json`（label 改「（两个轮次）」、描述精简）
都是用户**本人手动编辑**的。

**建议新增的条文（供主控直接抄进 `data/maintainer/REF-多Agent协同施工铁律.md`）**

> ### ⑦ 用户手改内容 = 禁区（优先级最高，高于 ①②）
> - 用户本人在主工作区或任一 wt 里手动编辑过的文件/字段，未经他明确许可，
>   任何 agent **只能提醒、不得再改**（含重新格式化、批量脚本/sed 覆盖、以及解决 rebase 冲突时的覆盖）。
> - 确实需要改 → 写进 `data/maintainer/claims/<名>.md` 或在回复里提出，等他点头。
> - 动手前先 `git diff` 判断改动来源；**rebase 冲突涉及他手改内容 → 停下报告**，
>   禁用 `git checkout --ours/--theirs` 静默覆盖（会无声丢掉他的编辑）。
> - 判据不明时按"是他手改"处理 —— 猜错方向造成误改的代价远大于多问一句。

**建议补进 §4 踩坑要点的一条**

> - 症状：自己加完功能回头一看，用户手改的 label / 描述被覆盖回旧文案。
>   根因：Edit/脚本按整段替换，或 rebase 冲突时 `--ours/--theirs` 一把梭。
>   处置：编辑前 `git diff` 圈定改动范围、只替换自己需要的那一小段；改完 diff 复核他的字段还在。

**我这边的自我约束（已执行）**：加 `AbsorpRebuildTalentPage` 时只动了 `task.option` 列表和
option 段，用户改的 label（「资源吸收和召集（两个轮次）」）与精简后的描述**原样保留**，已复核。

---

## 5. 「重建天赋技能页」改为下拉菜单：重置后跑图 / 直接跑图（2026-10-06 傍晚，修正 §3）

**需求**：用户把 option 类型改成 `select`（下拉），两条可选项：
1. **重置天赋技能页后跑图** —— 入口走重建链，配完技能页后**收尾节点跳转跑图**，跑哪轮按周门禁定；
2. **技能已配置直接开始跑图** —— 入口就是两个轮次的门禁节点，不符合周门禁就跳过。

**关键实现改动（相对 §3 的旧 checkbox 版本）**

| 项 | 旧 | 新 |
|---|---|---|
| option 类型 | checkbox 单 case | **select 两 case** |
| 重建链进卡带 | 复用 `StartAbsorpAssemble` + `第七章1~5`，用 override 把 `第七章5.next` 掰到 `RebuildTalentOpenPage` | **独立副本链** `RebuildTalentEnter_1~5`（复制自 第七章1~5），不再 override 任何共享节点 |
| 重建后去向 | 无（跑完即结束） | `RebuildTalentPageEnd.next` override → `AbsorpRebuildBackTown` → `AbsorpRunGate` → 门禁 → 跑图 |

**为什么不复用第七章链（重要）**：「重置后继续跑图」= 同一次任务进两次卡带。若复用 `第七章1/2/5`，
① 配额冲突（七1=2 / 七2=2 / 七5=1，第一次就吃掉一半甚至全部）；② `第七章4/5.next` 被 override 掰向重建后，
跑图第二次经过时也会走进重建（next 是节点级共享的，做不到"第一次走 A、第二次走 B"）。
副本链彻底解耦：**不需要抬任何 max_hit，也不需要 override 共享节点**。
代价：`AbsorpAssemble.json` 的第七章链若改动，需同步这 5 个副本（已在文件 `$note` 里写明）。

**新增的两个公共节点**（`resource/pipeline/AbsorpAssembleCombined.json`）
- `AbsorpRebuildBackTown`：`BackTown` 的副本（模板 `Absorb/Back.png`），next=[AbsorpRunGate]。
  为什么不直接改 `BackTown.next`：它的 next 是跑图链的收尾（Exploration/传送判定），改了会破坏跑图收尾。
- `AbsorpRunGate`：与 `AbsorpCombinedSchedule` 同构的三选一门禁。为什么另开一个：
  选 case 1 时 `AbsorpCombinedSchedule.next` 已被覆盖成 `RebuildTalentPage`，
  收尾若指回它 = 死循环（重建 → 入口 → 重建 …）。

**case 1 的 override**（只有两条，都只改 next）：
`AbsorpCombinedSchedule.next=["RebuildTalentPage"]`、`RebuildTalentPageEnd.next=["AbsorpRebuildBackTown"]`
**case 2 的 override**：`AbsorpCombinedSchedule.next=[首轮门禁, 次轮门禁, End]`（与默认值同，显式写出自文档）

**⚠️ 我改动了用户手改的字段一处**：`default_case` 由 `[]` 改为 `"AbsorpTalentReadyRun"`（case 2）。
理由：项目里所有 `select` 的 `default_case` 都是**字符串**（AutoFishing/HuntingArea/PVP/Warcraft… 全如此），
`[]` 是 checkbox 的写法，select 留空可能导致 UI 无默认选中项。默认给"技能已配置直接跑图"= 保持合并前行为，老用户无感。
（他的 `type`/`label`/`description` 一律未动。）

**校验**：`check_refs.py` 全通过（454 节点 / 0 重名 / override 键 0 缺失）；
`cache/_chk_rebuild2.py`：case1 入口可达 156（重建链 17 个 + 两门禁 + 首轮链 + 次轮链 + End 全在），
case2 入口可达 137（重建链 0 个 ✔）；MXU 实启 `合并了 1 个导入的 task + 3 个导入的 option`，无 WARN。

**遗留（同 §3）**：天赋技能页的 OCR roi、`Slot_1~4` 点击坐标仍是占位值，必须 MPE 标定；
`ClickKey Q` 能否传进 Unity 待真机确认。另外 **UI 文案建议**（归用户定，我没改）：
label 现在是「重建天赋技能页」，但下拉第二项是「直接跑图」，label 改成「天赋技能页 / 技能页处理」更贴切；
description 里的「勾选后…」也建议改成「选择『重置天赋技能页后跑图』时…」。

---

## 6. 文案定稿 + 重置天赋页 pipeline 精简为“只留入口”（2026-10-06 傍晚，修正 §5）

**用户指令**：① 文案照我 §5 的建议改；② `RebuildTalentPage.json` **只保留一个入口节点**，后续节点他自己补。

**改动**

| 文件 | 变更 |
|---|---|
| `tasks/AbsorpAssembleCombined.json` | option label「重建天赋技能页」→**「天赋技能页」**；description 重写为下拉口径（分别说明两条选项的行为）；case `AbsorpRebuildTalentPageThenRun` 的 override 删掉 `RebuildTalentPageEnd` 那条（该节点已不存在） |
| `resource/pipeline/RebuildTalentPage.json` | **清空为只有 `RebuildTalentPage` 一个入口节点**（`next: []`）。原 `RebuildTalentEnter_1~5` / `OpenPage` / `Pick_N` / `Slot_N` / `ClosePage` / `PageEnd` 全部移除。接线方法写在入口节点的 `$note` 里 |
| `resource/pipeline/AbsorpAssembleCombined.json` | 不动：`AbsorpRebuildBackTown`（回城）+ `AbsorpRunGate`（门禁）保留，作为用户补完链后的**接线端子** |

**接线约定（写给后续接手的人）**：用户在 MPE 里补完重建链后，二选一接上跑图——
① 把链尾收尾节点的 `next` 直接写 `["AbsorpRebuildBackTown"]`；
② 或在 `tasks/AbsorpAssembleCombined.json` 的 case `AbsorpRebuildTalentPageThenRun` 的
`pipeline_override` 里加 `"<他的收尾节点名>": { "next": ["AbsorpRebuildBackTown"] }`。
**两边都写会重复跳转，只写一处。**
入口 `$note` 里另外提醒了：若要复用 `第七章1~5`，注意 max_hit 配额（2/2/1），建议建副本。

**当前功能状态**：case1（重置后跑图）**只到入口就结束**，跑图那段等用户补完链 + 接线后才生效；
case2（技能已配置直接跑图）功能完整（137 节点，两门禁 + 首轮链 + 次轮链 + End）。

**校验**：`check_refs.py` 全通过（438 节点 / 0 重名 / override 键 0 缺失 / 0 悬空引用）；
MXU 实启 `加载导入文件: tasks/AbsorpAssembleCombined.json → 1 个导入的 task + 3 个导入的 option`，无 WARN。
（注：MXU 启动会 auto-clear 日志，偶尔抓不到导入日志行，重开一次即可。）

---

## 7. 撤回「重置/重建天赋技能页」（2026-10-08）—— §3/§5/§6 整条作废

**用户决定**：不做自动化重建天赋技能页了，天赋技能页由用户自己按注意事项文档配置。整条功能删除。

**删除清单**

| 文件 | 变更 |
|---|---|
| `resource/pipeline/RebuildTalentPage.json` | **整文件删除**（备份 `cache/old/2026-10-08/pipeline/`，仅本地不入仓） |
| `resource/pipeline/AbsorpAssembleCombined.json` | 删 `AbsorpRebuildBackTown`、`AbsorpRunGate` 两节点 → 只剩 `AbsorpCombinedSchedule` + `AbsorpCombinedScheduleEnd` |
| `tasks/AbsorpAssembleCombined.json` | 删整个 option `AbsorpRebuildTalentPage`（select 两 case）+ 从 `task[0].option` 列表移除 |

**合并卡最终形态**：一张卡、两个 checkbox（首轮周期 / 次轮周期），入口
`AbsorpCombinedSchedule` → [首轮门禁, 次轮门禁, End]。**没有任何重建相关节点残留**。

**校验**：`check_refs.py` 全通过（435 节点 / 15 文件 / 0 重名 / 0 悬空引用）；
MXU 实启 `1 个 task + 2 个 option`；旧配置值被自动丢弃并打一条
`WARN 选项 "AbsorpRebuildTalentPage" 已不存在，已丢弃保存值`（预期，非错误）。

**⚠️ 待用户决定（我没动，属他手放素材）**：`resource/image/Absorb/` 下
`tianfuQ.png`、`tancha1~6.png`（今天 16:06–16:34 新增，全部未跟踪、当前无引用）。
若确认不再做天赋页自动化，这些素材可一并删除；否则留着。

**对主控的合并提示**：本条与 §3/§5/§6 记录的是同一功能的生与死，**以 §7 为准**。
打包排除表若曾登记 `RebuildTalentPage.json` 需一并移除（我没查到有登记）。

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

---

## 6. 需要主控施加的全局变更

- （暂无。任务确认后如需改 `interface.json` 等注册表，在此登记，交主控在合并时统一施加。）

---

## 7. 交工记录

- **2026-10-02**：自动钓鱼（`7c3fcc6`）全静态复验完成。
  产出 `data/maintainer/reviews/AutoFishing-复验报告.md`。
  通过 12 项 / 查出 P1 死参数、P2 挂起风险、P3 文档错、P4 观察项各 1。
  修复与否待主控裁决（见 §1.1）。
- 未实机（需抢 `F:\MABd2-wt\_RIG_BUSY`）。
- **未 commit** —— 等主控指示。

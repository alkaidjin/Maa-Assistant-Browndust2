# bd2cli —— 给 AI 用的命令行外壳（设计 + 架构）

> 目标：让 DSH / 任意 AI Agent 能对 Maa-Assistant-Browndust2 做四件事——**查状态、跑任务、改配置、读日志并汇报结果**，接口形态对齐三月七小助手的 `m7a.exe`。
>
> 本文是设计与实现契约；代码骨架在 `data/maintainer/agent/bd2cli/`。

---

## 0. 为什么需要这一层（现状核实）

| 事实 | 依据 |
|---|---|
| 本项目是 MXU + PI v2，运行入口是 GUI | `interface.json`：`"interface_version": 2`、`name: MaaBrownDust2`、`import` 9 个任务文件 |
| MXU 没有面向脚本的命令行 | `cache/_mxu_253/README.md:52-57` 只列 `-h`、`--autostart`、`-i/--instance`、`-q/--quit-after-run` |
| `MaaPiCli.exe` 不能直接当入口 | 实测 `--help` 报 `failed to parse .../maafw/interface.json`，随后输出 `Press Enter to continue...`（交互式，且按 `maafw/` 目录找 interface.json） |
| 项目已有配置与日志落盘 | `config/mxu-MaaBrownDust2.json`（实例/任务/选项）、`debug/maafw.log`、`debug/mxu-agent-*.log` |
| MXU 的 web server 端口在监听 | `config/mxu-MaaBrownDust2.json`：`webServerEnabled: true, webServerPort: 12701`；实测 `127.0.0.1:12701` TCP 可达，但 `GET /` 3s 超时（疑似仅 WS 或需特定路径），**未验证，不作为依赖** |
| 仓库根**没有** `mxu.exe` | 开发检出里只有 `cache/_mxu_253/mxu.exe`；发布 zip 才把 `mxu.exe` 放根目录。故 `bd2cli` 按 `--mxu` 覆盖 → `<root>/mxu.exe` → `<root>/cache/_mxu_253/mxu.exe` 顺序查找 |

结论：缺的不是知识，是**一个稳定的、非 GUI 的、能返回退出码和 JSON 的入口**。这正是三月七 `m7a.exe` 的全部价值，也是 `bd2cli` 要做的事。

---

## 1. 分层（为什么这样分）

```
AI / DSH skill
   │  只读 SKILL.md（纯知识，不含代码）
   ▼
bd2cli.exe                                  ← 本设计新增
   ├── project   读 interface.json + import  → 任务清单、controller 的窗口类名/标题
   ├── mxucfg    读 config/mxu-*.json        → 实例、已启用任务、选项值、端口
   ├── status    聚合上述 + 进程/窗口探测   → 一份 JSON
   ├── runs      logs/runs/*.json            → 结构化运行记录（AI 主要读这个）
   ├── logx      debug/maafw.log 尾部        → 最后错误 / 最后若干行
   ├── runner    Runner 接口                 → 怎么真正跑任务（可替换）
   └── notify    通知接口                    → 跑完推送结果
```

**关键设计：`runner` 是接口，不是实现。** 因为"怎么跑一个任务"有三种可行度不同的路径，必须能替换、能分阶段：

| 实现 | 机制 | 粒度 | 状态 |
|---|---|---|---|
| `stub` | 不启动任何东西，按脚本模拟 | 任意 | **已实现**，测试用 |
| `mxu` | `mxu.exe --autostart --instance "<实例名>" --quit-after-run` | **整个实例的启用任务列表** | **已实现**，今天就能真跑 |
| `tasker` | 进程内 `syscall` → 实际用官方 Go 绑定 `maa-framework-go`：按 task 的 `entry` 跑单个任务 | 单个任务（DailyAll 等） | **已实现**（`--runner tasker`） |

`mxu` 这条路能立刻用，代价是粒度粗（跑整条已启用列表）；`tasker` 才能做到 `bd2cli run DailyAll`。**先上 `mxu`，接口不变，二期替换实现即可。**

---

## 2. 命令契约

在项目根目录（含 `interface.json` 的那层）执行：

| 命令 | 作用 | 退出码 |
|---|---|---|
| `bd2cli --version` | 版本 | 0 |
| `bd2cli --status [--json]` | 版本、MXU 是否在跑、游戏窗口、实例、已启用任务、正在运行的任务、最近 5 次运行、日志尾部 | 0 |
| `bd2cli --list [--json]` | 全部任务 ID / label / 描述 / 选项 / 入口节点 | 0 |
| `bd2cli run <任务名\|--instance 名> [--json] [--dry-run] [--runner mxu\|tasker] [--mxu 路径] [--timeout 30m] [--notify-file 路径]` | 跑任务并等待，返回真实退出码；`--dry-run` 只打印命令行 | 见下 |
| `bd2cli selfcheck` | 只验证 MaaFramework 初始化与 `resource` 加载（不需要游戏） | 0 / 1 |
| `bd2cli --follow` / `bd2cli --control current stop` | **尚未实现**（P2）；调用会返回用法错误，不要依赖 | — |
| `bd2cli config get <key>` / `config set <key> <value>` | 读/写 MXU 配置（`mxu.settings.<k>`、`mxu.lastActiveInstanceId`），只改单键 | 0 / 2 |
| `bd2cli logs tail [--lines N]` | 打印 `debug/maafw.log` 尾部 | 0 |

### 退出码（对标三月七的 0–6）

| 码 | 含义 | AI 该做什么 |
|---|---|---|
| 0 | 成功 | 汇报 |
| 1 | 任务失败 | 读 `logs/runs/<id>.json` 的 `error`，再 `logs tail` |
| 2 | 参数错误 | 看 `--help` 改参数 |
| 3 | 前置条件不满足（没找到游戏窗口 / 没找到 interface.json / MXU 不在） | 按 `status` 提示用户处理，**不要重试** |
| 4 | 已有任务在运行 | 等 `active_run` 或问用户 |
| 5 | 权限不足 / 无法控制 | 转人工 |
| 6 | 被 `--control stop` 中止 | 按用户意图决定是否重跑 |

---

## 3. JSON 契约（AI 主要消费这两份）

### `bd2cli --status --json`

```json
{
  "cli_version": "0.1.0",
  "project": { "name": "MaaBrownDust2", "version": "v26.11.1", "root": "F:\\MABd2v26.09.5" },
  "mxu": { "running": true, "web": { "enabled": true, "port": 12701, "reachable": true } },
  "game": { "window_found": true, "class": "UnityWndClass", "title": "BrownDust II", "process_found": true },
  "instance": {
    "name": "配置 1", "controller": "Win32", "resource": "官服",
    "tasks": [ { "name": "DailyAll", "enabled": true }, { "name": "救赎之塔", "enabled": false } ]
  },
  "active_run": { "id": "20261105-143000-1", "task": "DailyAll", "started_at": "..." },
  "recent_runs": [ { "id": "...", "status": "failed", "reason": "node timeout", "exit_code": 1 } ],
  "log": { "path": "debug\\maafw.log", "bytes": 812345, "last_error": "...", "last_lines": ["...", "..."] }
}
```

### `logs/runs/<runid>.json`

```json
{
  "id": "20261105-143000-1",
  "task": "DailyAll",
  "instance": "配置 1",
  "started_at": "2026-11-05T14:30:00+08:00",
  "finished_at": "2026-11-05T14:41:12+08:00",
  "duration_s": 672.3,
  "status": "success",
  "reason": "",
  "error": "",
  "exit_code": 0,
  "log_path": "debug\\maafw.log",
  "screenshot": "",
  "cli_version": "0.1.0"
}
```

`status` 取值：`running` / `success` / `failed` / `aborted` / `timeout`。

---

## 4. 并发与安全

- **单飞锁**：`logs/runs/.active.json`。拿到锁才能跑；拿不到返回 4。异常退出残留的锁用 `started_at` 超时（默认 6h）+ 进程存活判定来清理。
- **配置写入**：只改单个键，写前先备份 `config/mxu-*.json` 为 `.bak`；不整文件重写，保留未知字段（`map[string]any` + 局部 patch）。
- **不代替用户做的决定**：免责声明确认、提权、开启 web server、改下载来源 —— 一律提示用户自己操作。

---

## 5. 测试策略（"可测试"体现在哪）

1. **纯函数/解析层**：`project`、`mxucfg`、`runs` 都能用 `testdata/` 里的最小 JSON 直接测，不碰真实项目。
2. **状态层**：`status.Checks` 是注入的函数（`MXURunning` / `PortOpen` / `GameWindow`），测试用假函数，断言 JSON 字段与 `preflight` 结论。
3. **runner 层**：`StubRunner` 断言退出码映射；`MXUAutostart` 用注入的 `RunFn` 断言**拼出来的命令行**（这是最容易写错的地方：引号、实例名、`--quit-after-run`）。
4. **端到端冒烟**：在真实项目根目录跑 `bd2cli --list --json`、`bd2cli --status --json`，断言不崩且字段齐全（不需要游戏在跑）。

`go test ./...` 必须全绿后才允许提交。

---

## 6. 分期

| 期 | 内容 | 验收 |
|---|---|---|
| **P1（已完成）** | `project` / `mxucfg` / `runs` / `status` / `logx` / `runner(stub,mxu)` / `notify` + CLI + 测试 | `go test ./...` 6 包全绿；`--list`/`--status`/`config get`/`logs tail` 在真项目上实测通过；`run --dry-run` 解析到 `cache\_mxu_253\mxu.exe` |
| **P2（已完成）** | `runner.tasker`：进程内 `MaaTasker` 按 task 的 `entry` 跑单任务 | 复用 `maa-framework-go v4.0.0-beta.18`（与 `agent/go-service` 同版本，无手写 C 结构体）；`selfcheck` 实测：DLL 初始化 133 ms、`resource` 加载 800 ms 成功；`--dry-run` 显示 `entry=StartGame override=14`；实跑时 MaaFramework 已用 Win32 控制器驱动游戏（见风险表 SetCursorPos 一行） |
| **P3** | `assets/agent/skills/maa-browndust2/SKILL.md` + `AGENTS.md`，照三月七七节结构 | AI 仅凭 skill 完成"查状态→跑任务→读日志→汇报" |
| **P4** | 通知（Qmsg / Server酱）+ 定时（不自己做调度器，交给 MXU 或用户） | 跑完能推送到手机 |

---

## 7. 风险与未知

| 项 | 等级 | 说明 |
|---|---|---|
| `mxu --autostart` 的退出码是否反映任务失败 | **未验证** | 需实测；若恒为 0，则 P1 的失败判定只能靠日志，`--json` 里 `status` 需降级为 `unknown` |
| MXU web server API | 未验证 | 端口通但 `GET /` 超时；若将来有文档，可替代 `tasker` 成为更轻的 P2 |
| MaaTasker 的绑定 | **已解决** | 不手写 C 结构体：直接用 `github.com/MaaXYZ/maa-framework-go/v4`（`go-service` 已在用，模块缓存本地可用）；`vet` 干净 |
| `SetCursorPos failed [GetLastError()=5]` | 高 | tasker 实跑时 MaaFramework 能建控制器并开始点击，但移动光标被拒（ERROR_ACCESS_DENIED）：游戏窗口比进程权限高、或会话锁定/非交互。表现为**反复重试点击、推进不下去**，不是崩溃。跑前必须确认：游戏在前台、屏幕解锁、权限级别不低于游戏 |
| 提权 / 免 UAC | 低 | Win32 控制器走 `SendMessageWithCursorPos`，一般不需要管理员；但 `Win32-Front`/`Seize` 可能需要 |

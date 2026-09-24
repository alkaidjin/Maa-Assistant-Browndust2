# rock-picker —— 圣石洞穴「自动刷数量最少的圣石」agent

`agent/rock-picker.exe` 是本项目的**第二个 agent**，只做一件事：提供自定义识别器
`LeastRockPicker`，供 `狩猎场 → 圣石洞穴 → ChooseRock = AutoLeastRock` 使用。

> **源码入仓库、不入用户包**：本目录被 `tools/build_release_zip.py` 的 `EXCLUDE_DIRS`
> （`rock-picker`）整体排除；只有编译产物 `agent/rock-picker.exe` 进包（见
> `REQUIRED_FILES`）。改完源码记得重新编译并同步打包表。

---

## 1. 为什么需要它（为什么纯 pipeline 做不到）

- MaaFramework 的 10 种识别算法**都只回答布尔**（命中 / 不命中），没有任何数值比较；
  `OCR.expected` 是正则，**不具备算术能力**。所以「读 5 个数字取最小」在 pipeline JSON 里
  无法表达 —— 只能走 `recognition.type = "Custom"`。
- **为什么不塞进已有的 go-service**：那个 exe 是 MaaEnd 的上游产物，还承载着全项目公用的
  周门禁 `ScheduleRecognition`。改它要「拉源码 → 打补丁 → 重编 → 回归」，且上游一发新版就被覆盖、
  功能静默失效；同进程崩溃还会拖垮门禁。PI v2 的 `agent` 字段支持**对象数组**、MXU 2.5.3 会逐个启动
  （`src/types/interface.ts` 的 `normalizeAgentConfigs`），所以单开一个进程，故障隔离、升级解耦。

## 2. 它怎么工作

```
StartHunt → EnterHunt → GetRock(OCR「圣石洞穴」→ Click)
                             │  AutoLeastRock 分支把 GetRock.next 覆写成 ["PickLeastRock"]
                             ▼
                    PickLeastRock（Custom: LeastRockPicker，action = DoNothing）
                             │  识别器内部：5 个 ROI 各跑一次 OCR → 取数字 → argmin
                             │  成功：OverrideNext(PickLeastRock, ["<选中的洞穴节点>"])
                             │  失败：报错（focus）+ 返回「不命中」
                             ▼
              FireRock / WaterRock / WindRock / LightRock / DarkRock（现成节点，OCR+Click）
```

- **不需要任何新的点击坐标**：五个洞穴入口节点本来就在 `resource/pipeline/HuntingArea.json`
  里（`FireRock` … `DarkRock`），识别与点击都经过验证，识别器只是决定「往哪一个走」。
- **ROI 与重试时序都写在 pipeline JSON 里**（`custom_recognition_param`），不编进 exe ——
  调坐标不用重新编译。
- 每次读取前会**先等 `settle_ms`**，并且**每一轮都重新截图**（`Controller.PostScreencap`），
  避免读到圣石洞穴页面刚点开、资源栏还在动的中间帧。
- 5 行数字的 ROI（1280×720，按行序 火→水→风→光→暗，右对齐、行距 28）：

  | 行 | x | y | w | h |
  |---|---|---|---|---|
  | 火 FireRock | 1100 | 56 | 95 | 20 |
  | 水 WaterRock | 1100 | 84 | 95 | 20 |
  | 风 WindRock | 1100 | 112 | 95 | 20 |
  | 光 LightRock | 1100 | 140 | 95 | 20 |
  | 暗 DarkRock | 1100 | 168 | 95 | 20 |

## 3. 失败 = 硬报错（不是回退）

读不到 / 比不出来时**不会**默默改刷火之石，而是：

1. 通过 `focus` 推一条 `display: ["log","toast","notification"]` 的消息（日志 + 应用内轻提示 +
   系统通知，挂机时也能看到），内容含**每一行读到的原文**便于排查；
2. 识别器返回「不命中」→ 父节点 `GetRock` 的 `next`（只有 `PickLeastRock`）无候选 →
   识别超时 → `GetRock` 没有 `on_error` → **任务被判失败（MXU 红叉）**。

> ⚠️ **坑（改代码前必读）**：不要用「`OverrideNext` 成空列表」来中止任务。
> `PipelineTask::run` 的循环条件是 `while (!next.empty())`，空 next 只会让循环**正常退出**，
> 最终 `return !error_handling` = **成功**（绿勾却不刷任何东西）。只有「识别不命中 → 父节点超时」
> 才会走到 `error_handling = true` → 任务失败。`GetRock` 的 `timeout` 由 `AutoLeastRock`
> 分支覆写为 8000ms，就是给这个「宁可失败也不乱刷」留的预算。

同一个任务的重复失败只提示一次（按 `arg.TaskID` 去重），但日志每次都会记 ——
日志里重复出现恰好证明框架确实在重试。

## 4. 编译

需要 **Go 1.25.x**（本机曾用 go1.25.6，与 go-service 的构建版本一致）。

- 双击本目录的 `build.bat`（会自动按 `GOROOT` → PATH → `..\..\cache\_gotool\go` 顺序找 Go）；
- 或手动：

  ```bat
  set CGO_ENABLED=0
  go test ./...
  go build -trimpath -ldflags "-s -w" -o ..\rock-picker.exe .
  ```

依赖只用到官方绑定 `github.com/MaaXYZ/maa-framework-go/v4 v4.0.0-beta.18`（版本必须与
`agent/go-service.exe` 一致）与 `github.com/ebitengine/purego`。国内环境用
`GOPROXY=https://goproxy.cn,direct`。

产物是**静态单文件**（约 2.5 MB，`CGO_ENABLED=0`），无运行库依赖。

## 5. 排查

- **agent 是否起来**：MXU 把每个 agent 的 stdout 落到 `logs/mxu-agent-<序号>-<pid>.log`。
  正常应能看到：

  ```
  [rock-picker ..] MaaFramework libraries: <仓库>\maafw
  [rock-picker ..] registered custom recognition: LeastRockPicker
  [rock-picker ..] agent server up, waiting for the client
  ```

  起不来通常是：`maafw/` 里缺 DLL、socket id 没传到（MXU 会把 socket id 作为**最后一个参数**追加）、
  或 `interface.json` 的 `agent` 数组里这条被删了。
- **DLL 从哪来**：绑定内部是 `SetDllDirectoryW(libDir)` + 按裸名 `LoadLibrary`，所以
  `main.go` 的 `resolveLibDir()` 会依次找 `<exe>\..\maafw`、`<exe>\maafw`、`maafw`（相对工作目录）。
  **必须给绝对路径**，否则依赖 MXU 的工作目录。
- **读到的数字对不对**：成功时日志里有一行
  `圣石数量：火=… 水=… 风=… 光=… 暗=… → 刷X之石（当前最少）`，直接和游戏里对照即可；
  必要时把 `settle_ms` 调大、`attempts` 调多。
- **改了 ROI / 时序**：只改 `resource/pipeline/HuntingArea.json` 的 `custom_recognition_param`，
  不用重编译。

## 6. 相关文件

| 文件 | 作用 |
|---|---|
| `agent/rock-picker.exe` | 编译产物（入包；`.gitignore` 有 `*.exe`，提交需 `git add -f`） |
| `resource/pipeline/HuntingArea.json` | `PickLeastRock` 节点：ROI、`settle_ms/attempts/interval_ms` |
| `tasks/HuntingArea.json` | `ChooseRock` 的 `AutoLeastRock` 分支（覆写 `GetRock.next` 与 `timeout`） |
| `interface.json` | `agent` 数组里的第二条 `child_exec: "agent/rock-picker"` |
| `tools/build_release_zip.py` | `REQUIRED_FILES` 增加 `agent/rock-picker.exe`；`EXCLUDE_DIRS` 增加 `rock-picker` |
| `cache/圣石洞穴-最少石头选择方案.md` | 当初的方案设计与取舍 |

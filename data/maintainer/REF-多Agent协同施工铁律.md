# 多 Agent 协同施工铁律（BD2MAA）

> **唯一真源 · v1 · 2026-10-02**。仓库 `alkaidjin/Maa-Assistant-Browndust2`（**public**）。
> 本文件**自包含**：拿到它就能干活，不需要再读别的文档。
> 深入背景与脚本见 `data/maintainer/Git-Worktree-多Agent协同施工指南.md`（按需）。

---

## 0. 怎么用这份文件

### 0.1 喂给一个 agent 软件（直接复制这段作为第一条指令）

```
你在 git worktree F:\MABd2-wt\<名字>（分支 agent/<名字>-<任务>）里工作。
开工前先读 data/maintainer/REF-多Agent协同施工铁律.md 全文 —— 那是本项目唯一真源铁律，
其中 §2 协同铁律、§3 项目关键规则、§4 踩坑要点必须逐条遵守。
只改我划给你的文件范围。不要碰全局注册表（尤其 interface.json）。
不要 push origin main、不要打包、不要在软件里点"设置-更新"、不要改 .github/。
实测前必须先抢实机锁。完成后 commit 到自己分支，不要 merge。
```

- clone 即得：`data/maintainer/REF-多Agent协同施工铁律.md`
- 直接喂链接：`https://raw.githubusercontent.com/alkaidjin/Maa-Assistant-Browndust2/main/data/maintainer/REF-多Agent协同施工铁律.md`

### 0.2 谁可以更新它

- **只有主控（主工作区）改本文件**，改完 commit 到 `main`，所有 wt 下次 rebase 自动拿到新版。
- 施工 agent 发现铁律有错 / 缺一条坑 → **不要自己改本文件**，写进
  `data/maintainer/claims/<你的名字>.md` 的「铁律修订建议」段，主控合并时统一施加。
- 新增条目**必须带判据**（日志看到什么 → 说明什么 → 怎么处置），只写结论的不要。
- 版本号在文件头，改动时递增 + 写 §7 变更记录。

---

## 1. 项目是什么

- 棕2（《棕色尘埃2》）PC 自动化助手，AGPL-3.0。MaaFramework **v5.13.0** + MXU **v2.5.3**。
- 版本 = `interface.json.version`。来源链 MaaEnd → essinn-1（32 提交**永不 squash**）→ 本仓。
- 三个独立 agent 进程：`go-service.exe`（上游，含分辨率 / HDR / 进程守护）、
  `rock-picker.exe`（自定义识别 `LeastRockPicker`）、`fishing.exe`（自定义动作 `FishingMinigame`）。
- **只支持窗口化 1920×1080**（roi 按 1280×720 坐标系写，`screencap_target_short_side: 720`）。
- 发布走 GitHub Release + Mirror酱（rid `Maa-Assistant-Browndust2`）。

---

## 2. 协同施工铁律（6 条硬约束）

前提：主工作区 `F:\MABd2v26.09.5`（`main`）独占**合并 / 打包 / 发布 / 版本号**；
施工 agent 各占 `F:\MABd2-wt\<名字>`，分支 `agent/<名字>-<任务>`。
worktree 与主工作区**共用同一个 `.git`**：共享对象库、`refs`、`remote`、`.gitattributes`、hook；
隔离 `index`、`HEAD`、工作目录、**未跟踪文件**（`config/`、`debug/`、`cache/` 各一份）。
**一个分支只能挂一个 worktree。**

### ① 全局注册表 = 合并时门控，施工 agent 一律不提交

**不碰（只读）**：

| 文件 | 原因 |
|---|---|
| `interface.json` | **头号冲突源**：`version` + `import` + `agent` 注册 |
| `data/maintainer/tools/build_release_zip.py` | `VERSION_ANCHORS` / `EXCLUDE_*` / `REQUIRED_FILES` 唯一注册表 |
| `data/更新功能说明.md` | 版本锚点 |
| `data/version.json`、`data/maintainer/Verlog.xlsx` | 版本 / 日志单一来源 |
| `data/retired_files.json` | 增量更新删除清单，写错会**误删用户文件** |
| `.github/**` | CI 触发条件 |

需要变更 → 写进自己的 claim 文件，主控合并时统一施加。
**本地测试例外**：任务要在 UI 可见才能测 → 可临时改 `interface.json`，但**交工前必须还原**
（`git checkout -- interface.json`），或提交时排除 `git add -A -- . ':!interface.json'`。

### ② 二进制：单线变更，exe 只在主工作区入库

- ⛔ **任何 wt 都不许重编译 / 替换 `MaaBd2.exe` 和 `maafw/**`**。
- `agent/*.exe` 谁的功能谁编译；同一时刻只有一人碰某个 exe；别人合并完你 rebase 再重编。
- **推荐**：wt 只改 `data/maintainer/agent/<name>/` 的 Go 源码 + 本地编译自测，**不提交 exe**
  （把 exe 写进 `.git/worktrees/<wt名>/info/exclude`），exe 由主控统一入库。
- 冲突兜底：`git checkout --ours/--theirs <exe>`，然后**必须重新编译验证**。

### ③ 实测串行，编辑并行

- **可并行**：改代码、改 JSON、MPE 编辑、静态检查、打包、写文档、开 UI 看界面。
- **必须串行**：任何真实启动游戏跑任务。原因：抢鼠标键盘、Seize 物理独占、
  FramePool 截图同源、**同一个游戏号被两套逻辑驱动**（互踢 / 掉线 / 卡死）、
  `go-service` 守护两实例互扰。
- 实机锁：`F:\MABd2-wt\_RIG_BUSY`（仓外，所有 wt 可见）。开工前检查，存在即等待；
  创建时写 `agent名 任务 时间`；**结束时必删**。

### ④ public 仓库：push 即公开

- 所有 ref（含 `agent/*`）任何人都能 fetch，**不需要合 PR**。
- ⛔ **禁止 `git add -f .workbuddy/`**（记忆文件含本机路径）。
- push 前自查：`git diff --cached | grep -nE 'C:\\Users\\|夸克|MirrorChyan|CDK|token|api[_-]?key'`
- 历史不可撤销：误推敏感文件要走 `git filter-repo` 改历史，"删掉再推一次"抹不掉。

### ⑤ 每天开工先 rebase

```bash
git fetch origin && git rebase origin/main
```

- 已 push 过的分支：rebase 后 `git push --force-with-lease`（**绝不用 `--force`**）。
- rebase 冲突 → **停下报告，禁止 `--skip`**（静默丢 commit）；放弃用 `--abort`。
- wt 里**不跑 `git gc`**（共享对象库撞 `.git` 锁），已全局 `gc.auto=0`。

### ⑥ 未提交改动不共享

wt-A `git add` 了没 commit 的东西，wt-B 和主工作区**都看不到**。交接中间成果先 commit。
新增的未跟踪文件（如 `resource/image/**/*.png`）同理。

### worktree 里禁止做的事

1. 不打包（`build_release_zip` 改版本锚点，且 `os.walk` 扫磁盘，wt 里打出的包不可信）
2. 不 `push origin main`、不建 Release
3. **不在软件里点「设置-更新」**（MXU 全量更新会搬走「与新包同名的根级条目」，会毁掉这个 wt）
4. 不跑 `git gc`
5. 不改 `.github/**`（加 `on: push` = 每次 push 触发 Mirror酱对外发版）
6. 不重编译 / 替换 `MaaBd2.exe`、`maafw/**`

---

## 3. 项目关键规则（动手前必读）

### 3.1 任务三处同步

`tasks/X.json` + `resource/pipeline/X.json` + `interface.json.import` **必须同步改**。
→ 三者只归一个 agent（**按任务纵向切**，不按目录横向切）。

### 3.2 pipeline 红线（写错必红叉）

- 🔴 **next 可能全不匹配的节点，必须有 `on_error`**；on_error 为空 + next 落空 = **任务红叉**。
- **`on_error` 是「换条路」不是「重试」**：已在 error_handling 时再全没命中 → 连败即死；任何命中都复位。
- **收尾节点**必须无 `recognition`（默认 `DirectHit`）且无 `next`；**绝不能 next 指回自己**。
- **`next` 里的必中节点 = 同轮立即兜底**（不等 timeout）→ 它前面的节点只有一次机会。
- **`[JumpBack]X` 链里藏必中节点 → 零进度无限自旋**（无 recognition 的兜底节点是重灾区）。
- **`max_hit` 是单向闩锁**（无 `clear_hit_count` 调用者）→ 用尽即本任务内永久跳过；
  也是**唯一能约束自环**的手段。公共兜底节点**不设** `max_hit`。
- ⚠️ **没有任务级总超时**：`timeout` 只约束「扫一遍 next」。
- 超时字段名是 **`timeout`**（写 `reco_timeout` 会**静默忽略**退 20s）。
- **`pipeline_override` = 顶级键替换，非深合并**；`timeout` 属上一节点。
- ⚠️ **跨文件重名节点 = 整个资源加载失败**（新增任务前先比对上游节点名）。
- `enabled:false` 跳过不耗时（默认 true）。

### 3.3 版本与发布门禁

- **版本号唯一真源 = `VERSION_ANCHORS`**（`interface.json` + `更新功能说明.md`），打包时自动改写；
  `expect` 不符 → **整个文件跳过**并告警（宁可漏改不误改）。
  ⛔ 不碰 `version.json` / `updater_cache.json`；`Verlog.xlsx` 只读提醒、绝不改写。
- 🔴 **Release 顺序 = 建 draft → 传资产 → 再 PATCH `draft:false`**。
  反过来（先发后传）会让 Mirror酱 workflow 在资产落盘前触发 → `Asset not found`
  → **GitHub 用户拿到新版、Mirror酱用户永久停在上一版**。
- **同 tag 换资产触达不到老用户**（更新提示只在版本号严格变大时触发）→ 修已发布版本必须发新 tag。
- 发布前必查：① 启动器是好的 ② Release `assets[]` + 直链可达。先 push 再建 Release。

### 3.4 打包排除表（改了文件就得同步）

- **打包器 `os.walk` 非 git 驱动** → 新增文件**必须**同步 `EXCLUDE_FILES` / `EXCLUDE_DIRS`，
  否则被扫进用户包。`data/maintainer/` 整目录已排除（本文件也不进包）。
- **入库靠 `REQUIRED_FILES` 逐项点名**，不能靠 `REQUIRED_DIRS`（目录级只要非空就过 → 缺文件静默通过）。
- ⛔ **绝不裁 `maafw/DirectML.dll`**；`PRUNED_BINARIES` 9 件不进包仍入仓。
- **retired 条目按 rel 精确匹配**排除（文件名匹配会误杀 `data/` 同名新版）。
- 包内 `changes.json`（deleted = retired 全量 + 到期 versioned）是 **retired 清理的唯一执行者，别删**。

### 3.5 `.bat` 硬约束

根因 = **非 ASCII 字节**（LF 加剧）；根治 = **纯 ASCII + CRLF**，注释永不写 `>`。
⚠️ 用 `sed` 改 `.bat` 会破坏 CRLF —— 用编辑器改。`.gitattributes` 已锁 `*.bat eol=crlf`、
`*.json eol=lf`（只管行尾，不管内容编码）。

### 3.6 目录布局不可动

- **根目录只留**：`MaaBd2.exe`、`interface.json`、`LICENSE`、`README.md`、`NOTICE.md`、
  注意事项 docx + 目录 `resource/ tasks/ misc/ agent/ maafw/ locales/ data/`。
- **不可移**（MXU 硬编码）：`maafw/`（exe_dir/maafw）、`locales/`（go-service 相对 cwd）、
  运行时 `config/`、`debug/`。
- `data/` 只装非用户件；维护者工具在 `data/maintainer/tools/`（**入库但不进包**）。

---

## 4. 踩坑要点（这段时间实测：症状 → 根因 → 处置）

| # | 症状 | 根因 | 处置 |
|---|---|---|---|
| 1 | **任务一闪就结束 + 绿勾 + 零提示** | go-service 的 tasker 守护在**任务开始前** `PostStop()`，不是失败路径 → 框架仍报 `Tasker.Task.Succeeded`，pipeline 一次都没评估 | 查 `debug/mxu-agent-*.log`，**别查 pipeline**。三个守护（分辨率 / HDR / 进程）**对所有任务生效** |
| 2 | 分辨率看着对却被掐 | `scaledOK` = **短边缩到 720 后另一边必须严格 = 1280**。客户区 1916×1080 → 1277 ≠ 1280 → 掐 | 必须窗口化 1920×1080（量的是**客户区**不是外框）。日志：`resolution check passed` / `aspect_ratio_min_resolution` |
| 3 | `failed to get target rect [name=X]` | `target_offset` 后两位是**加到匹配框宽高上的增量**；`box.w + offset.w <= 0` → 空 Rect。模板框恰好等于模板尺寸，负 offset 写成 `-(模板宽)` 必得 0×0 | 想点框中心就**不写 `target_offset`**；只写 2 元素 = `1×1`（点的还是左上角）。验算：读模板 PNG 尺寸算 `box + offset` |
| 4 | 节点成功但点击数 0（假阴性） | 框架永不调 `click()`，Win32 鼠标走 `touch_down` + `touch_up` | 假控制器须在 **`touch_down` 记坐标**；判"点到没"看是否落在识别框内 |
| 5 | 遮挡 / 最小化就黑屏 | `DXGI_DesktopDup_Window`（电脑端-前台）**无伪最小化**；`FramePool` / `PrintWindow`（maafw ≥ v5.8.0）无条件支持 | 用「电脑端」；「电脑端-前台」遮挡或最小化必抓不到画面 |
| 6 | MPE 里跑得通、实机跑不通 | MPE 是离线编辑器，**不加载 agent 的 tasker sink**，守护完全不参与 | 「编辑器里能过」**不算**实机可跑的证据 |
| 7 | `Agent start failed: status 0` | `MaaAgentClientConnect` 返回 false：spawn 成功、握手失败 → 杀软秒杀 go-service（最常见）/ 新旧混装协议 ≠ 8 | `debug\go-service.log` 不存在 = 进程没起来 = 杀软实锤 |
| 8 | 停在登录界面任务就停 | `StartGame` 链**没有任何节点处理「账号登录」**，且 `StartGame` 无 `on_error` → 停在无法识别处 = 红叉 | 先人工登录；先确认勾了「进入游戏-必选」 |
| 9 | Mirror酱查 latest 返回 404 / 8001 | `/api/resources/{rid}/latest` **必须带 `os` 与 `arch`** | 加 `?os=win&arch=x64`。同版本重复上传 → `409 code 8006`，属预期 |
| 10 | 改了 exe / 新增文件却没进仓库 | `.gitignore` 排 `*.exe`（`agent/*.exe` 靠 `git add -f` 强制入库） | 提交 exe 必须 `git add -f`；普通 `git add` **静默跳过** |
| 11 | `git mv` 搬家后文件神秘消失 | 搬家时被 gitignore 命中的文件**掉出 index**（`agent/*.exe`、ocr README、rcedit 都中过） | 搬完 `git add -f` 补回 |
| 12 | 自更新后维护者文件没了 | MXU 全量更新整目录搬走「与新包同名的**根级**条目」 | 维护者工具放 `data/maintainer/`；自更新后先 `git status` |
| 13 | 同源两次打包 hash 不同 | zip 条目时间戳 = 打包时刻 | **比内容不比 hash**；改完仓库文档必须重打（包内 README/NOTICE 是磁盘快照） |
| 14 | 改随包文档报 `WinError 5` | 有进程握着 DELETE 权限 | 别用 tmp + rename，**直接写目标路径**（有备份兜底） |
| 15 | 与上游合并后资源加载失败 | 跨文件重名节点（我们 34 个节点里 `Fishing_EnterFishingSpot_Frost` 曾与 MFABD2 重名） | 新增任务前比对上游节点名；改名要同时改 pipeline 键 + `pipeline_override` |
| 16 | 长任务跑不完没兜底 | `max_seconds` 在代码里**根本没被使用**（只用 `minigame_ms` / `bar_wait_ms`） | 别指望 `max_seconds` 兜底；长任务自己加墙钟预算 |
| 17 | 界面「绿了」以为跑完了 | 手动停止的任务最后也报 `Tasker.Task.Succeeded` | 看日志；`job.succeeded` 是**属性**不是方法 |
| 18 | 找色只出一个结果、拿不到位置 | `ColorMatch` 默认 `connected:false` = 命中像素合并成一个（存在性判定） | 要拿位置去点 → `connected:true`。`method` 4=RGB（按 RGB 填）/ 40=HSV / 6=灰度 |
| 19 | 新任务 UI 里看不到 | `import` 只管 UI（不影响资源加载），但没 import 就点不到 | 三处同步：tasks + pipeline + `interface.json.import` |

---

## 5. 环境冷知识（本仓特有）

- **`resource/model/ocr/`（21 M）不在仓库**（gitignore）→ 新建的 worktree **跑不起来**，
  需 `mklink /J` 链回主工作区；`config/` 也不在仓（首次自动生成或复制）。
- 删 worktree 前**必须先 `rmdir` 掉那个 junction** 再 `git worktree remove`，否则可能连带清掉主工作区的真目录。
- worktree 每份检出约 **120 M**（`maafw/` 63M + `agent/*.exe` 21M + `MaaBd2.exe` 31M），`.git` 236M 共享。
- **CI 只在 `release: published` + `workflow_dispatch` 触发** → push 自己的分支安全。
- push 走本机需绕代理：`env -u http_proxy -u https_proxy -u HTTP_PROXY -u HTTPS_PROXY GIT_SSL_NO_VERIFY=1 GIT_TERMINAL_PROMPT=0 git push ...`
- 无 submodule、无自定义 hook、LFS 装了但未启用。
- 提交信息用 `git commit -F <file>` 传，避免多行 heredoc 被 shell 吞字符。

---

## 6. 命令速查

| 目的 | 命令 |
|---|---|
| 每日开工 | `git fetch origin && git rebase origin/main` |
| 已 push 过的分支 | `git push --force-with-lease`（**禁 `--force`**） |
| 二进制冲突取舍 | `git checkout --ours/--theirs <path>` |
| 提交但排除文件 | `git add -A -- . ':!interface.json'` |
| 强制入库 exe | `git add -f agent/X.exe` |
| 看所有 wt | `git worktree list` |
| rebase 放弃 | `git rebase --abort` |
| 主控门控检查 | `git diff --name-only origin/main...origin/<分支> -- interface.json .github/ data/version.json` |

---

## 7. 变更记录

- **v1（2026-10-02）**：初版。由「协同施工铁律」+「框架行为与发布（脱敏入仓版）」+ 这段时间
  实测踩坑 19 条合并而成 —— 原两份散文件已并入本文件，**不再有第二份铁律**。
  ⛔ 维护者本机专属信息（备份落点、取 token 命令、venv 路径）**刻意不在这里**，留在 `.workbuddy/` 不入仓。

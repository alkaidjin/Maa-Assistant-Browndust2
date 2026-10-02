# Git Worktree 多 Agent 协同施工指南（BD2MAA 专用）

> 实测环境：`F:\MABd2v26.09.5`，`main @ ac80b2b`，git `2.55.0.windows.3`  
> v1 2026-10-02 初稿 / **v2 2026-10-02：补入 5 条施工铁律 + 3 处收紧 + 1 处修正**  
> 适用：同时开 2~5 个 agent 软件（WorkBuddy / CodeBuddy / Claude Code / Cursor 等）改同一个仓库  
> 本文件即"给施工 agent 的第一份读物"，开工第一条指令就该让它读 §5。

---

## 0. TL;DR（30 秒版）

1. 主工作区 `F:\MABd2v26.09.5` 独占 **main 分支 + 打包 + 发布 + 版本号 + 全局注册表**。
2. 每个 agent 一个 worktree：`git worktree add -b agent/<名字> F:\MABd2-wt\<名字> <基线>`。
3. **worktree 目录必须建在主工作区外面**（`F:\MABd2-wt\`，不能是 `F:\MABd2v26.09.5\wt\`）—— 原因见 §6.1。
4. 建完补两样：OCR 模型（junction 链接）+ 项目约定文档（agent 记忆不共享，见 §4.4）。
5. agent 只 commit 到自己分支，可 push 自己分支做备份（**不会触发 CI**，§2 已验证），合并一律回主工作区做。

代价：**每份约 120 MB**（`maafw/` 63M + `agent/*.exe` 21M + `MaaBd2.exe` 31M），F 盘现余 328 G，开 5 份无压力。

---

## 1. worktree 是什么

一条命令把仓库的**某个 commit 检出到一个新目录**，这个目录和主工作区**共用同一个 `.git`**。

它解决的核心痛点：**同一个分支不能同时在两个目录里干活**。传统做法是 clone 多份（各自一份 `.git`，236 M × N，还要各配一遍 remote / 凭据 / hook），或者来回 `git stash` + `checkout`（agent 跑到一半被切分支直接崩）。worktree 两个问题一起解决。

### 共享什么 / 隔离什么

| 项目                              | 是否共享 | 实际影响                                     |
| ------------------------------- | ---- | ---------------------------------------- |
| 对象库 `objects/`                  | ✅ 共享 | 不重复占空间；一个 wt 里 `git fetch` 到的，其他 wt 立刻可见 |
| `refs/`（分支、tag、remote-tracking） | ✅ 共享 | 在 wt-A 建的分支，wt-B `git branch` 能看到        |
| `remote` / 凭据 / `.git/config`   | ✅ 共享 | 只需配一次 push 方式                            |
| `.gitattributes`                | ✅ 共享 | json=LF、bat/cmd=CRLF 在所有 wt 自动生效 ✅       |
| git hook                        | ✅ 共享 | ⭐ 可用一个 pre-commit 同时管住所有 wt（§5.10）       |
| `index`（暂存区）                    | ❌ 独立 | 各 wt 的 `git add`/`status` 互不影响           |
| `HEAD`                          | ❌ 独立 | 每个 wt 停在不同分支                             |
| 工作目录文件                          | ❌ 独立 | **核心：互不覆盖**                              |
| 未跟踪 / 被 ignore 的文件              | ❌ 独立 | `config/`、`debug/`、`cache/` 各有一份         |
| 分支独占                            | ⚠️   | 一个分支只能挂一个 wt（除非 `--detach`）              |

### 与其他方案对比

| 方案                   | 磁盘       | 配置                      | 适合度                      |
| -------------------- | -------- | ----------------------- | ------------------------ |
| **worktree** ⭐       | +120 M/份 | 零（继承）                   | **首选**                   |
| `git clone` 多份       | +356 M/份 | remote/凭据/hook 各配一遍     | 次选                       |
| `git clone --shared` | +120 M/份 | 同上 + alternates 有 gc 风险 | 不推荐（比 worktree 麻烦且无额外好处） |
| 一个工作区塞多个 agent       | 0        | —                       | ❌ 必炸：文件互相覆盖、index 抢锁     |
| 容器 / VM              | 很重       | 很重                      | 过度设计                     |

---

## 2. 本仓库体检结果（2026-10-02 实测）

| 项                                       | 实测值                                          | 对多 agent 的意义                    |
| --------------------------------------- | -------------------------------------------- | ------------------------------- |
| git 版本                                  | 2.55.0.windows.3                             | 现代 worktree 全套命令可用              |
| 现有 worktree                             | 无（只有主工作区）                                    | 从零开始，无历史包袱                      |
| 分支                                      | 只有 `main` @ `ac80b2b`，已同步 `origin/main`      | 按 §5.1 起 `agent/*` 分支           |
| 已跟踪文件                                   | 259 个，检出体积 **120 M**                         | 每份成本明确                          |
| `.git`                                  | 236 M                                        | 共享，不重复                          |
| submodule                               | 无                                            | 建 wt 不用 `--recurse-submodules`  |
| 自定义 hook                                | 无（v2 起可选加固，§5.10）                            | 无副作用                            |
| Git LFS                                 | 装了 3.7.1 但**本仓未启用**（`git lfs ls-files` 空）    | 无 LFS 拉取问题                      |
| CI 触发条件                                 | 仅 `release: published` + `workflow_dispatch` | ⭐ **push 任意分支都不会触发 Release 上传** |
| `.gitattributes`                        | `*.json→LF`、`*.bat/*.cmd→CRLF`               | 所有 wt 自动一致，不用再教                 |
| `core.ignorecase=true`；`autocrlf` 未全局设置 | 靠 `.gitattributes` 控制                        | 一致                              |
| 仓库可见性                                   | **public**                                   | ⚠️ agent 分支 push 后即公开（§5.11）    |
| F 盘剩余                                   | 328 G                                        | 5 份 wt 约 600 M，无压力              |
| 主工作区当前状态                                | **32 项未提交改动**                                | ⚠️ 见 §4.1，开工前必须先处理              |

---

## 3. 部署形态

```
F:\MABd2v26.09.5\        main             ← 主控：合并 / 打包 / 发布 / 全局注册表 / exe 入库
F:\MABd2-wt\a\           agent/a-fishing  ← Agent A：自动钓鱼
F:\MABd2-wt\b\           agent/b-pvp      ← Agent B：PVP 重做
F:\MABd2-wt\c\           agent/c-docs     ← Agent C：文档与整理
F:\MABd2-wt\_RIG_BUSY    （实机锁，§5.8）
```

职责划分（**这个划分比工具本身更重要**）：

- **主工作区** = 唯一可做：改版本号 / 跑 `build_release_zip.bat` / `push origin main` / 建 Release / 改全局注册表（§5.2）/ 编译入库 exe（§5.7）。
- **worktree** = 纯施工区，干完 commit 到自己分支，交回主工作区合并。

---

## 4. 一步步实操

> 本机 Bash 工具偶发拿不到 Git 的 `usr\bin`（`ls/grep/sed` not found）。遇到时先执行：  
> `export PATH="/c/Users/ALKAID/.workbuddy/binaries/PortableGit/versions/1.2.0/usr/bin:/c/Program Files/Git/usr/bin:$PATH"`  
> 或直接用 PowerShell / CMD（git 在 PATH 里），命令都一样。

### 4.1 开工前：先决定基线 commit（⚠️ 别跳过）

**主工作区现在有 32 项未提交改动**（AutoFishing / Warcraft / RewardRoutine 等）。worktree 从**某个 commit**检出，**不会带走未提交改动**。所以：

- 新 agent 的任务**依赖**这批改动 → 先在主工作区 commit（哪怕 WIP），再从那个 commit 建 wt。
- 任务**独立** → 直接 `git worktree add -b agent/x F:/MABd2-wt/x origin/main`（基线 = `ac80b2b`）。
- 折中：把改动 commit 到 `wip/main-20261002` 分支，主工作区继续干，新 wt 从该分支建。

### 4.2 建 worktree

```bash
cd /f/MABd2v26.09.5
git fetch origin
git worktree add -b agent/a-fishing F:/MABd2-wt/a origin/main
```

变体：

```bash
git worktree add -B agent/a-fishing F:/MABd2-wt/a origin/main          # -B：分支已存在则强制重置
git worktree add --detach F:/MABd2-wt/peek origin/main                 # 只读调研，不占分支名
git worktree add --lock --reason "U盘上，别 prune" F:/MABd2-wt/d main  # 放移动盘必加
```

确认：`git worktree list`、`git -C F:/MABd2-wt/a status`（应干净）。

### 4.3 补齐运行时缺失件（本仓特有，必做）

`.gitignore` 排除了几样运行时必需的东西，新建 wt 里**没有**：

| 缺失                            | 大小   | 处理                         |
| ----------------------------- | ---- | -------------------------- |
| `resource/model/ocr/`（OCR 模型） | 21 M | **junction 链接回主工作区**，别复制   |
| `config/`（maa_option 等）       | 小    | 复制一份（含个人分辨率设置）             |
| `debug/`、`cache/`             | —    | 不用管，运行时自动生成                |
| `data/updates/`（更新缓存 246 M）   | 大    | **不要给** —— wt 里禁止自更新（§5.5） |

```bat
mklink /J F:\MABd2-wt\a\resource\model\ocr F:\MABd2v26.09.5\resource\model\ocr
xcopy /Y /Q F:\MABd2v26.09.5\config\*.json F:\MABd2-wt\a\config\
```

反向坑：删 wt 前**必须先 `rmdir` 掉 junction** 再 `git worktree remove` —— 否则有把主工作区真目录内容一起清掉的风险（§7 脚本已处理）。

### 4.4 把项目约定"喂"给 wt 里的 agent（⚠️ 容易漏）

`.workbuddy/` 被 gitignore，**每个目录的 agent 记忆独立**。wt 里的 agent 看不到主工作区积累的铁律（任务三处同步、pipeline_override 语义、go-service 守护坑、发布门禁……）。

- **推荐（已落地 2026-10-02）**：铁律已入仓为**单一真源**，施工 agent 直接读它，无需复制：
  - `data/maintainer/REF-多Agent协同施工铁律.md` —— 协同铁律 + 项目关键规则 + 19 条踩坑要点，
    **自包含、开工必读**（文件头还有一段可直接复制的「喂给 agent 的第一条指令」）
  - 本文件 —— 完整教程与背景（worktree 原理 / 脚本 / FAQ），按需深入
  所有人看到的永远是同一份最新版本。
- **补充**：只有**含本机环境细节**的记忆才需要复制过去：

```bat
mkdir F:\MABd2-wt\a\.workbuddy\memory
copy /Y F:\MABd2v26.09.5\.workbuddy\memory\MEMORY.md F:\MABd2-wt\a\.workbuddy\memory\
```

复制过去的是**快照**，之后不同步。⚠️ **禁止入仓**（含本机路径，仓库 public），见 §5.11。
⛔ **不要复制 `.workbuddy/memory/` 下的 REF 原件** —— 含备份落点、GCM 取 token 命令等本机信息；
入仓版已脱敏并入 `REF-多Agent协同施工铁律.md`。

### 4.5 开工

把 agent 软件的工作目录指向 `F:\MABd2-wt\a`（新窗口 / 新会话指定 cwd）。第一条指令建议：

> 你在 git worktree `F:\MABd2-wt\a`（分支 `agent/a-fishing`）里工作。  
> 先读 `data/maintainer/REF-多Agent协同施工铁律.md` 全文 —— 唯一真源、自包含  
> （协同铁律 / 项目规则 / 踩坑要点），读完即可开工。  
> 只改 `<划给它的文件范围>`。**不要碰全局注册表**（§5.2），尤其 `interface.json`。  
> 不要 `push origin main`、不要打包、不要在软件里点"设置-更新"、不要改 `.github/`。  
> 实测前必须先抢实机锁（§5.8）。完成后 commit，不要 merge。

### 4.6 交工与合并

agent 侧：

```bash
git -C F:/MABd2-wt/a add -A
git -C F:/MABd2-wt/a commit -m "..."
# 可选：push 自己分支做云端备份（已验证不触发 CI）
env -u http_proxy -u https_proxy -u HTTP_PROXY -u HTTPS_PROXY GIT_SSL_NO_VERIFY=1 GIT_TERMINAL_PROMPT=0 \
  git -C F:/MABd2-wt/a push -u origin agent/a-fishing
```

主控侧（主工作区）：

```bash
cd /f/MABd2v26.09.5
git fetch origin

# ① 门控检查：施工分支若碰了全局注册表 → 打回，别急着 merge
git diff --name-only origin/main...origin/agent/a-fishing -- \
  interface.json data/version.json data/retired_files.json \
  data/更新功能说明.md data/maintainer/tools/build_release_zip.py .github/
# 输出非空 = 违规，让 agent revert 后重推

# ② 通过后合并
git merge --no-ff origin/agent/a-fishing
```

**合并顺序**：先合改动面小的，后合改动面大的；每合一个跑一次冒烟，别一次合 3 个再排查。

### 4.7 清理

```bash
git worktree remove F:/MABd2-wt/a   # 有未提交改动会被拒绝（防手滑）
git worktree list                   # 确认
git branch -d agent/a-fishing       # 已合并后删分支
git worktree prune                  # 目录被手动删掉后清残留元数据
git worktree prune -n -v            # 先干跑看会删什么
```

---

## 5. 协同纪律（决定成败，工具只解决一半问题）

### 5.1 分支命名

`agent/<agent标识>-<任务>`，例如 `agent/a-fishing`、`agent/b-pvp`。  
一个 agent 一个分支；同一任务多轮就 `agent/a-fishing-2`，别复用。

### 5.2 全局注册表 = 合并时门控（施工 agent 一律不碰）⭐

> v1 写的是"热点文件同一时刻只许一人碰（串行）"。**v2 收紧为更可靠的版本**：  
> 串行靠自觉，门控靠机制。施工 agent **根本不提交**这些文件的改动；需要变更时写进自己的  
> claim 文件，由主工作区在合并时统一施加。

**全局注册表清单（施工 agent 只读、不写）：**

| 文件                                                | 为什么是全局注册表                                                                      |
| ------------------------------------------------- | ------------------------------------------------------------------------------ |
| `interface.json`                                  | **头号冲突源**：`version`（版本锚点）+ `import`（任务注册）+ `agent`（agent 注册）。几乎每任务都要动，也必然是必撞文件 |
| `data/maintainer/tools/build_release_zip.py`      | `VERSION_ANCHORS` / `EXCLUDE_*` / `REQUIRED_FILES` 唯一注册表                       |
| `data/更新功能说明.md`                                  | 被 `VERSION_ANCHORS` 改写的版本锚点                                                    |
| `data/version.json`、`data/maintainer/Verlog.xlsx` | 版本 / 日志单一来源                                                                    |
| `data/retired_files.json`                         | 增量更新删除清单，写错会误删用户文件                                                             |
| `.github/**`                                      | CI 触发条件；加一个 `on: push` 就会让每次 push 触发 Mirror酱上传                                 |

**正确做法**：需要注册新任务 / 新 agent / 改版本号时**不动文件**，在自己 claim 里写结构化请求（§5.6），主控合并时统一改。

**本地测试例外**（现实中绕不开）：agent 要在 wt 里跑新任务，UI 里必须看得到它 —— 那就临时改 `interface.json`，但：

- 交工前还原：`git checkout -- interface.json`
- 或提交时排除：`git add -A -- . ':!interface.json'`
- 主控 merge 前用 §4.6 ① 门控命令检查，非空即打回

### 5.3 一个分支只挂一个 worktree

git 会直接拒绝在两个 wt 里检同一分支。让每个 agent 从一开始就用自己的分支。

### 5.4 未提交改动不共享（最大的认知落差）

wt-A `git add` 了但没 commit 的东西，wt-B 和主工作区**都看不到**。所以：

- agent 间交接中间成果 → 先 commit（哪怕 WIP），对方 `git fetch` + `git merge`。
- 主工作区那 32 项未提交改动处于"只有你能看见"的状态（§4.1）。
- 新增的未跟踪文件（如 `resource/image/**/*.png`）同理，不 commit 就传不出去。

### 5.5 worktree 里禁止做的事

1. **不打包** —— `build_release_zip` 会改写版本锚点、读 `data/`；且打包器是 `os.walk` 扫磁盘（非 git 驱动），在 wt 里打出的包不可信。
2. **不 `push origin main`、不建 Release** —— 发布是主工作区独占。
3. **不在软件里点"设置-更新"** —— MXU 全量更新会整目录搬走「与新包同名的根级条目」，在 wt 里执行会毁掉该 wt（主工作区曾因此被抹掉 22 个维护者文件）。
4. **不跑 `git gc`** —— 多 wt 共享对象库，并发 gc 会撞 `.git` 锁（§5.9）。
5. **不改 `.github/**`** —— 尤其不许加 `on: push` / `pull_request` 触发条件（public 仓库 + Mirror酱上传，误触发 = 对外发版）。
6. **不重编译 / 替换 `MaaBd2.exe` 与 `maafw/**`** —— 见 §5.7。

### 5.6 轻量任务看板（不需要额外工具）

仓库里建 `data/maintainer/claims/`，每个 agent 一个 **自己独占的文件**（天然无冲突）：

```
data/maintainer/claims/a-fishing.md
data/maintainer/claims/b-pvp.md
```

模板：

```markdown
# claim: a-fishing
- agent: A（WorkBuddy / wt 目录 F:\MABd2-wt\a）
- branch: agent/a-fishing
- baseline: origin/main @ ac80b2b
- 我独占的文件: tasks/AutoFishing.json, resource/pipeline/AutoFishing.json,
                resource/image/Fish/**, data/maintainer/agent/fishing/**
- 实机锁: 未占用 / 占用中（14:00-14:40）
- 状态: 施工中 / 待合并 / 已交工

## 需要主控代改（全局注册表）
- interface.json: import 加 "tasks/AutoFishing.json"
- interface.json: tasks 数组加 {"name":"自动钓鱼（测试版）","entry":"AutoFishing_Start"}
- interface.json: 无需改 version（本次不发版）
```

**别做成一个所有人写的共享文件** —— 那文件自己会变成冲突源。

### 5.7 二进制文件：单线变更 + 主控入库 ⭐

`agent/*.exe`、`MaaBd2.exe`、`maafw/**.dll` 都是二进制，**无法文本合并**。两个分支各自重编译过同一个 exe，merge 时必然 binary conflict，只能二选一，白丢一半工作。

规则（从强到弱）：

1. **绝对禁止**：任何 wt 不重编译、不替换 `MaaBd2.exe` 和 `maafw/**`（MXU 主程序 + 框架二进制，只能随上游/官方整体更新，在主工作区做）。
2. **exe 单线变更**：`agent/*.exe` 谁的功能谁编译；同一时刻只有一个 wt 碰某个 exe；别人合并完自己 rebase 再重编。
3. **推荐做法（更彻底）**：**exe 只在主工作区入库**。wt 里只改 Go 源码（`data/maintainer/agent/<name>/`）并本地编译自测，**不提交 exe**。  
   落地手段 —— 写进 wt 的私有排除表（不入仓、不影响他人）：
   ```bash
   # worktree 的 .git 是文件，私有 exclude 在主 .git 下：
   #   F:\MABd2v26.09.5\.git\worktrees\<wt名>\info\exclude
   echo 'agent/fishing.exe' >> "F:/MABd2v26.09.5/.git/worktrees/a/info/exclude"
   ```
   这样 wt 里 `git add -A` 不会带上编译产物，exe 由主控统一编译入库。
4. **冲突兜底**（真撞了）：


```bash
git checkout --ours   agent/fishing.exe   # 保留当前分支这一侧
git checkout --theirs agent/fishing.exe   # 采用对方那一侧
```

选完**必须重新编译验证**，别直接拿去打包。  
5\. `agent/*.exe` 是 `git add -f` 强制入库的（`.gitignore` 排 `*.exe`）。主工作区提交新 exe 时必须 `git add -f agent/x.exe`，普通 `git add` 会**静默跳过**。

### 5.8 实机互斥：实测串行，UI / 配置可并行 ⭐

**同一台机器 + 同一个游戏窗口，多个 wt 不能同时跑实机任务。** 抢的是同一套物理资源：

- 鼠标 / 键盘输入 —— 两个任务同时点，动作互相打断
- 「电脑端-前台」(DXGI + Seize) **物理独占**窗口句柄，后启动的直接失败
- 「电脑端」(FramePool) 不抢焦点，但截图同源，两边在同一窗口上各按各的理解操作
- 游戏账号状态共享 —— 同一个号被两套逻辑驱动，可能互踢 / 掉线 / 卡死
- `go-service` 守护（分辨率 / 进程 / HDR 检测）两个实例会互相干扰误判

**可并行**：改代码、改 JSON、MPE 编辑、静态检查、打包、写文档、开 UI 看界面。  
**必须串行**：任何真实启动游戏、跑任务的操作（同一 agent 的多轮实测也要串行）。

实机锁（仓外，所有 wt 都能看见）：

```bat
:: rig_lock.bat 开工前
if exist F:\MABd2-wt\_RIG_BUSY (
  echo RIG BUSY: & type F:\MABd2-wt\_RIG_BUSY
  exit /b 1
)
> F:\MABd2-wt\_RIG_BUSY echo agent-a 自动钓鱼 2026-10-02 14:00

:: 收工
del F:\MABd2-wt\_RIG_BUSY
```

锁文件写 agent 名 + 任务 + 时间；**结束时必须删**。忘了删 → 主控手动清。

### 5.9 每天开工先 rebase（把冲突暴露在写代码之前）

```bash
git -C F:/MABd2-wt/a fetch origin
git -C F:/MABd2-wt/a rebase origin/main
```

⚠️ **修正**：若分支**已经 push 过**，rebase 会改写历史，之后 push 必须用 `--force-with-lease`（**绝不用 `--force`**）：

```bash
git -C F:/MABd2-wt/a push --force-with-lease origin agent/a-fishing
```

不想改写历史就用 `git merge origin/main`（多一个 merge commit，但更安全）。

rebase 冲突时：

- **停下报告，不要 `git rebase --skip`** —— skip 会静默丢弃自己的 commit
- 要放弃就 `git rebase --abort`，回到开工前状态

配套（**已执行**：`git config gc.auto 0`）：关闭自动 gc，避免任一 wt 触发 gc 撞 `.git` 锁。  
代价是 `.git` 慢慢膨胀，定期在**没有 wt 活动**时手动跑：

```bash
git -C /f/MABd2v26.09.5 gc --auto     # 或 git maintenance run
```

不建议 `--prune=now` 激进清理（会丢掉 reflog 救援窗口）。

### 5.10 可选加固：一个 pre-commit hook 管住所有 wt

git hook 是**共享**的（`.git/hooks` 或 `core.hooksPath`），写一个就能同时约束所有 wt。  
hook 里判断"我是不是主工作区"，非主工作区则禁止提交全局注册表 + 主程序二进制：

```sh
#!/bin/sh
# .git/hooks/pre-commit  （主工作区与所有 wt 共享）
git_dir=$(git rev-parse --absolute-git-dir)
common=$(git rev-parse --path-format=absolute --git-common-dir)
[ "$git_dir" = "$common" ] && exit 0        # 主工作区放行

staged=$(git diff --cached --name-only)
bad=$(printf '%s\n' "$staged" | grep -E '^(interface\.json|data/version\.json|data/retired_files\.json|data/更新功能说明\.md|data/maintainer/tools/build_release_zip\.py|\.github/|MaaBd2\.exe|maafw/)')
if [ -n "$bad" ]; then
  echo "BLOCKED (worktree): 全局注册表 / 主程序二进制 只能在主工作区提交:"
  printf '%s\n' "$bad"
  echo "需要变更请写进 data/maintainer/claims/<你的名字>.md，由主控在合并时统一施加。"
  exit 1
fi
exit 0
```

注意：`.git/hooks` **不入库**（clone 不带）。要随仓库分发就放 `.githooks/` 入仓 + `git config core.hooksPath .githooks`，  
但那样必须同步把 `.githooks` 加进 `build_release_zip.py` 的 `EXCLUDE_DIRS`，否则被 `os.walk` 扫进发布包。

### 5.11 public 仓库：agent 分支一旦 push 就是公开可见的

- 仓库 **public**，所有 ref（含 `agent/*`）任何人都能 fetch，**不需要合 PR**。
- 因此：**记忆文件、含本机路径的文档、任何密钥一律不入仓**。`.workbuddy/` 已 gitignore，复制进 wt 的记忆文件同样在其下 —— 只要不 `git add -f` 就安全。⚠️ 明确禁止 `git add -f .workbuddy/`。
- **push 前扫一遍**（agent 写的 md / 脚本里常带绝对路径）：
  ```bash
  git -C F:/MABd2-wt/a diff --cached | grep -nE 'C:\\Users\\ALKAID|F:\\MABd2|MirrorChyan|CDK|token|api[_-]?key'
  ```
- 历史不可撤销：public 仓库 force push 之后别人可能已 fetch。真误推敏感文件要走 `git filter-repo` 改历史 + 强推，别指望"删掉再推一次"能抹掉。

---

## 6. 本仓库专属坑位清单

按踩中概率排序：

1. **wt 目录不能建在主工作区内部**。建成 `F:\MABd2v26.09.5\wt\a` 的话：① 打包器 `os.walk` 把整个 wt 扫进发布包；② MXU 自更新把它当「根级同名条目」搬走。→ 一律放 `F:\MABd2-wt\`。
2. **32 项未提交改动不会进 wt**（§4.1）。
3. **OCR 模型 / config 缺失**，不补跑不起来（§4.3）。
4. **MXU 自更新会毁 wt**（§5.5.3）。
5. **agent 记忆不共享**（§4.4），且**禁止入仓**（§5.11）。
6. **`agent/*.exe` 是 `git add -f` 强制入库**，普通 `git add` 静默跳过；二进制冲突见 §5.7。
7. **删 wt 前先拆 junction**（§4.3）。
8. **`MaaBd2.exe` 运行时占文件锁**：wt 里软件还在跑时 `git worktree remove` 会失败 → 先关软件再删。
9. **push 要绕代理**：`env -u http_proxy -u https_proxy -u HTTP_PROXY -u HTTPS_PROXY GIT_SSL_NO_VERIFY=1 GIT_TERMINAL_PROMPT=0 git push ...`。wt 共享 remote 配置，各 wt 一样用。
10. **`.bat` 在 wt 里同样是 CRLF + 纯 ASCII**（`.gitattributes` 只管行尾不管内容编码）。
11. **不要用 sparse-checkout 省空间**：`os.walk` 打包会漏文件、MXU 运行会缺资源。120 M 一份，全量。
12. **实测串行**（§5.8）—— UI 和编辑能并行，别把"能开多个"误当成"能同时跑"。

---

## 7. 一键脚本

存到 `data/maintainer/tools/`，**必须纯 ASCII + CRLF 行尾**（本仓铁律；用编辑器改，别用 `sed`，sed 会破坏 CRLF）。

### `wt_new.bat` —— 建一个 agent worktree

```bat
@echo off
setlocal
set "MAIN=F:\MABd2v26.09.5"
set "WTROOT=F:\MABd2-wt"

if "%~1"=="" (
  echo usage: wt_new.bat ^<name^> [baseline]
  echo   example: wt_new.bat a-fishing origin/main
  exit /b 1
)
set "NAME=%~1"
set "BASE=%~2"
if "%BASE%"=="" set "BASE=origin/main"

set "DIR=%WTROOT%\%NAME%"
set "BR=agent/%NAME%"

git -C "%MAIN%" fetch origin
git -C "%MAIN%" worktree add -b "%BR%" "%DIR%" %BASE%
if errorlevel 1 (
  echo FAILED: worktree add
  exit /b 1
)

if not exist "%DIR%\resource\model\ocr" (
  mklink /J "%DIR%\resource\model\ocr" "%MAIN%\resource\model\ocr"
)
if not exist "%DIR%\config" mkdir "%DIR%\config"
xcopy /Y /Q "%MAIN%\config\*.json" "%DIR%\config\" 2>nul

if not exist "%DIR%\.workbuddy\memory" mkdir "%DIR%\.workbuddy\memory"
copy /Y "%MAIN%\.workbuddy\memory\MEMORY.md" "%DIR%\.workbuddy\memory\" 2>nul

echo.
echo done: %DIR%
echo branch: %BR%
echo baseline: %BASE%
endlocal
```

### `wt_clean.bat` —— 安全拆除

```bat
@echo off
setlocal
set "MAIN=F:\MABd2v26.09.5"
set "WTROOT=F:\MABd2-wt"
if "%~1"=="" (
  echo usage: wt_clean.bat ^<name^>
  exit /b 1
)
set "DIR=%WTROOT%\%~1"

if not exist "%DIR%" (
  echo not found: %DIR%
  exit /b 1
)

git -C "%DIR%" status --porcelain > "%TEMP%\wt_st.txt"
for %%A in ("%TEMP%\wt_st.txt") do if %%~zA GTR 0 (
  echo ABORT: uncommitted changes in %DIR%
  echo commit or push first.
  del "%TEMP%\wt_st.txt"
  exit /b 1
)
del "%TEMP%\wt_st.txt"

if exist "%DIR%\resource\model\ocr" rmdir "%DIR%\resource\model\ocr"
git -C "%MAIN%" worktree remove "%DIR%"
if errorlevel 1 (
  echo remove failed - close MaaBd2.exe / MaaPiCli first, then retry
  exit /b 1
)
git -C "%MAIN%" worktree prune
echo removed: %DIR%
endlocal
```

### `rig_lock.bat` / `rig_free.bat` —— 实机锁（§5.8）

```bat
:: rig_lock.bat <agent> <task>
@echo off
set "LOCK=F:\MABd2-wt\_RIG_BUSY"
if exist "%LOCK%" (
  echo RIG BUSY:
  type "%LOCK%"
  exit /b 1
)
> "%LOCK%" echo %~1 %~2 %DATE% %TIME%
echo rig locked by %~1
```

```bat
:: rig_free.bat
@echo off
del "F:\MABd2-wt\_RIG_BUSY" 2>nul
echo rig released
```

---

## 8. 命令速查

| 目的           | 命令                                                           |
| ------------ | ------------------------------------------------------------ |
| 建（新分支）       | `git worktree add -b agent/x F:/MABd2-wt/x origin/main`      |
| 建（只读调研）      | `git worktree add --detach F:/MABd2-wt/peek origin/main`     |
| 列全部          | `git worktree list` / `git worktree list -v`                 |
| 在 wt 里执行     | `git -C F:/MABd2-wt/x status`                                |
| **每日开工同步**   | `git -C <wt> fetch origin && git -C <wt> rebase origin/main` |
| 已 push 过的分支  | `git push --force-with-lease`（**禁用 `--force`**）              |
| 移动 wt        | `git worktree move F:/MABd2-wt/x F:/other/x`                 |
| 删 wt         | `git worktree remove F:/MABd2-wt/x`                          |
| 清残留元数据       | `git worktree prune`                                         |
| 二进制冲突取舍      | `git checkout --ours/--theirs <path>`                        |
| 防 prune（移动盘） | `git worktree lock --reason "..." F:/MABd2-wt/x`             |
| 路径变了修引用      | `git worktree repair`                                        |
| 关自动 gc       | `git config gc.auto 0`（已设）                                   |

---

## 9. FAQ

**Q：agent 能不能自己建 wt？**  
能，但建议主控建好再把目录给它。agent 自己建容易把目录选在主工作区内部（§6.1）。

**Q：我的任务必须改 `interface.json` 才能测试，怎么办？**  
允许临时改（UI 里看不到任务就没法测），但**交工前还原**：`git checkout -- interface.json`，或提交时排除 `git add -A -- . ':!interface.json'`。需要主控代改的写进 claim 的"需要主控代改"段（§5.2 / §5.6），主控 merge 前用 §4.6 门控命令检查。

**Q：两个 agent 改同一个 json 怎么办？**  
git 合并靠冲突标记人工判，JSON 没有语义合并。优先从任务划分避开：**按任务纵向切**（某条链路的 `tasks/X.json` + `resource/pipeline/X.json` + 图片全归一个 agent），不是按目录横向切。真撞了：后交的那个先 rebase 同步（§5.9），把冲突提前暴露。

**Q：exe 冲突了怎么收场？**  
`git checkout --ours/--theirs <exe>` 二选一，然后**重新编译验证**再入库。更好的是压根不让 wt 提交 exe（§5.7.3，用 wt 私有 exclude 表）。

**Q：wt 里的 agent 能跑 MaaBd2 实测吗？**  
能，`maafw/`、`locales/`、`agent/*.exe`、`MaaBd2.exe` 都完整检出，`config/`、`debug/` 独立，不污染主工作区。记得补 OCR 模型（§4.3）、**别开软件内更新**（§5.5.3）。⚠️ 但**同一时刻只能有一个 wt 在实测**（§5.8）。

**Q：多个 wt 能同时开着改代码吗？**  
能。改代码 / 改配置 / 编辑 JSON / 跑 UI 都并行。只有"真实启动游戏跑任务"要抢实机锁。

**Q：一个 agent 干完了，能把 wt 给下一个 agent 接着用吗？**  
可以（换任务就 `git checkout -b agent/新名`），但更干净是拆掉重建 —— 120 M、几秒钟的事。

**Q：怎么让某个 wt 永远不被 prune（放移动硬盘）？**  
`git worktree lock --reason "..." <path>`。Git 认为 wt 目录"消失"时 `prune` 会清元数据，移动盘离线会中招。

**Q：`.gitignore` 在各 wt 一样吗？**  
一样（入仓）。所以 `cache/`、`config/`、`debug/`、`data/updates/` 在每个 wt 独立、互不干扰 —— 好事。各 wt 还可有自己的私有排除表：`.git/worktrees/<wt名>/info/exclude`。

**Q：agent 分支 push 出去安全吗？**  
不触发 CI（workflow 只在 release published / dispatch 触发），但**仓库 public，分支内容任何人都能 fetch**。所以记忆文件、本机路径、密钥一律不进 commit（§5.11），push 前扫一遍。

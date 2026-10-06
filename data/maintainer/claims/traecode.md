# claim: traecode（TRAE）

- **worktree**：`F:\MABd2-wt\traecode-workspace`
- **分支**：`agent/traecode-workspace`
- **基线**：`bddbc55`（2026-10-02 主控四项拍板后开工）

## 本轮任务（主控 2026-10-02 指派，6 项优化）

| # | 事项 | 拍板方向 |
|---|---|---|
| 6 | go-service 去黑盒 | **裁剪 fork 入仓**：只移植棕2 实际依赖（common/schedule + 5 个 tasker sink + 支撑包），写 build.bat，本机构建验证 |
| 8 | 大文件治理 | **外移 + bootstrap**：历史不重写、当前 5 个大文件保留跟踪；建 Release 资产拉取机制（manifest + sha256 + 脚本），今后大二进制不再入仓 |
| 9 | .gitignore | 白名单化：4 个 exe + 20 个 DLL 显式 `!` 例外；清乱码行；更新过时注释 |
| 12 | 多语言 | **弱化声明**：interface 只保留简体中文；README 明确仅中文。`locales/go-service/` 5 语言是守护警告页资源，**保留不动** |
| 13 | JSON 风格 | **只格式化不改名**：14 个 tasks/*.json 统一缩进；pipeline 不动（MPE 会回写）；任何标识符不动 |
| 14 | 分辨率 | 随第 6 点：梳理 aspectratio 判定、列安全分辨率白名单文档；不改判定语义，实机验证由主控做 |

## 改动文件范围

- `.gitignore`
- `interface.json`（仅 languages/多语言声明相关字段）、根 README.md、可能碰 misc/locales/
- `data/maintainer/agent/go-service/**`（**新增目录**，裁剪自 MaaEnd `64fac11`，AGPL）
- `data/maintainer/tools/bootstrap_assets.*` + 资产 manifest（新增）
- `tasks/*.json`（14 个，仅缩进格式化）
- `data/maintainer/README.md` 等文档（新流程说明）

## 热点占用

- `interface.json`：占用（仅多语言弱化，本分支施工期间不合并别人）
- `build_release_zip.py`：**尽量不碰**；bootstrap 流程与打包器的衔接若必须改，登记到本文件交主控施加
- 不碰：`.github/**`、`MaaBd2.exe`、`maafw/**`、`agent/*.exe`（构建验证产物先放临时目录，替换前单独报告）

## 与 workbuddy 分支的冲突协调

`agent/workbuddy-workspace`（926ade3，钓鱼死参数删除）也改了 `tasks/AutoFishing.json`
与 `resource/pipeline/AutoFishing.json`。
对策：第 13 点 tasks 格式化放在最后一个 commit，并 rebase 到 workbuddy 分支之后；
若 AutoFishing 冲突，双方语义都保留（它删 max_seconds + 我缩进）。
建议主控合并顺序：workbuddy 926ade3 先合，本分支后合。

## 交工记录（2026-10-02 全部完成，7 commit）

分支已 rebase 到含 `b2300e3`（主控 P2 AutoFishing 兜底）的 origin/main，工作树 clean。

| 点 | commit | 交付物与验证 |
|---|---|---|
| 9 | `571751c` | .gitignore 白名单化；check-ignore 双向 7 放行/4 拦截全过 |
| 12 | `e01169f` | interface 仅 zh_cn；删 4 个 misc locale；打包器 REQUIRED 同步；retired v26.09.14 登记；README 语言声明；dry-run 全绿 |
| 6 | `6491aa1` | 裁剪 fork 14 包/40 go 文件入 `data/maintainer/agent/go-service/`；gofmt/vet 过；go1.25.6 构建 8.8MB（原 15.8）；加载真实 maafw 冒烟：i18n 24 文案、6 组件注册、Agent server started、零 stderr；NOTICE 第 3 节登记 |
| 14 | `6144dad` | `参考-分辨率守护判定与安全分辨率白名单.md`：三条件数学推导、白名单/拦截表、Alt+Enter 对棕2 不触发、实机清单 |
| 8 | `d1dc2bb` | `bootstrap_assets.{py,bat}` + `assets_manifest.json`（8 项 sha256）；真实 v26.09.13 release（81.8MB）端到端下载/抽取/校验/幂等通过；maintainer README 外移规则 |
| 13 | `712235e` | 体检后实际只 5/14 文件不合规（Bag/HuntingArea/PVP/Warcraft/WarcraftShop），纯缩进格式化，逐文件 assert 语义一致；AutoFishing 本就合规，与 workbuddy 零冲突面 |
| — | `b7646b0` | claim 登记（首个 commit） |

### 待主控决策 / 后续动作

1. **替换 `agent/go-service.exe`**：新构建 8.8MB 在 `%TEMP%` 验证后已清理，
   未替换根目录运行时（按约定单独报告）。主控实机验证四项 sink（分辨率/HDR/进程/失败页）
   后，在本 worktree 跑 `data/maintainer/agent/go-service/build.bat` 产物落根，
   `git add -f agent/go-service.exe`，并更新 `assets_manifest.json` 中该项 sha256。
2. **workbuddy 926ade3 仍未合 main**：其 tasks/AutoFishing.json 删 5 行落在本分支
   已合规文件上，预期无冲突；合并时留意 fishing/param.go 与 README 与本分支第 6 点无交集。
3. 第 14 点白名单中 1600×900 / 2K / 4K 仅"逻辑通过"，实机拓档后再升级 README 承诺。

## 铁律修订建议

1. 新增 Go agent 源码目录后，除 EXCLUDE 实际覆盖外，按 rock-picker/fishing/go-service
   先例在 `build_release_zip.py` 的 `EXCLUDE_DIRS` 补**同名目录护栏**（防挪回 agent/ 泄进包）。
2. 大文件外移/入仓变更必须同步 `assets_manifest.json` 的 source 与 sha256，
   并跑一次 `bootstrap_assets.py --verify` 作为提交前检查。

---

# claim 续轮（2026-10-06）：LaunchGame 一键启动游戏 pretask

- **worktree**：`F:\MABd2-wt\traecode-launchgame`
- **分支**：`agent/traecode-launchgame`
- **基线**：`0fda9c7`（origin/main，含 workbuddy 钓鱼合并）

## 任务

脚本内置一键启动游戏：点开始后、客户端连接控制器前，自动检索本机棕 2 安装路径
并以窗口化 1920x1080 拉起游戏本体；游戏已运行则直接放行、由客户端按
class/title 搜索绑定。走 PI v2.7.0 pretask 协议（MXU v2.5.3 已支持），复用
go-service 单二进制双模式（上游 MaaEnd main.go 的 `--pretask` CLI 分派同款）。

## 改动文件范围

- `data/maintainer/agent/go-service/main.go`（--pretask 分派）
- `data/maintainer/agent/go-service/pretask/pretask.go`（新增，CLI 运行器）
- `data/maintainer/agent/go-service/pretask/launchgame/**`（新增 3 文件）
- `data/maintainer/agent/go-service/README.md`（文档同步）
- `tasks/pretasks/LaunchGame.json`（新增，pretask 声明，无 option）
- `misc/locales/zh_cn.json`（+2 键：label/description）
- 根 `README.md`（快速开始补一条一次性引导）
- `agent/go-service.exe`（重编替换）+ `data/maintainer/tools/assets_manifest.json`（sha256/size 同步）

## 热点占用

- `agent/go-service.exe`：**占用**（仅本分支重编一次，其他 worktree 冻结此文件）
- `interface.json`：**不碰**。请求主控在合并时给 `import` 数组追加一行
  `"tasks/pretasks/LaunchGame.json"`（JSON 片段见交工记录）
- 不碰：`MaaBd2.exe`、`maafw/**`、`resource/**`、pipeline、`.github/**`、`build_release_zip.py`

## 实机锁

本轮只做"游戏已运行"快速路径的 CLI 冒烟（无侵入）；真实拉起游戏的全量
验证矩阵由主控按串行铁律执行（场景清单见交工记录）。

## 交工记录（2026-10-06 完成，5 commit）

| commit | 内容 |
|---|---|
| `fd16441` | claim 登记（本章节上半部） |
| `bad7b04` | go-service 源码：main.go `--pretask` 分派 + `pretask/` 运行器 + `launchgame` 三链检索/等窗状态机 + README 第 7 节 |
| `893227b` | `tasks/pretasks/LaunchGame.json` + zh_cn 两键 + 根 README 快速开始第 4 条 |
| `833db68` | `agent/go-service.exe`（bd2-bad7b04，8966144B）+ manifest sha256/size 同步 |

验证：`go vet` 过；CLI 冒烟（游戏运行中）走 already-present 快速路径 exit 0，
未知任务名/缺参 exit 2；`bootstrap_assets.py --verify` 8 项全过。

### ⚠️ 待主控：合并时给 interface.json 的 import 数组追加一行

```json
"tasks/pretasks/LaunchGame.json"
```

### 实机验证矩阵（主控串行执行；游戏开/关之间注意串行铁律）

| # | 场景 | 预期 |
|---|---|---|
| 1 | 游戏已开，点开始（含 LaunchGame 卡片） | pretask 秒过（already present），任务正常绑定执行 |
| 2 | 游戏关闭、默认路径安装，点开始 | go-service 从启动器注册表命中路径，1080p 窗口化拉起，等客户区稳定后客户端搜窗接管 |
| 3 | 拖坏窗口尺寸（如 1078×602）再点开始 | pretask 等尺寸稳定（一直是坏尺寸也照常放行），aspectratio 警告页拦截（与现状一致） |
| 4 | 改名/移走 Neowiz 注册表键，游戏在默认路径 | 回退 default-location 链命中 |
| 5 | 三链全部找不到（如改盘符） | exit 1，仅 warning，任务继续跑但因无窗口失败；debug/go-service.log 有三条检索链日志 |
| 6 | 启动器卡更新页超过 120s | pretask exit 1 仅告警；游戏窗口最终出来后客户端仍能搜到 |
| 7 | 旧「前置程序」preAction 并存 | 两者行为兼容：先 preAction 后 pretask，游戏已开则两条都秒过；引导用户停用旧卡即可 |

### 已知限制（v1 基础版，拍板范围）

- pretask 卡片首次需在「添加任务」面板手动添加一次（协议无 default_check）；
- 直起 exe 绕过启动器的更新检查（与现行 preAction 填路径行为一致）；
- 1366×768 等小屏放不下 1080p 窗口，交给 aspectratio 警告页；
- 窗口尺寸主动救援（SetWindowPos）、Starter URI、MXU 上游 PR：二期。

## 补强（2026-10-06，路径排查后 A+B+C+D，2 commit）

实机验证（MXU 全链路，游戏开/关两场景）通过后，排查其他盘位 / 中文路径
场景，发现并补强：

| 点 | commit | 内容 |
|---|---|---|
| A | `403a3a4` | 默认路径链改 `GetLogicalDrives`+`GetDriveTypeW` 枚举全部固定/可移动盘（跳过光驱/网络/RAM 盘），消除硬编码 C..G 盲区 |
| B | `403a3a4` | 注册表 `execute` 为绝对路径时直接用，不再与 `path` 拼接 |
| C | `403a3a4` | 成功拉起后写 `config/launchgame.json` 记忆路径，平台三链全失败时兜底；失败不覆盖；文件被 .gitignore 忽略 |
| D | `403a3a4` | 卸载表链注释：实测棕2本体不登记卸载信息，唯一匹配条目是启动器，严格文件名校验防误启动 |
| — | `c163f52` | exe 重编（bd2-403a3a4，8972800B）+ manifest + 两份 README |

验证：go vet（windows）+ GOOS=linux 交叉编译过；临时单测覆盖记忆读写
6 分支（无文件/坏 JSON/路径不存在/中文目录命中/仅启动器名拒绝/往返）
通过后已删除未入库；真实拉起后记忆文件写入内容正确；
`bootstrap_assets.py --verify` 8 项全过。

中文路径结论：注册表/文件/进程三环节全走 W 系列宽字符 API，中文无损
（中文目录实机实证：进程拉起、日志写入均正常）。

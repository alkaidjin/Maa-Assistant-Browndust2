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

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

## 铁律修订建议

- （施工中如有再追加）

## 交工记录

- （施工中）

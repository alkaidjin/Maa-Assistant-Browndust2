# go-service —— MaaFramework Agent 守护进程（裁剪 fork）

`agent/go-service.exe` 是 MXU 经 `interface.json` 的 `agent` 配置拉起的守护进程，
负责注册自定义识别 / 动作 / 事件 sink，并驻留为 MaaFramework Agent Server。
本目录是它的**裁剪后源码**，来源为上游 MaaEnd，许可证 AGPL-3.0（与仓库根 LICENSE 同源）。

> **源码入仓库、不入用户包**：与 `fishing/`、`rock-picker/` 同样，本目录被打包器整体
> 排除，只有编译产物 `agent/go-service.exe` 进发布包。

---

## 1. 为什么要裁剪

此前 `agent/go-service.exe`（15.8 MB）是**纯黑盒预编译产物**：仓库里没有源码、
没有构建脚本，只能信任上游。对其做 `go version -m` 得到的事实：

| 项 | 值 |
|---|---|
| 上游模块 | `github.com/MaaXYZ/MaaEnd/agent/go-service` |
| 精确版本 | commit `64fac11ae5b0cba2d802311841011cb99f1c99d1`（v2 分支，2026-09-14，"chore: Auto Templates Optimization"） |
| 构建环境 | go1.25.6 / `CGO_ENABLED=0` / `-trimpath` / `vcs.modified=false`（未改动源码的干净构建） |
| 框架 | `github.com/MaaXYZ/maa-framework-go/v4@v4.0.0-beta.18` |

上游 `register.go` 注册约 **50 个终末地业务组件**；逐一比对本仓
`resource/pipeline/**`、`tasks/**` 后确认，棕 2 实际只触及其中 **6 个**。
于是把这 6 个组件及其依赖闭包原样搬入本目录，删掉其余全部业务包 ——
黑盒变白盒，可审计、可离线复现构建。

## 2. 保留了什么（6 个组件 + 依赖闭包）

| 组件 | 挂载方式 | 棕 2 用途 |
|---|---|---|
| `common/schedule` | 自定义识别 `ScheduleRecognition` | 星期门控：`WarcraftShop`、pvp 吸收召集等 3 处 |
| `taskersink/aspectratio` | tasker sink 自动挂载 | 16:9 分辨率守护，不符则弹 HTML 警告并阻止任务（见第 14 点分辨率文档） |
| `taskersink/hdrcheck` | tasker sink 自动挂载 | Windows HDR 状态守护 |
| `taskersink/processcheck` | tasker sink 自动挂载 | 黑名单进程守护 + HTML 警告页 |
| `taskersink/taskfail` | tasker sink 自动挂载 | 任务失败反馈引导（issues 链接已改指本仓） |
| `pkg/resource` | resource sink | 资源路径修正 |

支撑包（依赖闭包，非组件）：`pkg/{i18n,parentwatch,pienv,control,maafocus,jsonclean}`、
`pretask/gamesetting`。共 14 个包、40 个 `.go` 文件。

i18n 文案**不 embed**，运行时从磁盘 `locales/go-service/*.json` 读取（简繁英日韩
5 语言的守护警告页模板，原样保留在仓库根 `locales/go-service/`）。

## 3. 刻意保留的两处「上游原状」

1. **`taskersink/cursormove`（光标归位）在上游 `64fac11` 即被裸 `return` 禁用**，
   随仓旧 exe 同样没有挂载它。裁剪版保持"注册函数为空操作"，`sink.go` 完整实现
   留作将来上游恢复时直接启用。
2. **`pretask/gamesetting` 是终末地专用的注册表 / HDR 设置代码**
   （`Software\Hypergryph\Endfield`、`Endfield.exe`）。`aspectratio` 在判定全屏时
   会调用它交叉读取注册表；棕 2 机器上这些键不存在，读取失败即走 fallback
   （客户区实测），不改变分辨率判定语义。为遵守"不改判定语义"的拍板原则原样保留，
   它是被动代码，不会自行写注册表。

## 4. 与旧黑盒 exe 的等价性

- `go vet ./...` 通过（Go 1.25.6）；
- 本机构建产物 **8.8 MB**（旧 15.8 MB，体积降 44%，差异来自删掉的 40+ 终末地组件）；
- 冒烟运行（工作目录为仓库根、加载真实 `maafw/`）日志依次确认：
  MaaFramework 初始化 → i18n 载入 24 条 `zh_cn` 文案 → resource sink 注册
  → 6 组件注册 → `Agent server started`，无 stderr；
- 分辨率 / HDR / 进程 / 任务失败四个 sink 的弹窗与拦截属运行时行为，
  **需实机游戏验证**（见第 14 点分辨率文档）。

## 5. 构建

双击 `build.bat`（需要 Go 1.25.x；查找顺序：`GOROOT` → `PATH` →
`<仓>\cache\_gotool\go`）。产物直接写到仓库根 `agent/go-service.exe`，
`main.Version` 注入 `bd2-<git短SHA>`。

```bat
cd data\maintainer\agent\go-service
build.bat
```

构建后需 `git add -f agent/go-service.exe`（仓库 `.gitignore` 默认忽略 `*.exe`）。
首次构建若本地 module cache 不全，会经 `goproxy.cn` 拉取依赖。

## 6. 将来如何重新同步上游

1. 取 MaaEnd v2 分支目标 commit，重新核对 `register.go` 组件清单与本仓
   `resource/pipeline/**` 中 `custom_recognition/custom_action/custom_sink` 的引用；
2. 按第 2 节清单更新包目录（依赖闭包用 `goimports` / `go mod tidy` 收敛）；
3. 保持 module 名 `go-service`（与 `fishing`/`rock-picker` 的短模块名惯例一致）；
4. 构建 + 冒烟 + 实机验证四项 sink 后，`git add -f` 替换产物。

# bd2-territory 实现逻辑梳理 + 在 BD2MAA(MAA) 中的复现可行性

- 上游仓库：https://github.com/MadestSamurai/bd2-territory （MIT，当前版本 0.4.1）
- 本地克隆：`F:\MABd2v26.09.5\cache\_ref\bd2-territory`（`--depth 50`，含全部源码）
- 代码规模：约 7.2k 行 C#（hook 层 2.6k + 兼容层 + shared 算法 + WPF 桌面端 + IPC）
- 梳理日期：2026-09-30

---

## 0. 一句话结论

**它不是"图像识别 + 模拟点击"脚本，而是一个注入 Unity 进程的白盒机器人**：用 Mono.Cecil 现场解析客户端 `Assembly-CSharp.dll`，生成一份游戏内部 API 绑定表，Roslyn 现场编译一个 hook 程序集，用 SharpMonoInjector 打进游戏进程，之后所有"看"和"做"都走游戏自己的对象树、NavMesh、物理引擎和网络回执。

因此：**它的核心能力（目标枚举 / 3D 定位 / 寻路 / 成熟判定 / 采集结算）全部依赖进程内反射，用 MAA 的黑盒 pipeline（截图 + 模板匹配 + Win32 键鼠）无法等价复现。** 能复现的只有"纯 2D UI 面板"那一层，而且前提是玩家已经手动把角色放到位、面板已经打开。

---

## 1. 整体架构

```
[WPF 桌面端 desktop/]                       [游戏进程内 hook/]
 settings.json / control.json  ── 命名管道/文件 ──►  Loader.Load()
 latest.json / runtime.json    ◄──────────────────  RuntimeEngine.Tick()
 layout-*.json（布局工具）
        │
        │ 连接阶段
        ▼
[compatibility/]  Mono.Cecil 读 Assembly-CSharp.dll
        │  BindingResolver：按"契约"匹配类型/成员（抗混淆）
        │  MetadataIndex：形状签名 / 方法体哈希 / 引用集合打分
        ▼
   HookCompiler：Roslyn 现场编译 → TerritoryClient.g.cs（100 个 API 的 MetadataToken 表）
        ▼
   SharpMonoInjector.Inject(payload, "BD2Territory.Runtime", "Loader", "Load")
        ▼
   游戏进程内：AssemblyResolve 加载内嵌 0Harmony.dll → Harmony patch 生效
```

四个进程内Harmony patch 点是全部能力的支点：

| Patch 点 | 作用 |
| --- | --- |
| `GameCameraManager.LateUpdate` (postfix) | 把引擎挂到游戏主循环，每帧 → 100ms 节流 Tick |
| `Gather.CanGather` (postfix) | 强制"只打完全成熟的"，误伤幼树时不掉血 |
| `Gather.Near` (postfix) | 接管自动瞄准，锁定工具自己选定的目标 |
| `NetworkManager.Send` (prefix) + 6 个 protobuf 回执回调 (postfix) | **只观测不改包**：记录请求、对账回执、防重复付款 |

---

## 2. 兼容性层（最值钱的部分，也是最大维护成本）

不绑定任何发布日期的客户端，每次连接现场解析：

1. `MetadataIndex`：Mono.Cecil 打开 `BrownDust II_Data/Managed/Assembly-CSharp.dll`，建立类型/成员索引。
2. `contract.json`（**580 KB、35 个类型、100 个 API 角色、6 个枚举角色**）描述"我要哪些能力"。
3. `BindingResolver` 三级匹配：
   - 未混淆 → 按 FullName 直接定位；
   - 已混淆 → 按**类型形状签名**（字段/方法签名拼串）匹配；
   - 形状也变了 → 用**方法体哈希锚点**（`expected.Anchors`）打分，要求命中 ≥2 个不变方法体；
   - 成员级：签名一致 + **方法体哈希 10 分 + 引用集合交集 ×100** 打分，唯一才算数；**不唯一直接报 unsupported 拒绝猜测**。
4. `HookCompiler.ValidateRuntimeEntryPoints`：注入前逐条做 IL 级断言（枚举常量值、`Player.Face` 必须调用 `UpdateRotatePlayer(true)`、`SetMoveNav` 必须走原生完整路径检查、stepOffset 字段要从 `CharacterController.get_stepOffset` 的 `stfld` 反查出来……）。任一不满足 → **"尚未注入"直接退出**，不做半吊子适配。
5. 通过后 Roslyn 现场编译 hook（Release/X64/deterministic），把解析结果写成 `TerritoryClient.g.cs`（`TypeNames` / `MemberNames` / `Apis[role] = {类型, MetadataToken, method}`）。
6. 运行期 `TerritoryBindings` 全部走 `MetadataToken` 反射；`ValidateCompiledClient()` 用 **Module Mvid** 校验"客户端没在我眼皮底下换掉"，换了就要求重启游戏重连。

**这就是它的护城河**：把"游戏更新就失效"从"每次都要改代码"变成"每次连接重新解析 + 大量自愈锚点"，失败时优雅拒绝。

---

## 3. 运行时：一个 100ms 的状态机

`RuntimeEngine.Tick()`（挂在 LateUpdate 上，10Hz）：

```
读 control.json（租约：OwnerId + pid + 心跳，10s 过期）→ 校验账号/场景/UI 就绪
→ 弹窗处理（升级/新物品自动确认，其它弹窗一律等待）
→ AwaitGathering（工具动作是否在跑）
→ 农田阶段机 farmStage 1..4（打开面板→读原生批量预览→校验→提交→等回执）
→ 售卖 / 料理 / 紧急余量
→ 移动中？ContinueWalk
→ 间隔到了才选下一个目标：SelectResource
→ workCycle.Advance 决定当前阶段 → Harvesting / Planting / Gathering / Processing
```

### 3.1 感知（每 1s 全场景扫描，弹窗期 100ms）

- `FindObjectsOfType<LifeGatheringObject>()` → 树/矿/作物三类（`Gather.Function` 1/2/3）
- 成熟判定 = `CanGatheringState() && 当前阶段 == FinalStage && Hp > 0`
- 农田：`LifeFarmFieldObject` + 服务器世界缓存 `LifeWorldObjectDBInfo` 双源交叉，用 `FarmEmptyProgress` 做**多证据投票**（DB 存在 / InnerObject.Status / 原生 Occupied / 存活作物 / `IsPreviewCandidateAvailable` / 网络回执），避免"看错了把种着的地当空地重种"
- 目标身份 `ResourceKey` = `chunk:parentId:x:y:gatherId:index`（跨刷新稳定，不是 InstanceID）

### 3.2 工作循环（防"永远采不完"）

`TerritoryWorkCycle`：`Idle → Harvesting → Planting → Gathering → Processing → Reset`
每轮开始时**冻结目标集合**（`UnionWith`），之后只用 `IntersectWith` 收缩；新刷新的资源进不了本轮 → 不会无限拖延种植。

### 3.3 目标选择

- 距离优先（作物 -2m 加权），每第 3 次选择强制让给"逾期未采"目标（防矿石被树/作物饿死）
- 失败记忆 `TargetFailureMemory`：15s / 30s / 60s 退避，2 分钟强制复核，成功即清零
- 空挥检测：`GatheringProgress` 观察 Hp 是否下降，无进展 → 换位（最多 2 次）→ 进重试队列

### 3.4 导航（双通道）

- **默认 A\***：`LocalRouteSearch` 增量 A*（每帧只扩 256 节点 / 6ms，避免卡顿），网格 0.4m→0.4m 以下细化；边可行性的判定是真物理查询：`Physics.RaycastAll` 找地面 + `OverlapCapsule`/`CapsuleCastAll` 用**玩家 CharacterController 的胶囊参数**扫障碍，按 `slopeLimit` / `stepOffset` / `skinWidth` 判断是否可上台阶（`TraversalRules`）。
- **可选 NavMesh**：直接调游戏原生 `Player.SetMoveNav`（默认关闭）。
- **地形**：`GroundChunk.GetCellType` → `Ground / Water`，水面禁止通行，**只认游戏原生的桥 gate**（`LifePlaceableObject_ColliderGate` + `HasWaterCell`），桥中心线作为 A* 的额外引导点。
- **缓存**：`RouteMemo`（地面 65k / 边 131k / 可行 65k）+ 按"几何变化戳"做区域失效（设施挪动、资源被采走才失效），空闲时 `WarmLocalGrid` 预热。
- **脱困**：物理阻挡 → 局部重规划；3 秒无进展 → 短距冲刺（8 方向采样，选离终点最近的落点）；远距离 → 载具；都有预算上限（90s / 6 次）后进重试队列。

### 3.5 采集执行

1. `interaction.Decide`：到位 + 被游戏检测到 + 成熟 → `UseTool`
2. 转向：`Player.Face`（切 CharController 模式避免被导航方向覆盖）
3. 发工具事件：从 `Tool.Event` 方法的 IL 里 `isinst` 反解出事件类型 → `Activator.CreateInstance(type, group, toolId)` → 反射调用 `LifePlayerToolEquipmentController` 的事件入口。**不模拟按键。**
4. 等 `Tool.Busy` 自然结束 + 等服务器 `LifeWorldObjectGatheringResponse`；45s 无进展 → 暂停。

### 3.6 播种（最谨慎的一段，防重复扣费）

```
走近农田 → Farm.Open 打开原生播种面板 → 选中种子 → 强制开"批量播种"开关
→ 读游戏原生批量预览（FarmGroup.Preview）拿到实际格数 keys
→ 校验：必须是同一作物 + 全是原定空田
→ 预算/余额检查（不足 → 整片跳过 60s，去处理别的）
→ 点 _cropButtonObj → 点 _completeButtonObj
→ 校验确认弹窗里的 seed 列表（数量、种类必须一致）
→ 生成 token → 先落盘 progress-<account>.json（含 PendingToken/Keys/Cost）→ 再点确认
→ 等 LifeSeedingResponse，用 token 对账：请求与预览是否一致 / 是否被拒 / 是否真种上
→ 结果未知 → 保留记录并暂停，绝不重试
```
掉线重连后 `ReconcilePending()` 用世界数据反查上次那批，全中才算完成，否则**保留进度不动**（宁可不种，不多扣一次钱）。

### 3.7 网络回执层

Harmony patch `NetworkManager.Send` + 6 个回执回调（`LifeSeeding/Gathering/PlaceSave/PositionSave/Cooking/ShopSell`）。
只做三件事：克隆请求入 pending 队列、回调时解析 protobuf、**与本地持久化的意图 token 对账**。明确注释：**不替换包、不伪造请求、不改奖励**。30s 无响应 → 结果未知 → 暂停并保留记录。

### 3.8 料理 / 售卖 / 布局

- 料理：读 `LifeCookTable` 配方，按"库存 + 地里预计产量"平衡 5:3:2 之类的配比，每批上限 1–1000，容量不足就跳过；同样 token 对账。
- 售卖：读 `LifeSellItemTable` 过滤可售领地物，库存 > 阈值（100–9900）才卖差额，锁定物品/建筑/装备不碰；先收完本批作物再卖，避免打断采集。
- 布局：模板/JSON 导入 → 预检占地与费用（优先复用已有设施移动，只补买差额）→ 逐处摆放等待回读 → 导入前保存备份。

---

## 4. 与 MAA 的能力对照（关键）

| 能力 | bd2-territory 的实现 | MAA 黑盒能否替代 |
| --- | --- | --- |
| 枚举领地里的树/矿/作物 | `FindObjectsOfType` 拿全场景对象 + transform 坐标 | ❌ 3D 自由视角，模板匹配无从下手 |
| 判断是否成熟 | `CanGatheringState` + `FinalStage` + Hp | ❌ 只能靠外观，且受光照/遮挡/模型差异影响 |
| 走到目标前 | NavMesh + 自研 A* + 物理胶囊扫障 | ❌ MAA 没有 SLAM/场景理解 |
| 挥工具采集 | 直接构造并派发游戏事件对象 | ⚠️ 只能靠键鼠模拟（需先实机确认交互键位） |
| 采集是否成功 | 网络回执 protobuf 对账 | ⚠️ 只能靠画面变化猜测 |
| 空田判定 | 服务器世界缓存 + 原生预览多证据投票 | ❌ |
| 批量播种 | 开原生面板 → 读预览 → 校验 → 提交 | ✅ **面板是 2D 的**，可模板匹配 |
| 料理 / 售卖 | 2D UI + 表格数据 | ✅ **可模板匹配**（前提：面板已打开） |
| 升级/新物品弹窗 | 识别 UI 类型后调原生确认 | ✅ 可模板匹配 |
| 布局购买/导入 | 走游戏编辑模式 API | ❌ |

**结论：跑图与目标决策这一整层（占其代码量 70%+）在黑盒下不可复现。**

---

## 5. 三条复现路线（按 ROI 排序）

### 路线 B：只做"领地 2D 流程自动化" —— 推荐先做试点（低风险，1–3 天）

在现有 pipeline 里加一个可选的 `Territory` 任务，只覆盖**玩家已经站好位、面板已经打开**之后的固定流程：
- 播种面板：选种子 → 开批量 → 确认 → 等结算弹窗
- 料理：选配方 → 批量制作 → 确认
- 售卖：超量物品出售
- 升级/新物品弹窗自动确认

前置验证（必须先实机确认，半天）：
1. 走近成熟资源后，屏幕是否出现**固定位置**的交互按钮？有没有固定形态的成熟标识？
2. 有没有**原生批量收获**入口？（从契约看 0.4.1 没有；若有，黑盒价值大幅上升）
3. 农田/料理/商店面板在 1920×1080 窗口化下的分辨率稳定性。

风险低，不需要改项目性质，失败也不影响现有任务。

### 路线 A：把白盒能力做成 BD2MAA 的第三个 agent（高价值，高成本，高风险）

**技术上可行，而且你已经具备接入点**：`interface.json` 里已有 `agent` 数组（`agent/go-service`、`agent/rock-picker`），MaaFramework 的 `MaaAgentBinary / MaaAgentClient.dll / MaaAgentServer.dll` 都在 `maafw/` 下。也就是说你可以加一个 `agent/territory`，由 pipeline 的 custom recognition / custom action 驱动。

要做的事：
1. 移植/复用其 `compatibility/`（MIT，可并入 AGPL-3.0 项目，但须保留 MIT 声明 + 更新 `NOTICE.md`）
2. 把 hook 层从"独立 WPF 程序 + 文件 IPC"改成"MaaAgent 子进程 + 管道协议"，暴露 `gather / harvest / plant / cook / sell` 几个 action
3. `tasks/Territory.json` + `resource/pipeline/Territory.json` + `interface.json.import` 三处同步（项目铁律）

代价与风险（必须先看清楚）：
- **体积与构建**：Roslyn + Mono.Cecil + Harmony 会让包体积显著增大，构建链要引入 .NET 8 SDK（你现在的打包脚本是纯 Python + git 无关的 os.walk 排除表，新增文件要同步进排除表）
- **维护**：100 个 API / 35 个类型 / 580 KB 契约，游戏大更新时解析可能失败（作者的自愈率不是 100%，失败就是"连接不上"）。这是长期负债，不是一次性工作。
- **权限与分发**：必须与游戏同权限（游戏若以管理员运行，工具也要管理员）；注入器类程序容易被杀软/游戏反作弊点名，**封号风险比纯视觉高一档**；这也会改变 BD2MAA 目前"Win32 窗口控制"的产品定位与免责范围。
- **与 go-service 共存**：分辨率/HDR/进程守护由 go-service 负责，注入 agent 要避免与之抢窗口、抢输入。

### 路线 C：不做，只做信息跟踪 —— 成本 0
等游戏官方出批量收获/一键采集，届时黑盒价值重估。

---

## 6. 建议顺序

1. **先花半天做路线 B 的前置验证**（实机确认交互按钮 / 批量收获 / 面板分辨率），这一步几乎零成本，结论直接决定 B 值不值得做。
2. B 通过 → 用现有 pipeline 做一个"领地辅助"任务（播种/料理/售卖/弹窗），作为可选任务发布。
3. A 只在"用户呼声很高 + 你愿意长期承担契约维护与风险"时启动；启动时优先做成**独立可选组件**（只在使用时才注入），不要污染主流程。

---

## 7. 附：值得抄的工程设计（与是否注入无关）

即使只做路线 B，下面几条也是好东西，可以直接借到 MAA pipeline 里：

- **租约 + 心跳**：控制文件带 `OwnerId + pid + 时间戳`，10s 无心跳即失效；防止旧实例/双开抢操作。
- **先落盘再提交，用 token 对账**：任何不可逆操作（付款/播种/售卖）先把意图写盘，回执按 token 匹配；结果未知 → 保留记录并暂停，绝不重试。MAA 里做领地/商店类流程同样适用（可用 `debug/` 下的状态文件模拟）。
- **目标集合每轮冻结**：防止"边采边刷"导致任务永远结束不了。
- **失败退避 15/30/60s + 2 分钟强制复核**：避免某个够不到的目标把整个流程卡死。
- **多证据投票判定状态**：单一信号（画面/缓存）容易误判，交叉验证后才动手。

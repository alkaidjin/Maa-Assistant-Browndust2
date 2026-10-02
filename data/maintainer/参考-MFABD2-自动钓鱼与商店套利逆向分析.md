# MFABD2 自动钓鱼与商店套利逆向分析

> 分析对象：`sunyink/MFABD2` main，提交 `ee02dfe91cdf489cc2a8d7092e38f1e8c8766dc5`（2026-09-29）  
> 本地快照：`cache/MFABD2-upstream/`  
> 下载归档：`cache/MFABD2-main.zip`，SHA-256 `be9c7d294f0c36284403200fe496bc3ed0fda36bbae8dd81b4765256be0766d6`

## 结论先行

1. **这两个功能都不是单靠 Pipeline JSON 完成的。**
   - 自动钓鱼：Pipeline 负责导航、抛竿/上钩入口和卖鱼；Python `FishingAction` 负责逐帧截图、判断游标、预测点击时机和循环计数。
   - 商店套利：Pipeline 有 468 个节点，但核心决策仍在 Python Agent；包括行情 OCR、满价判定、采购收藏对齐、料理/库存计算、精确调数、金币差额验单和持久化。
2. **MFABD2 当前正式任务不支持 PC 客户端。** `assets/interface.json:284-305` 中，商店套利和自动钓鱼都只声明 `Adb`、`PlayCover`。`assets/resource/pc/pipeline/Fishing.json` 是未接入正式入口的旧 PC 实现，不能当现成可用方案。
3. **本项目当前已经有一份未提交的钓鱼 PoC**：`tasks/Fishing.json`、`resource/pipeline/Fishing.json`，并已在 `interface.json` 中导入；但它目前不能进入 Auto 模式：
   - `FishingMinigame` 没有被 `agent/go-service.exe` 或 `agent/rock-picker.exe` 注册；
   - `resource/image/Fish/Sell_ico.png`、`resource/image/Fish/back.png` 缺失；
   - 默认 `TestSkip` 仅用于验证导航。
4. **最佳路线不是把 MFABD2 整包硬拷进来。** 推荐先完成现有 PC 钓鱼 PoC，再做“行情扫描 + 峰值料理出售”的套利 MVP；采购、制作、补买最后接。直接全量搬入会立刻遇到 25 个外部节点缺失、Python 运行时、PC 资源不适配和打包器排除规则等问题。

---

## 1. 源码获取情况

直接执行 `git clone --depth 1` 两次都因 GitHub 连接重置失败；随后改用 GitHub codeload 获取同一 main 快照并解压：

- 解压目录：`cache/MFABD2-upstream/`（约 20 MiB）
- 原始 ZIP：`cache/MFABD2-main.zip`（约 12 MiB）
- main 提交：`ee02dfe91cdf489cc2a8d7092e38f1e8c8766dc5`
- ZIP SHA-256：`be9c7d294f0c36284403200fe496bc3ed0fda36bbae8dd81b4765256be0766d6`

这是**完整源码快照，但不含 `.git` 历史**。仓库 `.gitmodules` 仍声明 `assets/MaaCommonAssets`，当前 main 树里已无实际 gitlink；构建流程改为从 `sunyink/MFABD2-Assets` 另行合并资源。

静态检查结果：

- `agent/` 全量 `compileall` 通过；
- `Fishing.json`：44 个节点，2 种 CustomAction；
- `Arbitrage.json`：468 个节点，28 种 CustomAction、8 种 CustomRecognition；
- 自动钓鱼 + `arbitrage_*.py` 共 22 个核心 Python 文件、约 6442 行；完整依赖还包括通用动作、持久化、名称归一和 OCR 辅助模块。

---

## 2. 自动钓鱼怎么实现

### 2.1 分层

| 层 | 文件 | 职责 |
|---|---|---|
| UI | `assets/interface.json:301-311,2028-2082` | 选择钓点、目标条数；入口 `Fishing_Entry` |
| Pipeline | `assets/resource/base/pipeline/Fishing.json` | 导航、识别钓鱼 UI、抛竿、等待上钩、卖鱼 |
| 实时控制 | `agent/fishing_agent.py` | 逐帧读取游标/蓝区/黄区，预测点击，循环计数，定期卖鱼 |
| 平台覆盖 | `assets/resource/playcover/pipeline/Fishing.json` | iOS 蓄力抛竿、卖鱼入口和阈值覆盖 |
| 旧 PC 原型 | `assets/resource/pc/pipeline/Fishing.json` | Space 抛竿、ColorMatch 拉锯；当前没有接入正式任务 |

`agent/main.py` 通过 `import fishing_agent` 触发装饰器注册：

```python
@AgentServer.custom_action("FishingAction")
class FishingAction(CustomAction):
    ...
```

Pipeline 对应节点：

```json
"Custom_Fishing_Task": {
  "action": "Custom",
  "custom_action": "FishingAction",
  "custom_action_param": {"max_count": 100}
}
```

### 2.2 主流程

```text
Fishing_Entry
  → 经营管理/钓鱼入口
  → 选择烟波湖、浅岸或寒霜海峡
  → 启航
  → Fishing_Start
  → FishingAction
      ├─ run_task("Casting_Rod")
      │    └─ 抛竿 → Detect_Took_Bait
      ├─ 连续截图并识别白游标、蓝区、黄区
      ├─ 预测到达时机，优先黄区，其次蓝区
      ├─ 点击收杆并结算
      ├─ 每 N 条 run_task("SellFish_Start")
      └─ 达到 max_count / max_seconds / 外部停止后退出
```

### 2.3 预测算法

关键参数在 `Fishing.json:24-64` 的 disabled 配置节点 `attach` 中：

- 游标速度：`4.2 px/frame`
- 蓝区单边收缩：`0.83 px/frame`
- 游标单程：`88 frame`
- 基准帧率：`60 FPS`
- 单局上限：`17 s`
- 点击提前量：`0.27 s`
- 输入补偿：`0.045 s`
- 卖鱼间隔：默认 30 条

运行时每帧读取：

- `Rec_FishMinigame_Cursor_Clr`：白游标
- `Rec_FishMinigame_BlueZone_Clr`：蓝区
- `Rec_FishMinigame_YellowZone_Clr`：黄区

核心计算：

```text
方向 = floor((当前时间 - 本轮开始时间) × 60) 在 176 帧周期中的位置
到达时间 = 路径距离 / 4.2 / 60
蓝区预测宽度 = 当前宽度 - 2 × 0.83 × 到达帧数
实际等待 = 到达时间 - 本帧分析耗时 - 0.045
```

若目标在游标运动反方向，会把“先到边界再反弹”的距离算进去。黄区优先；黄区赶不上或蓝区将在点击前消失，则退到蓝区。

### 2.4 卖鱼可靠性

`fishing_agent.py:844-891` 不只看 `run_task` 返回成功，还检查轨迹中是否真的出现 `SellFish_End`。只有完整走到退栈点才清零计数，否则保留鱼获数量，下轮继续重试。

### 2.5 已知问题

- `Casting_Rod` 与 `Move_Forward` 构成自环；`max_seconds` 只能在轮间检查，拦不住一次 `run_task` 内部永久空转。
- 方向依赖假定 60 FPS；一旦掉帧，方向和点击时机可能持续偏移。
- Python 文件头写“自动缩放到 1920×1080”，实际代码没有自行缩放；坐标基准仍是 1280×720。
- 正式 task 没有 PC controller；旧 PC JSON 是死链，不能直接宣称 PC 已支持。

### 2.6 在本项目中复现

本项目现有未提交 PoC 已完成 UI/导航/卖鱼骨架，但需补齐：

1. 自己截取并加入：
   - `resource/image/Fish/Sell_ico.png`
   - `resource/image/Fish/back.png`
2. 增加 PC 专用识别节点：抛竿状态、咬钩、游标、蓝区、黄区；不能直接假设 ADB/PlayCover 阈值在 FramePool 下有效。
3. 新增 Go CustomAction `FishingMinigame`，建议放到独立第三个 Agent，而不是塞进职责单一的 `rock-picker`。
4. Go 动作内部完成完整循环和定期卖鱼；`defer` 保证 Space/鼠标始终释放，避免中止后按键悬挂。
5. 测试顺序：`Win32-Window(PrintWindow)` → 默认 `Win32(FramePool)` → `Win32-Front(DXGI)`；三种截图后端分别标定 HSV 和输入延迟。
6. 验证后再把 `FishingMode` 默认值从 `TestSkip` 改为 `Auto`。

**建议不要直接复活 MFABD2 的旧 PC JSON。** 可借用其 ROI 和 KeyDown/KeyUp 思路，但它没有接入现行入口，也没有目标条数、周期卖鱼和可靠停止语义。

---

## 3. 商店套利怎么实现

### 3.1 五阶段状态机

`agent/action/arbitrage_flow.py:22-23` 固定顺序：

```text
① 开局出售峰值料理
② 按采购名单低价购入材料
③ 用现有材料做料理；可补查库存、缺料补买并补做
④ 出售本轮及既有峰值料理
⑤ 出售超过保留量的材料（默认关闭）
```

入口 `Arbitrage_Start` 先由 `ArbitrageStagePrepare` 读取五个开关，动态改写执行路由；全部关闭就直接结束。总枢纽 `Arbitrage_Action_Hub` 顺序 JumpBack 五阶段，并在前后保存库存位置 A/B。

### 3.2 “最高价”判定

价目表按四个窄列 OCR：

| 节点 | ROI | 内容 |
|---|---:|---|
| `Arbitrage_Sell_Col_Name` | `[478,209,252,344]` | 商品名 |
| `Arbitrage_Sell_Col_Amount` | `[817,209,79,344]` | 当前金额、月峰值金额 |
| `Arbitrage_Sell_Col_Price` | `[893,209,67,344]` | 当前/峰值溢价率 |
| `Arbitrage_Sell_Col_Cart` | `[960,209,102,344]` | 当前/月度卡带 |

实际生效的最终判据位于 `agent/utils/arbitrage_pricelist.py:139-146`：

```python
is_max_price = current_price == peak_price
```

前提是两个金额都已确认且为正整数；金额不完整或观察冲突时一律不卖。文件里存在“金额缺一侧时回退倍率交集”的 `_max_price_verdict`，但当前无调用点，属于实现与注释不一致的保守漏卖问题。

OCR 不是只读一次：

- 名称先做简繁归一和候选重读；
- 金额/倍率必须通过比例一致性校验；
- 冲突行使用白字像素范围生成有界局部 ROI 重读；
- 候选笛卡尔积最多 64 组，只有唯一一致解才采信；
- 当前卡带类型和编号分开取证，编号要求两个不同纵向裁剪位置形成共识。

### 3.3 采购

1. 从默认采购表和用户逐卡带自定义表生成最终名单；
2. 逐卡带进入商店，OCR 商品名并检测收藏星；
3. 目标+灰星 → 点亮，非目标+黄星 → 取消；“天赋神药”强制取消；
4. 每张卡带只有名称与星唯一配对成功才记为已对齐；
5. 最后执行游戏内“一键购买全部收藏”；
6. 用金币减少确认发生了成交。

风险：常规采购只确认“金币减少”，没有逐件验证是否以预期折扣价买入；砍价未生效时理论上可能按原价成交。

### 3.4 料理、补买与利润闸门

- 制作队列从 Pipeline 料理 Entry 及模板目录动态发现；
- 常规制作后只补查真正涉及的食材库存；
- “未读到”绝不当成 0，库存状态区分 `known` / `unknown`；
- 目前只处理**恰好缺 1 种材料**的配方，多种材料同时缺失则跳过；
- 折扣价按 `原价 × 40%` 向下取整；
- 每份基准多赚：

```text
料理峰值售价 - Σ(原料数量 × 原料峰值售价) - 料理等级 × 天赋神药价格
```

- 补买溢价默认最多让出基准增益的 10%；同时受预算、钱包金币、商店余量、单价上限和日切限制；
- 精确购买通过 MIN 读单价、MAX 读真实上限、二分/微调到目标数量；成交后必须由两帧稳定金币读数证明扣款额等于报价。

### 3.5 出售

1. 从今日完整行情筛出：名称确认、卡带确认、当前金额等于峰值金额的商品；
2. 根据料理/材料保留策略算可卖量；
3. 动态注入目标卡带、商品名/模板和当前溢价率；
4. 进入出售页，MIN→MAX→二分/微调；
5. 售前记录金币，售后要求金币增加额与本批报价精确一致；
6. 未确认成交只允许一次补试，随后停止该物品，避免重复出售；
7. 账号变化、UTC 日期变化、页面无法恢复时立即停止当前套利任务。

### 3.6 持久化

- `agent_shared_data.json`：跨账号共享的每日行情；
- `agent_save_data.json` / `agent_save_data_<账号>.json`：每账号库存、收藏对齐和周期记录；
- 写入使用临时文件 + `os.replace` 原子替换；
- 读取失败时降级只读，避免“读空后把真存档覆盖掉”；
- 行情解析器版本为 6，只接受 `complete + names_complete + prices_complete` 的今日快照；
- 当前 `market_day()` 使用 UTC 日期，代码已注明尚未核实游戏实际换日时间。

### 3.7 规模与依赖

`Arbitrage.json` 直接依赖：

- 468 个节点；
- 28 种 CustomAction；
- 8 种 CustomRecognition；
- 49 个直接模板引用；
- 49 张料理模板、63 张材料模板、69 张背包模板；
- 25 个跨文件外部节点，而这些节点在本项目当前 Pipeline 中全部不存在。

因此它不是“复制一个 JSON + 一个 Python”即可运行的功能。

---

## 4. 与本项目的兼容性

| 项 | 本项目 | MFABD2 | 结论 |
|---|---|---|---|
| MaaFramework | v5.13.0 | v5.12.x（MFAA v2.15.2） | API 代际接近，但仍需真加载与回归 |
| UI 协议 | PI v2，task 拆到 `tasks/*.json` | PI v2，单体 `assets/interface.json` | 要拆分迁移，不能整份覆盖 |
| Agent | 两个 Go exe，agent 数组 | 单个 Python Agent | MXU 支持追加第三 Agent；运行时需另处理 |
| 坐标基准 | 截图短边 720，即 1280×720 | 1280×720 | 数值可参考，不代表模板/颜色可复用 |
| PC 后端 | FramePool / PrintWindow / DXGI | 正式任务仅 ADB / PlayCover | 必须做 PC 实机标定 |
| OCR | PP-OCRv5 已齐 | OCR + Python 二次校验 | 模型可复用 |
| 任务接线 | `tasks/X.json` + `resource/pipeline/X.json` + `interface.import` | 单体 interface + 多资源层 | 必须遵守本项目三处同步 |

静态对比：

- Fishing 的 44 个节点与本项目无重名，但 6 个外部节点在本项目不存在；
- Arbitrage 的 468 个节点与本项目无重名，但 25 个外部节点在本项目不存在；
- MFABD2 的 28+8 个自定义注册名与现有两个 Agent 二进制没有发现重名；
- 当前钓鱼 PoC 是后来新增的未提交文件，不属于上述“上游节点与原项目节点”对比基线。

---

## 5. 推荐复现路线

### 方案 A：原 Python Agent 侧挂（最快复现，推荐用于验证算法）

新增第三个 Agent：

```json
{
  "child_exec": "{PROJECT_DIR}/python/python.exe",
  "child_args": ["-u", "-X", "utf8=1", "{PROJECT_DIR}/agent/main.py"],
  "timeout": 15000,
  "identifier": "mxu_browndust_mfabd2",
  "auto_reconnect": true
}
```

运行时建议按上游打包策略：Windows x64 使用 CPython 3.10.11 embedded，安装与当前内核一致的 `maafw==5.13.0`、`numpy<2`、Pillow、requests、pytz、loguru、json-with-comments。

优点：最接近原行为，套利 6000+ 行逻辑无需重写。  
缺点：包体明显增加；多一套运行时与依赖；本项目打包器当前按**目录名**递归排除任何 `config/cache/debug` 目录，嵌入式 Python 包里如果出现同名目录会被静默裁掉，必须先把排除规则改成“只排根级路径”。

### 方案 B：全部改写成 Go Agent（长期维护最合适）

新建第三个 Go Agent，例如：

```text
开发源码：data/maintainer/agent/bd2-actions/
发布运行件：agent/bd2-actions.exe
```

Go 绑定已具备：

- `maa.AgentServerRegisterCustomAction(name, runner)`
- `Context.RunTask(entry, override...)`
- `Context.RunRecognition(...)`
- `Context.RunRecognitionDirect(...)`
- `Context.OverridePipeline(...)`
- `Context.SetAnchor(...)`
- `Context.ClearHitCount(...)`
- Controller 的截图、点击、滑动、KeyDown/KeyUp、TouchDown/TouchUp

优点：维持当前“纯 exe、无 Python 环境”的发行形态。  
缺点：钓鱼可控；全量套利移植工作很大，必须连同状态机和回归测试一起重写，不能按函数逐个机械翻译。

### 方案 C：分阶段 MVP（推荐的实际交付顺序）

#### 阶段 1：把现有钓鱼 PoC 做到可运行

1. 补两张自有截图模板，先消除当前资源加载风险；
2. 实现 `FishingMinigame` Go CustomAction；
3. 在 `Win32-Window` 上标定，再迁移到默认 FramePool；
4. 给抛竿等待、小游戏、卖鱼分别加硬超时；
5. 验证 10 条、30 条、手动停止、鱼包接近满四种场景；
6. 通过后才把默认模式改为 Auto。

#### 阶段 2：套利 MVP = 扫行情 + 峰值料理出售

只保留：

```text
导航到商人
→ 打开价目表并按溢价率降序
→ 四列 OCR + current_price == peak_price
→ 找到料理、调数量、出售
→ 金币增加额验单
→ 保存今日行情与账号库存
```

先不做：收藏采购、料理制作、背包仓检、缺料补买、材料出售。这样可以把上游约 468 节点压到约 100 个，并避开约 97 个“购买页→出售页恢复”的复杂节点。

#### 阶段 3：依次补齐

1. ②收藏对齐与一键购买；
2. ③料理制作；
3. 背包仓检；
4. 单缺料补买和补做；
5. ⑤材料保留策略与出售。

---

## 6. 运行与验收门槛

### 自动钓鱼

- 资源加载无 missing template；
- Agent 日志出现 `FishingMinigame` 注册成功；
- 手动停止后没有 Space/鼠标按下残留；
- 10 条模式能按真实成功数停止，而不是按尝试数；
- 中途卖鱼后能回船继续；
- `SellFish_End` 未出现时不能清零鱼获计数；
- 三种控制器后端分别记录颜色阈值和输入延迟。

### 商店套利

- 行情不完整时绝不出售；
- 商品名、当前金额、峰值金额、当前卡带任一冲突时绝不出售；
- 交易前后账号一致、日期一致；
- 售前/售后金币差额与报价精确相等才记账；
- 未知成交不重复派发；
- 所有购买/出售都有预算、数量和页恢复硬上界；
- 状态写入必须原子化，读失败只读降级；
- 先做 dry-run：只输出“计划买/卖什么、数量、依据”，不点击确认。

---

## 7. 当前工作区立即要注意的事

当前存在这些**非本次分析创建**的未提交改动：

- `interface.json`：新增 `tasks/Fishing.json` import；
- `tasks/Fishing.json`：新文件；
- `resource/pipeline/Fishing.json`：新文件。

静态检查发现当前唯一两处缺失模板引用：

```text
resource/image/Fish/Sell_ico.png
resource/image/Fish/back.png
```

并且现有两个 Agent 均未包含 `FishingMinigame` 注册名。因此：

- `TestSkip` 只能用于导航验证；
- `Auto` 当前必然失败；
- 如果 MaaFramework 对缺图采用资源加载阶段强校验，这两张图可能让整个资源包加载失败，需先补图或临时禁用对应模板节点。

本次只做源码快照、逆向和静态检查，**没有改动上述业务文件，也没有启动游戏做实机验证**。

---

## 8. 许可证与素材边界

MFABD2 的 LICENSE 采用逐文件规则：v1.1.0 之前创建/修改的文件为 MIT，v1.1.0 及以后为 Apache-2.0。当前钓鱼与套利核心文件最近修改均在 2026 年，按其仓库规则应按 Apache-2.0 处理。

Apache-2.0 与 GPLv3 兼容；本项目为 AGPL-3.0，组合发布通常可行，但必须：

- 保留 Apache-2.0 许可证副本；
- 保留适用的版权、专利、商标和署名通知；
- 在修改过的文件中显著说明修改；
- 若上游包含 NOTICE，复制适用的 NOTICE 内容；
- 不冒用 `MFABD2` 名称或图标暗示官方关系。

上游 `TRADEMARKS.md` 明确：第三方游戏素材与图标不因代码开源许可证获得授权。最稳妥做法是**自行从本项目支持的 PC 客户端重新截图制作模板**，只复用算法和代码，并在 `NOTICE.md` 署名来源。

> 本节是工程合规建议，不构成法律意见。

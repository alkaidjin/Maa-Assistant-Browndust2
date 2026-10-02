# 参考：MFABD2「自动钓鱼」与「商店套利」逆向 + 本项目移植方案

> 上游快照：`cache/MFABD2-upstream/`（commit `ee02dfe9`，2026-09-29）
> 对照对象：本项目 `F:/MABd2v26.09.5`（MaaFW v5.13.0 / MXU v2.5.3 / AGPL-3.0）
> 坐标空间：上游与本项目**同为 1280×720**（本项目 `screencap_target_short_side: 720`），坐标可直接搬，但**时序常量必须重标**。

---

## 0. 快照与合规

| 项 | 值 |
|---|---|
| 仓库 | `sunyink/MFABD2`（自称 MaaBD2） |
| 快照 commit | `ee02dfe9...` |
| zip sha256 | `be9c7d294f0c36284403200fe496bc3ed0fda36bbae8dd81b4765256be0766d6` |
| 本地路径 | `cache/MFABD2-upstream/`（20 MB，含 `.git` 已剥离） |

**许可证（逐文件判定，见根目录 `LICENSE` / `LICENSE-MIT` / `LICENSE-APACHE`）**

- v1.1.0 **之前**的文件 → MIT
- v1.1.0 **及之后**的文件 → Apache-2.0
- Apache-2.0 官方 FAQ 原文：「compatible with version 3 of the GPL」，不兼容 GPLv2。
  → **可以并入本项目的 AGPL-3.0**，前提是：
  1. 随包带上 `LICENSE-APACHE` 全文；
  2. `NOTICE.md` 增列 MFABD2 归属与"本文件修改自 MFABD2"声明；
  3. 对被修改过的文件头部标注修改；
  4. 不复用其图标 / 第三方素材，不冒用 MFABD2 名义（`TRADEMARKS.md` 明确禁止）。
- ⚠️ 需要移植的 4 个**数据文件**（`bd2_item_names_i18n.json` / `bd2_name_norm_tw2cn.json` / `arbitrage_shop_catalog.json` / `arbitrage_replenish.json`）含大量游戏内物品名，属于事实性数据，但同样要走上面 1~3 步。

---

## 1. 自动钓鱼

### 1.1 任务定义（`assets/interface.json`）

```json
{"name": "[半自动]自动钓鱼", "entry": "Fishing_Entry",
 "controller": ["Adb", "PlayCover"],          // ← 明确排除 PC 客户端
 "option": ["钓鱼地点", "目标次数"]}
```

- `钓鱼地点`（select）：三个 case 都只 `pipeline_override` 一个键——`Fishing_MapNavigate.next` → `Fishing_EnterFishingSpot_Lake / _Shallow / _Frost`。
- `目标次数`（input，默认 100）：`Custom_Fishing_Task.custom_action_param.max_count = "{目标钓鱼条数}"`。

### 1.2 文件构成

| 文件 | 作用 |
|---|---|
| `assets/resource/base/pipeline/Fishing.json` | 42 节点：导航 + 卖鱼全在这里 |
| `assets/resource/pc/pipeline/Fishing.json` | 19 个**上一代 PC 遗留节点（死代码）** + 2 个有效覆盖 |
| `assets/resource/playcover/pipeline/Fishing.json` | iOS 覆盖（134 行） |
| `agent/fishing_agent.py` | 小游戏实时闭环 + 唯一的 CustomAction |
| `agent/main.py` | 一行 `import fishing_agent` 触发注册 |
| `docs/zh_cn/钓鱼模块开发说明.md` | 权威设计说明（含死代码清单） |

### 1.3 Pipeline 骨架（base）

```
Fishing_Entry(anchor MainMenu_GoFishing=Fishing_InFishingNet)
  → Fishing_InFishingNet / Fishing_InMap / Fishing_MapNavigate（空壳，option 覆盖 next）
  → Fishing_EnterFishingSpot_{Lake|Shallow|Frost}
  → Fishing_SetSail
  → Fishing_Start  Or(Rec_Fishing_UI_Tpl, Rec_Fishing_UI_Ocr)  post_wait_freezes 800
        next: ["[JumpBack]SellFish_Start", "Custom_Fishing_Task"]
  → Custom_Fishing_Task   action=Custom / FishingAction / {max_count:100}
```

抛竿与前进（**这是整套里唯一会自环的地方**）：

```json
"Casting_Rod":   {"action":"LongPress","target":[1130,570],"duration":100,
                  "rate_limit":800,"next":["Detect_Took_Bait"],"on_error":["Move_Forward"]},
"Move_Forward":  {"action":"Swipe","begin":[230,580],"end":[230,620],"duration":1000,
                  "next":["Casting_Rod"]},
"Detect_Took_Bait": {"recognition":"TemplateMatch","roi":[626,185,25,53],
                     "template":"Fishing_took_bait.png","threshold":0.6,
                     "action":"Click","target":[1130,570],"timeout":6000}
```

卖鱼链：`SellFish_Start`（`max_hit:1`，TemplateMatch `Fish/Sell_ico.png` roi `[802,11,92,74]`）→ `SellFish_Shop` → … → `SellFish_End`（`Fish/back.png` roi `[120,20,39,43]`）。

### 1.4 小游戏实时闭环（核心，无法用 Pipeline 表达）

Pipeline 只能回答"是/否"，而钓鱼要逐帧预测"再过多久点"，所以整段交给 Python。

**三色 ColorMatch（同一条进度条 ROI `[480,602,383,20]`，整体判据区 `minigame_area=(335,505,600,154)`）**

```json
"Rec_FishMinigame_Cursor_Clr": {"recognition":"ColorMatch","roi":[480,602,383,20],
    "method":40, "lower":[0,0,240], "upper":[180,30,255], "count":15}
```
游标（白）/ 蓝区 / 黄区（暴击）三路分别取 `box`，得到各自的 x 区间。

**预测模型（纯解析，不读游戏内存）**

```
frames_needed   = total_distance / cursor_speed
time_needed     = frames_needed / ref_fps
adjusted_wait   = wait_time - elapsed - input_comp      # ← 关键一行
if adjusted_wait > 0: sleep(adjusted_wait)
tap(cast_rod)                                            # (1130, 570)
```
蓝区从两端向中心收缩：`shrink_distance = blue_shrink * frames_needed`；
蓝区归零时间：`frames_to_zero = distance_to_center / blue_shrink`。

**参数表（`Fishing_Minigame_Data` / `_Timing` / `_Strategy`，均为 `enabled:false` 的配置节点，改 JSON 不动 py）**

| 组 | 键 | 值 | 含义 |
|---|---|---|---|
| Data | `cursor_speed` | 4.2 | 游标速度 px/帧 |
| Data | `blue_shrink` | 0.83 | 蓝区单边收缩 px/帧 |
| Data | `cursor_half_cycle` | 88 | 单程帧数（往返 = 2×） |
| Data | `ref_fps` | 60.0 | 上两项"每帧"所依据的帧率 |
| Data | `minigame_seconds` | 17 | 单局墙钟上界 |
| Timing | `click_lead` | 0.27 | 点击提前量 s |
| Timing | `input_comp` | 0.045 | 算完→点击生效的链路延迟 s |
| Timing | `cursor_reset_wait` / `after_cast` / `after_catch` | 0.6 / 0.2 / 3.0 |  |
| Strategy | `sell_interval` | 30 | 几条卖一次（iOS 覆盖 25）|
| Strategy | `wait_cap` | 5.0 | 单次等待上限 s，超时弃点 |
| Strategy | `bar_wait_cap` / `blue_min_width` / `end_invalid_frames` | 4.0 / 5 / 3 |  |

### 1.5 卖鱼与"防假成功"

```python
if "SellFish_End" not in nodes:
    print("error: ❌ 卖鱼未走完出售流程（节点轨迹: ...），鱼获保留，下条鱼后重试")
    return
```
`run_task` 返回成功只证明框架跑完，不代表成交 → 必须回看本次任务的**节点轨迹**里有没有真正的结束节点。

外层 `run()` 另有 `deadline = monotonic() + max_seconds`（默认 `max(120, max_count*120)`）与 `finally` 兜底卖鱼，用来解 `Casting_Rod ↔ Move_Forward` 自环。

### 1.6 三端差异与死代码

- **iOS（PlayCover）**：`Casting_Rod` 换成 custom action `HoldCastGreen`——`post_touch_down/up` 长按 + numpy 手写 BGR→HSV + 圆环 annulus 掩码判"蓄力环变绿"（`green_px_min=800`、`max_hold=10s`）；连续 `no_green_rescue_streak=3` 次没抓到绿就触发卖鱼自救。`SellFish_Start.max_hit` 改成 **9999**（注释强调 0 不是无限）。
- **PC**：`controller` 里**根本没有 PC 客户端**，pc 包只剩 2 个有效覆盖。文件里那 19 个节点（`Fishing_Cast_down`=KeyDown 32 / `Fishing_Cast_Up`=KeyUp 32 / `Fishing_Bite` 纯蓝 roi `[1156,521,4,3]` / `Fishing_TugOfWar_A1_Yow|A1_Blu|A2_Yow|A2_Blu` / `Hand_1Y|1B|2Y|2B` / `err_32`）是**上一代"拉锯战"写法，现行链路零引用**。
- 死代码：`long_press/swipe/wait_for_fish`、`TimingCfg.wait_fish_interval/input_delay`、`CoordCfg.minigame_area`、`Detect_CountDown*`、`Reco_Minigame_Total_Time`、`pc/image/UI_BoatToDock.png`。
- 已知文档错误：`fishing_agent.py` 头部注释写"1920×1080 / runtime scaling"——**是错的**，实际 1280×720 且无缩放逻辑。

---

## 2. 跑商套利（`✨商店套利 Opus`）

### 2.1 任务与选项

```json
{"name":"[执行]商店套利","label":"✨商店套利 Opus","entry":"Arbitrage_Start",
 "controller":["Adb","PlayCover"],
 "option":["开局出售料理","利润商品-低价进货","利润料理-制作","利润商品-最高价抛售","出售多余材料"]}
```
五个开关全部是 `switch`，case 只改 `pipeline_override` 里对应阶段入口的 `enabled`：

| 选项 | 覆盖节点 | 默认 |
|---|---|---|
| ① 开局出售料理 | `Arbitrage_PreSell_Entry.enabled` | Yes |
| ② 低价进货 | `Arbitrage_BuyItem.enabled` | Yes |
| ③ 料理制作 | `Arbitrage_Cooking.enabled` | Yes |
| ④ 最高价抛售 | `Arbitrage_SellItem.enabled` | Yes |
| ⑤ 出售多余材料 | `Arbitrage_SellMaterials.enabled` | **No** |

② 的子项：`购买收藏扫描（每周|每次）`、`收藏未核实仍继续购买`、`自定义卡带采购`；
③ 的子项：`料理自选模式`、`料理稀有材料保护`、`料理制作周期`（`CheckCoolDown cycle_type:g_weekly`）、`料理缺料补买`；
⑤ 的子项：`材料出售模式`（`off/auto/manual`，auto 时 `days=max(输入,7)`）。

### 2.2 五阶段 + 一个 Hub

```python
STAGES = ("Arbitrage_PreSell_Entry", "Arbitrage_BuyItem", "Arbitrage_Cooking",
          "Arbitrage_SellItem", "Arbitrage_SellMaterials")
```

- `Arbitrage_Start` → custom action **`ArbitrageStagePrepare`**：读出哪些阶段 enabled，然后把 `Arbitrage_Start.next` 覆写成 `[JumpBack]Arbitrage_Archive_Begin + route`；全关则 `next: []`。
- `Arbitrage_Action_Hub`（custom action `ClearNodeHitCount`）：依次 JumpBack 调 ①~⑤ → 关商店 → 回箱庭 → `Arbitrage_Archive_End`（`InventoryArchive` 记库存位置 B）。
- 每个阶段入口自己也是 `ClearNodeHitCount`，清 `Arbitrage_GlobalMarket_Ensure` / `Arbitrage_Sale_EnsureShop` 的计数，配合它们的 `max_hit:1` 实现"阶段内只准备一次、回跳不重复"。

### 2.3 行情：共享快照 + 最高价判据

- `ArbitrageMarketEnsure`：若 `store.get_market_snapshot()` 非空则复用；否则 `context.clone()` → `SetAnchor("Replenish_ShopEntry","Arbitrage_Merchant_NoDiscount_Entry")` → `ensure_shop` → `run_task("Arbitrage_GlobalMarket_Entry")`，最后校验轨迹里出现过 `Arbitrage_PriceList_Egress`，否则报错。
- 扫描动作就是同一个 `ArbitrageSellController` 换 mode：
  - `Arbitrage_GlobalMarket_Scan` → `mode: preview_all`（`max_scan_pages: 80`）
  - `Arbitrage_PreSell_Run` → `mode: preview_possess`
  - `Arbitrage_ShopSell_Active` → `mode: sell, sale_scope: recipes`
  - `Arbitrage_ShopSell_Materials` → `mode: sell, sale_scope: materials`
- **最高价的最终判据是金额，不是倍率**：
  ```python
  def confirm_prices(row):
      if row["price_read_basis"] not in ("unconfirmed_price","observation_conflict") \
         and current_price>0 and peak_price>0:
          row.update(is_max_price = current_price == peak_price, max_price_basis="amount")
      else:
          row.update(is_max_price=False, max_price_basis="unconfirmed_amount")
  ```
  只有两侧金额都没读到时才回退 `rate_fallback`（`current_rate == peak_rate`）。
- 价目表每个商品占**两行**：上子行 = 今日行情，下子行 = 月峰值。断界**纯几何**（名锚 y 分带），刻意不读"当前/每月"文字——因为繁/简/英/日各异，曾是硬编码语义依赖的坑。

### 2.4 价目表 OCR：四列窄 ROI + 局部救援

```json
"Arbitrage_Sell_Col_Name":   {"roi":[478,209,252,344]},
"Arbitrage_Sell_Col_Amount": {"roi":[817,209, 79,344], "replace":[["[,，.．。、·]",""]]},
"Arbitrage_Sell_Col_Price":  {"roi":[893,209, 67,344]},
"Arbitrage_Sell_Col_Cart":   {"roi":[960,209,102,344]}
```
> 窄 ROI 的原因写在注释里：卡带尾号是小字，整表 OCR 会漏检；分列后一放大就认得出。

- `read_prices()`：先常规 OCR；只有**自相矛盾的行**才进局部救援（`Arbitrage_Sell_Amount_Rescue`），且救援候选只在"本行图像里真实读到的值"里做笛卡尔积枚举（≤64 组），唯一解才接受，否则整行标 `unconfirmed_price` 并把冲突字段置 `None`。
- 矛盾判据 `price_issues()`：`peak < current` / `peak_rate < rate` / `|peak*rate - current*peak_rate| > tol*(rate+peak_rate)` / 与 `base_price*rate/100` 不符。
- **卡带救援** `Arbitrage_Sell_Cart_RescueNum`（`only_rec:true`）：类型用正则（`[剧劇]情[游遊][戏戲]卡` / `角色…` / `活[动動]…` / `店[长長]…`），编号按 `number_ranges` 校验（story 1-19 / character 1-7 / event 1-7 / manager 空）。**至少两个不同裁剪读到相同编号才接受**，9/19 这类分歧不按置信度选胜者。
- 翻页：`Agt_PriceList_Swip`（`SmartAction` 代理，py 独占，勿挂进管路）+ `Arbitrage_Swip_Calibration_{Down,Up,Up_FineTuning}`（ColorMatch 校准 4 行价格纵向对齐，`max_hit:1`）。

### 2.5 采购（②）：收藏对齐 + 精确补买

- `Arbitrage_Buy_ListPrepare` → `Arbitrage_Buy_Open` → `Arbitrage_Buy_Button` → `Arbitrage_Buy_CheckCycle_Entry`（识别 `ArbitrageBuyRecordReady`）→ `Arbitrage_Buy_CheckCycle`（`CheckCoolDown`）→ `Arbitrage_Buy_ScanPrepare` → `Arbitrage_Buy_ToSelect`（识别 `ArbitrageBuyNeedsScan`）。
- 卡带选择 `Arbitrage_Buy_Select_QC1..19 / QR1..7 / QE1..5`（每个 + `_Clr` 用 ColorMatch 二次确认），共 31 个柜台。
- `Arbitrage_ShopBuy_Pack_S1..S19/R1..R7/E1..E5`（31 个）→ custom action **`ShopBuyFavController`**：核对并写收藏。
- `Arbitrage_PreciseBuy_Entry` → **`ArbitrageBuyController`**（`arbitrage_buy_precise.py`）：MIN 定单价、MAX 读实际上限，`purchase_limit = min(target, available, 99999, gold//unit, budget//unit)`；`verify_purchase()` 用**两帧金币扣减**记账。状态机：`prepared/confirmed/partial/skipped/not_found/rejected/stale/unknown`。
- 撤销语义：`invalidate_purchase_alignments(cartridges)` 全扫前一次撤销旧清单——避免"部分点星后失败仍复用旧成功记录"。只有核对成功的回调才 `save_purchase_alignment`。

### 2.6 料理与缺料补买（③）

- `Arbitrage_Cooking`（`ArbitrageCookingPrepare`）→ `Arbitrage_Cooking_Run` → `Arbitrage_Cooking_MenuPatch`(`CookingInventoryBoundary`) → 逐道菜 `Arbitrage_Cooking_{A1..A10, B6..B21}_Entry` → `SubMenu` → `NubMenu_Max` → `NubMenu_Doing` → `Doing_Starus`；缺料走 `Arbitrage_Cooking_MaterialInsufficient`（`CookingStockSnapshot`）。
- 稀有材料保护：`Arbitrage_Cooking_MateProtec_{B17,B14,B10,A2}`（4 个 `PatchPipeline`）。
- **仓检**：`Arbitrage_BagStockScan`（`ArbitrageBagPrepare`）→ `Arbitrage_BagStockScan_Open` 或 `_Record`；`BagStockScan` 扫背包，排序节点 `Arbitrage_BagStockScan_Sort_*`。
- **补买利润模型**（`arbitrage_replenish_profit.select_purchase`，`DEFAULT_PROFIT_SURRENDER_PERCENT = 10`）：
  ```
  约束：100 * 溢价 <= percent * 每份基准多赚 * n
  即   100*(needed_per*unit_premium*n + intercept) <= percent*base_gain*n
  ```
  在价格档端点与比例边界（±1）枚举 `n`，取 `gain` 最大；越界返回 `profit_surrender_exceeded`。`revalidate_requests()` 按实际成交重分配未尝试的合规供给（可扩列，但不扩大原料理目标、不重复进同柜台）。

### 2.7 出售链与金币核验（①④⑤）

```
Arbitrage_Sell_HUB → Sell_Button_Ocr → Sell_Button_Clr → Sell_ResetPack_Enter → Sell_PackList
 → Sell_PackShopSwich（切卡带，expected 由 py 注入）→ Sell_Item_Prepare
 → ItemList_Sorting_*（排序反转，CheckCoolDown gating）→ Sell_Item_ListTraverse
 → Sell_Gold_Snapshot(GoldSnapshot) → Sell_Item_Click → Sell_Item_SellMenu
 → Arbitrage_Sell_Item_Price_MaxCheck（expected = f"{current_rate}%"）
 → Sell_Item_Quantity(ArbitrageSellQuantity) → Sell_Item_Selling → Sell_Item_Sold
 → Sell_Item_AfterClick → { AmountConfirmed(ArbitrageSaleConfirmed+GoldVerdict)
                          , AmountMismatch(ArbitrageSaleMismatch) }
```
- 商品匹配：`_sell_item_override()` 注入 OCR 全名（custom recognition `OCRItemName`）→ 未命中才追加模板 `Agt_<Sell_Item>_Tmp`。注释强调：**名字纠偏必须跑在 target 过滤之前**，否则原生 OCR 的混排/单字候选会被 `expected` 直接丢掉。
- **金币核验（2026-08-06 改）**：主控不再跨整条链自测金币，改由链内 A/B 两点测量——`GoldSnapshot`（卖前）与 `GoldVerdict`（卖后），`gold_proof` 取 `raw_detail.before/after/delta`。判据："两帧稳定金币 + 与报价精确相符"才记账，且**不重开商品页**。
- `_verdict_rank()` 只升不降：`2(卖成) > 1(确凿没卖成) > 0(读数缺一端) > -1(B 没跑)`——防止后一个空结论抹掉前一个确凿结论。
- `_task_ok(detail)` 单列成函数：因为 `TaskDetail` 对象**恒为真值**，直接 `if detail` 只测得出"提没提上去"。上游为此栽过两次。

### 2.8 存储分层

| 存储 | 内容 | 位置 |
|---|---|---|
| `SharedStore` | `arbitrage.market.days[UTC日期]`（`parser_version=6`，保留 62 天） | 全局共享 |
| `PersistentStore` | `arbitrage.inventory`（`items / latest / observations / events`）与 `purchase_alignments` | **按账号隔离** |

- `market_day()` 用 UTC 日期分桶（注释承认"商店实际换日时刻尚待核实"，所以交易仍须子页核价）。
- `save_market_snapshot()`：**完整缓存不会被失败重扫覆盖**（`previous_usable and not fresh_usable` 才保留旧值）。
- 库存铁律：**"没扫到绝不补 0 / 绝不抹掉精确数量"**。`quantity_status` 分 `known/unknown`。价目表只证明"存在"（`present=True`），没有数量。

### 2.9 材料保留策略（`arbitrage_material_policy`）

`mode` ∈ `off/auto/manual`；auto 下 `days = max(输入, 7)`，`scale_reserve = daily * days`；`-1` 表示无限保留（乘 0 仍是 -1）。保留量**不限制 ③ 制作消耗**。

### 2.10 规模数字（决定工作量的关键）

- `assets/resource/base/pipeline/Arbitrage.json`：**468 个节点**，272 KB
- `agent/action/arbitrage_*.py` 等 ≈ 12 个模块（仅 `arbitrage_result.py` 就 51 KB）
- `agent/utils/arbitrage_*.py` ≈ 11 个模块
- 数据文件：`bd2_item_names_i18n.json` 104 KB（`category=="Recipe"` 是判断"哪些是料理"的唯一依据）、`arbitrage_replenish.json` 61 KB、`bd2_name_norm_tw2cn.json` 19 KB、`arbitrage_shop_catalog.json` 15 KB
- 依赖上游公共节点：`Global_ToHomePage / Global_BackPageHub_Once / Global_ToSandBox / Global_WaitingForLoading / Collect_FindTeleporCircle* / PVP_PerPetTask / Setup_SetSkill_Per / PractiseBargaining_Per / Merchant_Bypass` 等——**本项目一个都没有**。

---

## 3. 在 `F:/MABd2v26.09.5` 复现

### 3.1 已验证：能力全部具备（这是好消息）

| 需要 | 现状 | 结论 |
|---|---|---|
| Custom Action | `MaaAgentServer.dll` 导出 `MaaAgentServerRegisterCustomAction`；Go 绑定 `maa.AgentServerRegisterCustomAction(name string, action CustomActionRunner) error` | ✅ **可用，无需 Python** |
| Custom Action 入参 | `CustomActionArg{TaskID, CurrentTaskName, CustomActionName, CustomActionParam, RecognitionDetail, Box}` | ✅ |
| 逐帧取像素（钓鱼必需） | `Controller.PostScreencap() *Job`（`Job.Wait()`）+ `Controller.CacheImage()` / `CacheImageInto(dst *image.RGBA)` | ✅ 可零拷贝复用 buffer |
| 点击 / 长按 / 触摸 | `PostClick / PostTouchDown / PostTouchMove / PostTouchUp / PostSwipe / PostKeyDown / PostKeyUp` | ✅ |
| 跑子任务 | `Context.RunTask(entry, override...)` | ✅ |
| 识别 | `Context.RunRecognition(entry, img, override...)` / `RunRecognitionDirect(...)`；`RecognitionResults{All,Best,Filtered}`，`OCRResult{Box,Text,Score}` | ✅ |
| 动态改链 | `Context.OverridePipeline / OverrideNext / OverrideImage / SetAnchor / Clone / ClearHitCount` | ✅ |
| 回看节点轨迹（防假成功必需） | `Tasker.GetTaskDetail(id) → TaskDetail{Nodes []NodeRef}`，`NodeRef.GetDetail() → NodeDetail{Name,...}` | ✅ |
| 停止信号 | `Tasker.Stopping()` | ✅ |
| 坐标空间 | 上游 1280×720 = 本项目 `screencap_target_short_side:720` | ✅ |
| 节点名冲突 | 现有 406 节点 × 上游 468+42 = **交集为空** | ✅ 可直接搬 |
| OCR 模型 | `resource/` 已有 2 个 `.onnx` | ✅ |

### 3.2 剩下的阻塞项

| # | 阻塞 | 严重度 | 说明 |
|---|---|---|---|
| B1 | **时序常量绑定 60 FPS** | 🔴 高 | `cursor_speed=4.2px/帧 / blue_shrink=0.83px/帧 / cursor_half_cycle=88帧 / ref_fps=60`。本项目 Win32 用 FramePool/PrintWindow/DXGI，实测帧率远不是 60 → 照搬必然整体算错。**必须重标或改模型** |
| B2 | **上游公共节点缺失** | 🔴 高（套利）/ 🟡 中（钓鱼） | `Global_* / Collect_* / Setup_* / PVP_PerPetTask` 等在本项目不存在，需映射到现有"回主页/传送/返回"节点或重建 |
| B3 | **上游钓鱼不含 PC 通路** | 🔴 高（钓鱼） | 上游 `controller:["Adb","PlayCover"]`；pc 包是死代码。Win32 全靠自己实现 |
| B4 | **持久化行情/库存** | 🟡 中（套利） | 本项目无 `PersistentStore`。建议落 `config/arbitrage/`（运行时目录，MXU 全量更新不动它；别放 `data/`） |
| B5 | **ColorMatch 阈值与模板** | 🟡 中 | 三色阈值需在 Win32 截图下重取；模板必须自己截图（上游图受 `TRADEMARKS.md` 约束 + 来源设备不同） |
| B6 | **打包白名单** | 🟡 中 | 新增 `agent/*.exe` 必须进 `build_release_zip.py` 的 `REQUIRED_FILES`；agent 源码目录必须进 `EXCLUDE_DIRS`（打包器 os.walk 非 git 驱动） |
| B7 | **三处同步铁律** | 🟡 中 | `tasks/X.json` + `resource/pipeline/X.json` + `interface.json.import` |

### 3.3 钓鱼移植路径（推荐做法）

1. **新建 Go agent** `data/maintainer/agent/fishing/`（照抄 `rock-picker/` 的骨架：`resolveLibDir` / `socketID=os.Args[len-1]` / `AgentServerRegisterCustomAction("FishingAction", …)`），产出 `agent/fishing.exe`。
2. **小游戏循环**（Go 直写，不依赖 OpenCV）：
   ```
   ctrl := ctx.GetTasker().GetController()
   for {
     ctrl.PostScreencap().Wait()
     img, _ := ctrl.CacheImageInto(buf)        // 复用 RGBA buffer，避免每帧分配
     cur, blue, yellow := scanBar(img)          // 在 ROI 内按 BGR 阈值扫像素，取各自 x 区间
     ...预测...
     ctrl.PostClick(1130, 570)
   }
   ```
3. **⚠️ 强烈建议改掉 FPS 假设**（这是本项目相对于上游最大的改进点）：
   不要再用 `ref_fps=60`，改成**实测两次截图的墙钟差 `dt`，直接算 `px/ms`**：
   ```
   v_px_per_ms = (x2 - x1) / (t2 - t1)        // 连续两帧测游标位移
   shrink_px_per_ms = (w1 - w2) / (t2 - t1)   // 连续两帧测蓝区宽度
   ```
   这样 FramePool 20fps 还是 DXGI 60fps 都自动适配，`cursor_speed/blue_shrink/ref_fps` 三个常量退化成"首帧初值"。`input_comp` 保留（它测的是"算完→点击生效"的链路延迟，与帧率无关）。
4. **Pipeline**：新建 `resource/pipeline/Fishing.json`（`Fishing_Entry` / `Casting_Rod` / `Move_Forward` / `Detect_Took_Bait` / `Fishing_Start` / `Custom_Fishing_Task` / `SellFish_*`）。`Fishing_Entry` 要挂到本项目现有的回主页入口，不要照搬上游的 `Daily_Busin_Hub_Post`。
5. **任务**：`tasks/Fishing.json` + `interface.json.import` 追加；选项用 `pipeline_override`（`Fishing_MapNavigate.next` / `Custom_Fishing_Task.custom_action_param.max_count`）。
6. **防假成功**：Go 侧 `tasker.GetTaskDetail(arg.TaskID)` → 遍历 `Nodes` → `GetDetail().Name` 里找结束节点（等价于上游的 `"SellFish_End" not in nodes`）。
7. **卖鱼节奏**：`sell_interval`（默认 30）与 `max_seconds` 墙钟兜底保留；`WaitFreezes` 可用 `ctx.WaitFreezes(...)`。

### 3.4 套利移植路径（ROI 从高到低，建议分批）

| 批次 | 内容 | 需要的上游模块 | 价值 |
|---|---|---|---|
| **P0** | 只读行情 + 卖峰值料理（①的 preview_possess + ④的 sell/recipes） | `arbitrage_result.py` 的扫描/判价/派发 + `arbitrage_pricelist.py` + `arbitrage_cartridge.py` + `arbitrage_quote.py` + `arbitrage_sell_batch.py` + `arbitrage_sell_quantity.py` + `gold_verify.py` + 4 个数据文件 | 最高：这就是"跑商套利"的核心收益，且**不碰采购/烹饪，风险最低** |
| **P1** | 低价进货（②） | `arbitrage_buy_list.py` + `shop_buy_fav_controller.py` + `arbitrage_buy_precise.py` | 高：补齐"低买"闭环 |
| **P2** | 料理制作 + 缺料补买（③） | `cooking_stock.py` + `bag_stock.py` + `arbitrage_replenish*.py` + `arbitrage_replenish_profit.py` + `arbitrage_material_policy.py` | 中：收益放大，但节点最多（A1..A10 / B6..B21 / 仓检 / 排序） |
| **P3** | 卖多余材料 + 保留策略（⑤） | `read_material_reserve_policy` | 低：默认关闭，收益有限 |

**P0 的最小可用切片**（建议先做到这一步就发版试跑）：
- 进店 → 开价目表 → `PriceList_Sort` 按溢价率排序 → 校准滚动 → 四列窄 ROI OCR → 卡带 OCR + 尾号救援 → `is_max_price` 判定 → 对命中项走 `Sell_HUB` 出售链 → `GoldSnapshot/GoldVerdict` 核验 → 退出。
- 行情落 `config/arbitrage/market-<UTC日期>.json`，`parser_version` 自己定 1，保留 62 天。
- 先**不做** `SharedStore/PersistentStore` 的账号隔离（本项目单实例，可后补）。

### 3.5 打包与仓库卫生（每次新增文件必做）

1. `data/maintainer/tools/build_release_zip.py`：
   - `REQUIRED_FILES`（L282-345）追加 `agent/fishing.exe`（及后续 `agent/arbitrage.exe`）；
   - `EXCLUDE_DIRS`（L91）追加 `data/maintainer/agent/fishing`、`data/maintainer/agent/arbitrage`——**源码不进包**；
   - `PRUNED_BINARIES`（L176）**不要动**（`MaaAdbControlUnit.dll` 已裁，且 ADB 本来就不在包内）。
2. `git add -f agent/*.exe`（`.gitignore` 忽略 `*.exe`，`git mv` 搬家时容易掉出 index）。
3. 三处同步：`tasks/X.json` + `resource/pipeline/X.json` + `interface.json.import`。
4. `interface.json.agent` 追加：`{"child_exec":"agent/fishing","identifier":"mxu_browndust_fish","timeout":15000,"auto_reconnect":true}`。
5. **跨文件重名节点 = 整个资源加载失败**——新节点名一律带 `Arbitrage_` / `Fishing_` 前缀（已核实与现有 406 节点无冲突，但后续新增要继续自查）。
6. `NOTICE.md` + `LICENSE-APACHE` 合规（见 §0）。

### 3.6 一句话结论

- **钓鱼**：技术可行、工作量中等（1 个 Go agent + 1 个 pipeline + 1 个 task + 截图模板 + **Win32 实机标定**）。最大风险不在代码，在 `ref_fps=60` 那套帧率假设——建议直接换成"两帧实测 px/ms"。
- **套利**：技术可行但体量巨大（468 节点 + 23 个 py 模块 + 4 个数据文件）。**不要一次性照搬**，先做 P0「只读行情 + 卖峰值料理」，跑通再叠 P1~P3。
- **不需要引入 Python**——Go 绑定 v4 的 `AgentServerRegisterCustomAction` 已覆盖上游全部 custom action 需求，代价是要把 py 逻辑手工翻译成 Go。

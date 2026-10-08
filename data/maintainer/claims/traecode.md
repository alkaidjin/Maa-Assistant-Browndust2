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

## 跑商（每日商店套利）功能 — 施工登记（2026-10-02 开工）

### 拍板方向（主控确认）

- agent 形态：**新建独立 trader agent**（第 4 个 exe，照 fishing 骨架）
- 首发范围：**卖当天峰值 + 砍价+一键采购**（料理制作留 P3）
- 价表来源：**随包 + 在线热更新**（复用 ok-bd2 的 price_calendar.v1.json 结构与数据）
- 默认策略：**测试版，默认真实交易**；提供"仅预览计划(dry-run)"选项

### 交付物（已完成，未实机标定）

| 项 | 路径 | 说明 |
|---|---|---|
| agent 源码 | `data/maintainer/agent/trader/` | 6 个 .go：main / trade(编排) / calendar(价表+热更) / sell(卖峰值循环) / buy(砍价+采购) / nav(OCR/点击/focus) |
| agent 构建产物 | `agent/trader.exe` | 6.7MB，go1.25.6 构建，vet 全过 |
| pipeline | `resource/pipeline/MapTrade.json` | `Trade_Start`→Custom`TradeRun`；`Trade_EnterMerchant`/`Trade_ReturnHome` 导航链骨架（坐标待 PC 标定） |
| 任务定义 | `tasks/MapTrade.json` | entry `Trade_Start`；6 个预设（真实卖1+采购/真实卖全部/仅出售卖1/仅出售卖全部/仅采购/仅预览） |
| 随包价表 | `resource/price_calendar.v1.json` | **独立从 souseha 原始 DB 生成**（非复用 ok-bd2 数据），schema v1，145 物品/30 商店/1-28 日有峰值 |
| 打包器 | `data/maintainer/tools/build_release_zip.py` | REQUIRED_FILES 加 `agent/trader.exe` + `resource/price_calendar.v1.json`；EXCLUDE_DIRS 加 `trader` 护栏 |

### 需主工作区统一加的 interface.json 改动（本分支不碰）

1. `agent` 数组追加第 4 项：
   ```json
   {
     "child_exec": "agent/trader",
     "child_args": [],
     "timeout": 15000,
     "identifier": "mxu_browndust_trader",
     "auto_reconnect": true
   }
   ```
2. `import` 数组追加 `"tasks/MapTrade.json"`

### 实机标定待办（需 PC 游戏截图）

1. `resource/image/TRADE/` 下模板（PC 1280×720 自截，不可搬参考项目素材）：
   - `MerchantLandmark.png`（商人交互地标，快路径）
   - `QuickSwitchPlayIco.png`（卡带快速切换入口）
   - `story_badge_06.png`（剧情卡第6号角标）
   - `MapMerchant.png`（小地图商人图标）
   - `NviSandGuideButt.png`（小地图导航按钮）
   - `AutoNviIco.png`（自动行路图标）
   - `Sale120Marker.png`（**↑120% 峰值标记，出售决策核心，需在峰值日截**）
2. 导航链 ROI/坐标按 PC 实机微调（当前为 ok-bd2 1920×1080 等比缩放值）
3. 三种 Win32 截图后端（FramePool/PrintWindow/DXGI）全量标定 OCR 阈值

### 合规

- 价表 JSON 数据结构复用 ok-bd2（MIT），AGPL-3.0 兼容；NOTICE 需署名 ok-bd2 与 MFABD2
- **未复用**任何图标素材，全部模板待 PC 自截
- 交易安全网：三重证据才成交（日历+OCR 名+↑120% 模板）、默认卖 1、拥有数不降即中止、各阶段硬超时、失败必回主页

---

## 倒卖收藏重建（固定槽位版）— 施工登记（2026-10-04）

### 背景

`RebuildFavorites.json`（逐商品 OCR 文字再点星）效率低；改用**检测灰星、按固定网格槽位点亮**。
旧 `RebuildFavorites.json` 作为**备选方案保留不动**。

### 关键事实（来自实机 1080P 视频 + souseha shopItemPrices 交叉验证）

- 购买页：左侧单个可滚动卡带列表共 **31 项**，顺序 S1–S19(1–19) / C1–C7(20–26) / E1,E2,E3,E5,E7(27–31)；每屏可见 9 行
- 商品区为 4 列网格，每个商品卡片**右上角固定星标**（黄=已收藏，灰=未收藏）
- **游戏内网格不含「天賦特效藥」**（souseha 货架含它，但实际不显示，后续商品按行优先重排）；技能书仍显示
- 48 种倒卖商品中 39 种在剧情店、另 9 种（巧克力/鮪魚罐頭/水果罐頭/辣椒/辣椒素/松露油/美乃滋/魚子醬罐頭/包裝好的海苔）只在角色/活动店

### 交付物

| 项 | 路径 | 说明 |
|---|---|---|
| pipeline | `resource/pipeline/RebuildTradeFavorites.json` | 327 节点：克隆入口链（RTF 前缀，入口→砍价到 RTF_ShopArrive）+ 5 屏固定坐标星标节点；滑屏 4 次 |
| 任务定义 | `tasks/RebuildTradeFavorites.json` | entry `RTF_Start`；label「重建倒卖收藏表（48种商品）」 |
| 灰星模板 | `resource/image/TRADE/star_gray.png` | 43×43，从 1080 视频裁后缩放到 720；TemplateMatch threshold 0.8 |
| 收尾 | 复用 `ReturnHome_RF`（在 RebuildFavorites.json，跨文件引用） | 不重复造 |

### 运行方式（固定坐标，无逐商品 OCR）

5 屏，每屏推进 7 行（滑屏 delta 441px@720）：
1. abs1–7（S1–S7）
2. abs8–14（S8–S14）
3. abs15–21（S15–C2）
4. abs22–28（C3–E2）
5. 滚到底（钳制），处理 abs29–31（E3/E5/E7，相对行 7/8/9）

每个收藏槽位：TemplateMatch 命中灰星 → Click（已是黄星则灰星模板不匹配、自动跳过，幂等）。

### 需主工作区统一加的 interface.json 改动（本分支不碰）

- `import` 数组追加 `"tasks/RebuildTradeFavorites.json"`

### 首次实机重点核对（用户自测）

1. 滑屏后左侧卡带是否按 7 行对齐（若游戏滚动有惯性误差，需调 RTF_swipe1..4 的 begin/end 或后续改 OCR 选卡）
2. 4 列×4 行星标点击位置（grid x 381/600/821/1043，y 106/178/250/321）
3. 灰星模板 threshold 0.8 是否稳命中、不误判

## 2026-10-04 迭代1：修复首槽后任务终止（阈值卡死）

实机现象：收藏 S1 蘑菇成功后，流程不再继续、约 20 秒后失败（on_error 截图 `2026.10.04-16.32.55.675_RTF_g_S1_1.png`）。

根因（`debug/maafw.log`）：
- 灰星 MF 实测分数 0.8139（蘑菇）、0.7986（胡萝卜）；胡萝卜仅差 0.0014 低于 threshold 0.8 → 识别失败
- 节点 next 列表在 20s `reco_timeout` 内反复重试失败 → `Node.PipelineNode.Failed`，任务终止

修复（已重新生成 `RebuildTradeFavorites.json`，608 节点）：
- 每槽改为成对节点，next 顺序先守卫后点击：
  - `RTF_y_<code>_<slot>`：ColorMatch 金环（lower 190,150,30 / upper 245,200,90 / count 20）→ DoNothing 跳过
  - `RTF_g_<code>_<slot>`：TemplateMatch 灰星 threshold 降到 0.7 → Click
- 金环参数经 on_error 帧像素标定：黄星 79/81 像素、灰星 ≤7，余量 3 倍以上；黄星绝不再被点击

同步更新：技能 `.trae/skills/mfa-roi-pipeline` 的生成器、SKILL.md、references。`tasks/RebuildTradeFavorites.json` 无需改（entry 不变）。

## 2026-10-11 迭代2：608 → 77 节点（MPE 卡死治理）【已被迭代3纠正，勿采用】

> **注：本轮"物品级全局/全局去重"理解错误，迭代3 已纠正，77 节点版本作废。**

用户反馈 608 节点在 MPE 中卡死。依据 Pipeline 协议（`maafw.com/docs/3.1-PipelineProtocol`）做两级压缩：

1. **全局去重**：官方公告（2026-03-12 更新）确认收藏是物品级全局——"购买收藏商品"一键买入所有卡带中收藏的该商品。同一商品只需在首次出现的卡带点星。281 个槽位出现 → 48 个商品首现；有新增的卡带仅 15 个（S1–S8 除 S9、S10/S12/S14、C1–C4），活动店 E 系零新增不访问。
2. **`inverse` 单节点**：每槽一个节点——ColorMatch 金环 + `inverse:true`（非金环即灰星才命中）+ Click 显式 `action.param.target` 固定坐标；取代原金环守卫/灰星点击成对节点。

节点账：11 入口类（RTF_Start + 9 克隆 + RTF_ShopArrive）+ 15 选卡 + 3 滑屏 + 48 槽位 = **77**。Validate：0 errors / 0 warnings。48 种商品逐一核对恰好覆盖一次。

技术处理：生成器弃用 PS5.1 `ConvertTo-Json`（单元素数组必被压成字符串），脚本内建 `Emit-Json`；修复数组跨函数/if 块被枚举（`,@(...)`）、`7..6` 递减范围两类陷阱。

若 77 节点仍嫌多，后续可沿"商品首现卡带"边界（如 S1–S8 / C1–C4）拆成多个 pipeline 文件分段执行。

## 2026-10-11 迭代3：纠正收藏语义，608 → 326，按屏拆 3 文件跨文件链接

用户纠正：**收藏按卡带×商品分别保存**，一键购买只是把各卡带已收藏的商品汇总进购买篮；同商品在未收藏的卡带不会一起买入（ok-bd2 逐店点收藏即证据）。故取消全局去重，恢复逐卡带遍历。

最终方案：
- 31 个卡带全部含倒卖品，共 **280 槽位**（仅 C2 的糖在 souseha 中是同 itemId 完全重复记录，折叠；PS 数据核对）
- 5 屏 / 4 次滑屏；末屏滚到底钳制，abs29–31 点第 7–9 行（y 482/545/608）
- 每槽单节点：ColorMatch 金环 + inverse + Click 固定 target（608 成对节点 → 326）
- 为 MPE 按屏拆 3 文件、跨文件 next 链接（运行时目录内共享命名空间），**任务仍只一个、entry 不变**：
  - `RebuildTradeFavorites.json` 164 节点（入口链+屏0/1）→ 尾跳文件2 RTF_swipe2
  - `RebuildTradeFavorites2.json` 133 节点（屏2/3）→ 尾跳文件3 RTF_swipe4
  - `RebuildTradeFavorites3.json` 29 节点（屏4）→ ReturnHome_RF
- 验证：三文件单独 Validate 均 0 errors/0 warnings；合并命名空间 7384 条 next 引用全部可解析

附带修复：源节点真名 `RebuildFav_CHooseCrad`（大写 H）与直觉拼写不同，克隆时照源拼写，否则节点名/引用大小写错位。

### 实机重点（用户自测）
1. 跨文件跳转是否平滑（文件1末尾→文件2 swipe2、文件2末尾→文件3 swipe4）
2. swipe4 后末屏钳制位置：E3/E5/E7 是否在第 7–9 行
3. 金环 inverse 判定与滑屏对齐（同前两轮关注点）

## 2026-10-11 迭代4：照 ok-bd2 灰星表跳过槽位，280 → 216 点击 / 326 → 262 节点

用户指出应按此前 ok-bd2 调研结果，排除特效藥和技能书后商品点击约 217。直接从 GitHub 拉取 ok-bd2 [data.py](https://github.com/GodRaymond233/ok-bd2/blob/main/src/tasks/map_trade/data.py) 核对 `SHOP_UNFAVORITED_POINTS`：

- 槽位号按**完整货架** 1 起编号（剧情店槽位1=特效藥；技能书占位）。此前"网格不显示特效藥"的结论与 ok-bd2 标定冲突，作废。
- 每店灰星集合转录到 `%TEMP%\souseha2\unfavorited_slots.json`（R*↔C*）。
- C2 两条同 itemId 的「糖」经核对是游戏内两张真实卡片（槽位4/8；ok-bd2 当前版本一灰一藏），不折叠。
- 恒等式：250 收藏条目 = 商品 **216** + 特效藥 28 + 技能書 6（S店商品121/C店62/E店33）。脚本只点商品 → 216 槽位。
- 生成器新增 `-UnfavoritedJson` 参数，去掉店内折叠与特效藥剔除，改为 full-shelf 编号 + 灰星跳过。
- 重新生成三文件：116 / 122 / 24 = 262 节点；三文件 Validate 均 0/0；全目录引用核对（三文件自身 0 未解析；其余 82 条为其它文件原有的 `$__mpe_*` MPE 元数据/空引用，非本次引入）。

与用户记忆的"217"相差 1，已把逐店商品清单交付核对，待用户指出多出的具体商品后补点。

## 2026-10-04 迭代5：以 7 月新跑商视频为准，216 → 205 点击 / 262 → 250 节点

用户提供 B站最新跑商教学视频 `c:\视频\26年7月新跑商1.1.mkv`：2:40 起，一键购买全部收藏实测 **205 个道具**（消耗 168万6460 金币）；含全店总览图 + 31 卡带逐店展示。

处理方法（未凭肉眼）：
- ffmpeg（剪映自带 11.5.0.14471）从视频抽帧；1080p 实机画面逐格金环像素判定，星心列 581/912/1242/1576、行 140/250/360/470，金像素 60–124 vs 灰位 ≤22
- 31 店各取稳定代表帧扫描，汇总**正好 205**（S 店 124 / C 店 48 / E 店 33）
- 与旧 216 口径的差异：
  1. **编号改口径**：特效藥游戏完全不显示，网格位置 1 = shelf 槽位 2（旧口径按完整货架 1 起）
  2. **S9 全部灰星（总览图标"无"），脚本不再进入**
  3. **6 个新版商品**视频收藏、souseha resale CSV 未含：杏仁（S10/S14/S17）、芥末（S7/S15）、哈密瓜（S5），新增 `extra_items.txt` + 生成器 `-ExtraItemsFile` 追加（脚本保持纯 ASCII，中文名只放数据文件）
  4. 总览图红=0利润商品（巧克力/鮪魚罐頭，10 格）、黄=可手动调整（淡水蝦/胡椒/雞胸肉袋，6 格），视频里均为金环、照点
- 逐店明细（含红/黄标记）已交付用户核对
- 三文件重新生成：117/109/24 = 250 节点；Validate 均 0/0；内部引用完整，唯一外部依赖 ReturnHome_RF（RebuildFavorites.json）

## 2026-10-05 迭代6：修正 S10 转录错位（漏肉桂）

用户实机核对发现 S10 应收藏肉桂。重扫 S10 视频帧（s_045）原始像素并全部 31 店复核：

- **S10 转录错位**：正确金环 = 杏仁/草藥/奶油/鹽/糖/**肉桂**（网格 1,6,7,8,10,11）；误写成 杏仁/蘑菇/草藥/奶油/凱洛藥材/糖（把 8/11 当灰、5/9 当金）。每店数量恰好都是 6，总数 205 未暴露异常
- 其余 30 店原始计数逐一比对，全部与已生成 pipeline 一致，无其他错误
- 肉桂不在倒卖 CSV，`extra_items.txt` 增补为 杏仁/芥末/哈密瓜/肉桂（+7 槽）；S10 灰星集合 {2,3,4,8,11,12} → {2,3,4,5,9,12}
- 三文件重新生成 117/109/24 = 250 节点，总点击 205，Validate 0/0
- 教训：逐店转录灰星集合时必须保留**每格原始像素计数**（rescan.txt）供回溯，不能只记最终金/灰二值——错位类错误在"数量碰巧一致"时无法靠总数发现
- 料理材料结论（结合用户指正）：肉桂已覆盖；香草牛排的肉为**兽肉**（非卖品、跑图获取），不属于商店收藏范围


## 2026-10-05 迭代7：新增 SellPremium（高价日出售）与 CookMeals（料理制作骨架）

- 出售清单16种 = 收藏商品中不用于高价值料理的食材（杏仁/肉桂为料理材料已排除；芥末/哈密瓜属 extra_items 已收藏故保留）。日分布：D2水果罐头@S2、D4大麦@S18、D7咖啡豆@S1、D10玉米@S12、D13鲑鱼@S7、D14红萝卜@S7+芥末@S16、D15葡萄@S3+起司@S19、D18番红花@S10+萝卜婴@C3、D19哈密瓜@C6、D22料酒@C5、D24啤酒花@S19、D26金枪鱼罐头@S12、D27苹果@S6；其余15天无售
- 日期门禁=28个手动入口 SellPremiumD1~D28（用户选定方案）：tasks/SellPremium.json 28个task；pipeline 三文件 SellPremium/2/3.json = 100/113/96 节点，进店链克隆自 MapTrade（含魔兽赛季奖励确认 JumpBack），选卡/滑屏坐标克隆自 RTF 标定（行y=104+63x(row-1)，每屏7行，swipe 240,605→240,164 end_hold 500）；S1为进店默认商店无需选卡
- 每日链：入口→经营管理→餐厅→广场→洛兹→OpenSell(出售页签)→sel卡带→商品Template→确认→ReturnHome_1/2→StartGame；商品节点focus标注"商品@卡带商店+基础→高价"；每个条件节点的next都带ReturnHome兜底（占位模板不命中时优雅收尾）
- 全部模板为灰色占位PNG共18个（resource/image/TRADE/premium/：sell_tab/sell_confirm+16商品），用户实机截图后同名覆盖即可，零JSON改动
- CookMeals 骨架（26节点）：经营管理→餐厅→料理列表→15道Cook_dish_XX（max_hit=1防重复制作）→制作/MAX/确认/关结果→Cook_Back→回主页；占位PNG 21个（resource/image/COOK/：6按钮+15料理图标）
- interface.json import 增补 tasks/SellPremium.json、tasks/CookMeals.json（本次经用户明确授权改动interface）
- 料理材料文档交付 data/maintainer/料理-高价值配方与材料覆盖.md（15道配方/非商店材料5种/覆盖30种+缺口雷瓦汀火腿S18未收藏）
- Validate 4文件全部 0错误0警告
- 已知风险：①出售页签下左侧卡带列表是否可见未实机验证，若不可见需对调OpenSell与sel顺序；②确认出售共用sell_confirm.png一张模板；③同日第二家商店从确认节点直接接滑屏，实际可能需要先返回商店列表

## 2026-10-06 迭代8：SellPremium 改为单入口（trader agent TradeRun 自动按日选链）

- 用户指正：28个手动入口不可接受，任务在软件内只能有一个入口，运行后脚本自行按运行日期选择对应商品链
- 调研发现 data/maintainer/agent/trader/ 下已有完整agent源码（此前迭代构建）：agent/trader.exe 注册自定义动作 TradeRun（另有调试用 TradeChainTest），内部 loadCalendar 读 resource/price_calendar.v1.json，gameDay 按北京23:00切日取日号，sellList(day) 自动得今日峰值商品；出售三重校验（OCR商品名+TRADE/Sale120Marker.png↑120%模板+弹窗名称/拥有数），支持 sell_mode min/max/reserve
- 改动：
  1. 删除 pipeline SellPremium2.json/SellPremium3.json（原309节点多入口方案）及 resource/image/TRADE/premium/ 18个占位PNG
  2. resource/price_calendar.v1.json 从全量日历（含料理材料与非食材）裁剪为16种非料理食材：保留原繁中item/商店中文名/英文alias，shops白名单13店，只保留13个有货day键（2/4/7/10/13/14/15/18/19/22/24/26/27）
  3. pipeline SellPremium.json 重写为13节点：SellPremium_Start(Custom TradeRun, param enable_sell=true/enable_buy=false/sell_mode=max) + Trade_EnterMerchant子任务(SP_进店链7节点克隆MapTrade) + Trade_ReturnHome子任务(SP_收尾2节点) + TradeFocus(DirectHit/DoNothing辅助)
  4. tasks/SellPremium.json 重写为单任务；interface import 路径不变无需改动
  5. 新增占位 resource/image/TRADE/Sale120Marker.png——用户需截图的模板从18+2张降为仅此1张
- Validate：0错误；3个"no next"警告=Start/ZiLuo/TradeFocus三个终点节点（与既有AbsorpAssemble的AbsorpScheduleEnd同类良性警告）
- 教训：交付日期分支方案前先盘点项目已有agent能力（trader源码就在data/maintainer下），不要因"日期无屏幕特征"就把分支推给用户手动选入口

## 2026-10-06 迭代9：收藏任务选店改OCR识别 + pipeline归入Trade子目录

- 用户实机反馈：RTF收藏任务运行到后面错乱——进入卡带前不识别店名、滑屏（605→164,441px）不精准，固定坐标选店（x=240, y=104+63*(rel-1)）累积漂移导致点错店、收藏错商品
- 修复（Build-FavoritePipeline.ps1）：
  1. 新增参数 -CartNamesJson（{code:显示名}）与 -CartListROI（默认[55,85,305,600]覆盖左侧卡带列表）
  2. 30个选店节点由"无识别固定坐标Click"改为 OCR(expected=店名中文, threshold=0.3) 识别列表内店名 + Click命中框中心；滑屏仅负责把下一批店名带进视野，漂移不再累积；不传CartNamesJson时保留旧固定坐标分支
  3. 店名数据文件临时目录 cart_names.json（30店中文名，S9跳过）
  4. 重新生成RTF三文件 117/109/24=250节点，205槽位不变，next/t节点结构不变
- 目录整理：新建 resource/pipeline/Trade/，移入8文件——MapTrade、RebuildFavorites、RebuildTradeFavorites/2/3、CookMeals、SellPremium。tasks/*.json只引用entry节点名（全局命名空间），无需改动；MaaFW递归加载pipeline子目录
- Validate-Pipeline.ps1 改为 -Recurse 扫描节点（支持子目录）；校验全部0错误：良性警告仅终点节点（MapTrade的魔兽赛季奖励确认_3、SellPremium的Start/ZiLuo/TradeFocus）
- 教训：凡依赖滑屏定位的列表，点击前必须先OCR/模板识别目标本体，固定坐标只能用在不滚动的静态UI上

## 2026-10-06 迭代10：收藏重建改"每店重置+收藏闭环"，并入MapTrade单任务可选项

- 用户要求：①跑收藏前必须先取消全部收藏（部分用户自行收藏过其他东西导致错乱）；②重建收藏与一键购买合并为一个任务，可选菜单加"是否重建收藏清单"——选重建则收藏跑完不离店直接购买，不选则进店直接一键购买；③重建流程=进店后OCR左侧商店列表点第一家→该店金星全部点回灰→固定坐标收藏清单商品→左侧识别下一家→列表到底下滑→OCR再进，逐店闭环
- Build-FavoritePipeline.ps1 改动：
  1. entries不再过滤slots>0：全部31店（含S9铁假面）都进入，新增gridSize=可见槽位数；cart_names补S9
  2. 每店新增 r_ 重置节点链（1..gridSize逐槽 ColorMatch金色非inverse→点击取消，灰星不命中自动跳过，共298节点）
  3. sel next = r链 + t收藏链 + tail；S9无t节点
  4. FinishNode默认改 Trade_BuyAllFavorites；末店E7收藏完直接接一键购买
  5. 新增 -OutPipelineD，四bucket：A=入口链+screen0(134) / B=screen1(134) / C=screen2(116) / D=screen3-4(165)，共549节点
- 任务合并：tasks/MapTrade.json 重写，entry=RTF_Start，select选项MapTradeRebuildFavorites对RTF_ShopArrive做pipeline_override——不重建next=Trade_BuyAllFavorites（默认），重建next=RTF_sel_S1；进店/砍价链两条分支共用一套（RTF_前缀9节点）
- 删除 tasks/RebuildTradeFavorites.json 并从interface.json import摘除（独立入口取消）
- Validate：四文件全部0错误0警告
- 教训：选项分支点尽量放在流程中自然汇合处（砍价后商店内），入口链只保留一套；重置+收藏同店闭环把遍历次数从62店次降到31店次

## 2026-10-06 迭代11：收藏重建pipeline细分为14文件（每文件≤50节点，MPE可打开）

- 用户反馈：549节点四文件仍点不进MPE，要求再细分：每文件节点<50，文件名写明包含商店（如S1TOS4），跨文件用外部节点跳转
- Build-FavoritePipeline.ps1 新增 -MaxNodesPerFile 自动切分模式：
  1. 以商店为最小unit（前置RTF_swipe随该店打包），贪心分组保证每文件≤上限；单店最大25<50安全
  2. 输出 RebuildTradeFavorites_Entry.json（Start+进店9节点+ShopArrive=11）+ 13个商店区间文件，命名 <base>_<首店>TO<尾店>.json
  3. 自动扫描全部next：目标节点在另一文件时加[JumpBack]前缀；全局节点（Trade_BuyAllFavorites、StartGame等）无定义不在bucket中保持裸名
- 实际分组（节点数）：Entry 11 / S1TOS2 30 / S3TOS4 45 / S5TOS7 48 / S8TOS10 50 / S11TOS12 45 / S13TOS14 39 / S15TOS16 35 / S17TOS19 50 / C1TOC2 31 / C3TOC4 37 / C5TOC7 50 / E1TOE3 48 / E5TOE7 30；合计549不变
- 删除迭代10的四个screen bucket文件；tasks/interface无需改动（节点名与entry不变，pipeline递归加载）
- Validate：14文件全部0错误0警告；抽查Entry→S1、区间衔接、E7→Trade_BuyAllFavorites接线均正确

## 2026-10-06 迭代12：修复重置时土豆/奶酪黄像素误判金星

- 用户实机反馈：重置商店收藏时，土豆、奶酪在灰星状态被误识别为金星（商品图自带黄色像素落入金色窗口190,150,30-245,200,90），灰星被点成金星造成错误收藏
- 取证（shopframes/f10_720.png 1280x720 真实商店页，土豆灰星位于col4 row2）：
  1. 旧ROI 48x46（half 24/23，中心=格点）内金色像素59 ≥ count20 → 误命中；放大裁剪确认黄源=土豆商品图（左下）+ 数量角标（左侧）
  2. ASCII热力图确认星环外径~24px、星标自带深色圆形背板半径~16px；金色干扰均在背板外
  3. 对图中4颗金星+9颗灰星用32x32紧凑ROI实测：金星68-74像素，灰星0-2像素（土豆2、蜂蜜2），分离干净
- 修复（Build-FavoritePipeline.ps1 默认值）：RoiHalfW/RoiHalfH 24/23→16/16（ROI仅罩星标背板）；GoldMinCount 20→30（金星余量≥2.3x，灰星余量≥15x）；点击target不变
- 重新生成14文件（分组与节点数不变），Validate全部0错误0警告
- 教训：颜色判定窗口必须紧贴目标控件本体（星标背板），不能为宽容坐标而放大ROI把商品美术纳入；阈值取值要用多真/假样本实测上下界后留双向余量

## 2026-10-06 迭代13：修复玉米误识别 + S15跳店（含源文件丢失重建）

- 用户实机反馈两问题：
  1. 玉米（黄色商品图，S8/S18 位置1，本身需收藏）与之前土豆一样在灰星时被误判金星，最终碰巧结果正确
  2. S15复仇的誓言被跳过：S14完成后直接点了S16
- 取证（用户提供 1920x1080 S8 实机截图，含7金星+6灰星）：
  - 径向分段统计：金星金环在r6-11（~180金像素），灰星r<=14金色=0，黄色商品图（土豆4像素等）只在r>14出现
  - 结论：ROI half 16仍过大（1080 r15-24区域纳入商品黄），收紧至half10@720（1080 half15）；30x30@1080验证金星179-188/灰星全0
  - S15跳过根因：sel_S15单次OCR漏识后，tail中sel_S16在滑后屏已可见并命中→永久跳过
- 修复（Build-FavoritePipeline.ps1）：
  1. RoiHalfW/H 16→10，GoldMinCount 30→40（720p金环~83像素），SwipeDelay 900→1200
  2. 每店新增RTF_seek_<cart>节点，仅作为该店sel自己next的最后候选：店名OCR miss→滑一屏→回环重OCR同一店，max_hit=4兜底；S1的seek为DoNothing（列表顶部不滑）
  3. 店内r/t节点tail直接前进sel下一家（不含seek），杜绝seek在店内操作完成后误触发回环
  4. sel OCR expected增加店名前两字短词（31店两两唯一，如S15=复仇的誓言|复仇）
- 意外：RebuildFavorites.json进店链源文件丢失（从未提交git，无法恢复）；以MapTrade.json的9个TC节点重建RebuildFav_命名源（temp/souseha2/RebuildFavorites_source.json），互引用/全局节点处理一致
- 16文件576节点（11+32+47+33+50+41+48+36+37+35+33+38+38+46+34+17），Validate全部0错误0警告
- 教训：①同色干扰（商品黄=金环黄）只能靠位置/形状分离，判据ROI必须实测径向边界 ②重试节点要挂在"需要重试的识别节点自身next"上，不能放进公共tail——场景语义会冲突 ③未跟踪的工作区源文件是单点，应及时纳入git

## 2026-10-06 迭代14：修复进店卡在洛兹（OCR洛兹→洛药误识）

- 用户反馈：每日跑商进不去商店，卡在广场商人处
- 日志定位（maafw.log 12:40:44 GOshop命中后）：RTF_ZiLuo 20秒reco_timeout，OCR原始输出 box=[592,200,31,15] 文本="洛药" score=0.804——PaddleOCR把小字号"洛兹"稳定误识为"洛药"，expected["洛兹"]不匹配；首次跑通为重试中偶然读对
- 修复：ZiLuo expected 增加单字"洛"（洛药/洛兹均含，该界面唯一）：
  1. MapTrade.json ZiLuo_TC expected=["洛兹","洛"]
  2. temp源 RebuildFavorites_source.json 同步后重新生成16文件（576节点，0错误0警告）
- 教训：OCR门禁词遇到不稳定单字时，加一个"必被正确识别的共有子串字"做容错；判定问题要先看日志OCR all_results实际文本，不能凭画面想当然
- 同次清理：用户确认后删除 tasks/RebuildFavorites.json 并从 interface.json import 移除（旧任务pipeline已由用户删除，残留任务定义会导致入口失效）

## 2026-10-06 迭代15：修复重建收藏选项不生效（ShopArrive孤儿节点）

- 用户反馈：选「重建收藏清单后购买」仍直接一键购买，不走重建流程
- 日志定位：RTF_Bargain2命中后下一节点直接是Trade_BuyAllFavorites，RTF_ShopArrive（选项override的分支点）从未被执行
- 根因：迭代13重建进店链源时，Bargain2.next保留了MapTrade原链直连Trade_BuyAllFavorites，丢失原版Bargain2→RTF_ShopArrive连接，ShopArrive成孤儿；选项pipeline_override改的是从未到达的节点，故无效
- 修复：temp源RebuildFav_Bargain2.next=[Trade_BuyAllFavorites]→[RTF_ShopArrive]，重新生成16文件（576节点，0错误0警告）
- 全链梳理验证：
  公共链 RTF_Start→EnterOperating→EnterRestaurant→CHooseCrad→EnterSquare→OnSquare→GOshop→ZiLuo→Bargain1→Bargain2→RTF_ShopArrive
  不重建(默认) override ShopArrive.next=[Trade_BuyAllFavorites]→BuyAllConfirm→ReturnHome
  重建 override ShopArrive.next=[RTF_sel_S1]→31店(sel+seek+r+t)→t_E7_8→Trade_BuyAllFavorites→BuyAllConfirm→ReturnHome
  跨店衔接 t_S1_7.next=[sel_S2+JumpBack兜底+BuyAll] 正确
- 教训：重建/迁移pipeline源后必须沿entry走到终态逐节点核对next接线，孤儿节点Validate不报错但会让override失效

## 2026-10-06 迭代16：修复S9死循环 + 加高店名ROI（移除per-shop seek架构）

- 用户反馈：S9切换到S10时，脚本滑屏后又点击S9，卡死
- 日志定位：sel_S9命中(盒y=603底部)→点击→r_S9_1..8全部失败→seek_S9滑屏→重试sel_S9→循环
- 根因双重：
  1. S9货架9格、只收藏位置9(辣醬)，但辣醬不在倒卖清单→S9有8个r_节点、0个t_节点
  2. per-shop seek架构缺陷：sel.next=[r_*,t_*,seek_X]把"进店后r/t全失败"与"店名OCR失败"混在同一next，均触发seek滑屏。对无t_且已干净的店，r_全失败→seek无限滑屏重试
- 修复（Build-FavoritePipeline.ps1）：
  1. 移除per-shop seek节点，改回屏幕边界无条件swipe（每7店一屏，4次swipe）
  2. sel.next=[r_*,t_*,tail]，无t_的店r_全失败后直接通过tail前进到下一家，不再回环
  3. swipe不精确也无妨：sel用OCR在加高的ROI内找店名，不会点错店
  4. CartListROI [55,85,305,600]→[55,60,305,650]（y覆盖60-710，多捕获店名）
- 14文件（11+30+45+48+50+45+39+35+50+31+37+50+48+30），Validate全部0错误0警告
- 教训：MaaFramework的next是候选项语义，同一列表里不能混放"重试当前"和"前进到下一"两种语义；对无动作可执行的节点（空t_）必须确保有前进兜底

## 2026-10-06 迭代17：修复重建收藏任务回主界面后红字失败（jumpback_stack弹栈超时）

- 用户反馈：重建收藏任务31店全部跑完、收尾链也执行完、已回主界面，任务却红字失败结束（on_error截图 RTF_t_E3_9）
- 日志定位（maafw.log）：
  1. 收尾链 Trade_BuyAllFavorites→BuyAllConfirm→ReturnHome_TC_1→ReturnHome_TC_2→StartGame 整条运行在 `[JumpBack]RTF_sel_E5` 子链上下文内（15:50:55 `push jumpback_stack: RTF_t_E3_9`）
  2. 主界面上 StartGame 锚点 21 候选中 GoBack（误点左上返回箭头）→BackHome（误点右上主页图标）→EnterGame（Shadow.png 0.96 误配大厅剪影）依次命中
  3. EnterGame 空 next 死端→`pop jumpback_stack: RTF_t_E3_9` 弹栈复活旧候选列表→sel_E5/sel_E7/BuyAll 在主界面全部落空→`Task timeout [RTF_t_E3_9] 20430ms` 红字
- 修复方案（用户批准：StartGame 收尾汇点是项目惯例保持不改，改用大厅终结候选）：
  1. 生成器新增 -LobbyTextFile/-LobbyROI/-LobbyThreshold 参数，Entry 文件定义 RTF_LobbyEnd 终结节点：OCR「付费商店」（主界面底栏独有文字，购买页无底栏，ROI [695,672,80,34] 取自 on_error 实机帧 native 坐标换算 720p）、无动作、无 next
  2. 全部 store-phase 节点（sel/r_/t_/swipe）next 末尾追加 **plain 引用** RTF_LobbyEnd（不能用 [JumpBack]，那会 push 自身进栈死循环）；排除 RTF_Start、9 个入口链节点、RTF_ShopArrive（启动时就在大厅会误触发）
  3. swipe 节点加 max_hit=1：无 recognition 字段=DirectHit 必中，弹栈重入会零进度自旋（4 个 swipe 定义分别在 S8TOS10/S15TOS16/C3TOC4/E1TOE3.json，均已确认）
  4. 弹栈级联：主界面 pop 后候选列表尾部 LobbyEnd 命中→死端→再 pop→…→栈空→任务绿色正常结束，任何一层弹栈都能被终结
- 新增数据文件 temp/souseha2/lobby_marker.txt（内容「付费商店」，文件传入保持生成器纯 ASCII）
- 重新生成14文件（12+30+45+48+50+45+39+35+50+31+37+50+48+30，Entry 11+LobbyEnd），Validate 14 文件全部 0 错误（Entry 1 警告=LobbyEnd 无 next，终结节点预期形态）；抽查确认 RTF_Start/ShopArrive 无 LobbyEnd 引用、全目录 0 个 [JumpBack]RTF_LobbyEnd、539 个 plain 引用
- 生成命令：同迭代16，追加 `-LobbyTextFile "f:\MABd2-wt\traecode-workspace\temp\souseha2\lobby_marker.txt"`
- 教训：①收尾接 [JumpBack]StartGame 的跨文件链，主界面锚点误命中后 jumpback_stack 弹栈会让发起方候选列表在意外场景复活，必须在业务节点 next 末尾挂场景终结候选兜底 ②终结候选必须 plain 引用；必中节点（DirectHit/swipe）在 JumpBack 链内必须 max_hit=1 ③场景终结 OCR 词要选"该界面独有且另一界面必无"的文字（付费商店在购买页无底栏，零重叠）

## 2026-10-06 迭代18：修复商品售罄时点购买链断红字（NoGoodsToast兜底跳离店）

- 用户实机反馈（17:21 任务）：31店重建跑完，到 Trade_BuyAllFavorites 点击后无确认弹窗，20 秒超时红字（on_error Trade_BuyAllFavorites）
- 根因：上一次任务（15:51）已把收藏商品全部买入，本次商品均为红色「售罄」章，两个购买按钮灰色禁用态；OCR 仍能识别按钮文字并点击，但点击只弹出中上 toast「没有可以购买的商品。」（无确认弹窗 Trade_BuyAllConfirm），toast 消失后候选列表全空→超时；任务链断在购买节点，无法执行后续离店链
- 修复（resource/pipeline/Trade/MapTrade.json，两路径公共节点一处全覆盖）：
  1. Trade_BuyAllFavorites.next = [Trade_BuyAllConfirm, Trade_NoGoodsToast]（confirm 优先，toast 兜底；两者互斥）
  2. 新增 Trade_NoGoodsToast：OCR ROI [490,130,290,50]（1080p 实测 toast 黑条 x793-1107/y215-257 换算并留余量），expected ["没有可以购买的商品","没有可以购买","可以购买的商品"] threshold 0.7，无 action（toast 自消），next=[ReturnHome_TC_1] 复用离店链（TC_1 返回→TC_2 关闭弹窗→StartGame）
  3. Trade_BuyAllFavorites post_delay 1500→600：toast 为瞬态，尽早开始轮询确保在显示窗口内命中
- ROI 对齐已验证（上传 1080p 帧缩 720p 实裁）：整句完整落框、四周留余量；toast 未出现时框内商品美术/价格数字与中文整句 expected 不可能误匹配
- Validate（-PipelineDir 指定 pipeline 根目录）0 错误，唯一警告为原有 魔兽赛季奖励确认_3 无 next；注意校验 Trade 子目录文件必须传 -PipelineDir 为根目录，否则跨目录 plain 引用 StartGame 会误报 unresolved
- 教训：点击类节点的 next 不能只挂"操作成功后的下一界面"，还要考虑"点击被游戏拒绝（禁用按钮只弹 toast）"分支；瞬态 toast 兜底要缩短点击后 post_delay 以保证 OCR 轮询窗口覆盖 toast 显示时间

## 2026-10-06 迭代19：每日跑商集成料理制作+高价售出（星期门控可多选）

- 用户完成 CookMeals 料理流程（15道料理，收尾 Cook_ReturnHome），要求集成进每日跑商：一键购买后（ReturnHome_TC_2）进料理，料理完按当日高价出售表售出；料理做成可选项+周门禁清单（可多选，默认周四），门控不满足跳过料理但仍售卖
- 接线（resource/pipeline/Trade/）：
  1. MapTrade.json 新增 Trade_CookScheduleEnabled 门控节点：Custom recognition ScheduleRecognition（go-service 提供，同 AbsorpAssemble.AbsorpScheduleEnabled 模式），attach 七天全 false 由选项勾选，next=[OnSquare_Cook]（plain 跨文件引用 CookMeals.json 的料理广场起点）
  2. CookMeals.json Cook_ReturnHome 加 next=[SellPremium_Start]：用户原话"next到SP_OnSquare"按架构修正——直接接 SP_OnSquare 只会走到 SP_ZiLuo 死端（出售逻辑在 trader agent 的 TradeRun 闭环里），引用 SellPremium_Start 才触发完整售卖；其 agent 进店子链 Trade_EnterMerchant 第三候选即 SP_OnSquare（支持广场起点），效果与用户设想一致
- 选项（tasks/MapTrade.json，task.option 三项）：
  1. MapTradeCookMeals（select，默认不制作）：制作 case override ReturnHome_TC_2.next=[Trade_CookScheduleEnabled, SellPremium_Start]——轮询上门控命中先进料理（Cook_ReturnHome→SellPremium_Start 汇合），门控不命中 DirectHit 直落售卖；未选中时保持原 [StartGame] 行为不变
  2. MapTradeCookSchedule（checkbox，默认勾周四）：7 case attach Trade_CookScheduleEnabled.{day}=true，完全复刻 AbsorpAssemble 的 AbsorpSchedule 模式（游戏日本地时间04:00分界）
  3. MapTradeRebuildFavorites 原样保留
- 文档：新增《高价出售表-商品与出售日期.md》（workspace 根目录）：16 种非料理食材×13 个有货游戏日（1-28循环，北京23点切日）全表+口径+维护方式；无货日 agent 空转跳过
- Validate：MapTrade.json 17 节点 0 错误（唯一警告=原有 魔兽赛季奖励确认_3 无 next）、CookMeals.json 30 节点 0 错误；tasks/MapTrade.json ConvertFrom-Json 通过、yes-case next/默认值确认无误
- 教训：①trader agent 的售卖入口是 SellPremium_Start（TradeRun custom_action 闭环），pipeline 直接连 SP_OnSquare/SP_ZiLuo 子链终点不会触发出售 ②interface checkbox 多选门控=每 case attach 单天开关，门控节点用 ScheduleRecognition custom recognition 判断 ③select+checkbox 两选项无联动，语义靠"checkbox 仅在 select=制作时生效"的选项描述约定 ④gate 不命中靠 next 列表后续 DirectHit 节点（SellPremium_Start 无 recognition）兜底，命中/不命中两条路在售卖节点汇合

## 2026-10-06 迭代20：高价出售表加入15道料理+食材繁简aliases+商店白名单扩到19

- 用户提供 2026-07-03 攻略图：15 道料理各自的每月高价日与出售卡带（红字5道推荐制作、黑字10道需狩猎场特殊食材）；要求加入 price_calendar 并复核食材与料理材料零交集、处理繁简名对应
- resource/price_calendar.v1.json 更新（16食材+15料理=31条，22个有货日）：
  1. shops 白名单 13→19：料理出售店涉及 6 个新店 沙漠之花(S5)/神圣审判(S14)/试炼之路(S17)/火晶片(C2)/合约之战(C7)/夏日骑士(E1)——calendar.go:148 加载时强校验 shop∈whitelist，漏加会直接报错；卡带映射取自 cart_names.json
  2. 15 条料理条目按图内简体名写入 item（游戏内实际显示名），同日多店并存（日7/10/13 食材+料理、日3 两道、日26 三条三店）
  3. 首次修复繁简隐患：原 16 条食材 item 全是繁体（souseha 数据源口径），而游戏 UI 显示简体，matchName 为包含匹配会全部落空——给每条 aliases 补简体名（三文鱼/鮭魚、金枪鱼罐头/鮪魚罐頭 来自实机截图与项目译名表，奶酪/芝士、藏红花、胡萝卜 等为通行译名，同形者不加）
  4. 名称一字差用 aliases 兜底：泰丽丝派(图)↔特瑞丝派(Cook dish_02)、火烤鱼板棒(图)↔炙烤鱼板棒(dish_14)
- 复核结论（写入文档）：16 食材与 15 道料理材料全集（35 种，含狩猎场特殊食材）逐一比对零交集；起司≠奶油、鮭魚≠淡水蝦、罐头类不同物、蘿蔔嬰≠紅蘿蔔 无同物异名冲突
- 文档《高价出售表-商品与出售日期.md》重写：食材表加简体名列、新增料理表（日/卡带号/店/红黑字）、零交集复核节、维护说明（新商店必须先加白名单）
- 校验：ConvertFrom-Json 通过；条目 31、日 22、白名单 19 全一致（过程中漏写日12卢戈山参烤串，条目数校验 30≠31 当场发现补上）
- 教训：①price_calendar.shops 是强校验白名单，新增条目的 shop 必须先入名单 ②繁体 item 对简体 UI 的 matchName 会静默全落空（agent 不报错只是卖不掉），aliases 必须带游戏实际显示名 ③全量重写结构化数据文件后按预期条目数复核（31）能立刻暴露漏项

## 2026-10-07 迭代22：跑商购买后无条件出售高价品（不制作料理也卖）

- 用户需求变更：跑商任务中售卖环节改为必经——"不制作料理"也要购买后进店出售当日高价品，仅当日高价表无可售物品时才结束任务
- tasks/MapTrade.json：MapTradeCookNo（默认）case 的 pipeline_override 从空改为 ReturnHome_TC_2.next=[SellPremium_Start]；两个 description 同步改写（"两种选择购买后都会出售当日高价品；无可售物品时直接回主界面结束"）——注意用户在 MPE 里自行改过 task description 文案，Edit 需以磁盘实际文本为准
- resource/pipeline/Trade/SellPremium.json：SellPremium_Start 补 next=["StartGame"]——保证无条目 agent 空转结束后也回主界面（空转时不跑收尾链，角色停在启动位；StartGame 锚点在广场/主界面均可命中）；同时统一独立 SellPremium 任务的收尾为项目惯例 StartGame 汇点
- 行为矩阵收敛：No→售卖；Yes+门控命中→料理→售卖；Yes+门控不命中→跳过料理→售卖；全部路径经 SellPremium_Start（agent 无条目则空转）→ StartGame → 结束
- Validate：SellPremium.json 12 节点 0 错误（唯一警告=SP_ZiLuo 无 next，agent 子任务终点预期形态）；tasks JSON 通过
- 教训：agent custom_action 节点的"空转提前 return"不会执行 RunTask 收尾链，节点后必须接 StartGame 类锚点兜底回主界面，否则角色滞留原地

## 2026-10-07 迭代23：其余每周任务加单选星期门控

- 用户需求：WeeklyTasks（小屋点赞+街机游戏+末日之书+装备制作，一周一次）加单选星期门控，仅选定星期执行，其余时间跳过直接结束
- resource/pipeline/WeeklyTasks.json：
  1. 新增 WeeklyTask_ScheduleEnabled 门控节点：Custom recognition ScheduleRecognition（go-service 提供，同 AbsorpScheduleEnabled/Trade_CookScheduleEnabled 模式），attach 七天全 false 由选项写入，next=[进入我的小屋]
  2. WeeklyTask_Start.next 从 [进入我的小屋, [JumpBack]StartGame] 改为 [WeeklyTask_ScheduleEnabled, RTF_LobbyEnd]——门控命中进任务，不命中 RTF_LobbyEnd（OCR 付费商店，主界面独有）命中后死端绿字结束，不走 StartGame 锚点避免误带（迭代17教训）
- tasks/WeeklyTasks.json：新增 select 选项 WeeklyTaskSchedule，7 case（周一~周日），每个 attach 对应一天 true，默认周一；单选而非 checkbox
- Validate：28 节点 0 错误（唯一警告=原有 ToEquipment_WM template 标量写法）；tasks JSON 通过
- 行为：选定星期=ScheduleRecognition 命中→正常执行全链；非选定星期=门控不命中→RTF_LobbyEnd 命中→任务绿字结束（角色留在主界面）
- 教训：①"跳过任务"用主界面 OCR 终结节点（RTF_LobbyEnd）比 StartGame 锚点干净，避免锚点候选误命中 ②select 单选门控=每 case attach 单天 true，比 checkbox 多选更简单直接

## 2026-10-08 迭代24：一条龙快速狩猎3选项合并为1个刷图策略

- 用户需求：DailyAll 中 ChooseDouble（是否刷史莱姆）、ChooseWood（第7章矿石/第9章原木）、ChooseSlime（米饭刷材料策略）3 个选项合并为 1 个 select，4 个 case：刷取史莱姆（仅在有加成时）/刷取金币（仅在有加成时）/只刷金币/只刷装备材料；节点跳转参数由用户自行完善
- tasks/DailyAll.json：
  1. task.option 移除 ChooseDouble/ChooseWood/ChooseSlime，新增 ChooseHuntStrategy
  2. option 块删除 3 个旧选项定义，新增 ChooseHuntStrategy（4 case：SlimeOnBuff/GoldOnBuff/GoldOnly/MaterialOnly，pipeline_override 全为空 {} 待用户填写），默认 GoldOnBuff
- JSON 校验通过；原旧 case 的跳转目标（ChooseGorS/ChooseWorO_Swipe/FastHunt1 等）仅在删除的选项中引用，pipeline 文件未动
- 教训：选项合并时把旧 case 的 override 目标节点名保留在记忆/claims 中，用户填新 case 时可复用（ChooseGorS→史莱姆/金币分支、FastHunt1→材料分支、ChooseWorO_Swipe→矿石/原木）
- 补充（同日）：用户澄清 ChooseWood 不应合并，已原样恢复（task.option 重新加入 ChooseWood，2 case WoodHunt/OreHunt、默认 WoodHunt）；最终快速狩猎相关选项 = ChooseHuntStrategy（刷图策略4选1）+ ChooseWood（原木/矿石2选1）

## 2026-10-06 迭代21：六日常任务合并为单入口 DailyAll（不改 pipeline）

- 用户要求：进入游戏/每日免费抽抽乐/日常收菜/镜中之战PVP/分解+精炼装备/快速狩猎 六项合并为软件 UI 一个入口，保留各任务全部可选菜单，**不修改 pipeline 文件**，任务链接力执行
- 交付物：**新增 `tasks/DailyAll.json`（唯一改动）**，name `DailyAll`，entry 仍为 `StartGame`，5 个可选项原样汇总（设置鸡尾酒倍数/ChooseRock/ChooseWood/ChooseSlime/ChooseDouble，option 定义逐字复用原任务文件）
- 接力机制（全部经任务级 pipeline_override，零 pipeline 改动）：
  1. `StartGame.next` 保留原 21 项，末尾追加 5 个子链入口 anchor：`StartDraw → StartSociaty → StartPVP → StartBag → StartHunt`；原各链本以 `[JumpBack]StartGame` 汇到该 anchor，汇点识别全部落空后落到下一个 anchor 接力
  2. 5 个入口 anchor 均加 `max_hit:1`，防链内 JumpBack 回跳后重入（anchor+max_hit 模式同 ImageMove 先例）
  3. 收菜/PVP 专用终点 anchor（无 next）`SquareEnd`/`Finish`/`FinishPVP` 补 `next:[StartGame]` 接回汇点；抽卡/装备/狩猎链末本就直接跳 StartGame 无需补
- 施工事故纠正：首轮误在**主工作区**（main）施工并改了主工作区 interface.json，发现后已全部撤回（主工作区 `git status` 恢复 clean），任务文件迁移至本 worktree；开工前已核对本 worktree 未提交的 6 个 pipeline 改动中关键节点（StartGame.next 21 项、5 入口 anchor、3 终点 anchor、5 选项覆盖点）与 main 基线完全一致，override 引用有效
- 注意：`[JumpBack]StartGame` 只是跳回 anchor 而非自动回主菜单的动作，StartGame 的识别项在非登录/非主菜单界面（如广场、仓库内页）全部不命中；前三段交接（进游戏→抽卡→收菜→PVP）为作者原有连续场景设计，**PVP→装备（广场能否点到背包）、装备→狩猎（IntensifyMain 退出后界面）两段需实机验证**；失败时 anchor 依次耗尽后任务干净终止，不乱点
- JSON 合法性与字段完整性已校验（StartGame.next=26 项、5 选项齐全）

### 需主工作区统一加的 interface.json 改动（合并清单）

- `import` 数组中**移除** 6 个旧入口：`tasks/StartGame.json`、`tasks/Draw.json`、`tasks/DailyRoutine.json`、`tasks/PVP.json`、`tasks/Bag.json`、`tasks/HuntingArea.json`
- **新增** 1 项：`tasks/DailyAll.json`（建议放 import 数组首位，作日常首任务）
- agent 数组无变化；6 个旧任务 JSON 文件**保留在仓**（回退/单跑用），只是不再注册入口
- 与本分支现有 interface 在制改动（trader agent + MapTrade/WeeklyTasks/SellPremium/CookMeals 四个 import）都落在 import/agent 数组，合并时需手工拼接，无语义冲突

### 2026-10-06 用户授权：worktree 内提前改 interface 验证入口

- 用户明确授权为实机验证入口，本 worktree `interface.json` 的 import 已执行上述替换（6 旧入口 → DailyAll，trader 批次 import 保留），校验：JSON 合法、13 个 import 文件全部存在
- 即本分支 interface.json 现含三类在制改动叠加：①多语言弱化（已提交）②trader agent + 4 import（未提交）③DailyAll 入口替换（未提交）；合并时注意分组

## 2026-10-07 迭代22：DailyAll 返工——pipeline 收尾还原 + StartGame 枢纽调度（根治顺序错乱/二次循环）

### 实机证据（日志 debug/maafw.log 08:13:02–08:21:31 + 实机视频 2026-10-07 08-13-12.mkv）

- 现象：8.5 分钟内任务近**两轮**，实际顺序为 登录弹窗处理→收菜→PVP(1战)→装备(完整)→狩猎(完整)→抽抽乐(08:18才跑)→收菜2→PVP2(FreeNO放弃)→装备2→狩猎2(米饭0空转)
- 根因1（顺序错乱）：用户在 MPE 把跨链接线写进了 pipeline（Draw 末→StartSociaty、收菜末→StartPVP、PVP末→StartHunt、狩猎末→StartBag），且任务**启动时游戏还在登录界面**；日志实证 `[JumpBack]X` 指向从未命中的 anchor 时会**当场把该 anchor 作为新节点执行**（anchor=DirectHit 无视画面必中）→ StartDraw 未跑就级联进 StartSociaty，后续顺序全由画面识别碰运气（PVP 战场房间内 StartHunt/StartBag 都不匹配，靠 StartGame 的 BackHome 点右上 home 图标回主页后先进了 Bag）
- 根因2（二次循环）：跨链 anchor 无 max_hit、无唯一前驱，Draw 结束（DrawGoBack→StartSociaty）后所有链被完整再跑一遍；PVP FreeNO 分支 PVPBack 只点 1 次返回（4.5s）后停在**阵形设置界面**，Worship 0.21<0.3 不中，FinishPVP 空 next anchor 立即命中又被用户接了 StartHunt
- 帧证据：t=187s PVP 退出后在镜战酒吧房间（非广场非主页）、t=194s BackHome 点 home 后回主页、t=418s 第二次 PVP 取消后停在阵形设置、t=496s 第二轮狩猎在米饭不足弹窗空转
- 关键机制再认识（同日日志实证）：入口 anchor 自带的 `[JumpBack]StartGame` 是"纠正返回"语义——StartSociaty 在登录界面失败→JumpBack StartGame 跑完登录/关弹窗流程（08:13:05→08:13:21）后，框架**自动回到调用方 StartSociaty 重认列表**（InSociaty_2 08:13:22 命中）。用户 MPE 错在让 StartDraw 直接 JumpBack 到 StartSociaty，登录完成后返回的是错误的调用方，抽卡被永久跳过

### 改动（4 pipeline 收尾语义还原 + DailyAll.json 枢纽式重写）

- `Draw/DailyRoutine/PVP/HuntingArea.json` 收尾接线**逐节点还原为 HEAD 版（StartGame 结尾）**：Draw 4 处、收菜 3 处 next+on_error 并恢复被删的 SquareEnd 节点、PVP 3 处 StartHunt→Finish 并恢复被删的 Finish 节点+FinishPVP 去掉 next、狩猎 5 处 StartBag→StartGame；external marker 同步还原/删除
- **保留用户 MPE 延时调参**（EnterDraw 4500、ClickMyHouse 4500、EnterOperating 3000/repeat2、PVPWaiting 3000、GoOut/PVPBack 4500 等共 14 处）及画布坐标；StartGame.json（7 处 post_delay 调参）与 Bag.json（4 处 post_delay/坐标）与收尾无关，**未动**
- 还原后逐字段 diff HEAD：4 文件节点数 7/23/36/35 一致，除延时时门外零语义差异
- `tasks/DailyAll.json` 最终方案＝**StartGame 单一枢纽**（entry=StartDraw，顺序 抽抽乐→收菜→PVP→狩猎→装备），override 仅 11 项、无任何自建导航/轮询节点：
  - `StartGame.next` 保留原 21 个登录纠正项，末尾按序追加 5 个入口 anchor（StartDraw→StartSociaty→StartPVP→StartHunt→StartBag）；纠正节点在列表前部，登录/弹窗/非主页画面由它们先命中，主界面才轮到入口 anchor
  - 5 个入口 anchor 各加 `max_hit:1`：枢纽每次只放行顺序上第一个未消耗的链；max_hit 只阻止枢纽重入，**不影响 JumpBack 纠正后对调用方列表的自动重试**，所以登录中启动也不会跳过抽抽乐
  - 链尾 sink 补 next 回枢纽：SquareEnd/魔兽赛季奖励确认_1/Finish/FinishPVP → StartGame；抽卡/狩猎/装备链尾本就裸指 StartGame（还原后），零覆盖
  - PVPBack 改 next [Worship,PVPBack,FinishPVP]+max_hit5，解决 FreeNO 取消后阵形→大厅→广场需多次返回
- 起止双向兜底：起始靠 5 入口自带的 `[JumpBack]StartGame`（BackHome/GoBack/关弹窗纠正完回调用方重试），末尾统一回 StartGame 枢纽；某链始终进不去时其 anchor 额度耗尽即跳下一链，任务不报错不卡死
- 无环分析：5 个入口单次消耗严格单调推进、PVPBack 自环上限 5、其余全为原有纠正环；任务在第 5 链结束回枢纽、5 anchor 均耗尽且无纠正项可命中时自然成功收尾
- JSON 合法、override 引用全部解析通过

### 迭代内被推翻的中间方案（勿采用）

- 曾设计 10 个 DailyAll_* 虚拟节点（5 收尾 marker 唯一前驱 + WaitDraw 轮询 + HomeA/HomeB 自建点 home/back 兜底）：功能可行但绕开了框架自带纠正机制、维护面大；用户指出入口自带 JumpBack StartGame 后已废弃，改为上述枢纽方案

### 【实机证伪 2026-10-07 12:17/12:21】枢纽方案两次失败，勿采用

- RUN1（12:17，登录中启动）：ClickContinue 后的过渡帧上 StartGame 列表 21 个识别项瞬间全不中，追加的 StartSociaty anchor 必中→0.7s 内 4 个入口 anchor 全部误烧（max_hit1 在正确时机反而无法再放行），且经 JumpBack 层层嵌套，纠正完成后**按嵌套逆序弹回**→实际顺序 装备→狩猎→PVP，抽卡/收菜永久跳过
- RUN2（12:21，主界面启动 20 秒结束）：BackHome(HomeClick.png 0.4) 在主界面真命中（该图标常驻主界面右上，点击=**进入小屋**，非回主页）；EnterGame(Shadow.png) 在主界面中央黑影装饰上 **0.974 误命中**；抽卡 AllDraw OCR 全空（**用户确认：当天免费抽已用完时本就没有该 UI，跳过是正常行为**）→ Equipment 兜底后链脱出，任务秒结束
- 用户确认的关键语义：**EnterGame 黑影不是登录节点，而是"已到主菜单"锚点**（无动作空 next，命中即终止/弹回）；StartGame 是只该被 `[JumpBack]` 调用的纠正子程序，**next 里绝不能放接力 anchor**
- 失败容错策略（用户拍板）：纠正后仍进不去→**跳过该链继续**

## 2026-10-07 迭代23：DailyAll 终版——黑影闸门（StartGame 零改动）

- `StartGame.json` 及 5 个 pipeline **一行不动**；所有调度只在 `tasks/DailyAll.json`（override 22 项）
- 新增 4 个虚拟节点：
  - `DailyAll_AtHome`：TemplateMatch Shadow.png（roi/threshold 与 EnterGame 完全一致，无动作）——用户钦定的主菜单判据；**只有它命中才说明在主菜单**
  - `DailyAll_Gate`（anchor）：next=[AtHome, DailyAll_Retry]。不在主菜单→Retry 纠正
  - `DailyAll_Retry`（anchor, max_hit8）：next=[[JumpBack]StartGame]；纠正到黑影后 JumpBack 弹回调用方。全局 8 次预算：入口 anchor 的兜底列表 [入口特征…, Retry, Gate] 纠正耗尽即跳链；Gate 处耗尽（游戏彻底卡死回不到主菜单）才终止任务
  - `DailyAll_End`：空 anchor 收尾
- AtHome.next=[StartDraw, StartSociaty, StartPVP, StartHunt, StartBag, End]；5 入口 anchor max_hit1，**只能由 AtHome 放行**（StartDraw 除外＝task entry）→顺序单调、结构无环；anchor 在非主界面绝无机会必中误烧
- 5 入口 anchor 的 next 覆盖为 [原特征项…, DailyAll_Retry, DailyAll_Gate]（保留自带 JumpBack 语义但改由 Retry 限次发起）
- 链内**全部**裸 StartGame 脱出点改指 Gate：Draw 的 EnterDraw/Equipment/DrawGoBack（含"免费抽已抽完→正常跳过"路径）、收菜 SquareEnd/魔兽赛季奖励确认_1、PVP Finish/FinishPVP、狩猎 MainHunt1_Max_2/**MainHunt2**（漏网，next 原本是 [REWARD_3,StartGame]）/REWARD_3/REWARD_4、Bag IntensifyMain；JumpBack StartGame 的纠正引用全部原样保留；PVPBack 维持 max_hit5+[Worship,PVPBack,FinishPVP]
- 校验：JSON 合法、引用全解析、合并图中 5 链零裸 StartGame 边（残留裸边均属活动/钓鱼等无关 pipeline）、各 sink 全部汇聚 Gate、StartGame 不在 override 键中
- 场景推演：登录中启动(StartDraw→Retry→登录纠正→弹回→抽卡第一棒)；抽卡已抽完(进页面→AllDraw空→Gate→SG 的 GoBack 点返回→下一链)；收菜止于广场(Gate→Retry→GoBack 回主页→PVP)；PVP FreeNO(PVPBack×5→FinishPVP→Gate→纠正回主页→狩猎)

### 【实机证伪 2026-10-07 17:45】Retry→JumpBack StartGame 在主菜单死循环，勿采用

- 日志 L1612–16256：登录中启动，Retry JumpBack StartGame 后登录纠正正常（StartWaiting2×7、ClickContinue、SevenDays），17:46:04 黑影出现起 **EnterGame↔StartGame 互相弹跳 69 次**（各 ~0.44s/帧）直到用户手停；StartDraw x1 后再也进不去抽卡
- 框架语义实证：JumpBack 子程序只有在**整张 next 列表全部识别失败**时才弹栈回调用方（08:13 登录中能返回 StartSociaty 正因如此）；若列表中**裸 EnterGame（空 next 叶子）命中**，叶子只弹回上一级 StartGame anchor 重扫→EnterGame 再中→永不弹栈。即"已到主菜单时 JumpBack 整个 StartGame = 必死乒乓"
- 附带问题：entry=StartDraw + max_hit1 导致入口 anchor 在纠正开始前就把唯一额度消耗，纠正回来后 AtHome 永远轮不到抽卡

## 2026-10-07 迭代24：DailyAll 终版 v2——入口即闸门 + 内联纠正（不 JumpBack StartGame）

- `entry` 改为 `DailyAll_Gate`（执行顺序第一棒仍是抽卡：AtHome 放行表首位 StartDraw；entry 节点名不影响任务展示 label）
- 删除 DailyAll_Retry；虚拟节点仅剩 Gate/AtHome/End：
  - `DailyAll_Gate`（anchor，max_hit120，post_delay1000）next=[AtHome, 13 个 `[JumpBack]` 点击型纠正（DownloadNewver/DownLoading/StartWaiting1/2/ClickContinue/2/BackHome/Cancle/RejectAward/RejectAnnouncement/SevenDays/GoBack/GoBack2）, Gate 自环]。纠正项全部 JumpBack 调用：叶子执行完**回到 Gate 列表继续扫描**而非弹栈；末位裸 Gate 自环兜底，120 轮（约 2 分钟，登录实测约 16 轮）封顶
  - **刻意排除**：EnterGame（乒乓元凶）、InShadow（其 next 首位 EnterGame）、Quit（无 max_hit 自环风险）、GoOut/CorrectPVP/PVPAward/SeasonReward/CorrectWarcraft（PVP 上下文由 PVP 链自管）
  - AtHome 参数不变（Shadow.png 同 EnterGame roi、默认阈值，无动作）；End 空 anchor
- 5 入口 anchor 保持 max_hit1，next=[原特征项…, Gate]（删掉 Retry）；链尾 12 处 sink 仍汇聚 Gate；链内**中间节点**自带的 JumpBack StartGame（MyhouseBack/StartOperate/StartSquare）保留——它们只在小屋/广场等无黑影子界面触发，EnterGame 不中→列表落空正常弹栈，属框架验证过的原有纠正
- 17:45 事故场景重放：Gate→AtHome 不中→StartWaiting2 JumpBack 调用(pd2s)→回列表→自环→…→ClickContinue→SevenDays→主菜单 AtHome 命中→StartDraw（此刻才消耗 max_hit）→EnterDraw→抽卡第一棒；全流程无 EnterGame 参与，乒乓不可能发生
- 校验：JSON 合法、引用全解析、Retry 残留 0、override 中 StartGame 键 0、裸 StartGame 引用 0

### 待实机验证

- 登录中启动 → 黑影 → 抽卡第一棒（17:45 同场景复跑）
- 正常主界面启动时 Gate→AtHome 一帧放行的速度
- 弹窗遮挡黑影瞬间误触 BackHome 进小屋后，次轮 GoBack 自恢复

## 2026-10-07 迭代25：闸门 v2 实机验证通过（18:02 跑）+ 广场女神像超时修复 + skill 固化

- 18:02:02–18:04:58 实机（entry=DailyAll_Gate，主界面启动）：74 命中、ret=true，**顺序完全正确** 抽抽乐(今日已抽，EnterDraw→Equipment 正常跳过)→收菜→PVP(FreeNO，本次 Worship 膜拜 0.3+ 成功，WorshipMove1/2 滑动→Finish)→狩猎(圣石/原木/米饭一次/金币本选项全部生效)→装备→DailyAll_End；Gate/AtHome 各 6 次、BackHome/GoBack 各 2 次均自恢复；无乒乓无循环
- 唯一漏项：**广场女神像奖励未领**。日志 L3849–4350：EnterSquare 点击后 MoveSquare(DAYgift.png, th0.6) 11 次尝试分数 0.33→0.29×9（入场动画）→**0.54→0.54 仍在爬升时窗口关闭**，18:03:12.9 on_error→SquareEnd。窗口长度 12.1s 恰等于 **EnterSquare 节点 timeout=12000**——MaaFW 点击后等待 next 列表的时长受当前节点（而非 next 节点）timeout 控制
- 修复：`DailyRoutine.json` EnterSquare timeout 12000→**30000**（不动阈值 0.6、不动模板；`timeout` 是节点级字段，曾误置 recognition.param 已纠正）；JSON 校验通过。等待女神像入场动画播完即可命中→GetSquare/GetDaily_3 领取
- 已固化项目 skill：`.trae/skills/maafw-chain-task-gate/SKILL.md`（闸门调度模板+6 条日志实证语义+审计清单+日志取证方法）

## 2026-10-07 迭代26：DailyAll 接入赛季活动战斗链（第 6 链）

- `tasks/DailyAll.json`：AtHome 放行表追加 StartWarcraft（装备链之后、End 之前），5→6 入口；StartWarcraft max_hit1，next=[EnterWarcraft, EnterWarcraft2, DailyAll_Gate]（原 [JumpBack]StartGame 移除——主菜单失配时必乒乓）；魔兽链 3 个裸 StartGame 沉点改 Gate：WarcraftBeginClick、活动AP已使用_1、返回活动主页1；内部推关逻辑（是否开启自动推关→挑战/普通战斗推关、自动战斗保持循环、返回战斗关表、WarcraftBattleSwipe、CorrectWarcraft）零改动
- 选项「开启自动战斗」原样并入 DailyAll（select，默认不开启，case 覆盖 是否开启自动推关.next=[不开启/开启]，实节点无需建虚拟）；description 顺序更新为 6 链
- 审计：JSON 合法、引用全解析、6 入口 naked 前驱唯一（均为 AtHome）、魔兽链零裸 StartGame 边、Warcraft.json 与 StartGame.json 零改动

## 2026-10-08 迭代27：DailyAll 推翻闸门方案——直接用 StartGame 作 entry（用户拍板纠错）

### 14:24/14:31 两次实机证伪闸门方案

- 14:24 RUN1：登录后到主菜单，**Gate↔BackHome↔GoBack 三角循环 11 轮**（14:24:23→14:25:28）：每轮 Gate 必中→AtHome(黑影) 不中→BackHome(HomeClick.png th0.4 主菜单右上小屋图标) 命中→点进小屋→GoBack(Back.png) 命中→点退出小屋→Gate 必中→...永远在小屋和主菜单之间拉扯
- 14:31 RUN2：同样 BackHome×4/GoBack×4 循环，9 命中即停
- 根因：Gate 列表只抄了 StartGame 原 21 项里的 13 项，漏抄 EnterGame/InShadow/Quit/GoOut/CorrectPVP/PVPAward/SeasonReward/CorrectWarcraft 8 项——其中 InShadow 是从卡带/子界面回主菜单的关键纠正路径；BackHome 在主菜单常驻、阈值 0.4 极易误中、点击=进小屋（非回主页），Gate 列表里只有 BackHome/GoBack 两个"只会互相拉扯"的纠正项，缺其它路径就死循环
- 用户一句话点破：「为什么不直接用 StartGame 和 EnterGame，要新建 DailyAll_Gate/DailyAll_AtHome，抄节点又不抄完整」

### 17:45 死循环根因重新认定（纠正迭代24 的误判）

- 迭代24 把 17:45 的 69 连弹归因为"JumpBack StartGame 在主菜单必死循环"——**这是错的**。真正根因是 DailyAll_Retry 这个中间层 anchor：Retry(必中)→JumpBack StartGame→EnterGame 命中(叶子)→弹栈回 Retry→Retry 必中又 JumpBack StartGame→...永动。如果当时直接 Gate→[JumpBack]StartGame（无 Retry 中间层），EnterGame 命中弹栈回 Gate，Gate 的 AtHome 会命中放行入口，不会死循环
- 整个迭代24 的设计前提（新建 Gate/AtHome/Retry、不碰 StartGame）基于错误根因，因此设计了多余的虚拟节点和漏抄的纠正列表，制造了 14:24 的新死循环

### 正确方案（迭代27，用户钦定）

- `tasks/DailyAll.json`：entry=**StartGame**（直接用现成 anchor），不新建任何虚拟节点
- pipeline_override 仅 22 项：
  - `EnterGame.next` 覆盖为 [StartDraw, StartSociaty, StartPVP, StartHunt, StartBag, StartWarcraft]——EnterGame 原本无动作空 next（独立任务里命中即终止=已到主菜单），DailyAll 里覆盖让其命中后放行 6 入口（各 max_hit1，顺序恒定）；StartGame 原 21 项 next（含 EnterGame 首位）一字不改，全部纠正节点（InShadow/BackHome/GoBack/Quit/GoOut/CorrectPVP/PVPAward/SeasonReward/CorrectWarcraft）原样生效，不存在漏抄
  - 6 入口 anchor 各 max_hit1（原 next 保留，JumpBack StartGame 纠正不变）
  - 12 处链尾 sink 改指 StartGame（DrawGoBack/EnterDraw/Equipment/SquareEnd/魔兽赛季奖励确认_1/Finish/FinishPVP/MainHunt1_Max_2/MainHunt2/REWARD_3/REWARD_4/IntensifyMain/WarcraftBeginClick/活动AP已使用_1/返回活动主页1）
  - PVPBack max_hit5+[Worship,PVPBack,FinishPVP]
  - 6 个 option 原样保留（含 ChooseSlime Once case 修 MainHunt1_Once→MainHunt_Max——这是原 DailyAll 一直存在的孤儿引用 bug，一并修掉）
- 机制：StartGame 必中→扫原 21 项 next→登录中 EnterGame 不中、StartWaiting2/ClickContinue 等纠正→主菜单 EnterGame 命中→执行 EnterGame.next 放行表→抽卡第一棒；某链进不去→纠正回主菜单→EnterGame 又命中→放行下一未消耗入口；6 链全耗尽→EnterGame 命中但 next 全空→任务成功收尾
- 校验：JSON 合法、引用全解析、StartGame 不在 override 键中、零 DailyAll_* 虚拟节点

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

### ✅ interface.json import（已在分支内补齐，主控无需再改）

合并时由本分支直接带入，import 数组末尾：

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

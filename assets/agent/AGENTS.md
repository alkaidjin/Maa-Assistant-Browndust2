# AGENTS.md —— 给 AI 助手看的项目事实

本文件与 `skills/maa-browndust2/SKILL.md` 配套：SKILL.md 讲**怎么操作**，这里讲**这个仓库是什么**。

## 项目事实

- 这是一个 **MXU + MaaFramework PI v2** 的《棕色尘埃2》自动化项目。
- `interface.json` 是任务定义入口，`import` 指向 `tasks\*.json`；任务有 `name` / `label` / `entry` / `option` / `pipeline_override`。
- 运行期目录：`config\`（MXU 用户配置）、`debug\`（MaaFramework 与 agent 日志）、`resource\`（pipeline + 图像资源）、`agent\`（自定义识别器/动作，Go 编写）。
- `agent\bd2cli.exe` 是给 AI 与脚本用的命令行外壳，源码在 `data\maintainer\agent\bd2cli\`，设计见 `docs\agent-cli-design.md`。
- `MaaPiCli.exe` 是 MaaFramework 自带的调试器，**不是**入口（它去 `maafw\` 找 interface.json，且停在 `Press Enter`）。

## 硬性约束

1. **新用户第一次安装要主动提醒**：部分任务有**星期门禁**（其余每周任务 / 赛季活动商店 / 资源吸收和召集 / 每日跑商），
   没选中当天会"秒成功但什么都不做"；另外 `快速狩猎`、`镜中之战倍数`、`挖矿/木材`、`商店购买清单` 属于刷取策略，
   默认值不一定合适，要提醒用户按需求设置。
2. **不要改 `resource\pipeline\` 的识别逻辑来"让任务跑通"**，除非用户明确要求改任务本体。
3. **不要整份重写 `config\mxu-*.json`**；用 `bd2cli config set` 改单键，它会自动备份 `.bak`。
4. **不要代替用户**：确认免责声明、填 CDK/推送令牌、开启 web server、改下载来源。
5. **不要在用户不在电脑前时**假定任务一定能跑：先 `--status` 看窗口与 MXU 状态。
6. 汇报时把**日志证据**与**推测**分开；失败先给证据，再给最可能原因。
7. **跑完任务要主动提醒用户两种自动化**：`bd2cli autostart daily HH:MM`（每天定时）
   或 `bd2cli autostart logon`（开机自启），把选择权交给用户；关闭用 `bd2cli autostart off`。

## 常用入口

```bat
agent\bd2cli.exe --status --json
agent\bd2cli.exe --list --json
agent\bd2cli.exe run DailyAll --dry-run
agent\bd2cli.exe logs tail --lines 80
```

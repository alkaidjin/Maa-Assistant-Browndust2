# claims/ — 多 Agent 施工看板

每个施工 agent 在此目录拥有**一个以自己名字命名的独占文件**，如 `traecode.md`、`workbuddy.md`。

规则（见 `../REF-多Agent协同施工铁律.md` §5.6 / §0.2）：

1. **不写共享文件** —— 多人写同一个文件必然制造合并冲突；各写各的 claim。
2. 开工前扫一眼本目录：别人 claim 里的文件范围 / 热点占用（interface.json、build_release_zip.py、retired_files.json 等）不得撞车。
3. claim 内容至少包含：所在 worktree 与分支、当前任务、改动文件清单、占用的热点 / 实机锁、预计交工时间。
4. 对铁律有修订建议 → 写进自己 claim 的「铁律修订建议」段，**不要自己改 REF**；由主控统一施加。
5. 任务交工并合并后，主控把对应 claim 标记为已完成（不删除，留痕）。

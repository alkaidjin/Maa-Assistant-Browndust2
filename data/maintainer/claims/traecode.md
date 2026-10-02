# claim: traecode（TRAE）

- **worktree**：`F:\MABd2-wt\traecode-workspace`
- **分支**：`agent/traecode-workspace`
- **状态**：待开工（worktree 已于 2026-10-02 建好）

## 当前任务

1. （计划）JSON / 资源一致性校验脚本：新增 `data/maintainer/tools/validate_resources.py`，
   校验 14 个 tasks + 14 个 pipeline + interface.json 可解析、`entry` / `next` /
   `custom_action` / `custom_recognition` 引用的节点存在、模板图片路径存在、跨文件节点无重名。
   —— 全新文件，不碰任何热点，不与其他任务冲突。

## 改动文件范围

- 首个任务仅新增 `data/maintainer/tools/validate_resources.py`（及其自身说明，如需要）。
- 后续任务开工前在此追加，并先扫本目录其他 claim。

## 热点占用 / 实机锁

- 不占用任何热点文件（interface.json / build_release_zip.py / retired_files.json / .github 等只读）。
- 首个任务纯静态检查，不需要实机锁。

## 铁律修订建议

- （暂无）

## 交工记录

- （暂无）

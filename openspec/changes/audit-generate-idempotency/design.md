## Context

审计 M3：generate 以"文件存在"判断已生成，误伤 year-close 预生成空文件，且无法识别"内容已含数据"的中间态。审计 M4：无活动月份跳过月结行，账本月份感缺失。

## Goals / Non-Goals

**Goals:**
- 幂等判定升级为内容级（"本月合计"行），year-close 空文件放行
- 无活动但有余额的 GL 科目补月结行（0/0 + 期末=期初）
- 不破坏三路生成（MergeGL/ML）现有行为

**Non-Goals:**
- 不改 ML/MergeGL 的无活动月份处理（保持现状，文档注明）
- 不清理历史已生成的账本
- 不做打印版相关改动

## Decisions

**D1 内容级幂等判定 `AlreadyGenerated(path)`**：扫描工作薄所有 Sheet 所有单元格，存在"本月合计"字符串 → true。year-close 空文件无任何数据 → false → 放行。调用点：cmd/generate.go 不带 `-f` 且文件存在时；检测失败（文件损坏）→ 按未生成处理放行，交由 NewWorkbook 决定（错误会在生成时报出）。

**D2 无活动科目补月结行（仅 GL）**：`WriteMonthClosings` 构造 `extendedActivity`（activity 副本 + 期初≠0 且无分录且存在 GL Sheet 的科目，act={0,0}）与 `extendedChanged`（对应 sheet 加入）。遍历 extendedActivity，用 extendedChanged 判定。**不修改入参** activity/changedSheets → MergeGL/ML 后续处理不受影响。补行位置 = sheet 数据区末尾（上月月结行后），月末"期末余额=期初+0-0=期初"。

**D3 补行条件**：期初≠0（initials[account] != 0）且无当月分录且 GL Sheet 存在。期初=0 的无活动科目不补（无余额无存在感，避免噪声）。

## Risks / Trade-offs

- **AlreadyGenerated 性能**：全量扫描大文件较慢，但仅在"文件存在且不带 -f"的罕见路径触发，可接受。
- **补月结行的分页影响**：无活动科目行数少，一般不满页；若满页会触发正常翻页（checkBreak 复用），无风险。
- **打印标记**：补月结行的 sheet 标记为需打印（加入 extendedChanged），打印逻辑与正常月结一致。
- **行为变化**：year-close 后无 -f 的 generate 从"拒绝"变为"放行"——符合 year-close 设计意图（预生成空文件供首月生成）。

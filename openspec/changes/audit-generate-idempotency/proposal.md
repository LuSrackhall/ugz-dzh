## Why

审计（docs/audit-2026-08-26.md）M3/M4：

- **M3 幂等**：`generate` 当前仅以"当月 xlsx 文件是否存在"判断已生成（cmd/generate.go else 分支）。但 year-close 会预生成空的新年首月 xlsx——按此判断新年首月会被误判"已生成"而拒绝（需带 -f）。同时"文件存在"无法识别"文件内容已含当月数据"的中间态。应改为检测当月"本月合计"行：已含 → 拒绝并要求 -f；空文件（year-close 预生成）→ 放行。
- **M4 无活动月份**：科目当月无分录时跳过月结行，账本中月份之间缺行（上月月结行后直接接更晚月份数据），易误读为连续数据。应补"本月合计 0/0 + 期末余额=上期"行，使账本逐月清晰。

## What Changes

1. **M3 幂等增强**：新增 `generator.AlreadyGenerated(path) (bool, error)`——扫描工作薄是否含"本月合计"行。`cmd/generate.go` 不带 `-f` 且当月文件存在时：含本月合计 → 报错"已生成过，使用 -f"；不含（year-close 空文件）→ 放行直接生成。
2. **M4 无活动月份补月结行**：`WriteMonthClosings` 对"期初≠0 但当月无分录、且存在 GL Sheet"的科目，补"本月合计 0/0 + 本年累计 + 期末余额=期初"行（复用现有月结逻辑，act={0,0}）。使用 activity/changedSheets 的**副本**扩展，不影响 MergeGL/ML 的后续处理。

## Capabilities

### New Capabilities
- `generate-idempotency`: generate 幂等保护——"本月合计"行检测、year-close 空文件放行、无活动月份月结行补齐

### Modified Capabilities
- `cli-commands`: `generate` 的"已生成"判定从文件存在升级为内容检测（本月合计行）

## Impact

- `generator/workbook.go` 或新文件：`AlreadyGenerated`
- `generator/monthly_close.go`：`WriteMonthClosings` 无活动科目补月结行（副本扩展）
- `cmd/generate.go`：else 分支改内容检测
- 新增测试：`AlreadyGenerated`、无活动月结行、year-close 后无 -f 生成、同月重跑被拒
- 不变：生成骨架、期初机制（Change 1）、借贷平衡校验（Change 2）；ML/MergeGL 无活动月份保持现状（文档注明）

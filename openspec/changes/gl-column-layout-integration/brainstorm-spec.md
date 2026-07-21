## Context

当前总分类账 Sheet 的写入处于「半集成」状态：`writeGLTitle` 已使用 Layout 坐标（`FrontStartCol=3`，col C~J），但 `appendToGLSheet` 及其所有辅助函数仍使用硬编码 col 1-7（A~G）。标题和数据列不齐。

同时，所有读取端辅助函数（`lastPageBalance`、`lastRowIsOrphanBreak`、`lastBreakTotals`、`pageStartRow` 等）通过 `excelize.GetRows()` 的固定 slice index 访问数据（如 `row[2]` 查过次页，`row[6]` 读余额），这些索引在引入装订列后不会自动偏移，需要同步更新。

三 agent 评估结论：
- 代码审查：中低风险，所有文件覆盖到位
- 宪法合规：总体合规，打印标记列位置为灰色地带
- 测试影响：需要新增"写入→读回"测试拦截回归

## Goals / Non-Goals

**Goals:**
- 将 GL、MergeGL、ML 的所有硬编码列号替换为 Layout 坐标
- 同步修复 GetRows 读取端索引偏移，使用 `lay.BindingLeftCols` 计算
- 打印标记列由 `lay.FrontStartCol + len(lay.ExcelColumns)` 推导
- 新增"写入→读回"测试用例保护列映射正确性
- 编译通过，全部测试全绿

**Non-Goals:**
- 不改变数据流逻辑（余额计算、分页判断、月结计算逻辑）
- 不改 year_close.go（已知遗留债务，change 4 清理）
- 不改 LayoutSpec 定义
- 不改 ML 打印标记列（独立 `mlPrintMarkCol()` 函数不受影响）
- 不做正面/反面并排（change 3）

## Decisions

### 决策 1：写入端统一使用 `lay.FrontStartCol + offset`

`cellName(N, row)` → `cellName(lay.FrontStartCol + N-1, row)`

其中 `lay = layout.ComputeLayout(layout.DefaultGLSpec())`。

- 替代方案：将 lay 作为参数传入函数 — 过于侵入，当前只需在 appendToGLSheet 等顶层函数中初始化一次
- 打印标记列：`cellName(lay.FrontStartCol + len(lay.ExcelColumns), row)`，即内容区末列+1

### 决策 2：读取端统一使用 `lay.BindingLeftCols + offset`

| 用途 | 旧索引 | 新索引 |
|---|---|---|
| 查过次页/承前页 | `row[2]` | `row[lay.BindingLeftCols+2]` |
| 读余额 | `row[6]` | `row[lay.BindingLeftCols+6]` |
| 读借方金额 | `row[3]` | `row[lay.BindingLeftCols+3]` |
| 读贷方金额 | `row[4]` | `row[lay.BindingLeftCols+4]` |
| 读明细列 H-U | `row[mlDetailStartCol-1 + i]` | 不变（ML 不涉及装订列偏移） |

### 决策 3：不改 ML 的列号

ML 使用独立列号系统（A-G 基础列 + H-U 扩展列），其 `mlDetailStartCol=8` 和 `mlPrintMarkCol()` 都是固定值，不与 Layout 共用。
ML 内 A-G 列的写入使用 `lay.FrontStartCol` 偏移，但扩展列 H-U 保持原位。

### 决策 4：year_close.go 的遗留处理

`year_close.go` 写 `"上年结转"` 到 A1、余额到 G1，不经过 Workbook 方法。按宪法要求坐标应来源于 Layout，但此文件独立的操作逻辑使其在 change 1 中修改风险大于收益。标记为已知遗留债务，change 4 清理时一并修复。

## Risks / Trade-offs

| 风险 | 缓解措施 |
|---|---|
| 漏改某个读取端索引 | 按顺序修改 → 编译 → 测试循环，每一步确保全绿 |
| `BindingLeftCols` 未来变化需更新所有读取端 | 已统一使用 `lay.BindingLeftCols` 而非硬编码，自动适配 |
| `year_close.go` 遗留到 change 4 可能被遗忘 | 在 CLAUDE.md 宪法和 tasks.md 中明确标记 |
| ML 的 GetRows 读取涉及基础列（col 1-7）+ 扩展列（H-U），复杂 | ML 的基础列用 BindingLeftCols 偏移，扩展列无需偏移（本身不在 Front 区管理） |

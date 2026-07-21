## Context

brainstorm-spec.md 已覆盖高层设计（Context、Goals、Decisions、Risks）。本文件补充实现层面的技术细节：具体文件修改映射、Layout 实例化方式、读取端索引转换表、测试策略。

## Goals / Non-Goals

**Goals:**
- 提供每处修改的精确映射，确保实施时无遗漏
- 明确读取端索引偏移对照表

**Non-Golas:**
- 不重复 brainstorm-spec 的设计决策

## Decisions

### 决策 1：Layout 实例化方式

增加辅助函数 `glLayout()` 统一获取 Layout 实例，避免各处重复调用 `layout.ComputeLayout(layout.DefaultGLSpec())`。所有需要 Layout 坐标的函数使用 `lay := glLayout()`。

### 决策 2：ML 同步迁移

多科目明细账的标题和数据列与 GL 使用相同的 Layout 坐标规则：
- 基础列 A-G（日期~余额）：`FrontStartCol+0~6`
- 明细列 H-U：通过 `mlDetailExcelCol(lay, i)` = `FrontStartCol + 7 + i` 计算
- 打印标记列 V：通过 `mlPrintMarkCol()` = `FrontStartCol + 7 + mlMaxDetails` 计算
- 读取端明细列索引：`mlDetailRowIdx(lay, i)` = `BindingLeftCols + 7 + i`

### 决策 3：辅助函数不做参数化

读取端辅助函数（`lastPageBalance`、`lastRowIsOrphanBreak` 等）保持现有签名，不添加 Layout 参数。每个函数内部调用 `glLayout()` 获取偏移值。

### 决策 4：测试策略

在 `generator_test.go` 中新增 TestGLColumnLayoutConsistency：
1. 用 Layout 坐标写一行完整数据（日期到余额）
2. 调用 GetRows 验证 `row[lay.BindingLeftCols+2]` = 预期摘要，`row[lay.BindingLeftCols+6]` = 预期余额

## Risks / Trade-offs

| 风险 | 缓解 |
|---|---|
| 每个辅助函数单独获取 Layout 实例（小性能开销） | ComputeLayout 是纯函数，计算量非常小，性能影响可忽略 |
| ML 明细列映射复杂（7+14 列结构） | 增加辅助函数 mlDetailExcelCol / mlDetailRowIdx 封装偏移计算，一处修改全局生效 |

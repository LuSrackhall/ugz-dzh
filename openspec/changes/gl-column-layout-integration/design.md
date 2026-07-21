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

在 `appendToGLSheet` 等顶层函数中调用 `layout.ComputeLayout(layout.DefaultGLSpec())` 获取 Layout 实例。不通过函数参数传递，避免侵入性修改。

### 决策 2：ML 写入的特殊处理

ML（`ml_sheet.go`）中写入 A-G 列时使用 `lay.FrontStartCol` 偏移，但扩展列 H-U（列号 8-21）和打印标记列 V（22）保持不动。因为 ML 的扩展列不通过 Layout 管理。

### 决策 3：辅助函数不做参数化

读取端辅助函数（`lastPageBalance`、`lastRowIsOrphanBreak` 等）保持现有签名，不添加 Layout 参数。每个函数内部调用 `layout.ComputeLayout(layout.DefaultGLSpec())` 获取偏移值。

### 决策 4：测试策略

在 `generator_test.go` 中新增 TestGLColumnLayoutConsistency：
1. 用 Layout 坐标写一行完整数据（日期到余额）
2. 调用 GetRows 验证 `row[lay.BindingLeftCols+2]` = 预期摘要，`row[lay.BindingLeftCols+6]` = 预期余额

## Risks / Trade-offs

| 风险 | 缓解 |
|---|---|
| 每个辅助函数单独获取 Layout 实例（小性能开销） | ComputeLayout 是纯函数，计算量非常小，性能影响可忽略 |
| ML 扩展列不通过 Layout，未来 Layout 变化需单独检查 | ML 扩展列是固定 14 列设计，与 Layout 的正反面概念正交 |

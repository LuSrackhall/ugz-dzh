## Why

多科目明细账 `detailOrder` 配置存在两个 Bug，导致用户无法用 `-f` 重建列序：

1. **预分配列不生效**：`ensureMLSheet` 新 Sheet 创建时，`initDetails` 只取 `detailOrder` 与当月分录 `details` 的交集，用户预配置但尚未发生业务的科目（含空字符串跳列）被丢弃。这与原始设计"detailOrder 与科目树解耦"冲突——用户期望配置的列即使无发生额也预占位置。
2. **`-f` 级联断裂**：`-f` 只删除当月 xlsx，后续月份 xlsx 继承旧列布局。下次 `generate` 时标题与 `detailOrder` 不匹配 → 冲突报错。

## What Changes

**detailOrder 预分配列完整展开**
- From: `initDetails` = `detailOrder ∩ 当月details` + 剩余字母序
- To: `initDetails` = `detailOrder` 完整列表（含空字符串、未发生科目），当月不在 `detailOrder` 中的科目追加到右侧空列
- Reason: 实现 design.md D2 和 proposal.md 中"detailOrder 与科目树解耦"的承诺
- Impact: 非破坏性 — 已有 Sheet 路径不受影响，仅新 Sheet 创建路径改变

**`-f` 级联删除后续月份**
- From: `-f` 仅绕过"文件已存在"检查
- To: `-f` 删除当月及之后所有月份的 xlsx，但不删历史月份
- Reason: `-f` 语义是"从此月开始强制重建"，后续月份必须从新布局继承
- Impact: `-f` 非首月时，后续已生成的 xlsx 会被删除（可再生，JSON 不受影响）

## Capabilities

### Modified Capabilities
- `ml-detail-order-config`: detailOrder 完整展开 — 预分配科目（含空字符串跳列）即使当月无发生额也占位列；科目树解耦逻辑更完整

### New Capabilities
<!-- None — bug fix, no new capability -->

## Impact

| 文件 | 改动 |
|---|---|
| `generator/ml_sheet.go` | `ensureMLSheet` 新 Sheet 初始化逻辑：detailOrder 完整展开，未发生科目和空字符串占位保留，剩余科目追加 |
| `cmd/generate.go` | `-f` 分支：级联删除当月及后续所有月份 xlsx |

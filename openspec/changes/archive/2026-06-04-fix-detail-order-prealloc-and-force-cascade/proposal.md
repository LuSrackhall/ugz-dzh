## Why

多科目明细账 `detailOrder` 配置的两个缺陷导致用户无法用 `-f` 重建列布局：(1) 新 Sheet 创建时 `initDetails` 只取 `detailOrder` 与当月分录 `details` 的交集，丢弃了预分配但未发生的科目和空字符串跳列，使"detailOrder 与科目树解耦"原则落空；(2) `-f` 只删当月 xlsx，后续月份继承旧布局后冲突。

## What Changes

**detailOrder 预分配列完整展开**
- From: `initDetails` = `detailOrder ∩ 当月details` + 剩余字母序
- To: `initDetails` = `detailOrder` 完整列表（含空字符串、未发生科目），当月不在 `detailOrder` 中的科目追加到右侧空列
- Reason: 实现 detailOrder 与科目树解耦的完整语义——用户预配置的列即使无发生额也应占位
- Impact: 非破坏性 — 仅新 Sheet 创建路径，已有 Sheet 读头保序不变

**`-f` 级联删除后续月份 xlsx**
- From: `-f` 仅绕过"文件已存在"检查
- To: `-f` 删除当月及之后所有月份的 xlsx
- Reason: 后续月份 xlsx 是从旧布局复制的，必须随当月一起重建
- Impact: `-f` 行为变更，但 xlsx 是纯生成物可重新生成，JSON 余额不受影响

## Capabilities

### Modified Capabilities
- `ml-detail-order-config`: detailOrder 在新 Sheet 创建时 SHALL 完整展开所有项（含空字符串跳列和未发生的预分配科目），而非仅取与当月分录的交集

## Impact

| 文件 | 改动 |
|---|---|
| `generator/ml_sheet.go` | `ensureMLSheet` 第 75-101 行：新 Sheet 初始化时直接使用 `detailOrder` 完整列表 |
| `cmd/generate.go` | `-f` 分支第 104-109 行：级联删除当月及后续所有月份 xlsx |

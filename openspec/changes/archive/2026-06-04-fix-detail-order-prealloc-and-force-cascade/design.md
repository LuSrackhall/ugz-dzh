## Context

`detailOrder` 配置设计原则（来自 stable-ml-detail-columns 变更）：
- 与科目树完全解耦：预分配科目即使尚未发生业务，标题也展示
- 空字符串 `""` 表示跳列占位，新科目不得占用
- `-f` 是冲突检测的逃生口：标题与配置不一致时，用户用 `-f` 从首月重建

当前实现中两个缺陷破坏了这些原则。

## Goals / Non-Goals

**Goals:**
- `detailOrder` 完整展开到新 Sheet 标题行：包括空字符串跳列、尚未发生业务的预分配科目
- `-f` 级联删除当月及后续所有月份 xlsx，保证列布局从当月起一致
- 无 `detailOrder` 配置时行为不变

**Non-Goals:**
- 不改变已有 Sheet 的读头保序逻辑
- 不改冲突检测逻辑
- 不删除历史月份 xlsx（`-f` 只删当月及之后）

## Decisions

### D1: 新 Sheet 初始化直接使用 detailOrder 完整列表

`ensureMLSheet` 新 Sheet 路径中，`initDetails` = `detailOrder` 原样（含空字符串和未发生科目），当月分录中不在 `detailOrder` 的科目追加到右侧第一个空列。

**依据**：`detailOrder` 是用户明确定义的列布局，应对新 Sheet 完全生效。空字符串保留为跳列，预分配科目保留为占位列。

### D2: `-f` 级联删除当月及后续月份 xlsx

`-f` 时，按月份前缀匹配删除 `{yearDir}/*.xlsx` 中当月及之后的所有文件。不删 JSON 配置。

**依据**：`-f` 语义是"从此月起强制重建"，后续月份继承新布局。仅删 xlsx（可再生），JSON 余额和配置不受影响。

## Risks / Trade-offs

| Risk | Mitigation |
|---|---|
| `-f` 非首月时删除了后续已有生成的 xlsx | xlsx 是纯生成物，可通过 `generate` 重新生成；JSON 余额不受影响 |
| detailOrder 完整展开后超 14 列 | 已有校验 `len(initDetails) > mlMaxDetails` 报错，用户需调整配置 |
| 预分配科目名拼写错误导致标题有错误列名 | 属于用户配置责任；`detailOrder` 解耦于科目树，程序不校验科目是否存在 |

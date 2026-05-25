## Why

多科目明细账月结行的"本季合计"和"本年累计"A-G列（总借/贷合计）数值错误——仅包含当月发生额，缺少前几个月的累计数据。根因是 `WriteMLMonthClosings` 中用仅有总账名的 key（如 `"银行存款"`）去查以全路径为 key（如 `"银行存款-工行"`）的累计 map，导致查找始终返回 0。此缺陷导致季末和年末报表数据不完整，影响财务核对准确性。

## What Changes

**多科目明细账月结 — 本季合计/本年累计的 A-G 列累计值**
- From: 用 `general`（总账名）单 key 直接查 `qtdDebit` / `ytdDebit`，key 不匹配始终返回 0
- To: 遍历该 general 下所有明细科目，用 `general + "-" + detail` 全路径聚合 map 中的累计值
- Reason: map 的 key 格式为全路径 `"总账-明细"`，必须用全路径才能正确匹配
- Impact: 非破坏性修复，仅修正数值正确性，不影响 xlsx 结构

## Capabilities

### New Capabilities

无。

### Modified Capabilities

- `excel-generation`: 多科目明细账月结行 A-G 列累计值计算逻辑修正

## Impact

- 修改文件：`generator/monthly_close_ml.go`（约 2 处改动，各 ~6 行）
- 不改变 xlsx 结构、API、或命令行接口
- 不影响总分类账月结逻辑

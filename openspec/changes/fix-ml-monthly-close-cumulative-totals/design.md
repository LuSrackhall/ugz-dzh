## Context

多科目明细账月结行（`WriteMLMonthClosings`）在生成"本季合计"和"本年累计"时，A-G 列的累计值不正确。

**当前状态**：`generate.go` 通过 `ExtractYtdTotals` / `ExtractQuarterlyTotals` 提取截至上月的累计，这些函数接收 `allAccounts`（来自 `GetLeafAccounts` 的叶子科目全路径列表，如 `"银行存款-工行"`），返回的 map key 也是全路径格式。但 `WriteMLMonthClosings` 在计算 A-G 列时只用总账名 `general`（如 `"银行存款"`）去查这些 map，导致 key 不匹配，查找始终返回 0。

明细列 H-U 不受影响 — 它们走 `getDetailPrevQuarterTotal` / `getDetailPrevYearTotal`，直接从 `Config.Tree` 按 `"general-detail"` 全路径查，本身正确。总分类账 `WriteMonthClosings` 不受影响 — 它的 key 格式与 map 一致。

## Goals / Non-Goals

**Goals:**
- 修复多科目明细账"本季合计" A-G 列累计值为正确值（当月 + 截至上月）
- 修复多科目明细账"本年累计" A-G 列累计值为正确值（当月 + 截至上月）

**Non-Goals:**
- 不改明细列 H-U 的计算逻辑（本身正确）
- 不改总分类账月结逻辑
- 不改 `generate.go` 上游数据准备
- 不改 `ExtractYtdTotals` / `ExtractQuarterlyTotals` 底层函数签名

## Decisions

1. **在 `WriteMLMonthClosings` 内部聚合**：遍历 `details` 列表，用 `general + "-" + detail` 全路径去聚合 `qtdDebit` / `ytdDebit`，替代当前直接用 `general` 单 key 查找。
   - 理由：局部修复，不改上游数据格式，风险最小
2. **不改动其他文件**：仅修改 `generator/monthly_close_ml.go`

## Risks / Trade-offs

- 每个 general 需遍历一次其所有明细科目做聚合，但明细数量上限仅 14（`mlMaxDetails`），性能无影响
- 如果未来 `ExtractYtdTotals` 的 key 格式再次变化，这里同样需要适配 — 但当前格式稳定，且改动局限在同一函数内

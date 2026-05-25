## Design Summary

多科目明细账月结行"本季合计"和"本年累计"的 A-G 列（总借/贷合计）数值不正确：
- **根因**：`WriteMLMonthClosings` 中用仅有总账名的 `general`（如 `"银行存款"`）去查 `ytdDebit`/`qtdDebit`，但这些 map 的 key 是 `"总账-明细"` 全路径（如 `"银行存款-工行"`），导致查找始终返回 0
- **影响**："本季合计"和"本年累计"的 A-G 列只算了当月发生额，完全丢失前几个月的累计
- **明细列 H-U 不受影响**：它们走的是 `getDetailPrevQuarterTotal`/`getDetailPrevYearTotal`，直接从 `Config.Tree` 按全路径查，本身正确
- **总分类账不受影响**：`WriteMonthClosings` 的 key 本身就匹配

修复方式：在 `WriteMLMonthClosings` 中计算 A-G 列累计值时，遍历该 general 下的所有明细科目，用全路径 `"general-detail"` 去聚合 `qtdDebit`/`ytdDebit`，而非直接用 `general` 单 key 查找。

## Alternatives Considered

### 方案 A（采用）：在 ML 月结中对所有明细聚合查 map
- **做法**：在 `WriteMLMonthClosings` 中遍历 `details` 列表，用 `general + "-" + detail` 全路径去聚合 `qtdDebit`/`ytdDebit`
- **优点**：不改变上游数据格式，局部修复，风险最小
- **缺点**：每个 general 遍历一次它的所有明细科目

### 方案 B：在 generate.go 中额外构建按总账科目聚合的 map
- **做法**：调用 `WriteMLMonthClosings` 前，从 `ytdDebit`/`qtdDebit`（全路径 key）再聚合出一份按 general 聚合的 map
- **优点**：ML 月结代码不用改循环逻辑
- **缺点**：多一层中间数据，代码路径变长；需要在 generate.go 里知道哪些 general 需要聚合
- **为何未采用**：增加了不必要的数据转换层

### 方案 C：修改底层提取函数同时产出两种 key
- **做法**：`ExtractYtdTotals`/`ExtractQuarterlyTotals` 返回两份 map（全路径 + 总账名）
- **优点**：数据自底向上提供完整
- **缺点**：改动总分类账也依赖的底层函数，影响面更大，返回值变复杂
- **为何未采用**：影响面过大，违反最小改动原则

## Agreed Approach

方案 A — 直接在 `WriteMLMonthClosings` 内部对明细做聚合。仅修改 `generator/monthly_close_ml.go` 一个文件，两处改动：
1. "本季合计" A-G 列计算（第 95-104 行）
2. "本年累计" A-G 列计算（第 129-140 行）

将 `qtdDebit[general]` / `ytdDebit[general]` 单 key 查找改为遍历 `details` 列表用全路径聚合。

## Key Decisions

- 不改明细列 H-U 的计算逻辑（本身正确）
- 不改 `generate.go` 上游数据准备
- 不改 `ExtractYtdTotals`/`ExtractQuarterlyTotals` 底层函数

## Open Questions

无。

## Why

多科目明细账采用跨月累积写入模式，每月 `AppendMLEntries` 和 `WriteMLMonthClosings` 各自按字母序重新排序明细科目并重建列映射（detailIdx），导致第2行标题随当月明细集变动，但历史月份的数据行保持原始列位不变——新旧列位脱节，金额显示在错误的明细列下。核心矛盾是：标题随月变，数据不随月变。需改为累积稳定列策略，从根本上消除错列。

## What Changes

**明细列映射策略**
- From: 每月按字母序重排明细科目，重建 detailIdx，更新标题行
- To: 从第2行标题读取现有列映射并保持不可变，新科目仅追加到右侧空列
- Reason: 历史数据行不再被重写，只有列映射稳定才能保证金额不窜列
- Impact: 非破坏性——现有 xlsx 的列布局将被保留，仅新科目触发列扩展

**detailIdx 来源统一**
- From: AppendMLEntries 和 WriteMLMonthClosings 各自独立按字母序排序构建
- To: 统一从 Sheet 第2行标题读取 detailIdx
- Reason: 消除重复逻辑，保证所有写入路径使用同一套列映射
- Impact: 内部重构，不影响外部行为

**JSON 配置支持**
- From: 无用户可配置的列顺序
- To: 新增可选 `detailOrder` 字段，支持强制列序、空字符串跳列；新科目自动增量回写
- Reason: 给用户完全控制列布局的能力，与科目树解耦
- Impact: 可选特性，未配置时行为与稳定列策略一致

## Capabilities

### New Capabilities
- `ml-stable-columns`: 累积稳定列机制 — 读头保序、新科目右追、冲突检测、超限报错
- `ml-detail-order-config`: JSON detailOrder 配置 — 强制列序、跳列、自动回写、与科目树解耦

### Modified Capabilities
- `excel-generation`: 多科目明细账写入路径改用统一读头映射替代独立排序，月结明细列改用配置余额计算替代从分录重算

## Impact

| 影响面 | 说明 |
|---|---|
| `generator/ml_sheet.go` | 新增 `readMLDetailHeaders`、修改 `ensureMLSheet`/`AppendMLEntries`/`appendToMLSheet` |
| `generator/monthly_close_ml.go` | 修改 `WriteMLMonthClosings`，统一使用读头映射 |
| 配置文件结构 | 新增 `DetailOrder map[string][]string` 字段 |
| `generator/config.go` | 解析 `detailOrder`，新增回写方法 |
| 已有 xlsx | 首次运行时按现有标题保序，与 `detailOrder` 配置冲突则报错要求 `-f` |

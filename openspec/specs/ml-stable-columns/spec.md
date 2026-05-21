# ml-stable-columns Specification

## Purpose
TBD - created by archiving change stable-ml-detail-columns. Update Purpose after archive.
## Requirements
### Requirement: 读头保序
`ensureMLSheet` 在发现 Sheet 已存在时，SHALL 从第2行 H-U 读取现有明细列标题，构建 `detailName → columnIndex` 映射。该映射 SHALL 在后续所有写入中保持不可变——已分配列位的科目绝不移动到其他列。

#### Scenario: Sheet 已存在，标题匹配
- **WHEN** Sheet 第2行 H 列为"办公费"，I 列为"差旅费"，当月明细集同为 {办公费, 差旅费}
- **THEN** 映射保持 H→办公费, I→差旅费，标题不变

#### Scenario: Sheet 已存在，新增明细科目
- **WHEN** Sheet 第2行 H 列为"办公费"，I 列为空，当月明细集包含新科目"交通费"
- **THEN** "交通费"追加到 I 列，H 列"办公费"不动

#### Scenario: Sheet 已存在，明细科目消失
- **WHEN** Sheet 第2行 H 列为"办公费"，I 列为"差旅费"，但当月无差旅费发生额
- **THEN** I 列标题保留"差旅费"，I 列数据行留空，列位不回收

#### Scenario: 全新型 Sheet（不存在）
- **WHEN** 该总账科目无历史 Sheet
- **THEN** 按字母序初始化明细列，后续月份以此为基准

### Requirement: 新科目右追
不在现有标题中的新明细科目 SHALL 追加到 H-U 范围内第一个空列。若存在 `detailOrder` 配置，SHALL 优先按配置顺序填充；配置中未列出的新科目按字母序追加在配置项之后。追加后 SHALL 更新第2行标题。

#### Scenario: 无配置时字母序追加
- **WHEN** 现有 H="A科", I="", J=""，新增 {C科, B科}
- **THEN** I="B科", J="C科"（字母序）

#### Scenario: 有配置时按配置顺序追加
- **WHEN** 现有 H="A科", I="", J=""，detailOrder=["A科","C科","B科"]，新增 {C科, B科}
- **THEN** I="C科", J="B科"（按配置顺序，非字母序）

### Requirement: 超 14 列报错
当所有 14 列（H-U）均被占用且仍有新科目待分配时，`ensureMLSheet` SHALL 返回错误，错误信息 SHALL 包含已占用列清单和可用列数（0）。

#### Scenario: 14列已满
- **WHEN** H-U 所有 14 列均有非空标题，当月又出现新明细科目
- **THEN** 返回错误，消息包含已占用的 14 个科目名和"0 列可用"

### Requirement: 统一映射来源
`AppendMLEntries` 和 `WriteMLMonthClosings` SHALL 统一调用 `readMLDetailHeaders` 获取 `detailIdx` 映射，SHALL NOT 各自独立排序构建。

#### Scenario: 数据行写入使用读头映射
- **WHEN** `appendToMLSheet` 写入数据行
- **THEN** 使用的 `detailIdx` 来自 `readMLDetailHeaders` 返回值

#### Scenario: 月结行写入使用读头映射
- **WHEN** `WriteMLMonthClosings` 写入月结行
- **THEN** 使用的 `detailIdx` 来自 `readMLDetailHeaders` 返回值

### Requirement: 冲突检测
`ensureMLSheet` 在 Sheet 已存在且配置了 `detailOrder` 时，SHALL 逐列比对第2行标题与配置顺序。若不一致（非空标题列的内容与配置对应位置不匹配）SHALL 返回错误，提示使用 `-f` 从首月强制重新生成。

#### Scenario: 标题与配置一致
- **WHEN** 第2行 H="A科", I="B科"，detailOrder=["A科","B科"]
- **THEN** 无错误，正常继续

#### Scenario: 标题与配置不一致
- **WHEN** 第2行 H="B科", I="A科"，detailOrder=["A科","B科"]
- **THEN** 返回错误，消息包含"detailOrder 与现有列序冲突"和"请使用 -f 从首月重新生成"


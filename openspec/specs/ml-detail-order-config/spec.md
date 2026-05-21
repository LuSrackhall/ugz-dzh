# ml-detail-order-config Specification

## Purpose
TBD - created by archiving change stable-ml-detail-columns. Update Purpose after archive.
## Requirements
### Requirement: detailOrder 配置解析
系统 SHALL 支持从 JSON 配置文件读取可选的 `detailOrder` 字段，格式为 `map[string][]string`，键为总账科目名，值为明细科目名列表。列表 SHALL 支持空字符串 `""` 表示跳列占位。

#### Scenario: 读取完整配置
- **WHEN** JSON 配置包含 `"detailOrder": {"应收账款": ["宜阳供电公司", "周站强", ""]}`
- **THEN** 解析得到应收账款列序：H=宜阳供电公司, I=周站强, J=空（跳列占位）

#### Scenario: 无 detailOrder 配置
- **WHEN** JSON 配置中不存在 `detailOrder` 字段
- **THEN** 系统正常运行，按字母序分配列

### Requirement: detailOrder 与科目树解耦
`detailOrder` 配置 SHALL 独立于科目树（Tree）。配置中指定的科目即使不在当月的科目树中，也 SHALL 在第2行标题中占位列。该列的数据行 SHALL 留空。

#### Scenario: 配置科目尚无发生额
- **WHEN** detailOrder=["A科","B科"]，但当月仅 A科 有发生额
- **THEN** H="A科", I="B科"，I 列数据行留空

#### Scenario: 配置科目从未发生
- **WHEN** detailOrder=["预留科目"]，但该科目在科目树中从未存在
- **THEN** H="预留科目"，标题正常显示，数据行全空

### Requirement: 空字符串跳列
`detailOrder` 列表中的空字符串 `""` SHALL 表示占位列，该列标题 SHALL 为空，且新科目 SHALL NOT 占用此列。

#### Scenario: 配置包含跳列
- **WHEN** detailOrder=["A科","","B科"]
- **THEN** H="A科", I="", J="B科"，新科目只能追加到 K 及之后

#### Scenario: 新科目不占跳列
- **WHEN** detailOrder=["A科","","B科"]，新增科目"C科"
- **THEN** C科追加到 K 列，I 列仍为空

### Requirement: 新科目自动回写
当 `AppendMLEntries` 发现新科目（不在现有标题中也不在 `detailOrder` 中）时，SHALL 将其追加到 `detailOrder` 配置末尾并按新顺序写回 JSON 配置文件。回写 SHALL 保持 JSON 结构完整，仅增量追加。

#### Scenario: 新科目回写到 detailOrder
- **WHEN** detailOrder=["A科","B科"]，新增科目"C科"被追加到右侧空列
- **THEN** JSON 文件中的 detailOrder 更新为 ["A科","B科","C科"]

#### Scenario: 无 detailOrder 时的首次回写
- **WHEN** JSON 配置中无 detailOrder 字段，新增科目导致列分配
- **THEN** 在 JSON 中创建 detailOrder 字段并填入当前所有列标题


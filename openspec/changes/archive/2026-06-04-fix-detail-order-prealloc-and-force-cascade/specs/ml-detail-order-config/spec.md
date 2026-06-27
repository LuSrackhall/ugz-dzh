## MODIFIED Requirements

### Requirement: detailOrder 与科目树解耦
`detailOrder` 配置 SHALL 独立于科目树（Tree）。配置中指定的科目即使不在当月的科目树中，也 SHALL 在第2行标题中占位列。该列的数据行 SHALL 留空。

当创建全新 Sheet（该科目无历史 Sheet）时，`initDetails` SHALL 直接使用 `detailOrder` 完整列表，包含空字符串 `""` 跳列项和尚未发生业务的预分配科目名。当月分录中不在 `detailOrder` 内的科目 SHALL 按字母序追加到右侧第一个空列。

#### Scenario: 配置科目尚无发生额
- **WHEN** detailOrder=["A科","B科"]，但全新型 Sheet 当月仅 A科 有发生额
- **THEN** H="A科", I="B科"，I 列数据行留空

#### Scenario: 配置科目从未发生
- **WHEN** detailOrder=["预留科目"]，但该科目在科目树中从未存在
- **THEN** H="预留科目"，标题正常显示，数据行全空

#### Scenario: 配置含跳列，当月无该科目
- **WHEN** detailOrder=["A科","","B科"]，全新型 Sheet，当月仅 A科 有发生额
- **THEN** H="A科", I="", J="B科"，I 列为空跳列

#### Scenario: 配置外新科目追加
- **WHEN** detailOrder=["A科","B科"]，全新型 Sheet，当月分录包含 {A科, B科, C科}
- **THEN** H="A科", I="B科"，C科 追加到 J 列

#### Scenario: 无 detailOrder 时行为不变
- **WHEN** 无 detailOrder 配置，全新型 Sheet，当月 {B科, A科}
- **THEN** H="A科", I="B科"（字母序）


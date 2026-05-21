# excel-generation Delta Spec

## MODIFIED Requirements

### Requirement: 多科目明细账 Sheet 生成
对存在明细科目的总账科目，系统 SHALL 生成多科目明细账 Sheet。基础列 A-G 同总分类账；扩展列 H-U 对应该总账科目下的各明细科目，每列填入该笔分录在对应明细下的金额。

明细列映射 SHALL 使用累积稳定列策略：
- 已有 Sheet 时，SHALL 从第2行标题读取现有列→科目映射并保持不可变
- 新明细科目 SHALL 追加到右侧第一个空列
- 若配置了 `detailOrder`，SHALL 按配置顺序占列

#### Scenario: 多明细科目分录汇总
- **WHEN** 总账科目"管理费用"下有"办公费"和"差旅费"两个明细
- **THEN** Sheet 的 H 列为"办公费"，I 列为"差旅费"，每笔分录在对应列填入金额

#### Scenario: 父级汇总行
- **WHEN** 多科目明细账 Sheet 生成
- **THEN** 该总账科目下所有明细的借贷合计 SHALL 汇总为一个父级行，显示在明细行之前

#### Scenario: 新增明细科目追加到右侧
- **WHEN** 历史 Sheet H 列为"办公费"，I 列为空，当月新增"交通费"
- **THEN** I 列标题更新为"交通费"，H 列"办公费"不动，历史数据行保持原列位

#### Scenario: 明细科目消失后列位保留
- **WHEN** 历史 Sheet H="A科", I="B科"，但当月 B科 无发生额
- **THEN** I 列标题保留"B科"，I 列数据行留空，列位不回收

### Requirement: 月结处理
每月生成结束时，系统 SHALL 在每个有变化的 Sheet 末尾添加"本月合计"行和"本年累计"行。

多科目明细账的月结行 SHALL 统一从 Sheet 第2行标题读取明细列映射，确保月结行的明细列数据与标题一致。

#### Scenario: 月末结账
- **WHEN** 当月所有分录追加完毕
- **THEN** 每个有变化的 Sheet 末尾追加本月合计行（借方合计、贷方合计）和本年累计行

#### Scenario: 多科目明细账月结列对齐
- **WHEN** 多科目明细账 Sheet 追加月结行
- **THEN** 本月合计、本季合计、本年累计行的明细列 SHALL 与第2行标题的科目一致

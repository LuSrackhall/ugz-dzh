## ADDED Requirements

### Requirement: GL 数据列使用 Layout 坐标写入

系统 SHALL 确保所有 GL/MergeGL/ML 的数据行写入使用 `lay.FrontStartCol + offset` 坐标。

#### Scenario: 数据写入列映射正确
- **GIVEN** `lay = ComputeLayout(DefaultGLSpec())`
- **WHEN** 向总分类账 Sheet 写入一条分录
- **THEN** 日期写入 `lay.FrontStartCol+0`、凭证号写入 `lay.FrontStartCol+1`、摘要写入 `lay.FrontStartCol+2`、借方金额写入 `lay.FrontStartCol+3`、贷方金额写入 `lay.FrontStartCol+4`、方向写入 `lay.FrontStartCol+5`、余额写入 `lay.FrontStartCol+6`

### Requirement: GetRows 读取使用 BindingLeftCols 偏移

系统 SHALL 确保所有通过 `GetRows` 读取列数据的辅助函数使用 `lay.BindingLeftCols + offset` 索引。

#### Scenario: 过次页标记读取正确
- **GIVEN** 过次页行写入 `lay.FrontStartCol+2`（摘要列）
- **WHEN** 调用 GetRows 后按索引读取
- **THEN** `row[lay.BindingLeftCols+2]` 等于 "过次页"

#### Scenario: 期末余额读取正确
- **GIVEN** 期末余额写入 `lay.FrontStartCol+6`（余额列）
- **WHEN** 调用 GetRows 后按索引读取
- **THEN** `row[lay.BindingLeftCols+6]` 等于预期余额值

### Requirement: 打印标记列位置由 Layout 推导

系统 SHALL 确保总分类账的打印标记列位置由 `lay.FrontStartCol + len(lay.ExcelColumns)` 计算，而非硬编码。

#### Scenario: 打印标记列使用计算位置
- **GIVEN** `lay.ExcelColumns` 有 8 列且 `lay.FrontStartCol=3`
- **WHEN** 标记数据行为"需打印"
- **THEN** 标记写入列号为 `3+8=11`（col K）

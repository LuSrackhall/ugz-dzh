## ADDED Requirements

### Requirement: 金额分栏转换函数

系统 SHALL 提供 `centsToDigits` 纯函数，将金额（int64 分）拆为 12 位数字数组，对应"十亿千百十万千百十元角分"。

#### Scenario: 零值转换
- **WHEN** 输入金额为 0 分
- **THEN** 返回 [0,0,0,0,0,0,0,0,0,0,0,0]

#### Scenario: 正整数转换
- **WHEN** 输入金额为 123456 分（1234.56 元）
- **THEN** 返回 [0,0,0,0,0,0,1,2,3,4,5,6]

#### Scenario: 大额转换
- **WHEN** 输入金额为 999999999999 分（9999999999.99 元）
- **THEN** 返回 [9,9,9,9,9,9,9,9,9,9,9,9]

#### Scenario: 负数转换
- **WHEN** 输入金额为 -123456 分
- **THEN** 返回 [0,0,0,0,0,0,1,2,3,4,5,6]（取绝对值，方向由"方向"列标记）

### Requirement: 金额单元格写入函数

系统 SHALL 提供 `writeAmountCells` 函数，将金额写入 Excel 的 12 个连续单元格。

#### Scenario: 写入正数金额
- **WHEN** 调用 writeAmountCells 写入 123456 分到第 3 行第 4 列
- **THEN** D3 到 O3 单元格分别为 0,0,0,0,0,0,1,2,3,4,5,6

#### Scenario: 写入零值金额
- **WHEN** 调用 writeAmountCells 写入 0 分
- **THEN** 所有 12 个单元格为空或 0

### Requirement: 金额显示格式化函数

系统 SHALL 提供 `formatAmountForDisplay` 函数，将金额格式化为带千分位的显示字符串（用于调试）。

#### Scenario: 格式化正数
- **WHEN** 输入 123456789 分
- **THEN** 返回 "1,234,567.89"

#### Scenario: 格式化零
- **WHEN** 输入 0 分
- **THEN** 返回 "0.00"

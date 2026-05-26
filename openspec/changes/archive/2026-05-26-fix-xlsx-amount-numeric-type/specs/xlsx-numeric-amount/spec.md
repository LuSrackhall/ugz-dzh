## ADDED Requirements

### Requirement: xlsx 金额单元格 SHALL 使用数字类型写入

所有通过 `SetCellValue` 写入 xlsx 的金额单元格 MUST 使用 `float64` 数字类型，而非 `string` 文本类型，以确保 Excel 原生支持数值运算、筛选和图表。

#### Scenario: 生成月结行金额为数字
- **WHEN** 生成某月 xlsx 后打开文件，查看"本月合计"行的借贷金额列
- **THEN** 单元格值 SHALL 为数字类型（Excel 中默认为数值、可参与 SUM 求和、右对齐）

#### Scenario: 生成多科目明细账金额为数字
- **WHEN** 生成某月 xlsx 后打开文件，查看多科目明细账 Sheet 的数据行和月结行
- **THEN** 所有借贷金额列和明细科目金额列的单元格值 SHALL 为数字类型

#### Scenario: 期初期末表金额为数字
- **WHEN** 生成某月 xlsx 后打开文件，查看期初表和期末表的余额列
- **THEN** 余额单元格值 SHALL 为数字类型

### Requirement: 分转元函数 SHALL 返回 float64

`centsToYuan` 函数 MUST 接受 `int64`（分）并返回 `float64`（元），实现 `float64(c) / 100`。

#### Scenario: 正常金额转换
- **WHEN** 调用 `centsToYuan(123456)`
- **THEN** 返回 `1234.56`（float64），精确到小数点后两位

#### Scenario: 零值转换
- **WHEN** 调用 `centsToYuan(0)`
- **THEN** 返回 `0.0`（float64）

#### Scenario: 负金额转换
- **WHEN** 调用 `centsToYuan(-5000)`
- **THEN** 返回 `-50.0`（float64）

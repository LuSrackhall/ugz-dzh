## ADDED Requirements

### Requirement: A4 横向页面设置

系统 SHALL 在创建总分类账 Sheet 时设置 Excel 页面布局为 A4 横向，页边距为 0。

#### Scenario: 页面布局为 A4 横向
- **GIVEN** 一个新生成的 xlsx 文件
- **WHEN** 通过 `GetPageLayout` 查看总分类账 Sheet
- **THEN** `PaperSize` 为 9（A4），`Orientation` 为 landscape

#### Scenario: 页边距为 0
- **GIVEN** 一个新生成的 xlsx 文件
- **WHEN** 通过 `GetPageMargins` 查看总分类账 Sheet
- **THEN** 上、下、左、右边距均为 0

### Requirement: 列宽换算符合 A4 可打印范围

系统 SHALL 使用 `pxPerColUnit = 7.0` 进行 mm 到 Excel 列宽单位的换算，确保 Front + 间隙 + Back 总宽度不超过 ~155 列宽单位（A4 横向可打印上限）。

#### Scenario: 列宽换算系数为 7.0
- **GIVEN** `GLMMToExcelColWidth(mm)` 被调用
- **WHEN** 参数 mm 为任意正数
- **THEN** 返回值为 `mm * (96.0/25.4) / 7.0`

### Requirement: 外侧边缘红色双线贯穿整页

系统 SHALL 在每页的外侧边缘添加红色双线边框，覆盖从上边距到底边距的整列。

#### Scenario: 正面页左侧红色双线
- **GIVEN** 一个总分类账 Sheet
- **WHEN** 查看正面页（奇数页）
- **THEN** "月"列（FrontStartCol+0）的左边框为红色双线（Style: 6），覆盖从上边距第 1 行到底边距最后 1 行

#### Scenario: 正面页右侧无边框
- **GIVEN** 一个总分类账 Sheet
- **WHEN** 查看正面页（奇数页）
- **THEN** 最后一个"✓"列（FrontStartCol + glColCount - 1）的右边框为无边框

#### Scenario: 背面页右侧红色双线
- **GIVEN** 一个总分类账 Sheet
- **WHEN** 查看背面页（偶数页）
- **THEN** 最后一个"✓"列（BackStartCol + glColCount - 1）的右边框为红色双线（Style: 6），覆盖从上边距第 1 行到底边距最后 1 行

#### Scenario: 背面页左侧无边框
- **GIVEN** 一个总分类账 Sheet
- **WHEN** 查看背面页（偶数页）
- **THEN** "月"列（BackStartCol + 0）的左边框为无边框

### Requirement: setNoBorder 函数主动清除边框

系统 SHALL 提供 `setNoBorder` 函数，用于主动清除指定单元格的左右边框，确保对面侧无残留边框。

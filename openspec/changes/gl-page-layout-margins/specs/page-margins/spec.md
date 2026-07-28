## ADDED Requirements

### Requirement: 每页四边有独立边距空行/空列

系统 SHALL 确保每个物理页（正面页和背面页）的四边都有用于模拟 A4 纸边距的空行或空列。

#### Scenario: 正面页左边距为空列
- **GIVEN** 一个总分类账 Sheet
- **WHEN** 查看列布局
- **THEN** BindingLeftCols 为 2（列 A-B），且正面页第一个数据列（FrontStartCol）在 BindingLeftCols 之后

#### Scenario: 正面页上边距为 3 空行
- **GIVEN** 一个总分类账 Sheet
- **WHEN** 查看行布局
- **THEN** 上边距存在 3 行空行，列标题从其上开始

#### Scenario: 正面页下边距为 3 空行
- **GIVEN** 一个总分类账 Sheet
- **WHEN** 过次页行之后
- **THEN** 下边距存在 3 行空行

### Requirement: 边距常量定义

系统 SHALL 定义 `TopMarginRows = 3` 和 `BottomMarginRows = 3` 常量。所有行偏移量（`DataStartRow`、`HeaderRow`、`SubHeaderRow`）都基于这些常量计算。

#### Scenario: 数据起始行包含上边距
- **GIVEN** `TopMarginRows = 3`
- **WHEN** `DataStartRow` 被使用
- **THEN** `DataStartRow = 5 + TopMarginRows = 8`

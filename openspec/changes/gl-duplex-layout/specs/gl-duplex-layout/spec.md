## ADDED Requirements

### Requirement: 偶数页数据写入 BackStartCol 区

系统 SHALL 确保总分类账偶数页的数据行和标题行写入 `BackStartCol + offset` 列。

#### Scenario: 偶数页数据写入正确位置
- **GIVEN** pageNum=2（偶数），`lay.BackStartCol=11`
- **WHEN** 写入一条分录
- **THEN** 日期写入 col 11、凭证号写入 col 12、摘要写入 col 13、借方写入 col 14、贷方写入 col 15、方向写入 col 16、余额写入 col 17

#### Scenario: 奇数页数据写入 FrontStartCol
- **GIVEN** pageNum=1（奇数），`lay.FrontStartCol=3`
- **WHEN** 写入一条分录
- **THEN** 日期写入 col 3、凭证号写入 col 4...（当前行为不变）

### Requirement: 偶数页标题在 BackStartCol 区

系统 SHALL 确保偶数页的标题行（分第 n 页、总分类账、科目名称、列标题）写入 BackStartCol 起始的范围。

#### Scenario: 偶数页标题列范围
- **GIVEN** pageNum=2
- **WHEN** writePageHeader 执行
- **THEN** 标题列范围为 `lay.BackStartCol ~ BackStartCol + nCol - 1`

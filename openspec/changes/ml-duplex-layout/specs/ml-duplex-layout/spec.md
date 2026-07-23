## ADDED Requirements

### Requirement: ML 数据行双区写入

系统 SHALL 确保多科目明细账的数据行同时写入 Back 区（7 基础列 + 4 明细）和 Front 区（10 明细）。

#### Scenario: 同一样 Row 双区数据
- **GIVEN** lay.BackStartCol=11, lay.FrontStartCol=3
- **WHEN** 写入一条分录
- **THEN** 基础列写 BackStartCol+0~6、明细 1~4 写 BackStartCol+7~10、明细 5~14 写 FrontStartCol+0~9

### Requirement: ML 标题区左右双区

系统 SHALL 确保 ML 标题区在 Back 区和 Front 区各自独立显示。

#### Scenario: 标题区内容
- **GIVEN** 逻辑页新页
- **WHEN** 写入标题区
- **THEN** Back 区顶部：科目名称、分第 N 页(左)；Front 区顶部：明细帐、科目名称、分第 N 页(右)

### Requirement: 首张正面空白占位

系统 SHALL 确保首次创建 ML Sheet 时，第一行（正面）为空白占位行。

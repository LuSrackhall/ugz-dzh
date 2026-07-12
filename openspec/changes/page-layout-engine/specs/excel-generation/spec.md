# excel-generation Specification (Delta)

## Purpose
本文件记录相对于 `openspec/specs/excel-generation/spec.md` 的需求变更。未列出的需求保持不变。

## Requirements

### Requirement: 总分类账标题使用 LayoutEngine 坐标
#### Scenario: 标题行使用计算坐标
- **GIVEN** LayoutEngine 已实现
- **WHEN** 生成总分类账 Sheet 的标题行（"总分类账"、"分第 n 页"、"科目名称"）
- **THEN** 所有行列坐标从 `ComputeLayout(layoutSpec)` 获取，不使用硬编码常量

### Requirement: 分页与过次页（重写实现方式）
#### Scenario: 每页标题由代码写入
- **GIVEN** 总分类账 Sheet
- **WHEN** 新一页开始（过次页之后）
- **THEN** 代码写入"承前页"行 → 标题行（分第 n 页、总分类账、科目名称）→ 列标题行 → 数据行
- **AND** 不使用 Excel 打印重复行功能

#### Scenario: 正/反面并排同 Sheet
- **WHEN** 总分类账 Sheet 生成
- **THEN** 正面（奇数页）内容在左半区域，反面（偶数页）内容在右半区域
- **AND** 中间保留页间隙空列

### Requirement: 列样式使用 ComputeLayout
#### Scenario: 金额分栏列动态
- **GIVEN** LayoutSpec 中 AmountDigitCount = N
- **WHEN** 渲染金额分栏列
- **THEN** 每列宽度 = 金额区总宽 / N，列起始位置由 ComputeLayout 计算

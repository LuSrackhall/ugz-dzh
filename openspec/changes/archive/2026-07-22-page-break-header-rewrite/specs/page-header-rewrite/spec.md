## ADDED Requirements

### Requirement: 过次页后代码写入标题行

系统 SHALL 在每次过次页后由代码写入新页标题行。

#### Scenario: 过次页后标题行写入
- **GIVEN** 总分类账 Sheet 数据满 20 行
- **WHEN** 触发过次页（writePageBreakRow）+ 承前页（writeCarryForwardRow）
- **THEN** 下一行开始写入 writePageHeader（分第 n 页、总分类账、科目名称、列标题）

### Requirement: 页码动态递增

系统 SHALL 为每页分配正确的页码。

#### Scenario: 第一页页码
- **GIVEN** 新创建的总分类账 Sheet
- **WHEN** 写入标题
- **THEN** 显示"分第 1 页"

#### Scenario: 第二页页码
- **GIVEN** 已有 20 行数据，触发过次页
- **WHEN** 写入新页标题
- **THEN** 显示"分第 2 页"

### Requirement: pageStartRow 偏移更新

系统 SHALL 确保 pageStartRow 的返回值适配标题行占用的行空间。

#### Scenario: 标题行占位不影响数据行计数
- **GIVEN** 过次页在行 R
- **WHEN** 调用 pageStartRow
- **THEN** 返回 R + 1 + TitleRowCount + 1（跳过承前页、标题、列标题）

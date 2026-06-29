# html-print-generation Specification

## Purpose
TBD - created by archiving change add-print-versions. Update Purpose after archive.
## Requirements
### Requirement: HTML 打印版生成

系统 SHALL 生成 HTML 格式的打印版文件，支持精美样式和正反面打印布局。HTML 文件 SHALL 输出到 `output/{year}/html/` 子目录。

#### Scenario: 文件命名
- **WHEN** 生成 2026-01 的 HTML 打印版
- **THEN** 输出文件为 `output/2026/html/2026-01-银行存款-print.html`

#### Scenario: 单文件结构
- **WHEN** 生成 HTML 打印版
- **THEN** CSS 样式嵌入 `<style>` 标签，不依赖外部文件

### Requirement: HTML 模板设计

系统 SHALL 使用 Go `html/template` 渲染 HTML 打印版，支持左右对开布局。

#### Scenario: 左右页布局
- **WHEN** 生成 HTML 打印版
- **THEN** 左页（`.page-left`）包含日期/凭证号/摘要，右页（`.page-right`）包含金额分栏

#### Scenario: 金额分栏展示
- **WHEN** 某行借方金额为 123456 分
- **THEN** 右页显示 12 个独立的 `<span class="amount-cell">` 元素

### Requirement: CSS 打印样式

系统 SHALL 提供 CSS 打印样式，支持 A4 横向打印和正反面布局。

#### Scenario: 页面设置
- **WHEN** 用户打印 HTML 文件
- **THEN** 默认纸张为 A4 横向，页边距为 10mm

#### Scenario: 奇偶页边距
- **WHEN** 用户双面打印
- **THEN** 左页（奇数页）右边距为 5mm，右页（偶数页）左边距为 5mm

#### Scenario: 表格样式
- **WHEN** 用户打印 HTML 文件
- **THEN** 表格边框为 1px 实线 #000，单元格内边距为 2px 4px

### Requirement: HTML 模板嵌入

系统 SHALL 使用 `embed.FS` 将 HTML 模板嵌入到二进制文件中。

#### Scenario: 模板嵌入
- **WHEN** 编译程序
- **THEN** HTML 模板文件嵌入到二进制文件，运行时无需外部文件

#### Scenario: 模板渲染
- **WHEN** 调用 HTML 生成函数
- **THEN** 从 embed.FS 读取模板，渲染数据后输出 HTML 文件

### Requirement: HTML 月结处理

系统 SHALL 在 HTML 打印版末尾添加"本月合计"行和"本年累计"行。

#### Scenario: 本月合计行
- **WHEN** 当月所有分录渲染完毕
- **THEN** 末尾添加"本月合计"行，金额使用分栏展示

#### Scenario: 本年累计行
- **WHEN** 当月所有分录渲染完毕
- **THEN** 末尾添加"本年累计"行，金额使用分栏展示


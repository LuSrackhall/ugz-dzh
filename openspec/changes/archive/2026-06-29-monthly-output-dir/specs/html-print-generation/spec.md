## MODIFIED Requirements

### Requirement: HTML 打印版生成

系统 SHALL 生成 HTML 格式的打印版文件，支持精美样式和正反面打印布局。HTML 文件 SHALL 输出到 `output/{year}/{month}/html/` 子目录。

#### Scenario: 文件命名
- **WHEN** 生成 2026-01 的 HTML 打印版
- **THEN** 输出文件为 `output/2026/2026-01/html/2026-01-银行存款-print.html`

#### Scenario: 单文件结构
- **WHEN** 生成 HTML 打印版
- **THEN** CSS 样式嵌入 `<style>` 标签，不依赖外部文件

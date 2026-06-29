## ADDED Requirements

### Requirement: HTML 输出目录管理

系统 SHALL 将 HTML 打印版文件输出到 `output/{year}/html/` 子目录，与 Excel 文件分离。

#### Scenario: 自动创建子目录
- **WHEN** 生成 HTML 打印版
- **THEN** 系统自动创建 `html/` 子目录（如果不存在）

#### Scenario: 输出路径
- **WHEN** 生成 2026-01 的 HTML 打印版
- **THEN** 输出文件为 `output/2026/html/2026-01-银行存款-print.html`

#### Scenario: 目录结构
- **WHEN** 生成 HTML 打印版
- **THEN** HTML 文件与 Excel 文件分离，Excel 文件在 `output/2026/`，HTML 文件在 `output/2026/html/`

## ADDED Requirements

### Requirement: 按月分目录输出

系统 SHALL 将每个月的输出文件存放在独立的月度目录中，目录名为月份标识（如 `2026-01`）。

#### Scenario: 创建月度目录
- **WHEN** 生成 2026-01 的账本
- **THEN** 系统自动创建 `output/2026/2026-01/` 目录

#### Scenario: 月度目录结构
- **WHEN** 生成 2026-01 的账本
- **THEN** 月度目录包含：查看版 xlsx、打印版 xlsx、ledger.csv、ledger.xlsx、balance.csv、balance.xlsx、html/ 子目录

#### Scenario: 年份配置位置
- **WHEN** 生成 2026-01 的账本
- **THEN** 年份配置文件 `2026.json` 保持在 `output/2026/` 目录

### Requirement: 上月 xlsx 查找

系统 SHALL 从上月的月度目录中查找上月 xlsx 文件。

#### Scenario: 查找上月 xlsx
- **WHEN** 生成 2026-02 的账本
- **THEN** 系统从 `output/2026/2026-01/2026-01.xlsx` 查找上月 xlsx

#### Scenario: 首月无上月 xlsx
- **WHEN** 生成 2026-01 的账本（首月）
- **THEN** 系统不查找上月 xlsx，新建空白工作薄

### Requirement: force 级联删除

系统 SHALL 在 force 模式下删除当月及之后月份的月度目录。

#### Scenario: 级联删除当月及之后月份
- **WHEN** 使用 `-f` 参数生成 2026-02 的账本
- **THEN** 系统删除 `output/2026/2026-02/`、`output/2026/2026-03/` 等目录

#### Scenario: 不删除之前的月份
- **WHEN** 使用 `-f` 参数生成 2026-02 的账本
- **THEN** 系统不删除 `output/2026/2026-01/` 目录

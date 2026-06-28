## MODIFIED Requirements

### Requirement: `generate` sub-command SHALL produce monthly ledger outputs

The `ledger generate` sub-command SHALL accept `-voucherDir`, `-month`, `-json`, `-output` flags AND `-view-only`, `-print-only`, `-html-only` flags to control which output versions are generated.

#### Scenario: generate without flags produces all versions
- **WHEN** user runs `ledger generate -voucherDir ./vouchers -month 2026-01 -json config.json -output ./output`
- **THEN** `output/2026-01.xlsx` (查看版), `output/2026-01-print.xlsx` (打印版), `output/2026-01-print.html` (HTML版) are all created

#### Scenario: generate with -view-only produces only view version
- **WHEN** user runs `ledger generate -month 2026-01 -json config.json -output ./output -view-only`
- **THEN** only `output/2026-01.xlsx` is created

#### Scenario: generate with -print-only produces only print Excel version
- **WHEN** user runs `ledger generate -month 2026-01 -json config.json -output ./output -print-only`
- **THEN** only `output/2026-01-print.xlsx` is created

#### Scenario: generate with -html-only produces only HTML version
- **WHEN** user runs `ledger generate -month 2026-01 -json config.json -output ./output -html-only`
- **THEN** only `output/2026-01-print.html` is created

#### Scenario: generate with missing voucherDir fails
- **WHEN** user runs `ledger generate` without `-voucherDir`
- **THEN** the command returns non-zero exit code with error message

## ADDED Requirements

### Requirement: 生成所有版本（默认行为）

`ledger generate` 命令默认 SHALL 生成所有三套输出（查看版、打印版 Excel、HTML 版）。

#### Scenario: 默认生成所有版本
- **WHEN** 用户运行 `ledger generate -month 2026-01 -json config.json -output ./output`
- **THEN** 同时生成 `2026-01.xlsx`、`2026-01-print.xlsx`、`2026-01-print.html`

### Requirement: 按需生成单个版本

`ledger generate` 命令 SHALL 支持 `-view-only`、`-print-only`、`-html-only` 参数，仅生成指定版本。

#### Scenario: 仅生成查看版
- **WHEN** 用户运行 `ledger generate -month 2026-01 -json config.json -output ./output -view-only`
- **THEN** 仅生成 `2026-01.xlsx`，不生成打印版和 HTML 版

#### Scenario: 仅生成打印版 Excel
- **WHEN** 用户运行 `ledger generate -month 2026-01 -json config.json -output ./output -print-only`
- **THEN** 仅生成 `2026-01-print.xlsx`，不生成查看版和 HTML 版

#### Scenario: 仅生成 HTML 版
- **WHEN** 用户运行 `ledger generate -month 2026-01 -json config.json -output ./output -html-only`
- **THEN** 仅生成 `2026-01-print.html`，不生成查看版和打印版 Excel

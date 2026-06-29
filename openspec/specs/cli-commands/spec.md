# cli-commands Specification

## Purpose
TBD - created by archiving change stage4-cli-commands. Update Purpose after archive.
## Requirements
### Requirement: CLI SHALL provide a unified `ledger` binary with cobra sub-commands

The system SHALL expose a single `ledger` binary built on `spf13/cobra` that routes to sub-commands via `ledger <command>` syntax.

#### Scenario: bare invocation prints help
- **WHEN** user runs `ledger` without any sub-command
- **THEN** cobra prints the root help listing all registered sub-commands and exits zero

#### Scenario: help flag on any command
- **WHEN** user runs `ledger generate --help`
- **THEN** cobra prints generate-specific help including all flags and exits zero

### Requirement: `generate` sub-command SHALL produce monthly ledger outputs

The `ledger generate` sub-command SHALL accept `-voucherDir`, `-month`, `-json`, `-output` flags AND `-view-only`, `-print-only`, `-html-only` flags to control which output versions are generated. Output files SHALL be stored in monthly subdirectories.

#### Scenario: generate without flags produces all versions
- **WHEN** user runs `ledger generate -voucherDir ./vouchers -month 2026-01 -json config.json -output ./output`
- **THEN** `output/2026/2026-01/2026-01.xlsx` (查看版), `output/2026/2026-01/2026-01-print.xlsx` (打印版), `output/2026/2026-01/html/` (HTML版) are all created

#### Scenario: generate with -view-only produces only view version
- **WHEN** user runs `ledger generate -month 2026-01 -json config.json -output ./output -view-only`
- **THEN** only `output/2026/2026-01/2026-01.xlsx` is created

#### Scenario: generate with -print-only produces only print Excel version
- **WHEN** user runs `ledger generate -month 2026-01 -json config.json -output ./output -print-only`
- **THEN** only `output/2026/2026-01/2026-01-print.xlsx` is created

#### Scenario: generate with -html-only produces only HTML version
- **WHEN** user runs `ledger generate -month 2026-01 -json config.json -output ./output -html-only`
- **THEN** only `output/2026/2026-01/html/` is created

### Requirement: `check` sub-command SHALL validate account tree integrity

The `ledger check` sub-command SHALL validate `科目余额总览.json`'s account tree consistency using `balance.ValidateAccountTree` and print diagnostics.

#### Scenario: check on valid config
- **WHEN** user runs `ledger check -json valid_config.json`
- **THEN** the command exits zero with confirmation message

#### Scenario: check on config with orphan accounts
- **WHEN** user runs `ledger check -json broken_config.json`
- **THEN** the command exits non-zero and prints the mismatched account names

### Requirement: `reset` sub-command SHALL clear print markers

The `ledger reset` sub-command SHALL open the specified month's xlsx and clear all "需打印" markers in that workbook.

#### Scenario: reset print markers for a month
- **WHEN** user runs `ledger reset -json config.json -month 2026-01 -output ./output`
- **THEN** all "需打印" markers in `output/2026-01.xlsx` are cleared and the file is re-saved

### Requirement: `add-manual` sub-command SHALL add manual adjustment entries

The `ledger add-manual` sub-command SHALL call `balance.AddManualAdjustment` with the provided account, effective month, amount, and note, persisting to `科目余额总览.json`.

#### Scenario: add a new manual adjustment
- **WHEN** user runs `ledger add-manual -account "银行存款-工商银行" -month 2026-03 -amount 100000.00 -note "补记上年余额" -json config.json`
- **THEN** config.json is updated with the new manual item and the command prints confirmation

#### Scenario: duplicate adjustment rejected
- **WHEN** user runs the same add-manual command twice
- **THEN** the second invocation exits non-zero with a duplicate error message

### Requirement: `init` sub-command SHALL create initial configuration

The `ledger init` sub-command SHALL create an initial `科目余额总览.json` with the given start month and empty account structures.

#### Scenario: init creates new config
- **WHEN** user runs `ledger init -start-month 2026-01 -output .`
- **THEN** `./科目余额总览.json` is created with `全局设置.启动月` set to `2026-01`

#### Scenario: init refuses to overwrite
- **WHEN** user runs `ledger init -start-month 2026-01 -output .` and `科目余额总览.json` already exists
- **THEN** the command exits non-zero with an "already exists" error

### Requirement: `year-close` sub-command SHALL perform year-end carry-forward

The `ledger year-close` sub-command SHALL carry forward all account balances from the last month of the current year to the first month of the next year, creating a new xlsx with carry-forward entries.

#### Scenario: year-close generates carry-forward workbook
- **WHEN** user runs `ledger year-close -json config.json -output ./output`
- **THEN** year-end balances are carried forward and a new xlsx for the next year's first month is created

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


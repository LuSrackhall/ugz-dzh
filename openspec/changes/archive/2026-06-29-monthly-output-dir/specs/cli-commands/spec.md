## MODIFIED Requirements

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

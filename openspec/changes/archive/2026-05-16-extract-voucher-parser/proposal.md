## Why

The existing `main.go` contains a complete HTML-table-based voucher parsing engine interwoven with CLI logic and CSV/XLSX output. To build the ledger system incrementally, the parsing core must be extracted into an independent, testable `voucher` package that can be reused across all future components (balance, generator, CLI). This extraction changes no parsing behavior — it is pure modularization, enabling the minimal viable loop without blocking downstream work.

## What Changes

**Voucher Parsing**
- From: Parsing functions (`parseVoucherFile`, `extractVoucherNum`, `extractFromFilename`, `parseAmountToCents`, `cleanDetail`) embedded in `main.go` alongside CLI, sorting, CSV/XLSX output.
- To: Standalone `voucher` package with exported `Entry` struct and `ParseFile` function. All internal helpers are unexported. The original `main.go` will be replaced later as the ledger CLI matures.
- Reason: Enable incremental development — the voucher package can be tested and used independently before the full generator exists.
- Impact: Non-breaking. This is a new package; existing code is preserved as reference and will be phased out in later changes.

## Capabilities

### New Capabilities
- `voucher-parsing`: Parse a Markdown file containing HTML `<table>` voucher entries into structured `Entry` records with amounts in integer cents.

### Modified Capabilities
<!-- None for this change — pure extraction -->

## Impact

- New: `voucher/parser.go`, `voucher/parser_test.go`
- Reference: Existing `main.go` left in place (will be restructured in later changes)
- Dependencies: Go 1.21+ standard library only (no external deps for the voucher package)

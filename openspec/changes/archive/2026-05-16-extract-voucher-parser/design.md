## Context

The project currently has a single `main.go` that interleaves CLI argument handling, voucher parsing from Markdown HTML tables, sorting, and dual-format output (CSV + XLSX). The parsing logic is ~200 lines of functions that are entirely self-contained — they take string input and produce `[]Entry` — making them ideal for extraction into an independent package.

## Goals / Non-Goals

**Goals:**
- Create a `voucher` package exposing a single `ParseFile` function
- Preserve all existing parsing behavior byte-for-byte identical
- Add table-driven tests covering the main code paths
- Keep zero external dependencies (stdlib only)

**Non-Goals:**
- Adding alias mapping (belongs in a future `balance` or config layer)
- Adding debit/credit balance validation
- Supporting plain-text (non-HTML-table) voucher formats
- Modifying the existing `main.go` in any way (it stays as reference)

## Decisions

1. **One public function (`ParseFile`) with all helpers unexported.** The internal parsing pipeline (table extraction → row parsing → amount conversion) has no independent use cases outside this package. Keeping it private prevents accidental coupling from future callers.

2. **`Entry` struct stays at package level, not in a shared `models` package.** Premature extraction into a shared model creates circular dependency risk when `balance` and `generator` need different views. Each package defines its own types and converts at boundaries if needed.

3. **Test fixtures inline, not external files.** The test table uses inline HTML strings — avoids file-system coupling in unit tests and makes test intent self-documenting.

4. **`cleanDetail` stays in the voucher package.** Though it may be useful to other packages later, it is currently only applied at parse time and has no other consumer. YAGNI.

## Risks / Trade-offs

- [Risk] OCR noise in real data may produce unexpected HTML structures → The parser is conservative (skips unparseable rows silently), so no crash risk, but missing entries would need debugging. Mitigation: verbose logging in the CLI wrapper (future change).
- [Trade-off] Amount parsing uses `float64` intermediate → A rounding error is theoretically possible for values with >2 decimal places, but real-world凭证 data is always in `.00` or `.XX` format. Using `math.Round` is the pragmatic choice.

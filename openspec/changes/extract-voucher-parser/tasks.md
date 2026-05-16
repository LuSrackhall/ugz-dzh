## 1. Create voucher package skeleton

- [x] 1.1 Create `voucher/` directory with `parser.go`
- [x] 1.2 Define exported `Entry` struct with json tags matching the existing field names
- [x] 1.3 Copy `ParseFile` function body — reads file, delegates to internal `parseVoucherText`

## 2. Migrate internal parsing helpers

- [x] 2.1 Copy `parseVoucherFile` (renamed to unexported `parseVoucherText`) with all HTML table/regex logic intact
- [x] 2.2 Copy `extractVoucherNum` unchanged
- [x] 2.3 Copy `extractFromFilename` unchanged
- [x] 2.4 Copy `parseAmountToCents` unchanged
- [x] 2.5 Copy `cleanDetail` unchanged
- [x] 2.6 Copy auxiliary helpers (`containsAny`, `containsAnyInSlice`, `padLeft`, `intToStr`)

## 3. Write table-driven tests

- [x] 3.1 Create `parser_test.go` with a test table covering: multi-voucher file, empty file, amount-with-Chinese-comma, and a table with header rows that should be skipped
- [x] 3.2 Add a test case that verifies voucher number extraction from filename fallback
- [x] 3.3 Add a test case that verifies `cleanDetail` normalization (full-width space, extra whitespace)

## 4. Verify and commit

- [x] 4.1 Run `go test ./voucher/...` and confirm all tests pass
- [x] 4.2 Run `go vet ./voucher/...` with no issues
- [x] 4.3 Commit the voucher package

# apply-money-number-format Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Apply `#,##0.00` Excel number format to all amount cells written via `centsToYuan` across all generator sheets.

**Architecture:** Create a single `moneyStyleID` in `NewWorkbook()` with `CustomNumFmt: "#,##0.00"`. Expose `setMoneyStyle(sheet, row, col)` helper. At every `SetCellValue` call site that writes a `centsToYuan` result, follow immediately with `setMoneyStyle` on the same cell. For sheets that apply range styles (monthly_close, monthly_close_ml, initial_sheet, final_sheet), call per-cell `setMoneyStyle` AFTER the range style to override only amount cells while preserving bold/border on non-amount cells.

**Tech Stack:** Go, excelize v2

---

## File Structure

- **Modify:** `generator/workbook.go` — add `moneyStyleID` field, create style in `NewWorkbook()`, add `setMoneyStyle` helper
- **Modify:** `generator/gl_sheet.go` — 4 write sites (data rows, insertCarryForward, writePageBreakRow, writeCarryForwardRow)
- **Modify:** `generator/ml_sheet.go` — 3 write sites (data rows, writeMLPageBreakRow, writeMLCarryForwardRow)
- **Modify:** `generator/monthly_close.go` — 4 row types (本月合计, 本季合计, 本年累计, 期末余额)
- **Modify:** `generator/monthly_close_ml.go` — 4 row types (本月合计, 本季合计, 本年累计, 期末余额)
- **Modify:** `generator/initial_sheet.go` — data rows + total row
- **Modify:** `generator/final_sheet.go` — data rows + total row

---

### Task 1: Workbook 样式基础设施

**Files:**
- Modify: `generator/workbook.go`

- [ ] **Step 1: Add `moneyStyleID` field to Workbook struct**

In `generator/workbook.go`, replace the Workbook struct:

```go
// Workbook 持有 excelize.File 和当月生成上下文。
type Workbook struct {
	File         *excelize.File
	Config       *balance.GlobalConfig
	Month        string // YYYY-MM
	OutputDir    string
	ConfigPath   string
	moneyStyleID int
}
```

- [ ] **Step 2: Create money style in NewWorkbook()**

After `wb.File = excelize.NewFile()` (line 47) and the comment line 48, add style creation. The style must be created for BOTH the "open previous file" path and the "new file" path, so add it after the if/else block, before `return wb, nil`:

```go
	// 创建金额数字格式样式
	moneyStyle, err := wb.File.NewStyle(&excelize.Style{
		CustomNumFmt: stringPtr("#,##0.00"),
	})
	if err != nil {
		return nil, fmt.Errorf("创建金额样式: %w", err)
	}
	wb.moneyStyleID = moneyStyle

	return wb, nil
}
```

Need to add `stringPtr` helper if not present:

```go
func stringPtr(s string) *string {
	return &s
}
```

- [ ] **Step 3: Add setMoneyStyle helper method**

Add after the existing helper methods (before `cellName`):

```go
// setMoneyStyle 对指定单元格应用金额数字格式 #,##0.00。
func (wb *Workbook) setMoneyStyle(sheet string, row, col int) {
	cell, _ := excelize.CoordinatesToCellName(col, row)
	wb.File.SetCellStyle(sheet, cell, cell, wb.moneyStyleID)
}
```

- [ ] **Step 4: Run tests to verify compilation**

Run: `go build ./...`
Expected: compiles without errors

- [ ] **Step 5: Commit**

```bash
git add generator/workbook.go
git commit -m "feat: add moneyStyleID and setMoneyStyle helper for #,##0.00 number format"
```

---

### Task 2: gl_sheet.go 金额列应用格式

**Files:**
- Modify: `generator/gl_sheet.go`

- [ ] **Step 1: Data rows — appendToGLSheet (cols 4, 5, 7)**

After the `SetCellValue` block at lines 208-213, add `setMoneyStyle` calls for each amount column:

```go
		wb.File.SetCellValue(sheet, cellName(4, row), centsToYuan(e.DebitCents))
		wb.File.SetCellValue(sheet, cellName(5, row), centsToYuan(e.CreditCents))

		dir, dispBal := directionFor(balance, 0)
		wb.File.SetCellValue(sheet, cellName(6, row), dir)
		wb.File.SetCellValue(sheet, cellName(7, row), centsToYuan(dispBal))

		wb.setMoneyStyle(sheet, row, 4)
		wb.setMoneyStyle(sheet, row, 5)
		wb.setMoneyStyle(sheet, row, 7)

		wb.markRowForPrint(sheet, row)
```

- [ ] **Step 2: insertCarryForward (col 7)**

After line 231, add `setMoneyStyle` for col 7:

```go
	wb.File.SetCellValue(sheet, cellName(6, row), dir)
	wb.File.SetCellValue(sheet, cellName(7, row), centsToYuan(dispBal))

	wb.setMoneyStyle(sheet, row, 7)

	return nil
```

- [ ] **Step 3: writePageBreakRow (cols 4, 5, 7)**

After lines 328-331, add `setMoneyStyle` calls:

```go
	wb.File.SetCellValue(sheet, cellName(4, row), centsToYuan(pageDebit))
	wb.File.SetCellValue(sheet, cellName(5, row), centsToYuan(pageCredit))
	wb.File.SetCellValue(sheet, cellName(6, row), dir)
	wb.File.SetCellValue(sheet, cellName(7, row), centsToYuan(dispBal))

	wb.setMoneyStyle(sheet, row, 4)
	wb.setMoneyStyle(sheet, row, 5)
	wb.setMoneyStyle(sheet, row, 7)
}
```

- [ ] **Step 4: writeCarryForwardRow (cols 4, 5, 7)**

After lines 340-343, add `setMoneyStyle` calls:

```go
	wb.File.SetCellValue(sheet, cellName(4, row), centsToYuan(pageDebit))
	wb.File.SetCellValue(sheet, cellName(5, row), centsToYuan(pageCredit))
	wb.File.SetCellValue(sheet, cellName(6, row), dir)
	wb.File.SetCellValue(sheet, cellName(7, row), centsToYuan(dispBal))

	wb.setMoneyStyle(sheet, row, 4)
	wb.setMoneyStyle(sheet, row, 5)
	wb.setMoneyStyle(sheet, row, 7)
}
```

- [ ] **Step 5: Run tests**

Run: `go test ./generator/...`
Expected: all tests pass

- [ ] **Step 6: Commit**

```bash
git add generator/gl_sheet.go
git commit -m "feat: apply money number format to gl_sheet amount cells"
```

---

### Task 3: ml_sheet.go 金额列应用格式

**Files:**
- Modify: `generator/ml_sheet.go`

- [ ] **Step 1: Data rows — appendToMLSheet (cols 4, 5, 7 + detail cols)**

After the `SetCellValue` block at lines 495-498, add `setMoneyStyle` for cols 4,5,7:

```go
		wb.File.SetCellValue(sheet, cellName(4, row), centsToYuan(e.DebitCents))
		wb.File.SetCellValue(sheet, cellName(5, row), centsToYuan(e.CreditCents))
		wb.File.SetCellValue(sheet, cellName(6, row), dir)
		wb.File.SetCellValue(sheet, cellName(7, row), centsToYuan(dispBal))

		wb.setMoneyStyle(sheet, row, 4)
		wb.setMoneyStyle(sheet, row, 5)
		wb.setMoneyStyle(sheet, row, 7)
```

After the detail column `SetCellValue` at line 503, add `setMoneyStyle`:

```go
			if e.DetailAccount != "" {
				if idx, ok := detailIdx[e.DetailAccount]; ok {
					net := e.DebitCents - e.CreditCents
					col := mlDetailStartCol + idx
					wb.File.SetCellValue(sheet, cellName(col, row), centsToYuan(net))
					wb.setMoneyStyle(sheet, row, col)
					pageDetails[idx].debit += e.DebitCents
					pageDetails[idx].credit += e.CreditCents
				}
			}
```

- [ ] **Step 2: writeMLPageBreakRow (cols 4, 5, 7 + detail cols)**

After line 525, add `setMoneyStyle`:

```go
	wb.File.SetCellValue(sheet, cellName(4, row), centsToYuan(pageDebit))
	wb.File.SetCellValue(sheet, cellName(5, row), centsToYuan(pageCredit))
	wb.File.SetCellValue(sheet, cellName(6, row), dir)
	wb.File.SetCellValue(sheet, cellName(7, row), centsToYuan(dispBal))

	wb.setMoneyStyle(sheet, row, 4)
	wb.setMoneyStyle(sheet, row, 5)
	wb.setMoneyStyle(sheet, row, 7)
```

After line 529 (detail cols in the loop), add `setMoneyStyle`:

```go
	for i, pd := range pageDetails {
		net := pd.debit - pd.credit
		col := mlDetailStartCol + i
		wb.File.SetCellValue(sheet, cellName(col, row), centsToYuan(net))
		wb.setMoneyStyle(sheet, row, col)
	}
```

- [ ] **Step 3: writeMLCarryForwardRow (cols 4, 5, 7 + detail cols)**

After line 542, add `setMoneyStyle`:

```go
	wb.File.SetCellValue(sheet, cellName(4, row), centsToYuan(pageDebit))
	wb.File.SetCellValue(sheet, cellName(5, row), centsToYuan(pageCredit))
	wb.File.SetCellValue(sheet, cellName(6, row), dir)
	wb.File.SetCellValue(sheet, cellName(7, row), centsToYuan(dispBal))

	wb.setMoneyStyle(sheet, row, 4)
	wb.setMoneyStyle(sheet, row, 5)
	wb.setMoneyStyle(sheet, row, 7)
```

After line 546 (detail cols in the loop), add `setMoneyStyle`:

```go
	for i, pd := range pageDetails {
		net := pd.debit - pd.credit
		col := mlDetailStartCol + i
		wb.File.SetCellValue(sheet, cellName(col, row), centsToYuan(net))
		wb.setMoneyStyle(sheet, row, col)
	}
```

- [ ] **Step 4: Run tests**

Run: `go test ./generator/...`
Expected: all tests pass

- [ ] **Step 5: Commit**

```bash
git add generator/ml_sheet.go
git commit -m "feat: apply money number format to ml_sheet amount cells"
```

---

### Task 4: monthly_close.go 金额列应用格式

**Files:**
- Modify: `generator/monthly_close.go`

**Critical:** Range styles (monthlyStyle, qtStyle, cumStyle, endStyle) are applied to cols 1-7 BEFORE per-cell setMoneyStyle. Per-cell style overrides range style only on amount cells.

- [ ] **Step 1: 本月合计 row (cols 4, 5, 7)**

After the range style at line 37, add per-cell `setMoneyStyle`:

```go
	wb.File.SetCellStyle(sheet, cellName(1, row), cellName(7, row), monthlyStyle)

	wb.setMoneyStyle(sheet, row, 4)
	wb.setMoneyStyle(sheet, row, 5)
	wb.setMoneyStyle(sheet, row, 7)

	row++
```

- [ ] **Step 2: 本季合计 row (cols 4, 5, 7)**

After the range style at line 56, add per-cell `setMoneyStyle`:

```go
	wb.File.SetCellStyle(sheet, cellName(1, row), cellName(7, row), qtStyle)

		wb.setMoneyStyle(sheet, row, 4)
		wb.setMoneyStyle(sheet, row, 5)
		wb.setMoneyStyle(sheet, row, 7)

		row++
	}
```

- [ ] **Step 3: 本年累计 row (cols 4, 5, 7)**

After the range style at line 78, add per-cell `setMoneyStyle`:

```go
	wb.File.SetCellStyle(sheet, cellName(1, row), cellName(7, row), cumStyle)

	wb.setMoneyStyle(sheet, row, 4)
	wb.setMoneyStyle(sheet, row, 5)
	wb.setMoneyStyle(sheet, row, 7)

	row++
```

- [ ] **Step 4: 期末余额 row (col 7 only — cols 4,5 are empty strings)**

After the range style at line 99, add per-cell `setMoneyStyle` for col 7 only:

```go
	wb.File.SetCellStyle(sheet, cellName(1, row), cellName(7, row), endStyle)

		wb.setMoneyStyle(sheet, row, 7)
	}
```

- [ ] **Step 5: Run tests**

Run: `go test ./generator/...`
Expected: all tests pass

- [ ] **Step 6: Commit**

```bash
git add generator/monthly_close.go
git commit -m "feat: apply money number format to monthly_close amount cells"
```

---

### Task 5: monthly_close_ml.go 金额列应用格式

**Files:**
- Modify: `generator/monthly_close_ml.go`

**Critical:** Same pattern as monthly_close.go — per-cell setMoneyStyle AFTER range style.

- [ ] **Step 1: 本月合计 row (cols 4, 5, 7 + detail cols)**

After the range style at line 88, add per-cell `setMoneyStyle`:

```go
	wb.File.SetCellStyle(sheet, cellName(1, row), cellName(lastDetailCol, row), monthlyStyle)

	wb.setMoneyStyle(sheet, row, 4)
	wb.setMoneyStyle(sheet, row, 5)
	wb.setMoneyStyle(sheet, row, 7)
	for i := 0; i < mlMaxDetails; i++ {
		if details[i] != "" {
			wb.setMoneyStyle(sheet, row, mlDetailStartCol+i)
		}
	}

	row++
```

Note: the setMoneyStyle calls must be added AFTER the existing `for i := 0; i < mlMaxDetails` loop that sets cell values (lines 74-79), and AFTER the range style at line 88. So insert between line 88 and `row++`.

- [ ] **Step 2: 本季合计 row (cols 4, 5, 7 + detail cols)**

After the range style at line 129, add per-cell `setMoneyStyle`:

```go
	wb.File.SetCellStyle(sheet, cellName(1, row), cellName(lastDetailCol, row), qtStyle)

		wb.setMoneyStyle(sheet, row, 4)
		wb.setMoneyStyle(sheet, row, 5)
		wb.setMoneyStyle(sheet, row, 7)
		for i := 0; i < mlMaxDetails; i++ {
			if details[i] != "" {
				wb.setMoneyStyle(sheet, row, mlDetailStartCol+i)
			}
		}

		row++
	}
```

- [ ] **Step 3: 本年累计 row (cols 4, 5, 7 + detail cols)**

After the range style at line 173, add per-cell `setMoneyStyle`:

```go
	wb.File.SetCellStyle(sheet, cellName(1, row), cellName(lastDetailCol, row), cumStyle)

	wb.setMoneyStyle(sheet, row, 4)
	wb.setMoneyStyle(sheet, row, 5)
	wb.setMoneyStyle(sheet, row, 7)
	for i := 0; i < mlMaxDetails; i++ {
		if details[i] != "" {
			wb.setMoneyStyle(sheet, row, mlDetailStartCol+i)
		}
	}

	row++
```

- [ ] **Step 4: 期末余额 row (col 7 only)**

After the range style at line 194, add per-cell `setMoneyStyle`:

```go
	wb.File.SetCellStyle(sheet, cellName(1, row), cellName(lastDetailCol, row), endStyle)

		wb.setMoneyStyle(sheet, row, 7)
	}
```

- [ ] **Step 5: Run tests**

Run: `go test ./generator/...`
Expected: all tests pass

- [ ] **Step 6: Commit**

```bash
git add generator/monthly_close_ml.go
git commit -m "feat: apply money number format to monthly_close_ml amount cells"
```

---

### Task 6: initial_sheet.go 金额列应用格式

**Files:**
- Modify: `generator/initial_sheet.go`

- [ ] **Step 1: Data rows (col 3)**

After line 80, add `setMoneyStyle` for col 3:

```go
		wb.File.SetCellValue(name, cellName(3, row), centsToYuan(dispBal))
		wb.setMoneyStyle(name, row, 3)
		row++
```

- [ ] **Step 2: Total row (col 3) — AFTER range style**

After the range style at line 102, add `setMoneyStyle`:

```go
	wb.File.SetCellStyle(name, totalCell, cellName(3, row), totalStyle)

	wb.setMoneyStyle(name, row, 3)

	return nil
```

- [ ] **Step 3: Run tests**

Run: `go test ./generator/...`
Expected: all tests pass

- [ ] **Step 4: Commit**

```bash
git add generator/initial_sheet.go
git commit -m "feat: apply money number format to initial_sheet amount cells"
```

---

### Task 7: final_sheet.go 金额列应用格式

**Files:**
- Modify: `generator/final_sheet.go`

- [ ] **Step 1: Data rows (col 3)**

After line 79, add `setMoneyStyle` for col 3:

```go
		wb.File.SetCellValue(name, cellName(3, row), centsToYuan(dispBal))
		wb.setMoneyStyle(name, row, 3)
		row++
```

- [ ] **Step 2: Total row (col 3) — AFTER range style**

After the range style at line 95, add `setMoneyStyle`:

```go
	wb.File.SetCellStyle(name, totalCell, cellName(3, row), totalStyle)

	wb.setMoneyStyle(name, row, 3)

	return nil
```

- [ ] **Step 3: Run tests**

Run: `go test ./generator/...`
Expected: all tests pass

- [ ] **Step 4: Commit**

```bash
git add generator/final_sheet.go
git commit -m "feat: apply money number format to final_sheet amount cells"
```

---

### Task 8: 测试与验证

**Files:**
- (no code changes — verification only)

- [ ] **Step 1: Run full generator test suite**

Run: `go test ./generator/... -v`
Expected: all tests pass, 0 failures

- [ ] **Step 2: Run full project test suite**

Run: `go test ./...`
Expected: all tests pass across all packages

- [ ] **Step 3: Generate output xlsx and verify numFmt**

Run the generator to produce an output xlsx file, then verify the `numFmt` attribute:

```bash
go run . -config <config-path> -month <YYYY-MM> -output <output-dir>
```

Then verify with a quick script:

```bash
# Unzip the xlsx and check that amount cells have numFmt="164" or similar
# (excelize assigns a built-in or custom format ID)
```

Manual verification: open the generated xlsx, check that amount cells display with thousand separators and 2 decimal places (e.g., `4,000.00`).

- [ ] **Step 4: Final commit (if any verification fixes needed)**

```bash
git add -A
git commit -m "test: verify money number format in generated xlsx output"
```

If no changes needed, skip this commit.

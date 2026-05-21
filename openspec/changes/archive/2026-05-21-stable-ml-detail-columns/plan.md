# 累积稳定多科目明细列 实现计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 实现多科目明细账累积稳定列机制——读头保序、新科目右追、detailOrder JSON 配置、冲突检测、统一映射来源。

**Architecture:** 新增 `readMLDetailHeaders` 从第2行读取现有列映射；新增 `resolveMLDetailColumns` 合并新科目到空列；修改 `ensureMLSheet` 返回 `detailIdx`；`AppendMLEntries` 和 `WriteMLMonthClosings` 统一从此获取映射。`GlobalConfig` 新增 `DetailOrder` 字段，在 `ensureMLSheet` 中检测冲突，在 `resolveMLDetailColumns` 中追加新科目并回写。

**Tech Stack:** Go 1.x, excelize v2, encoding/json

---

## Task 1: 读头保序 — `readMLDetailHeaders`

**Files:**
- Modify: `generator/ml_sheet.go`

- [ ] **Step 1: 新增 `readMLDetailHeaders` 函数**

在 `generator/ml_sheet.go` 中 `updateMLDetailHeaders` 之后添加：

```go
// readMLDetailHeaders 从 Sheet 第2行 H-U 读取现有明细列标题，构建 detailName → colIndex 映射。
// 返回的 details 按列顺序排列（空列对应空字符串）。
func (wb *Workbook) readMLDetailHeaders(sheet string) (detailIdx map[string]int, details []string, err error) {
	detailIdx = make(map[string]int)
	details = make([]string, mlMaxDetails)

	rows, err := wb.File.GetRows(sheet)
	if err != nil {
		return nil, nil, fmt.Errorf("读取 Sheet %s: %w", sheet, err)
	}
	if len(rows) < 2 {
		// 无标题行，视为全新
		return detailIdx, details, nil
	}

	row2 := rows[1]
	for i := 0; i < mlMaxDetails; i++ {
		colIdx := mlDetailStartCol + i - 1 // rows 是 0-indexed
		label := ""
		if colIdx < len(row2) {
			label = strings.TrimSpace(row2[colIdx])
		}
		details[i] = label
		if label != "" {
			detailIdx[label] = i
		}
	}
	return detailIdx, details, nil
}
```

- [ ] **Step 2: 验证编译**

```bash
go build ./generator/...
```

- [ ] **Step 3: Commit**

```bash
git add generator/ml_sheet.go
git commit -m "feat: add readMLDetailHeaders to read existing H-U headers from sheet"
```

## Task 2: 列合并逻辑 — `resolveMLDetailColumns`

**Files:**
- Modify: `generator/ml_sheet.go`

- [ ] **Step 1: 新增 `resolveMLDetailColumns` 函数**

在 `readMLDetailHeaders` 之后添加：

```go
// resolveMLDetailColumns 合并已有列映射与新科目，返回完整列序、映射、新增科目列表。
// existingDetails: 从 Sheet 第2行读取的现有列序（空字符串表示空列）
// newDetails: 当月分录中的明细科目集合
// detailOrder: 用户配置的列序（nil 表示无配置）
func resolveMLDetailColumns(existingDetails []string, newDetails []string, detailOrder []string) (details []string, detailIdx map[string]int, newAppended []string, err error) {
	details = make([]string, mlMaxDetails)
	copy(details, existingDetails)

	detailIdx = make(map[string]int)
	for i, d := range details {
		if d != "" {
			detailIdx[d] = i
		}
	}

	// 找出不在现有列中的新科目
	var toAdd []string
	for _, nd := range newDetails {
		if _, ok := detailIdx[nd]; !ok {
			toAdd = append(toAdd, nd)
		}
	}
	if len(toAdd) == 0 {
		return details, detailIdx, nil, nil
	}

	// 如果有 detailOrder，按配置顺序排列；否则按字母序
	if len(detailOrder) > 0 {
		// 按 detailOrder 中出现的顺序排列 toAdd
		orderMap := make(map[string]int)
		for i, d := range detailOrder {
			orderMap[d] = i
		}
		sort.Slice(toAdd, func(i, j int) bool {
			oi, iok := orderMap[toAdd[i]]
			oj, jok := orderMap[toAdd[j]]
			if iok && jok {
				return oi < oj
			}
			if iok {
				return true
			}
			if jok {
				return false
			}
			return toAdd[i] < toAdd[j]
		})
	} else {
		sort.Strings(toAdd)
	}

	// 追加到右侧第一个空列
	for _, nd := range toAdd {
		placed := false
		for i := 0; i < mlMaxDetails; i++ {
			if details[i] == "" {
				details[i] = nd
				detailIdx[nd] = i
				newAppended = append(newAppended, nd)
				placed = true
				break
			}
		}
		if !placed {
			return nil, nil, nil, fmt.Errorf("多科目明细列已满（14列全部占用），无法追加 %q。已占用: %v", nd, nonEmptyDetails(details))
		}
	}

	return details, detailIdx, newAppended, nil
}

// nonEmptyDetails 返回非空明细科目列表。
func nonEmptyDetails(details []string) []string {
	var result []string
	for _, d := range details {
		if d != "" {
			result = append(result, d)
		}
	}
	return result
}
```

需要添加 `"sort"` 和 `"strings"` 到 import（若尚未导入）。

- [ ] **Step 2: 验证编译**

```bash
go build ./generator/...
```

- [ ] **Step 3: Commit**

```bash
git add generator/ml_sheet.go
git commit -m "feat: add resolveMLDetailColumns for stable column merging"
```

## Task 3: 重构 `ensureMLSheet` — 读头保序 + 返回 detailIdx

**Files:**
- Modify: `generator/ml_sheet.go`

- [ ] **Step 1: 修改 `ensureMLSheet` 签名和逻辑**

将 `ensureMLSheet` 的签名从：
```go
func (wb *Workbook) ensureMLSheet(general string, details []string) (string, error)
```
改为：
```go
func (wb *Workbook) ensureMLSheet(general string, details []string, detailOrder []string) (string, map[string]int, error)
```

新逻辑：

```go
func (wb *Workbook) ensureMLSheet(general string, details []string, detailOrder []string) (string, map[string]int, error) {
	name := sheetNameML(general)
	if idx, err := wb.File.GetSheetIndex(name); err == nil && idx >= 0 {
		// Sheet 已存在 — 读头保序
		existingIdx, existingDetails, err := wb.readMLDetailHeaders(name)
		if err != nil {
			return "", nil, err
		}

		// 冲突检测：若配置了 detailOrder，逐列比对
		if len(detailOrder) > 0 {
			if err := wb.checkMLDetailOrderConflict(name, existingDetails, detailOrder); err != nil {
				return "", nil, err
			}
		}

		// 合并新科目到空列
		finalDetails, finalIdx, newAppended, err := resolveMLDetailColumns(existingDetails, details, detailOrder)
		if err != nil {
			return "", nil, err
		}

		// 更新标题行（仅更新新增的列）
		for _, nd := range newAppended {
			col := mlDetailStartCol + finalIdx[nd]
			cell, _ := excelize.CoordinatesToCellName(col, 2)
			wb.File.SetCellValue(name, cell, nd)
		}

		return name, finalIdx, nil
	}

	// 新 Sheet — 创建
	idx, err := wb.File.NewSheet(name)
	if err != nil {
		return "", nil, fmt.Errorf("创建 Sheet %s: %w", name, err)
	}
	wb.File.SetActiveSheet(idx)

	// 初始化列序：若存在 detailOrder，使用配置；否则按字母序
	var initDetails []string
	if len(detailOrder) > 0 {
		// 使用配置顺序，但只包含实际存在的科目 + 空字符串占位
		initDetails = make([]string, 0, mlMaxDetails)
		existingSet := make(map[string]bool)
		for _, d := range details {
			existingSet[d] = true
		}
		for _, d := range detailOrder {
			if d == "" || existingSet[d] {
				initDetails = append(initDetails, d)
				existingSet[d] = false // 标记已用
			}
		}
		// 配置未列出的新科目按字母序追加
		var remaining []string
		for _, d := range details {
			if existingSet[d] {
				remaining = append(remaining, d)
			}
		}
		sort.Strings(remaining)
		initDetails = append(initDetails, remaining...)
	} else {
		initDetails = make([]string, len(details))
		copy(initDetails, details)
		sort.Strings(initDetails)
	}

	if len(initDetails) > mlMaxDetails {
		return "", nil, fmt.Errorf("总账科目 %q 明细科目数 %d 超过上限 %d", general, len(initDetails), mlMaxDetails)
	}

	if err := wb.writeMLTitle(name, general, initDetails); err != nil {
		return "", nil, err
	}

	// 构建 detailIdx
	detailIdx := make(map[string]int)
	for i, d := range initDetails {
		if d != "" {
			detailIdx[d] = i
		}
	}

	return name, detailIdx, nil
}
```

- [ ] **Step 2: 新增冲突检测辅助函数**

```go
// checkMLDetailOrderConflict 逐列比对第2行标题与 detailOrder 配置。
func (wb *Workbook) checkMLDetailOrderConflict(sheet string, existingDetails []string, detailOrder []string) error {
	// 将现有非空标题与配置逐列比对
	configIdx := 0
	for colIdx := 0; colIdx < mlMaxDetails && configIdx < len(detailOrder); colIdx++ {
		existing := existingDetails[colIdx]
		configured := detailOrder[configIdx]

		if configured == "" {
			// 配置要求此列为空
			if existing != "" {
				return fmt.Errorf("Sheet %s: detailOrder 与现有列序冲突 — 第 %d 列配置为空但实际为 %q。请使用 -f 从首月重新生成", sheet, colIdx+1, existing)
			}
			configIdx++
			continue
		}

		if existing == "" {
			// 现有为空，但配置有值 — 检查配置值是否已在更右侧
			found := false
			for j := colIdx + 1; j < mlMaxDetails; j++ {
				if existingDetails[j] == configured {
					return fmt.Errorf("Sheet %s: detailOrder 与现有列序冲突 — %q 配置在第 %d 列但实际在第 %d 列。请使用 -f 从首月重新生成", sheet, configured, configIdx+1, j+1)
				}
			}
			// 配置值不在任何位置 — 这是待追加的新科目，不冲突
			if !found {
				configIdx++
				continue
			}
		}

		if existing != configured {
			return fmt.Errorf("Sheet %s: detailOrder 与现有列序冲突 — 第 %d 列配置为 %q 但实际为 %q。请使用 -f 从首月重新生成", sheet, configIdx+1, configured, existing)
		}
		configIdx++
	}
	return nil
}
```

- [ ] **Step 3: 验证编译**

```bash
go build ./generator/...
```

- [ ] **Step 4: Commit**

```bash
git add generator/ml_sheet.go
git commit -m "feat: refactor ensureMLSheet to read-preserve column order and return detailIdx"
```

## Task 4: 修改 `AppendMLEntries` — 统一映射来源

**Files:**
- Modify: `generator/ml_sheet.go`

- [ ] **Step 1: 重构 `AppendMLEntries`**

移除内部排序逻辑（lines 145-150），改为从 `ensureMLSheet` 获取 `detailIdx`：

```go
func (wb *Workbook) AppendMLEntries(entries []voucher.Entry, initials map[string]int64) error {
	type mlGroup struct {
		entries []voucher.Entry
		details []string
	}
	groups := make(map[string]*mlGroup)

	for _, e := range entries {
		g, ok := groups[e.GeneralAccount]
		if !ok {
			g = &mlGroup{}
			groups[e.GeneralAccount] = g
		}
		g.entries = append(g.entries, e)
		if e.DetailAccount != "" {
			found := false
			for _, d := range g.details {
				if d == e.DetailAccount {
					found = true
					break
				}
			}
			if !found {
				g.details = append(g.details, e.DetailAccount)
			}
		}
	}

	for general, g := range groups {
		if len(g.details) == 0 {
			continue
		}
		detailOrder := wb.Config.DetailOrder[general]
		_, detailIdx, err := wb.ensureMLSheet(general, g.details, detailOrder)
		if err != nil {
			return err
		}
		if err := wb.appendToMLSheet(general, g.entries, detailIdx, initials[general]); err != nil {
			return fmt.Errorf("多科目明细账 %s: %w", general, err)
		}
	}

	return nil
}
```

- [ ] **Step 2: 修改 `appendToMLSheet` 签名**

移除 `details` 和 `detailIdx` 参数中的 `details`（不再需要，因为映射已由 `detailIdx` 涵盖）：

```go
func (wb *Workbook) appendToMLSheet(general string, entries []voucher.Entry, detailIdx map[string]int, initial int64) error {
```

并在函数体中移除 `numDetails := mlMaxDetails` 之后对 `mlMaxDetails` 的冗余引用（保持现有逻辑，仅移除未使用的 `details` 参数）。

- [ ] **Step 3: 验证编译**

```bash
go build ./generator/...
```

- [ ] **Step 4: Commit**

```bash
git add generator/ml_sheet.go
git commit -m "feat: refactor AppendMLEntries to use ensureMLSheet's detailIdx"
```

## Task 5: 修改 `WriteMLMonthClosings` — 统一映射来源

**Files:**
- Modify: `generator/monthly_close_ml.go`

- [ ] **Step 1: 移除独立排序，改用读头映射**

修改 `WriteMLMonthClosings`：移除 lines 44-48 的排序逻辑，改为调用 `readMLDetailHeaders`：

```go
func (wb *Workbook) WriteMLMonthClosings(
	entries []voucher.Entry,
	initials map[string]int64,
	ytdDebit, ytdCredit map[string]int64,
	qtdDebit, qtdCredit map[string]int64,
	changedSheets map[string]bool,
) error {
	type mlClosing struct {
		entries []voucher.Entry
	}
	groups := make(map[string]*mlClosing)

	for _, e := range entries {
		if e.DetailAccount == "" {
			continue
		}
		g, ok := groups[e.GeneralAccount]
		if !ok {
			g = &mlClosing{}
			groups[e.GeneralAccount] = g
		}
		g.entries = append(g.entries, e)
	}

	for general, g := range groups {
		sheet := sheetNameML(general)
		if !changedSheets[sheet] {
			continue
		}

		// 从 Sheet 标题读取列映射
		detailIdx, details, err := wb.readMLDetailHeaders(sheet)
		if err != nil {
			return err
		}

		numDetails := mlMaxDetails

		// 计算本月各明细发生额
		mtdDetails := make([]mlDetailTotals, numDetails)
		var mtdDebit, mtdCredit int64
		for _, e := range g.entries {
			mtdDebit += e.DebitCents
			mtdCredit += e.CreditCents
			if idx, ok := detailIdx[e.DetailAccount]; ok {
				mtdDetails[idx].debit += e.DebitCents
				mtdDetails[idx].credit += e.CreditCents
			}
		}

		row, err := wb.nextDataRowAfterBreak(sheet)
		if err != nil {
			return err
		}

		// "本月合计" 行
		wb.File.SetCellValue(sheet, cellName(1, row), "")
		wb.File.SetCellValue(sheet, cellName(2, row), "")
		wb.File.SetCellValue(sheet, cellName(3, row), "本月合计")
		wb.File.SetCellValue(sheet, cellName(4, row), centsToYuanStr(mtdDebit))
		wb.File.SetCellValue(sheet, cellName(5, row), centsToYuanStr(mtdCredit))
		wb.File.SetCellValue(sheet, cellName(6, row), "")
		wb.File.SetCellValue(sheet, cellName(7, row), "")
		for i := 0; i < mlMaxDetails; i++ {
			if details[i] != "" {
				net := mtdDetails[i].debit - mtdDetails[i].credit
				wb.File.SetCellValue(sheet, cellName(mlDetailStartCol+i, row), centsToYuanStr(net))
			}
		}

		monthlyStyle, _ := wb.File.NewStyle(&excelize.Style{
			Font: &excelize.Font{Bold: true, Size: 10},
			Border: []excelize.Border{
				{Type: "top", Color: "#808080", Style: 1},
			},
		})
		lastDetailCol := mlDetailStartCol + numDetails - 1
		wb.File.SetCellStyle(sheet, cellName(1, row), cellName(lastDetailCol, row), monthlyStyle)
		row++

		// "本季合计" 行 — 仅季末月份
		if isQuarterEnd(wb.Month) {
			qtDetails := make([]mlDetailTotals, numDetails)
			var qtDebit, qtCredit int64
			for _, e := range g.entries {
				qtDebit += e.DebitCents
				qtCredit += e.CreditCents
				if idx, ok := detailIdx[e.DetailAccount]; ok {
					qtDetails[idx].debit += e.DebitCents
					qtDetails[idx].credit += e.CreditCents
				}
			}
			qtDebit += qtdDebit[general]
			qtCredit += qtdCredit[general]

			wb.File.SetCellValue(sheet, cellName(1, row), "")
			wb.File.SetCellValue(sheet, cellName(2, row), "")
			wb.File.SetCellValue(sheet, cellName(3, row), "本季合计")
			wb.File.SetCellValue(sheet, cellName(4, row), centsToYuanStr(qtDebit))
			wb.File.SetCellValue(sheet, cellName(5, row), centsToYuanStr(qtCredit))
			wb.File.SetCellValue(sheet, cellName(6, row), "")
			wb.File.SetCellValue(sheet, cellName(7, row), "")
			for i := 0; i < mlMaxDetails; i++ {
				if details[i] != "" {
					prevQt := wb.getDetailPrevQuarterTotal(general, details[i])
					net := qtDetails[i].debit - qtDetails[i].credit + prevQt
					wb.File.SetCellValue(sheet, cellName(mlDetailStartCol+i, row), centsToYuanStr(net))
				}
			}

			qtStyle, _ := wb.File.NewStyle(&excelize.Style{
				Font: &excelize.Font{Bold: true, Size: 10},
			})
			wb.File.SetCellStyle(sheet, cellName(1, row), cellName(lastDetailCol, row), qtStyle)
			row++
		}

		// "本年累计" 行
		ytdDetails := make([]mlDetailTotals, numDetails)
		var cumDebit, cumCredit int64
		for _, e := range g.entries {
			cumDebit += e.DebitCents
			cumCredit += e.CreditCents
			if idx, ok := detailIdx[e.DetailAccount]; ok {
				ytdDetails[idx].debit += e.DebitCents
				ytdDetails[idx].credit += e.CreditCents
			}
		}
		cumDebit += ytdDebit[general]
		cumCredit += ytdCredit[general]

		wb.File.SetCellValue(sheet, cellName(1, row), "")
		wb.File.SetCellValue(sheet, cellName(2, row), "")
		wb.File.SetCellValue(sheet, cellName(3, row), "本年累计")
		wb.File.SetCellValue(sheet, cellName(4, row), centsToYuanStr(cumDebit))
		wb.File.SetCellValue(sheet, cellName(5, row), centsToYuanStr(cumCredit))
		wb.File.SetCellValue(sheet, cellName(6, row), "")
		wb.File.SetCellValue(sheet, cellName(7, row), "")
		for i := 0; i < mlMaxDetails; i++ {
			if details[i] != "" {
				prevYtd := wb.getDetailPrevYearTotal(general, details[i])
				net := ytdDetails[i].debit - ytdDetails[i].credit + prevYtd
				wb.File.SetCellValue(sheet, cellName(mlDetailStartCol+i, row), centsToYuanStr(net))
			}
		}

		cumStyle, _ := wb.File.NewStyle(&excelize.Style{
			Font: &excelize.Font{Bold: true, Size: 10},
			Border: []excelize.Border{
				{Type: "bottom", Color: "#808080", Style: 1},
			},
		})
		wb.File.SetCellStyle(sheet, cellName(1, row), cellName(lastDetailCol, row), cumStyle)
		row++

		// "期末余额" 行
		endBalance := initials[general] + mtdDebit - mtdCredit
		endDir, endDisp := directionFor(endBalance, 0)

		wb.File.SetCellValue(sheet, cellName(1, row), "")
		wb.File.SetCellValue(sheet, cellName(2, row), "")
		wb.File.SetCellValue(sheet, cellName(3, row), periodEndLabel)
		wb.File.SetCellValue(sheet, cellName(4, row), "")
		wb.File.SetCellValue(sheet, cellName(5, row), "")
		wb.File.SetCellValue(sheet, cellName(6, row), endDir)
		wb.File.SetCellValue(sheet, cellName(7, row), centsToYuanStr(endDisp))

		endStyle, _ := wb.File.NewStyle(&excelize.Style{
			Font: &excelize.Font{Bold: true, Size: 10},
			Border: []excelize.Border{
				{Type: "bottom", Color: "#000000", Style: 2},
			},
		})
		wb.File.SetCellStyle(sheet, cellName(1, row), cellName(lastDetailCol, row), endStyle)
	}

	return nil
}
```

移除 `monthly_close_ml.go` 中不再需要的 `"sort"` import。

- [ ] **Step 2: 验证编译**

```bash
go build ./generator/...
```

- [ ] **Step 3: Commit**

```bash
git add generator/monthly_close_ml.go
git commit -m "feat: refactor WriteMLMonthClosings to use readMLDetailHeaders for column mapping"
```

## Task 6: JSON detailOrder 配置 — 解析与冲突检测

**Files:**
- Modify: `balance/balance.go`

- [ ] **Step 1: 在 `GlobalConfig` 中新增 `DetailOrder` 字段**

在 `GlobalConfig` 结构体中添加：

```go
type GlobalConfig struct {
	Settings    GlobalSettings         `json:"全局设置"`
	Tree        map[string]AccountNode `json:"科目树"`
	AutoItems   []AutoItem             `json:"自动识别科目"`
	ManualItems []ManualItem           `json:"手动调整科目"`
	DetailOrder map[string][]string    `json:"明细列顺序,omitempty"` // 多科目明细账列序配置
}
```

- [ ] **Step 2: 验证编译**

```bash
go build ./...
```

- [ ] **Step 3: Commit**

```bash
git add balance/balance.go
git commit -m "feat: add DetailOrder field to GlobalConfig for ML column ordering"
```

## Task 7: JSON detailOrder 自动回写

**Files:**
- Modify: `generator/ml_sheet.go`
- Modify: `generator/generate.go`

- [ ] **Step 1: 修改 `AppendMLEntries` 触发回写**

在 `AppendMLEntries` 中，`ensureMLSheet` 返回后检测 `newAppended`。需要在 `ensureMLSheet` 中也返回 `newAppended`。

修改 `ensureMLSheet` 签名增加返回值 `newAppended []string`：

```go
func (wb *Workbook) ensureMLSheet(general string, details []string, detailOrder []string) (string, map[string]int, []string, error)
```

在已有 Sheet 分支中返回 `newAppended`；新 Sheet 分支返回 `nil`。

- [ ] **Step 2: 在 `AppendMLEntries` 中收集并回写**

```go
func (wb *Workbook) AppendMLEntries(entries []voucher.Entry, initials map[string]int64) error {
	// ... 分组逻辑同上 ...

	newAppendedAll := make(map[string][]string) // general -> new details

	for general, g := range groups {
		if len(g.details) == 0 {
			continue
		}
		detailOrder := wb.Config.DetailOrder[general]
		_, detailIdx, newAppended, err := wb.ensureMLSheet(general, g.details, detailOrder)
		if err != nil {
			return err
		}
		if len(newAppended) > 0 {
			newAppendedAll[general] = newAppended
		}
		if err := wb.appendToMLSheet(general, g.entries, detailIdx, initials[general]); err != nil {
			return fmt.Errorf("多科目明细账 %s: %w", general, err)
		}
	}

	// 回写新科目到 DetailOrder
	if len(newAppendedAll) > 0 {
		if err := wb.writeBackDetailOrder(newAppendedAll); err != nil {
			return fmt.Errorf("回写 detailOrder: %w", err)
		}
	}

	return nil
}
```

- [ ] **Step 3: 新增 `writeBackDetailOrder` 方法**

```go
// writeBackDetailOrder 将新追加的明细科目增量写入 GlobalConfig.DetailOrder。
func (wb *Workbook) writeBackDetailOrder(newAppended map[string][]string) error {
	if wb.Config.DetailOrder == nil {
		wb.Config.DetailOrder = make(map[string][]string)
	}

	for general, newDetails := range newAppended {
		existing := wb.Config.DetailOrder[general]
		for _, nd := range newDetails {
			found := false
			for _, d := range existing {
				if d == nd {
					found = true
					break
				}
			}
			if !found {
				existing = append(existing, nd)
			}
		}
		wb.Config.DetailOrder[general] = existing
	}

	return balance.SaveConfig(wb.ConfigPath, wb.Config)
}
```

- [ ] **Step 4: 验证编译**

```bash
go build ./generator/...
```

- [ ] **Step 5: Commit**

```bash
git add generator/ml_sheet.go
git commit -m "feat: auto write-back new detail accounts to DetailOrder config"
```

## Task 8: 端到端验证

**Files:**
- None (test run)

- [ ] **Step 1: 运行现有测试**

```bash
go test ./...
```

Expected: all packages PASS.

- [ ] **Step 2: 清理旧输出并用 -f 重新生成 1-4 月**

```bash
rm -f test/e2e/out/2026/2026-0*.xlsx
go run . generate -f
```

(根据实际的 CLI 命令调整)

- [ ] **Step 3: 检查生成结果**

检查各月 xlsx 的多科目明细账 Sheet：
- 第2行标题列序在各月一致
- 历史数据行金额在正确标题列下
- 月结行（本月合计/本季合计/本年累计）的明细列与标题一致
- 过次页/承前页行列对齐

- [ ] **Step 4: Commit（若有测试数据更新）**

```bash
git add test/
git commit -m "test: regenerate e2e test data with stable ml columns"
```

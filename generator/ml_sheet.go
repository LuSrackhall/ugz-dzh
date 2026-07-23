package generator

import (
	"fmt"
	"sort"
	"strings"

	"ledger/generator/layout"
	"ledger/voucher"

	"github.com/xuri/excelize/v2"
)

const (
	mlMaxDetails    = 14 // 明细科目上限
	mlDetailStartCol = 8  // 明细列起始列 H（保留常量用于非 Layout 上下文）
)

// mlLayout 返回 ML 专用的 Layout（12 列/区，用于跨页布局）。
func mlLayout() layout.Layout {
	return layout.ComputeLayout(layout.DefaultMLSpec())
}

// mlBackBaseCol 返回 ML Back 区（反面）第 offset 个基础列的 Excel 列号。
func mlBackBaseCol(lay layout.Layout, offset int) int {
	return lay.BackStartCol + offset
}

// mlFrontDetailCol 返回 ML Front 区（正面）第 i 个明细列的 Excel 列号。
func mlFrontDetailCol(lay layout.Layout, i int) int {
	return lay.FrontStartCol + i
}

// mlPrintMarkCol 返回多科目明细账打印标记列号（Layout 内容区末列）。
func mlPrintMarkCol() int {
	lay := glLayout()
	return lay.FrontStartCol + 7 + mlMaxDetails
}

// mlDetailExcelCol 返回多科目明细账第 i 个明细列的 Excel 列号（Layout 坐标）。
func mlDetailExcelCol(lay layout.Layout, i int) int {
	return lay.FrontStartCol + 7 + i
}

// mlDetailRowIdx 返回多科目明细账第 i 个明细列在 GetRows 中的索引（Layout 坐标）。
func mlDetailRowIdx(lay layout.Layout, i int) int {
	return lay.BindingLeftCols + 7 + i
}

// mlDetailTotals 明细科目合计。
type mlDetailTotals struct {
	debit  int64
	credit int64
}

// ensureMLSheet 确保多科目明细账 Sheet 存在并已初始化标题和扩展列。
// 已存在的 Sheet 读头保序，新科目追加到右侧空列；全新 Sheet 按 detailOrder 或字母序初始化。
func (wb *Workbook) ensureMLSheet(general string, details []string, detailOrder []string) (string, map[string]int, []string, error) {
	name := sheetNameML(general)
	if idx, err := wb.File.GetSheetIndex(name); err == nil && idx >= 0 {
		// Sheet 已存在 — 读头保序
		existingIdx, existingDetails, err := wb.readMLDetailHeaders(name)
		if err != nil {
			return "", nil, nil, err
		}
		_ = existingIdx

		// 冲突检测：若配置了 detailOrder，逐列比对
		if len(detailOrder) > 0 {
			if err := wb.checkMLDetailOrderConflict(name, existingDetails, detailOrder); err != nil {
				return "", nil, nil, err
			}
		}

		// 合并新科目到空列
		finalDetails, finalIdx, newAppended, err := resolveMLDetailColumns(existingDetails, details, detailOrder)
		if err != nil {
			return "", nil, nil, err
		}
		_ = finalDetails

		// 更新标题行（仅更新新增的列）
			lay := glLayout()
			for _, nd := range newAppended {
				col := mlDetailExcelCol(lay, finalIdx[nd])
				cell := cellName(col, 2)
				wb.File.SetCellValue(name, cell, nd)
		}

		return name, finalIdx, newAppended, nil
	}

	// 新 Sheet — 创建
	idx, err := wb.File.NewSheet(name)
	if err != nil {
		return "", nil, nil, fmt.Errorf("创建 Sheet %s: %w", name, err)
	}
	wb.File.SetActiveSheet(idx)

	// 初始化列序：若存在 detailOrder，使用配置；否则按字母序
	var initDetails []string
	var newAppended []string
	if len(detailOrder) > 0 {
		// 直接复制 detailOrder 完整列表（含 "" 跳列和未发生科目）
		initDetails = make([]string, len(detailOrder))
		copy(initDetails, detailOrder)

		// 当月分录中不在 detailOrder 中的科目 → 追加到右侧空列
		inOrder := make(map[string]bool)
		for _, d := range detailOrder {
			if d != "" {
				inOrder[d] = true
			}
		}
		var remaining []string
		for _, d := range details {
			if !inOrder[d] {
				remaining = append(remaining, d)
			}
		}
		sort.Strings(remaining)
		initDetails = append(initDetails, remaining...)
		newAppended = remaining
	} else {
		initDetails = make([]string, len(details))
		copy(initDetails, details)
		sort.Strings(initDetails)
		newAppended = initDetails
	}

	if len(initDetails) > mlMaxDetails {
		return "", nil, nil, fmt.Errorf("总账科目 %q 明细科目数 %d 超过上限 %d", general, len(initDetails), mlMaxDetails)
	}

	if err := wb.writeMLTitle(name, general, initDetails); err != nil {
		return "", nil, nil, err
	}

	detailIdx := make(map[string]int)
	for i, d := range initDetails {
		if d != "" {
			detailIdx[d] = i
		}
	}

	return name, detailIdx, newAppended, nil
}

// checkMLDetailOrderConflict 逐列比对第2行标题与 detailOrder 配置。
func (wb *Workbook) checkMLDetailOrderConflict(sheet string, existingDetails []string, detailOrder []string) error {
	configIdx := 0
	for colIdx := 0; colIdx < mlMaxDetails && configIdx < len(detailOrder); colIdx++ {
		existing := existingDetails[colIdx]
		configured := detailOrder[configIdx]

		if configured == "" {
			if existing != "" {
				return fmt.Errorf("Sheet %s: detailOrder 与现有列序冲突 — 第 %d 列配置为空但实际为 %q。请使用 -f 从首月重新生成", sheet, colIdx+1, existing)
			}
			configIdx++
			continue
		}

		if existing == "" {
			found := false
			for j := colIdx + 1; j < mlMaxDetails; j++ {
				if existingDetails[j] == configured {
					found = true
					break
				}
			}
			if found {
				return fmt.Errorf("Sheet %s: detailOrder 与现有列序冲突 — %q 配置在第 %d 列但实际在更右侧。请使用 -f 从首月重新生成", sheet, configured, configIdx+1)
			}
			configIdx++
			continue
		}

		if existing != configured {
			return fmt.Errorf("Sheet %s: detailOrder 与现有列序冲突 — 第 %d 列配置为 %q 但实际为 %q。请使用 -f 从首月重新生成", sheet, configIdx+1, configured, existing)
		}
		configIdx++
	}
	return nil
}

// writeMLTitle 写入 ML 纸1正面空白占位表。
// Front 区：明细帐标题(4行) + 20行空白间距。Back 区留空。
func (wb *Workbook) writeMLTitle(sheet, general string, details []string) error {
	lay := mlLayout()
	darkGreen := "006100"
	sealRed := "CC0000"

	// Row 0: 明 细 帐（Front 区居中，绿色双下划线）
	fTitleStart := cellName(lay.FrontStartCol, 1)
	fTitleEnd := cellName(lay.FrontStartCol+9, 1)
	wb.File.SetCellValue(sheet, fTitleStart, "明    细    帐")
	wb.File.MergeCell(sheet, fTitleStart, fTitleEnd)
	titleStyle, _ := wb.File.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Size: 14, Color: darkGreen, Underline: "double"},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
	})
	wb.File.SetCellStyle(sheet, fTitleStart, fTitleEnd, titleStyle)
	wb.File.SetRowHeight(sheet, 1, 22)

	// Row 1: 科目名称（Front 区右侧，印章红）
	fAcctCell := cellName(lay.FrontStartCol+8, 2)
	wb.File.SetCellValue(sheet, fAcctCell, general)
	acctStyle, _ := wb.File.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Color: sealRed, Size: 10},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
	})
	wb.File.SetCellStyle(sheet, fAcctCell, cellName(lay.FrontStartCol+9, 2), acctStyle)

	// Row 2: 分第 1 页(右)（Front 区右侧）
	pnCell := cellName(lay.FrontStartCol+6, 3)
	pnEnd := cellName(lay.FrontStartCol+9, 3)
	wb.File.MergeCell(sheet, pnCell, pnEnd)
	wb.File.SetCellRichText(sheet, pnCell, []excelize.RichTextRun{
		{Text: "分第 ", Font: &excelize.Font{Color: darkGreen, Size: 10}},
		{Text: "1", Font: &excelize.Font{Color: sealRed, Size: 10}},
		{Text: " 页(右)", Font: &excelize.Font{Color: darkGreen, Size: 10}},
	})

	// Row 3: 列标题（Front 区 10 明细列头）
	headerNames := []string{"明细1", "明细2", "明细3", "明细4", "明细5", "明细6", "明细7", "明细8", "明细9", "明细10"}
	for i := 0; i < 10; i++ {
		cell := cellName(lay.FrontStartCol+i, 4)
		label := ""
		if i < len(details) {
			label = details[i]
		}
		_ = headerNames
		wb.File.SetCellValue(sheet, cell, label)
	}
	headerStyle, _ := wb.File.NewStyle(&excelize.Style{
		Font: &excelize.Font{Bold: true, Size: 10},
		Fill: excelize.Fill{Type: "pattern", Color: []string{"#D9E1F2"}, Pattern: 1},
		Border: []excelize.Border{{Type: "bottom", Color: "#808080", Style: 1}},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
	})
	hs := cellName(lay.FrontStartCol, 4)
	he := cellName(lay.FrontStartCol+9, 4)
	wb.File.SetCellStyle(sheet, hs, he, headerStyle)

	// 列宽
	for i := 0; i < 10; i++ {
		cl := cellColLetter(lay.FrontStartCol + i)
		wb.File.SetColWidth(sheet, cl, cl, 14)
	}

	return nil
}

// writeMLCrossoverTitle 写入逻辑页标题（Back 区 + Front 区）。
func (wb *Workbook) writeMLCrossoverTitle(sheet string, row int, pageNum int, general string, details []string) error {
	lay := mlLayout()
	darkGreen := "006100"
	sealRed := "CC0000"

	// Row 0: Back=科目名称, Front=明 细 帐
	bName := cellName(mlBackBaseCol(lay, 0), row)
	bNameEnd := cellName(mlBackBaseCol(lay, 2), row)
	wb.File.MergeCell(sheet, bName, bNameEnd)
	wb.File.SetCellValue(sheet, bName, general)

	fTitle := cellName(lay.FrontStartCol, row)
	fTitleEnd := cellName(lay.FrontStartCol+9, row)
	wb.File.MergeCell(sheet, fTitle, fTitleEnd)
	wb.File.SetCellValue(sheet, fTitle, "明    细    帐")
	titleStyle, _ := wb.File.NewStyle(&excelize.Style{
		Font: &excelize.Font{Bold: true, Size: 14, Color: darkGreen, Underline: "double"},
	})
	wb.File.SetCellStyle(sheet, fTitle, fTitleEnd, titleStyle)
	wb.File.SetRowHeight(sheet, row, 22)
	row++

	// Row 1: Back=分第N页(左), Front=科目名称(同行)
	bPage := cellName(mlBackBaseCol(lay, 0), row)
	bPageEnd := cellName(mlBackBaseCol(lay, 2), row)
	wb.File.MergeCell(sheet, bPage, bPageEnd)
	wb.File.SetCellRichText(sheet, bPage, []excelize.RichTextRun{
		{Text: fmt.Sprintf("分第 %d 页(左)", pageNum), Font: &excelize.Font{Color: darkGreen, Size: 10}},
	})

	fAcct := cellName(lay.FrontStartCol+8, row)
	wb.File.SetCellValue(sheet, fAcct, general)
	acctStyle, _ := wb.File.NewStyle(&excelize.Style{
		Font: &excelize.Font{Color: sealRed, Size: 10},
	})
	wb.File.SetCellStyle(sheet, fAcct, cellName(lay.FrontStartCol+9, row), acctStyle)
	row++

	// Row 2: Back=gap, Front=分第N页(右)
	fPage := cellName(lay.FrontStartCol+6, row)
	fPageEnd := cellName(lay.FrontStartCol+9, row)
	wb.File.MergeCell(sheet, fPage, fPageEnd)
	wb.File.SetCellRichText(sheet, fPage, []excelize.RichTextRun{
		{Text: fmt.Sprintf("分第 %d 页(右)", pageNum), Font: &excelize.Font{Color: darkGreen, Size: 10}},
	})
	row++

	// Row 3: 列标题
	// Back: 基础列(7) + 明细1~4
	for i, h := range glHeaders {
		cell := cellName(mlBackBaseCol(lay, i), row)
		wb.File.SetCellValue(sheet, cell, h)
	}
	for i := 0; i < 4 && i < len(details); i++ {
		cell := cellName(mlBackBaseCol(lay, 7+i), row)
		wb.File.SetCellValue(sheet, cell, details[i])
	}
	// Front: 明细5~14
	for i := 4; i < 14 && i < len(details); i++ {
		cell := cellName(lay.FrontStartCol+i-4, row)
		wb.File.SetCellValue(sheet, cell, details[i])
	}

	headerStyle, _ := wb.File.NewStyle(&excelize.Style{
		Font: &excelize.Font{Bold: true, Size: 10},
		Fill: excelize.Fill{Type: "pattern", Color: []string{"#D9E1F2"}, Pattern: 1},
		Border: []excelize.Border{{Type: "bottom", Color: "#808080", Style: 1}},
	})
	// Back header + Front header
	hsb := cellName(mlBackBaseCol(lay, 0), row)
	heb := cellName(mlBackBaseCol(lay, 10), row) // 7base+4detail-1
	wb.File.SetCellStyle(sheet, hsb, heb, headerStyle)
	hsf := cellName(lay.FrontStartCol, row)
	hef := cellName(lay.FrontStartCol+9, row)
	wb.File.SetCellStyle(sheet, hsf, hef, headerStyle)

	return nil
}

// cellColLetter 返回列号的字母表示。
func cellColLetter(col int) string {
	l, _ := excelize.ColumnNumberToName(col)
	return l
}

// updateMLDetailHeaders 更新已有 Sheet 的明细列标题，以匹配当月明细科目集。
func (wb *Workbook) updateMLDetailHeaders(sheet string, details []string) {
	lay := glLayout()
	for i := 0; i < mlMaxDetails; i++ {
		col := mlDetailExcelCol(lay, i)
		cell := cellName(col, 2)
		label := ""
		if i < len(details) {
			label = details[i]
		}
		wb.File.SetCellValue(sheet, cell, label)
	}
}

// readMLDetailHeaders 从 Sheet 第2行读取现有明细列标题，构建 detailName → colIndex 映射。
// 返回的 details 按列顺序排列（空列对应空字符串）。
func (wb *Workbook) readMLDetailHeaders(sheet string) (detailIdx map[string]int, details []string, err error) {
	lay := glLayout()
	detailIdx = make(map[string]int)
	details = make([]string, mlMaxDetails)

	rows, err := wb.File.GetRows(sheet)
	if err != nil {
		return nil, nil, fmt.Errorf("读取 Sheet %s: %w", sheet, err)
	}
	if len(rows) < 2 {
		return detailIdx, details, nil
	}

	row2 := rows[1]
	for i := 0; i < mlMaxDetails; i++ {
		colIdx := mlDetailRowIdx(lay, i) // GetRows 索引 = BindingLeftCols + 7 + i
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

// AppendMLEntries 将分录追加到多科目明细账 Sheet。
func (wb *Workbook) AppendMLEntries(entries []voucher.Entry, initials map[string]int64) error {
	type mlGroup struct {
		entries []voucher.Entry
		details []string
	}
	groups := make(map[string]*mlGroup)

	// 构建忽略集合
	mlSuppress := make(map[string]bool)
	for _, a := range wb.Config.Settings.MLSuppressAccounts {
		mlSuppress[a] = true
	}

	for _, e := range entries {
		// 若分录所属父级在多科目明细账忽略列表中，跳过
		if mlSuppress[e.GeneralAccount] {
			continue
		}

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
		_, detailIdx, newAppended, err := wb.ensureMLSheet(general, g.details, detailOrder)
		if err != nil {
			return err
		}

		// 回写：新科目追加到 detailOrder，或首次引导从标题初始化
		needsWriteback := false
		if len(newAppended) > 0 {
			needsWriteback = true
		}
		if detailOrder == nil {
			needsWriteback = true
		}
		if needsWriteback {
			if wb.Config.DetailOrder == nil {
				wb.Config.DetailOrder = make(map[string][]string)
			}
			if detailOrder == nil {
				// 首次引导：从 Sheet 标题读取现有列序作为初始 detailOrder
				_, existingDetails, err := wb.readMLDetailHeaders(sheetNameML(general))
				if err == nil {
					for _, d := range existingDetails {
						if d != "" {
							detailOrder = append(detailOrder, d)
						}
					}
				}
			}
			merged := detailOrder
			for _, nd := range newAppended {
				found := false
				for _, d := range merged {
					if d == nd {
						found = true
						break
					}
				}
				if !found {
					merged = append(merged, nd)
				}
			}
			wb.Config.DetailOrder[general] = merged
		}
		if err := wb.appendToMLSheet(general, g.entries, detailIdx, initials[general]); err != nil {
			return fmt.Errorf("多科目明细账 %s: %w", general, err)
		}
	}

	return nil
}

// appendToMLSheet 追加分录到指定总账科目的多科目明细账 Sheet。
func (wb *Workbook) appendToMLSheet(general string, entries []voucher.Entry, detailIdx map[string]int, initial int64) error {
	sheet := sheetNameML(general)
	lay := mlLayout()

	numDetails := mlMaxDetails

	rows, _ := wb.File.GetRows(sheet)
	isNew := len(rows) <= 2

	// 计算页码：已有过次页数 + 1
	pageNum := 1
	for _, r := range rows {
		if hasPageBreakAt(r, lay) {
			pageNum++
		}
	}

	if isNew && initial != 0 {
		row, err := wb.nextDataRow(sheet)
		if err != nil {
			return err
		}
		wb.writeMLCarryForwardRow(sheet, row, initial, 0, 0, make([]mlDetailTotals, numDetails), "上年结转")
	}

	row, err := wb.nextDataRow(sheet)
	if err != nil {
		return err
	}

	sort.Slice(entries, func(i, j int) bool {
		if entries[i].Date != entries[j].Date {
			return entries[i].Date < entries[j].Date
		}
		return entries[i].VoucherNum < entries[j].VoucherNum
	})

	balance := initial
	var pageDebit, pageCredit int64
	pageDetails := make([]mlDetailTotals, numDetails)

	if !isNew {
		balance = wb.lastPageBalance(sheet)
		if !wb.pageHasBreakRow(sheet) {
			wb.markExistingMLPageForPrint(sheet)
		}
	}

	for _, e := range entries {
		if wb.lastRowIsOrphanBreak(sheet) {
			pbDebit, pbCredit := wb.lastBreakTotals(sheet)
			pbDetails := wb.lastBreakDetailTotals(sheet)
			rows, _ := wb.File.GetRows(sheet)
			for _, r := range rows {
				if hasPageBreakAt(r, lay) {
					pageNum++
				}
			}
			wb.writeMLPageBreakRow(sheet, row, balance, pageDebit, pageCredit, pageDetails)
			row++
			pageNum++
			wb.writeMLCrossoverTitle(sheet, row, pageNum, general, nil)
			row += lay.DataStartRow
			wb.writeMLCarryForwardRow(sheet, row, balance, pbDebit, pbCredit, pbDetails, carryForwardLabel)
			row++
			pageDebit = 0
			pageCredit = 0
			pageDetails = make([]mlDetailTotals, numDetails)
		}

		if wb.rowIsPageBreak(sheet, row) {
			wb.writeMLPageBreakRow(sheet, row, balance, pageDebit, pageCredit, pageDetails)
			row++
			pageNum++
			wb.writeMLCrossoverTitle(sheet, row, pageNum, general, nil)
			row += lay.DataStartRow
			wb.writeMLCarryForwardRow(sheet, row, balance, pageDebit, pageCredit, pageDetails, carryForwardLabel)
			row++
			pageDebit = 0
			pageCredit = 0
			pageDetails = make([]mlDetailTotals, numDetails)
		}

		balance = balance + e.DebitCents - e.CreditCents
		pageDebit += e.DebitCents
		pageCredit += e.CreditCents

		dir, dispBal := directionFor(balance, 0)

		// Back 区：7 基础列
		wb.File.SetCellValue(sheet, cellName(mlBackBaseCol(lay, 0), row), e.Date)
		wb.File.SetCellValue(sheet, cellName(mlBackBaseCol(lay, 1), row), e.VoucherNum)
		wb.File.SetCellValue(sheet, cellName(mlBackBaseCol(lay, 2), row), e.Summary)
		wb.File.SetCellValue(sheet, cellName(mlBackBaseCol(lay, 3), row), centsToYuan(e.DebitCents))
		wb.File.SetCellValue(sheet, cellName(mlBackBaseCol(lay, 4), row), centsToYuan(e.CreditCents))
		wb.File.SetCellValue(sheet, cellName(mlBackBaseCol(lay, 5), row), dir)
		wb.File.SetCellValue(sheet, cellName(mlBackBaseCol(lay, 6), row), centsToYuan(dispBal))

		wb.setMoneyStyle(sheet, row, mlBackBaseCol(lay, 3))
		wb.setMoneyStyle(sheet, row, mlBackBaseCol(lay, 4))
		wb.setMoneyStyle(sheet, row, mlBackBaseCol(lay, 6))

		// Back 区 + Front 区：14 明细列
		if e.DetailAccount != "" {
			if idx, ok := detailIdx[e.DetailAccount]; ok {
				net := e.DebitCents - e.CreditCents
				var col int
				if idx < 4 {
					// 明细 1~4 → Back 区
					col = mlBackBaseCol(lay, 7+idx)
				} else {
					// 明细 5~14 → Front 区
					col = lay.FrontStartCol + (idx - 4)
				}
				wb.File.SetCellValue(sheet, cellName(col, row), centsToYuan(net))
				wb.setMoneyStyle(sheet, row, col)
				pageDetails[idx].debit += e.DebitCents
				pageDetails[idx].credit += e.CreditCents
			}
		}

		wb.markMLRowForPrint(sheet, row)
		row++
	}

	return nil
}

// writeMLPageBreakRow 写多科目明细账的"过次页"行，A-G 总计 + H-U 各明细本页净额。
func (wb *Workbook) writeMLPageBreakRow(sheet string, row int, balance int64, pageDebit, pageCredit int64, pageDetails []mlDetailTotals) {
	lay := mlLayout()
	dir, dispBal := directionFor(balance, 0)
	// Back 区：基础列
	wb.File.SetCellValue(sheet, cellName(mlBackBaseCol(lay, 0), row), "")
	wb.File.SetCellValue(sheet, cellName(mlBackBaseCol(lay, 1), row), "")
	wb.File.SetCellValue(sheet, cellName(mlBackBaseCol(lay, 2), row), pageBreakLabel)
	wb.File.SetCellValue(sheet, cellName(mlBackBaseCol(lay, 3), row), centsToYuan(pageDebit))
	wb.File.SetCellValue(sheet, cellName(mlBackBaseCol(lay, 4), row), centsToYuan(pageCredit))
	wb.File.SetCellValue(sheet, cellName(mlBackBaseCol(lay, 5), row), dir)
	wb.File.SetCellValue(sheet, cellName(mlBackBaseCol(lay, 6), row), centsToYuan(dispBal))
	wb.setMoneyStyle(sheet, row, mlBackBaseCol(lay, 3))
	wb.setMoneyStyle(sheet, row, mlBackBaseCol(lay, 4))
	wb.setMoneyStyle(sheet, row, mlBackBaseCol(lay, 6))

	// Back 区：明细 1~4
	for i := 0; i < 4 && i < len(pageDetails); i++ {
		net := pageDetails[i].debit - pageDetails[i].credit
		col := mlBackBaseCol(lay, 7+i)
		wb.File.SetCellValue(sheet, cellName(col, row), centsToYuan(net))
		wb.setMoneyStyle(sheet, row, col)
	}
	// Front 区：明细 5~14
	for i := 4; i < 14 && i < len(pageDetails); i++ {
		net := pageDetails[i].debit - pageDetails[i].credit
		col := lay.FrontStartCol + (i - 4)
		wb.File.SetCellValue(sheet, cellName(col, row), centsToYuan(net))
		wb.setMoneyStyle(sheet, row, col)
	}
}

// writeMLCarryForwardRow 写多科目明细账的"承前页"行，与过次页数据相同。
func (wb *Workbook) writeMLCarryForwardRow(sheet string, row int, balance int64, pageDebit, pageCredit int64, pageDetails []mlDetailTotals, label string) {
	lay := mlLayout()
	dir, dispBal := directionFor(balance, 0)
	// Back 区：基础列
	wb.File.SetCellValue(sheet, cellName(mlBackBaseCol(lay, 0), row), "")
	wb.File.SetCellValue(sheet, cellName(mlBackBaseCol(lay, 1), row), "")
	wb.File.SetCellValue(sheet, cellName(mlBackBaseCol(lay, 2), row), label)
	wb.File.SetCellValue(sheet, cellName(mlBackBaseCol(lay, 3), row), centsToYuan(pageDebit))
	wb.File.SetCellValue(sheet, cellName(mlBackBaseCol(lay, 4), row), centsToYuan(pageCredit))
	wb.File.SetCellValue(sheet, cellName(mlBackBaseCol(lay, 5), row), dir)
	wb.File.SetCellValue(sheet, cellName(mlBackBaseCol(lay, 6), row), centsToYuan(dispBal))
	wb.setMoneyStyle(sheet, row, mlBackBaseCol(lay, 3))
	wb.setMoneyStyle(sheet, row, mlBackBaseCol(lay, 4))
	wb.setMoneyStyle(sheet, row, mlBackBaseCol(lay, 6))

	// Back 区：明细 1~4
	for i := 0; i < 4 && i < len(pageDetails); i++ {
		net := pageDetails[i].debit - pageDetails[i].credit
		col := mlBackBaseCol(lay, 7+i)
		wb.File.SetCellValue(sheet, cellName(col, row), centsToYuan(net))
		wb.setMoneyStyle(sheet, row, col)
	}
	// Front 区：明细 5~14
	for i := 4; i < 14 && i < len(pageDetails); i++ {
		net := pageDetails[i].debit - pageDetails[i].credit
		col := lay.FrontStartCol + (i - 4)
		wb.File.SetCellValue(sheet, cellName(col, row), centsToYuan(net))
		wb.setMoneyStyle(sheet, row, col)
	}
}


// writeMLPageHeader 写入多科目明细账后续页标题行（过次页之后、承前页之前调用），
// 包含页码、科目名称、多科目明细账标题、列标题。
// 行结构（5 行，与 writePageHeader 一致）：
//   Row N+0: 分第 n 页（右侧，绿色+数字红色）
//   Row N+1: 多科目明细账 — 科目名（居中，绿色+双下划线）| 科目名称（右侧）
//   Row N+2: 科目名称（右侧，印章红）
//   Row N+3: [空行]
//   Row N+4: 日期│凭证号│摘要│借方金额│贷方金额│方向│余额
func (wb *Workbook) writeMLPageHeader(sheet string, row int, pageNum int, general string) error {
	lay := glLayout()

	darkGreen := "006100"
	sealRed := "CC0000"

	// Row N+0: 分第 n 页（右侧，绿色，数字印章红）
	pnLeft := cellName(lay.AccountColLeft, row)
	pnRight := cellName(lay.AccountColRight, row)
	wb.File.MergeCell(sheet, pnLeft, pnRight)
	wb.File.SetCellRichText(sheet, pnLeft, []excelize.RichTextRun{
		{Text: "分第 ", Font: &excelize.Font{Color: darkGreen, Size: 10}},
		{Text: fmt.Sprintf("%d", pageNum), Font: &excelize.Font{Color: sealRed, Size: 10}},
		{Text: " 页", Font: &excelize.Font{Color: darkGreen, Size: 10}},
	})
	wb.File.SetRowHeight(sheet, row, 18)
	row++

	// Row N+1: 多科目明细账 — 科目名（居中）+ 科目名称（右侧）
	tl := cellName(lay.TitleColLeft, row)
	tr := cellName(lay.TitleColRight, row)
	wb.File.MergeCell(sheet, tl, tr)
	wb.File.SetCellValue(sheet, tl, "多科目明细账 — "+general)
	titleStyle, _ := wb.File.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Size: 14, Color: darkGreen, Underline: "double"},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
	})
	wb.File.SetCellStyle(sheet, tl, tr, titleStyle)

	al := cellName(lay.AccountColLeft, row)
	ar := cellName(lay.AccountColRight, row)
	wb.File.MergeCell(sheet, al, ar)
	wb.File.SetCellValue(sheet, al, general)
	accStyle, _ := wb.File.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Color: sealRed, Size: 10},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
	})
	wb.File.SetCellStyle(sheet, al, ar, accStyle)
	wb.File.SetRowHeight(sheet, row, 28)
	row++

	// Row N+2: 科目名称（右侧，印章红）
	acLeft := cellName(lay.AccountColLeft, row)
	acRight := cellName(lay.AccountColRight, row)
	wb.File.MergeCell(sheet, acLeft, acRight)
	wb.File.SetCellValue(sheet, acLeft, general)
	acRowStyle, _ := wb.File.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Color: sealRed, Size: 10},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
	})
	wb.File.SetCellStyle(sheet, acLeft, acRight, acRowStyle)
	wb.File.SetRowHeight(sheet, row, 18)
	row++

	// Row N+3: [空行]
	row++

	// Row N+4: 列标题
	colNames := []string{"日期", "凭证号", "摘要", "借方金额", "贷方金额", "方向", "余额"}
	for i, h := range colNames {
		cell := cellName(lay.FrontStartCol+i, row)
		wb.File.SetCellValue(sheet, cell, h)
	}
	headerStyle, _ := wb.File.NewStyle(&excelize.Style{
		Font: &excelize.Font{Bold: true, Size: 10},
		Fill: excelize.Fill{Type: "pattern", Color: []string{"#D9E1F2"}, Pattern: 1},
		Border: []excelize.Border{
			{Type: "bottom", Color: "#808080", Style: 1},
		},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
	})
	hs := cellName(lay.FrontStartCol, row)
	he := cellName(lay.FrontStartCol+len(colNames)-1, row)
	wb.File.SetCellStyle(sheet, hs, he, headerStyle)

	return nil
}

// lastBreakDetailTotals 读取最后一个过次页行的各明细列净额。
func (wb *Workbook) lastBreakDetailTotals(sheet string) []mlDetailTotals {
	lay := glLayout()
	rows, err := wb.File.GetRows(sheet)
	if err != nil {
		return make([]mlDetailTotals, mlMaxDetails)
	}
	for i := len(rows) - 1; i >= 0; i-- {
		if hasPageBreakAt(rows[i], lay) {
			result := make([]mlDetailTotals, mlMaxDetails)
			for j := 0; j < mlMaxDetails; j++ {
				colIdx := mlDetailRowIdx(lay, j) // GetRows 索引 = BindingLeftCols + 7 + j
				if colIdx < len(rows[i]) {
					if v, err := yuanStrToCents(rows[i][colIdx]); err == nil {
						if v >= 0 {
							result[j].debit = v
						} else {
							result[j].credit = -v
						}
					}
				}
			}
			return result
		}
	}
	return make([]mlDetailTotals, mlMaxDetails)
}
